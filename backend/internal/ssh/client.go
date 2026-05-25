// Package ssh connects to remote servers over SSH to probe their specs and run
// the bootstrap script. It satisfies the ServerProber, ServerBootstrapper, and
// KeyMaker ports declared in internal/usecase.
package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"embed"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
	"golang.org/x/crypto/ssh"
)

//go:embed scripts/bootstrap.sh.tmpl
var scripts embed.FS

// Client connects to servers over SSH. One Client serves probing, bootstrap,
// and key generation.
type Client struct {
	dialTimeout time.Duration
}

var (
	_ usecase.ServerProber       = (*Client)(nil)
	_ usecase.ServerBootstrapper = (*Client)(nil)
	_ usecase.KeyMaker           = (*Client)(nil)
	_ usecase.LocalKeyProvider   = (*Client)(nil)
)

// NewClient returns an SSH client with a sensible dial timeout.
func NewClient() *Client {
	return &Client{dialTimeout: 15 * time.Second}
}

// dial opens an SSH connection and verifies the server's host key. When
// t.Fingerprint is empty (first contact with a fresh VPS) it captures the key's
// fingerprint trust-on-first-use, so the caller can pin it. When t.Fingerprint
// is set, the presented key MUST match it or the connection is refused — host
// key verification runs before authentication, so this also stops a
// man-in-the-middle from ever receiving the root password.
func (c *Client) dial(ctx context.Context, t usecase.SSHTarget) (*ssh.Client, string, error) {
	var fingerprint string
	verify := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint = ssh.FingerprintSHA256(key)
		if t.Fingerprint != "" && fingerprint != t.Fingerprint {
			return fmt.Errorf("host key mismatch for %s: presented %s, expected %s — refusing to connect (possible man-in-the-middle)", t.Host, fingerprint, t.Fingerprint)
		}
		return nil
	}
	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            []ssh.AuthMethod{ssh.Password(t.Password)},
		HostKeyCallback: verify,
		Timeout:         c.dialTimeout,
	}

	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	dialer := net.Dialer{Timeout: c.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(sshConn, chans, reqs), fingerprint, nil
}

const probeScript = `. /etc/os-release 2>/dev/null || true
echo "os=${NAME:-unknown} ${VERSION_ID:-}"
echo "kernel=$(uname -r)"
echo "arch=$(uname -m)"
echo "cores=$(nproc 2>/dev/null || echo 0)"
echo "cpu=$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | sed 's/.*: //')"
echo "mem_mb=$(free -m 2>/dev/null | awk '/^Mem:/{print $2}')"
echo "disk_gb=$(df -BG / 2>/dev/null | awk 'NR==2{gsub("G","",$2); print $2}')"
echo "hostname=$(hostname)"
`

// Probe connects and reads the server's hardware and OS details. It returns the
// detected specs and the host key fingerprint captured during the handshake.
func (c *Client) Probe(ctx context.Context, t usecase.SSHTarget) (usecase.ServerSpecs, string, error) {
	client, fingerprint, err := c.dial(ctx, t)
	if err != nil {
		return usecase.ServerSpecs{}, "", err
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return usecase.ServerSpecs{}, fingerprint, fmt.Errorf("new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	out, err := session.Output(probeScript)
	if err != nil {
		return usecase.ServerSpecs{}, fingerprint, fmt.Errorf("probe command: %w", err)
	}
	return parseSpecs(string(out)), fingerprint, nil
}

func parseSpecs(out string) usecase.ServerSpecs {
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(line, "=")
		if ok {
			fields[key] = strings.TrimSpace(val)
		}
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }
	return usecase.ServerSpecs{
		OS:       strings.TrimSpace(fields["os"]),
		Kernel:   fields["kernel"],
		Arch:     fields["arch"],
		CPUCores: atoi(fields["cores"]),
		CPUModel: fields["cpu"],
		MemoryMB: atoi(fields["mem_mb"]),
		DiskGB:   atoi(fields["disk_gb"]),
		Hostname: fields["hostname"],
	}
}

// Bootstrap renders the bootstrap script for the given parameters and runs it as
// root over SSH, streaming combined stdout/stderr to out as it executes. The run
// is aborted if ctx is cancelled (e.g. the client disconnects).
func (c *Client) Bootstrap(ctx context.Context, t usecase.SSHTarget, p usecase.BootstrapParams, out io.Writer) error {
	script, err := renderBootstrap(p)
	if err != nil {
		return err
	}

	client, _, err := c.dial(ctx, t)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	session.Stdout = out
	session.Stderr = out
	session.Stdin = strings.NewReader(script)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGTERM)
			_ = session.Close()
		case <-done:
		}
	}()

	if err := session.Run("bash -s"); err != nil {
		return fmt.Errorf("bootstrap script: %w", err)
	}
	return nil
}

func renderBootstrap(p usecase.BootstrapParams) (string, error) {
	raw, err := scripts.ReadFile("scripts/bootstrap.sh.tmpl")
	if err != nil {
		return "", fmt.Errorf("read bootstrap template: %w", err)
	}
	tmpl, err := template.New("bootstrap").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse bootstrap template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("render bootstrap template: %w", err)
	}
	return buf.String(), nil
}

// LocalPublicKey reads the operator's own SSH public key from this machine,
// trying the common key names in order. It returns "" (no error) when none is
// found, so a user without a local key simply doesn't get one installed.
func (c *Client) LocalPublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil //nolint:nilerr // no home dir → no local key to offer, not a failure
	}
	for _, name := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
		//nolint:gosec // G304: path is the OS home dir joined with a fixed allowlist of public-key filenames — no user-controlled input
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		if key := strings.TrimSpace(string(data)); key != "" {
			return key, nil
		}
	}
	return "", nil
}

// Generate creates an ed25519 keypair for mountabo's access to a server. It
// returns the private key as an OpenSSH PEM and the public key in
// authorized_keys form.
func (c *Client) Generate(comment string) (privatePEM, publicKey string, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("build signer: %w", err)
	}
	authorized := ssh.MarshalAuthorizedKey(signer.PublicKey())

	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	return string(pem.EncodeToMemory(block)), strings.TrimSpace(string(authorized)), nil
}

package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ErrSetupInProgress is returned when a setup is already running for a server,
// preventing concurrent or duplicate bootstrap runs against the same host.
var ErrSetupInProgress = errors.New("setup already in progress for this server")

// BootstrapUser is the non-root account mountabo creates and uses on every
// server. Fixed by product decision.
const BootstrapUser = "mountabo"

// ErrServerNotFound is returned when no server matches an id.
var ErrServerNotFound = errors.New("server not found")

// ServerStatus tracks where a server is in its lifecycle.
type ServerStatus string

// Server lifecycle states.
const (
	StatusProbed    ServerStatus = "probed"     // added + specs detected, not yet bootstrapped
	StatusSettingUp ServerStatus = "setting_up" // bootstrap running
	StatusReady     ServerStatus = "ready"      // bootstrap completed
	StatusFailed    ServerStatus = "failed"     // bootstrap failed; can retry
)

// ServerSpecs is the hardware/OS detail probed over SSH.
type ServerSpecs struct {
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	CPUCores int    `json:"cpuCores"`
	CPUModel string `json:"cpuModel"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Hostname string `json:"hostname"`
}

// Server is a VPS the user has added. Secrets (root password, mountabo private
// key) live in the vault, never on this struct.
type Server struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	IP          string       `json:"ip"`
	SSHPort     int          `json:"sshPort"`
	Timezone    string       `json:"timezone"`
	Status      ServerStatus `json:"status"`
	Specs       ServerSpecs  `json:"specs"`
	Fingerprint string       `json:"fingerprint"`
	CreatedAt   time.Time    `json:"createdAt"`
	// UserPublicKey is the operator's own SSH public key (a public value, safe to
	// store) installed alongside mountabo's so the human can also reach the box.
	UserPublicKey string `json:"userPublicKey,omitempty"`
	// Options are the ids of opt-in hardening settings currently applied to the
	// server (persisted). Toggled per-server and applied live via ApplyOptions.
	Options []string `json:"options"`
}

// SSHTarget is where and how to reach a server over SSH.
type SSHTarget struct {
	Host     string
	Port     int
	User     string
	Password string
	// PrivateKey, when set, is used for key-based auth (the mountabo user)
	// instead of Password (used for the initial root connection).
	PrivateKey string
	// Fingerprint is the expected SHA256 host key fingerprint. Empty means
	// trust-on-first-use: the dialer captures and returns the key's fingerprint
	// so it can be pinned. Non-empty means the connection MUST present this exact
	// key or be refused (man-in-the-middle protection). Host key verification
	// happens before authentication, so pinning also protects the password.
	Fingerprint string
}

// BootstrapParams fills the bootstrap script template.
type BootstrapParams struct {
	User      string
	Timezone  string
	PublicKey string // mountabo's generated public key
	// UserPublicKey is the operator's own public key, installed alongside
	// mountabo's so they can SSH in directly. Empty if none is available.
	UserPublicKey string
	// Options are the ids of opt-in hardening steps to append after the base
	// setup, in catalog order. Empty means base setup only.
	Options []string
}

// SetupOption is an opt-in hardening step the operator can choose to apply. The
// Description explains what it does and its trade-off so the choice is informed.
// Category groups related options in the UI.
type SetupOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// SetupOptions is the catalog of optional steps, every one a parameterless
// on/off toggle with matching enable/disable scripts. Array order is the apply
// order (harden-ssh last, since it can disable root/password login); the UI
// groups by Category. None are applied unless the operator selects them.
var SetupOptions = []SetupOption{
	{ID: "firewall", Category: "Network", Name: "UFW firewall", Description: "Block inbound traffic except SSH, HTTP and HTTPS. Reduces exposure — but Docker can publish container ports past UFW unless you bind them to localhost."},

	{ID: "ssh-limits", Category: "SSH", Name: "Limit SSH auth attempts", Description: "Set MaxAuthTries 3 and LoginGraceTime 30. Cheaper than fail2ban — but multi-key ssh-agents can hit the limit on legitimate logins."},
	{ID: "ssh-allowusers", Category: "SSH", Name: "Restrict SSH to mountabo + root", Description: "Whitelist who may SSH in (AllowUsers mountabo root). Blocks everyone else silently — handy, until you forget you added it."},
	{ID: "fail2ban", Category: "SSH", Name: "fail2ban", Description: "Temporarily ban IPs after repeated failed SSH logins, to blunt brute-force. Can briefly lock you out if you mistype your own login several times."},
	{ID: "crowdsec", Category: "SSH", Name: "CrowdSec", Description: "Modern fail2ban with community-shared blocklists — blocks IPs flagged across the network. Better signal, heavier; the shared list occasionally bans something unexpected."},

	{ID: "netdata", Category: "Monitoring", Name: "Netdata (local)", Description: "Real-time CPU/RAM/disk/network/Docker dashboard, bound to 127.0.0.1:19999. Reach it via an SSH tunnel or a reverse proxy with auth."},
	{ID: "uptime-kuma", Category: "Monitoring", Name: "Uptime Kuma", Description: "Self-hosted uptime monitor with notifications, on 127.0.0.1:3001. Runs on the box it monitors, so host critical alerts elsewhere too."},
	{ID: "ntfy", Category: "Monitoring", Name: "ntfy", Description: "Self-hosted push notifications on 127.0.0.1:8080 — pipe alerts in via curl. Dead simple; self-host avoids the public server's rate limits."},
	{ID: "journald-persistent", Category: "Monitoring", Name: "Persistent journald logs", Description: "Keep system logs across reboots (capped at 2G) so postmortems are possible."},

	{ID: "auto-updates", Category: "System", Name: "Automatic security updates", Description: "Install unattended-upgrades so security patches apply on their own. Keeps the box patched; may occasionally trigger a reboot."},
	{ID: "sysctl", Category: "System", Name: "sysctl hardening", Description: "Kernel knobs: SYN cookies, restricted dmesg, reverse-path filtering, no ICMP redirects. Free defense-in-depth."},
	{ID: "chrony", Category: "System", Name: "Accurate time (chrony)", Description: "Keep the clock accurate — required for valid TLS certs, JWT validation, and log correlation."},
	{ID: "zram", Category: "System", Name: "zram compressed swap", Description: "Compressed-RAM swap so the OOM killer doesn't fire on transient spikes. Buys headroom on small boxes."},
	{ID: "swapfile", Category: "System", Name: "4G swap file", Description: "Disk-backed swap (4G) for memory headroom. Slower than RAM and wears SSDs over time — prefer zram on tiny boxes."},
	{ID: "ulimits", Category: "System", Name: "Raise file-descriptor limits", Description: "Raise nofile to 65535 for services that accept many connections, to avoid 'too many open files'."},

	{ID: "aide", Category: "Audit", Name: "AIDE file integrity", Description: "Hash critical system files and alert on changes — catches tampering. Package updates trigger alerts you must re-baseline."},
	{ID: "auditd", Category: "Audit", Name: "auditd", Description: "Kernel-level syscall audit trail with user/process context. Gold standard for forensics — logs are noisy."},
	{ID: "rkhunter", Category: "Audit", Name: "rkhunter", Description: "Installs the rootkit scanner for on-demand checks. Cheap insurance; signature-based, so it misses novel rootkits."},

	{ID: "harden-ssh", Category: "SSH", Name: "Harden SSH (key-only)", Description: "Disable root login and password authentication — SSH becomes key-only. Strong, BUT you can be locked out if your key isn't installed, and your root password stops working over SSH (only the provider console remains)."},
}

// canonicalOptions returns the requested option ids that exist in the catalog,
// in catalog order, dropping unknowns and duplicates.
func canonicalOptions(requested []string) []string {
	want := map[string]bool{}
	for _, id := range requested {
		want[id] = true
	}
	var out []string
	for _, opt := range SetupOptions {
		if want[opt.ID] {
			out = append(out, opt.ID)
		}
	}
	return out
}

// ── ports (consumed here, implemented by adapters) ──

// ServerProber connects to a server and reads its specs, returning the host key
// fingerprint captured on connect.
type ServerProber interface {
	Probe(ctx context.Context, t SSHTarget) (ServerSpecs, string, error)
}

// ServerBootstrapper runs the bootstrap script over SSH, streaming output to out.
type ServerBootstrapper interface {
	Bootstrap(ctx context.Context, t SSHTarget, p BootstrapParams, out io.Writer) error
}

// OptionApplier enables/disables hardening options on an already-set-up server
// (connecting as the mountabo user via its key, running with sudo), streaming
// output to out. add and remove are option ids in catalog order.
type OptionApplier interface {
	ApplyOptions(ctx context.Context, t SSHTarget, add, remove []string, out io.Writer) error
}

// KeyMaker generates an SSH keypair for mountabo's access to a server.
type KeyMaker interface {
	Generate(comment string) (privatePEM, publicKey string, err error)
}

// LocalKeyProvider reads the operator's own SSH public key from this machine
// (mountabo runs locally), so it can be installed for their direct access.
// Returns "" (no error) when none is found.
type LocalKeyProvider interface {
	LocalPublicKey() (string, error)
}

// ServerStore persists servers (a JSON file today, SQLite later).
type ServerStore interface {
	List() ([]Server, error)
	Get(id string) (Server, error)
	Save(s Server) error
	Delete(id string) error
}

// SecretVault stores arbitrary secrets in the OS keychain (root passwords,
// generated private keys), keyed by an opaque string.
type SecretVault interface {
	SaveSecret(key, value string) error
	LoadSecret(key string) (string, error)
	DeleteSecret(key string) error
}

// AddServerInput is what the user supplies to add a server.
type AddServerInput struct {
	Name          string
	IP            string
	Port          int
	Timezone      string
	RootPassword  string
	UserPublicKey string // optional: the operator's own public key (paste fallback)
}

// ServerService adds, lists, and bootstraps servers.
type ServerService struct {
	store     ServerStore
	prober    ServerProber
	boot      ServerBootstrapper
	applier   OptionApplier
	keys      KeyMaker
	localKeys LocalKeyProvider
	vault     SecretVault

	mu        sync.Mutex
	settingUp map[string]bool // server ids with a bootstrap/apply in flight
}

// NewServerService wires the service to its ports.
func NewServerService(store ServerStore, prober ServerProber, boot ServerBootstrapper, applier OptionApplier, keys KeyMaker, localKeys LocalKeyProvider, vault SecretVault) *ServerService {
	return &ServerService{store: store, prober: prober, boot: boot, applier: applier, keys: keys, localKeys: localKeys, vault: vault, settingUp: map[string]bool{}}
}

// Add connects to the server with the root password, probes its specs, and
// records it as "probed". The root password is held in the vault (encrypted by
// the OS) for the setup step and kept afterwards for root SSH/console access; it
// is destroyed only when the server is removed. The password never lands on the
// Server struct or in the store.
func (s *ServerService) Add(ctx context.Context, in AddServerInput) (Server, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.IP = strings.TrimSpace(in.IP)
	in.Timezone = strings.TrimSpace(in.Timezone)
	if in.Name == "" || in.IP == "" {
		return Server{}, fmt.Errorf("name and ip are required")
	}
	if in.RootPassword == "" {
		return Server{}, fmt.Errorf("root password is required")
	}
	if in.Timezone == "" {
		return Server{}, fmt.Errorf("timezone is required")
	}
	if in.Port == 0 {
		in.Port = 22
	}

	target := SSHTarget{Host: in.IP, Port: in.Port, User: "root", Password: in.RootPassword}
	specs, fingerprint, err := s.prober.Probe(ctx, target)
	if err != nil {
		return Server{}, fmt.Errorf("probe server: %w", err)
	}

	server := Server{
		ID:            newID(),
		Name:          in.Name,
		IP:            in.IP,
		SSHPort:       in.Port,
		Timezone:      in.Timezone,
		Status:        StatusProbed,
		Specs:         specs,
		Fingerprint:   fingerprint,
		CreatedAt:     time.Now().UTC(),
		UserPublicKey: strings.TrimSpace(in.UserPublicKey),
	}

	if err := s.vault.SaveSecret(rootPasswordKey(server.ID), in.RootPassword); err != nil {
		return Server{}, fmt.Errorf("store root password: %w", err)
	}
	if err := s.store.Save(server); err != nil {
		return Server{}, fmt.Errorf("save server: %w", err)
	}
	return server, nil
}

// Options returns the catalog of opt-in hardening steps for the UI to present.
func (s *ServerService) Options() []SetupOption {
	return SetupOptions
}

// List returns all added servers.
func (s *ServerService) List() ([]Server, error) {
	servers, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	return servers, nil
}

// Get returns one server by id.
func (s *ServerService) Get(id string) (Server, error) {
	server, err := s.store.Get(id)
	if err != nil {
		return Server{}, err
	}
	return server, nil
}

// Setup runs the bootstrap on a probed server, streaming progress to out. It
// generates mountabo's SSH key, installs the public half on the server, applies
// any opt-in hardening options the operator selected, and on success stores the
// private half in the vault and marks the server ready. Progress lines are
// written to out as they happen.
func (s *ServerService) Setup(ctx context.Context, id string, options []string, out io.Writer) error {
	// Guard against concurrent/duplicate bootstraps for the same server (e.g. a
	// reconnecting client) so we never run multiple SSH setups at once — which
	// could corrupt state or trip the server's fail2ban.
	s.mu.Lock()
	if s.settingUp[id] {
		s.mu.Unlock()
		return ErrSetupInProgress
	}
	s.settingUp[id] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.settingUp, id)
		s.mu.Unlock()
	}()

	server, err := s.store.Get(id)
	if err != nil {
		return err
	}

	password, err := s.vault.LoadSecret(rootPasswordKey(id))
	if err != nil {
		return fmt.Errorf("load root password (re-add the server): %w", err)
	}

	privateKey, publicKey, err := s.keys.Generate(fmt.Sprintf("mountabo@%s", server.Name))
	if err != nil {
		return fmt.Errorf("generate ssh key: %w", err)
	}

	server.Status = StatusSettingUp
	if err := s.store.Save(server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}

	// Resolve the operator's own public key so they get direct SSH access:
	// prefer one they pasted, else auto-detect a local ~/.ssh key. If neither
	// exists, only mountabo will have access — say so in the live log.
	userKey := strings.TrimSpace(server.UserPublicKey)
	if userKey == "" {
		if detected, derr := s.localKeys.LocalPublicKey(); derr == nil {
			userKey = detected
		}
	}
	if userKey == "" {
		_, _ = io.WriteString(out, "==> note: no personal SSH key found (~/.ssh) or pasted — only mountabo will have key access to this server\n")
	}

	// Pin the host key captured when the server was added: if it doesn't match,
	// the bootstrap (and the root password) must not go to an impostor.
	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: "root", Password: password, Fingerprint: server.Fingerprint}
	params := BootstrapParams{User: BootstrapUser, Timezone: server.Timezone, PublicKey: publicKey, UserPublicKey: userKey, Options: canonicalOptions(options)}

	if bootErr := s.boot.Bootstrap(ctx, target, params, out); bootErr != nil {
		server.Status = StatusFailed
		if err := s.store.Save(server); err != nil {
			return fmt.Errorf("save server after bootstrap failure: %w", err)
		}
		return fmt.Errorf("bootstrap: %w", bootErr)
	}

	if err := s.vault.SaveSecret(privateKeyKey(id), privateKey); err != nil {
		return fmt.Errorf("store mountabo key: %w", err)
	}
	// The root password is kept in the keychain (encrypted at rest), not
	// discarded. mountabo does not harden sshd, so the operator keeps using it
	// for root SSH/console access. It is destroyed only when the server is
	// removed.

	server.Status = StatusReady
	if err := s.store.Save(server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

// ApplyOptions changes which hardening options are applied to an already-set-up
// server to match desired: it connects as the mountabo user with the stored key
// (root password not needed) and runs the enable scripts for newly-ticked
// options and the disable scripts for unticked ones, streaming to out, then
// persists the new set. Only valid once the server is ready (the mountabo key
// exists).
func (s *ServerService) ApplyOptions(ctx context.Context, id string, desired []string, out io.Writer) error {
	s.mu.Lock()
	if s.settingUp[id] {
		s.mu.Unlock()
		return ErrSetupInProgress
	}
	s.settingUp[id] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.settingUp, id)
		s.mu.Unlock()
	}()

	server, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if server.Status != StatusReady {
		return fmt.Errorf("server must be set up before changing options")
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return fmt.Errorf("load mountabo key: %w", err)
	}

	desired = canonicalOptions(desired)
	current := canonicalOptions(server.Options)
	add := subtract(desired, current)
	remove := subtract(current, desired)
	if len(add) == 0 && len(remove) == 0 {
		_, _ = io.WriteString(out, "==> no changes to apply\n")
		return nil
	}

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	if err := s.applier.ApplyOptions(ctx, target, add, remove, out); err != nil {
		return fmt.Errorf("apply options: %w", err)
	}

	server.Options = desired
	if err := s.store.Save(server); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

// subtract returns the ids in a that are not in b, preserving a's order.
func subtract(a, b []string) []string {
	inB := map[string]bool{}
	for _, x := range b {
		inB[x] = true
	}
	var out []string
	for _, x := range a {
		if !inB[x] {
			out = append(out, x)
		}
	}
	return out
}

// Remove deletes a server and destroys its secrets, completing the key
// lifecycle: the mountabo private key and any retained root password are wiped
// from the keychain before the server record is dropped. Secret deletes are
// idempotent, so a partially-set-up server removes cleanly too.
func (s *ServerService) Remove(id string) error {
	if _, err := s.store.Get(id); err != nil {
		return err
	}
	if err := s.vault.DeleteSecret(privateKeyKey(id)); err != nil {
		return fmt.Errorf("destroy mountabo key: %w", err)
	}
	if err := s.vault.DeleteSecret(rootPasswordKey(id)); err != nil {
		return fmt.Errorf("destroy root password: %w", err)
	}
	if err := s.store.Delete(id); err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	return nil
}

func rootPasswordKey(id string) string { return "server-" + id + "-rootpw" }
func privateKeyKey(id string) string   { return "server-" + id + "-key" }

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

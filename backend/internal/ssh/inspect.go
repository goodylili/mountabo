package ssh

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.PortInspector = (*Client)(nil)

// listenPortsScript prints the local "address:port" of every listening TCP and
// UDP socket, one per line. ss needs no root to enumerate sockets (only to name
// the owning process, which we don't ask for), so this runs as the mountabo
// user without sudo.
const listenPortsScript = `ss -H -ltun 2>/dev/null | awk '{print $5}'`

// ListeningPorts connects as the target user and returns the distinct ports in
// a listening state on the server, sorted ascending.
func (c *Client) ListeningPorts(ctx context.Context, t usecase.SSHTarget) ([]int, error) {
	out, err := c.runOutput(ctx, t, listenPortsScript)
	if err != nil {
		return nil, fmt.Errorf("list listening ports: %w", err)
	}
	return parseListeningPorts(out), nil
}

// parseListeningPorts pulls the port out of each "address:port" line that ss
// prints (e.g. "0.0.0.0:22", "[::]:80", "127.0.0.1:323"), dropping anything
// without a numeric port, and returns the sorted distinct set.
func parseListeningPorts(out string) []int {
	seen := map[int]bool{}
	for _, line := range strings.Split(out, "\n") {
		field := strings.TrimSpace(line)
		idx := strings.LastIndex(field, ":")
		if idx < 0 {
			continue
		}
		port, err := strconv.Atoi(field[idx+1:])
		if err != nil {
			continue
		}
		seen[port] = true
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

// runOutput dials, runs command in a session, and returns its combined
// stdout/stderr. The session is closed if ctx is cancelled.
func (c *Client) runOutput(ctx context.Context, t usecase.SSHTarget, command string) (string, error) {
	client, _, err := c.dial(ctx, t)
	if err != nil {
		return "", err
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close()
		case <-done:
		}
	}()

	out, err := session.CombinedOutput(command)
	if err != nil {
		return string(out), fmt.Errorf("run %q: %w", command, err)
	}
	return string(out), nil
}

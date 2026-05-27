package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// ErrUnknownTool is returned when a dashboard request names a tool mountabo does
// not know how to open.
var ErrUnknownTool = errors.New("unknown monitoring tool")

// ErrToolNotInstalled is returned when a dashboard is requested for a tool that
// is not in the server's applied options.
var ErrToolNotInstalled = errors.New("monitoring tool not installed on this server")

// DashboardTool describes a self-hosted monitoring tool whose web UI binds to
// the server's loopback, so it can only be reached by tunneling over SSH.
type DashboardTool struct {
	// ID is the hardening option id (e.g. "uptime-kuma").
	ID string
	// Port is the loopback TCP port the tool's web UI listens on.
	Port int
}

// dashboardTools is the closed set of monitoring tools mountabo can open a
// tunnel to. Only Uptime Kuma has a web UI mountabo surfaces; a request for any
// other id is treated as an unknown tool. Keyed by option id.
var dashboardTools = map[string]DashboardTool{
	"uptime-kuma": {ID: "uptime-kuma", Port: 3001},
}

// DashboardToolByID returns the tool for an id and whether it is known.
func DashboardToolByID(id string) (DashboardTool, bool) {
	t, ok := dashboardTools[id]
	return t, ok
}

// DashboardTunnel opens a local TCP forward to a loopback address on a server,
// carried over an SSH connection (the `ssh -L` model). The listener binds to an
// ephemeral 127.0.0.1 port on this machine; every connection it accepts is dialed
// to 127.0.0.1:<port> through the SSH transport and bytes are copied both ways.
// Because it forwards raw TCP, it carries HTTP and websockets transparently and
// serves the tool at the root of the local port. It only forwards; nothing
// destructive runs.
type DashboardTunnel interface {
	// OpenTunnel starts a forward and returns the local address it is listening
	// on (host:port) and a function to close it (which stops the listener and the
	// underlying SSH connection).
	OpenTunnel(ctx context.Context, t SSHTarget, port int) (localAddr string, stop func() error, err error)
}

// openTunnel is one live forward for a (serverId, tool) pair: the local address
// it serves and the closer that tears down its listener and SSH connection.
type openTunnel struct {
	localAddr string
	close     func() error
}

// ServerDashboardService opens SSH local port-forward tunnels to a set-up
// server's loopback monitoring dashboards (Uptime Kuma), connecting as the
// mountabo user with its stored key. The tunnel binds to this machine's loopback
// only (never network-exposed) and forwards raw TCP, so the dashboard's HTTP and
// websockets work and it is served at the root of the local port. It only allows
// known tools, and only when that tool is in the server's applied options, so a
// dashboard can never be opened for a tool that was never installed. Open tunnels
// are reused for the same (server, tool) pair and closed on shutdown.
type ServerDashboardService struct {
	servers ServerStore
	vault   SecretVault
	tunnel  DashboardTunnel

	mu      sync.Mutex
	tunnels map[string]openTunnel // keyed by serverID + "/" + toolID
}

// NewServerDashboardService wires the service to its ports.
func NewServerDashboardService(servers ServerStore, vault SecretVault, tunnel DashboardTunnel) *ServerDashboardService {
	return &ServerDashboardService{
		servers: servers,
		vault:   vault,
		tunnel:  tunnel,
		tunnels: map[string]openTunnel{},
	}
}

// Open ensures an SSH tunnel to the named tool's loopback web UI on the server is
// running and returns the local URL (http://127.0.0.1:<port>/) the browser can
// load directly. The tool must be one mountabo knows how to open (ErrUnknownTool
// otherwise) and must be in the server's applied options (ErrToolNotInstalled
// otherwise); the server must be set up. An existing open tunnel for the same
// (server, tool) pair is reused rather than reopened.
func (s *ServerDashboardService) Open(ctx context.Context, id, toolID string) (string, error) {
	tool, ok := dashboardTools[toolID]
	if !ok {
		return "", ErrUnknownTool
	}

	server, err := s.servers.Get(id)
	if err != nil {
		return "", err
	}
	if server.Status != StatusReady {
		return "", ErrToolNotInstalled
	}
	if !slices.Contains(server.Options, toolID) {
		return "", ErrToolNotInstalled
	}

	key := tunnelKey(id, toolID)

	s.mu.Lock()
	if existing, ok := s.tunnels[key]; ok {
		s.mu.Unlock()
		return "http://" + existing.localAddr + "/", nil
	}
	s.mu.Unlock()

	secret, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return "", err
	}

	target := SSHTarget{
		Host:        server.IP,
		Port:        server.SSHPort,
		User:        BootstrapUser,
		PrivateKey:  secret,
		Fingerprint: server.Fingerprint,
	}
	localAddr, closeFn, err := s.tunnel.OpenTunnel(ctx, target, tool.Port)
	if err != nil {
		return "", fmt.Errorf("open dashboard tunnel: %w", err)
	}

	// A concurrent Open for the same pair may have raced us to a tunnel; if so,
	// keep theirs and tear ours down so only one listener lives per pair.
	s.mu.Lock()
	if existing, ok := s.tunnels[key]; ok {
		s.mu.Unlock()
		_ = closeFn()
		return "http://" + existing.localAddr + "/", nil
	}
	s.tunnels[key] = openTunnel{localAddr: localAddr, close: closeFn}
	s.mu.Unlock()

	return "http://" + localAddr + "/", nil
}

// Close stops every open tunnel and releases their listeners and SSH
// connections. It is called on backend shutdown so no forward outlives the
// process.
func (s *ServerDashboardService) Close() error {
	s.mu.Lock()
	tunnels := s.tunnels
	s.tunnels = map[string]openTunnel{}
	s.mu.Unlock()

	var errs []error
	for _, t := range tunnels {
		if err := t.close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func tunnelKey(serverID, toolID string) string { return serverID + "/" + toolID }

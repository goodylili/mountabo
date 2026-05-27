package usecase

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"slices"
)

// ErrUnknownTool is returned when a dashboard request names a tool mountabo does
// not know how to proxy.
var ErrUnknownTool = errors.New("unknown monitoring tool")

// ErrToolNotInstalled is returned when a dashboard is requested for a tool that
// is not in the server's applied options.
var ErrToolNotInstalled = errors.New("monitoring tool not installed on this server")

// DashboardTool describes a self-hosted monitoring tool whose web UI binds to
// the server's loopback, so it can only be reached by tunneling over SSH.
type DashboardTool struct {
	// ID is the hardening option id (e.g. "netdata").
	ID string
	// Port is the loopback TCP port the tool's web UI listens on.
	Port int
}

// dashboardTools is the closed set of monitoring tools mountabo can reverse
// proxy. journald-persistent has no web UI, so it is deliberately absent: a
// request for it is treated as an unknown tool. Keyed by option id.
var dashboardTools = map[string]DashboardTool{
	"netdata":     {ID: "netdata", Port: 19999},
	"uptime-kuma": {ID: "uptime-kuma", Port: 3001},
	"ntfy":        {ID: "ntfy", Port: 8080},
}

// DashboardTool returns the proxyable tool for an id and whether it is known.
func DashboardToolByID(id string) (DashboardTool, bool) {
	t, ok := dashboardTools[id]
	return t, ok
}

// DashboardRequest is one proxied HTTP request to a tool's loopback web UI,
// independent of any HTTP framework so the usecase owns its own shape. Path is
// the request path relative to the tool's root (no leading host), already
// including any query string.
type DashboardRequest struct {
	Method string
	// Path is the tool-relative request target, e.g. "/" or "/api/info?x=1".
	Path   string
	Header nethttp.Header
	Body   io.Reader
}

// DashboardResponse is the tool's reply, ready to be relayed back to the caller.
// Body must be closed by the caller.
type DashboardResponse struct {
	Status int
	Header nethttp.Header
	Body   io.ReadCloser
}

// DashboardTunnel proxies one HTTP request to a loopback address on a server
// through an SSH connection: it dials 127.0.0.1:port over the SSH transport and
// speaks HTTP to the tool, returning its response. It only reads/relays; nothing
// destructive runs.
type DashboardTunnel interface {
	Proxy(ctx context.Context, t SSHTarget, port int, req DashboardRequest) (DashboardResponse, error)
}

// ServerDashboardService reverse proxies a set-up server's loopback monitoring
// dashboards (Netdata, Uptime Kuma, ntfy) over its SSH connection, connecting as
// the mountabo user with its stored key. It only allows the known tools, and
// only when that tool is in the server's applied options, so a dashboard can
// never be reached for a tool that was never installed.
type ServerDashboardService struct {
	servers ServerStore
	vault   SecretVault
	tunnel  DashboardTunnel
}

// NewServerDashboardService wires the service to its ports.
func NewServerDashboardService(servers ServerStore, vault SecretVault, tunnel DashboardTunnel) *ServerDashboardService {
	return &ServerDashboardService{servers: servers, vault: vault, tunnel: tunnel}
}

// Proxy relays req to the named tool's loopback web UI on the server, over SSH.
// The tool must be one mountabo knows how to proxy (ErrUnknownTool otherwise)
// and must be in the server's applied options (ErrToolNotInstalled otherwise).
// The server must be set up. The caller owns the returned body and must close
// it.
func (s *ServerDashboardService) Proxy(ctx context.Context, id, toolID string, req DashboardRequest) (DashboardResponse, error) {
	tool, ok := dashboardTools[toolID]
	if !ok {
		return DashboardResponse{}, ErrUnknownTool
	}

	server, err := s.servers.Get(id)
	if err != nil {
		return DashboardResponse{}, err
	}
	if server.Status != StatusReady {
		return DashboardResponse{}, ErrToolNotInstalled
	}
	if !slices.Contains(server.Options, toolID) {
		return DashboardResponse{}, ErrToolNotInstalled
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return DashboardResponse{}, err
	}

	target := SSHTarget{
		Host:        server.IP,
		Port:        server.SSHPort,
		User:        BootstrapUser,
		PrivateKey:  key,
		Fingerprint: server.Fingerprint,
	}
	return s.tunnel.Proxy(ctx, target, tool.Port, req)
}

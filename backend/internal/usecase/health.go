package usecase

import (
	"context"
	"fmt"
	"net"
	"strconv"
)

// AppHealth is a point-in-time read of whether a deployed app is responding,
// probed over the server's existing SSH connection. Reachable is true when the
// probe got any HTTP response from the app; Status is the HTTP status code it
// returned (0 when the app did not answer at all). Target is the address that
// was probed (a loopback port or the app's domain), surfaced so the UI can say
// what it checked.
type AppHealth struct {
	Reachable bool   `json:"reachable"`
	Status    int    `json:"status"`
	Target    string `json:"target"`
	// Detail is a short, human-readable reason, set when the app is unreachable
	// (connection refused, timed out) or when no address could be derived.
	Detail string `json:"detail,omitempty"`
}

// AppProber probes an HTTP endpoint from the server itself over SSH (a curl of
// the given URL on the box) and reports whether it answered and with what HTTP
// status. It only reads; it never changes anything on the server.
type AppProber interface {
	ProbeHTTP(ctx context.Context, t SSHTarget, url string) (reachable bool, status int, err error)
}

// DeploymentLookup reads a single tracked deployment by its app name, so the
// health service can find which server and port to probe. Found is false when
// no deployment has that app.
type DeploymentLookup interface {
	List() ([]Deployment, error)
}

// AppHealthService reports whether a deployed app is up, by probing it from its
// own server over SSH: it curls the app on the server's loopback (the recorded
// published port) or its configured domain, and reports up/down plus the HTTP
// status. It connects as the mountabo user with the stored key and only reads.
type AppHealthService struct {
	deployments DeploymentLookup
	servers     ServerStore
	vault       SecretVault
	prober      AppProber
}

// NewAppHealthService wires the service to its ports.
func NewAppHealthService(deployments DeploymentLookup, servers ServerStore, vault SecretVault, prober AppProber) *AppHealthService {
	return &AppHealthService{deployments: deployments, servers: servers, vault: vault, prober: prober}
}

// Health probes the deployment named by app and reports whether it is up. It
// resolves the deployment's server and the address to probe (its domain, or its
// loopback published port), connects to the server as the mountabo user, and
// curls the app from the box. ErrDeploymentNotFound is returned when no
// deployment has that app; ErrServerNotFound propagates from the store. A probe
// that connects but gets no answer (app down) is NOT an error: it returns an
// AppHealth with Reachable false so the card can show "unhealthy".
func (s *AppHealthService) Health(ctx context.Context, app string) (AppHealth, error) {
	deployments, err := s.deployments.List()
	if err != nil {
		return AppHealth{}, fmt.Errorf("list deployments: %w", err)
	}
	var dep Deployment
	found := false
	for _, d := range deployments {
		if d.App == app {
			dep, found = d, true
			break
		}
	}
	if !found {
		return AppHealth{}, ErrDeploymentNotFound
	}

	server, err := s.servers.Get(dep.ServerID)
	if err != nil {
		return AppHealth{}, err
	}
	if server.Status != StatusReady {
		return AppHealth{}, fmt.Errorf("server must be set up before checking app health")
	}

	url, target := probeURL(server, dep)
	if url == "" {
		return AppHealth{
			Reachable: false,
			Detail:    "no published port or domain is configured for this app, so its health cannot be probed",
		}, nil
	}

	key, err := s.vault.LoadSecret(privateKeyKey(dep.ServerID))
	if err != nil {
		return AppHealth{}, fmt.Errorf("load server key: %w", err)
	}

	t := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	reachable, status, err := s.prober.ProbeHTTP(ctx, t, url)
	if err != nil {
		return AppHealth{}, fmt.Errorf("probe app: %w", err)
	}

	h := AppHealth{Reachable: reachable, Status: status, Target: target}
	if !reachable {
		h.Detail = "the app did not respond on " + target
	}
	return h, nil
}

// probeURL derives the URL to curl from the server and prefers a configured
// domain (over HTTPS) so the check follows the same path a visitor takes; absent
// a domain it curls the app on the server's loopback at its recorded published
// port. It returns the URL to probe and a short human-readable target label for
// the UI. Both are "" when nothing can be derived (no domain and no port). The
// probe runs on the server, so loopback is reachable even when the port is bound
// to 127.0.0.1 only.
func probeURL(server Server, dep Deployment) (url, target string) {
	if len(server.Domains) > 0 {
		host := server.Domains[0].Host
		return "https://" + host, host
	}
	if dep.Port > 0 {
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(dep.Port))
		return "http://" + addr, addr
	}
	return "", ""
}

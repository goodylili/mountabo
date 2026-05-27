package usecase

import (
	"context"
	"fmt"
)

// ServerMetrics is a point-in-time read of a server's host health, gathered over
// SSH. Zero fields mean "unavailable" (e.g. a command missing on the box).
type ServerMetrics struct {
	CPUCores      int     `json:"cpuCores"`
	Load1         float64 `json:"load1"` // 1-minute load average
	MemUsedMB     int     `json:"memUsedMB"`
	MemTotalMB    int     `json:"memTotalMB"`
	DiskUsedGB    int     `json:"diskUsedGB"`
	DiskTotalGB   int     `json:"diskTotalGB"`
	UptimeSeconds int     `json:"uptimeSeconds"`
}

// MetricsInspector reads a server's host metrics over SSH.
type MetricsInspector interface {
	Metrics(ctx context.Context, t SSHTarget) (ServerMetrics, error)
}

// ServerMetricsService reports a set-up server's live host health, connecting as
// the mountabo user with its stored key. It only reads.
type ServerMetricsService struct {
	servers   ServerStore
	vault     SecretVault
	inspector MetricsInspector
}

// NewServerMetricsService wires the service to its ports.
func NewServerMetricsService(servers ServerStore, vault SecretVault, inspector MetricsInspector) *ServerMetricsService {
	return &ServerMetricsService{servers: servers, vault: vault, inspector: inspector}
}

// Metrics returns the server's current host metrics. The server must be set up
// (the mountabo key exists). ErrServerNotFound propagates from the store.
func (s *ServerMetricsService) Metrics(ctx context.Context, id string) (ServerMetrics, error) {
	server, err := s.servers.Get(id)
	if err != nil {
		return ServerMetrics{}, err
	}
	if server.Status != StatusReady {
		return ServerMetrics{}, fmt.Errorf("server must be set up before reading metrics")
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return ServerMetrics{}, fmt.Errorf("load server key: %w", err)
	}

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	metrics, err := s.inspector.Metrics(ctx, target)
	if err != nil {
		return ServerMetrics{}, fmt.Errorf("read metrics: %w", err)
	}
	return metrics, nil
}

package usecase

import (
	"context"
	"fmt"
)

// logsDefaultTail is the number of log lines returned when the caller asks for
// none; logsMaxTail caps how many they may ask for, so one request can never
// pull an unbounded log over SSH.
const (
	logsDefaultTail = 200
	logsMaxTail     = 1000
)

// LogInspector reads the deployed app's recent container logs over SSH. tail is
// the (already bounded) number of trailing lines to return.
type LogInspector interface {
	Logs(ctx context.Context, t SSHTarget, tail int) ([]string, error)
}

// ServerLogsService reports a set-up server's deployed app logs, connecting as
// the mountabo user with its stored key. It only reads, mirroring the metrics
// service.
type ServerLogsService struct {
	servers   ServerStore
	vault     SecretVault
	inspector LogInspector
}

// NewServerLogsService wires the service to its ports.
func NewServerLogsService(servers ServerStore, vault SecretVault, inspector LogInspector) *ServerLogsService {
	return &ServerLogsService{servers: servers, vault: vault, inspector: inspector}
}

// Logs returns the server's deployed app logs, the most recent tail lines. The
// server must be set up (the mountabo key exists). tail is clamped to a sensible
// default when zero or negative and capped at logsMaxTail. ErrServerNotFound
// propagates from the store.
func (s *ServerLogsService) Logs(ctx context.Context, id string, tail int) ([]string, error) {
	if tail <= 0 {
		tail = logsDefaultTail
	}
	if tail > logsMaxTail {
		tail = logsMaxTail
	}

	server, err := s.servers.Get(id)
	if err != nil {
		return nil, err
	}
	if server.Status != StatusReady {
		return nil, fmt.Errorf("server must be set up before reading logs")
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return nil, fmt.Errorf("load server key: %w", err)
	}

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	lines, err := s.inspector.Logs(ctx, target, tail)
	if err != nil {
		return nil, fmt.Errorf("read logs: %w", err)
	}
	return lines, nil
}

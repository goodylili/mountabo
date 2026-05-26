package usecase

import (
	"context"
	"fmt"
)

// PortInspector lists the ports currently in a listening state on a server,
// connecting over SSH. It only reads; it never changes anything.
type PortInspector interface {
	ListeningPorts(ctx context.Context, t SSHTarget) ([]int, error)
}

// ServerPortService reports which ports are already occupied on a set-up
// server, so the UI can flag (never auto-fix) a deploy port that would collide.
type ServerPortService struct {
	servers   ServerStore
	vault     SecretVault
	inspector PortInspector
}

// NewServerPortService wires the service to its ports.
func NewServerPortService(servers ServerStore, vault SecretVault, inspector PortInspector) *ServerPortService {
	return &ServerPortService{servers: servers, vault: vault, inspector: inspector}
}

// Listening returns the ports already bound on the server, connecting as the
// mountabo user with its stored key. The server must be set up (the key
// exists). ErrServerNotFound propagates from the store.
func (s *ServerPortService) Listening(ctx context.Context, id string) ([]int, error) {
	server, err := s.servers.Get(id)
	if err != nil {
		return nil, err
	}
	if server.Status != StatusReady {
		return nil, fmt.Errorf("server must be set up before checking ports")
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return nil, fmt.Errorf("load server key: %w", err)
	}

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	ports, err := s.inspector.ListeningPorts(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("inspect ports: %w", err)
	}
	return ports, nil
}

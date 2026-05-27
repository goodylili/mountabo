package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// execTimeout bounds how long a single command may run before its session is
// cancelled, so a hung or interactive command cannot tie up the request
// forever. execMaxOutputBytes caps the captured combined output, so a chatty
// command (or a deliberate `yes`) cannot pull an unbounded stream back over the
// HTTP response.
const (
	execTimeout        = 60 * time.Second
	execMaxOutputBytes = 256 << 10 // 256 KiB
)

// CommandRunner runs a single shell command on a server over SSH and returns its
// combined stdout/stderr (bounded to maxBytes) and the command's exit code. A
// non-zero exit is returned as a normal result with a nil error; err is reserved
// for the connection or session itself failing.
type CommandRunner interface {
	Exec(ctx context.Context, t SSHTarget, command string, maxBytes int) (output string, exitCode int, err error)
}

// ExecResult is the outcome of running one command on a server.
type ExecResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
	// Truncated is true when the command produced more output than the captured
	// cap, so the UI can say the rest was dropped.
	Truncated bool `json:"truncated"`
}

// ServerExecService runs an operator-supplied command on a set-up server,
// connecting as the mountabo user with its stored key. It mirrors the read-only
// metrics/logs services in shape, but this one runs whatever the operator typed:
// it is the human, on their own authorised infrastructure, who is responsible
// for the command. mountabo never originates or auto-runs a command here.
type ServerExecService struct {
	servers ServerStore
	vault   SecretVault
	runner  CommandRunner
}

// NewServerExecService wires the service to its ports.
func NewServerExecService(servers ServerStore, vault SecretVault, runner CommandRunner) *ServerExecService {
	return &ServerExecService{servers: servers, vault: vault, runner: runner}
}

// Exec runs command on the server as the mountabo user and returns its combined
// output and exit code. The server must be set up (the mountabo key exists). The
// command is run under a bounded timeout and its output is capped.
// ErrServerNotFound propagates from the store.
func (s *ServerExecService) Exec(ctx context.Context, id, command string) (ExecResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return ExecResult{}, fmt.Errorf("command is required")
	}

	server, err := s.servers.Get(id)
	if err != nil {
		return ExecResult{}, err
	}
	if server.Status != StatusReady {
		return ExecResult{}, fmt.Errorf("server must be set up before running commands")
	}

	key, err := s.vault.LoadSecret(privateKeyKey(id))
	if err != nil {
		return ExecResult{}, fmt.Errorf("load server key: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	target := SSHTarget{Host: server.IP, Port: server.SSHPort, User: BootstrapUser, PrivateKey: key, Fingerprint: server.Fingerprint}
	output, exitCode, err := s.runner.Exec(ctx, target, command, execMaxOutputBytes)
	if err != nil {
		return ExecResult{}, fmt.Errorf("run command: %w", err)
	}
	return ExecResult{
		Output:    output,
		ExitCode:  exitCode,
		Truncated: len(output) >= execMaxOutputBytes,
	}, nil
}

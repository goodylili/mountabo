package usecase

import (
	"context"
	"fmt"
	"strconv"
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
	// Cwd is the working directory the shell ended in after the command ran, so
	// the next request can resume from it (a fresh SSH session would otherwise
	// reset to the user's home each command, making `cd` look broken).
	Cwd string `json:"cwd"`
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

// execEndMarker is the unique sentinel the wrapped command appends after it
// finishes, carrying the resulting working directory and the inner exit code.
// It is unlikely to collide with command output, but the parser only honours it
// as the very last line so the chance of a stray match is minimal.
const execEndMarker = "__MOUNTABO_END__"

// Exec runs command on the server as the mountabo user and returns its combined
// output, exit code, and the working directory the shell ended in. cwd is the
// directory the command starts in: passing the previous response's Cwd makes
// `cd` (and resolving relative paths) feel persistent across separate calls,
// even though each call opens a new SSH session. An empty cwd starts in the
// user's home. The server must be set up (the mountabo key exists). The
// command is run under a bounded timeout and its output is capped.
func (s *ServerExecService) Exec(ctx context.Context, id, command, cwd string) (ExecResult, error) {
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
	output, _, err := s.runner.Exec(ctx, target, wrapCommand(command, cwd), execMaxOutputBytes)
	if err != nil {
		return ExecResult{}, fmt.Errorf("run command: %w", err)
	}

	cleanOutput, newCwd, exitCode := parseExecResult(output, cwd)
	return ExecResult{
		Output:    cleanOutput,
		ExitCode:  exitCode,
		Truncated: len(output) >= execMaxOutputBytes,
		Cwd:       newCwd,
	}, nil
}

// wrapCommand decorates command so the shell first restores the previous working
// directory (falling back to ~ when cwd is empty or no longer exists, so a `cd`
// to a deleted directory cannot strand the session) and then appends a single
// sentinel line carrying the resulting pwd and the inner exit code. The wrapper
// itself exits 0 so the SSH session does not report a non-zero exit when the
// inner command fails: the parser reads the real code from the sentinel.
func wrapCommand(command, cwd string) string {
	cdLine := "cd ~"
	if cwd != "" {
		cdLine = fmt.Sprintf("cd %s 2>/dev/null || cd ~", shellSingleQuote(cwd))
	}
	return fmt.Sprintf(
		"%s\n%s\n__mtb_e=$?\nprintf '\\n%s:%%s:%%s\\n' \"$(pwd)\" \"$__mtb_e\"\nexit 0",
		cdLine, command, execEndMarker,
	)
}

// parseExecResult pulls the trailing sentinel line out of output, returning the
// cleaned output, the new working directory, and the inner exit code. When the
// sentinel is missing (a truncated read, or a shell that died before printing
// it), the original output is returned with the previous cwd preserved and a
// generic non-zero exit, so the operator still sees something useful.
func parseExecResult(output, prevCwd string) (string, string, int) {
	trimmed := strings.TrimRight(output, "\n")
	nl := strings.LastIndexByte(trimmed, '\n')
	var last string
	if nl < 0 {
		last = trimmed
	} else {
		last = trimmed[nl+1:]
	}
	if !strings.HasPrefix(last, execEndMarker+":") {
		return output, prevCwd, 1
	}
	rest := last[len(execEndMarker)+1:]
	colon := strings.LastIndexByte(rest, ':')
	if colon < 0 {
		return output, prevCwd, 1
	}
	newCwd := rest[:colon]
	code := 1
	if n, err := strconv.Atoi(rest[colon+1:]); err == nil {
		code = n
	}
	clean := ""
	if nl > 0 {
		clean = strings.TrimRight(trimmed[:nl], "\n")
	}
	return clean, newCwd, code
}

// shellSingleQuote wraps s in single quotes for safe use in a /bin/sh command,
// escaping any embedded single quotes. Kept here so the usecase does not depend
// on the ssh package for a tiny helper.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

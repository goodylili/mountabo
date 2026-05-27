package ssh

import (
	"context"
	"errors"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	"golang.org/x/crypto/ssh"
)

var _ usecase.CommandRunner = (*Client)(nil)

// Exec connects as the target user, runs a single command, and returns its
// combined stdout/stderr (bounded to maxBytes) along with the command's exit
// code. A non-zero exit is a normal result here, not a Go error: the caller (an
// interactive terminal) wants to see the output and status of a command that
// failed, exactly as a shell would. A Go error is reserved for the connection or
// session itself failing. The session is closed if ctx is cancelled (e.g. the
// command runs past the request timeout).
func (c *Client) Exec(ctx context.Context, t usecase.SSHTarget, command string, maxBytes int) (output string, exitCode int, err error) {
	client, _, err := c.dial(ctx, t)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", 0, fmt.Errorf("new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// CombinedOutput buffers everything in memory; a capWriter is wired as the
	// session's writer instead so the captured output can never exceed maxBytes
	// regardless of how chatty the command is.
	cw := &capWriter{limit: maxBytes}
	session.Stdout = cw
	session.Stderr = cw

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close()
		case <-done:
		}
	}()

	runErr := session.Run(command)
	out := cw.String()

	if runErr == nil {
		return out, 0, nil
	}
	// A command that exited non-zero is reported through ExitError, not as a
	// failure of Exec: return its code with the output collected so far.
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		return out, exitErr.ExitStatus(), nil
	}
	// Anything else (ctx cancelled mid-run, transport dropped) is a real error.
	if ctx.Err() != nil {
		return out, 0, fmt.Errorf("run command: %w", ctx.Err())
	}
	return out, 0, fmt.Errorf("run command: %w", runErr)
}

// capWriter accumulates output up to limit bytes and silently drops the rest, so
// a runaway command cannot exhaust memory. It never errors, so the SSH session
// keeps draining the remote stream to completion.
type capWriter struct {
	buf   []byte
	limit int
}

func (c *capWriter) Write(p []byte) (int, error) {
	if room := c.limit - len(c.buf); room > 0 {
		if len(p) <= room {
			c.buf = append(c.buf, p...)
		} else {
			c.buf = append(c.buf, p[:room]...)
		}
	}
	// Report the full length written so the SSH stream is fully consumed.
	return len(p), nil
}

func (c *capWriter) String() string { return string(c.buf) }

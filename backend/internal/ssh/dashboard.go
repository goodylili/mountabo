package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
	"golang.org/x/crypto/ssh"
)

var _ usecase.DashboardTunnel = (*Client)(nil)

// keepaliveInterval is how often the forwarder pings the server over the SSH
// connection. A long-lived dashboard tunnel sits idle between requests, so
// without this the server's sshd idle timeout (or a NAT in the middle) drops
// the connection and the dashboard stops loading. ~30s is well inside the
// default ClientAliveInterval window.
const keepaliveInterval = 30 * time.Second

// OpenTunnel sets up an SSH local port-forward (the `ssh -L` model): it binds a
// listener to an ephemeral 127.0.0.1 port on this machine and, for every
// connection it accepts, dials 127.0.0.1:port on the server through the
// established SSH connection and copies bytes both directions until either side
// closes. The listener stays on loopback, so the forward is never exposed off
// this machine; the SSH client is kept alive for the listener's lifetime and
// closed by the returned closer. Because it forwards raw TCP, HTTP and
// websockets ride through transparently and the tool is served at the root of
// the local port.
func (c *Client) OpenTunnel(ctx context.Context, t usecase.SSHTarget, port int) (string, func() error, error) {
	client, _, err := c.dial(ctx, t)
	if err != nil {
		return "", nil, err
	}

	// Loopback only: the port is ephemeral (:0) and never bound to a routable
	// interface, so nothing off this machine can reach the forward.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return "", nil, fmt.Errorf("open local tunnel listener: %w", err)
	}

	remoteAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	fwd := &forwarder{
		listener: listener,
		client:   client,
		remote:   remoteAddr,
		done:     make(chan struct{}),
	}
	go fwd.serve()
	fwd.wg.Add(1)
	go fwd.keepalive()

	return listener.Addr().String(), fwd.close, nil
}

// forwarder accepts connections on a loopback listener and forwards each through
// the SSH client to a fixed remote loopback address. It owns the listener and the
// SSH client for its lifetime; close stops accepting and tears both down.
type forwarder struct {
	listener net.Listener
	client   *ssh.Client
	remote   string

	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// serve accepts until the listener is closed, handling each connection in its own
// goroutine tracked by the wait group so close can drain them.
func (f *forwarder) serve() {
	for {
		local, err := f.listener.Accept()
		if err != nil {
			return // listener closed by close(), or a fatal accept error
		}
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.handle(local)
		}()
	}
}

// keepalive pings the server over the SSH connection on a fixed interval so the
// long-lived tunnel is not torn down while it sits idle between dashboard
// requests (an sshd idle timeout or an intermediate NAT would otherwise drop
// it). It exits when the forwarder is closed; a failed request means the
// connection is already gone, so it stops too. It uses a reusable ticker rather
// than time.After in the loop so no timer accumulates.
func (f *forwarder) keepalive() {
	defer f.wg.Done()
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-f.done:
			return
		case <-ticker.C:
			// A global keepalive@openssh.com request; wantReply true so the call
			// blocks until the server answers, which is what proves the link is
			// alive. A non-nil error means the connection is dead.
			if _, _, err := f.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// handle dials the remote loopback address through the SSH connection and pipes
// bytes both ways. Each direction runs in its own goroutine; when one finishes
// (its source closed), the peer connection is closed to unblock the other copy,
// then both are awaited. It does NOT return on the first copy alone: a websocket
// or other long-lived stream often has one half quiet for a long time, and
// tearing the pair down on the first io.Copy would kill it. If the SSH dial
// fails (the tunnel is going away or the tool is not listening), the local
// connection is dropped. A forwarder close also unblocks both copies.
func (f *forwarder) handle(local net.Conn) {
	defer func() { _ = local.Close() }()

	remote, err := f.client.Dial("tcp", f.remote)
	if err != nil {
		return
	}
	defer func() { _ = remote.Close() }()

	// Closing the peer conn is what makes a blocked io.Copy on it return, so each
	// direction unblocks the other when its source ends. once guards against a
	// double close racing the f.done watcher.
	var closeOnce sync.Once
	unblock := func() {
		closeOnce.Do(func() {
			_ = local.Close()
			_ = remote.Close()
		})
	}

	// A forwarder close must also tear this pair down even if both directions are
	// idle; this watcher exits when the pair finishes on its own.
	pairDone := make(chan struct{})
	go func() {
		select {
		case <-f.done:
			unblock()
		case <-pairDone:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, local)
		unblock() // local closed its write half; drop remote so its read returns
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(local, remote)
		unblock() // remote closed; drop local so its read returns
	}()
	wg.Wait()
	close(pairDone)
}

// close stops accepting new connections, waits for in-flight forwards to drain,
// and closes the SSH client. It is idempotent.
func (f *forwarder) close() error {
	var err error
	f.closeOnce.Do(func() {
		close(f.done)
		err = f.listener.Close()
		f.wg.Wait()
		if cerr := f.client.Close(); cerr != nil && err == nil {
			err = cerr
		}
	})
	return err
}

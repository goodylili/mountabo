package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/goodylili/mountabo/internal/usecase"
	"golang.org/x/crypto/ssh"
)

var _ usecase.DashboardTunnel = (*Client)(nil)

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

// handle dials the remote loopback address through the SSH connection and pipes
// bytes both ways until either side closes. If the SSH dial fails (the tunnel is
// going away or the tool is not listening), the local connection is dropped.
func (f *forwarder) handle(local net.Conn) {
	defer func() { _ = local.Close() }()

	remote, err := f.client.Dial("tcp", f.remote)
	if err != nil {
		return
	}
	defer func() { _ = remote.Close() }()

	// Copy in both directions; the first side to close ends the pair.
	pipeDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remote, local)
		pipeDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(local, remote)
		pipeDone <- struct{}{}
	}()

	select {
	case <-pipeDone:
	case <-f.done:
	}
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

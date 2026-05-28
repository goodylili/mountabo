package ssh

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.DashboardTunnel = (*Client)(nil)

// keepaliveInterval is how often the dashboard proxy pings the server over the
// SSH connection. The tunnel sits idle between page loads, so without this the
// server's sshd idle timeout (or a NAT in the middle) drops the link and the
// dashboard stops loading. ~30s is well inside the default ClientAliveInterval
// window.
const keepaliveInterval = 30 * time.Second

// shutdownTimeout bounds the in-process HTTP shutdown so close never hangs.
const shutdownTimeout = 2 * time.Second

// OpenTunnel exposes a server-side loopback dashboard (Uptime Kuma) at a local
// HTTP URL the browser can iframe. It binds an HTTP reverse proxy to an
// ephemeral 127.0.0.1 port on this machine; every request to that listener is
// dialed to 127.0.0.1:port on the server through the established SSH connection
// (so the dashboard is never exposed off the server) and the response is
// streamed back. Two response headers are stripped on the way back:
// X-Frame-Options and Content-Security-Policy. Uptime Kuma defaults to
// X-Frame-Options: SAMEORIGIN, which would block any iframe from a different
// origin; removing it lets the deployment card embed the dashboard inline
// instead of forcing a new tab. httputil.ReverseProxy carries HTTP/1.1
// Connection: Upgrade through to the transport, so the dashboard's websockets
// (socket.io) ride through transparently.
func (c *Client) OpenTunnel(ctx context.Context, t usecase.SSHTarget, port int) (string, func() error, error) {
	client, _, err := c.dial(ctx, t)
	if err != nil {
		return "", nil, err
	}

	// Loopback only: ephemeral port, never bound to a routable interface, so
	// nothing off this machine can reach the proxy.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return "", nil, fmt.Errorf("open local tunnel listener: %w", err)
	}

	remoteAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	upstream := &url.URL{Scheme: "http", Host: remoteAddr}
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.Transport = &http.Transport{
		// Dial through the SSH connection. The reverse-proxy ignores the
		// network/addr arguments here because Director already rewrote Host to
		// the upstream; we always dial the same remote loopback address.
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return client.Dial("tcp", remoteAddr)
		},
	}
	rp.ModifyResponse = func(resp *http.Response) error {
		// Strip framing-protection headers so the dashboard embeds in an iframe.
		// CSP also blocks framing via frame-ancestors, hence both removed.
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("Content-Security-Policy-Report-Only")
		return nil
	}

	srv := &http.Server{
		Handler:           rp,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Keep the SSH connection alive between dashboard requests so an idle
	// tunnel doesn't die.
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(listener)
	}()

	var once sync.Once
	closer := func() error {
		var cerr error
		once.Do(func() {
			close(done)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			cerr = srv.Shutdown(shutdownCtx)
			if err := client.Close(); err != nil && cerr == nil {
				cerr = err
			}
			wg.Wait()
		})
		return cerr
	}

	return listener.Addr().String(), closer, nil
}

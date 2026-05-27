package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"strconv"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.DashboardTunnel = (*Client)(nil)

// Proxy dials 127.0.0.1:port on the server through the established SSH
// connection and speaks HTTP to the loopback-bound monitoring tool there,
// returning its response. The SSH client's Dial gives an in-tunnel net.Conn, so
// an http.Transport that dials through it reaches the tool without the tool ever
// being exposed off the box. Only the request the caller built is sent; nothing
// runs on the server.
func (c *Client) Proxy(ctx context.Context, t usecase.SSHTarget, port int, req usecase.DashboardRequest) (usecase.DashboardResponse, error) {
	client, _, err := c.dial(ctx, t)
	if err != nil {
		return usecase.DashboardResponse{}, err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	// One transport per request, dialing the fixed loopback addr through the SSH
	// connection. CloseIdleConnections + closing the ssh client happen once the
	// response body is consumed, via a wrapper around the body.
	transport := &nethttp.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return client.Dial("tcp", addr)
		},
	}

	target := "http://" + addr + req.Path
	httpReq, err := nethttp.NewRequestWithContext(ctx, req.Method, target, req.Body)
	if err != nil {
		_ = client.Close()
		return usecase.DashboardResponse{}, fmt.Errorf("build dashboard request: %w", err)
	}
	for key, vals := range req.Header {
		for _, v := range vals {
			httpReq.Header.Add(key, v)
		}
	}
	// Host header is the loopback addr the tool expects, not mountabo's.
	httpReq.Host = addr

	resp, err := transport.RoundTrip(httpReq) //nolint:bodyclose // body is closed by the dashboardBody wrapper the caller closes
	if err != nil {
		_ = client.Close()
		return usecase.DashboardResponse{}, fmt.Errorf("proxy to %s: %w", addr, err)
	}

	return usecase.DashboardResponse{
		Status: resp.StatusCode,
		Header: resp.Header,
		// Closing the body also closes the SSH connection and drops the transport's
		// idle connections, so a single dashboard request owns its whole tunnel.
		Body: &dashboardBody{body: resp.Body, transport: transport, client: client},
	}, nil
}

// dashboardBody ties the proxied response body's lifetime to the SSH tunnel it
// rode in on: closing it closes the body, the transport's idle connections, and
// the SSH client, so nothing leaks once the response is relayed.
type dashboardBody struct {
	body      io.ReadCloser
	transport *nethttp.Transport
	client    interface{ Close() error }
}

func (d *dashboardBody) Read(p []byte) (int, error) { return d.body.Read(p) }

func (d *dashboardBody) Close() error {
	_ = d.body.Close()
	d.transport.CloseIdleConnections()
	return d.client.Close()
}

package usecase

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"strings"
	"testing"
)

// fakeTunnel records the port it was asked to proxy and returns a canned reply.
type fakeTunnel struct {
	port   int
	called bool
}

func (f *fakeTunnel) Proxy(_ context.Context, _ SSHTarget, port int, _ DashboardRequest) (DashboardResponse, error) {
	f.port = port
	f.called = true
	return DashboardResponse{
		Status: nethttp.StatusOK,
		Header: nethttp.Header{},
		Body:   io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func readyServerWith(options ...string) Server {
	return Server{ID: "s1", IP: "10.0.0.1", SSHPort: 22, Status: StatusReady, Options: options}
}

func TestDashboard_ProxiesInstalledTool(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("firewall", "netdata"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY")
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, vault, tunnel)

	resp, err := svc.Proxy(context.Background(), "s1", "netdata", DashboardRequest{Method: "GET", Path: "/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if !tunnel.called || tunnel.port != 19999 {
		t.Fatalf("expected netdata port 19999 to be proxied, got called=%v port=%d", tunnel.called, tunnel.port)
	}
}

func TestDashboard_RejectsUnknownTool(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("journald-persistent"))
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, newFakeVault(), tunnel)

	// journald-persistent has no web UI, so it is not a proxyable tool.
	_, err := svc.Proxy(context.Background(), "s1", "journald-persistent", DashboardRequest{Method: "GET", Path: "/"})
	if !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("expected ErrUnknownTool, got %v", err)
	}
	if tunnel.called {
		t.Fatal("tunnel should not be dialed for an unknown tool")
	}
}

func TestDashboard_RejectsToolNotInstalled(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("firewall")) // netdata not installed
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, newFakeVault(), tunnel)

	_, err := svc.Proxy(context.Background(), "s1", "netdata", DashboardRequest{Method: "GET", Path: "/"})
	if !errors.Is(err, ErrToolNotInstalled) {
		t.Fatalf("expected ErrToolNotInstalled, got %v", err)
	}
	if tunnel.called {
		t.Fatal("tunnel should not be dialed when the tool is not installed")
	}
}

func TestDashboard_RejectsUnknownServer(t *testing.T) {
	svc := NewServerDashboardService(newMemServerStore(), newFakeVault(), &fakeTunnel{})
	_, err := svc.Proxy(context.Background(), "missing", "netdata", DashboardRequest{Method: "GET", Path: "/"})
	if !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("expected ErrServerNotFound, got %v", err)
	}
}

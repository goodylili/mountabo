package usecase

import (
	"context"
	"errors"
	"testing"
)

// fakeTunnel records the port it was asked to forward and returns a canned local
// address, tracking whether its closer ran.
type fakeTunnel struct {
	port   int
	called bool
	closed bool
}

func (f *fakeTunnel) OpenTunnel(_ context.Context, _ SSHTarget, port int) (string, func() error, error) {
	f.port = port
	f.called = true
	return "127.0.0.1:54321", func() error {
		f.closed = true
		return nil
	}, nil
}

func readyServerWith(options ...string) Server {
	return Server{ID: "s1", IP: "10.0.0.1", SSHPort: 22, Status: StatusReady, Options: options}
}

func TestDashboard_OpensTunnelForInstalledTool(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("firewall", "uptime-kuma"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY")
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, vault, tunnel)

	url, err := svc.Open(context.Background(), "s1", "uptime-kuma")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tunnel.called || tunnel.port != 3001 {
		t.Fatalf("expected uptime-kuma port 3001 to be forwarded, got called=%v port=%d", tunnel.called, tunnel.port)
	}
	if url != "http://127.0.0.1:54321/" {
		t.Fatalf("expected local tunnel url, got %q", url)
	}
}

func TestDashboard_ReusesOpenTunnel(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("uptime-kuma"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY")
	tunnel := &countingTunnel{}
	svc := NewServerDashboardService(store, vault, tunnel)

	if _, err := svc.Open(context.Background(), "s1", "uptime-kuma"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := svc.Open(context.Background(), "s1", "uptime-kuma"); err != nil {
		t.Fatalf("second open: %v", err)
	}
	if tunnel.opens != 1 {
		t.Fatalf("expected the open tunnel to be reused (1 open), got %d", tunnel.opens)
	}
}

func TestDashboard_CloseTearsDownTunnels(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("uptime-kuma"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY")
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, vault, tunnel)

	if _, err := svc.Open(context.Background(), "s1", "uptime-kuma"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !tunnel.closed {
		t.Fatal("expected the tunnel to be closed on Close")
	}
}

func TestDashboard_RejectsUnknownTool(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("uptime-kuma"))
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, newFakeVault(), tunnel)

	// netdata is no longer a known tool, so it is rejected before any tunnel.
	_, err := svc.Open(context.Background(), "s1", "netdata")
	if !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("expected ErrUnknownTool, got %v", err)
	}
	if tunnel.called {
		t.Fatal("tunnel should not be opened for an unknown tool")
	}
}

func TestDashboard_RejectsToolNotInstalled(t *testing.T) {
	store := newMemServerStore()
	_ = store.Save(readyServerWith("firewall")) // uptime-kuma not installed
	tunnel := &fakeTunnel{}
	svc := NewServerDashboardService(store, newFakeVault(), tunnel)

	_, err := svc.Open(context.Background(), "s1", "uptime-kuma")
	if !errors.Is(err, ErrToolNotInstalled) {
		t.Fatalf("expected ErrToolNotInstalled, got %v", err)
	}
	if tunnel.called {
		t.Fatal("tunnel should not be opened when the tool is not installed")
	}
}

func TestDashboard_RejectsUnknownServer(t *testing.T) {
	svc := NewServerDashboardService(newMemServerStore(), newFakeVault(), &fakeTunnel{})
	_, err := svc.Open(context.Background(), "missing", "uptime-kuma")
	if !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("expected ErrServerNotFound, got %v", err)
	}
}

// countingTunnel counts how many times a tunnel was opened, to verify reuse.
type countingTunnel struct{ opens int }

func (c *countingTunnel) OpenTunnel(_ context.Context, _ SSHTarget, _ int) (string, func() error, error) {
	c.opens++
	return "127.0.0.1:54321", func() error { return nil }, nil
}

package usecase

import (
	"context"
	"errors"
	"testing"
)

// fakeAppProber records the URL it was asked to probe and returns a canned result,
// so the health service's URL derivation can be asserted without SSH.
type fakeAppProber struct {
	url       string
	reachable bool
	status    int
	err       error
}

func (f *fakeAppProber) ProbeHTTP(_ context.Context, _ SSHTarget, url string) (bool, int, error) {
	f.url = url
	return f.reachable, f.status, f.err
}

func readyServer(id, ip string, domains ...Domain) Server {
	return Server{ID: id, IP: ip, SSHPort: 22, Status: StatusReady, Domains: domains}
}

func TestAppHealth_ProbesLoopbackPort(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{{App: "shop", ServerID: "s1", Port: 8080}}}
	servers := newMemServerStore()
	_ = servers.Save(readyServer("s1", "203.0.113.7"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "key")
	prober := &fakeAppProber{reachable: true, status: 200}

	svc := NewAppHealthService(deps, servers, vault, prober)
	got, err := svc.Health(context.Background(), "shop")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if prober.url != "http://127.0.0.1:8080" {
		t.Errorf("probed url = %q, want loopback port", prober.url)
	}
	if !got.Reachable || got.Status != 200 || got.Target != "127.0.0.1:8080" {
		t.Errorf("unexpected health: %+v", got)
	}
}

func TestAppHealth_PrefersDomain(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{{App: "shop", ServerID: "s1", Port: 8080}}}
	servers := newMemServerStore()
	_ = servers.Save(readyServer("s1", "203.0.113.7", Domain{Host: "shop.example.com"}))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "key")
	prober := &fakeAppProber{reachable: true, status: 200}

	svc := NewAppHealthService(deps, servers, vault, prober)
	got, err := svc.Health(context.Background(), "shop")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if prober.url != "https://shop.example.com" {
		t.Errorf("probed url = %q, want the domain over https", prober.url)
	}
	if got.Target != "shop.example.com" {
		t.Errorf("target = %q, want the domain host", got.Target)
	}
}

func TestAppHealth_DownIsNotAnError(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{{App: "shop", ServerID: "s1", Port: 8080}}}
	servers := newMemServerStore()
	_ = servers.Save(readyServer("s1", "203.0.113.7"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "key")
	prober := &fakeAppProber{reachable: false, status: 0}

	svc := NewAppHealthService(deps, servers, vault, prober)
	got, err := svc.Health(context.Background(), "shop")
	if err != nil {
		t.Fatalf("Health (down app): %v", err)
	}
	if got.Reachable || got.Detail == "" {
		t.Errorf("want unreachable with a detail, got %+v", got)
	}
}

func TestAppHealth_NoPortNoDomain(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{{App: "shop", ServerID: "s1"}}}
	servers := newMemServerStore()
	_ = servers.Save(readyServer("s1", "203.0.113.7"))
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "key")
	prober := &fakeAppProber{}

	svc := NewAppHealthService(deps, servers, vault, prober)
	got, err := svc.Health(context.Background(), "shop")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Reachable || prober.url != "" {
		t.Errorf("want no probe and unreachable, got %+v (probed %q)", got, prober.url)
	}
}

func TestAppHealth_UnknownApp(t *testing.T) {
	svc := NewAppHealthService(&fakeDeploymentStore{}, newMemServerStore(), newFakeVault(), &fakeAppProber{})
	if _, err := svc.Health(context.Background(), "nope"); !errors.Is(err, ErrDeploymentNotFound) {
		t.Fatalf("want ErrDeploymentNotFound, got %v", err)
	}
}

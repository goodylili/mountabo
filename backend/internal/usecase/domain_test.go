package usecase

import (
	"context"
	"io"
	"strings"
	"testing"
)

func readyServerSvc(t *testing.T) (*ServerService, *fakeRunner, *memServerStore) {
	t.Helper()
	store, vault := newMemServerStore(), newFakeVault()
	runner := &fakeRunner{}
	svc := NewServerService(store, fakeProber{}, fakeBootstrapper{}, &fakeApplier{}, runner, fakeKeyMaker{}, fakeLocalKeyProvider{}, vault)
	_ = store.Save(Server{ID: "s1", IP: "1.2.3.4", SSHPort: 22, Status: StatusReady})
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	return svc, runner, store
}

func TestAddDomain_RunsScriptAndPersists(t *testing.T) {
	svc, runner, store := readyServerSvc(t)

	in := DomainInput{Host: "App.Example.com", Aliases: []string{"www.example.com", "App.Example.com", ""}, Upstream: "3000"}
	if err := svc.AddDomain(context.Background(), "s1", in, io.Discard); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	// The rendered script targets the lower-cased host and proxies to the port.
	for _, want := range []string{"app.example.com", "certbot certonly", "127.0.0.1:3000"} {
		if !strings.Contains(runner.script, want) {
			t.Errorf("script missing %q", want)
		}
	}
	got, _ := store.Get("s1")
	if len(got.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(got.Domains))
	}
	d := got.Domains[0]
	if d.Host != "app.example.com" {
		t.Errorf("host = %q, want lower-cased", d.Host)
	}
	// The self-alias and the blank are dropped; only www survives.
	if len(d.Aliases) != 1 || d.Aliases[0] != "www.example.com" {
		t.Errorf("aliases = %v, want [www.example.com]", d.Aliases)
	}
}

func TestAddDomain_UpsertsByHost(t *testing.T) {
	svc, _, store := readyServerSvc(t)
	ctx := context.Background()

	if err := svc.AddDomain(ctx, "s1", DomainInput{Host: "example.com", Upstream: "3000"}, io.Discard); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := svc.AddDomain(ctx, "s1", DomainInput{Host: "example.com", Upstream: "8080"}, io.Discard); err != nil {
		t.Fatalf("second add: %v", err)
	}
	got, _ := store.Get("s1")
	if len(got.Domains) != 1 {
		t.Fatalf("re-adding the same host should update in place, got %d domains", len(got.Domains))
	}
	if got.Domains[0].Upstream != "8080" {
		t.Errorf("upstream = %q, want updated 8080", got.Domains[0].Upstream)
	}
}

func TestAddDomain_RejectsInvalidHost(t *testing.T) {
	svc, _, _ := readyServerSvc(t)
	if err := svc.AddDomain(context.Background(), "s1", DomainInput{Host: "not a domain"}, io.Discard); err == nil {
		t.Fatal("expected an invalid-domain error")
	}
}

func TestAddDomain_RequiresReady(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := NewServerService(store, fakeProber{}, fakeBootstrapper{}, &fakeApplier{}, &fakeRunner{}, fakeKeyMaker{}, fakeLocalKeyProvider{}, vault)
	_ = store.Save(Server{ID: "s1", Status: StatusProbed})
	if err := svc.AddDomain(context.Background(), "s1", DomainInput{Host: "example.com"}, io.Discard); err == nil {
		t.Fatal("expected error adding a domain to a non-ready server")
	}
}

func TestRemoveDomain_RunsTeardownAndDrops(t *testing.T) {
	svc, runner, store := readyServerSvc(t)
	ctx := context.Background()
	_ = svc.AddDomain(ctx, "s1", DomainInput{Host: "a.example.com", Upstream: "3000"}, io.Discard)
	_ = svc.AddDomain(ctx, "s1", DomainInput{Host: "b.example.com", Upstream: "3001"}, io.Discard)

	if err := svc.RemoveDomain(ctx, "s1", "A.example.com", io.Discard); err != nil {
		t.Fatalf("RemoveDomain: %v", err)
	}
	if !strings.Contains(runner.script, "certbot delete") {
		t.Errorf("teardown script should delete the cert, got %q", runner.script)
	}
	got, _ := store.Get("s1")
	if len(got.Domains) != 1 || got.Domains[0].Host != "b.example.com" {
		t.Errorf("domains = %v, want only b.example.com", got.Domains)
	}
}

func TestPreviewDomain_RendersWithoutSideEffects(t *testing.T) {
	svc, runner, store := readyServerSvc(t)
	art, err := svc.PreviewDomain(DomainInput{Host: "example.com", Upstream: "3000"})
	if err != nil {
		t.Fatalf("PreviewDomain: %v", err)
	}
	if !strings.Contains(art.TLSConfig, "proxy_pass http://127.0.0.1:3000;") {
		t.Error("preview TLS config missing proxy_pass")
	}
	if art.SitePath != "/etc/nginx/sites-available/example.com.conf" {
		t.Errorf("site path = %q", art.SitePath)
	}
	if runner.script != "" {
		t.Error("preview must not run anything")
	}
	if got, _ := store.Get("s1"); len(got.Domains) != 0 {
		t.Error("preview must not persist a domain")
	}
}

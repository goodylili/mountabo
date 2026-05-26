package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeInspector struct {
	ports  []int
	err    error
	target SSHTarget
}

func (f *fakeInspector) ListeningPorts(_ context.Context, t SSHTarget) ([]int, error) {
	f.target = t
	return f.ports, f.err
}

func TestServerPortService_ListsViaMountaboKey(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", IP: "5.6.7.8", SSHPort: 22, Status: StatusReady, Fingerprint: "fp"})
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	insp := &fakeInspector{ports: []int{22, 80, 443}}
	svc := NewServerPortService(store, vault, insp)

	got, err := svc.Listening(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Listening: %v", err)
	}
	if len(got) != 3 || got[1] != 80 {
		t.Errorf("ports = %v, want [22 80 443]", got)
	}
	// Connects as the mountabo user with the stored key and pins the host key.
	if insp.target.User != BootstrapUser || insp.target.PrivateKey != "KEY-PEM" || insp.target.Fingerprint != "fp" {
		t.Errorf("unexpected target %+v", insp.target)
	}
}

func TestServerPortService_RequiresReadyServer(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", Status: StatusProbed})
	svc := NewServerPortService(store, vault, &fakeInspector{})

	if _, err := svc.Listening(context.Background(), "s1"); err == nil {
		t.Fatal("expected an error for a server that is not ready")
	}
}

func TestServerPortService_NotFound(t *testing.T) {
	svc := NewServerPortService(newMemServerStore(), newFakeVault(), &fakeInspector{})
	if _, err := svc.Listening(context.Background(), "missing"); !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("want ErrServerNotFound, got %v", err)
	}
}

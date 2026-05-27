package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeLogInspector struct {
	lines []string
	tail  int
}

func (f *fakeLogInspector) Logs(_ context.Context, _ SSHTarget, tail int) ([]string, error) {
	f.tail = tail
	return f.lines, nil
}

func TestServerLogsService_DefaultsAndCapsTail(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", IP: "5.6.7.8", SSHPort: 22, Status: StatusReady, Fingerprint: "fp"})
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	insp := &fakeLogInspector{lines: []string{"hello"}}
	svc := NewServerLogsService(store, vault, insp)

	if _, err := svc.Logs(context.Background(), "s1", 0); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if insp.tail != logsDefaultTail {
		t.Errorf("tail = %d, want default %d", insp.tail, logsDefaultTail)
	}

	if _, err := svc.Logs(context.Background(), "s1", 999999); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if insp.tail != logsMaxTail {
		t.Errorf("tail = %d, want capped %d", insp.tail, logsMaxTail)
	}
}

func TestServerLogsService_RequiresReadyServer(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", Status: StatusProbed})
	svc := NewServerLogsService(store, vault, &fakeLogInspector{})

	if _, err := svc.Logs(context.Background(), "s1", 100); err == nil {
		t.Fatal("expected an error for a server that is not ready")
	}
}

func TestServerLogsService_NotFound(t *testing.T) {
	svc := NewServerLogsService(newMemServerStore(), newFakeVault(), &fakeLogInspector{})
	if _, err := svc.Logs(context.Background(), "missing", 100); !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("want ErrServerNotFound, got %v", err)
	}
}

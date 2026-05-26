package usecase

import (
	"context"
	"errors"
	"io"
	"testing"
)

type fakeProber struct {
	specs ServerSpecs
	fp    string
	err   error
}

func (f fakeProber) Probe(context.Context, SSHTarget) (ServerSpecs, string, error) {
	return f.specs, f.fp, f.err
}

type fakeBootstrapper struct{ err error }

func (f fakeBootstrapper) Bootstrap(_ context.Context, _ SSHTarget, _ BootstrapParams, out io.Writer) error {
	_, _ = io.WriteString(out, "==> bootstrapping\n")
	return f.err
}

type fakeApplier struct {
	add, remove []string
	err         error
}

func (f *fakeApplier) ApplyOptions(_ context.Context, _ SSHTarget, add, remove []string, out io.Writer) error {
	f.add, f.remove = add, remove
	_, _ = io.WriteString(out, "==> applying options\n")
	return f.err
}

type fakeKeyMaker struct{}

func (fakeKeyMaker) Generate(string) (string, string, error) {
	return "PRIVATE-KEY-PEM", "ssh-ed25519 AAAA mountabo", nil
}

type fakeLocalKeyProvider struct{ key string }

func (f fakeLocalKeyProvider) LocalPublicKey() (string, error) { return f.key, nil }

type fakeVault struct{ secrets map[string]string }

func newFakeVault() *fakeVault { return &fakeVault{secrets: map[string]string{}} }
func (v *fakeVault) SaveSecret(k, val string) error {
	v.secrets[k] = val
	return nil
}
func (v *fakeVault) LoadSecret(k string) (string, error) { return v.secrets[k], nil }
func (v *fakeVault) DeleteSecret(k string) error {
	delete(v.secrets, k)
	return nil
}

type memServerStore struct{ servers map[string]Server }

func newMemServerStore() *memServerStore { return &memServerStore{servers: map[string]Server{}} }
func (m *memServerStore) List() ([]Server, error) {
	out := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		out = append(out, s)
	}
	return out, nil
}
func (m *memServerStore) Get(id string) (Server, error) {
	s, ok := m.servers[id]
	if !ok {
		return Server{}, ErrServerNotFound
	}
	return s, nil
}
func (m *memServerStore) Save(s Server) error { m.servers[s.ID] = s; return nil }
func (m *memServerStore) Delete(id string) error {
	delete(m.servers, id)
	return nil
}

func newService(store ServerStore, vault SecretVault, boot ServerBootstrapper) *ServerService {
	return NewServerService(store, fakeProber{specs: ServerSpecs{CPUCores: 4}}, boot, &fakeApplier{}, fakeKeyMaker{}, fakeLocalKeyProvider{}, vault)
}

func TestApplyOptions_DiffsPersistsAndOrders(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	applier := &fakeApplier{}
	svc := NewServerService(store, fakeProber{}, fakeBootstrapper{}, applier, fakeKeyMaker{}, fakeLocalKeyProvider{}, vault)
	_ = store.Save(Server{ID: "s1", IP: "1.2.3.4", SSHPort: 22, Status: StatusReady, Options: []string{"firewall"}})
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")

	if err := svc.ApplyOptions(context.Background(), "s1", []string{"harden-ssh", "fail2ban"}, io.Discard); err != nil {
		t.Fatalf("ApplyOptions: %v", err)
	}
	// add in catalog order (fail2ban before harden-ssh); remove = firewall
	if len(applier.add) != 2 || applier.add[0] != "fail2ban" || applier.add[1] != "harden-ssh" {
		t.Errorf("add = %v, want [fail2ban harden-ssh]", applier.add)
	}
	if len(applier.remove) != 1 || applier.remove[0] != "firewall" {
		t.Errorf("remove = %v, want [firewall]", applier.remove)
	}
	got, _ := store.Get("s1")
	if len(got.Options) != 2 {
		t.Errorf("persisted options = %v, want 2 entries", got.Options)
	}
}

func TestApplyOptions_RequiresReady(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := NewServerService(store, fakeProber{}, fakeBootstrapper{}, &fakeApplier{}, fakeKeyMaker{}, fakeLocalKeyProvider{}, vault)
	_ = store.Save(Server{ID: "s1", Status: StatusProbed})
	if err := svc.ApplyOptions(context.Background(), "s1", []string{"firewall"}, io.Discard); err == nil {
		t.Fatal("expected error applying options to a non-ready server")
	}
}

func TestAdd_StoresRootPasswordInVaultNotOnServer(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := newService(store, vault, fakeBootstrapper{})

	server, err := svc.Add(context.Background(), AddServerInput{
		Name: "edge-1", IP: "1.2.3.4", Timezone: "Africa/Lagos", RootPassword: "s3cret",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if server.Status != StatusProbed {
		t.Errorf("status = %q, want probed", server.Status)
	}
	if vault.secrets[rootPasswordKey(server.ID)] != "s3cret" {
		t.Error("root password not stored in vault")
	}
}

func TestSetup_StoresKeyAndKeepsRootPasswordForRecovery(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := newService(store, vault, fakeBootstrapper{})
	server, _ := svc.Add(context.Background(), AddServerInput{
		Name: "edge-1", IP: "1.2.3.4", Timezone: "UTC", RootPassword: "s3cret",
	})

	if err := svc.Setup(context.Background(), server.ID, nil, io.Discard); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if vault.secrets[rootPasswordKey(server.ID)] != "s3cret" {
		t.Error("root password should be kept after setup for break-glass recovery")
	}
	if vault.secrets[privateKeyKey(server.ID)] != "PRIVATE-KEY-PEM" {
		t.Error("mountabo private key should be stored after setup")
	}
	got, _ := store.Get(server.ID)
	if got.Status != StatusReady {
		t.Errorf("status = %q, want ready", got.Status)
	}
}

func TestSetup_FailureKeepsPasswordForRetry(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := newService(store, vault, fakeBootstrapper{err: io.ErrClosedPipe})
	server, _ := svc.Add(context.Background(), AddServerInput{
		Name: "edge-1", IP: "1.2.3.4", Timezone: "UTC", RootPassword: "s3cret",
	})

	if err := svc.Setup(context.Background(), server.ID, nil, io.Discard); err == nil {
		t.Fatal("expected setup error")
	}
	if vault.secrets[rootPasswordKey(server.ID)] != "s3cret" {
		t.Error("root password should be kept for retry after a failed setup")
	}
	got, _ := store.Get(server.ID)
	if got.Status != StatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
}

func TestCanonicalOptions_FiltersAndOrders(t *testing.T) {
	// Unknown ids dropped, duplicates collapsed, output in catalog order
	// (harden-ssh last) regardless of request order.
	got := canonicalOptions([]string{"harden-ssh", "bogus", "firewall", "firewall"})
	want := []string{"firewall", "harden-ssh"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestRemove_DestroysAllSecrets(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	svc := newService(store, vault, fakeBootstrapper{})
	server, _ := svc.Add(context.Background(), AddServerInput{
		Name: "edge-1", IP: "1.2.3.4", Timezone: "UTC", RootPassword: "s3cret",
	})
	_ = svc.Setup(context.Background(), server.ID, nil, io.Discard)

	if err := svc.Remove(server.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(vault.secrets) != 0 {
		t.Errorf("secrets not fully destroyed: %v", vault.secrets)
	}
	if _, err := store.Get(server.ID); !errors.Is(err, ErrServerNotFound) {
		t.Errorf("server should be gone, got err=%v", err)
	}
}

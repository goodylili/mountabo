package usecase

import (
	"context"
	"io"
	"strings"
	"testing"
)

type putCall struct{ branch, path, content string }

type fakeRepoWriter struct {
	calls []putCall
	err   error
}

func (f *fakeRepoWriter) PutFile(_ context.Context, _ Token, _, _, branch, path, content, _ string) error {
	f.calls = append(f.calls, putCall{branch: branch, path: path, content: content})
	return f.err
}

type fakeEnvManager struct {
	env string
	err error
}

func (f *fakeEnvManager) EnsureEnvironment(_ context.Context, _ Token, _, _, name string) error {
	f.env = name
	return f.err
}

type fakeSecretSetter struct {
	env     string
	secrets []NamedSecret
	err     error
}

func (f *fakeSecretSetter) SetEnvSecrets(_ context.Context, _ Token, _, _, environment string, secrets []NamedSecret) error {
	f.env, f.secrets = environment, secrets
	return f.err
}

type fakeDeploymentStore struct{ saved []Deployment }

func (f *fakeDeploymentStore) List() ([]Deployment, error) { return f.saved, nil }
func (f *fakeDeploymentStore) Save(d Deployment) error {
	f.saved = append(f.saved, d)
	return nil
}

// DeleteByApp removes every saved deployment with the given app, reporting
// whether any was removed, so the store doubles as a DeploymentDeleter in tests.
func (f *fakeDeploymentStore) DeleteByApp(app string) (bool, error) {
	kept := make([]Deployment, 0, len(f.saved))
	removed := false
	for _, d := range f.saved {
		if d.App == app {
			removed = true
			continue
		}
		kept = append(kept, d)
	}
	f.saved = kept
	return removed, nil
}

type fakeDeployKeyManager struct {
	title     string
	publicKey string
	readOnly  bool
	called    bool
}

func (f *fakeDeployKeyManager) AddDeployKey(_ context.Context, _ Token, _, _, title, publicKey string, readOnly bool) (int64, error) {
	f.called, f.title, f.publicKey, f.readOnly = true, title, publicKey, readOnly
	return 1, nil
}

type fakeDeployKeyInstaller struct {
	keyName string
	key     string
}

func (f *fakeDeployKeyInstaller) InstallDeployKey(_ context.Context, _ SSHTarget, keyName, privateKey string, out io.Writer) error {
	f.keyName, f.key = keyName, privateKey
	_, _ = io.WriteString(out, "==> deploy key installed\n")
	return nil
}

type fakeTokenStore struct {
	token Token
	err   error
}

func (f fakeTokenStore) Save(Token) error     { return nil }
func (f fakeTokenStore) Load() (Token, error) { return f.token, f.err }
func (f fakeTokenStore) Delete() error        { return nil }

func readyDeployFixture(t *testing.T) (*memServerStore, *fakeVault, *fakeRepoWriter, *fakeEnvManager, *fakeSecretSetter, *DeployService) {
	t.Helper()
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", IP: "5.6.7.8", SSHPort: 22, Status: StatusReady})
	_ = vault.SaveSecret(privateKeyKey("s1"), "MOUNTABO-KEY-PEM")
	repo, envs, secrets := &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{}
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, repo, envs, secrets, &fakeDeploymentStore{}, fakeKeyMaker{}, &fakeDeployKeyManager{}, &fakeDeployKeyInstaller{})
	return store, vault, repo, envs, secrets, svc
}

func deployInput() DeployInput {
	return DeployInput{
		ServerID:  "s1",
		App:       "shop",
		Owner:     "acme",
		Repo:      "shop",
		Branch:    "main",
		DeployDir: "/opt/shop",
		Ports:     []DeployPort{{EnvVar: "FRONTEND_PORT", Value: "3000", Container: "3000"}},
		EnvVars:   []DeployEnvVar{{Key: "DATABASE_URL", Value: "postgres://secret"}},
	}
}

func TestDeploy_WritesArtifactsProvisionsEnvAndSecrets(t *testing.T) {
	_, _, repo, envs, secrets, svc := readyDeployFixture(t)

	var out strings.Builder
	if err := svc.Deploy(context.Background(), deployInput(), &out); err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	// Two commits: deploy.sh at the repo root, then the per-branch workflow.
	if len(repo.calls) != 2 {
		t.Fatalf("want 2 file writes, got %d", len(repo.calls))
	}
	if repo.calls[0].path != "deploy.sh" {
		t.Errorf("first write path = %q, want deploy.sh", repo.calls[0].path)
	}
	if repo.calls[1].path != ".github/workflows/mountabo-deploy-main.yml" {
		t.Errorf("second write path = %q", repo.calls[1].path)
	}
	for _, c := range repo.calls {
		if c.branch != "main" {
			t.Errorf("write to branch %q, want main", c.branch)
		}
	}

	// Environment defaults to the branch.
	if envs.env != "main" || secrets.env != "main" {
		t.Errorf("environment = %q/%q, want main", envs.env, secrets.env)
	}

	// Managed connection secrets + the operator's env var, with the right values.
	want := map[string]string{
		"SERVER_HOST":    "5.6.7.8",
		"SERVER_USER":    BootstrapUser,
		"SERVER_SSH_KEY": "MOUNTABO-KEY-PEM",
		"DEPLOY_DIR":     "/opt/shop",
		"DATABASE_URL":   "postgres://secret",
	}
	got := map[string]string{}
	for _, s := range secrets.secrets {
		got[s.Name] = s.Value
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("secret %s = %q, want %q", name, got[name], value)
		}
	}

	// Secret values must never reach the live output stream.
	if log := out.String(); strings.Contains(log, "MOUNTABO-KEY-PEM") || strings.Contains(log, "postgres://secret") {
		t.Errorf("secret value leaked into stream output:\n%s", log)
	}
}

func TestDeploy_EnvironmentOverridesBranch(t *testing.T) {
	_, _, _, envs, secrets, svc := readyDeployFixture(t)
	in := deployInput()
	in.Environment = "production"

	if err := svc.Deploy(context.Background(), in, new(strings.Builder)); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if envs.env != "production" || secrets.env != "production" {
		t.Errorf("environment = %q/%q, want production", envs.env, secrets.env)
	}
}

func TestDeploy_RequiresReadyServer(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", Status: StatusProbed})
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{}, &fakeDeploymentStore{}, fakeKeyMaker{}, &fakeDeployKeyManager{}, &fakeDeployKeyInstaller{})

	if err := svc.Deploy(context.Background(), deployInput(), new(strings.Builder)); err == nil {
		t.Fatal("expected an error deploying to a server that is not ready")
	}
}

func TestDeploy_RejectsInvalidEnvVarName(t *testing.T) {
	_, _, repo, _, _, svc := readyDeployFixture(t)
	in := deployInput()
	in.EnvVars = []DeployEnvVar{{Key: "9bad name", Value: "x"}}

	if err := svc.Deploy(context.Background(), in, new(strings.Builder)); err == nil {
		t.Fatal("expected an error for an invalid env var name")
	}
	if len(repo.calls) != 0 {
		t.Errorf("nothing should be written when validation fails, got %d writes", len(repo.calls))
	}
}

func TestDeploy_NotConnected(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", Status: StatusReady})
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY")
	svc := NewDeployService(store, vault, fakeTokenStore{err: ErrNotConnected}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{}, &fakeDeploymentStore{}, fakeKeyMaker{}, &fakeDeployKeyManager{}, &fakeDeployKeyInstaller{})

	err := svc.Deploy(context.Background(), deployInput(), new(strings.Builder))
	if err == nil {
		t.Fatal("expected ErrNotConnected to propagate")
	}
}

func TestPreview_GeneratesArtifactsWithoutSideEffects(t *testing.T) {
	_, _, repo, envs, secrets, svc := readyDeployFixture(t)

	art, err := svc.Preview(deployInput())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if art.WorkflowPath != ".github/workflows/mountabo-deploy-main.yml" {
		t.Errorf("workflow path = %q", art.WorkflowPath)
	}
	if !strings.Contains(art.DeployScript, "docker compose build") {
		t.Error("expected a compose deploy script by default")
	}
	if !strings.Contains(art.Workflow, "environment: main") {
		t.Error("expected the workflow to pin the environment")
	}
	names := map[string]bool{}
	for _, s := range art.Secrets {
		names[s.Name] = true
	}
	for _, n := range []string{"SERVER_HOST", "SERVER_SSH_KEY", "DEPLOY_DIR", "DATABASE_URL"} {
		if !names[n] {
			t.Errorf("preview secrets missing %s", n)
		}
	}
	// Preview is pure: nothing is committed, no environment or secrets are set.
	if len(repo.calls) != 0 || envs.env != "" || len(secrets.secrets) != 0 {
		t.Error("Preview must have no side effects")
	}
}

func TestPreview_RejectsInvalidEnvVarName(t *testing.T) {
	_, _, _, _, _, svc := readyDeployFixture(t)
	in := deployInput()
	in.EnvVars = []DeployEnvVar{{Key: "bad name", Value: "x"}}
	if _, err := svc.Preview(in); err == nil {
		t.Fatal("expected an error for an invalid env var name")
	}
}

func TestDeploy_RegistersAndInstallsDeployKey(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", IP: "5.6.7.8", SSHPort: 22, Status: StatusReady, Fingerprint: "fp"})
	_ = vault.SaveSecret(privateKeyKey("s1"), "MOUNTABO-KEY-PEM")
	mgr, inst := &fakeDeployKeyManager{}, &fakeDeployKeyInstaller{}
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{}, &fakeDeploymentStore{}, fakeKeyMaker{}, mgr, inst)

	if err := svc.Deploy(context.Background(), deployInput(), new(strings.Builder)); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	// First deploy registers a read-only key and installs the private half on
	// the server under a per-repo file name.
	if !mgr.called || !mgr.readOnly {
		t.Errorf("expected a read-only deploy key registration, got %+v", mgr)
	}
	if inst.keyName != "mountabo_deploy_acme_shop" {
		t.Errorf("install key name = %q", inst.keyName)
	}
	if inst.key != "PRIVATE-KEY-PEM" {
		t.Errorf("installed key = %q, want the generated private key", inst.key)
	}
	if vault.secrets[deployKeyVaultKey("acme", "shop")] != "PRIVATE-KEY-PEM" {
		t.Error("deploy private key not stored for reuse")
	}
}

func TestDeploy_ReusesStoredDeployKey(t *testing.T) {
	store, vault := newMemServerStore(), newFakeVault()
	_ = store.Save(Server{ID: "s1", IP: "5.6.7.8", SSHPort: 22, Status: StatusReady})
	_ = vault.SaveSecret(privateKeyKey("s1"), "MOUNTABO-KEY-PEM")
	_ = vault.SaveSecret(deployKeyVaultKey("acme", "shop"), "EXISTING-DEPLOY-KEY")
	mgr, inst := &fakeDeployKeyManager{}, &fakeDeployKeyInstaller{}
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{}, &fakeDeploymentStore{}, fakeKeyMaker{}, mgr, inst)

	if err := svc.Deploy(context.Background(), deployInput(), new(strings.Builder)); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if mgr.called {
		t.Error("should not re-register a deploy key when one is already stored")
	}
	if inst.key != "EXISTING-DEPLOY-KEY" {
		t.Errorf("installed key = %q, want the stored one", inst.key)
	}
}

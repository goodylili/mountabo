package usecase

import (
	"context"
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
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, repo, envs, secrets)
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
		Ports:     DeployPorts{Frontend: "3000", Backend: "8080"},
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
	svc := NewDeployService(store, vault, fakeTokenStore{token: Token{AccessToken: "tok"}}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{})

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
	svc := NewDeployService(store, vault, fakeTokenStore{err: ErrNotConnected}, &fakeRepoWriter{}, &fakeEnvManager{}, &fakeSecretSetter{})

	err := svc.Deploy(context.Background(), deployInput(), new(strings.Builder))
	if err == nil {
		t.Fatal("expected ErrNotConnected to propagate")
	}
}

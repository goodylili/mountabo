package usecase

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/goodylili/mountabo/internal/workflow"
)

// secretName matches a valid GitHub Actions secret name: letters, digits and
// underscores, not starting with a digit. Env var keys must satisfy this before
// their values can be stored as secrets.
var secretName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DeployPorts are the published container ports for the deployment.
type DeployPorts struct {
	Frontend string
	Backend  string
	Postgres string
	Redis    string
}

// DeployEnvVar is one application environment variable; its value is stored as a
// GitHub Actions secret and injected into the deploy at run time.
type DeployEnvVar struct {
	Key   string
	Value string
}

// DeployInput is what the operator supplies to wire continuous deployment of a
// repo branch to one of their servers. Environment names the GitHub deployment
// environment whose secrets the workflow uses; it defaults to Branch.
type DeployInput struct {
	ServerID    string
	App         string
	Owner       string
	Repo        string
	Branch      string
	Environment string
	RootDir     string
	DeployDir   string
	Ports       DeployPorts
	EnvVars     []DeployEnvVar
}

// NamedSecret is a GitHub Actions secret to set on an environment. Values are
// never logged or echoed; only names appear in progress output.
type NamedSecret struct {
	Name  string
	Value string
}

// ── ports (consumed here, implemented by the github adapter) ──

// RepoWriter commits a single file to a repo branch via the contents API,
// creating it or updating it in place.
type RepoWriter interface {
	PutFile(ctx context.Context, t Token, owner, repo, branch, path, content, message string) error
}

// EnvManager creates (or leaves in place) a deployment environment on a repo.
type EnvManager interface {
	EnsureEnvironment(ctx context.Context, t Token, owner, repo, name string) error
}

// EnvSecretSetter stores Actions secrets on a repo's deployment environment,
// encrypting each value with the environment's public key.
type EnvSecretSetter interface {
	SetEnvSecrets(ctx context.Context, t Token, owner, repo, environment string, secrets []NamedSecret) error
}

// DeployService configures continuous deployment of a repo to a server: it
// commits the workflow and deploy.sh, creates the deployment environment, and
// stores the secrets the workflow needs (server connection details, managed by
// mountabo, plus the operator's own env vars).
type DeployService struct {
	servers ServerStore
	vault   SecretVault
	tokens  TokenStore
	repo    RepoWriter
	envs    EnvManager
	secrets EnvSecretSetter
}

// NewDeployService wires the service to its ports.
func NewDeployService(servers ServerStore, vault SecretVault, tokens TokenStore, repo RepoWriter, envs EnvManager, secrets EnvSecretSetter) *DeployService {
	return &DeployService{servers: servers, vault: vault, tokens: tokens, repo: repo, envs: envs, secrets: secrets}
}

// Deploy writes the deploy artifacts to the repo, provisions the environment,
// and sets its secrets, streaming each step to out so the operator follows
// along live. The deployment itself runs later on GitHub's runner (triggered by
// a push), so nothing executes on the server here; what runs there is the
// committed deploy.sh, observable in the Actions log. SERVER_SSH_KEY is
// mountabo's stored private key for the box; it is set as a secret but never
// written to out.
func (s *DeployService) Deploy(ctx context.Context, in DeployInput, out io.Writer) error {
	in.App = strings.TrimSpace(in.App)
	in.Owner = strings.TrimSpace(in.Owner)
	in.Repo = strings.TrimSpace(in.Repo)
	in.Branch = strings.TrimSpace(in.Branch)
	in.DeployDir = strings.TrimSpace(in.DeployDir)
	if in.ServerID == "" || in.App == "" || in.Owner == "" || in.Repo == "" || in.Branch == "" {
		return fmt.Errorf("server, app, owner, repo and branch are required")
	}
	if in.DeployDir == "" {
		return fmt.Errorf("deploy directory is required")
	}
	for _, v := range in.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" && !secretName.MatchString(k) {
			return fmt.Errorf("invalid environment variable name %q", k)
		}
	}

	server, err := s.servers.Get(in.ServerID)
	if err != nil {
		return err
	}
	if server.Status != StatusReady {
		return fmt.Errorf("server must be set up before deploying")
	}

	token, err := s.tokens.Load()
	if err != nil {
		return fmt.Errorf("load github token: %w", err)
	}

	// mountabo's private key for this box becomes SERVER_SSH_KEY so GitHub's
	// runner can SSH in as the mountabo user (its public half is already an
	// authorized_key from bootstrap).
	key, err := s.vault.LoadSecret(privateKeyKey(in.ServerID))
	if err != nil {
		return fmt.Errorf("load server key: %w", err)
	}

	env := strings.TrimSpace(in.Environment)
	if env == "" {
		env = in.Branch
	}
	cfg := workflow.Config{
		App:         in.App,
		Owner:       in.Owner,
		Repo:        in.Repo,
		Branch:      in.Branch,
		Environment: env,
		RootDir:     in.RootDir,
		DeployDir:   in.DeployDir,
		Ports:       workflow.Ports(in.Ports),
		EnvVars:     toWorkflowEnv(in.EnvVars),
	}

	progress(out, "writing %s", workflow.DeployScriptPath)
	if err := s.repo.PutFile(ctx, token, in.Owner, in.Repo, in.Branch, workflow.DeployScriptPath, workflow.DeployScript(cfg), "mountabo: add deploy script"); err != nil {
		return fmt.Errorf("write deploy script: %w", err)
	}

	wfPath := workflow.Path(cfg)
	progress(out, "writing %s", wfPath)
	if err := s.repo.PutFile(ctx, token, in.Owner, in.Repo, in.Branch, wfPath, workflow.Workflow(cfg), "mountabo: add deploy workflow"); err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}

	progress(out, "creating environment %s", env)
	if err := s.envs.EnsureEnvironment(ctx, token, in.Owner, in.Repo, env); err != nil {
		return fmt.Errorf("create environment: %w", err)
	}

	secrets := deploySecrets(server, key, in)
	progress(out, "setting %d secrets on environment %s", len(secrets), env)
	if err := s.secrets.SetEnvSecrets(ctx, token, in.Owner, in.Repo, env, secrets); err != nil {
		return fmt.Errorf("set environment secrets: %w", err)
	}

	progress(out, "deploy configured, push to %s to trigger a deploy", in.Branch)
	return nil
}

// deploySecrets is the full secret set: server connection details mountabo
// manages, the deploy directory, then the operator's env vars (blank keys
// dropped), in a stable order.
func deploySecrets(server Server, sshKey string, in DeployInput) []NamedSecret {
	secrets := []NamedSecret{
		{Name: "SERVER_HOST", Value: server.IP},
		{Name: "SERVER_USER", Value: BootstrapUser},
		{Name: "SERVER_SSH_KEY", Value: sshKey},
		{Name: "DEPLOY_DIR", Value: in.DeployDir},
	}
	for _, v := range in.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" {
			secrets = append(secrets, NamedSecret{Name: k, Value: v.Value})
		}
	}
	return secrets
}

func toWorkflowEnv(vars []DeployEnvVar) []workflow.EnvVar {
	out := make([]workflow.EnvVar, 0, len(vars))
	for _, v := range vars {
		out = append(out, workflow.EnvVar{Key: v.Key, Value: v.Value})
	}
	return out
}

// progress writes one "==> ..." line to the live output stream, the same
// convention the bootstrap and apply-options flows use.
func progress(out io.Writer, format string, args ...any) {
	_, _ = io.WriteString(out, "==> "+fmt.Sprintf(format, args...)+"\n")
}

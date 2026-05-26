package usecase

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/goodylili/mountabo/internal/workflow"
)

// secretName matches a valid GitHub Actions secret name: letters, digits and
// underscores, not starting with a digit. Env var keys must satisfy this before
// their values can be stored as secrets.
var secretName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DeployPort is one published port the operator configured. EnvVar/Value back a
// compose stack's host port; Value/Container back a `docker run -p` mapping.
type DeployPort struct {
	EnvVar    string
	Value     string
	Container string
}

// DeployEnvVar is one application environment variable; its value is stored as a
// GitHub Actions secret and injected into the deploy at run time.
type DeployEnvVar struct {
	Key   string
	Value string
}

// DeployInput is what the operator supplies to wire continuous deployment of a
// repo branch to one of their servers. Strategy is "compose" or "docker"
// (defaulting to compose); Environment names the GitHub deployment environment
// whose secrets the workflow uses, defaulting to Branch.
type DeployInput struct {
	ServerID    string
	App         string
	Owner       string
	Repo        string
	Branch      string
	Environment string
	Strategy    string
	RootDir     string
	DeployDir   string
	Ports       []DeployPort
	EnvVars     []DeployEnvVar
}

// NamedSecret is a GitHub Actions secret to set on an environment. Values are
// never logged or echoed; only names appear in progress output.
type NamedSecret struct {
	Name  string
	Value string
}

// SecretMeta describes a secret the deploy needs, without its value: Managed is
// true for the server-connection secrets mountabo fills in, false for the
// operator's own env vars. Used by the preview so the UI can show what will be
// set and by whom.
type SecretMeta struct {
	Name    string `json:"name"`
	Managed bool   `json:"managed"`
}

// DeployArtifacts is exactly what mountabo will commit and configure: the
// workflow file (and its path), deploy.sh, and the secrets it needs. Generated
// purely from a DeployInput so the UI can preview byte-for-byte what a deploy
// would do.
type DeployArtifacts struct {
	WorkflowPath string       `json:"workflowPath"`
	Workflow     string       `json:"workflow"`
	DeployScript string       `json:"deployScript"`
	Secrets      []SecretMeta `json:"secrets"`
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
// mountabo, plus the operator's own env vars). It is also the single source of
// truth for the generated files, so the UI previews via Preview.
type DeployService struct {
	servers ServerStore
	vault   SecretVault
	tokens  TokenStore
	repo    RepoWriter
	envs    EnvManager
	secrets EnvSecretSetter
	history DeploymentStore
}

// NewDeployService wires the service to its ports.
func NewDeployService(servers ServerStore, vault SecretVault, tokens TokenStore, repo RepoWriter, envs EnvManager, secrets EnvSecretSetter, history DeploymentStore) *DeployService {
	return &DeployService{servers: servers, vault: vault, tokens: tokens, repo: repo, envs: envs, secrets: secrets, history: history}
}

// Preview generates the deploy artifacts from the config alone, no server, no
// token, no side effects, so the configure UI can show exactly what a deploy
// would commit and set.
func (s *DeployService) Preview(in DeployInput) (DeployArtifacts, error) {
	if err := validateDeploy(in); err != nil {
		return DeployArtifacts{}, err
	}
	cfg := buildConfig(in)
	return DeployArtifacts{
		WorkflowPath: workflow.Path(cfg),
		Workflow:     workflow.Workflow(cfg),
		DeployScript: workflow.DeployScript(cfg),
		Secrets:      secretMetas(in),
	}, nil
}

// Deploy writes the deploy artifacts to the repo, provisions the environment,
// and sets its secrets, streaming each step to out so the operator follows
// along live. The deployment itself runs later on GitHub's runner (triggered by
// a push), so nothing executes on the server here; what runs there is the
// committed deploy.sh, observable in the Actions log. SERVER_SSH_KEY is
// mountabo's stored private key for the box; it is set as a secret but never
// written to out.
func (s *DeployService) Deploy(ctx context.Context, in DeployInput, out io.Writer) error {
	if in.ServerID == "" {
		return fmt.Errorf("server is required")
	}
	if err := validateDeploy(in); err != nil {
		return err
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

	cfg := buildConfig(in)
	env := cfg.Environment
	if env == "" {
		env = cfg.Branch
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

	// Record the deployment so the monitor can show its history. A failure here
	// must not fail the deploy, which already succeeded, so it is only noted.
	record := Deployment{
		ID:           newID(),
		App:          cfg.App,
		Owner:        in.Owner,
		Repo:         in.Repo,
		Branch:       in.Branch,
		Environment:  env,
		ServerID:     in.ServerID,
		WorkflowFile: fmt.Sprintf("mountabo-deploy-%s.yml", in.Branch),
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.history.Save(record); err != nil {
		progress(out, "note: deploy succeeded but recording history failed: %v", err)
	}

	progress(out, "deploy configured, push to %s to trigger a %s deploy", in.Branch, cfg.Strategy)
	return nil
}

// validateDeploy checks the config fields common to preview and deploy.
func validateDeploy(in DeployInput) error {
	if strings.TrimSpace(in.App) == "" || strings.TrimSpace(in.Owner) == "" ||
		strings.TrimSpace(in.Repo) == "" || strings.TrimSpace(in.Branch) == "" {
		return fmt.Errorf("app, owner, repo and branch are required")
	}
	if strings.TrimSpace(in.DeployDir) == "" {
		return fmt.Errorf("deploy directory is required")
	}
	for _, v := range in.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" && !secretName.MatchString(k) {
			return fmt.Errorf("invalid environment variable name %q", k)
		}
	}
	return nil
}

// buildConfig maps a validated DeployInput onto the generator's config.
func buildConfig(in DeployInput) workflow.Config {
	strategy := workflow.Compose
	if in.Strategy == string(workflow.Docker) {
		strategy = workflow.Docker
	}
	ports := make([]workflow.Port, 0, len(in.Ports))
	for _, p := range in.Ports {
		ports = append(ports, workflow.Port{EnvVar: p.EnvVar, Value: p.Value, Container: p.Container})
	}
	envs := make([]workflow.EnvVar, 0, len(in.EnvVars))
	for _, v := range in.EnvVars {
		envs = append(envs, workflow.EnvVar{Key: v.Key, Value: v.Value})
	}
	return workflow.Config{
		App:         strings.TrimSpace(in.App),
		Owner:       strings.TrimSpace(in.Owner),
		Repo:        strings.TrimSpace(in.Repo),
		Branch:      strings.TrimSpace(in.Branch),
		Environment: strings.TrimSpace(in.Environment),
		RootDir:     in.RootDir,
		DeployDir:   strings.TrimSpace(in.DeployDir),
		Strategy:    strategy,
		Ports:       ports,
		EnvVars:     envs,
	}
}

// deploySecrets is the full secret set with values: server connection details
// mountabo manages, the deploy directory, then the operator's env vars (blank
// keys dropped), in a stable order.
func deploySecrets(server Server, sshKey string, in DeployInput) []NamedSecret {
	secrets := []NamedSecret{
		{Name: "SERVER_HOST", Value: server.IP},
		{Name: "SERVER_USER", Value: BootstrapUser},
		{Name: "SERVER_SSH_KEY", Value: sshKey},
		{Name: "DEPLOY_DIR", Value: strings.TrimSpace(in.DeployDir)},
	}
	for _, v := range in.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" {
			secrets = append(secrets, NamedSecret{Name: k, Value: v.Value})
		}
	}
	return secrets
}

// secretMetas is the same secret set as deploySecrets but names-only, for the
// preview: SERVER_* + DEPLOY_DIR are mountabo-managed, env vars are the
// operator's.
func secretMetas(in DeployInput) []SecretMeta {
	metas := []SecretMeta{
		{Name: "SERVER_HOST", Managed: true},
		{Name: "SERVER_USER", Managed: true},
		{Name: "SERVER_SSH_KEY", Managed: true},
		{Name: "DEPLOY_DIR", Managed: true},
	}
	for _, v := range in.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" {
			metas = append(metas, SecretMeta{Name: k, Managed: false})
		}
	}
	return metas
}

// progress writes one "==> ..." line to the live output stream, the same
// convention the bootstrap and apply-options flows use.
func progress(out io.Writer, format string, args ...any) {
	_, _ = io.WriteString(out, "==> "+fmt.Sprintf(format, args...)+"\n")
}

package workflow

import (
	"strings"
	"testing"
)

func base() Config {
	return Config{
		App: "shop", Owner: "acme", Repo: "shop", Branch: "main",
		RootDir: "./apps/web", DeployDir: "/opt/shop",
		Ports:   []Port{{EnvVar: "FRONTEND_PORT", Value: "3000", Container: "3000"}},
		EnvVars: []EnvVar{{Key: "DATABASE_URL", Value: "x"}},
	}
}

func TestDeployScript_Compose(t *testing.T) {
	c := base()
	c.Strategy = Compose
	s := DeployScript(c)
	for _, want := range []string{"docker compose build", "docker compose up -d", "export FRONTEND_PORT=3000", `cd "apps/web"`} {
		if !strings.Contains(s, want) {
			t.Errorf("compose script missing %q", want)
		}
	}
	if strings.Contains(s, "docker run") {
		t.Error("compose script should not use docker run")
	}
}

func TestDeployScript_Docker(t *testing.T) {
	c := base()
	c.Strategy = Docker
	s := DeployScript(c)
	for _, want := range []string{"docker build -t", "docker run -d", "-p 3000:3000", `--restart unless-stopped`} {
		if !strings.Contains(s, want) {
			t.Errorf("docker script missing %q", want)
		}
	}
	if strings.Contains(s, "docker compose") {
		t.Error("docker script should not use docker compose")
	}
}

func TestWorkflow_PinsEnvironmentAndSecrets(t *testing.T) {
	c := base()
	c.Environment = "production"
	w := Workflow(c)
	for _, want := range []string{"environment: production", "DATABASE_URL: ${{ secrets.DATABASE_URL }}", "envs: DATABASE_URL,DEPLOY_DIR", "deploy.sh main"} {
		if !strings.Contains(w, want) {
			t.Errorf("workflow missing %q", want)
		}
	}
}

func TestWorkflow_EnvironmentDefaultsToBranch(t *testing.T) {
	if !strings.Contains(Workflow(base()), "environment: main") {
		t.Error("environment should default to the branch")
	}
}

// Package workflow generates the exact files mountabo commits to a repo for a
// docker-compose deploy over SSH: a GitHub Actions workflow that copies
// deploy.sh to the server and runs it, plus deploy.sh itself. Generation is
// pure (config in, text out) and mirrors the frontend's preview generator
// (src/lib/deploy-template.ts) so what the operator previews is byte-for-byte
// what gets committed.
package workflow

import (
	"fmt"
	"strings"
)

// Ports are the published container ports for one environment.
type Ports struct {
	Frontend string
	Backend  string
	Postgres string
	Redis    string
}

// EnvVar is one application environment variable. Its value becomes a GitHub
// Actions secret; the generated files only ever reference it by name.
type EnvVar struct {
	Key   string
	Value string
}

// Config is everything the generated files derive from. Environment is the
// GitHub deployment environment the workflow runs in (so environment-scoped
// secrets resolve); it defaults to Branch when empty.
type Config struct {
	App         string
	Owner       string
	Repo        string
	Branch      string
	Environment string
	RootDir     string
	DeployDir   string
	Ports       Ports
	EnvVars     []EnvVar
}

// DeployScriptPath is where deploy.sh is committed (repo root). The workflow's
// scp step copies it from here by name.
const DeployScriptPath = "deploy.sh"

// environment returns the deployment environment name, defaulting to the branch.
func (c Config) environment() string {
	if e := strings.TrimSpace(c.Environment); e != "" {
		return e
	}
	return c.Branch
}

// envNames returns the application env var keys, in order, dropping blanks.
func (c Config) envNames() []string {
	out := make([]string, 0, len(c.EnvVars))
	for _, v := range c.EnvVars {
		if k := strings.TrimSpace(v.Key); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// Path is the in-repo path of the generated workflow, one per branch.
func Path(c Config) string {
	return fmt.Sprintf(".github/workflows/mountabo-deploy-%s.yml", c.Branch)
}

// Workflow renders the GitHub Actions workflow YAML. The passthrough env block
// and envs list cover the operator's env vars plus DEPLOY_DIR, all sourced from
// secrets; the job pins the deployment environment so its secrets resolve.
func Workflow(c Config) string {
	names := append(c.envNames(), "DEPLOY_DIR")

	var envBlock strings.Builder
	for i, n := range names {
		if i > 0 {
			envBlock.WriteByte('\n')
		}
		fmt.Fprintf(&envBlock, "          %s: ${{ secrets.%s }}", n, n)
	}
	envsList := strings.Join(names, ",")

	return fmt.Sprintf(`name: %[1]s deploy (%[2]s)

on:
  push:
    branches:
      - %[2]s
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: %[3]s
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Copy deploy script to server
        uses: appleboy/scp-action@v0.1.7
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SERVER_SSH_KEY }}
          source: "deploy.sh"
          target: "/tmp/%[1]s-deploy"

      - name: Deploy via docker compose
        uses: appleboy/ssh-action@v1.1.0
        env:
%[4]s
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SERVER_SSH_KEY }}
          envs: %[5]s
          script: |
            chmod +x /tmp/%[1]s-deploy/deploy.sh
            /tmp/%[1]s-deploy/deploy.sh %[2]s
`, c.App, c.Branch, c.environment(), envBlock.String(), envsList)
}

// DeployScript renders deploy.sh: it clones or fast-forwards the repo on the
// server, writes a .env from the port settings and the operator's env vars
// (injected by the workflow), then rebuilds and swaps the compose stack.
func DeployScript(c Config) string {
	// Strip a leading "./" or "/" and any trailing "/" so rootDir is a clean
	// relative path; "" means the repo root (no cd).
	root := strings.TrimRight(strings.TrimLeft(c.RootDir, "./"), "/")
	cdRoot := ""
	if root != "" {
		cdRoot = fmt.Sprintf("cd %q\n", root)
	}

	var envFileLines strings.Builder
	for i, n := range c.envNames() {
		if i > 0 {
			envFileLines.WriteByte('\n')
		}
		fmt.Fprintf(&envFileLines, "%s=${%s}", n, n)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

BRANCH="${1:-%[2]s}"
DEPLOY_DIR="${DEPLOY_DIR:-%[6]s}/$BRANCH"
REPO_URL="git@github.com:%[3]s/%[4]s.git"

echo "=== %[1]s deploy (branch: $BRANCH) ==="

# Ports for this environment.
export FRONTEND_PORT=%[7]s
export BACKEND_PORT=%[8]s
export POSTGRES_PORT=%[9]s
export REDIS_PORT=%[10]s

mkdir -p "$(dirname "$DEPLOY_DIR")"

if [ -d "$DEPLOY_DIR/.git" ]; then
  echo "Pulling latest changes..."
  cd "$DEPLOY_DIR"
  git fetch origin
  git reset --hard "origin/$BRANCH"
else
  echo "Cloning repository..."
  rm -rf "$DEPLOY_DIR"
  git clone --branch "$BRANCH" "$REPO_URL" "$DEPLOY_DIR"
  cd "$DEPLOY_DIR"
fi

%[5]s# Write .env (ports + your environment variables) for docker compose.
cat > .env <<EOF
FRONTEND_PORT=${FRONTEND_PORT}
BACKEND_PORT=${BACKEND_PORT}
POSTGRES_PORT=${POSTGRES_PORT}
REDIS_PORT=${REDIS_PORT}
%[11]s
EOF

# Zero-downtime: build new images while old containers keep running, then swap.
echo "Building images..."
docker compose build
echo "Swapping to new containers..."
docker compose up -d --force-recreate --remove-orphans

docker compose ps
echo "=== Deployment complete ==="
`, c.App, c.Branch, c.Owner, c.Repo, cdRoot, c.DeployDir,
		c.Ports.Frontend, c.Ports.Backend, c.Ports.Postgres, c.Ports.Redis,
		envFileLines.String())
}

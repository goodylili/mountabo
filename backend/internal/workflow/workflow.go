// Package workflow generates the exact files mountabo commits to a repo to
// deploy it over SSH: a GitHub Actions workflow that copies deploy.sh to the
// server and runs it, plus deploy.sh itself. Generation is pure (config in,
// text out) and is the single source of truth, the configure UI previews these
// same bytes by calling the backend, so the preview always matches what gets
// committed.
//
// Two deploy strategies are supported. "compose" drives a docker compose stack
// (the repo has a Compose file); "docker" builds the repo's Dockerfile and runs
// a single container, publishing each EXPOSE'd port to an editable host port.
package workflow

import (
	"fmt"
	"strings"
)

// Strategy selects how the committed deploy.sh ships the app.
type Strategy string

const (
	// Compose deploys with `docker compose` (the repo has a Compose file).
	Compose Strategy = "compose"
	// Docker builds the repo's Dockerfile and runs one container.
	Docker Strategy = "docker"
)

// Port is one published port. EnvVar/Value drive a compose stack's host port
// (written into .env and referenced by the Compose file); Value/Container drive
// a plain `docker run -p Value:Container` mapping. Value is the host port the
// operator chose; Container is the port the app listens on inside.
type Port struct {
	EnvVar    string
	Value     string
	Container string
}

// EnvVar is one application environment variable; its value is a GitHub Actions
// secret, the generated files only ever reference it by name.
type EnvVar struct {
	Key   string
	Value string
}

// Config is everything the generated files derive from. Environment is the
// GitHub deployment environment whose secrets the workflow uses; it defaults to
// Branch. Strategy defaults to Compose when empty.
type Config struct {
	App         string
	Owner       string
	Repo        string
	Branch      string
	Environment string
	RootDir     string
	DeployDir   string
	Strategy    Strategy
	Ports       []Port
	EnvVars     []EnvVar
	// DeployKeyFile, when set, is the name of the repo's read-only deploy key
	// under the deploy user's ~/.ssh; deploy.sh uses it for git over SSH. Empty
	// falls back to whatever key the server's git already has.
	DeployKeyFile string
}

// DeployScriptPath is where deploy.sh is committed (repo root). The workflow's
// scp step copies it from here by name.
const DeployScriptPath = "deploy.sh"

func (c Config) environment() string {
	if e := strings.TrimSpace(c.Environment); e != "" {
		return e
	}
	return c.Branch
}

func (c Config) strategy() Strategy {
	if c.Strategy == Docker {
		return Docker
	}
	return Compose
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

// Workflow renders the GitHub Actions workflow YAML. It is the same for both
// strategies, it only copies deploy.sh to the server and runs it; the strategy
// lives entirely in deploy.sh. The passthrough env block and envs list cover
// the operator's env vars plus DEPLOY_DIR (all from secrets); the job pins the
// deployment environment so its secrets resolve.
func Workflow(c Config) string {
	names := append(c.envNames(), "DEPLOY_DIR")

	var envBlock strings.Builder
	for i, n := range names {
		if i > 0 {
			envBlock.WriteByte('\n')
		}
		fmt.Fprintf(&envBlock, "          %s: ${{ secrets.%s }}", n, n)
	}

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

      - name: Deploy on the server
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
`, c.App, c.Branch, c.environment(), envBlock.String(), strings.Join(names, ","))
}

// DeployScript renders deploy.sh for the configured strategy.
func DeployScript(c Config) string {
	if c.strategy() == Docker {
		return dockerScript(c)
	}
	return composeScript(c)
}

// header is the shared top of every deploy.sh: strict mode, the branch and
// deploy directory, the repo's SSH URL, and a banner.
func header(c Config) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

BRANCH="${1:-%[1]s}"
DEPLOY_DIR="${DEPLOY_DIR:-%[2]s}/$BRANCH"
REPO_URL="git@github.com:%[3]s/%[4]s.git"

echo "=== %[5]s deploy (branch: $BRANCH) ==="`, c.Branch, c.DeployDir, c.Owner, c.Repo, c.App)
}

// gitAuth points git at the repo's read-only deploy key for SSH clones, when
// one is configured. mountabo installs that key on the server out of band.
func gitAuth(c Config) string {
	if c.DeployKeyFile == "" {
		return ""
	}
	return fmt.Sprintf(`# Use the repo's read-only deploy key for git over SSH.
export GIT_SSH_COMMAND="ssh -i $HOME/.ssh/%s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"

`, c.DeployKeyFile)
}

// clonePull clones the repo on first deploy, or fast-forwards it on later ones.
const clonePull = `mkdir -p "$(dirname "$DEPLOY_DIR")"

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
fi`

// cdRoot is the `cd <rootDir>` line (with trailing newline) when the app lives
// in a sub-directory, or "" for the repo root.
func cdRoot(c Config) string {
	root := strings.TrimRight(strings.TrimLeft(c.RootDir, "./"), "/")
	if root == "" {
		return ""
	}
	return fmt.Sprintf("cd %q\n", root)
}

// envFileBody is the heredoc body written to .env, one KEY=${KEY} per line. The
// values come from the workflow's injected secrets at run time. Compose also
// gets the port env vars (the Compose file reads them); docker does not (its
// ports are -p flags).
func envFileBody(c Config, includePorts bool) string {
	var lines []string
	if includePorts {
		for _, p := range c.Ports {
			if p.EnvVar != "" {
				lines = append(lines, fmt.Sprintf("%s=${%s}", p.EnvVar, p.EnvVar))
			}
		}
	}
	for _, k := range c.envNames() {
		lines = append(lines, fmt.Sprintf("%s=${%s}", k, k))
	}
	return strings.Join(lines, "\n")
}

func composeScript(c Config) string {
	var b strings.Builder
	b.WriteString(header(c))
	b.WriteByte('\n')

	var exports []string
	for _, p := range c.Ports {
		if p.EnvVar != "" {
			exports = append(exports, fmt.Sprintf("export %s=%s", p.EnvVar, hostValue(p)))
		}
	}
	if len(exports) > 0 {
		b.WriteString("\n# Ports for this environment (detected from your compose file).\n")
		b.WriteString(strings.Join(exports, "\n"))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(gitAuth(c))
	b.WriteString(clonePull)
	b.WriteString("\n\n")
	b.WriteString(cdRoot(c))
	b.WriteString("# Write .env (ports + your environment variables) for docker compose.\ncat > .env <<EOF\n")
	b.WriteString(envFileBody(c, true))
	b.WriteString("\nEOF\n\n")
	b.WriteString(`# Zero-downtime: build new images while old containers keep running, then swap.
echo "Building images..."
docker compose build
echo "Swapping to new containers..."
docker compose up -d --force-recreate --remove-orphans

docker compose ps
echo "=== Deployment complete ==="
`)
	return b.String()
}

func dockerScript(c Config) string {
	var b strings.Builder
	b.WriteString(header(c))
	b.WriteString("\n\n")
	b.WriteString(gitAuth(c))
	b.WriteString(clonePull)
	b.WriteString("\n\n")
	b.WriteString(cdRoot(c))
	b.WriteString("# Write .env (your environment variables) for the container.\ncat > .env <<EOF\n")
	b.WriteString(envFileBody(c, false))
	b.WriteString("\nEOF\n\n")

	fmt.Fprintf(&b, "IMAGE=%q\n", c.App+":$BRANCH")
	fmt.Fprintf(&b, "CONTAINER=%q\n\n", c.App+"-$BRANCH")
	b.WriteString(`echo "Building image..."
docker build -t "$IMAGE" .
echo "Swapping container..."
docker rm -f "$CONTAINER" 2>/dev/null || true
docker run -d --name "$CONTAINER" --restart unless-stopped \
  --env-file .env`)
	for _, p := range c.Ports {
		if p.Container != "" {
			fmt.Fprintf(&b, " \\\n  -p %s:%s", hostValue(p), p.Container)
		}
	}
	b.WriteString(" \\\n  \"$IMAGE\"\n")
	b.WriteString(`docker ps --filter "name=$CONTAINER"
echo "=== Deployment complete ==="
`)
	return b.String()
}

// hostValue is the host port to publish: the operator's chosen value, falling
// back to the container port when they left it blank.
func hostValue(p Port) string {
	if v := strings.TrimSpace(p.Value); v != "" {
		return v
	}
	return p.Container
}

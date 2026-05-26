// Generates the exact files mountabo will write to a repo for a docker-compose
// deploy over SSH: a GitHub Actions workflow that copies deploy.sh to the server
// and runs it, plus the deploy.sh itself. Everything here is derived from the
// user's configuration so the preview matches what gets committed.
//
// NB: `${'$'}{{ secrets.X }}` and bash `${'$'}{VAR}` must survive as literal text,
// so every literal `$` that precedes `{` is escaped (`\$`) to avoid JS template
// interpolation. Real interpolations use the unescaped `${...}`.

export type EnvVar = { key: string; value: string };

export type DeployConfig = {
  app: string;
  owner: string;
  repo: string;
  branch: string;
  rootDir: string;
  deployDir: string;
  ports: { frontend: string; backend: string; postgres: string; redis: string };
  envVars: EnvVar[];
};

function envNames(cfg: DeployConfig): string[] {
  return cfg.envVars.map((v) => v.key.trim()).filter(Boolean);
}

// The GitHub Actions secrets this deploy needs: server connection (auto-set by
// mountabo) + the user's env vars (their values become secrets).
export function deploySecrets(cfg: DeployConfig): { name: string; managed: "mountabo" | "you" }[] {
  const server: { name: string; managed: "mountabo" | "you" }[] = [
    { name: "SERVER_HOST", managed: "mountabo" },
    { name: "SERVER_USER", managed: "mountabo" },
    { name: "SERVER_SSH_KEY", managed: "mountabo" },
    { name: "DEPLOY_DIR", managed: "mountabo" },
  ];
  return [...server, ...envNames(cfg).map((n) => ({ name: n, managed: "you" as const }))];
}

export function workflowPath(cfg: DeployConfig): string {
  return `.github/workflows/mountabo-deploy-${cfg.branch}.yml`;
}

export function generateWorkflow(cfg: DeployConfig): string {
  const names = [...envNames(cfg), "DEPLOY_DIR"];
  const envBlock = names.map((n) => `          ${n}: \${{ secrets.${n} }}`).join("\n");
  const envsList = names.join(",");

  return `name: ${cfg.app} deploy (${cfg.branch})

on:
  push:
    branches:
      - ${cfg.branch}
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Copy deploy script to server
        uses: appleboy/scp-action@v0.1.7
        with:
          host: \${{ secrets.SERVER_HOST }}
          username: \${{ secrets.SERVER_USER }}
          key: \${{ secrets.SERVER_SSH_KEY }}
          source: "deploy.sh"
          target: "/tmp/${cfg.app}-deploy"

      - name: Deploy via docker compose
        uses: appleboy/ssh-action@v1.1.0
        env:
${envBlock}
        with:
          host: \${{ secrets.SERVER_HOST }}
          username: \${{ secrets.SERVER_USER }}
          key: \${{ secrets.SERVER_SSH_KEY }}
          envs: ${envsList}
          script: |
            chmod +x /tmp/${cfg.app}-deploy/deploy.sh
            /tmp/${cfg.app}-deploy/deploy.sh ${cfg.branch}
`;
}

export function generateDeployScript(cfg: DeployConfig): string {
  const root = cfg.rootDir.replace(/^\.?\/*/, "").replace(/\/*$/, ""); // "" for repo root
  const cdRoot = root ? `cd "${root}"\n` : "";
  const envFileLines = envNames(cfg)
    .map((k) => `${k}=\${${k}}`)
    .join("\n");

  return `#!/usr/bin/env bash
set -euo pipefail

BRANCH="\${1:-${cfg.branch}}"
DEPLOY_DIR="\${DEPLOY_DIR:-${cfg.deployDir}}/$BRANCH"
REPO_URL="git@github.com:${cfg.owner}/${cfg.repo}.git"

echo "=== ${cfg.app} deploy (branch: $BRANCH) ==="

# Ports for this environment.
export FRONTEND_PORT=${cfg.ports.frontend}
export BACKEND_PORT=${cfg.ports.backend}
export POSTGRES_PORT=${cfg.ports.postgres}
export REDIS_PORT=${cfg.ports.redis}

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

${cdRoot}# Write .env (ports + your environment variables) for docker compose.
cat > .env <<EOF
FRONTEND_PORT=\${FRONTEND_PORT}
BACKEND_PORT=\${BACKEND_PORT}
POSTGRES_PORT=\${POSTGRES_PORT}
REDIS_PORT=\${REDIS_PORT}
${envFileLines}
EOF

# Zero-downtime: build new images while old containers keep running, then swap.
echo "Building images..."
docker compose build
echo "Swapping to new containers..."
docker compose up -d --force-recreate --remove-orphans

docker compose ps
echo "=== Deployment complete ==="
`;
}

// Generates the exact files mountabo will write to a repo for a docker-compose
// deploy over SSH: a GitHub Actions workflow that copies deploy.sh to the server
// and runs it, plus the deploy.sh itself. Everything here is derived from the
// user's configuration so the preview matches what gets committed.
//
// NB: `${'$'}{{ secrets.X }}` and bash `${'$'}{VAR}` must survive as literal text,
// so every literal `$` that precedes `{` is escaped (`\$`) to avoid JS template
// interpolation. Real interpolations use the unescaped `${...}`.

export type EnvVar = { key: string; value: string };

// parseEnvFile turns the contents of a .env file (or pasted blob) into rows.
// It accepts `KEY=value`, `export KEY=value`, blank lines, and `#` comments,
// and strips one layer of surrounding single/double quotes from values. It does
// not strip inline comments, secret values legitimately contain `#`.
export function parseEnvFile(text: string): EnvVar[] {
  const out: EnvVar[] = [];
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const body = line.startsWith("export ") ? line.slice(7).trim() : line;
    const eq = body.indexOf("=");
    if (eq <= 0) continue; // no key, or no `=`
    const key = body.slice(0, eq).trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) continue; // not a valid env name
    let value = body.slice(eq + 1).trim();
    if (value.length >= 2 && (value[0] === '"' || value[0] === "'") && value.at(-1) === value[0]) {
      value = value.slice(1, -1);
    }
    out.push({ key, value });
  }
  return out;
}

// mergeEnv overlays imported vars onto existing rows: empty rows are dropped,
// a matching key is updated in place, new keys are appended. Always returns at
// least one row so the form keeps an editable line.
export function mergeEnv(existing: EnvVar[], incoming: EnvVar[]): EnvVar[] {
  const out = existing.filter((r) => r.key.trim());
  for (const v of incoming) {
    const i = out.findIndex((r) => r.key === v.key);
    if (i >= 0) out[i] = v;
    else out.push(v);
  }
  return out.length ? out : [{ key: "", value: "" }];
}

export type DeployConfig = {
  app: string;
  owner: string;
  repo: string;
  branch: string;
  // GitHub deployment environment the workflow runs in (so environment secrets
  // resolve). Defaults to the branch name.
  environment?: string;
  rootDir: string;
  deployDir: string;
  // Host ports mountabo sets for this deployment, one per environment variable
  // the repo's compose file binds a host port to (e.g. FRONTEND_PORT=3000).
  // Detected from the project itself, so it is empty for repos that declare no
  // variable-backed ports.
  ports: PortVar[];
  envVars: EnvVar[];
};

export type PortVar = { envVar: string; value: string };

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
    environment: ${cfg.environment ?? cfg.branch}
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
  // Port env vars detected from the repo's compose file, plus the user's vars.
  const portExports = cfg.ports.map((p) => `export ${p.envVar}=${p.value}`).join("\n");
  const portBlock = portExports ? `\n# Ports for this environment (detected from your compose file).\n${portExports}\n` : "";
  const envFileLines = [...cfg.ports.map((p) => p.envVar), ...envNames(cfg)]
    .map((k) => `${k}=\${${k}}`)
    .join("\n");

  return `#!/usr/bin/env bash
set -euo pipefail

BRANCH="\${1:-${cfg.branch}}"
DEPLOY_DIR="\${DEPLOY_DIR:-${cfg.deployDir}}/$BRANCH"
REPO_URL="git@github.com:${cfg.owner}/${cfg.repo}.git"

echo "=== ${cfg.app} deploy (branch: $BRANCH) ==="
${portBlock}
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

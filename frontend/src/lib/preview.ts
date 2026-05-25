import type { Server, Source } from "@/lib/data";

// The exact file mountabo would write to the repo. Kept as a template so the
// preview pane shows precisely what lands in .github/workflows/.
export function workflowYaml(source: Source, server: Server, branch: string): string {
  return `name: mountabo deploy

on:
  push:
    branches: [${branch}]

concurrency:
  group: mountabo-deploy-\${{ github.ref }}
  cancel-in-progress: true

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: deploy over ssh
        uses: appleboy/ssh-action@v1
        with:
          host: \${{ secrets.SERVER_IP }}
          username: \${{ secrets.SERVER_USER }}
          key: \${{ secrets.SSH_PRIVATE_KEY }}
          port: ${server.specs.sshPort}
          script: |
            cd ~/apps/${source.name}
            git pull --ff-only origin ${branch}
            docker compose pull
            docker compose up -d --remove-orphans
            docker image prune -f`;
}

export type SecretRow = { name: string; value: string; masked?: boolean };

export function secretRows(server: Server): SecretRow[] {
  return [
    { name: "SERVER_IP", value: server.ip },
    { name: "SERVER_USER", value: "root" },
    { name: "SSH_PRIVATE_KEY", value: "··· ed25519 · 256 bit · masked", masked: true },
  ];
}

export type DeployKeyInfo = {
  algo: string;
  bits: string;
  publicKey: string;
  fingerprint: string;
};

export const deployKey: DeployKeyInfo = {
  algo: "ed25519",
  bits: "256 bit",
  publicKey:
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH8m4Qb2k9pZ0qLxQ2vR7sT1cW3nF6dG9hJ0kL2mN4p mountabo@falkenstein-1",
  fingerprint: "SHA256:9Xb2k+Qe7Lp0qLxQ2vR7sT1cW3nF6dG9hJ0kL2mN4p",
};

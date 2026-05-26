// Shared shapes for the local UI. Repositories come from the Go backend (GitHub
// API) via lib/repos. Servers and deployments will come from the backend
// (SQLite + GitHub Actions runs) once those flows exist; until then they are
// empty, the UI shows real state only, never fabricated data.

export type Source = {
  owner: string;
  name: string;
  branch: string;
  branchAccent?: boolean;
  updated: string;
  language: string;
  loc?: string;
  private?: boolean;
  hasDocker?: boolean;
};

export type ServerStatus = "healthy" | "idle";

export type Server = {
  id: string;
  name: string;
  initial: string;
  accent?: boolean;
  provider: string;
  plan: string;
  ip: string;
  region: string;
  status: ServerStatus;
  uptimePct?: string;
  uptimeLabel: string;
  specs: {
    cpu: string;
    ram: string;
    os: string;
    ping: string;
    sshPort: number;
  };
};

// No servers until the user adds one (server-add flow is not built yet).
export const servers: Server[] = [];

export type RunStatus = "success" | "failed" | "running";

export type DeployRun = {
  sha: string;
  message: string;
  status: RunStatus;
  when: string;
  duration: string;
};

export type Deployment = {
  app: string;
  repo: string;
  serverId: string;
  branch: string;
  status: "live" | "idle" | "failing";
  url: string;
  uptimePct: string;
  lastDeploy: string;
  metrics: { cpu: string; mem: string; ping: string };
  runs: DeployRun[];
};

// What's currently running where, assembled from GitHub Actions run history +
// on-demand server pings. Empty until that backend flow exists.
export const deployments: Deployment[] = [];

export function findServer(id?: string): Server | undefined {
  return servers.find((s) => s.id === id);
}

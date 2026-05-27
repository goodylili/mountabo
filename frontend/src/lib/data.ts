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
  // How the repo containerizes: "compose", "docker" (Dockerfile only), or
  // "none". Drives the container filter on the deploy picker.
  kind?: "compose" | "docker" | "none";
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
  // Applied hardening option ids (incl. monitoring tools like "netdata"), so the
  // monitor can show which tools are set up and offer to install the rest.
  options?: string[];
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
  // Direct GitHub links: the run on Actions and the commit. Open in a new tab.
  runUrl: string;
  commitUrl: string;
};

export type Deployment = {
  app: string;
  repo: string;
  serverId: string;
  branch: string;
  status: "live" | "idle" | "failing";
  // The running app's public address (what the operator actually visits).
  liveUrl: string;
  // The GitHub Actions workflow file/page that drives the deployment (secondary
  // link, not where the app lives).
  workflowUrl: string;
  uptimePct: string;
  lastDeploy: string;
  metrics: { cpu: string; mem: string; ping: string };
  runs: DeployRun[];
  // Tracking (from the SQLite deploy-event log): total times deployed + a recent
  // timeline (newest first).
  deploys?: number;
  timeline?: { when: string; environment: string }[];
};

// What's currently running where, assembled from GitHub Actions run history +
// on-demand server pings. Empty until that backend flow exists.
export const deployments: Deployment[] = [];

export function findServer(id?: string): Server | undefined {
  return servers.find((s) => s.id === id);
}

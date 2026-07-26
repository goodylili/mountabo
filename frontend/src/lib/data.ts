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
  // Applied hardening option ids (incl. monitoring tools like "uptime-kuma"), so the
  // monitor can show which tools are set up and offer to install the rest.
  options?: string[];
};

// No servers until the user adds one (server-add flow is not built yet).
export const servers: Server[] = [];

export type RunStatus = "success" | "failed" | "running";

// ReleaseStatus is a release's state in the deploy log. "pending" is a release
// recorded at deploy time whose Actions run has not completed (or aged out), so
// no live run status has been overlaid yet.
export type ReleaseStatus = RunStatus | "pending";

// Release is one entry in a deployment's release history. sha/actor are captured
// durably at deploy time; status/runUrl are overlaid live from the matching
// Actions run when available.
export type Release = {
  when: string;
  environment: string;
  sha: string;
  status: ReleaseStatus;
  actor: string;
  runUrl: string;
};

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
  // Primary published port on the server, used to scope custom domains to this
  // environment (only domains with this upstream are listed on its card). Zero
  // when no port was configured (a compose stack with no host publish).
  port: number;
  uptimePct: string;
  lastDeploy: string;
  metrics: { cpu: string; mem: string; ping: string };
  runs: DeployRun[];
  // Tracking (from the SQLite deploy-event log): total times deployed + a recent
  // release history (newest first). The field keeps the name "timeline" to match
  // the backend payload key.
  deploys?: number;
  timeline?: Release[];
  // locked is true while a deploy to this target is in flight (the backend's
  // in-process deploy lock is held); lockedSince is a relative time like "2m ago".
  // The card shows a lock indicator and offers a "clear stuck lock" action.
  locked?: boolean;
  lockedSince?: string;
};

// What's currently running where, assembled from GitHub Actions run history +
// on-demand server pings. Empty until that backend flow exists.
export const deployments: Deployment[] = [];

export function findServer(id?: string): Server | undefined {
  return servers.find((s) => s.id === id);
}

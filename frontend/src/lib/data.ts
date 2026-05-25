// Mock state for the local UI. In the shipped product these come from the Go
// backend (GitHub API for sources, SQLite for servers). Shapes mirror what the
// backend will eventually return so the views don't change.

export type Source = {
  owner: string;
  name: string;
  branch: string;
  branchAccent?: boolean;
  updated: string;
  language: string;
  loc?: string;
  private?: boolean;
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

export const ACCOUNT_LOGIN = "goodylili";

export const sources: Source[] = [
  {
    owner: "goodylili",
    name: "mountabo",
    branch: "main",
    updated: "10h ago",
    language: "go",
    loc: "8.4k loc",
  },
  {
    owner: "goodylili",
    name: "dropboy",
    branch: "main",
    updated: "2d ago",
    language: "typescript",
  },
  {
    owner: "goodylili",
    name: "portfolio",
    branch: "main",
    updated: "2d ago",
    language: "next.js",
    private: true,
  },
  {
    owner: "goodylili",
    name: "carousels",
    branch: "main",
    updated: "2d ago",
    language: "svelte",
  },
  {
    owner: "goodylili",
    name: "prop-firm",
    branch: "turbo",
    branchAccent: true,
    updated: "may 19",
    language: "monorepo",
    private: true,
  },
];

export const servers: Server[] = [
  {
    id: "falkenstein-1",
    name: "falkenstein-1",
    initial: "H",
    accent: true,
    provider: "hetzner",
    plan: "cpx21",
    ip: "49.13.103.7",
    region: "fsn1, de",
    status: "healthy",
    uptimePct: "99.2%",
    uptimeLabel: "up 43d",
    specs: { cpu: "4 vcpu", ram: "8 gb", os: "ubuntu 24.04", ping: "38 ms", sshPort: 22 },
  },
  {
    id: "contabo-storage",
    name: "contabo-storage",
    initial: "C",
    provider: "contabo",
    plan: "vps-s",
    ip: "178.18.246.21",
    region: "eu",
    status: "healthy",
    uptimePct: "100%",
    uptimeLabel: "up 91d",
    specs: { cpu: "6 vcpu", ram: "16 gb", os: "debian 12", ping: "61 ms", sshPort: 22 },
  },
  {
    id: "do-montreal",
    name: "do-montreal",
    initial: "D",
    provider: "digitalocean",
    plan: "s-2vcpu",
    ip: "104.131.x.x",
    region: "tor1",
    status: "idle",
    uptimeLabel: "last 12d",
    specs: { cpu: "2 vcpu", ram: "4 gb", os: "ubuntu 22.04", ping: "-", sshPort: 22 },
  },
];

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

// What's currently running where. In production this is assembled from GitHub
// Actions run history + on-demand server pings; mountabo itself isn't in the path.
export const deployments: Deployment[] = [
  {
    app: "mountabo",
    repo: "goodylili/mountabo",
    serverId: "falkenstein-1",
    branch: "main",
    status: "live",
    url: "https://mountabo.goodylili.dev",
    uptimePct: "99.98%",
    lastDeploy: "10h ago",
    metrics: { cpu: "12%", mem: "1.8 / 8 gb", ping: "38 ms" },
    runs: [
      { sha: "e62d137", message: "first commit", status: "success", when: "10h ago", duration: "1m 12s" },
      { sha: "a91f4c2", message: "wire health endpoint", status: "success", when: "1d ago", duration: "1m 04s" },
      { sha: "77bd0e9", message: "bump go 1.25", status: "success", when: "2d ago", duration: "58s" },
      { sha: "3c5a118", message: "fix compose restart", status: "failed", when: "2d ago", duration: "41s" },
      { sha: "0de77a1", message: "initial deploy", status: "success", when: "3d ago", duration: "1m 20s" },
    ],
  },
  {
    app: "dropboy",
    repo: "goodylili/dropboy",
    serverId: "contabo-storage",
    branch: "main",
    status: "live",
    url: "https://dropboy.goodylili.dev",
    uptimePct: "100%",
    lastDeploy: "2d ago",
    metrics: { cpu: "6%", mem: "0.9 / 16 gb", ping: "61 ms" },
    runs: [
      { sha: "b2e91a7", message: "add presigned uploads", status: "success", when: "2d ago", duration: "2m 30s" },
      { sha: "f4d2c80", message: "tighten cors", status: "success", when: "4d ago", duration: "2m 18s" },
    ],
  },
  {
    app: "portfolio",
    repo: "goodylili/portfolio",
    serverId: "do-montreal",
    branch: "main",
    status: "failing",
    url: "https://goodylili.dev",
    uptimePct: "-",
    lastDeploy: "12d ago",
    metrics: { cpu: "-", mem: "-", ping: "-" },
    runs: [
      { sha: "9a01ff3", message: "redesign hero", status: "failed", when: "12d ago", duration: "3m 02s" },
      { sha: "7712bbe", message: "add og images", status: "failed", when: "12d ago", duration: "2m 55s" },
      { sha: "1c0a4de", message: "switch to app router", status: "success", when: "20d ago", duration: "2m 40s" },
    ],
  },
];

export function findSource(full?: string): Source {
  const match = sources.find((s) => `${s.owner}/${s.name}` === full || s.name === full);
  return match ?? sources[0];
}

export function findServer(id?: string): Server {
  return servers.find((s) => s.id === id) ?? servers[0];
}

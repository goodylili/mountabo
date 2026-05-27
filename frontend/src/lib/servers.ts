// Server state comes from the Go backend (probed over SSH, persisted to
// ~/.mountabo/servers.json). These shapes mirror the backend JSON exactly.

export type ServerStatus = "probed" | "setting_up" | "ready" | "failed";

export type ServerSpecs = {
  os: string;
  kernel: string;
  arch: string;
  cpuCores: number;
  cpuModel: string;
  memoryMB: number;
  diskGB: number;
  hostname: string;
};

export type ServerView = {
  id: string;
  name: string;
  ip: string;
  sshPort: number;
  timezone: string;
  status: ServerStatus;
  specs: ServerSpecs;
  fingerprint: string;
  createdAt: string;
  options: string[] | null;
  history?: ChangeEvent[] | null;
  domains?: Domain[] | null;
};

// A custom domain fronted by nginx + HTTPS on a server, proxying to a local app
// port. Mirrors the backend Domain JSON.
export type Domain = {
  host: string;
  aliases?: string[] | null;
  upstream: string;
  email?: string;
  createdAt: string;
};

// A recorded configuration change applied to a server.
export type ChangeEvent = {
  at: string;
  added?: string[];
  removed?: string[];
  status: string; // "applied" | "failed"
};

// A parameter an option needs (collected inline when ticked).
export type OptionParam = {
  key: string;
  label: string;
  default?: string;
  placeholder?: string;
};

// An opt-in hardening step the operator can choose. Category groups them in the UI.
export type SetupOption = {
  id: string;
  name: string;
  category: string;
  description: string;
  params?: OptionParam[];
};

// Maps a real ServerView to the display Server shape the configure/deploy views
// expect, filling fields the backend doesn't track yet with sensible stand-ins.
export function toDisplayServer(s: ServerView): import("@/lib/data").Server {
  return {
    id: s.id,
    name: s.name,
    initial: (s.name[0] ?? "?").toUpperCase(),
    provider: "vps",
    plan: s.specs.arch || "n/a",
    ip: s.ip,
    region: s.timezone || "n/a",
    status: s.status === "ready" ? "healthy" : "idle",
    uptimeLabel: s.status,
    specs: {
      cpu: s.specs.cpuCores ? `${s.specs.cpuCores} vcpu` : "n/a",
      ram: s.specs.memoryMB ? `${Math.round(s.specs.memoryMB / 1024)} gb` : "n/a",
      os: s.specs.os || "n/a",
      ping: "n/a",
      sshPort: s.sshPort,
    },
    options: s.options ?? [],
  };
}

export function backendURL(): string {
  return process.env.MOUNTABO_BACKEND ?? "http://localhost:7778";
}

// Fetches added servers from the backend. Returns [] when the backend is
// unreachable so the UI shows an honest empty state.
export async function getServers(): Promise<ServerView[]> {
  try {
    const resp = await fetch(`${backendURL()}/api/servers`, { cache: "no-store" });
    if (!resp.ok) return [];
    return (await resp.json()) as ServerView[];
  } catch {
    return [];
  }
}

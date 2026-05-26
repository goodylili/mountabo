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
};

// An opt-in hardening step the operator can choose at setup time.
export type SetupOption = {
  id: string;
  name: string;
  description: string;
};

// Maps a real ServerView to the display Server shape the configure/deploy views
// expect, filling fields the backend doesn't track yet with sensible stand-ins.
export function toDisplayServer(s: ServerView): import("@/lib/data").Server {
  return {
    id: s.id,
    name: s.name,
    initial: (s.name[0] ?? "?").toUpperCase(),
    provider: "vps",
    plan: s.specs.arch || "—",
    ip: s.ip,
    region: s.timezone || "—",
    status: s.status === "ready" ? "healthy" : "idle",
    uptimeLabel: s.status,
    specs: {
      cpu: s.specs.cpuCores ? `${s.specs.cpuCores} vcpu` : "—",
      ram: s.specs.memoryMB ? `${Math.round(s.specs.memoryMB / 1024)} gb` : "—",
      os: s.specs.os || "—",
      ping: "—",
      sshPort: s.sshPort,
    },
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

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
};

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

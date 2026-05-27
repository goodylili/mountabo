// ServerMetrics is a point-in-time read of a server's host health (from the Go
// backend over SSH). Zero values mean "unavailable".
export type ServerMetrics = {
  cpuCores: number;
  load1: number;
  memUsedMB: number;
  memTotalMB: number;
  diskUsedGB: number;
  diskTotalGB: number;
  uptimeSeconds: number;
};

// fetchServerMetrics reads a server's live host metrics. Returns null on any
// failure (server not set up, unreachable) so the monitor falls back to "n/a".
export async function fetchServerMetrics(serverId: string, signal?: AbortSignal): Promise<ServerMetrics | null> {
  if (!serverId) return null;
  try {
    const resp = await fetch(`/api/servers/${serverId}/metrics`, { cache: "no-store", signal });
    if (!resp.ok) return null;
    return (await resp.json()) as ServerMetrics;
  } catch {
    return null;
  }
}

// Formatting helpers for the monitor.
export function fmtLoad(m: ServerMetrics): string {
  if (!m.cpuCores) return `${m.load1.toFixed(2)} load`;
  const pct = Math.min(100, Math.round((m.load1 / m.cpuCores) * 100));
  return `${pct}% · ${m.load1.toFixed(2)} load`;
}

export function fmtMem(m: ServerMetrics): string {
  if (!m.memTotalMB) return "n/a";
  const gb = (v: number) => (v / 1024).toFixed(v >= 1024 ? 1 : 2);
  return `${gb(m.memUsedMB)} / ${gb(m.memTotalMB)} GB`;
}

export function fmtDisk(m: ServerMetrics): string {
  if (!m.diskTotalGB) return "n/a";
  return `${m.diskUsedGB} / ${m.diskTotalGB} GB`;
}

export function fmtUptime(m: ServerMetrics): string {
  if (!m.uptimeSeconds) return "n/a";
  const d = Math.floor(m.uptimeSeconds / 86400);
  const h = Math.floor((m.uptimeSeconds % 86400) / 3600);
  if (d > 0) return `${d}d ${h}h`;
  const mins = Math.floor((m.uptimeSeconds % 3600) / 60);
  return h > 0 ? `${h}h ${mins}m` : `${mins}m`;
}

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
  // The metrics are read over SSH on demand and can fail transiently (a cold
  // backend on first paint, a brief connection hiccup), which would otherwise
  // leave the card showing n/a on load. Retry a couple of times with a short
  // backoff so a transient miss does not stick. A 401 (not connected) is final.
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const resp = await fetch(`/api/servers/${serverId}/metrics`, { cache: "no-store", signal });
      if (resp.status === 401) return null;
      if (resp.ok) return (await resp.json()) as ServerMetrics;
    } catch {
      if (signal?.aborted) return null;
    }
    if (attempt < 2) await new Promise((r) => setTimeout(r, 400 * (attempt + 1)));
  }
  return null;
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

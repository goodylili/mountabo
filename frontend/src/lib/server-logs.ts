// A point-in-time read of a server's running container logs (from the Go backend
// over SSH). Each running container's lines are grouped under a header line of
// the form "==> <container> <==". Mirrors the backend /api/servers/{id}/logs
// JSON. The monitor reads this on demand for the open deployment's server.

export type ServerLogs = {
  lines: string[];
};

// fetchServerLogs reads a server's recent container logs. tail bounds how many
// lines come back (backend default 200, capped at 1000). Returns null on any
// failure (server not set up, unreachable) so the monitor shows an empty state.
export async function fetchServerLogs(
  serverId: string,
  tail = 200,
  signal?: AbortSignal,
): Promise<ServerLogs | null> {
  if (!serverId) return null;
  try {
    const resp = await fetch(`/api/servers/${serverId}/logs?tail=${tail}`, {
      cache: "no-store",
      signal,
    });
    if (!resp.ok) return null;
    const data = (await resp.json()) as Partial<ServerLogs>;
    return { lines: data.lines ?? [] };
  } catch {
    return null;
  }
}

// isLogHeader reports whether a log line is a container group header
// ("==> <container> <=="), so the viewer can style it as a section divider.
export function isLogHeader(line: string): boolean {
  return line.startsWith("==> ") && line.endsWith(" <==");
}

// logHeaderName pulls the container name out of a header line.
export function logHeaderName(line: string): string {
  return line.replace(/^==> /, "").replace(/ <==$/, "");
}

// LOG_TS matches docker's leading RFC3339 timestamp (added by
// `docker logs --timestamps`): a date, a time with optional fractional seconds,
// and a UTC "Z" or numeric offset, followed by the rest of the line.
const LOG_TS = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2}))\s+([\s\S]*)$/;

// splitLogTimestamp separates a line's leading timestamp from its message so the
// viewer can lead with and emphasise the date and time. A line with no
// recognisable timestamp (a wrapped continuation, or a header) comes back with
// an empty ts and the whole line as text.
export function splitLogTimestamp(line: string): { ts: string; text: string } {
  const clean = line.startsWith("﻿") ? line.slice(1) : line; // GitHub job logs prefix a BOM
  const m = LOG_TS.exec(clean);
  if (!m) return { ts: "", text: clean };
  return { ts: m[1], text: m[2] };
}

// formatLogTimestamp renders an ISO timestamp as "YYYY-MM-DD HH:MM:SS" in UTC,
// to seconds, for a compact, readable date and time. Returns the raw value when
// it cannot be parsed.
export function formatLogTimestamp(ts: string): string {
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}:${p(d.getUTCSeconds())}`;
}

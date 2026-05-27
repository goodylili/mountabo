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

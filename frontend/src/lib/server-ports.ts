// fetchListeningPorts returns the ports already in use on a server (as reported
// by the Go backend over SSH). It returns [] on any failure or for a server
// that isn't set up, so the UI simply shows no collisions rather than breaking.
export async function fetchListeningPorts(serverId: string, signal?: AbortSignal): Promise<number[]> {
  if (!serverId) return [];
  try {
    const resp = await fetch(`/api/servers/${serverId}/ports`, { cache: "no-store", signal });
    if (!resp.ok) return [];
    const data = (await resp.json()) as { listening?: number[] };
    return data.listening ?? [];
  } catch {
    return [];
  }
}

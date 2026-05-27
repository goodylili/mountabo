// AppHealth is a point-in-time read of whether a deployed app is responding
// (from the Go backend, which curls the app from its own server over SSH).
export type AppHealth = {
  // True when the app answered the probe (any HTTP response).
  reachable: boolean;
  // The HTTP status the app returned (0 when it did not answer at all).
  status: number;
  // The address that was probed (a loopback port, or the app's domain).
  target: string;
  // A short reason when the app is unreachable or no address could be derived.
  detail?: string;
};

// fetchAppHealth probes whether a deployment's app is up, by its app name.
// Returns null on any failure to reach the backend (server not set up,
// unreachable) so the card falls back to an "unknown" indicator rather than
// claiming the app is down. An app that is reachable-but-down comes back as a
// normal AppHealth with reachable:false.
export async function fetchAppHealth(app: string, signal?: AbortSignal): Promise<AppHealth | null> {
  if (!app) return null;
  try {
    const resp = await fetch(`/api/deployments/${encodeURIComponent(app)}/health`, {
      cache: "no-store",
      signal,
    });
    if (!resp.ok) return null;
    return (await resp.json()) as AppHealth;
  } catch {
    return null;
  }
}

// deleteDeployment forgets mountabo's tracking of a deployment by its app name.
// Returns true when the backend removed it (or it was already gone, 404), false
// on any other failure, so the caller can decide whether to drop the card.
export async function deleteDeployment(app: string): Promise<boolean> {
  try {
    const resp = await fetch(`/api/deployments/${encodeURIComponent(app)}`, { method: "DELETE" });
    return resp.ok || resp.status === 404;
  } catch {
    return false;
  }
}

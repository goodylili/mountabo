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
// deploymentPath builds the API path for a deployment's identity — (owner, repo,
// environment), which is unique, unlike an app name that two repos can share.
function deploymentPath(owner: string, repo: string, environment: string): string {
  return `/api/deployments/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/${encodeURIComponent(environment)}`;
}

export async function fetchAppHealth(
  owner: string,
  repo: string,
  environment: string,
  signal?: AbortSignal,
): Promise<AppHealth | null> {
  if (!owner || !repo || !environment) return null;
  try {
    const resp = await fetch(`${deploymentPath(owner, repo, environment)}/health`, {
      cache: "no-store",
      signal,
    });
    if (!resp.ok) return null;
    return (await resp.json()) as AppHealth;
  } catch {
    return null;
  }
}

// deleteDeployment tears down a deployment by its identity (stops the container,
// removes the deploy workflow, forgets the record). Returns true only when the
// backend confirms removal (2xx, or 404 meaning it was already gone), false on
// any other status or failure, so the caller drops the card only on real
// success. The teardown does SSH and GitHub work, so it is given a long timeout.
export async function deleteDeployment(owner: string, repo: string, environment: string): Promise<boolean> {
  try {
    const resp = await fetch(deploymentPath(owner, repo, environment), {
      method: "DELETE",
      signal: AbortSignal.timeout(90_000),
    });
    return resp.ok || resp.status === 404;
  } catch {
    return false;
  }
}

// clearDeployLock force-releases a stuck deploy lock for a deployment (the
// recovery action when a deploy wedged and left the target locked). Idempotent on
// the backend, so it returns true on any 2xx. Returns false only on a real
// failure to reach the backend, so the UI can surface an error.
export async function clearDeployLock(owner: string, repo: string, environment: string): Promise<boolean> {
  try {
    const resp = await fetch(`${deploymentPath(owner, repo, environment)}/lock`, { method: "DELETE" });
    return resp.ok;
  } catch {
    return false;
  }
}

// setDeploymentPort records the app's published port on a deployment, so a
// domain attached to it (which proxies to that port) associates with the
// deployment. Called after a domain is added. Returns false on any failure so the
// caller can surface it (the domain is live but not yet associated).
export async function setDeploymentPort(
  owner: string,
  repo: string,
  environment: string,
  port: number,
): Promise<boolean> {
  if (!owner || !repo || !environment || !Number.isFinite(port) || port < 1) return false;
  try {
    const resp = await fetch(`${deploymentPath(owner, repo, environment)}/port`, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ port }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

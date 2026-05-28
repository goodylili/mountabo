import { backendURL } from "@/lib/servers";

// getRepoBranches reads every branch on owner/repo from the Go backend, which
// pages through the GitHub API with the user's stored OAuth token. Returns []
// when the backend is unreachable or the repo is missing, so the picker shows
// an honest empty state and the operator can still type a branch by hand.
export async function getRepoBranches(
  owner: string,
  repo: string,
  signal?: AbortSignal,
): Promise<string[]> {
  try {
    const resp = await fetch(
      `${backendURL()}/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/branches`,
      { signal, cache: "no-store" },
    );
    if (!resp.ok) return [];
    const data = (await resp.json()) as { branches?: string[] };
    return Array.isArray(data.branches) ? data.branches : [];
  } catch {
    return [];
  }
}

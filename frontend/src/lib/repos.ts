import type { Source } from "@/lib/data";

type RepoResponse = {
  owner: string;
  name: string;
  fullName: string;
  private: boolean;
  defaultBranch: string;
  language: string;
  pushedAt: string;
  hasDocker: boolean;
  kind: "compose" | "docker" | "none";
};

function relativeTime(iso: string): string {
  if (!iso) return "n/a";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "n/a";
  const secs = Math.max(0, Math.floor((Date.now() - then) / 1000));
  const mins = Math.floor(secs / 60);
  const hrs = Math.floor(mins / 60);
  const days = Math.floor(hrs / 24);
  if (days > 0) return `${days}d ago`;
  if (hrs > 0) return `${hrs}h ago`;
  if (mins > 0) return `${mins}m ago`;
  return "just now";
}

// Fetches the connected account's repositories from the Go backend, which reads
// them from GitHub (public and private) using the token in the OS keychain.
// Returns [] when not connected or the backend is unreachable, so the UI shows
// an honest empty state rather than fabricated repos.
//
// GitHub repo listing is the slowest call in the app and occasionally fails
// transiently (a cold backend on first paint, a secondary rate-limit, a flaky
// connection). A single swallowed failure used to leave the page stuck on an
// empty list until a full reload, so we retry a couple of times with a short
// backoff and bound each attempt with a timeout. A 401 means "not connected",
// which is a real empty state, so we return immediately without retrying.
export async function getRepos(): Promise<Source[]> {
  const backend = process.env.MOUNTABO_BACKEND ?? "http://localhost:7778";
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const resp = await fetch(`${backend}/api/github/repos`, {
        cache: "no-store",
        signal: AbortSignal.timeout(15_000),
      });
      if (resp.status === 401) return [];
      if (!resp.ok) throw new Error(`repos request failed: ${resp.status}`);
      const repos = (await resp.json()) as RepoResponse[];
      return repos.map((r) => ({
        owner: r.owner,
        name: r.name,
        branch: r.defaultBranch || "main",
        updated: relativeTime(r.pushedAt),
        language: r.language ? r.language.toLowerCase() : "n/a",
        private: r.private,
        hasDocker: r.hasDocker,
        kind: r.kind ?? "none",
      }));
    } catch {
      if (attempt < 2) await new Promise((r) => setTimeout(r, 300 * (attempt + 1)));
    }
  }
  return [];
}

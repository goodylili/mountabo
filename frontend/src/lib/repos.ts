import type { Source } from "@/lib/data";

type RepoResponse = {
  owner: string;
  name: string;
  fullName: string;
  private: boolean;
  defaultBranch: string;
  language: string;
  pushedAt: string;
};

function relativeTime(iso: string): string {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
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
export async function getRepos(): Promise<Source[]> {
  const backend = process.env.MOUNTABO_BACKEND ?? "http://localhost:7778";
  try {
    const resp = await fetch(`${backend}/api/github/repos`, { cache: "no-store" });
    if (!resp.ok) return [];
    const repos = (await resp.json()) as RepoResponse[];
    return repos.map((r) => ({
      owner: r.owner,
      name: r.name,
      branch: r.defaultBranch || "main",
      updated: relativeTime(r.pushedAt),
      language: r.language ? r.language.toLowerCase() : "—",
      private: r.private,
    }));
  } catch {
    return [];
  }
}

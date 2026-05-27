// The live GitHub Actions run walkthrough for a repo + branch: the latest
// workflow run, its jobs, and each job's steps with per-step status. Mirrors the
// backend /api/github/run-steps JSON. The monitor renders this to walk the
// operator through every step GitHub runs and polls it while a run is going.

// GitHub's raw run/step lifecycle states.
export type GithubStatus = "queued" | "in_progress" | "completed" | "";
export type GithubConclusion = "success" | "failure" | "cancelled" | "skipped" | "timed_out" | "";

export type RunStep = {
  name: string;
  status: GithubStatus;
  conclusion: GithubConclusion;
  number: number;
};

export type RunJob = {
  name: string;
  status: GithubStatus;
  conclusion: GithubConclusion;
  htmlUrl: string;
  steps: RunStep[];
};

export type RunSteps = {
  runUrl: string;
  status: GithubStatus;
  conclusion: GithubConclusion;
  jobs: RunJob[];
};

const EMPTY: RunSteps = { runUrl: "", status: "", conclusion: "", jobs: [] };

// fetchRunSteps reads the latest workflow run's jobs and steps for a repo and
// branch. No side effects. Returns an empty walkthrough on any failure so the
// monitor falls back to an honest "no run yet" state rather than an error.
export async function fetchRunSteps(
  owner: string,
  repo: string,
  ref: string,
  signal?: AbortSignal,
): Promise<RunSteps> {
  if (!owner || !repo || !ref) return EMPTY;
  try {
    const qs = new URLSearchParams({ owner, repo, ref });
    const resp = await fetch(`/api/github/run-steps?${qs.toString()}`, { cache: "no-store", signal });
    if (!resp.ok) return EMPTY;
    const data = (await resp.json()) as Partial<RunSteps>;
    return {
      runUrl: data.runUrl ?? "",
      status: (data.status ?? "") as GithubStatus,
      conclusion: (data.conclusion ?? "") as GithubConclusion,
      jobs: data.jobs ?? [],
    };
  } catch {
    return EMPTY;
  }
}

// runActive reports whether a run is still going, so the monitor knows to keep
// polling. A queued or in-progress run (or one with no conclusion yet) is active.
export function runActive(r: RunSteps): boolean {
  if (!r.status) return false;
  return r.status !== "completed";
}

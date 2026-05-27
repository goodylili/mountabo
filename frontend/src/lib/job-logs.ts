// One GitHub Actions job's full log, fetched through the local API (which
// proxies the Go backend). The deploy walkthrough opens a job to show what each
// step printed and, when a step failed, the error itself.

// fetchJobLogs reads the plain-text log lines of a single job. Returns an empty
// list on any failure so the walkthrough can show an honest empty state.
export async function fetchJobLogs(
  owner: string,
  repo: string,
  jobId: number,
  signal?: AbortSignal,
): Promise<string[]> {
  if (!owner || !repo || !jobId) return [];
  try {
    const qs = new URLSearchParams({ owner, repo, jobId: String(jobId) });
    const resp = await fetch(`/api/github/job-logs?${qs.toString()}`, { cache: "no-store", signal });
    if (!resp.ok) return [];
    const data = (await resp.json()) as { lines?: string[] };
    return data.lines ?? [];
  } catch {
    return [];
  }
}

export type LogLineKind = "error" | "warning" | "group" | "command" | "normal";

// classifyJobLogLine reads GitHub's workflow-command markers (after any leading
// timestamp has been stripped) so the viewer can colour errors and warnings and
// treat ##[group] headers as section labels. It returns the line with the marker
// removed. ##[endgroup] lines carry no content and come back as an empty
// "group" line the caller can drop.
export function classifyJobLogLine(text: string): { kind: LogLineKind; text: string } {
  const m = /^##\[(error|warning|group|endgroup|command|debug|notice|section)\](.*)$/.exec(text);
  if (!m) return { kind: "normal", text };
  const marker = m[1];
  const rest = m[2];
  switch (marker) {
    case "error":
      return { kind: "error", text: rest };
    case "warning":
      return { kind: "warning", text: rest };
    case "group":
    case "section":
      return { kind: "group", text: rest };
    case "endgroup":
      return { kind: "group", text: "" };
    case "command":
      return { kind: "command", text: rest };
    default:
      return { kind: "normal", text: rest };
  }
}

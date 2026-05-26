// DetectedPort is one published port mountabo found in a repo's container
// config. When editable is true the host port is bound to an environment
// variable (envVar) that mountabo can set at deploy time, and host carries its
// default; otherwise the port is fixed in the file and shown read-only.
export type DetectedPort = {
  service: string;
  envVar: string;
  host: string;
  container: string;
  editable: boolean;
};

// normalizeDir strips a leading "./" and surrounding slashes so a root
// directory like "./" or "/app/" becomes "" or "app", matching what the backend
// expects (empty means the repo root).
export function normalizeDir(rootDir: string): string {
  return rootDir.replace(/^\.?\/*/, "").replace(/\/*$/, "");
}

// DetectResult is what port detection reports: the deploy strategy that fits
// the repo ("compose" when it has a Compose file, "docker" when only a
// Dockerfile, "" when neither) plus the published ports it declares.
export type DetectResult = { strategy: "compose" | "docker" | ""; ports: DetectedPort[] };

// fetchDetectedPorts asks the local API (which proxies the Go backend) for the
// strategy and ports of a repo at a given branch and directory. It returns an
// empty result when nothing is detectable so the caller can simply hide the
// ports section.
export async function fetchDetectedPorts(
  owner: string,
  repo: string,
  ref: string,
  dir: string,
  signal?: AbortSignal,
): Promise<DetectResult> {
  const params = new URLSearchParams({ owner, repo, ref, dir });
  const resp = await fetch(`/api/github/ports?${params}`, { cache: "no-store", signal });
  if (!resp.ok) return { strategy: "", ports: [] };
  return (await resp.json()) as DetectResult;
}

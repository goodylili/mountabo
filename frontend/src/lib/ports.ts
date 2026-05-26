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

// fetchDetectedPorts asks the local API (which proxies the Go backend) for the
// ports declared in a repo at a given branch and directory. It returns [] when
// nothing is detectable so the caller can simply hide the ports section.
export async function fetchDetectedPorts(
  owner: string,
  repo: string,
  ref: string,
  dir: string,
  signal?: AbortSignal,
): Promise<DetectedPort[]> {
  const params = new URLSearchParams({ owner, repo, ref, dir });
  const resp = await fetch(`/api/github/ports?${params}`, { cache: "no-store", signal });
  if (!resp.ok) return [];
  return (await resp.json()) as DetectedPort[];
}

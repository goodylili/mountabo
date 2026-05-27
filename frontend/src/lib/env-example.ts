// fetchEnvExampleKeys asks the local API (which proxies the Go backend) for the
// variable names declared in a repo's .env.example (or a common variant) at a
// given branch and directory, so the configure form can pre-fill the env var
// rows. It returns an empty list when there is no example file (or on any
// error), so the caller can simply leave the existing rows untouched.
export async function fetchEnvExampleKeys(
  owner: string,
  repo: string,
  ref: string,
  dir: string,
  signal?: AbortSignal,
): Promise<string[]> {
  const params = new URLSearchParams({ owner, repo, ref, dir });
  const resp = await fetch(`/api/github/env-example?${params}`, { cache: "no-store", signal });
  if (!resp.ok) return [];
  const data = (await resp.json()) as unknown;
  return Array.isArray(data) ? (data as string[]) : [];
}

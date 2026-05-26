// TreeEntry is one path in a repo's file tree, as returned by the Go backend
// (git/trees). dir is true for a directory, false for a file.
export type TreeEntry = { path: string; dir: boolean };

// fetchRepoTree loads a repo's full tree at a ref via the local API (which
// proxies the Go backend). It returns [] on any failure so the picker can fall
// back to a plain editable field rather than break the form.
export async function fetchRepoTree(
  owner: string,
  repo: string,
  ref: string,
  signal?: AbortSignal,
): Promise<TreeEntry[]> {
  if (!owner || !repo || !ref) return [];
  const params = new URLSearchParams({ owner, repo, ref });
  const resp = await fetch(`/api/github/tree?${params}`, { cache: "no-store", signal });
  if (!resp.ok) return [];
  return (await resp.json()) as TreeEntry[];
}

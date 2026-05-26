// The deploy artifacts the backend generates from a config: the workflow file
// (and its in-repo path), deploy.sh, and the secrets the deploy needs. This is
// exactly what a deploy would commit and set, so the configure UI previews it.

export type PreviewSecret = { name: string; managed: boolean };

export type DeployPreview = {
  workflowPath: string;
  workflow: string;
  deployScript: string;
  secrets: PreviewSecret[];
};

export type PreviewPort = { envVar: string; value: string; container: string };

export type PreviewRequest = {
  app: string;
  owner: string;
  repo: string;
  branch: string;
  environment?: string;
  strategy: string; // "compose" | "docker"
  rootDir: string;
  deployDir: string;
  ports: PreviewPort[];
  envVars: { key: string; value: string }[];
};

// fetchPreview asks the backend to generate the deploy artifacts. On a
// validation or backend error it returns { error } so the form can show why the
// preview can't render yet, rather than throwing.
export async function fetchPreview(
  req: PreviewRequest,
  signal?: AbortSignal,
): Promise<DeployPreview | { error: string }> {
  const resp = await fetch("/api/deploy/preview", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(req),
    cache: "no-store",
    signal,
  });
  const data = (await resp.json()) as DeployPreview | { error?: string };
  if (!resp.ok) return { error: ("error" in data && data.error) || "preview failed" };
  return data as DeployPreview;
}

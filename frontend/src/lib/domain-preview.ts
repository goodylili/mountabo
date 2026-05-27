import type { DomainFormValue } from "@/components/server-domains";

// The exact artifacts configuring a domain writes and runs on the server: the
// two nginx vhost configs (plain http, then http with TLS), the setup script,
// and where the config lives. Mirrors the backend DomainArtifacts JSON. The
// confirmation gate previews this so nothing reaches the server unseen.
export type DomainPreview = {
  sitePath: string;
  httpConfig: string;
  tlsConfig: string;
  script: string;
};

// Builds the preview query string from the same fields the add-domain stream
// uses, so the previewed steps match what configuring the domain will run.
function previewQuery(v: DomainFormValue): string {
  const qs = new URLSearchParams();
  qs.set("host", v.host);
  if (v.upstream) qs.set("upstream", v.upstream);
  if (v.aliases.length) qs.set("aliases", v.aliases.join(","));
  if (v.email) qs.set("email", v.email);
  if (v.staging) qs.set("staging", "1");
  return qs.toString();
}

// fetchDomainPreview asks the backend to render a domain's nginx config and
// setup script. No side effects. On a backend or validation error it returns
// { error } so the gate can say why the steps can't be shown yet.
export async function fetchDomainPreview(
  v: DomainFormValue,
  signal?: AbortSignal,
): Promise<DomainPreview | { error: string }> {
  const resp = await fetch(`/api/servers/domains/preview?${previewQuery(v)}`, {
    cache: "no-store",
    signal,
  });
  const data = (await resp.json()) as DomainPreview | { error?: string };
  if (!resp.ok) return { error: ("error" in data && data.error) || "preview failed" };
  return data as DomainPreview;
}

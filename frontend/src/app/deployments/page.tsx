import { Suspense } from "react";
import { Header } from "@/components/header";
import { MonitorView } from "@/components/monitor-view";
import { getDeployments } from "@/lib/deployments";
import { getServers } from "@/lib/servers";
import type { SetupOption } from "@/lib/servers";
import { backendURL } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function MonitorPage() {
  // Cookie read only, fast, so the header + skeleton stream out immediately.
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "deployments" }]} account={account} container="max-w-[1100px]" />
      {/* /api/deployments fans out to GitHub Actions for every deployment, so it
          is the slow call here. Suspense lets the shell paint now and streams the
          monitor in when ready, instead of blocking the whole page on it. */}
      <Suspense fallback={<MonitorSkeleton />}>
        <MonitorData />
      </Suspense>
    </div>
  );
}

async function MonitorData() {
  // Deployments + servers + the hardening catalog all feed the expanded card:
  // servers (full ServerView) drive the domain and monitoring panels, the
  // catalog resolves option ids to display names for the confirmation gate.
  const [deployments, servers, catalog] = await Promise.all([
    getDeployments(),
    getServers(),
    getOptionCatalog(),
  ]);

  const now = new Date();
  const stamp = now.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZone: "GMT",
  });

  return (
    <MonitorView deployments={deployments} servers={servers} catalog={catalog} stamp={`${stamp} GMT`} />
  );
}

// Loads the hardening option catalog (id/name/description) from the backend so
// the monitor can name monitoring tools and gate applies. Returns [] when the
// backend is unreachable, the monitor falls back to the option ids.
async function getOptionCatalog(): Promise<SetupOption[]> {
  try {
    const resp = await fetch(`${backendURL()}/api/servers/options`, { cache: "no-store" });
    if (!resp.ok) return [];
    return (await resp.json()) as SetupOption[];
  } catch {
    return [];
  }
}

function MonitorSkeleton() {
  return (
    <main className="mx-auto w-full max-w-[1100px] flex-1 px-4 py-10 sm:px-6 lg:px-8" aria-busy="true">
      <div className="h-3 w-24 animate-pulse rounded bg-surface-2" />
      <div className="mt-5 h-8 w-full max-w-72 animate-pulse rounded-lg bg-surface-2" />
      <div className="mt-8 space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-24 animate-pulse rounded-xl border border-line bg-surface" />
        ))}
      </div>
    </main>
  );
}

import { Suspense } from "react";
import { Header } from "@/components/header";
import { NewDeployment } from "@/components/new-deployment";
import { getRepos } from "@/lib/repos";
import { getServers } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function Home() {
  // Cookie read only, fast, so the header + skeleton stream out immediately.
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  const now = new Date();
  const stamp = `${now
    .toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })
    .toUpperCase()} · ${now
    .toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", timeZone: "GMT" })} GMT`;

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "new" }]} account={account} container="max-w-[1400px]" />
      {/* The repo list is the slow call (full GitHub pagination). Suspense lets
          the shell paint now and streams the deployment panel in when ready,
          instead of blocking the whole page on it. */}
      <Suspense fallback={<DeploySkeleton />}>
        <DeployData account={account} stamp={stamp} />
      </Suspense>
    </div>
  );
}

async function DeployData({ account, stamp }: { account: string | null; stamp: string }) {
  const [sources, servers] = await Promise.all([
    account ? getRepos() : Promise.resolve([]),
    getServers(),
  ]);
  return <NewDeployment sources={sources} servers={servers} account={account} stamp={stamp} />;
}

function DeploySkeleton() {
  return (
    <main className="mx-auto w-full max-w-[1400px] flex-1 px-8 py-10" aria-busy="true">
      <div className="h-3 w-24 animate-pulse rounded bg-surface-2" />
      <div className="mt-5 h-10 w-80 animate-pulse rounded-lg bg-surface-2" />
      <div className="mt-8 flex gap-2">
        <div className="h-9 w-28 animate-pulse rounded-lg bg-surface-2" />
        <div className="h-9 w-28 animate-pulse rounded-lg bg-surface" />
      </div>
      <div className="mt-6 space-y-2">
        {Array.from({ length: 7 }).map((_, i) => (
          <div key={i} className="h-16 animate-pulse rounded-xl border border-line bg-surface" />
        ))}
      </div>
    </main>
  );
}

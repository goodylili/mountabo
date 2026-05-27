import { Header } from "@/components/header";
import { NewDeployment } from "@/components/new-deployment";
import { getServers } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function Home() {
  // Cookie read plus a quick local server listing; the slow repository listing
  // is loaded (and cached for 12 hours) in the browser by NewDeployment, so the
  // page no longer blocks on full GitHub pagination before it can paint.
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;
  const servers = await getServers();

  const now = new Date();
  const stamp = `${now
    .toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" })
    .toUpperCase()} · ${now
    .toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", timeZone: "GMT" })} GMT`;

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "new" }]} account={account} container="max-w-[1100px]" />
      <NewDeployment servers={servers} account={account} stamp={stamp} />
    </div>
  );
}

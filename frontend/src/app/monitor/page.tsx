import { Header } from "@/components/header";
import { MonitorView } from "@/components/monitor-view";
import { getDeployments } from "@/lib/deployments";
import { getServers, toDisplayServer } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function MonitorPage() {
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  const [deployments, serverViews] = await Promise.all([getDeployments(), getServers()]);
  const servers = serverViews.map(toDisplayServer);

  const now = new Date();
  const stamp = now.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZone: "GMT",
  });

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "monitor" }]} account={account} container="max-w-[1100px]" />
      <MonitorView deployments={deployments} servers={servers} stamp={`${stamp} GMT`} />
    </div>
  );
}

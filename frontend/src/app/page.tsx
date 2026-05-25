import { Header } from "@/components/header";
import { NewDeployment } from "@/components/new-deployment";
import { servers, sources } from "@/lib/data";
import { getGithubConnection } from "@/lib/session";

export default async function Home() {
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
      <NewDeployment
        sources={sources}
        servers={servers}
        account={account}
        stamp={stamp}
      />
    </div>
  );
}

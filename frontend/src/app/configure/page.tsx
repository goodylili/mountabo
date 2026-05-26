import { redirect } from "next/navigation";
import { Header } from "@/components/header";
import { ConfigureView } from "@/components/configure-view";
import { deployKey, secretRows, workflowYaml } from "@/lib/preview";
import { getRepos } from "@/lib/repos";
import { getServers, toDisplayServer } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function ConfigurePage({
  searchParams,
}: {
  searchParams: Promise<{ repo?: string; server?: string }>;
}) {
  const { repo, server: serverId } = await searchParams;
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  // The deployment page needs a connected account, a real repo, and a real
  // (set-up) server. Resolve repos + servers concurrently; bail home if any of
  // the three can't be resolved.
  const [sources, servers] = await Promise.all([
    account ? getRepos() : Promise.resolve([]),
    getServers(),
  ]);

  const source = sources.find((s) => `${s.owner}/${s.name}` === repo);
  const real = servers.find((s) => s.id === serverId);
  if (!account || !source || !real) {
    redirect("/");
  }

  const server = toDisplayServer(real);
  const branch = source.branch;

  return (
    <div className="flex min-h-screen flex-col">
      <Header
        crumbs={[
          { label: "new", href: "/" },
          { label: `${source.name} / configure` },
        ]}
        back
        container="max-w-[1400px]"
      />
      <ConfigureView
        source={source}
        server={server}
        branch={branch}
        account={account}
        yaml={workflowYaml(source, server, branch)}
        secrets={secretRows(server)}
        deployKey={deployKey}
      />
    </div>
  );
}

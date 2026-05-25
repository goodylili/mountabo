import { Header } from "@/components/header";
import { ConfigureView } from "@/components/configure-view";
import { findServer, findSource } from "@/lib/data";
import { deployKey, secretRows, workflowYaml } from "@/lib/preview";
import { getGithubConnection } from "@/lib/session";

export default async function ConfigurePage({
  searchParams,
}: {
  searchParams: Promise<{ repo?: string; server?: string }>;
}) {
  const { repo, server: serverId } = await searchParams;
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  const source = findSource(repo);
  const server = findServer(serverId);
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

import { redirect } from "next/navigation";
import { Header } from "@/components/header";
import { ConfigureView } from "@/components/configure-view";
import { findServer } from "@/lib/data";
import { deployKey, secretRows, workflowYaml } from "@/lib/preview";
import { getRepos } from "@/lib/repos";
import { getGithubConnection } from "@/lib/session";

export default async function ConfigurePage({
  searchParams,
}: {
  searchParams: Promise<{ repo?: string; server?: string }>;
}) {
  const { repo, server: serverId } = await searchParams;
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  // Configuring requires a connected account, a chosen repo, and a real server.
  // Until the server-add flow exists there are no servers, so this redirects
  // home rather than rendering a screen built on data that doesn't exist.
  const server = findServer(serverId);
  if (!account || !repo || !server) {
    redirect("/");
  }

  const sources = await getRepos();
  const source = sources.find((s) => `${s.owner}/${s.name}` === repo);
  if (!source) {
    redirect("/");
  }

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

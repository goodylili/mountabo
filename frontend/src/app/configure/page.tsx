import { redirect } from "next/navigation";
import { Header } from "@/components/header";
import { ConfigureView } from "@/components/configure-view";
import type { Source } from "@/lib/data";
import { getServers, toDisplayServer } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

export default async function ConfigurePage({
  searchParams,
}: {
  searchParams: Promise<{ repo?: string; branch?: string; server?: string }>;
}) {
  const { repo, branch: branchParam, server: serverId } = await searchParams;
  const conn = await getGithubConnection();
  const account = conn.connected ? conn.login : null;

  // The repo + branch arrive in the URL from the picker (which already has them
  // loaded), so this page does not refetch the full repo listing, by far the
  // slowest call in the app, just to resolve one already-selected repo. Only
  // the server list is needed here, and it is a fast local read.
  const servers = await getServers();
  const slash = repo?.indexOf("/") ?? -1;
  const owner = repo && slash > 0 ? repo.slice(0, slash) : "";
  const name = repo && slash > 0 ? repo.slice(slash + 1) : "";
  const branch = branchParam ?? "";
  const real = servers.find((s) => s.id === serverId);
  if (!account || !owner || !name || !branch || !real) {
    redirect("/");
  }

  const server = toDisplayServer(real);
  const source: Source = { owner, name, branch, updated: "", language: "" };

  return (
    <div className="flex min-h-screen flex-col">
      <Header
        crumbs={[
          { label: "new", href: "/" },
          { label: `${source.name} / configure` },
        ]}
        back
        container="max-w-[1100px]"
      />
      <ConfigureView source={source} server={server} branch={branch} account={account} />
    </div>
  );
}

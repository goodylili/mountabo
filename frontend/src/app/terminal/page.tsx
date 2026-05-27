import { Header } from "@/components/header";
import { TerminalConsole } from "@/components/terminal-console";
import { getServers } from "@/lib/servers";
import { getGithubConnection } from "@/lib/session";

// The terminal page lets the operator run a single shell command on one of their
// set-up servers over SSH, and ask an AI helper to suggest a command for a plain
// English request. The AI helper only suggests; the human reviews and runs every
// command through a confirmation gate, so nothing is ever auto executed.
export default async function TerminalPage() {
  const [conn, servers] = await Promise.all([getGithubConnection(), getServers()]);
  const account = conn.connected ? conn.login : null;

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "terminal" }]} account={account} container="max-w-[1100px]" />
      <TerminalConsole servers={servers} />
    </div>
  );
}

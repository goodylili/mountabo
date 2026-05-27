// The self-hosted monitoring tools whose web UI mountabo can open from the
// deployment page. Each binds to the server's loopback, so it is reached through
// the SSH-tunneled reverse proxy on the backend (never exposed publicly).
//
// embed says whether the dashboard renders cleanly inside an iframe. Netdata is
// a static metrics dashboard and embeds well. Uptime Kuma and ntfy lean on
// websockets for their live data, which the request and response proxy does not
// carry, so we surface a clear in app panel with the proxied link and a one line
// note instead of a broken iframe. journald-persistent has no web UI at all, so
// it is intentionally not listed here.

export type DashboardTool = {
  // id is the hardening option id, matched against a server's options.
  id: string;
  label: string;
  // embed: render in an iframe (true) or show a link panel (false).
  embed: boolean;
  // note is the one line explanation shown under a link only dashboard.
  note: string;
};

export const DASHBOARD_TOOLS: DashboardTool[] = [
  {
    id: "netdata",
    label: "Netdata",
    embed: true,
    note: "real time CPU, memory, disk and network, embedded over the ssh tunnel.",
  },
  {
    id: "uptime-kuma",
    label: "Uptime Kuma",
    embed: false,
    note: "Uptime Kuma streams its status over websockets, which the proxy does not carry, so open it in a tab through the tunnel.",
  },
  {
    id: "ntfy",
    label: "ntfy",
    embed: false,
    note: "ntfy holds its alert feed open over websockets, so open it in a tab through the tunnel rather than embedding it.",
  },
];

// dashboardPath is the proxy URL for a tool's dashboard root on a server. The
// backend tunnels it to 127.0.0.1:<tool port> over SSH. The trailing slash keeps
// the tool's relative assets resolving under this path.
export function dashboardPath(serverId: string, toolId: string): string {
  return `/api/servers/${serverId}/dashboard/${toolId}/`;
}

// installedDashboards returns the dashboards whose tool is in the server's
// applied options, in catalog order, so only what is actually installed shows.
export function installedDashboards(options: string[] | null | undefined): DashboardTool[] {
  const set = new Set(options ?? []);
  return DASHBOARD_TOOLS.filter((t) => set.has(t.id));
}

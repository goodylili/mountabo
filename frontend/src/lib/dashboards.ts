// The self-hosted monitoring tools whose web UI mountabo can open from the
// deployment page. Each binds to the server's loopback, so it is reached through
// an SSH local port-forward tunnel the backend opens (never exposed publicly).
//
// Only Uptime Kuma is supported. It relies on websockets (socket.io) and expects
// to be served at the site root, neither of which a sub-path HTTP request and
// response proxy can satisfy. The backend instead opens a raw TCP `ssh -L`
// tunnel and returns a local http://127.0.0.1:<port>/ URL, which carries HTTP and
// websockets transparently and serves the tool at root, so it loads in an iframe
// straight from that local port.

export type DashboardTool = {
  // id is the hardening option id, matched against a server's options.
  id: string;
  label: string;
  // note is the one line explanation shown alongside the dashboard.
  note: string;
};

export const DASHBOARD_TOOLS: DashboardTool[] = [
  {
    id: "uptime-kuma",
    label: "Uptime Kuma",
    note: "self hosted uptime monitor, served at the root of a loopback SSH tunnel so its websockets connect directly.",
  },
];

// OpenedDashboard is the backend's reply to the open-tunnel call: the local URL
// the browser loads directly.
export type OpenedDashboard = { url: string };

// openDashboard asks the backend to open an SSH local port-forward tunnel to the
// tool on a server and returns the local http://127.0.0.1:<port>/ URL to load,
// or null on any failure (server not set up, tool not installed, ssh failure).
export async function openDashboard(
  serverId: string,
  toolId: string,
  signal?: AbortSignal,
): Promise<OpenedDashboard | null> {
  if (!serverId || !toolId) return null;
  try {
    const resp = await fetch(`/api/servers/${serverId}/dashboard/${toolId}/open`, {
      method: "POST",
      cache: "no-store",
      signal,
    });
    if (!resp.ok) return null;
    const data = (await resp.json()) as OpenedDashboard;
    return data.url ? data : null;
  } catch {
    return null;
  }
}

// installedDashboards returns the dashboards whose tool is in the server's
// applied options, in catalog order, so only what is actually installed shows.
export function installedDashboards(options: string[] | null | undefined): DashboardTool[] {
  const set = new Set(options ?? []);
  return DASHBOARD_TOOLS.filter((t) => set.has(t.id));
}

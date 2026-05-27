import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Opens an SSH local port-forward tunnel to a server's loopback monitoring
// dashboard (Uptime Kuma) on the Go backend, which binds a listener to its own
// loopback and forwards raw TCP over the server's SSH connection. The backend
// returns {"url":"http://127.0.0.1:<port>/"}, which the browser then loads
// directly (in an iframe or a tab): because the tunnel carries raw TCP, the
// tool's HTTP and websockets both work and it is served at the root of that
// local port. This handler only relays the small JSON open call, not the
// dashboard traffic itself.
export async function POST(_request: Request, { params }: { params: Promise<{ id: string; tool: string }> }) {
  const { id, tool } = await params;
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/servers/${id}/dashboard/${tool}/open`, {
      method: "POST",
      cache: "no-store",
      signal: AbortSignal.timeout(25_000),
    });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
  const body = await resp.text();
  return new NextResponse(body, {
    status: resp.status,
    headers: { "content-type": "application/json" },
  });
}

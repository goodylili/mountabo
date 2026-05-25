import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "add server" to the Go backend, which SSHes in with the root password,
// probes the specs, and stores the server. The root password only transits the
// local backend; it is never persisted by the frontend.
export async function POST(request: Request) {
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/servers`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: await request.text(),
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

import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies domain-preview generation to the Go backend, which renders the exact
// nginx vhost configs and setup script for a domain without touching a server.
// No side effects: the backend generates from the query alone. The confirmation
// gate calls this to show exactly what configuring a domain would write and run.
export async function GET(request: Request) {
  const search = new URL(request.url).search;
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/servers/domains/preview${search}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(15_000),
    });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
  const text = await resp.text();
  return new NextResponse(text, {
    status: resp.status,
    headers: { "content-type": "application/json" },
  });
}

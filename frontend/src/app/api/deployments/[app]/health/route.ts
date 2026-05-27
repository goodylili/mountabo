import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the app health probe to the Go backend, which curls the deployed app
// from its own server over SSH (its loopback published port, or its domain) and
// reports up/down plus the HTTP status. The monitor calls this on demand.
export async function GET(_request: Request, { params }: { params: Promise<{ app: string }> }) {
  const { app } = await params;
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/deployments/${encodeURIComponent(app)}/health`, {
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

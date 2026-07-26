import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "clear stuck deploy lock" to the Go backend, which force-releases the
// in-process deploy lock for the app so a new deploy can start. Idempotent:
// clearing an app that is not locked still succeeds.
export async function DELETE(_request: Request, { params }: { params: Promise<{ app: string }> }) {
  const { app } = await params;
  try {
    const resp = await fetch(`${backendURL()}/api/deployments/${encodeURIComponent(app)}/lock`, {
      method: "DELETE",
    });
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

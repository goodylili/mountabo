import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "clear stuck deploy lock" to the Go backend, which force-releases the
// in-process deploy lock for the app's environment so a new deploy can start.
// Idempotent: clearing a target that is not locked still succeeds.
export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ app: string; environment: string }> },
) {
  const { app, environment } = await params;
  try {
    const resp = await fetch(
      `${backendURL()}/api/deployments/${encodeURIComponent(app)}/${encodeURIComponent(environment)}/lock`,
      { method: "DELETE" },
    );
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

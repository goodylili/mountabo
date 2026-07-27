import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "record the app's published port" to the Go backend, so a domain
// attached to this deployment (which proxies to that port) associates with it.
export async function PUT(
  request: Request,
  { params }: { params: Promise<{ app: string; environment: string }> },
) {
  const { app, environment } = await params;
  try {
    const resp = await fetch(
      `${backendURL()}/api/deployments/${encodeURIComponent(app)}/${encodeURIComponent(environment)}/port`,
      { method: "PUT", headers: { "content-type": "application/json" }, body: await request.text() },
    );
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

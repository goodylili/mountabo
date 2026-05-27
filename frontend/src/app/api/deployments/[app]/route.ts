import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "forget a deployment" to the Go backend, which removes mountabo's
// tracking of the deployment (its record + append-only deploy history). It does
// not stop the running container or remove the committed workflow.
export async function DELETE(_request: Request, { params }: { params: Promise<{ app: string }> }) {
  const { app } = await params;
  try {
    const resp = await fetch(`${backendURL()}/api/deployments/${encodeURIComponent(app)}`, {
      method: "DELETE",
    });
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

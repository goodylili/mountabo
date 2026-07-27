import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "forget a deployment" (an app's environment) to the Go backend, which
// removes mountabo's tracking of the deployment (its record + append-only deploy
// history). It does not stop the running container or remove the committed workflow.
export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ app: string; environment: string }> },
) {
  const { app, environment } = await params;
  try {
    const resp = await fetch(
      `${backendURL()}/api/deployments/${encodeURIComponent(app)}/${encodeURIComponent(environment)}`,
      { method: "DELETE" },
    );
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

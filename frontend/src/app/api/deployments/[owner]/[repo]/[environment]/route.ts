import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "forget a deployment" (owner/repo/environment) to the Go backend, which
// removes mountabo's tracking of the deployment (its record + append-only deploy
// history). It does not stop the running container or remove the committed workflow.
export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ owner: string; repo: string; environment: string }> },
) {
  const { owner, repo, environment } = await params;
  try {
    const resp = await fetch(
      `${backendURL()}/api/deployments/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/${encodeURIComponent(environment)}`,
      { method: "DELETE" },
    );
    return new NextResponse(null, { status: resp.status });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
}

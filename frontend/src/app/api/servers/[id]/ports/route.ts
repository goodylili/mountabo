import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the server port check to the Go backend, which SSHes into the server
// as the mountabo user and lists the ports already listening. The configure UI
// uses it to flag a deploy port that would collide; the port stays editable.
export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/servers/${id}/ports`, {
      cache: "no-store",
      signal: AbortSignal.timeout(20_000),
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

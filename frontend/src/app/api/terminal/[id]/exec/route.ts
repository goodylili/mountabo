import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies "run one command on a server" to the Go backend, which SSHes in as the
// mountabo user with the stored key, runs the command under a bounded timeout,
// and returns the combined output and exit code. The frontend never holds an SSH
// key, so this always goes through the backend. A longer client timeout than the
// usual proxies because the command itself may take a while.
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const body = await request.text();
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/servers/${encodeURIComponent(id)}/exec`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(90_000),
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

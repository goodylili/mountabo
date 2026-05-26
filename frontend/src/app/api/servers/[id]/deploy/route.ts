import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the live deploy stream from the Go backend. Deploy is a POST (its body
// carries env var values that become secrets, so they must not travel in a URL),
// and the backend answers with Server-Sent Events; we forward the body upstream
// and stream the events straight back so progress arrives in real time.
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const body = await request.text();

  let upstream: Response;
  try {
    upstream = await fetch(`${backendURL()}/api/servers/${id}/deploy`, {
      method: "POST",
      headers: { "content-type": "application/json", accept: "text/event-stream" },
      body,
    });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache, no-transform",
      connection: "keep-alive",
    },
  });
}

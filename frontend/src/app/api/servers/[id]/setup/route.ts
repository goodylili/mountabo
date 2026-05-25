import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the live bootstrap stream from the Go backend. The browser opens an
// EventSource to this same-origin route; we stream the backend's Server-Sent
// Events straight through so setup progress arrives in real time.
export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  const upstream = await fetch(`${backendURL()}/api/servers/${id}/setup`, {
    headers: { accept: "text/event-stream" },
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache, no-transform",
      connection: "keep-alive",
    },
  });
}

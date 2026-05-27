import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the live "add domain" stream from the backend. The browser opens an
// EventSource here with ?host=&upstream=&aliases=&email=&staging=; we stream the
// backend's SSE (nginx install + certbot progress) straight through.
export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const search = new URL(request.url).search;

  const upstream = await fetch(`${backendURL()}/api/servers/${id}/domains/add${search}`, {
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

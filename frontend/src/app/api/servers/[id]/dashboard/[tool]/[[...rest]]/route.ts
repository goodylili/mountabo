import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies a server's loopback monitoring dashboard (Netdata, Uptime Kuma, ntfy)
// to the Go backend, which tunnels the request over the server's SSH connection
// and dials 127.0.0.1:<tool port> there. The frontend never talks to the tool
// directly: the tool binds to the server's loopback and is only reachable
// through the SSH tunnel the backend owns. This handler simply relays the
// request and the tool's response (status, headers, body) through, so an iframe
// or an opened tab pointed at this path renders the dashboard and its assets.
//
// One handler serves every method and the whole asset tree under a tool, since
// dashboards load css/js/api calls relative to their root. Hop-by-hop headers
// are dropped by the backend; here we relay the body and content type verbatim.
async function proxy(
  request: Request,
  { params }: { params: Promise<{ id: string; tool: string; rest?: string[] }> },
) {
  const { id, tool, rest } = await params;
  const suffix = rest && rest.length > 0 ? `/${rest.map(encodeURIComponent).join("/")}` : "/";
  const search = new URL(request.url).search;
  const url = `${backendURL()}/api/servers/${id}/dashboard/${tool}${suffix}${search}`;

  // Forward the method and body so the tool's own UI behaves; GET/HEAD carry no
  // body. duplex is required when streaming a request body in the fetch spec.
  const hasBody = request.method !== "GET" && request.method !== "HEAD";
  let resp: Response;
  try {
    resp = await fetch(url, {
      method: request.method,
      headers: request.headers,
      body: hasBody ? request.body : undefined,
      ...(hasBody ? { duplex: "half" } : {}),
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(30_000),
    } as RequestInit);
  } catch {
    return new Response("could not reach the dashboard over the mountabo backend", { status: 502 });
  }

  // Relay the tool's response as-is. Strip content-length (the body may be
  // re-chunked) and any framing header that would stop the dashboard embedding.
  const headers = new Headers(resp.headers);
  headers.delete("content-length");
  headers.delete("content-encoding");
  headers.delete("x-frame-options");
  headers.delete("content-security-policy");
  return new Response(resp.body, { status: resp.status, headers });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const HEAD = proxy;
export const OPTIONS = proxy;

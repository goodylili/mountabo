import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies an AI command-helper request to the Go backend, which holds the
// Anthropic key and calls Claude (with prompt caching). When the key is unset
// the backend replies 200 with configured=false and a hint, never a 500, so the
// UI can show "AI is not configured". The suggestion is advisory only: the
// operator reviews and runs it through the exec route, nothing is auto executed.
export async function POST(request: Request) {
  const body = await request.text();
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/ai/command`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(40_000),
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

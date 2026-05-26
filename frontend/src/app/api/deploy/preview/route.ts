import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies deploy-preview generation to the Go backend, which is the single
// source of truth for the committed workflow + deploy.sh + secret list. No side
// effects: the backend generates from config alone. The configure UI calls this
// as the operator edits, so the preview always matches what a deploy commits.
export async function POST(request: Request) {
  const body = await request.text();
  let resp: Response;
  try {
    resp = await fetch(`${backendURL()}/api/deploy/preview`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(15_000),
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

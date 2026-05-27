import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the live GitHub Actions run walkthrough to the Go backend, which reads
// the latest workflow run for a repo + branch (using the keychain token) and
// returns its jobs and their per-step status. The monitor polls this while a
// deployment's run is in progress to walk the operator through every step
// GitHub runs (checkout, copy deploy.sh, ssh run deploy.sh, and so on).
export async function GET(request: Request) {
  const incoming = new URL(request.url);
  const target = new URL(`${backendURL()}/api/github/run-steps`);
  target.search = incoming.search; // forward owner, repo, ref

  let resp: Response;
  try {
    resp = await fetch(target, { cache: "no-store", signal: AbortSignal.timeout(15_000) });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
  const body = await resp.text();
  return new NextResponse(body, {
    status: resp.status,
    headers: { "content-type": "application/json" },
  });
}

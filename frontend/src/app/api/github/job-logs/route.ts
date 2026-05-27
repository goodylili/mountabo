import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies a single job's log to the Go backend, which resolves the GitHub
// signed log URL with the keychain token and returns the plain-text lines. The
// deploy walkthrough calls this when the operator opens a job, to show what each
// step printed and what failed.
export async function GET(request: Request) {
  const incoming = new URL(request.url);
  const target = new URL(`${backendURL()}/api/github/job-logs`);
  target.search = incoming.search; // forward owner, repo, jobId

  let resp: Response;
  try {
    resp = await fetch(target, { cache: "no-store", signal: AbortSignal.timeout(20_000) });
  } catch {
    return NextResponse.json({ error: "couldn't reach the mountabo backend" }, { status: 502 });
  }
  const body = await resp.text();
  return new NextResponse(body, {
    status: resp.status,
    headers: { "content-type": "application/json" },
  });
}

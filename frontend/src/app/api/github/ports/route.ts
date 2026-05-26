import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies port detection to the Go backend, which reads the repo's compose file
// (or Dockerfile) from GitHub using the keychain token and returns the published
// ports it found. The configure UI calls this whenever the repo, branch, or root
// directory changes, so it always offers the project's real ports.
export async function GET(request: Request) {
  const incoming = new URL(request.url);
  const target = new URL(`${backendURL()}/api/github/ports`);
  target.search = incoming.search; // forward owner, repo, ref, dir

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

import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies env var discovery to the Go backend, which reads the repo's
// .env.example (or a common variant) from GitHub using the keychain token and
// returns the variable names it declares. The configure UI calls this when the
// repo, branch, or root directory changes, to pre-fill the env var rows.
export async function GET(request: Request) {
  const incoming = new URL(request.url);
  const target = new URL(`${backendURL()}/api/github/env-example`);
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

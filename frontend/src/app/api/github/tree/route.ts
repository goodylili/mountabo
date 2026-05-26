import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies repo-tree listing to the Go backend, which reads the whole tree from
// GitHub (git/trees, recursive) using the keychain token. The configure UI's
// directory/file picker calls this once per repo+ref and filters client-side.
export async function GET(request: Request) {
  const incoming = new URL(request.url);
  const target = new URL(`${backendURL()}/api/github/tree`);
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

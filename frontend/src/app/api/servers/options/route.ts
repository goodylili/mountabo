import { NextResponse } from "next/server";
import { backendURL } from "@/lib/servers";

export const dynamic = "force-dynamic";

// Proxies the catalog of opt-in hardening steps (id/name/description) from the
// backend so the setup UI can present them for the operator to choose.
export async function GET() {
  try {
    const resp = await fetch(`${backendURL()}/api/servers/options`, { cache: "no-store" });
    const body = await resp.text();
    return new NextResponse(body, { status: resp.status, headers: { "content-type": "application/json" } });
  } catch {
    return NextResponse.json([], { status: 200 });
  }
}

import { NextResponse } from "next/server";
import { GH_COOKIE } from "@/lib/session";

export const dynamic = "force-dynamic";

// Disconnecting clears the local session cookie AND tells the backend to delete
// the token from the OS keychain, so "disconnect" actually revokes mountabo's
// stored access rather than just hiding it in the UI.
export async function GET(request: Request) {
  const origin = new URL(request.url).origin;
  const backend = process.env.MOUNTABO_BACKEND ?? "http://localhost:7778";

  try {
    await fetch(`${backend}/api/github/token`, { method: "DELETE" });
  } catch {
    // Backend unreachable, still clear the local session below.
  }

  const res = NextResponse.redirect(`${origin}/connect`);
  res.cookies.delete(GH_COOKIE);
  return res;
}

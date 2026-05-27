import { NextResponse } from "next/server";
import { buildAuthorizeUrl } from "@/lib/github";
import { GH_STATE_COOKIE } from "@/lib/session";

export const dynamic = "force-dynamic";

// Kicks off the OAuth App authorization flow. Token exchange + keychain
// storage are the Go backend's job; this handler only builds the authorize URL
// and sets a CSRF state cookie. Without GITHUB_CLIENT_ID there is nothing real
// to do, so we send the user back with a config error rather than faking a
// connection.
export async function GET(request: Request) {
  const origin = new URL(request.url).origin;
  const clientId = process.env.GITHUB_CLIENT_ID;
  if (!clientId) {
    return NextResponse.redirect(`${origin}/connect?error=config`);
  }

  const redirectUri = `${origin}/api/github/callback`;
  const state = crypto.randomUUID();

  const res = NextResponse.redirect(buildAuthorizeUrl({ clientId, redirectUri, state }));
  res.cookies.set(GH_STATE_COOKIE, state, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    maxAge: 600,
  });
  return res;
}

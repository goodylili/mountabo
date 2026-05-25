import { NextResponse } from "next/server";
import { buildAuthorizeUrl } from "@/lib/github";
import { GH_STATE_COOKIE } from "@/lib/session";

export const dynamic = "force-dynamic";

// Kicks off the GitHub OAuth web flow with exactly the scopes in lib/github.ts.
// Token exchange + keychain storage are the Go backend's job; this handler only
// builds the authorize URL and sets a CSRF state cookie. With no GITHUB_CLIENT_ID
// configured we short-circuit to the callback in demo mode so the local UI is
// fully navigable without a registered GitHub OAuth app.
export async function GET(request: Request) {
  const origin = new URL(request.url).origin;
  const redirectUri = `${origin}/api/github/callback`;
  const state = crypto.randomUUID();
  const clientId = process.env.GITHUB_CLIENT_ID;

  const target = clientId
    ? buildAuthorizeUrl({ clientId, redirectUri, state })
    : `${origin}/api/github/callback?demo=1&state=${state}`;

  const res = NextResponse.redirect(target);
  res.cookies.set(GH_STATE_COOKIE, state, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    maxAge: 600,
  });
  return res;
}

import { NextResponse } from "next/server";
import { ACCOUNT_LOGIN } from "@/lib/data";
import { GH_COOKIE, GH_STATE_COOKIE } from "@/lib/session";

export const dynamic = "force-dynamic";

// GitHub redirects back here after the user approves. We verify the CSRF state,
// then hand the authorization `code` to the Go backend, which exchanges it for a
// token (using the client secret) and stores that token in the OS keychain: the
// secret never touches the browser. The backend returns the connected login,
// which we persist in a session cookie so the UI can show the account.
//
// `demo=1` is only ever set by the authorize route when no GITHUB_CLIENT_ID is
// configured; with real credentials present the flow always goes through the
// backend, and a missing/failed backend is surfaced as an error (no silent
// fake-success).
export async function GET(request: Request) {
  const url = new URL(request.url);
  const origin = url.origin;
  const code = url.searchParams.get("code");
  const state = url.searchParams.get("state");
  const demo = url.searchParams.get("demo") === "1";

  const cookieState = request.headers
    .get("cookie")
    ?.match(new RegExp(`${GH_STATE_COOKIE}=([^;]+)`))?.[1];

  if (!state || state !== cookieState) {
    return NextResponse.redirect(`${origin}/connect?error=state`);
  }

  let login = ACCOUNT_LOGIN;

  if (!demo) {
    if (!code) {
      return NextResponse.redirect(`${origin}/connect?error=exchange`);
    }

    const backend = process.env.MOUNTABO_BACKEND ?? "http://localhost:7778";
    let resp: Response;
    try {
      resp = await fetch(`${backend}/api/github/exchange`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ code, redirectUri: `${origin}/api/github/callback` }),
      });
    } catch {
      // The Go backend holds the client secret and the keychain; without it the
      // connection genuinely cannot be completed.
      return NextResponse.redirect(`${origin}/connect?error=backend`);
    }

    if (!resp.ok) {
      return NextResponse.redirect(`${origin}/connect?error=exchange`);
    }

    const data = (await resp.json()) as { login?: string };
    if (!data.login) {
      return NextResponse.redirect(`${origin}/connect?error=exchange`);
    }
    login = data.login;
  }

  const res = NextResponse.redirect(`${origin}/`);
  res.cookies.set(GH_COOKIE, login, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });
  res.cookies.delete(GH_STATE_COOKIE);
  return res;
}

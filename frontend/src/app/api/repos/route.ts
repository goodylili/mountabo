import { NextResponse } from "next/server";
import { getRepos } from "@/lib/repos";

export const dynamic = "force-dynamic";

// Serves the connected account's repositories to the browser so the deploy
// picker can cache them in localStorage (see lib/repo-cache) instead of paying
// for the full GitHub listing on every visit. getRepos handles the keychain
// token, pagination, and transient-failure retries server-side, and returns an
// empty list (never an error) when GitHub is not connected.
export async function GET() {
  const repos = await getRepos();
  return NextResponse.json(repos);
}

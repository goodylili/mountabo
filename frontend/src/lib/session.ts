import { cookies } from "next/headers";
import { ACCOUNT_LOGIN } from "@/lib/data";

export const GH_COOKIE = "mountabo_gh_login";
export const GH_STATE_COOKIE = "mountabo_gh_state";

export type GithubConnection =
  | { connected: true; login: string; demo: boolean }
  | { connected: false };

// Reads the connected GitHub account from the session cookie set by the OAuth
// callback. When no OAuth app is configured (local demo, no GITHUB_CLIENT_ID),
// we treat the session as connected to the demo account so the populated UI is
// reviewable without a real GitHub app.
export async function getGithubConnection(): Promise<GithubConnection> {
  const login = (await cookies()).get(GH_COOKIE)?.value;
  if (login) return { connected: true, login, demo: false };
  if (!process.env.GITHUB_CLIENT_ID) {
    return { connected: true, login: ACCOUNT_LOGIN, demo: true };
  }
  return { connected: false };
}

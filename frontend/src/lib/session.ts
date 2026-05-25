import { cookies } from "next/headers";

export const GH_COOKIE = "mountabo_gh_login";
export const GH_STATE_COOKIE = "mountabo_gh_state";

export type GithubConnection =
  | { connected: true; login: string }
  | { connected: false };

// Reads the connected GitHub account from the session cookie the OAuth callback
// sets after the Go backend exchanges the code and stores the token. No cookie
// means not connected — there is no demo/default account.
export async function getGithubConnection(): Promise<GithubConnection> {
  const login = (await cookies()).get(GH_COOKIE)?.value;
  return login ? { connected: true, login } : { connected: false };
}

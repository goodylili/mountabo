// Single source of truth for the GitHub access mountabo asks for.
//
// mountabo connects as a classic OAuth App via the web authorization flow. The
// `repo` scope returns ALL of a user's repositories, public and private, owned
// and organization, regardless of any per-repo install (unlike a GitHub App,
// which only sees repos it was installed on). `workflow` is required to write
// the deploy workflow file. The list below is the human-readable view of the
// scopes; the connect screen renders it verbatim so the user sees what they
// grant.

// GITHUB_SCOPES are the OAuth scopes requested at authorize time. `repo` covers
// reading every repo plus writing deploy keys and Actions secrets; `workflow`
// is needed to commit .github/workflows files.
export const GITHUB_SCOPES = ["repo", "workflow"];

export type GithubPermission = {
  /** The OAuth scope name. */
  permission: string;
  label: string;
  access: "read" | "write" | "read + write";
  /** Why mountabo needs it, in terms of a concrete action. */
  reason: string;
};

export const GITHUB_PERMISSIONS: GithubPermission[] = [
  {
    permission: "repo",
    label: "repositories",
    access: "read + write",
    reason:
      "list all your repositories (public and private), commit the deploy script + workflow, add the read-only deploy key, and set the Actions secrets the workflow reads.",
  },
  {
    permission: "workflow",
    label: "actions workflows",
    access: "write",
    reason: "create and update the deploy workflow under .github/workflows.",
  },
];

export const GITHUB_AUTHORIZE_URL = "https://github.com/login/oauth/authorize";

/**
 * Build the GitHub OAuth App authorization URL. Called server-side from the
 * authorize route. The requested scopes (GITHUB_SCOPES) determine the access of
 * the resulting token, `repo` is what makes every repository visible.
 */
export function buildAuthorizeUrl(opts: {
  clientId: string;
  redirectUri: string;
  state: string;
}): string {
  const params = new URLSearchParams({
    client_id: opts.clientId,
    redirect_uri: opts.redirectUri,
    state: opts.state,
    scope: GITHUB_SCOPES.join(" "),
    allow_signup: "false",
  });
  return `${GITHUB_AUTHORIZE_URL}?${params.toString()}`;
}

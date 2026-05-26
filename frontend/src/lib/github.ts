// Single source of truth for the GitHub access mountabo asks for.
//
// mountabo connects as a GitHub App via the user-to-server web authorization
// flow. Unlike classic OAuth apps, a GitHub App takes no `scope` parameter, the
// access it has is fixed by the App's configured permissions. The list below is
// the human-readable view of those permissions, each mapping to one of mountabo's
// on-repo actions: add a deploy key, write the deploy workflow file, and set the
// Actions secrets the workflow reads. The connect screen renders it verbatim so
// the user sees exactly what they are granting.

export type GithubPermission = {
  /** The GitHub App permission name. */
  permission: string;
  label: string;
  access: "read" | "write" | "read + write";
  /** Why mountabo needs it, in terms of a concrete action. */
  reason: string;
};

export const GITHUB_PERMISSIONS: GithubPermission[] = [
  {
    permission: "contents",
    label: "repository contents",
    access: "read + write",
    reason:
      "read your repository and write .github/workflows/mountabo-deploy.yml into it.",
  },
  {
    permission: "administration",
    label: "repository administration",
    access: "read + write",
    reason: "add the read-only SSH deploy key to your repository.",
  },
  {
    permission: "secrets",
    label: "actions secrets",
    access: "read + write",
    reason:
      "store the three Actions secrets the workflow needs (SERVER_IP, SERVER_USER, SSH_PRIVATE_KEY).",
  },
  {
    permission: "workflows",
    label: "actions workflows",
    access: "write",
    reason: "create and update the deploy workflow file under .github/workflows.",
  },
  {
    permission: "metadata",
    label: "repository metadata",
    access: "read",
    reason: "the baseline read access GitHub requires of every app.",
  },
];

export const GITHUB_AUTHORIZE_URL = "https://github.com/login/oauth/authorize";

/**
 * Build the GitHub App user-authorization URL. Called server-side from the
 * authorize route. A GitHub App takes no `scope` parameter (permissions come
 * from the App configuration, see GITHUB_PERMISSIONS), so we send only the
 * client_id, redirect_uri, and a CSRF state.
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
    allow_signup: "false",
  });
  return `${GITHUB_AUTHORIZE_URL}?${params.toString()}`;
}

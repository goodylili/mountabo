// Single source of truth for the GitHub access mountabo asks for.
//
// mountabo connects via the GitHub OAuth web flow (classic scopes), matching the
// product's three on-repo actions: add a deploy key, write the deploy workflow
// file, and set the Actions secrets the workflow reads. Every scope below maps
// to one of those actions: nothing broader is requested. The connect screen
// renders this list verbatim so the user sees exactly what they are granting.

export type GithubScope = {
  /** The literal OAuth scope string sent to GitHub. */
  scope: string;
  label: string;
  access: "read" | "write" | "read + write";
  /** Why mountabo needs it, in terms of a concrete action. */
  reason: string;
};

export const GITHUB_SCOPES: GithubScope[] = [
  {
    scope: "repo",
    label: "repositories",
    access: "read + write",
    reason:
      "list your repositories, add a read-only deploy key, and store the three Actions secrets the workflow needs (SERVER_IP, SERVER_USER, SSH_PRIVATE_KEY).",
  },
  {
    scope: "workflow",
    label: "actions workflows",
    access: "write",
    reason:
      "create and update .github/workflows/mountabo-deploy.yml: GitHub requires this scope to push files under .github/workflows.",
  },
  {
    scope: "read:user",
    label: "your profile",
    access: "read",
    reason: "show which GitHub account is connected. mountabo never reads private profile data.",
  },
];

export const GITHUB_AUTHORIZE_URL = "https://github.com/login/oauth/authorize";

/** Space-delimited scope string for the OAuth `scope` parameter. */
export function scopeParam(): string {
  return GITHUB_SCOPES.map((s) => s.scope).join(" ");
}

/** Build the GitHub authorize URL. Called server-side from the route handler. */
export function buildAuthorizeUrl(opts: {
  clientId: string;
  redirectUri: string;
  state: string;
}): string {
  const params = new URLSearchParams({
    client_id: opts.clientId,
    redirect_uri: opts.redirectUri,
    scope: scopeParam(),
    state: opts.state,
    allow_signup: "false",
  });
  return `${GITHUB_AUTHORIZE_URL}?${params.toString()}`;
}

package ssh

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.ContainerTeardown = (*Client)(nil)

// teardownScriptFmt stops and removes every container belonging to an app,
// covering both deploy strategies and three possible compose project-name
// variants the containers may carry as the
// "com.docker.compose.project=<value>" label:
//
//	%[1]s = "<app>-<branch>" (current convention pinned by composeScript),
//	%[2]s = "<branch>"        (compose's default before we pinned the name:
//	                          the deploy directory basename equalled branch),
//	%[3]s = "<app>"           (any older single-app pinning).
//
// It also docker-rm's plain containers named %[1]s/%[2]s/%[3]s for the
// docker-run strategy. Best-effort by design: a server without docker, or with
// no matching container, is a clean no-op, not a failure. The leading
// `command -v docker` guard prints an explanatory line and exits 0 when docker
// is absent so the caller never sees an error; the per-project loop only runs
// docker rm when ids were found, and `docker rm -f` is `|| true` so "no such
// container" never fails the script. All three values are single-quote-escaped
// by the caller.
const teardownScriptFmt = `if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not installed on this server"
  exit 0
fi
for proj in %[1]s %[2]s %[3]s; do
  ids=$(docker ps -aq --filter "label=com.docker.compose.project=$proj")
  [ -z "$ids" ] || echo "$ids" | xargs docker rm -f
done
for name in %[1]s %[2]s %[3]s; do
  docker rm -f "$name" 2>/dev/null || true
done
echo "torn down %[3]s"
`

// RemoveApp stops and removes the app's running container(s) on the server over
// SSH, so the app stops existing. It looks for compose stacks under three
// possible project-name labels ("<app>-<branch>", "<branch>", "<app>") so it
// catches both the current convention and legacy deploys, and also docker-rm's
// any plain container with one of those names for the docker-run strategy. It
// is best-effort: docker missing on the box or no matching container is NOT an
// error, only a connection or session failure is. It connects as the target
// user (the mountabo user, via its key) and runs without sudo, that user is a
// member of the docker group from bootstrap.
func (c *Client) RemoveApp(ctx context.Context, t usecase.SSHTarget, app, branch string) error {
	appBranch := shellSingleQuote(app + "-" + branch)
	br := shellSingleQuote(branch)
	a := shellSingleQuote(app)
	if _, err := c.runOutput(ctx, t, fmt.Sprintf(teardownScriptFmt, appBranch, br, a)); err != nil {
		return fmt.Errorf("remove app %s: %w", app, err)
	}
	return nil
}

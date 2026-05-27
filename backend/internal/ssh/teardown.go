package ssh

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.ContainerTeardown = (*Client)(nil)

// teardownScriptFmt stops and removes every container belonging to an app,
// covering both deploy strategies: a docker compose stack (its containers carry
// the "com.docker.compose.project=<app>" label) and a single container run
// plainly under the app's name. It is best-effort by design: a server without
// docker, or with no matching container, is a clean no-op, not a failure. The
// leading `command -v docker` guard prints an explanatory line and exits 0 when
// docker is absent so the caller never sees an error; `xargs -r` skips
// `docker rm` entirely when the filter matches nothing, and the plain
// `docker rm -f` is `|| true` so a "no such container" never fails the script.
// %[1]s is the compose project label value (single-quote-escaped) and %[2]s is
// the plain container name (single-quote-escaped) by the caller.
const teardownScriptFmt = `if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not installed on this server"
  exit 0
fi
docker ps -aq --filter label=com.docker.compose.project=%[1]s | xargs -r docker rm -f
docker rm -f %[2]s 2>/dev/null || true
echo "torn down %[2]s"
`

// RemoveApp stops and removes the app's running container(s) on the server over
// SSH, so the app stops existing: both a docker compose stack labelled with the
// app's project name and a plain container named after the app. It is
// best-effort: docker missing on the box or no matching container is NOT an
// error, only a connection or session failure is. It connects as the target
// user (the mountabo user, via its key) and runs without sudo, that user is a
// member of the docker group from bootstrap.
func (c *Client) RemoveApp(ctx context.Context, t usecase.SSHTarget, app string) error {
	quoted := shellSingleQuote(app)
	if _, err := c.runOutput(ctx, t, fmt.Sprintf(teardownScriptFmt, quoted, quoted)); err != nil {
		return fmt.Errorf("remove app %s: %w", app, err)
	}
	return nil
}

package ssh

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.DeployKeyInstaller = (*Client)(nil)

// InstallDeployKey writes the repo's read-only deploy private key to the deploy
// user's ~/.ssh/<keyName> (0600) over SSH, so deploy.sh can git clone the repo.
// It connects as the target user (the mountabo user, via its key) and runs
// without sudo, the key lands in that user's own home. Streams progress to out;
// the key's contents are never echoed.
func (c *Client) InstallDeployKey(ctx context.Context, t usecase.SSHTarget, keyName, privateKey string, out io.Writer) error {
	if err := c.runScript(ctx, t, "bash -s", installDeployKeyScript(keyName, privateKey), out); err != nil {
		return fmt.Errorf("install deploy key: %w", err)
	}
	return nil
}

// installDeployKeyScript writes the key via a quoted heredoc (so the PEM is
// taken literally, not expanded) under a tight umask, then locks it to 0600.
func installDeployKeyScript(keyName, privateKey string) string {
	return fmt.Sprintf(`set -euo pipefail
log() { echo "==> $*"; }
install -d -m 700 "$HOME/.ssh"
umask 077
cat > "$HOME/.ssh/%[1]s" <<'MOUNTABO_DEPLOY_KEY_EOF'
%[2]s
MOUNTABO_DEPLOY_KEY_EOF
chmod 600 "$HOME/.ssh/%[1]s"
log "deploy key installed at ~/.ssh/%[1]s"
`, keyName, strings.TrimRight(privateKey, "\n"))
}

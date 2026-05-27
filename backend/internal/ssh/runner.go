package ssh

import (
	"context"
	"fmt"
	"io"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.RootRunner = (*Client)(nil)

// RunAsRoot pipes a script to the server and runs it as root via passwordless
// sudo, streaming combined output to out. It connects as the mountabo user with
// its key (the same path ApplyOptions uses), so it is only valid on a server that
// has already been set up. The script is responsible for its own strict mode and
// logging.
func (c *Client) RunAsRoot(ctx context.Context, t usecase.SSHTarget, script string, out io.Writer) error {
	if err := c.runScript(ctx, t, "sudo -n bash -s", script, out); err != nil {
		return fmt.Errorf("run root script: %w", err)
	}
	return nil
}

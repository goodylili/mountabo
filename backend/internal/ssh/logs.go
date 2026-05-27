package ssh

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.LogInspector = (*Client)(nil)

// logsScript prints the recent logs of every running Docker container on the
// box, each block headed by "==> <name> <==" so a multi-app server stays
// readable. --timestamps prefixes every line with its RFC3339 UTC time, so the
// viewer can lead with the date and time of each entry. It runs as the mountabo
// user (a member of the docker group from bootstrap), no sudo, and tails the
// same way regardless of whether the app was deployed with docker compose or a
// plain docker run, both produce ordinary containers that `docker logs` can
// read. When Docker is absent or no container is running it prints a single
// explanatory line rather than failing, so the caller always gets a clean
// answer.
//
// %d is the bounded tail count, substituted by the caller.
const logsScript = `if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not installed on this server"
  exit 0
fi
names=$(docker ps --format '{{.Names}}' 2>/dev/null)
if [ -z "$names" ]; then
  echo "no running containers on this server"
  exit 0
fi
for name in $names; do
  echo "==> $name <=="
  docker logs --timestamps --tail %d "$name" 2>&1 || echo "(could not read logs for $name)"
done
`

// Logs connects as the target user and returns the deployed app's recent
// container logs, most recent tail lines per container, as a slice of lines.
func (c *Client) Logs(ctx context.Context, t usecase.SSHTarget, tail int) ([]string, error) {
	out, err := c.runOutput(ctx, t, fmt.Sprintf(logsScript, tail))
	if err != nil {
		return nil, fmt.Errorf("read logs: %w", err)
	}
	return splitLogLines(out), nil
}

// splitLogLines turns the combined command output into trimmed lines, dropping a
// single trailing blank line so the slice carries no empty tail entry.
func splitLogLines(out string) []string {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}
	}
	return lines
}

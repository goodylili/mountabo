package ssh

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.AppProber = (*Client)(nil)

// probeScriptFmt curls a URL from the server and prints just the HTTP status
// code on its own line (000 when no response was received at all, curl's
// convention). It is read-only (-sS, no body kept via -o /dev/null), follows no
// redirects so the status is the app's own, bounds the connection and total time
// so a hung app cannot wedge the probe, and accepts a self-signed/Let's Encrypt
// staging certificate with -k since the check only cares that the app answers.
// %q is the URL, single-quote-escaped by the caller for the shell.
const probeScriptFmt = `if ! command -v curl >/dev/null 2>&1; then echo "no-curl"; exit 0; fi
curl -sS -k -o /dev/null --max-time 8 --connect-timeout 5 -w '%%{http_code}' %s 2>/dev/null || echo 000`

// ProbeHTTP curls url from the server over SSH and reports whether the app
// answered and with what HTTP status. A status of 000 (curl could not connect)
// or any 5xx beyond reach means unreachable; any other real HTTP status counts
// as reachable (the app served a response, even a 404). curl missing on the box
// is reported as unreachable with status 0, not an error, so the card can still
// render an honest "unhealthy".
func (c *Client) ProbeHTTP(ctx context.Context, t usecase.SSHTarget, url string) (bool, int, error) {
	out, err := c.runOutput(ctx, t, fmt.Sprintf(probeScriptFmt, shellSingleQuote(url)))
	if err != nil {
		return false, 0, fmt.Errorf("probe %s: %w", url, err)
	}
	line := strings.TrimSpace(out)
	// Take the last non-empty line: the script may print a warning before the
	// status code, but the code is always last.
	if fields := strings.Fields(line); len(fields) > 0 {
		line = fields[len(fields)-1]
	}
	if line == "no-curl" {
		return false, 0, nil
	}
	// A non-numeric or zero result means the line was not a usable HTTP status,
	// so the app is unreachable: a normal down state, not a probe error. The
	// parse error is intentionally discarded (Atoi returns 0 on failure).
	status, _ := strconv.Atoi(line)
	if status == 0 {
		return false, 0, nil
	}
	return true, status, nil
}

// shellSingleQuote wraps s in single quotes for safe use in a /bin/sh command,
// escaping any embedded single quotes. The URL is derived from a validated host
// or a numeric port, but quoting it keeps the probe robust regardless.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

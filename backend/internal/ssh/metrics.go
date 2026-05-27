package ssh

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
)

var _ usecase.MetricsInspector = (*Client)(nil)

// metricsScript prints the host's CPU cores + load, memory (used/total MB),
// disk (used/total GB of /), and uptime (seconds) as key=value lines. It uses
// only base tools and reads as the connected user, no sudo.
const metricsScript = `echo "cores=$(nproc 2>/dev/null || echo 0)"
echo "load=$(cut -d' ' -f1 /proc/loadavg 2>/dev/null)"
echo "mem=$(free -m 2>/dev/null | awk '/^Mem:/{print $3" "$2}')"
echo "disk=$(df -BG / 2>/dev/null | awk 'NR==2{gsub("G","",$3); gsub("G","",$2); print $3" "$2}')"
echo "uptime=$(cut -d' ' -f1 /proc/uptime 2>/dev/null | cut -d. -f1)"
`

// Metrics connects as the target user and reads the server's current host
// metrics.
func (c *Client) Metrics(ctx context.Context, t usecase.SSHTarget) (usecase.ServerMetrics, error) {
	out, err := c.runOutput(ctx, t, metricsScript)
	if err != nil {
		return usecase.ServerMetrics{}, fmt.Errorf("read metrics: %w", err)
	}
	return parseMetrics(out), nil
}

func parseMetrics(out string) usecase.ServerMetrics {
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if key, val, ok := strings.Cut(line, "="); ok {
			fields[key] = strings.TrimSpace(val)
		}
	}

	m := usecase.ServerMetrics{
		CPUCores:      atoi(fields["cores"]),
		Load1:         atof(fields["load"]),
		UptimeSeconds: atoi(fields["uptime"]),
	}
	if used, total, ok := twoInts(fields["mem"]); ok {
		m.MemUsedMB, m.MemTotalMB = used, total
	}
	if used, total, ok := twoInts(fields["disk"]); ok {
		m.DiskUsedGB, m.DiskTotalGB = used, total
	}
	return m
}

func atoi(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }

func atof(s string) float64 { f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return f }

// twoInts parses "a b" (two space-separated integers), e.g. "1203 3936".
func twoInts(s string) (a, b int, ok bool) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(parts[0])
	y, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return x, y, true
}

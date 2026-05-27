// Package http is mountabo's HTTP transport adapter: it builds the server,
// routes, and handlers that expose the usecase layer over the local API. It
// depends inward on internal/usecase and internal/config only.
package http

import (
	nethttp "net/http"
	"time"

	"github.com/goodylili/mountabo/internal/config"
)

// NewServer builds the local API server. Timeouts are set defensively even
// though the listener is loopback-only: ReadHeaderTimeout bounds slow-header
// clients. WriteTimeout is generous because some reads fan out to GitHub for
// every repo (listing + per-repo container detection) and can legitimately take
// many seconds on large accounts; a tight 30s here made the repo list fail with
// an i/o timeout and the UI retry into an empty list. Streaming handlers clear
// the deadline entirely (see streamSSE).
func NewServer(cfg *config.Config, handler nethttp.Handler) *nethttp.Server {
	return &nethttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

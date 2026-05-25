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
// clients, and the write timeout comfortably covers the round trips to GitHub
// that a token exchange makes.
func NewServer(cfg *config.Config, handler nethttp.Handler) *nethttp.Server {
	return &nethttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

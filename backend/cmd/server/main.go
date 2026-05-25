// Command server runs mountabo's local API: it composes the GitHub connection
// flow and serves it over a loopback HTTP listener.
package main

import (
	"context"
	"errors"
	"log/slog"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"

	httpadapter "github.com/goodylili/mountabo/internal/adapter/http"
	"github.com/goodylili/mountabo/internal/config"
	"github.com/goodylili/mountabo/internal/github"
	"github.com/goodylili/mountabo/internal/keychain"
	"github.com/goodylili/mountabo/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	logger := slog.Default()

	// Compose the GitHub connection flow: OAuth exchange (github) + account
	// lookup (github) + token persistence (keychain), driven by the connector.
	connector := usecase.NewGitHubConnector(
		github.NewOAuth(cfg.GitHub.ClientID, cfg.GitHub.ClientSecret),
		github.NewClient(),
		keychain.NewStore(),
	)

	githubHandler := httpadapter.NewGitHubHandler(connector, logger)
	router := httpadapter.NewRouter(githubHandler)
	srv := httpadapter.NewServer(cfg, router)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

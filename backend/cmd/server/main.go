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
	"path/filepath"
	"syscall"

	httpadapter "github.com/goodylili/mountabo/internal/adapter/http"
	"github.com/goodylili/mountabo/internal/adapter/repository"
	"github.com/goodylili/mountabo/internal/config"
	"github.com/goodylili/mountabo/internal/github"
	"github.com/goodylili/mountabo/internal/keychain"
	"github.com/goodylili/mountabo/internal/ssh"
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

	keyStore := keychain.NewStore()

	// Compose the GitHub connection flow: OAuth exchange + account lookup + repo
	// listing + port detection (all github) + token persistence (keychain),
	// driven by the connector. One github.Client serves account, repo, and
	// container-config reads.
	ghClient := github.NewClient()
	// The OAuth adapter both exchanges codes and refreshes tokens; the token
	// manager wraps the keychain store so every Load returns a still-valid token
	// (GitHub App user tokens expire ~8h), keeping the user's OAuth credential
	// usable for all GitHub requests, repos, deploy keys, secrets, and workflows.
	oauth := github.NewOAuth(cfg.GitHub.ClientID, cfg.GitHub.ClientSecret)
	tokens := usecase.NewTokenManager(keyStore, oauth)
	connector := usecase.NewGitHubConnector(
		oauth,
		ghClient,
		ghClient,
		ghClient,
		tokens,
	)
	// Repo tree listing (directory/file picker) reads on the user's behalf with
	// the same refreshing token.
	treeSvc := usecase.NewTreeService(tokens, ghClient)
	githubHandler := httpadapter.NewGitHubHandler(connector, treeSvc, logger)

	// Compose the server flow: SSH probe + bootstrap + key generation (all ssh),
	// JSON-file persistence, and keychain secrets. One ssh.Client serves probe,
	// bootstrap, keygen, and port inspection.
	sshClient := ssh.NewClient()
	serverStore := repository.NewServerFile(filepath.Join(cfg.DataDir, "servers.json"))
	serverSvc := usecase.NewServerService(serverStore, sshClient, sshClient, sshClient, sshClient, sshClient, keyStore)
	serverPortSvc := usecase.NewServerPortService(serverStore, keyStore, sshClient)
	serverMetricsSvc := usecase.NewServerMetricsService(serverStore, keyStore, sshClient)
	serversHandler := httpadapter.NewServersHandler(serverSvc, serverPortSvc, serverMetricsSvc, logger)

	// Compose the deploy flow: commit the workflow + deploy.sh and provision the
	// environment/secrets (all github), reading the server record (JSON store)
	// and mountabo's stored key (keychain). One github.Client and keychain.Store
	// already in hand serve these too.
	deploymentStore := repository.NewDeploymentFile(filepath.Join(cfg.DataDir, "deployments.json"))
	// The deploy flow also mints a per-repo read-only deploy key: ssh generates
	// the keypair and installs the private half on the server; github registers
	// the public half on the repo.
	deploySvc := usecase.NewDeployService(serverStore, keyStore, tokens, ghClient, ghClient, ghClient, deploymentStore, sshClient, ghClient, sshClient)
	deployHandler := httpadapter.NewDeployHandler(deploySvc, logger)

	// Compose the monitor: configured deployments (JSON store) enriched with
	// their GitHub Actions runs (github), read with the refreshing token.
	monitorSvc := usecase.NewMonitorService(deploymentStore, tokens, ghClient)
	monitorHandler := httpadapter.NewMonitorHandler(monitorSvc, logger)

	router := httpadapter.NewRouter(githubHandler, serversHandler, deployHandler, monitorHandler)
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

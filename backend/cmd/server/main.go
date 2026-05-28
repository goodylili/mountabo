// Command server runs mountabo's local API: it composes the GitHub connection
// flow and serves it over a loopback HTTP listener.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	nethttp "net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	httpadapter "github.com/goodylili/mountabo/internal/adapter/http"
	"github.com/goodylili/mountabo/internal/adapter/repository"
	"github.com/goodylili/mountabo/internal/ai"
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
	// The OAuth adapter exchanges authorization codes for the access token; the
	// keychain store persists it, and every later GitHub request reads it back:
	// repos, deploy keys, secrets, and workflows.
	oauth := github.NewOAuth(cfg.GitHub.ClientID, cfg.GitHub.ClientSecret)
	connector := usecase.NewGitHubConnector(
		oauth,
		ghClient,
		ghClient,
		ghClient,
		keyStore,
	)
	// Repo tree listing (directory/file picker) reads on the user's behalf with
	// the same keychain token.
	treeSvc := usecase.NewTreeService(keyStore, ghClient)
	// .env.example discovery: read the repo's example env file so the configure
	// form can pre-fill the env var rows for the operator to fill in.
	envExampleSvc := usecase.NewEnvExampleService(keyStore, ghClient)
	// Run-step progress: read the latest deploy run's jobs + steps so the UI can
	// show each GitHub Actions step's live status, with the same keychain token.
	runStepsSvc := usecase.NewRunStepsService(keyStore, ghClient)
	// Branches lister: the new-environment picker on the deployment card reads
	// this so the operator chooses from the repo's real branches instead of
	// typing a name.
	branchesSvc := usecase.NewBranchesService(keyStore, ghClient)
	githubHandler := httpadapter.NewGitHubHandler(connector, treeSvc, envExampleSvc, runStepsSvc, branchesSvc, logger)

	// Compose the server flow: SSH probe + bootstrap + key generation (all ssh),
	// JSON-file persistence, and keychain secrets. One ssh.Client serves probe,
	// bootstrap, keygen, and port inspection.
	sshClient := ssh.NewClient()
	serverStore := repository.NewServerFile(filepath.Join(cfg.DataDir, "servers.json"))
	serverSvc := usecase.NewServerService(serverStore, sshClient, sshClient, sshClient, sshClient, sshClient, sshClient, keyStore)
	serverPortSvc := usecase.NewServerPortService(serverStore, keyStore, sshClient)
	serverMetricsSvc := usecase.NewServerMetricsService(serverStore, keyStore, sshClient)
	serverLogsSvc := usecase.NewServerLogsService(serverStore, keyStore, sshClient)
	// The dashboard service opens SSH local port-forward tunnels to a server's
	// loopback monitoring UI (Uptime Kuma) over the same ssh.Client, binding to
	// this machine's loopback so the browser loads it directly. Tunnels are torn
	// down on shutdown.
	serverDashboardSvc := usecase.NewServerDashboardService(serverStore, keyStore, sshClient)
	// Uptime Kuma has no public HTTP setup endpoint, so mountabo generates a
	// fresh admin credential pair, seeds it into UK's SQLite from inside the
	// container, and persists it in the keychain so the dashboard panel can
	// surface it the next time the page loads.
	uptimeKumaAdminSvc := usecase.NewUptimeKumaAdminService(serverStore, keyStore, sshClient)
	defer func() { _ = serverDashboardSvc.Close() }()
	serversHandler := httpadapter.NewServersHandler(serverSvc, serverPortSvc, serverMetricsSvc, serverLogsSvc, serverDashboardSvc, uptimeKumaAdminSvc, logger)

	// Compose the deploy flow: commit the workflow + deploy.sh and provision the
	// environment/secrets (all github), reading the server record (JSON store)
	// and mountabo's stored key (keychain). One github.Client and keychain.Store
	// already in hand serve these too.
	//
	// Deployments persist in SQLite (not JSON) so each deploy is tracked in an
	// append-only event log queryable over time, not just upserted away.
	db, err := repository.OpenSQLite(filepath.Join(cfg.DataDir, "mountabo.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()
	deploymentStore := repository.NewDeploymentSQL(db)
	// The deploy flow also mints a per-repo read-only deploy key: ssh generates
	// the keypair and installs the private half on the server; github registers
	// the public half on the repo.
	deploySvc := usecase.NewDeployService(serverStore, keyStore, keyStore, ghClient, ghClient, ghClient, deploymentStore, sshClient, ghClient, sshClient, ghClient)
	deployHandler := httpadapter.NewDeployHandler(deploySvc, logger)

	// Compose the monitor: configured deployments (SQLite store) enriched with
	// their GitHub Actions runs (github), read with the keychain token, and the
	// server store so each deployment can surface its live app URL. Deleting a
	// deployment is a real teardown: ssh stops and removes the app's container(s)
	// on its server (keychain key), github removes the committed deploy workflow +
	// deploy.sh from the repo (keychain token), and the SQLite store then forgets
	// its tracking. Teardown is best-effort and logged.
	monitorSvc := usecase.NewMonitorService(deploymentStore, deploymentStore, deploymentStore, keyStore, ghClient, serverStore, keyStore, sshClient, ghClient, ghClient, ghClient, logger)
	// App health: probe whether a deployed app is responding by curling it from
	// its own server over SSH (the same ssh.Client + server store + keychain key
	// the metrics/logs services use), so the card shows real up/down status.
	appHealthSvc := usecase.NewAppHealthService(deploymentStore, serverStore, keyStore, sshClient)
	monitorHandler := httpadapter.NewMonitorHandler(monitorSvc, appHealthSvc, logger)

	// Compose the terminal page: run a single operator command on a set-up server
	// over SSH (the same ssh.Client + server store + keychain key the read-only
	// metrics/logs services use), and an AI command helper backed by Anthropic.
	// The AI client reads ANTHROPIC_API_KEY from config; an empty key makes the
	// helper return a structured "not configured" result instead of failing. The
	// helper only suggests, the operator runs the command through /exec.
	serverExecSvc := usecase.NewServerExecService(serverStore, keyStore, sshClient)
	aiClient := ai.NewClient(cfg.AI.APIKey, cfg.AI.Model)
	aiCommandSvc := usecase.NewAICommandService(aiClient)
	terminalHandler := httpadapter.NewTerminalHandler(serverExecSvc, aiCommandSvc, logger)

	router := httpadapter.NewRouter(githubHandler, serversHandler, deployHandler, monitorHandler, terminalHandler)
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

package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// MonitorHandler serves deploy history (the configured deployments with their
// recent GitHub Actions runs), an SSH-based app health probe, and deletion of a
// deployment's tracking.
type MonitorHandler struct {
	svc    *usecase.MonitorService
	health *usecase.AppHealthService
	log    *slog.Logger
}

// NewMonitorHandler wires the handler to the monitor service, the app-health
// probe service, and a logger.
func NewMonitorHandler(svc *usecase.MonitorService, health *usecase.AppHealthService, log *slog.Logger) *MonitorHandler {
	return &MonitorHandler{svc: svc, health: health, log: log}
}

// Deployments returns each configured deployment enriched with its recent runs.
// Not-connected is reported as 401; an empty list is a normal "nothing deployed
// yet" answer.
func (h *MonitorHandler) Deployments(w nethttp.ResponseWriter, r *nethttp.Request) {
	history, err := h.svc.History(r.Context())
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("list deployments failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not list deployments")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(nethttp.StatusOK)
	if err := json.NewEncoder(w).Encode(history); err != nil {
		h.log.Error("encode deployments", "err", err)
	}
}

// Health probes whether the deployment named in the path is responding, from
// its own server over SSH, and returns up/down plus the HTTP status. Not-found
// (no tracked deployment, or its server) is 404; a failed SSH probe is 502. An
// app that is simply down is a 200 with reachable:false, so the card can show an
// honest unhealthy indicator.
func (h *MonitorHandler) Health(w nethttp.ResponseWriter, r *nethttp.Request) {
	app := r.PathValue("app")
	health, err := h.health.Health(r.Context(), app)
	switch {
	case errors.Is(err, usecase.ErrDeploymentNotFound):
		h.writeError(w, nethttp.StatusNotFound, "deployment not found")
		return
	case errors.Is(err, usecase.ErrServerNotFound):
		h.writeError(w, nethttp.StatusNotFound, "server not found")
		return
	case err != nil:
		h.log.Error("probe app health failed", "app", app, "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not probe the app over ssh")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(nethttp.StatusOK)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		h.log.Error("encode app health", "err", err)
	}
}

// Delete tears the deployment named in the path fully down: it stops and
// removes the app's container(s) on its server, deletes the committed deploy
// workflow and deploy.sh from the repo, and removes the deployment record and
// its deploy history. The teardown is best-effort: soft failures (server
// unreachable, file already gone) are logged but still return 204, so the user
// is never stuck. Not-found is 404.
func (h *MonitorHandler) Delete(w nethttp.ResponseWriter, r *nethttp.Request) {
	app := r.PathValue("app")
	if err := h.svc.Delete(r.Context(), app); err != nil {
		if errors.Is(err, usecase.ErrDeploymentNotFound) {
			h.writeError(w, nethttp.StatusNotFound, "deployment not found")
			return
		}
		h.log.Error("delete deployment failed", "app", app, "err", err)
		h.writeError(w, nethttp.StatusInternalServerError, "could not delete the deployment")
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

func (h *MonitorHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

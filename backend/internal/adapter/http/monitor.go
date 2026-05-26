package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// MonitorHandler serves deploy history: the configured deployments with their
// recent GitHub Actions runs.
type MonitorHandler struct {
	svc *usecase.MonitorService
	log *slog.Logger
}

// NewMonitorHandler wires the handler to the monitor service and a logger.
func NewMonitorHandler(svc *usecase.MonitorService, log *slog.Logger) *MonitorHandler {
	return &MonitorHandler{svc: svc, log: log}
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

func (h *MonitorHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

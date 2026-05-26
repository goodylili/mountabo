package http

import (
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// DeployHandler configures continuous deployment of a repo to a server and
// previews the files it would commit. Deploy is a POST (not the GET the
// browser's EventSource uses) because the request carries env var values that
// become secrets, so they must travel in the body, never a URL; its response is
// streamed as Server-Sent Events. Preview is a plain JSON POST with no side
// effects.
type DeployHandler struct {
	svc *usecase.DeployService
	log *slog.Logger
}

// NewDeployHandler wires the handler to the deploy service and a logger.
func NewDeployHandler(svc *usecase.DeployService, log *slog.Logger) *DeployHandler {
	return &DeployHandler{svc: svc, log: log}
}

type deployRequest struct {
	App         string `json:"app"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Branch      string `json:"branch"`
	Environment string `json:"environment"`
	Strategy    string `json:"strategy"`
	RootDir     string `json:"rootDir"`
	DeployDir   string `json:"deployDir"`
	Ports       []struct {
		EnvVar    string `json:"envVar"`
		Value     string `json:"value"`
		Container string `json:"container"`
	} `json:"ports"`
	EnvVars []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"envVars"`
}

// input maps the decoded request onto the usecase input, attaching the server
// id (from the path for Deploy, empty for Preview).
func (req deployRequest) input(serverID string) usecase.DeployInput {
	ports := make([]usecase.DeployPort, 0, len(req.Ports))
	for _, p := range req.Ports {
		ports = append(ports, usecase.DeployPort{EnvVar: p.EnvVar, Value: p.Value, Container: p.Container})
	}
	envVars := make([]usecase.DeployEnvVar, 0, len(req.EnvVars))
	for _, v := range req.EnvVars {
		envVars = append(envVars, usecase.DeployEnvVar{Key: v.Key, Value: v.Value})
	}
	return usecase.DeployInput{
		ServerID:    serverID,
		App:         req.App,
		Owner:       req.Owner,
		Repo:        req.Repo,
		Branch:      req.Branch,
		Environment: req.Environment,
		Strategy:    req.Strategy,
		RootDir:     req.RootDir,
		DeployDir:   req.DeployDir,
		Ports:       ports,
		EnvVars:     envVars,
	}
}

// Preview generates the workflow, deploy.sh, and secret list for the given
// config and returns them as JSON. No server, token, or side effects, so the
// configure UI can render exactly what a deploy would commit.
func (h *DeployHandler) Preview(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req deployRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	artifacts, err := h.svc.Preview(req.input(""))
	if err != nil {
		// Validation errors are safe to surface (no secrets), and tell the UI
		// what to fix.
		h.writeError(w, nethttp.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(nethttp.StatusOK)
	if err := json.NewEncoder(w).Encode(artifacts); err != nil {
		h.log.Error("encode preview", "err", err)
	}
}

// Deploy commits the workflow and deploy.sh to the repo, provisions the
// deployment environment, and sets its secrets, streaming progress as SSE.
func (h *DeployHandler) Deploy(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req deployRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	in := req.input(r.PathValue("id"))
	streamSSE(w, h.log, "deploy configured", func(out io.Writer) error {
		return h.svc.Deploy(r.Context(), in, out)
	})
}

func (h *DeployHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

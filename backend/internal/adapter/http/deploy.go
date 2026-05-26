package http

import (
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// DeployHandler wires continuous deployment of a repo to a server. It is a POST
// (not the GET the browser's EventSource uses) because the request carries env
// var values that become secrets, so they must travel in the body, never a URL,
// the response is still streamed as Server-Sent Events.
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
	RootDir     string `json:"rootDir"`
	DeployDir   string `json:"deployDir"`
	Ports       struct {
		Frontend string `json:"frontend"`
		Backend  string `json:"backend"`
		Postgres string `json:"postgres"`
		Redis    string `json:"redis"`
	} `json:"ports"`
	EnvVars []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"envVars"`
}

// Deploy commits the workflow and deploy.sh to the repo, provisions the
// deployment environment, and sets its secrets, streaming progress as SSE.
func (h *DeployHandler) Deploy(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req deployRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}

	envVars := make([]usecase.DeployEnvVar, 0, len(req.EnvVars))
	for _, v := range req.EnvVars {
		envVars = append(envVars, usecase.DeployEnvVar{Key: v.Key, Value: v.Value})
	}

	in := usecase.DeployInput{
		ServerID:    r.PathValue("id"),
		App:         req.App,
		Owner:       req.Owner,
		Repo:        req.Repo,
		Branch:      req.Branch,
		Environment: req.Environment,
		RootDir:     req.RootDir,
		DeployDir:   req.DeployDir,
		Ports: usecase.DeployPorts{
			Frontend: req.Ports.Frontend,
			Backend:  req.Ports.Backend,
			Postgres: req.Ports.Postgres,
			Redis:    req.Ports.Redis,
		},
		EnvVars: envVars,
	}

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

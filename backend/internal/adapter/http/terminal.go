package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// TerminalHandler serves the terminal page's two endpoints: running a single
// command on a server over SSH, and asking the AI helper to suggest a command
// for a plain-English request. The AI helper only suggests; the human reviews
// and runs the command through the exec endpoint, so nothing the model produces
// is ever executed automatically.
type TerminalHandler struct {
	exec *usecase.ServerExecService
	ai   *usecase.AICommandService
	log  *slog.Logger
}

// NewTerminalHandler wires the handler to the exec and AI-command services.
func NewTerminalHandler(exec *usecase.ServerExecService, ai *usecase.AICommandService, log *slog.Logger) *TerminalHandler {
	return &TerminalHandler{exec: exec, ai: ai, log: log}
}

type execRequest struct {
	Command string `json:"command"`
}

// Exec runs a single operator-supplied command on the chosen server over SSH and
// returns its combined output and exit code. A command that exits non-zero is a
// normal 200 result (the output and exitCode tell the story); 502 is reserved
// for the SSH connection itself failing. Not-found is 404.
func (h *TerminalHandler) Exec(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	var req execRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Command == "" {
		h.writeError(w, nethttp.StatusBadRequest, "command is required")
		return
	}

	result, err := h.exec.Exec(r.Context(), id, req.Command)
	if errors.Is(err, usecase.ErrServerNotFound) {
		h.writeError(w, nethttp.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		h.log.Error("run server command failed", "id", id, "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not run the command on the server")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, result)
}

type aiCommandRequest struct {
	Prompt  string `json:"prompt"`
	Context string `json:"context"`
}

// AICommand asks the AI helper to suggest a shell command for a plain-English
// request. When ANTHROPIC_API_KEY is unset this returns a clean 200 with
// configured=false and a hint to set the key (never a 500), so a missing key is
// a displayable state in the UI. The suggestion is advisory only: it is shown to
// the operator, who must review and explicitly run it.
func (h *TerminalHandler) AICommand(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req aiCommandRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prompt == "" {
		h.writeError(w, nethttp.StatusBadRequest, "prompt is required")
		return
	}

	result, err := h.ai.Suggest(r.Context(), req.Prompt, req.Context)
	if err != nil {
		h.log.Error("ai command suggestion failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not get a suggestion from the AI helper")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, result)
}

func (h *TerminalHandler) writeJSON(w nethttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

func (h *TerminalHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

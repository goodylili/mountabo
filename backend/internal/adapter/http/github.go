package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"github.com/goodylili/mountabo/internal/usecase"
)

// maxBodyBytes caps request bodies. Exchange payloads are a short code plus a
// redirect URI, so this is generous while still bounding memory use.
const maxBodyBytes = 64 << 10

// GitHubHandler serves the GitHub connection endpoints over the usecase layer.
type GitHubHandler struct {
	connector *usecase.GitHubConnector
	log       *slog.Logger
}

// NewGitHubHandler wires the handler to the connector and a logger.
func NewGitHubHandler(connector *usecase.GitHubConnector, log *slog.Logger) *GitHubHandler {
	return &GitHubHandler{connector: connector, log: log}
}

type exchangeRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirectUri"`
}

type accountResponse struct {
	Login string `json:"login"`
}

type statusResponse struct {
	Connected bool   `json:"connected"`
	Login     string `json:"login,omitempty"`
}

// Exchange completes the OAuth web flow: it takes the authorization code the
// browser received and asks the connector to turn it into a stored token,
// returning the connected login. The code and token never appear in responses
// or logs.
func (h *GitHubHandler) Exchange(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req exchangeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing authorization code")
		return
	}

	account, err := h.connector.Connect(r.Context(), req.Code, req.RedirectURI)
	if err != nil {
		h.log.Error("github connect failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not complete github exchange")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, accountResponse{Login: account.Login})
}

// Status reports whether a token is stored and, if so, which account it belongs
// to. A missing token is a normal "not connected" answer, not an error.
func (h *GitHubHandler) Status(w nethttp.ResponseWriter, r *nethttp.Request) {
	account, err := h.connector.Status(r.Context())
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeJSON(w, nethttp.StatusOK, statusResponse{Connected: false})
		return
	}
	if err != nil {
		h.log.Error("github status failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not read github status")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, statusResponse{Connected: true, Login: account.Login})
}

// Disconnect removes the stored token from the keychain.
func (h *GitHubHandler) Disconnect(w nethttp.ResponseWriter, _ *nethttp.Request) {
	if err := h.connector.Disconnect(); err != nil {
		h.log.Error("github disconnect failed", "err", err)
		h.writeError(w, nethttp.StatusInternalServerError, "could not disconnect github")
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

// decodeJSON reads a single JSON object from the request body, bounding its size
// and rejecting unknown fields so malformed callers fail loudly.
func decodeJSON(w nethttp.ResponseWriter, r *nethttp.Request, dst any) error {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (h *GitHubHandler) writeJSON(w nethttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The response is already committed by WriteHeader, so an encode failure can
	// only be logged, not recovered from.
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

func (h *GitHubHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

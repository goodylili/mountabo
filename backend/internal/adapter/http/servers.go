package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
)

// ServersHandler serves the add-server, list, live-setup, port-check, and
// host-metrics endpoints.
type ServersHandler struct {
	svc     *usecase.ServerService
	ports   *usecase.ServerPortService
	metrics *usecase.ServerMetricsService
	log     *slog.Logger
}

// NewServersHandler wires the handler to the server service, the port-check and
// metrics services, and a logger.
func NewServersHandler(svc *usecase.ServerService, ports *usecase.ServerPortService, metrics *usecase.ServerMetricsService, log *slog.Logger) *ServersHandler {
	return &ServersHandler{svc: svc, ports: ports, metrics: metrics, log: log}
}

// Metrics reports a server's live host health (cpu/load, memory, disk, uptime),
// read over SSH, so the monitor can show real numbers instead of placeholders.
func (h *ServersHandler) Metrics(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	metrics, err := h.metrics.Metrics(r.Context(), id)
	if errors.Is(err, usecase.ErrServerNotFound) {
		h.writeError(w, nethttp.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		h.log.Error("read server metrics failed", "id", id, "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not read server metrics")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, metrics)
}

// Ports reports the ports already listening on a server so the configure UI can
// flag a deploy port that would collide. The port stays editable; mountabo
// never changes it.
func (h *ServersHandler) Ports(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	ports, err := h.ports.Listening(r.Context(), id)
	if errors.Is(err, usecase.ErrServerNotFound) {
		h.writeError(w, nethttp.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		h.log.Error("list server ports failed", "id", id, "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not check server ports")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, map[string][]int{"listening": ports})
}

type addServerRequest struct {
	Name          string `json:"name"`
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Timezone      string `json:"timezone"`
	RootPassword  string `json:"rootPassword"`
	UserPublicKey string `json:"userPublicKey"`
}

// Add probes a server over SSH with the root password and records it. The root
// password is consumed here and stored only in the keychain, never echoed back.
func (h *ServersHandler) Add(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req addServerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeError(w, nethttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.IP == "" || req.RootPassword == "" || req.Timezone == "" {
		h.writeError(w, nethttp.StatusBadRequest, "name, ip, timezone, and rootPassword are required")
		return
	}

	server, err := h.svc.Add(r.Context(), usecase.AddServerInput{
		Name:          req.Name,
		IP:            req.IP,
		Port:          req.Port,
		Timezone:      req.Timezone,
		RootPassword:  req.RootPassword,
		UserPublicKey: req.UserPublicKey,
	})
	if err != nil {
		h.log.Error("add server failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not reach the server, check the ip and root password")
		return
	}
	h.writeJSON(w, nethttp.StatusCreated, server)
}

// Options returns the catalog of opt-in hardening steps so the UI can let the
// operator choose which to apply.
func (h *ServersHandler) Options(w nethttp.ResponseWriter, _ *nethttp.Request) {
	h.writeJSON(w, nethttp.StatusOK, h.svc.Options())
}

// List returns all added servers (without secrets).
func (h *ServersHandler) List(w nethttp.ResponseWriter, _ *nethttp.Request) {
	servers, err := h.svc.List()
	if err != nil {
		h.log.Error("list servers failed", "err", err)
		h.writeError(w, nethttp.StatusInternalServerError, "could not list servers")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, servers)
}

// Setup runs the bootstrap on a server, streaming progress as Server-Sent Events.
func (h *ServersHandler) Setup(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	var options []string
	if raw := r.URL.Query().Get("options"); raw != "" {
		options = strings.Split(raw, ",")
	}
	streamSSE(w, h.log, "server is ready", func(out io.Writer) error {
		return h.svc.Setup(r.Context(), id, options, out)
	})
}

// ApplyOptions enables/disables hardening options on a server to match the
// ?set=… selection, streaming the live log as SSE.
func (h *ServersHandler) ApplyOptions(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	var desired []string
	if raw := q.Get("set"); raw != "" {
		desired = strings.Split(raw, ",")
	}
	// Per-option params arrive as param.<optionID>.<key>=value. Option ids use
	// hyphens (never dots), so splitting on the first dot is unambiguous.
	params := map[string]map[string]string{}
	for key, vals := range q {
		rest, ok := strings.CutPrefix(key, "param.")
		if !ok || len(vals) == 0 {
			continue
		}
		optID, pKey, ok := strings.Cut(rest, ".")
		if !ok {
			continue
		}
		if params[optID] == nil {
			params[optID] = map[string]string{}
		}
		params[optID][pKey] = vals[0]
	}
	streamSSE(w, h.log, "settings applied", func(out io.Writer) error {
		return h.svc.ApplyOptions(r.Context(), id, desired, params, out)
	})
}

// streamSSE runs a long, output-producing operation and relays it as Server-Sent
// Events: each output line is a `data:` event, ending with a terminal `done`
// (with successMsg) or `error` event. The per-response write deadline is
// cleared because these can run for many minutes. Shared by every streaming
// handler (setup, apply-options, deploy).
func streamSSE(w nethttp.ResponseWriter, log *slog.Logger, successMsg string, run func(io.Writer) error) {
	flusher, ok := w.(nethttp.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nethttp.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "streaming unsupported"})
		return
	}
	if err := nethttp.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		log.Warn("clear write deadline", "err", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(nethttp.StatusOK)
	flusher.Flush()

	sw := &sseWriter{w: w, flusher: flusher}
	err := run(sw)
	sw.flushPartial()

	switch {
	case err == nil:
		sw.event("done", successMsg)
	case errors.Is(err, usecase.ErrServerNotFound):
		sw.event("error", "server not found")
	case errors.Is(err, usecase.ErrNotConnected):
		sw.event("error", "github not connected")
	case errors.Is(err, usecase.ErrSetupInProgress):
		sw.event("error", "another setup/apply is already running for this server")
	default:
		log.Error("stream operation failed", "err", err)
		// Surface the (secret-free) reason, then the terminal error event.
		sw.data("✗ " + strings.ReplaceAll(err.Error(), "\n", " "))
		sw.event("error", "failed")
	}
}

// Delete removes a server and destroys its keychain secrets.
func (h *ServersHandler) Delete(w nethttp.ResponseWriter, r *nethttp.Request) {
	id := r.PathValue("id")
	if err := h.svc.Remove(id); err != nil {
		if errors.Is(err, usecase.ErrServerNotFound) {
			h.writeError(w, nethttp.StatusNotFound, "server not found")
			return
		}
		h.log.Error("remove server failed", "id", id, "err", err)
		h.writeError(w, nethttp.StatusInternalServerError, "could not remove server")
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

func (h *ServersHandler) writeJSON(w nethttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("encode response", "err", err)
	}
}

func (h *ServersHandler) writeError(w nethttp.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

// sseWriter adapts an io.Writer (the bootstrap's combined output) to Server-Sent
// Events: it buffers partial writes and emits one `data:` event per line.
type sseWriter struct {
	w       nethttp.ResponseWriter
	flusher nethttp.Flusher
	buf     []byte
}

func (s *sseWriter) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	for {
		i := bytes.IndexByte(s.buf, '\n')
		if i < 0 {
			break
		}
		line := string(s.buf[:i])
		s.buf = s.buf[i+1:]
		s.data(line)
	}
	return len(p), nil
}

func (s *sseWriter) data(line string) {
	_, _ = fmt.Fprintf(s.w, "data: %s\n\n", strings.TrimRight(line, "\r"))
	s.flusher.Flush()
}

func (s *sseWriter) flushPartial() {
	if len(s.buf) > 0 {
		s.data(string(s.buf))
		s.buf = nil
	}
}

func (s *sseWriter) event(name, data string) {
	_, _ = fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", name, data)
	s.flusher.Flush()
}

package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
)

// maxBodyBytes caps request bodies. Exchange payloads are a short code plus a
// redirect URI, so this is generous while still bounding memory use.
const maxBodyBytes = 64 << 10

// GitHubHandler serves the GitHub connection endpoints over the usecase layer.
type GitHubHandler struct {
	connector  *usecase.GitHubConnector
	tree       *usecase.TreeService
	envExample *usecase.EnvExampleService
	runSteps   *usecase.RunStepsService
	branches   *usecase.BranchesService
	log        *slog.Logger
}

// NewGitHubHandler wires the handler to the connector, the repo-tree service,
// the env-example service, the run-steps service, the branches service, and a
// logger.
func NewGitHubHandler(connector *usecase.GitHubConnector, tree *usecase.TreeService, envExample *usecase.EnvExampleService, runSteps *usecase.RunStepsService, branches *usecase.BranchesService, log *slog.Logger) *GitHubHandler {
	return &GitHubHandler{connector: connector, tree: tree, envExample: envExample, runSteps: runSteps, branches: branches, log: log}
}

// Branches lists every branch on owner/repo so the new-environment picker can
// present the real branch list instead of a free-text field. owner and repo
// are required; not-connected is reported as 401.
func (h *GitHubHandler) Branches(w nethttp.ResponseWriter, r *nethttp.Request) {
	owner, repo := r.PathValue("owner"), r.PathValue("repo")
	if owner == "" || repo == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner or repo")
		return
	}
	names, err := h.branches.List(r.Context(), owner, repo)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("list branches failed", "owner", owner, "repo", repo, "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not list repository branches")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, map[string][]string{"branches": names})
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

type repoResponse struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"defaultBranch"`
	Language      string `json:"language"`
	PushedAt      string `json:"pushedAt"`
	HasDocker     bool   `json:"hasDocker"`
	Kind          string `json:"kind"` // "compose" | "docker" | "none"
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

// Repos lists the connected account's repositories (public and private) so the
// UI can show them. Not-connected is reported as 401 rather than an error.
func (h *GitHubHandler) Repos(w nethttp.ResponseWriter, r *nethttp.Request) {
	repos, err := h.connector.Repositories(r.Context())
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("list repos failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not list repositories")
		return
	}

	out := make([]repoResponse, 0, len(repos))
	for _, rp := range repos {
		pushed := ""
		if !rp.PushedAt.IsZero() {
			pushed = rp.PushedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, repoResponse{
			Owner:         rp.Owner,
			Name:          rp.Name,
			FullName:      rp.FullName,
			Private:       rp.Private,
			DefaultBranch: rp.DefaultBranch,
			Language:      rp.Language,
			PushedAt:      pushed,
			HasDocker:     rp.HasDocker,
			Kind:          rp.Kind,
		})
	}
	h.writeJSON(w, nethttp.StatusOK, out)
}

type portResponse struct {
	Service   string `json:"service"`
	EnvVar    string `json:"envVar"`
	Host      string `json:"host"`
	Container string `json:"container"`
	Editable  bool   `json:"editable"`
}

type portsResponse struct {
	Strategy string         `json:"strategy"` // "compose", "docker", or "" when neither
	Ports    []portResponse `json:"ports"`
}

// Ports reports the published ports declared in a repo's container config and
// the deploy strategy that fits, so the configure UI can offer the project's
// real ports and generate the right deploy. owner and repo are required; ref
// (branch/sha) and dir (sub-directory) are optional. A repo with no detectable
// ports returns an empty array and strategy "", not an error.
func (h *GitHubHandler) Ports(w nethttp.ResponseWriter, r *nethttp.Request) {
	q := r.URL.Query()
	owner, repo := q.Get("owner"), q.Get("repo")
	if owner == "" || repo == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner or repo")
		return
	}

	ref := usecase.RepoRef{Owner: owner, Name: repo, Ref: q.Get("ref"), Dir: q.Get("dir")}
	ports, strategy, err := h.connector.DetectPorts(r.Context(), ref)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("detect ports failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not detect ports")
		return
	}

	out := portsResponse{Strategy: string(strategy), Ports: make([]portResponse, 0, len(ports))}
	for _, p := range ports {
		out.Ports = append(out.Ports, portResponse{
			Service:   p.Service,
			EnvVar:    p.EnvVar,
			Host:      p.Host,
			Container: p.Container,
			Editable:  p.Editable,
		})
	}
	h.writeJSON(w, nethttp.StatusOK, out)
}

// Tree lists every path in a repo at a ref so the configure UI can offer a
// directory/file picker instead of a free-text path. owner, repo and ref are
// all required. Not-connected is reported as 401.
func (h *GitHubHandler) Tree(w nethttp.ResponseWriter, r *nethttp.Request) {
	q := r.URL.Query()
	owner, repo, ref := q.Get("owner"), q.Get("repo"), q.Get("ref")
	if owner == "" || repo == "" || ref == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner, repo or ref")
		return
	}

	entries, err := h.tree.Tree(r.Context(), owner, repo, ref)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("list tree failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not list repository tree")
		return
	}
	h.writeJSON(w, nethttp.StatusOK, entries)
}

// EnvExample reports the variable names declared in a repo's example env file
// (.env.example or a common variant), so the configure UI can pre-fill the env
// var rows for the operator to fill in. owner and repo are required; ref
// (branch/sha) and dir (sub-directory) are optional. A repo with no example file
// returns an empty array, not an error.
func (h *GitHubHandler) EnvExample(w nethttp.ResponseWriter, r *nethttp.Request) {
	q := r.URL.Query()
	owner, repo := q.Get("owner"), q.Get("repo")
	if owner == "" || repo == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner or repo")
		return
	}

	ref := usecase.RepoRef{Owner: owner, Name: repo, Ref: q.Get("ref"), Dir: q.Get("dir")}
	keys, err := h.envExample.Keys(r.Context(), ref)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("read env example failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not read the example env file")
		return
	}

	out := make([]string, 0, len(keys))
	out = append(out, keys...)
	h.writeJSON(w, nethttp.StatusOK, out)
}

// RunSteps reports the latest deploy run's job and step progress for owner/repo
// on a branch, so the UI can show each GitHub Actions step's live status. owner
// and repo are required; ref is the branch (defaulting to "main"). A workflow
// with no run yet returns an empty result, not an error. Not-connected is
// reported as 401.
func (h *GitHubHandler) RunSteps(w nethttp.ResponseWriter, r *nethttp.Request) {
	q := r.URL.Query()
	owner, repo := q.Get("owner"), q.Get("repo")
	if owner == "" || repo == "" {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner or repo")
		return
	}
	ref := q.Get("ref")
	if ref == "" {
		ref = "main"
	}

	steps, err := h.runSteps.Steps(r.Context(), owner, repo, ref)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("read run steps failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not read the deploy run steps")
		return
	}
	if steps.Jobs == nil {
		steps.Jobs = []usecase.RunJob{}
	}
	h.writeJSON(w, nethttp.StatusOK, steps)
}

// JobLogs returns one job's plain-text log (split into lines) from the latest
// deploy run, so the walkthrough can show what each step printed and what
// failed. owner, repo and jobId are required. Not-connected is reported as 401.
func (h *GitHubHandler) JobLogs(w nethttp.ResponseWriter, r *nethttp.Request) {
	q := r.URL.Query()
	owner, repo := q.Get("owner"), q.Get("repo")
	jobID, _ := strconv.ParseInt(q.Get("jobId"), 10, 64)
	if owner == "" || repo == "" || jobID == 0 {
		h.writeError(w, nethttp.StatusBadRequest, "missing owner, repo or jobId")
		return
	}

	lines, err := h.runSteps.JobLog(r.Context(), owner, repo, jobID)
	if errors.Is(err, usecase.ErrNotConnected) {
		h.writeError(w, nethttp.StatusUnauthorized, "github not connected")
		return
	}
	if err != nil {
		h.log.Error("read job logs failed", "err", err)
		h.writeError(w, nethttp.StatusBadGateway, "could not read the job logs")
		return
	}

	out := make([]string, 0, len(lines))
	out = append(out, lines...)
	h.writeJSON(w, nethttp.StatusOK, map[string][]string{"lines": out})
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

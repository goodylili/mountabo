package usecase

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// Deployment is a repo+branch mountabo has configured to deploy to a server. It
// is persisted on a successful deploy so the monitor can show its history.
type Deployment struct {
	ID           string    `json:"id"`
	App          string    `json:"app"`
	Owner        string    `json:"owner"`
	Repo         string    `json:"repo"`
	Branch       string    `json:"branch"`
	Environment  string    `json:"environment"`
	ServerID     string    `json:"serverId"`
	WorkflowFile string    `json:"workflowFile"`
	CreatedAt    time.Time `json:"createdAt"`
}

// DeploymentStore persists configured deployments (a JSON file today). Save
// upserts by owner+repo+branch, so re-deploying updates the record rather than
// duplicating it.
type DeploymentStore interface {
	List() ([]Deployment, error)
	Save(d Deployment) error
}

// WorkflowRun is one GitHub Actions run of a deploy workflow.
type WorkflowRun struct {
	SHA        string
	Title      string
	Status     string // queued | in_progress | completed
	Conclusion string // success | failure | ... (set once completed)
	CreatedAt  time.Time
	UpdatedAt  time.Time
	// HTMLURL is the Actions run page on github.com, so the UI can link straight
	// to the run.
	HTMLURL string
}

// WorkflowRunLister lists recent runs of a repo's workflow file on a branch,
// newest first, capped to limit.
type WorkflowRunLister interface {
	ListWorkflowRuns(ctx context.Context, t Token, owner, repo, workflowFile, branch string, limit int) ([]WorkflowRun, error)
}

// RunView is one deploy run shaped for the UI.
type RunView struct {
	SHA       string `json:"sha"`
	Message   string `json:"message"`
	Status    string `json:"status"` // success | failed | running
	When      string `json:"when"`
	Duration  string `json:"duration"`
	RunURL    string `json:"runUrl"`    // the Actions run page on github.com
	CommitURL string `json:"commitUrl"` // the run's commit on github.com
}

// DeployEvent is one recorded deploy from the append-only tracking log.
type DeployEvent struct {
	At          time.Time
	Environment string
}

// DeployEventReader reads a target's deploy history from the tracking log: the
// most recent events (newest first, capped to limit) and the total count.
type DeployEventReader interface {
	DeployEvents(owner, repo, branch string, limit int) (events []DeployEvent, total int, err error)
}

// EventView is one tracked deploy shaped for the UI timeline.
type EventView struct {
	When        string `json:"when"`
	Environment string `json:"environment"`
}

// DeploymentStatus is a configured deployment plus its recent run history,
// ready for the monitor UI.
type DeploymentStatus struct {
	App    string `json:"app"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	// WorkflowURL is the deploy workflow file's Actions page on github.com (where
	// the run history lives). This is the value the field previously named "url"
	// carried; it was renamed so its meaning is explicit alongside LiveURL.
	WorkflowURL string `json:"workflowUrl"`
	// LiveURL points at the running app itself: the server's custom domain when
	// one is configured, otherwise the server's IP and the app's deploy port.
	// Empty when the app's address cannot be derived.
	LiveURL    string      `json:"liveUrl"`
	ServerID   string      `json:"serverId"`
	Status     string      `json:"status"` // live | idle | failing
	LastDeploy string      `json:"lastDeploy"`
	Runs       []RunView   `json:"runs"`
	Deploys    int         `json:"deploys"`  // total times deployed (from the tracking log)
	Timeline   []EventView `json:"timeline"` // recent deploys, newest first
}

// monitorRunLimit caps recent runs; monitorEventLimit caps the deploy timeline.
const (
	monitorRunLimit   = 5
	monitorEventLimit = 10
	monitorFetchLimit = 8 // concurrent per-deployment GitHub/DB fetches
)

// MonitorService reports deploy history: the configured deployments enriched
// with their recent GitHub Actions runs, read on the connected user's behalf.
type MonitorService struct {
	deployments DeploymentStore
	events      DeployEventReader
	tokens      TokenStore
	runs        WorkflowRunLister
	servers     ServerStore
}

// NewMonitorService wires the service to its ports. The server store is read to
// derive each deployment's live app URL (its domain, or its IP).
func NewMonitorService(deployments DeploymentStore, events DeployEventReader, tokens TokenStore, runs WorkflowRunLister, servers ServerStore) *MonitorService {
	return &MonitorService{deployments: deployments, events: events, tokens: tokens, runs: runs, servers: servers}
}

// History lists every configured deployment with its recent runs. It returns
// ErrNotConnected when no token is stored. A deployment whose runs can't be read
// (repo deleted, permissions, no runs yet) still appears, just with no runs.
func (s *MonitorService) History(ctx context.Context) ([]DeploymentStatus, error) {
	deployments, err := s.deployments.List()
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	if len(deployments) == 0 {
		return []DeploymentStatus{}, nil
	}

	token, err := s.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}

	// Index the servers by id so each deployment can derive its live app URL
	// (domain or IP) without a per-deployment lookup. A failure to read them is
	// not fatal: deployments still appear, just without a live URL.
	serversByID := map[string]Server{}
	if servers, serr := s.servers.List(); serr == nil {
		for _, sv := range servers {
			serversByID[sv.ID] = sv
		}
	}

	// Each deployment needs an independent GitHub Actions lookup, so fan them out
	// (bounded) rather than paying for them one after another: the page used to
	// block on the sum of every deployment's round-trip. Each goroutine writes
	// its own slot, so the result keeps the deployment order without locking.
	out := make([]DeploymentStatus, len(deployments))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(monitorFetchLimit)
	for i, d := range deployments {
		g.Go(func() error {
			// Best-effort per deployment: a repo that's deleted, lacks
			// permission, or has no runs yet still appears, just without runs.
			runs, _ := s.runs.ListWorkflowRuns(gctx, token, d.Owner, d.Repo, d.WorkflowFile, d.Branch, monitorRunLimit)
			events, total, _ := s.events.DeployEvents(d.Owner, d.Repo, d.Branch, monitorEventLimit)
			out[i] = buildStatus(d, serversByID[d.ServerID], runs, events, total)
			return nil
		})
	}
	_ = g.Wait() // goroutines are best-effort and never return an error; Wait only joins them
	return out, nil
}

// buildStatus assembles a deployment's UI status from its recent runs (newest
// first); the latest run sets the headline status and last-deploy time. server
// is the deployment's target (zero value when it can't be resolved), used to
// derive the live app URL.
func buildStatus(d Deployment, server Server, runs []WorkflowRun, events []DeployEvent, deploys int) DeploymentStatus {
	st := DeploymentStatus{
		App:         d.App,
		Repo:        d.Owner + "/" + d.Repo,
		Branch:      d.Branch,
		ServerID:    d.ServerID,
		WorkflowURL: fmt.Sprintf("https://github.com/%s/%s/actions/workflows/%s", d.Owner, d.Repo, d.WorkflowFile),
		LiveURL:     liveURL(server),
		Status:      "idle",
		LastDeploy:  "n/a",
		Runs:        make([]RunView, 0, len(runs)),
		Deploys:     deploys,
		Timeline:    make([]EventView, 0, len(events)),
	}
	for i, r := range runs {
		view := RunView{
			SHA:       shortSHA(r.SHA),
			Message:   r.Title,
			Status:    runStatus(r),
			When:      relativeTime(r.CreatedAt),
			Duration:  runDuration(r),
			RunURL:    r.HTMLURL,
			CommitURL: commitURL(d.Owner, d.Repo, r.SHA),
		}
		st.Runs = append(st.Runs, view)
		if i == 0 {
			st.Status = deployStatus(view.Status)
			st.LastDeploy = view.When
		}
	}
	for _, e := range events {
		st.Timeline = append(st.Timeline, EventView{When: relativeTime(e.At), Environment: e.Environment})
	}
	return st
}

// commitURL is the github.com page for a run's commit, or "" when the sha is
// unknown.
func commitURL(owner, repo, sha string) string {
	if sha == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s/commit/%s", owner, repo, sha)
}

// liveURL points at the running app on its server: a configured custom domain
// over HTTPS when one exists, otherwise the server's IP over HTTP with the
// app's port. The port is taken from a domain's upstream when one is set;
// absent that it is unknown, so it is omitted (a plain host URL). It returns ""
// when the server (and so its address) is unknown.
func liveURL(server Server) string {
	if len(server.Domains) > 0 {
		return "https://" + server.Domains[0].Host
	}
	if server.IP == "" {
		return ""
	}
	return "http://" + server.IP
}

// runStatus collapses GitHub's status+conclusion into the UI's three states.
func runStatus(r WorkflowRun) string {
	if r.Status != "completed" {
		return "running"
	}
	if r.Conclusion == "success" {
		return "success"
	}
	return "failed"
}

func deployStatus(latestRun string) string {
	switch latestRun {
	case "success", "running":
		return "live"
	case "failed":
		return "failing"
	default:
		return "idle"
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	if sha == "" {
		return "n/a"
	}
	return sha
}

func runDuration(r WorkflowRun) string {
	if r.Status != "completed" || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		return "n/a"
	}
	d := r.UpdatedAt.Sub(r.CreatedAt)
	if d < 0 {
		return "n/a"
	}
	if m := int(d.Minutes()); m > 0 {
		return fmt.Sprintf("%dm %ds", m, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	switch d := time.Since(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

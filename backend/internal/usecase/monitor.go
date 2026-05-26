package usecase

import (
	"context"
	"fmt"
	"time"
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
}

// WorkflowRunLister lists recent runs of a repo's workflow file on a branch,
// newest first, capped to limit.
type WorkflowRunLister interface {
	ListWorkflowRuns(ctx context.Context, t Token, owner, repo, workflowFile, branch string, limit int) ([]WorkflowRun, error)
}

// RunView is one deploy run shaped for the UI.
type RunView struct {
	SHA      string `json:"sha"`
	Message  string `json:"message"`
	Status   string `json:"status"` // success | failed | running
	When     string `json:"when"`
	Duration string `json:"duration"`
}

// DeploymentStatus is a configured deployment plus its recent run history,
// ready for the monitor UI.
type DeploymentStatus struct {
	App        string    `json:"app"`
	Repo       string    `json:"repo"`
	Branch     string    `json:"branch"`
	ServerID   string    `json:"serverId"`
	URL        string    `json:"url"`
	Status     string    `json:"status"` // live | idle | failing
	LastDeploy string    `json:"lastDeploy"`
	Runs       []RunView `json:"runs"`
}

// monitorRunLimit caps how many recent runs the monitor shows per deployment.
const monitorRunLimit = 5

// MonitorService reports deploy history: the configured deployments enriched
// with their recent GitHub Actions runs, read on the connected user's behalf.
type MonitorService struct {
	deployments DeploymentStore
	tokens      TokenStore
	runs        WorkflowRunLister
}

// NewMonitorService wires the service to its ports.
func NewMonitorService(deployments DeploymentStore, tokens TokenStore, runs WorkflowRunLister) *MonitorService {
	return &MonitorService{deployments: deployments, tokens: tokens, runs: runs}
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

	out := make([]DeploymentStatus, 0, len(deployments))
	for _, d := range deployments {
		runs, _ := s.runs.ListWorkflowRuns(ctx, token, d.Owner, d.Repo, d.WorkflowFile, d.Branch, monitorRunLimit) // best-effort
		out = append(out, buildStatus(d, runs))
	}
	return out, nil
}

// buildStatus assembles a deployment's UI status from its recent runs (newest
// first); the latest run sets the headline status and last-deploy time.
func buildStatus(d Deployment, runs []WorkflowRun) DeploymentStatus {
	st := DeploymentStatus{
		App:        d.App,
		Repo:       d.Owner + "/" + d.Repo,
		Branch:     d.Branch,
		ServerID:   d.ServerID,
		URL:        fmt.Sprintf("https://github.com/%s/%s/actions/workflows/%s", d.Owner, d.Repo, d.WorkflowFile),
		Status:     "idle",
		LastDeploy: "n/a",
		Runs:       make([]RunView, 0, len(runs)),
	}
	for i, r := range runs {
		view := RunView{
			SHA:      shortSHA(r.SHA),
			Message:  r.Title,
			Status:   runStatus(r),
			When:     relativeTime(r.CreatedAt),
			Duration: runDuration(r),
		}
		st.Runs = append(st.Runs, view)
		if i == 0 {
			st.Status = deployStatus(view.Status)
			st.LastDeploy = view.When
		}
	}
	return st
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

package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRunLister struct {
	runs map[string][]WorkflowRun // keyed "owner/repo"
}

func (f fakeRunLister) ListWorkflowRuns(_ context.Context, _ Token, owner, repo, _, _ string, _ int) ([]WorkflowRun, error) {
	return f.runs[owner+"/"+repo], nil
}

type fakeEventReader struct {
	events []DeployEvent
	total  int
}

func (f fakeEventReader) DeployEvents(_, _, _ string, _ int) ([]DeployEvent, int, error) {
	return f.events, f.total, nil
}

func TestMonitorHistory_EnrichesWithRuns(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", ServerID: "s1", WorkflowFile: "mountabo-deploy-main.yml"},
	}}
	now := time.Now()
	runs := fakeRunLister{runs: map[string][]WorkflowRun{
		"acme/shop": {
			{SHA: "abcdef1234", Title: "latest", Status: "completed", Conclusion: "success", CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-1 * time.Minute)},
			{SHA: "0000000", Title: "older", Status: "completed", Conclusion: "failure", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		},
	}}
	events := fakeEventReader{total: 3, events: []DeployEvent{
		{At: now.Add(-time.Minute), Environment: "main"},
		{At: now.Add(-time.Hour), Environment: "main"},
	}}
	svc := NewMonitorService(deps, events, &fakeStore{loadTok: Token{AccessToken: "tok"}}, runs)

	got, err := svc.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(got))
	}
	d := got[0]
	// Latest run is a success, so the deployment reads as live.
	if d.Repo != "acme/shop" || d.Status != "live" {
		t.Errorf("unexpected status: %+v", d)
	}
	// Tracking: total deploy count + recent timeline surfaced.
	if d.Deploys != 3 {
		t.Errorf("deploys = %d, want 3", d.Deploys)
	}
	if len(d.Timeline) != 2 || d.Timeline[0].Environment != "main" {
		t.Errorf("timeline wrong: %+v", d.Timeline)
	}
	if len(d.Runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(d.Runs))
	}
	if d.Runs[0].SHA != "abcdef1" || d.Runs[0].Status != "success" || d.Runs[0].Duration != "1m 0s" {
		t.Errorf("run[0] wrong: %+v", d.Runs[0])
	}
	if d.Runs[1].Status != "failed" {
		t.Errorf("run[1] status = %s, want failed", d.Runs[1].Status)
	}
}

func TestMonitorHistory_EmptyWithoutDeployments(t *testing.T) {
	svc := NewMonitorService(&fakeDeploymentStore{}, fakeEventReader{}, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{})
	got, err := svc.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want no deployments, got %d", len(got))
	}
}

func TestMonitorHistory_NotConnected(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{{Owner: "a", Repo: "r", Branch: "main"}}}
	svc := NewMonitorService(deps, fakeEventReader{}, &fakeStore{loadErr: ErrNotConnected}, fakeRunLister{})
	if _, err := svc.History(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("want ErrNotConnected, got %v", err)
	}
}

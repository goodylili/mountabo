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

// fakeContainerTeardown records the apps it was asked to remove, so a test can
// assert the container teardown ran for the right app.
type fakeContainerTeardown struct {
	removed []string
	err     error
}

func (f *fakeContainerTeardown) RemoveApp(_ context.Context, _ SSHTarget, app string) error {
	f.removed = append(f.removed, app)
	return f.err
}

// fakeRepoFileDeleter records the repo paths it was asked to delete.
type fakeRepoFileDeleter struct {
	deleted []string
	err     error
}

func (f *fakeRepoFileDeleter) DeleteFile(_ context.Context, _ Token, owner, repo, _, path, _ string) error {
	f.deleted = append(f.deleted, owner+"/"+repo+":"+path)
	return f.err
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
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7"})
	svc := NewMonitorService(deps, events, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, runs, servers, newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, nil)

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
	// With no domain, the live URL is the server's IP over HTTP.
	if d.LiveURL != "http://203.0.113.7" {
		t.Errorf("liveURL = %q, want http://203.0.113.7", d.LiveURL)
	}
	if d.WorkflowURL != "https://github.com/acme/shop/actions/workflows/mountabo-deploy-main.yml" {
		t.Errorf("workflowURL = %q", d.WorkflowURL)
	}
	if d.Runs[0].CommitURL != "https://github.com/acme/shop/commit/abcdef1234" {
		t.Errorf("run[0] commitURL = %q", d.Runs[0].CommitURL)
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
	svc := NewMonitorService(&fakeDeploymentStore{}, fakeEventReader{}, &fakeDeploymentStore{}, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, nil)
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
	svc := NewMonitorService(deps, fakeEventReader{}, deps, &fakeStore{loadErr: ErrNotConnected}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, nil)
	if _, err := svc.History(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("want ErrNotConnected, got %v", err)
	}
}

func TestMonitorDelete_TearsDownContainerWorkflowAndRecord(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", ServerID: "s1", WorkflowFile: "mountabo-deploy-main.yml"},
	}}
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7", SSHPort: 22, Status: StatusReady})
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	teardown := &fakeContainerTeardown{}
	files := &fakeRepoFileDeleter{}
	svc := NewMonitorService(deps, fakeEventReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, servers, vault, teardown, files, nil)

	if err := svc.Delete(context.Background(), "shop"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The container was stopped for the right app.
	if len(teardown.removed) != 1 || teardown.removed[0] != "shop" {
		t.Errorf("container teardown = %v, want [shop]", teardown.removed)
	}
	// Both committed artifacts were deleted from the repo.
	wantFiles := []string{"acme/shop:deploy.sh", "acme/shop:.github/workflows/mountabo-deploy-main.yml"}
	if len(files.deleted) != len(wantFiles) {
		t.Fatalf("deleted files = %v, want %v", files.deleted, wantFiles)
	}
	for i, want := range wantFiles {
		if files.deleted[i] != want {
			t.Errorf("deleted[%d] = %q, want %q", i, files.deleted[i], want)
		}
	}
	// The record is gone.
	if got, _ := deps.List(); len(got) != 0 {
		t.Errorf("record still present after delete: %v", got)
	}
}

func TestMonitorDelete_NotFound(t *testing.T) {
	deps := &fakeDeploymentStore{}
	svc := NewMonitorService(deps, fakeEventReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, nil)
	if err := svc.Delete(context.Background(), "missing"); !errors.Is(err, ErrDeploymentNotFound) {
		t.Fatalf("want ErrDeploymentNotFound, got %v", err)
	}
}

func TestMonitorDelete_RemovesRecordEvenWhenTeardownFails(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", ServerID: "s1", WorkflowFile: "mountabo-deploy-main.yml"},
	}}
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7", SSHPort: 22, Status: StatusReady})
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	// Both the container removal and the repo file deletion fail; the record must
	// still be removed so the user is never stuck.
	teardown := &fakeContainerTeardown{err: errors.New("ssh down")}
	files := &fakeRepoFileDeleter{err: errors.New("github down")}
	svc := NewMonitorService(deps, fakeEventReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, servers, vault, teardown, files, nil)

	if err := svc.Delete(context.Background(), "shop"); err != nil {
		t.Fatalf("Delete should succeed despite soft teardown failures, got %v", err)
	}
	if got, _ := deps.List(); len(got) != 0 {
		t.Errorf("record still present after delete: %v", got)
	}
}

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

type fakeReleaseReader struct {
	releases []Release
	total    int
}

func (f fakeReleaseReader) Releases(_, _, _ string, _ int) ([]Release, int, error) {
	return f.releases, f.total, nil
}

// fakeContainerTeardown records the apps it was asked to remove, so a test can
// assert the container teardown ran for the right app.
type fakeContainerTeardown struct {
	removed []string
	err     error
}

func (f *fakeContainerTeardown) RemoveApp(_ context.Context, _ SSHTarget, app, _ string) error {
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

// fakeRepoEnvDeleter records every environment it was asked to delete, so tests
// can assert that teardown undoes the deployment environment.
type fakeRepoEnvDeleter struct {
	deleted []string
	err     error
}

func (f *fakeRepoEnvDeleter) DeleteEnvironment(_ context.Context, _ Token, owner, repo, name string) error {
	f.deleted = append(f.deleted, owner+"/"+repo+":"+name)
	return f.err
}

// fakeRepoKeyDeleter records every (repo, title prefix) it was asked to clear.
type fakeRepoKeyDeleter struct {
	cleared []string
	err     error
}

func (f *fakeRepoKeyDeleter) DeleteDeployKey(_ context.Context, _ Token, owner, repo, titlePrefix string) error {
	f.cleared = append(f.cleared, owner+"/"+repo+":"+titlePrefix)
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
	// Releases are stored as pending; the read path overlays live status/run URL
	// from the run that built each (matched by commit SHA). The first matches the
	// successful run; the second has no matching run and keeps its stored status.
	releases := fakeReleaseReader{total: 3, releases: []Release{
		{At: now.Add(-time.Minute), Environment: "main", CommitSHA: "abcdef1234", Status: "pending", Actor: "octocat"},
		{At: now.Add(-time.Hour), Environment: "main", CommitSHA: "deadbeef", Status: "pending"},
	}}
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7"})
	svc := NewMonitorService(deps, releases, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, runs, servers, newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)

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
	// First release joins the successful run by SHA: live status + actor + short SHA.
	if d.Timeline[0].Status != "success" || d.Timeline[0].SHA != "abcdef1" || d.Timeline[0].Actor != "octocat" {
		t.Errorf("release[0] overlay wrong: %+v", d.Timeline[0])
	}
	// Second release has no matching run, so it keeps its stored pending status.
	if d.Timeline[1].Status != "pending" {
		t.Errorf("release[1] status = %q, want pending", d.Timeline[1].Status)
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

func TestMonitorHistory_SurfacesDeployLockAndClears(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", Environment: "production", ServerID: "s1", WorkflowFile: "mountabo-deploy-production.yml"},
	}}
	locker := NewDeployLocker()
	locker.TryAcquire(lockKey("acme", "shop", "production")) // a deploy to this environment is in flight
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, locker, nil)

	got, err := svc.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 1 || !got[0].Locked {
		t.Fatalf("want a locked deployment, got %+v", got)
	}
	if got[0].LockedSince == "" {
		t.Error("LockedSince should be set while locked")
	}

	// ClearLock frees it, and a subsequent read shows it unlocked.
	if !svc.ClearLock("acme", "shop", "production") {
		t.Error("ClearLock should report it cleared a held lock")
	}
	got, err = svc.History(context.Background())
	if err != nil {
		t.Fatalf("History after clear: %v", err)
	}
	if got[0].Locked {
		t.Error("deployment should be unlocked after ClearLock")
	}
}

// TestLiveURL_MatchesDeploymentsOwnDomain guards that a deployment's live URL is
// the domain whose upstream is ITS port — not just the server's first domain
// (a server can front several deployments, each with its own domain).
func TestLiveURL_MatchesDeploymentsOwnDomain(t *testing.T) {
	server := Server{IP: "203.0.113.7", Domains: []Domain{
		{Host: "other.example.com", Upstream: "8080"},
		{Host: "meals.example.com", Upstream: "9090"},
	}}
	if got := liveURL(server, 9090); got != "https://meals.example.com" {
		t.Errorf("liveURL(port 9090) = %q, want its own domain", got)
	}
	// A deployment with no recorded port (0) claims no domain — falls back to IP,
	// rather than borrowing the server's first domain.
	if got := liveURL(server, 0); got != "http://203.0.113.7" {
		t.Errorf("liveURL(port 0) = %q, want the server IP fallback", got)
	}
}

func TestMonitorSetPort_RecordsPortForAppEnvironment(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Environment: "production", ServerID: "s1"},
	}}
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)

	if err := svc.SetPort("acme", "shop", "production", 8010); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if deps.saved[0].Port != 8010 {
		t.Errorf("recorded port = %d, want 8010", deps.saved[0].Port)
	}
	if err := svc.SetPort("acme", "shop", "staging", 8010); !errors.Is(err, ErrDeploymentNotFound) {
		t.Errorf("SetPort for a missing environment = %v, want ErrDeploymentNotFound", err)
	}
}

func TestMonitorHistory_EmptyWithoutDeployments(t *testing.T) {
	svc := NewMonitorService(&fakeDeploymentStore{}, fakeReleaseReader{}, &fakeDeploymentStore{}, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, &fakeDeploymentStore{}, NewDeployLocker(), nil)
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
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadErr: ErrNotConnected}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)
	if _, err := svc.History(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("want ErrNotConnected, got %v", err)
	}
}

func TestMonitorDelete_TearsDownContainerWorkflowAndRecord(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", Environment: "production", ServerID: "s1", WorkflowFile: "mountabo-deploy-production.yml"},
	}}
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7", SSHPort: 22, Status: StatusReady})
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	teardown := &fakeContainerTeardown{}
	files := &fakeRepoFileDeleter{}
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, servers, vault, teardown, files, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)

	if err := svc.Delete(context.Background(), "acme", "shop", "production"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The container was stopped for the right app.
	if len(teardown.removed) != 1 || teardown.removed[0] != "shop" {
		t.Errorf("container teardown = %v, want [shop]", teardown.removed)
	}
	// Both committed artifacts were deleted from the repo.
	wantFiles := []string{"acme/shop:deploy.sh", "acme/shop:.github/workflows/mountabo-deploy-production.yml"}
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
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, newMemServerStore(), newFakeVault(), &fakeContainerTeardown{}, &fakeRepoFileDeleter{}, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)
	if err := svc.Delete(context.Background(), "acme", "shop", "missing"); !errors.Is(err, ErrDeploymentNotFound) {
		t.Fatalf("want ErrDeploymentNotFound, got %v", err)
	}
}

func TestMonitorDelete_RemovesRecordEvenWhenTeardownFails(t *testing.T) {
	deps := &fakeDeploymentStore{saved: []Deployment{
		{App: "shop", Owner: "acme", Repo: "shop", Branch: "main", Environment: "production", ServerID: "s1", WorkflowFile: "mountabo-deploy-production.yml"},
	}}
	servers := newMemServerStore()
	_ = servers.Save(Server{ID: "s1", IP: "203.0.113.7", SSHPort: 22, Status: StatusReady})
	vault := newFakeVault()
	_ = vault.SaveSecret(privateKeyKey("s1"), "KEY-PEM")
	// Both the container removal and the repo file deletion fail; the record must
	// still be removed so the user is never stuck.
	teardown := &fakeContainerTeardown{err: errors.New("ssh down")}
	files := &fakeRepoFileDeleter{err: errors.New("github down")}
	svc := NewMonitorService(deps, fakeReleaseReader{}, deps, &fakeStore{loadTok: Token{AccessToken: "tok"}}, fakeRunLister{}, servers, vault, teardown, files, &fakeRepoEnvDeleter{}, &fakeRepoKeyDeleter{}, deps, NewDeployLocker(), nil)

	if err := svc.Delete(context.Background(), "acme", "shop", "production"); err != nil {
		t.Fatalf("Delete should succeed despite soft teardown failures, got %v", err)
	}
	if got, _ := deps.List(); len(got) != 0 {
		t.Errorf("record still present after delete: %v", got)
	}
}

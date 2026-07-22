package repository

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
)

func TestDeploymentSQL_UpsertsStateAndTracksEvents(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = db.Close() }()
	store := NewDeploymentSQL(db)

	if list, err := store.List(); err != nil || len(list) != 0 {
		t.Fatalf("fresh db: list=%v err=%v", list, err)
	}

	first := usecase.Deployment{
		ID: "d1", App: "shop", Owner: "acme", Repo: "shop", Branch: "main",
		Environment: "main", ServerID: "s1", WorkflowFile: "mountabo-deploy-main.yml",
		Port:      8080,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Save(first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	// Re-deploy the same target with changed fields: state upserts (still one
	// row), id + created_at stay original, other fields update.
	second := first
	second.ID = "ignored"
	second.ServerID = "s2"
	second.CreatedAt = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Save(second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(list))
	}
	got := list[0]
	if got.ID != "d1" {
		t.Errorf("id = %q, want the original d1 kept on upsert", got.ID)
	}
	if got.ServerID != "s2" {
		t.Errorf("serverID = %q, want updated to s2", got.ServerID)
	}
	if got.Port != 8080 {
		t.Errorf("port = %d, want 8080 persisted", got.Port)
	}
	if !got.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("createdAt = %v, want the original first-deploy time", got.CreatedAt)
	}

	// Both deploys are tracked in the append-only event log.
	var events int
	if err := db.QueryRow(`SELECT COUNT(*) FROM deploy_events`).Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 2 {
		t.Errorf("deploy_events = %d, want 2", events)
	}

	// Deleting by app removes the state row and its whole event history, and
	// reports that something was removed; deleting an unknown app reports false.
	removed, err := store.DeleteByApp("shop")
	if err != nil || !removed {
		t.Fatalf("DeleteByApp(shop): removed=%v err=%v", removed, err)
	}
	if list, err := store.List(); err != nil || len(list) != 0 {
		t.Fatalf("after delete: list=%v err=%v", list, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM deploy_events`).Scan(&events); err != nil {
		t.Fatalf("count events after delete: %v", err)
	}
	if events != 0 {
		t.Errorf("deploy_events after delete = %d, want 0", events)
	}
	if removed, err := store.DeleteByApp("missing"); err != nil || removed {
		t.Errorf("DeleteByApp(missing): removed=%v err=%v, want false/nil", removed, err)
	}
}

func TestDeploymentSQL_RecordsReleasesAndLinksPrevious(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = db.Close() }()
	store := NewDeploymentSQL(db)

	base := usecase.Deployment{
		ID: "d1", App: "shop", Owner: "acme", Repo: "shop", Branch: "main",
		Environment: "main", ServerID: "s1", WorkflowFile: "mountabo-deploy-main.yml",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	first := base
	first.CommitSHA, first.Actor = "aaa111", "octocat"
	if err := store.Save(first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	second := base
	second.CommitSHA, second.Actor = "bbb222", "hubot"
	if err := store.Save(second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	releases, total, err := store.Releases("acme", "shop", "main", 10)
	if err != nil {
		t.Fatalf("Releases: %v", err)
	}
	if total != 2 || len(releases) != 2 {
		t.Fatalf("releases total=%d len=%d, want 2/2", total, len(releases))
	}
	// Newest first: the second deploy leads.
	newest, oldest := releases[0], releases[1]
	if newest.CommitSHA != "bbb222" || newest.Actor != "hubot" {
		t.Errorf("newest commit/actor = %q/%q, want bbb222/hubot", newest.CommitSHA, newest.Actor)
	}
	if newest.Status != "pending" {
		t.Errorf("newest status = %q, want pending", newest.Status)
	}
	// The newest release links back to the oldest; the first release links to nothing.
	if newest.PrevReleaseID == nil || *newest.PrevReleaseID != oldest.ID {
		t.Errorf("newest prevReleaseID = %v, want %d", newest.PrevReleaseID, oldest.ID)
	}
	if oldest.PrevReleaseID != nil {
		t.Errorf("oldest prevReleaseID = %v, want nil", *oldest.PrevReleaseID)
	}
}

// TestOpenSQLite_MigratesLegacyDeployEvents proves an old database whose
// deploy_events table predates the release columns is migrated in place: opening
// it adds the columns without error and existing rows read back with defaults.
func TestOpenSQLite_MigratesLegacyDeployEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Build a pre-release-columns deploy_events table and seed one row.
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := legacy.Exec(`CREATE TABLE deploy_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner TEXT NOT NULL, repo TEXT NOT NULL, branch TEXT NOT NULL,
		environment TEXT NOT NULL, server_id TEXT NOT NULL, app TEXT NOT NULL, at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := legacy.Exec(`INSERT INTO deploy_events (owner, repo, branch, environment, server_id, app, at)
		VALUES ('acme','shop','main','main','s1','shop','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	_ = legacy.Close()

	// Opening through OpenSQLite must add the release columns idempotently.
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite on legacy db: %v", err)
	}
	defer func() { _ = db.Close() }()

	releases, total, err := NewDeploymentSQL(db).Releases("acme", "shop", "main", 10)
	if err != nil {
		t.Fatalf("Releases after migration: %v", err)
	}
	if total != 1 || len(releases) != 1 {
		t.Fatalf("releases total=%d len=%d, want 1/1", total, len(releases))
	}
	// The legacy row reads back with defaulted release fields.
	if r := releases[0]; r.Status != "pending" || r.CommitSHA != "" || r.PrevReleaseID != nil {
		t.Errorf("migrated row = %+v, want pending/empty/nil defaults", r)
	}
}

package repository

import (
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
}

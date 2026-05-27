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

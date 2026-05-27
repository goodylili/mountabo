package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO), registers "sqlite"
)

// schema is applied on open; idempotent so it runs every startup. deployments
// holds the current state (one row per target); deploy_events is an append-only
// log so deploy history is tracked and queryable over time.
const schema = `
CREATE TABLE IF NOT EXISTS deployments (
	owner TEXT NOT NULL,
	repo TEXT NOT NULL,
	branch TEXT NOT NULL,
	id TEXT NOT NULL,
	app TEXT NOT NULL,
	environment TEXT NOT NULL,
	server_id TEXT NOT NULL,
	workflow_file TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (owner, repo, branch)
);
CREATE TABLE IF NOT EXISTS deploy_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner TEXT NOT NULL,
	repo TEXT NOT NULL,
	branch TEXT NOT NULL,
	environment TEXT NOT NULL,
	server_id TEXT NOT NULL,
	app TEXT NOT NULL,
	at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deploy_events_target ON deploy_events(owner, repo, branch, at);
`

// OpenSQLite opens (creating if needed) the SQLite database at path and applies
// the schema. The caller owns the returned *sql.DB and must Close it.
func OpenSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

// DeploymentSQL persists configured deployments in SQLite and records an
// append-only event on every deploy, for durable tracking. It satisfies
// usecase.DeploymentStore.
type DeploymentSQL struct {
	db *sql.DB
}

var (
	_ usecase.DeploymentStore   = (*DeploymentSQL)(nil)
	_ usecase.DeployEventReader = (*DeploymentSQL)(nil)
)

// NewDeploymentSQL returns a SQLite-backed deployment store over an open db.
func NewDeploymentSQL(db *sql.DB) *DeploymentSQL {
	return &DeploymentSQL{db: db}
}

// List returns the current deployments (one per target), oldest first.
func (s *DeploymentSQL) List() ([]usecase.Deployment, error) {
	rows, err := s.db.Query(`SELECT id, app, owner, repo, branch, environment, server_id, workflow_file, created_at
		FROM deployments ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("query deployments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []usecase.Deployment
	for rows.Next() {
		var d usecase.Deployment
		var createdAt string
		if err := rows.Scan(&d.ID, &d.App, &d.Owner, &d.Repo, &d.Branch, &d.Environment, &d.ServerID, &d.WorkflowFile, &createdAt); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

// Save upserts the deployment by owner+repo+branch, keeping the original id and
// first-seen created_at on re-deploy, and appends a deploy event for tracking.
// Both writes share one transaction so history never drifts from state.
func (s *DeploymentSQL) Save(d usecase.Deployment) error {
	createdAt := d.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Keep the original id and created_at on conflict (first-deploy time), so
	// the deploy_events log carries the per-deploy history instead.
	if _, err := tx.Exec(`INSERT INTO deployments
		(owner, repo, branch, id, app, environment, server_id, workflow_file, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner, repo, branch) DO UPDATE SET
			app = excluded.app,
			environment = excluded.environment,
			server_id = excluded.server_id,
			workflow_file = excluded.workflow_file`,
		d.Owner, d.Repo, d.Branch, d.ID, d.App, d.Environment, d.ServerID, d.WorkflowFile,
		createdAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("upsert deployment: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO deploy_events
		(owner, repo, branch, environment, server_id, app, at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.Owner, d.Repo, d.Branch, d.Environment, d.ServerID, d.App, now); err != nil {
		return fmt.Errorf("record deploy event: %w", err)
	}
	return tx.Commit()
}

// DeployEvents returns a target's most recent deploys (newest first, capped to
// limit) from the tracking log, plus the total number ever recorded.
func (s *DeploymentSQL) DeployEvents(owner, repo, branch string, limit int) ([]usecase.DeployEvent, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM deploy_events WHERE owner = ? AND repo = ? AND branch = ?`,
		owner, repo, branch).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count deploy events: %w", err)
	}

	rows, err := s.db.Query(`SELECT at, environment FROM deploy_events
		WHERE owner = ? AND repo = ? AND branch = ?
		ORDER BY at DESC, id DESC LIMIT ?`, owner, repo, branch, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("query deploy events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []usecase.DeployEvent
	for rows.Next() {
		var e usecase.DeployEvent
		var at string
		if err := rows.Scan(&at, &e.Environment); err != nil {
			return nil, 0, fmt.Errorf("scan deploy event: %w", err)
		}
		e.At, _ = time.Parse(time.RFC3339, at)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO), registers "sqlite"
)

// schema is applied on open; idempotent so it runs every startup. deployments
// holds the current state (one row per target); deploy_events is an append-only
// log so deploy history is tracked and queryable over time. Each deploy_events
// row is a release: the extra columns (commit_sha, image_ref, status, health,
// actor, run_url, prev_event_id) turn the bare event into a durable release
// record that rollback and release history build on. Fields known only after a
// deploy runs (image_ref, health, and the terminal status/run_url) are provisioned
// here and populated by later phases; status defaults to 'pending'.
// The deployment identity is (owner, repo, environment): a repo can have several
// first-class environments (production, staging, ...), each with its own branch,
// server, domain and release history. branch is an attribute of the environment,
// not part of the key. Older databases keyed by (owner, repo, branch) are
// rebuilt in place on open (see migrateDeploymentsKey); since environment
// defaulted to the branch, that rebuild is a no-op for existing data.
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
	port INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	PRIMARY KEY (owner, repo, environment)
);
CREATE TABLE IF NOT EXISTS deploy_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner TEXT NOT NULL,
	repo TEXT NOT NULL,
	branch TEXT NOT NULL,
	environment TEXT NOT NULL,
	server_id TEXT NOT NULL,
	app TEXT NOT NULL,
	at TEXT NOT NULL,
	commit_sha TEXT NOT NULL DEFAULT '',
	image_ref TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	health TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT '',
	run_url TEXT NOT NULL DEFAULT '',
	prev_event_id INTEGER
);
CREATE INDEX IF NOT EXISTS idx_deploy_events_target ON deploy_events(owner, repo, environment, at);
`

// releaseColumns are the deploy_events columns added after the table first
// shipped. CREATE TABLE IF NOT EXISTS leaves an existing table's columns
// untouched, so each is added via ALTER TABLE on open; a "duplicate column"
// error just means a current-schema DB already has it. Kept in one place so the
// migration and the release schema never drift.
var releaseColumns = []string{
	`ALTER TABLE deploy_events ADD COLUMN commit_sha TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE deploy_events ADD COLUMN image_ref TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE deploy_events ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'`,
	`ALTER TABLE deploy_events ADD COLUMN health TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE deploy_events ADD COLUMN actor TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE deploy_events ADD COLUMN run_url TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE deploy_events ADD COLUMN prev_event_id INTEGER`,
}

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
	// Migrate databases created before the port column existed. CREATE TABLE IF
	// NOT EXISTS leaves an old table's columns untouched, so add it here; a
	// "duplicate column" error just means a current-schema DB already has it.
	if _, err := db.Exec(`ALTER TABLE deployments ADD COLUMN port INTEGER NOT NULL DEFAULT 0`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		_ = db.Close()
		return nil, fmt.Errorf("add port column: %w", err)
	}
	// Migrate databases created before deploy_events carried release fields. Same
	// idempotent ALTER pattern: a "duplicate column" error means a current-schema
	// DB already has the column, anything else is a real failure.
	for _, stmt := range releaseColumns {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			_ = db.Close()
			return nil, fmt.Errorf("add release column: %w", err)
		}
	}
	// Migrate databases whose deployments table still keys by (owner, repo,
	// branch) to the environment key. Runs after the port migration so the rebuild
	// can carry the port column across.
	if err := migrateDeploymentsKey(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("re-key deployments: %w", err)
	}
	return db, nil
}

// migrateDeploymentsKey rebuilds an old deployments table (PRIMARY KEY on
// branch) under the environment key. SQLite cannot alter a primary key in place,
// so it copies into a new table and swaps. It is a no-op when the table is
// already environment-keyed (fresh DBs, or an already-migrated one). Rows are
// deduped to the latest created_at per (owner, repo, environment) — a collision
// only arises if an operator explicitly reused one environment name across
// branches, which never happened while environment defaulted to the branch.
func migrateDeploymentsKey(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(deployments)`)
	if err != nil {
		return fmt.Errorf("inspect deployments: %w", err)
	}
	branchIsKey := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan column info: %w", err)
		}
		if name == "branch" && pk > 0 {
			branchIsKey = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	if !branchIsKey {
		return nil // already environment-keyed
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`CREATE TABLE deployments_new (
		owner TEXT NOT NULL,
		repo TEXT NOT NULL,
		branch TEXT NOT NULL,
		id TEXT NOT NULL,
		app TEXT NOT NULL,
		environment TEXT NOT NULL,
		server_id TEXT NOT NULL,
		workflow_file TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		PRIMARY KEY (owner, repo, environment)
	)`); err != nil {
		return fmt.Errorf("create rebuilt table: %w", err)
	}
	// OR REPLACE + ORDER BY created_at ASC: rows are inserted oldest-first, so on
	// an (owner, repo, environment) conflict the latest deploy wins.
	if _, err := tx.Exec(`INSERT OR REPLACE INTO deployments_new
		(owner, repo, branch, id, app, environment, server_id, workflow_file, port, created_at)
		SELECT owner, repo, branch, id, app, environment, server_id, workflow_file, port, created_at
		FROM deployments ORDER BY created_at ASC`); err != nil {
		return fmt.Errorf("copy rows: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE deployments`); err != nil {
		return fmt.Errorf("drop old table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE deployments_new RENAME TO deployments`); err != nil {
		return fmt.Errorf("rename rebuilt table: %w", err)
	}
	return tx.Commit()
}

// DeploymentSQL persists configured deployments in SQLite and records an
// append-only event on every deploy, for durable tracking. It satisfies
// usecase.DeploymentStore.
type DeploymentSQL struct {
	db *sql.DB
}

var (
	_ usecase.DeploymentStore      = (*DeploymentSQL)(nil)
	_ usecase.ReleaseReader        = (*DeploymentSQL)(nil)
	_ usecase.DeploymentDeleter    = (*DeploymentSQL)(nil)
	_ usecase.DeploymentPortSetter = (*DeploymentSQL)(nil)
)

// NewDeploymentSQL returns a SQLite-backed deployment store over an open db.
func NewDeploymentSQL(db *sql.DB) *DeploymentSQL {
	return &DeploymentSQL{db: db}
}

// List returns the current deployments (one per target), oldest first.
func (s *DeploymentSQL) List() ([]usecase.Deployment, error) {
	rows, err := s.db.Query(`SELECT id, app, owner, repo, branch, environment, server_id, workflow_file, port, created_at
		FROM deployments ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("query deployments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []usecase.Deployment
	for rows.Next() {
		var d usecase.Deployment
		var createdAt string
		if err := rows.Scan(&d.ID, &d.App, &d.Owner, &d.Repo, &d.Branch, &d.Environment, &d.ServerID, &d.WorkflowFile, &d.Port, &createdAt); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

// Save upserts the deployment by owner+repo+environment, keeping the original id
// and first-seen created_at on re-deploy, and appends a deploy event for
// tracking. Both writes share one transaction so history never drifts from state.
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
		(owner, repo, branch, id, app, environment, server_id, workflow_file, port, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner, repo, environment) DO UPDATE SET
			branch = excluded.branch,
			app = excluded.app,
			server_id = excluded.server_id,
			workflow_file = excluded.workflow_file,
			port = excluded.port`,
		d.Owner, d.Repo, d.Branch, d.ID, d.App, d.Environment, d.ServerID, d.WorkflowFile, d.Port,
		createdAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("upsert deployment: %w", err)
	}

	// Link this release to the environment's previous one (NULL for the first), so
	// release history is a chain rollback can walk. Computed inside the same
	// transaction so a concurrent deploy cannot slot a release in between.
	var prevID sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(id) FROM deploy_events WHERE owner = ? AND repo = ? AND environment = ?`,
		d.Owner, d.Repo, d.Environment).Scan(&prevID); err != nil {
		return fmt.Errorf("find previous release: %w", err)
	}

	// A release is 'pending' until its GitHub Actions run completes; the terminal
	// status and run_url are overlaid live from the run at read time. image_ref
	// and health are provisioned but populated by later phases.
	if _, err := tx.Exec(`INSERT INTO deploy_events
		(owner, repo, branch, environment, server_id, app, at, commit_sha, actor, status, image_ref, health, run_url, prev_event_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', '', '', '', ?)`,
		d.Owner, d.Repo, d.Branch, d.Environment, d.ServerID, d.App, now, d.CommitSHA, d.Actor, prevID); err != nil {
		return fmt.Errorf("record release: %w", err)
	}
	return tx.Commit()
}

// SetPort updates a deployment's recorded published port (owner, repo,
// environment), without appending a release event — it records the port an
// operator declared (e.g. when attaching a domain) so the deployment and its
// domain agree on the port. It reports whether a row was updated; false means no
// such deployment (so the caller can 404 rather than report a phantom success).
func (s *DeploymentSQL) SetPort(owner, repo, environment string, port int) (bool, error) {
	res, err := s.db.Exec(`UPDATE deployments SET port = ? WHERE owner = ? AND repo = ? AND environment = ?`,
		port, owner, repo, environment)
	if err != nil {
		return false, fmt.Errorf("set deployment port: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return affected > 0, nil
}

// Delete removes a tracked deployment and its entire append-only event history by
// its identity (owner, repo, environment), in one transaction so state and
// history never drift. It reports whether a deployment row was actually removed
// (false means no such deployment, so the caller can answer 404).
func (s *DeploymentSQL) Delete(owner, repo, environment string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Drop the history first, then the state, so a row never loses its state while
	// its events linger (the reverse would briefly orphan events on partial fail,
	// though the transaction makes both atomic regardless).
	if _, err := tx.Exec(`DELETE FROM deploy_events WHERE owner = ? AND repo = ? AND environment = ?`,
		owner, repo, environment); err != nil {
		return false, fmt.Errorf("delete deploy events: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM deployments WHERE owner = ? AND repo = ? AND environment = ?`,
		owner, repo, environment)
	if err != nil {
		return false, fmt.Errorf("delete deployment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return affected > 0, nil
}

// Releases returns a target's most recent releases (newest first, capped to
// limit) from the append-only log, plus the total number ever recorded. Each
// release carries the durable fields captured at deploy time (commit sha, actor,
// environment, status, previous-release link); image_ref/health/run_url are
// provisioned but populated by later phases.
func (s *DeploymentSQL) Releases(owner, repo, environment string, limit int) ([]usecase.Release, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM deploy_events WHERE owner = ? AND repo = ? AND environment = ?`,
		owner, repo, environment).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count releases: %w", err)
	}

	rows, err := s.db.Query(`SELECT id, at, environment, commit_sha, image_ref, status, health, actor, run_url, prev_event_id
		FROM deploy_events
		WHERE owner = ? AND repo = ? AND environment = ?
		ORDER BY at DESC, id DESC LIMIT ?`, owner, repo, environment, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("query releases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []usecase.Release
	for rows.Next() {
		var r usecase.Release
		var at string
		var prevID sql.NullInt64
		if err := rows.Scan(&r.ID, &at, &r.Environment, &r.CommitSHA, &r.ImageRef, &r.Status, &r.Health, &r.Actor, &r.RunURL, &prevID); err != nil {
			return nil, 0, fmt.Errorf("scan release: %w", err)
		}
		r.At, _ = time.Parse(time.RFC3339, at)
		if prevID.Valid {
			r.PrevReleaseID = &prevID.Int64
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

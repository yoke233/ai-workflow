package storesqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCutover_PurgeLegacyTaskPlanRows_WhenMissingStructuredContract(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-cutover-1', 'proj', '/tmp/proj-cutover-1')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status) VALUES ('plan-legacy-open', 'proj-cutover-1', 'legacy', 'reviewing')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_items (id, plan_id, title, description, status) VALUES ('task-legacy-open', 'plan-legacy-open', 'legacy task', 'legacy description', 'pending')`); err != nil {
		t.Fatalf("insert task_item: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var plans int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_plans WHERE id='plan-legacy-open'`).Scan(&plans); err != nil {
		t.Fatalf("count task_plans: %v", err)
	}
	if plans != 0 {
		t.Fatalf("legacy non-done task plan should be purged, got count=%d", plans)
	}

	var items int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_items WHERE id='task-legacy-open'`).Scan(&items); err != nil {
		t.Fatalf("count task_items: %v", err)
	}
	if items != 0 {
		t.Fatalf("legacy non-done task item should be purged, got count=%d", items)
	}
}

func TestCutover_PreserveDonePipelines_AndResetDanglingTaskRelations(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-cutover-2', 'proj', '/tmp/proj-cutover-2')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO pipelines (id, project_id, name, template, status, artifacts_json)
VALUES
	('pipe-done', 'proj-cutover-2', 'done', 'standard', 'done', '{"legacy_draft":"legacy"}'),
	('pipe-running', 'proj-cutover-2', 'running', 'standard', 'running', '{"legacy_review":"legacy"}')
`); err != nil {
		t.Fatalf("insert pipelines: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status) VALUES ('plan-legacy-done', 'proj-cutover-2', 'legacy done', 'done')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO task_items (id, plan_id, title, description, pipeline_id, status)
VALUES
	('task-keep', 'plan-legacy-done', 'keep', 'done pipeline relation', 'pipe-done', 'done'),
	('task-dangling-missing', 'plan-legacy-done', 'dangling-missing', 'missing pipeline relation', 'pipe-missing', 'done'),
	('task-dangling-running', 'plan-legacy-done', 'dangling-running', 'running pipeline relation', 'pipe-running', 'done')
`); err != nil {
		t.Fatalf("insert task_items: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var donePipes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pipelines WHERE id='pipe-done'`).Scan(&donePipes); err != nil {
		t.Fatalf("count done pipeline: %v", err)
	}
	if donePipes != 1 {
		t.Fatalf("done pipeline should be preserved, got count=%d", donePipes)
	}

	var runningPipes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pipelines WHERE id='pipe-running'`).Scan(&runningPipes); err != nil {
		t.Fatalf("count running pipeline: %v", err)
	}
	if runningPipes != 0 {
		t.Fatalf("non-done legacy pipeline should be purged, got count=%d", runningPipes)
	}

	assertPipelineID(t, db, "task-keep", "pipe-done")
	assertPipelineID(t, db, "task-dangling-missing", "")
	assertPipelineID(t, db, "task-dangling-running", "")
}

func assertPipelineID(t *testing.T, db *sql.DB, taskID, want string) {
	t.Helper()

	var pipelineID sql.NullString
	if err := db.QueryRow(`SELECT pipeline_id FROM task_items WHERE id=?`, taskID).Scan(&pipelineID); err != nil {
		t.Fatalf("query task %s: %v", taskID, err)
	}

	got := ""
	if pipelineID.Valid {
		got = pipelineID.String
	}
	if got != want {
		t.Fatalf("task %s pipeline_id=%q, want %q", taskID, got, want)
	}
}

func openLegacySQLite(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "legacy-cutover.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func seedLegacySchema(db *sql.DB) error {
	const legacySchema = `
CREATE TABLE projects (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	repo_path    TEXT NOT NULL UNIQUE,
	github_owner TEXT,
	github_repo  TEXT,
	config_json  TEXT,
	created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE pipelines (
	id             TEXT PRIMARY KEY,
	project_id     TEXT NOT NULL,
	name           TEXT NOT NULL,
	template       TEXT NOT NULL,
	status         TEXT NOT NULL DEFAULT 'created',
	artifacts_json TEXT DEFAULT '{}',
	run_count      INTEGER DEFAULT 0,
	last_error_type TEXT,
	queued_at      DATETIME,
	last_heartbeat_at DATETIME,
	created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE task_plans (
	id           TEXT PRIMARY KEY,
	project_id   TEXT NOT NULL,
	session_id   TEXT,
	name         TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'draft',
	wait_reason  TEXT NOT NULL DEFAULT '',
	fail_policy  TEXT NOT NULL DEFAULT 'block',
	review_round INTEGER DEFAULT 0,
	created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE task_items (
	id          TEXT PRIMARY KEY,
	plan_id     TEXT NOT NULL,
	title       TEXT NOT NULL,
	description TEXT NOT NULL,
	labels      TEXT DEFAULT '[]',
	depends_on  TEXT DEFAULT '[]',
	template    TEXT NOT NULL DEFAULT 'standard',
	pipeline_id TEXT,
	external_id TEXT,
	status      TEXT NOT NULL DEFAULT 'pending',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := db.Exec(legacySchema)
	return err
}

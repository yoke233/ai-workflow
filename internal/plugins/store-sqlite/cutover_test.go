package storesqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCutover_MigratesLegacyRows_IntoIssueSchema(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-cutover-1', 'proj', '/tmp/proj-cutover-1')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO pipelines (id, project_id, name, template, status, task_item_id)
VALUES ('pipe-cutover-1', 'proj-cutover-1', 'pipe', 'standard', 'done', 'task-legacy-1')
`); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status, fail_policy) VALUES ('plan-legacy-1', 'proj-cutover-1', 'legacy', 'reviewing', 'human')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO task_items (id, plan_id, title, description, pipeline_id, status, created_at)
VALUES
	('task-legacy-1', 'plan-legacy-1', 'legacy task 1', 'legacy desc 1', 'pipe-cutover-1', 'done', '2026-03-01T00:00:00Z'),
	('task-legacy-2', 'plan-legacy-1', 'legacy task 2', 'legacy desc 2', '', 'pending', '2026-03-01T01:00:00Z')
`); err != nil {
		t.Fatalf("insert task_items: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO review_records (plan_id, round, reviewer, verdict, issues, fixes, score)
VALUES ('plan-legacy-1', 1, 'ai-panel', 'issues_found', '[]', '[]', 78)
`); err != nil {
		t.Fatalf("insert review_record: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	assertTableExists(t, db, "issues")
	assertTableExists(t, db, "issue_attachments")
	assertTableExists(t, db, "issue_changes")
	assertTableNotExists(t, db, "task_plans")
	assertTableNotExists(t, db, "task_items")

	assertPipelineIssueID(t, db, "pipe-cutover-1", "task-legacy-1")

	var (
		projectID  string
		title      string
		body       string
		failPolicy string
		pipelineID sql.NullString
	)
	if err := db.QueryRow(`
SELECT project_id, title, body, fail_policy, pipeline_id
FROM issues
WHERE id='task-legacy-1'
`).Scan(&projectID, &title, &body, &failPolicy, &pipelineID); err != nil {
		t.Fatalf("query issue row: %v", err)
	}
	if projectID != "proj-cutover-1" || title != "legacy task 1" || body != "legacy desc 1" || failPolicy != "human" {
		t.Fatalf("unexpected issue payload: project_id=%q title=%q body=%q fail_policy=%q", projectID, title, body, failPolicy)
	}
	if !pipelineID.Valid || pipelineID.String != "pipe-cutover-1" {
		t.Fatalf("expected migrated issue to keep pipeline_id=pipe-cutover-1, got valid=%v value=%q", pipelineID.Valid, pipelineID.String)
	}

	var reviewIssueID string
	if err := db.QueryRow(`SELECT issue_id FROM review_records WHERE reviewer='ai-panel'`).Scan(&reviewIssueID); err != nil {
		t.Fatalf("query migrated review_records.issue_id: %v", err)
	}
	if reviewIssueID != "plan-legacy-1" {
		t.Fatalf("expected review_records.plan_id to migrate into issue_id, got %q", reviewIssueID)
	}
	var reviewSummary string
	var reviewRawOutput string
	if err := db.QueryRow(`SELECT COALESCE(summary, ''), COALESCE(raw_output, '') FROM review_records WHERE reviewer='ai-panel'`).Scan(&reviewSummary, &reviewRawOutput); err != nil {
		t.Fatalf("query review_records.summary/raw_output after cutover: %v", err)
	}
	if reviewSummary != "" || reviewRawOutput != "" {
		t.Fatalf("expected migrated review summary/raw_output empty defaults, got summary=%q raw_output=%q", reviewSummary, reviewRawOutput)
	}
}

func TestCutover_Wave3Flag_Idempotent(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-cutover-2', 'proj', '/tmp/proj-cutover-2')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO pipelines (id, project_id, name, template, status, task_item_id)
VALUES
	('pipe-cutover-2', 'proj-cutover-2', 'pipe', 'standard', 'done', 'task-cutover-2')
`); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status) VALUES ('plan-cutover-2', 'proj-cutover-2', 'legacy done', 'done')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO task_items (id, plan_id, title, description, pipeline_id, status)
VALUES ('task-cutover-2', 'plan-cutover-2', 'keep', 'done pipeline relation', 'pipe-cutover-2', 'done')
`); err != nil {
		t.Fatalf("insert task_item: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations second run: %v", err)
	}

	var flag string
	if err := db.QueryRow(`SELECT flag_value FROM migration_flags WHERE flag_key='wave3_issue_cutover_done'`).Scan(&flag); err != nil {
		t.Fatalf("query wave3 cutover flag: %v", err)
	}
	if flag != "1" {
		t.Fatalf("expected wave3 cutover flag=1, got %q", flag)
	}

	var issues int
	if err := db.QueryRow(`SELECT COUNT(*) FROM issues WHERE id='task-cutover-2'`).Scan(&issues); err != nil {
		t.Fatalf("count migrated issue rows: %v", err)
	}
	if issues != 1 {
		t.Fatalf("expected idempotent cutover to keep single issue row, got count=%d", issues)
	}

	assertPipelineIssueID(t, db, "pipe-cutover-2", "task-cutover-2")
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
	task_item_id   TEXT,
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

CREATE TABLE review_records (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	plan_id    TEXT NOT NULL,
	round      INTEGER NOT NULL,
	reviewer   TEXT NOT NULL,
	verdict    TEXT NOT NULL,
	issues     TEXT DEFAULT '[]',
	fixes      TEXT DEFAULT '[]',
	score      INTEGER,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_review_records_plan ON review_records(plan_id);
`
	_, err := db.Exec(legacySchema)
	return err
}

func assertPipelineIssueID(t *testing.T, db *sql.DB, pipelineID, want string) {
	t.Helper()

	var issueID sql.NullString
	if err := db.QueryRow(`SELECT issue_id FROM pipelines WHERE id=?`, pipelineID).Scan(&issueID); err != nil {
		t.Fatalf("query pipelines.issue_id for %s: %v", pipelineID, err)
	}

	got := ""
	if issueID.Valid {
		got = issueID.String
	}
	if got != want {
		t.Fatalf("pipeline %s issue_id=%q, want %q", pipelineID, got, want)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()

	ok, err := hasTable(db, table)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	if !ok {
		t.Fatalf("expected table %s to exist", table)
	}
}

func assertTableNotExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()

	ok, err := hasTable(db, table)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	if ok {
		t.Fatalf("expected table %s to be dropped", table)
	}
}

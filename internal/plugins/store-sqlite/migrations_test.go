package storesqlite

import (
	"database/sql"
	"testing"
)

func TestMigration_Wave3IssueCutoverSchema_BackwardCompatible(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-mig-1', 'proj', '/tmp/proj-mig-1')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pipelines (id, project_id, name, template, status, task_item_id) VALUES ('pipe-mig-1', 'proj-mig-1', 'pipe', 'standard', 'done', 'task-mig-1')`); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status) VALUES ('plan-mig-1', 'proj-mig-1', 'done plan', 'done')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_items (id, plan_id, title, description, pipeline_id, status) VALUES ('task-mig-1', 'plan-mig-1', 'task', 'desc', 'pipe-mig-1', 'done')`); err != nil {
		t.Fatalf("insert task_item: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO review_records (plan_id, round, reviewer, verdict, issues, fixes, score)
VALUES ('plan-mig-1', 2, 'ai-panel', 'approved', '[]', '[]', 90)
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

	assertColumnExists(t, db, "pipelines", "issue_id")
	assertColumnNotExists(t, db, "pipelines", "task_item_id")
	assertColumnExists(t, db, "review_records", "issue_id")
	assertColumnNotExists(t, db, "review_records", "plan_id")
	assertColumnExists(t, db, "review_records", "summary")
	assertColumnExists(t, db, "review_records", "raw_output")

	var issueRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM issues WHERE id='task-mig-1'`).Scan(&issueRows); err != nil {
		t.Fatalf("count migrated issues: %v", err)
	}
	if issueRows != 1 {
		t.Fatalf("expected migrated issue row count=1, got %d", issueRows)
	}

	var reviewIssueID string
	if err := db.QueryRow(`SELECT issue_id FROM review_records WHERE reviewer='ai-panel'`).Scan(&reviewIssueID); err != nil {
		t.Fatalf("query migrated review_records.issue_id: %v", err)
	}
	if reviewIssueID != "plan-mig-1" {
		t.Fatalf("expected review_records.issue_id=plan-mig-1, got %q", reviewIssueID)
	}
	var reviewSummary string
	var reviewRawOutput string
	if err := db.QueryRow(`SELECT COALESCE(summary, ''), COALESCE(raw_output, '') FROM review_records WHERE reviewer='ai-panel'`).Scan(&reviewSummary, &reviewRawOutput); err != nil {
		t.Fatalf("query migrated review_records.summary/raw_output: %v", err)
	}
	if reviewSummary != "" || reviewRawOutput != "" {
		t.Fatalf("expected default empty summary/raw_output after migration, got summary=%q raw_output=%q", reviewSummary, reviewRawOutput)
	}

	var wave3Flag string
	if err := db.QueryRow(`SELECT flag_value FROM migration_flags WHERE flag_key='wave3_issue_cutover_done'`).Scan(&wave3Flag); err != nil {
		t.Fatalf("query wave3 cutover flag: %v", err)
	}
	if wave3Flag != "1" {
		t.Fatalf("expected wave3 cutover flag=1, got %q", wave3Flag)
	}
}

func TestMigration_BackfillPipelineIssueID_FromLegacyTaskItems(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	if err := seedLegacySchema(db); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-mig-2', 'proj', '/tmp/proj-mig-2')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_plans (id, project_id, name, status) VALUES ('plan-mig-2', 'proj-mig-2', 'done plan', 'done')`); err != nil {
		t.Fatalf("insert task_plan: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pipelines (id, project_id, name, template, status, task_item_id) VALUES ('pipe-mig-2', 'proj-mig-2', 'pipe', 'standard', 'done', '')`); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO task_items (id, plan_id, title, description, pipeline_id, status, created_at)
VALUES
	('task-early', 'plan-mig-2', 'early', 'early', 'pipe-mig-2', 'done', '2026-03-01T00:00:00Z'),
	('task-late', 'plan-mig-2', 'late', 'late', 'pipe-mig-2', 'done', '2026-03-01T01:00:00Z')
`); err != nil {
		t.Fatalf("insert task_items: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var issueID string
	if err := db.QueryRow(`SELECT COALESCE(issue_id, '') FROM pipelines WHERE id='pipe-mig-2'`).Scan(&issueID); err != nil {
		t.Fatalf("query pipelines.issue_id: %v", err)
	}
	if issueID != "task-early" {
		t.Fatalf("expected deterministic backfill issue_id=task-early, got %q", issueID)
	}
}

func TestMigration_AddsIssueIDAndQueuedAtBeforeCreatingIndexes(t *testing.T) {
	db := openLegacySQLite(t)
	defer db.Close()

	// Simulate an older local DB: pipelines exists but lacks queued_at/issue_id.
	// applyMigrations must still finish and rebuild to wave3-compatible schema.
	if _, err := db.Exec(`
CREATE TABLE pipelines (
	id                TEXT PRIMARY KEY,
	project_id        TEXT NOT NULL,
	name              TEXT NOT NULL,
	template          TEXT NOT NULL,
	status            TEXT NOT NULL DEFAULT 'created',
	current_stage     TEXT,
	stages_json       TEXT NOT NULL DEFAULT '[]',
	artifacts_json    TEXT DEFAULT '{}',
	config_json       TEXT DEFAULT '{}',
	branch_name       TEXT,
	worktree_path     TEXT,
	error_message     TEXT,
	max_total_retries INTEGER DEFAULT 5,
	total_retries     INTEGER DEFAULT 0,
	run_count         INTEGER DEFAULT 0,
	last_error_type   TEXT,
	task_item_id      TEXT,
	last_heartbeat_at DATETIME,
	started_at        DATETIME,
	finished_at       DATETIME,
	created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		t.Fatalf("seed legacy pipelines table: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	assertColumnExists(t, db, "pipelines", "queued_at")
	assertColumnExists(t, db, "pipelines", "issue_id")
	assertColumnNotExists(t, db, "pipelines", "task_item_id")
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	ok, err := hasColumn(db, table, column)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if !ok {
		t.Fatalf("expected column %s.%s to exist", table, column)
	}
}

func assertColumnNotExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	ok, err := hasColumn(db, table, column)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if ok {
		t.Fatalf("expected column %s.%s to be absent", table, column)
	}
}

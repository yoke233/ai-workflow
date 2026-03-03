package storesqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

const schemaTables = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    repo_path    TEXT NOT NULL UNIQUE,
    github_owner TEXT,
    github_repo  TEXT,
    config_json  TEXT,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pipelines (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    description       TEXT,
    template          TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'created',
    current_stage     TEXT,
    stages_json       TEXT NOT NULL,
    artifacts_json    TEXT DEFAULT '{}',
    config_json       TEXT DEFAULT '{}',
    issue_number      INTEGER,
    pr_number         INTEGER,
    branch_name       TEXT,
    worktree_path     TEXT,
    error_message     TEXT,
    max_total_retries INTEGER DEFAULT 5,
    total_retries     INTEGER DEFAULT 0,
    run_count         INTEGER DEFAULT 0,
    last_error_type   TEXT,
    issue_id          TEXT,
    queued_at         DATETIME,
    last_heartbeat_at DATETIME,
    started_at        DATETIME,
    finished_at       DATETIME,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS checkpoints (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id    TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage          TEXT NOT NULL,
    status         TEXT NOT NULL,
    agent_used     TEXT,
    artifacts_json TEXT DEFAULT '{}',
    tokens_used    INTEGER DEFAULT 0,
    retry_count    INTEGER DEFAULT 0,
    error_message  TEXT,
    started_at     DATETIME NOT NULL,
    finished_at    DATETIME,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_pipeline ON checkpoints(pipeline_id);

CREATE TABLE IF NOT EXISTS logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,
    type        TEXT NOT NULL,
    agent       TEXT,
    content     TEXT NOT NULL,
    timestamp   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_logs_pipeline_stage ON logs(pipeline_id, stage);
CREATE INDEX IF NOT EXISTS idx_logs_id ON logs(id);

CREATE TABLE IF NOT EXISTS human_actions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,
    action      TEXT NOT NULL,
    message     TEXT,
    source      TEXT NOT NULL,
    user_id     TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_human_actions_pipeline ON human_actions(pipeline_id);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    agent_session_id TEXT NOT NULL DEFAULT '',
    messages    TEXT NOT NULL DEFAULT '[]',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_project ON chat_sessions(project_id);

CREATE TABLE IF NOT EXISTS chat_run_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    update_type     TEXT NOT NULL DEFAULT '',
    payload_json    TEXT NOT NULL DEFAULT '{}',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_run_events_session_created
ON chat_run_events(chat_session_id, created_at, id);

CREATE TABLE IF NOT EXISTS issues (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    session_id        TEXT REFERENCES chat_sessions(id) ON DELETE SET NULL,
    title             TEXT NOT NULL,
    body              TEXT NOT NULL DEFAULT '',
    labels            TEXT NOT NULL DEFAULT '[]',
    milestone_id      TEXT NOT NULL DEFAULT '',
    attachments       TEXT NOT NULL DEFAULT '[]',
    depends_on        TEXT NOT NULL DEFAULT '[]',
    blocks            TEXT NOT NULL DEFAULT '[]',
    priority          INTEGER NOT NULL DEFAULT 0,
    template          TEXT NOT NULL DEFAULT 'standard',
    auto_merge        INTEGER NOT NULL DEFAULT 1,
    state             TEXT NOT NULL DEFAULT 'open',
    status            TEXT NOT NULL DEFAULT 'draft',
    pipeline_id       TEXT,
    version           INTEGER NOT NULL DEFAULT 1,
    superseded_by     TEXT NOT NULL DEFAULT '',
    external_id       TEXT,
    fail_policy       TEXT NOT NULL DEFAULT 'block',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    closed_at         DATETIME
);

CREATE TABLE IF NOT EXISTS issue_attachments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS issue_changes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    field      TEXT NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    reason     TEXT NOT NULL DEFAULT '',
    changed_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS review_records (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   TEXT NOT NULL,
    round      INTEGER NOT NULL,
    reviewer   TEXT NOT NULL,
    verdict    TEXT NOT NULL,
    summary    TEXT NOT NULL DEFAULT '',
    raw_output TEXT NOT NULL DEFAULT '',
    issues     TEXT DEFAULT '[]',
    fixes      TEXT DEFAULT '[]',
    score      INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

const schemaIndexes = `
CREATE INDEX IF NOT EXISTS idx_pipelines_project ON pipelines(project_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_status ON pipelines(status);
CREATE INDEX IF NOT EXISTS idx_pipelines_status_queued_at ON pipelines(status, queued_at, created_at);
CREATE INDEX IF NOT EXISTS idx_pipelines_project_status ON pipelines(project_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_issues_project_status ON issues(project_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_session ON issues(session_id);
CREATE INDEX IF NOT EXISTS idx_issues_pipeline ON issues(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_issue_attachments_issue ON issue_attachments(issue_id);
CREATE INDEX IF NOT EXISTS idx_issue_changes_issue ON issue_changes(issue_id);
CREATE INDEX IF NOT EXISTS idx_review_records_issue ON review_records(issue_id);
`

func applyMigrations(db *sql.DB) error {
	if _, err := db.Exec(schemaTables); err != nil {
		return fmt.Errorf("exec schema tables: %w", err)
	}

	// Keep older local sqlite files backward-compatible when new columns are introduced.
	if err := ensureColumns(db, "pipelines", map[string]string{
		"run_count":         "run_count INTEGER DEFAULT 0",
		"last_error_type":   "last_error_type TEXT",
		"queued_at":         "queued_at DATETIME",
		"last_heartbeat_at": "last_heartbeat_at DATETIME",
		"issue_id":          "issue_id TEXT",
	}); err != nil {
		return err
	}
	if err := ensureColumns(db, "chat_sessions", map[string]string{
		"agent_session_id": "agent_session_id TEXT NOT NULL DEFAULT ''",
	}); err != nil {
		return err
	}
	if err := ensureColumns(db, "issues", map[string]string{
		"auto_merge": "auto_merge INTEGER NOT NULL DEFAULT 1",
	}); err != nil {
		return err
	}
	if err := ensureColumns(db, "review_records", map[string]string{
		"summary":    "summary TEXT NOT NULL DEFAULT ''",
		"raw_output": "raw_output TEXT NOT NULL DEFAULT ''",
	}); err != nil {
		return err
	}
	if err := applyWave3IssueCutover(db); err != nil {
		return err
	}

	// Create indexes after backward-compatible column adds.
	// Otherwise, older DB files missing new columns (e.g. pipelines.queued_at) will fail when creating indexes.
	if _, err := db.Exec(schemaIndexes); err != nil {
		return fmt.Errorf("exec schema indexes: %w", err)
	}
	return nil
}

func applyWave3IssueCutover(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS migration_flags (
	flag_key   TEXT PRIMARY KEY,
	flag_value TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("ensure migration_flags: %w", err)
	}

	var done int
	if err := db.QueryRow(`SELECT COUNT(*) FROM migration_flags WHERE flag_key='wave3_issue_cutover_done' AND flag_value='1'`).Scan(&done); err != nil {
		return fmt.Errorf("query wave3 cutover flag: %w", err)
	}
	if done > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin wave3 issue cutover: %w", err)
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		return fmt.Errorf("enable deferred foreign keys: %w", err)
	}
	if err := ensureWave3IssueTables(tx); err != nil {
		return err
	}
	if err := migrateLegacyTaskRowsToIssues(tx); err != nil {
		return err
	}
	if err := rebuildPipelinesForWave3(tx); err != nil {
		return err
	}
	if err := rebuildReviewRecordsForWave3(tx); err != nil {
		return err
	}
	if err := dropLegacyTaskTables(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO migration_flags(flag_key, flag_value)
VALUES ('wave3_issue_cutover_done', '1')
ON CONFLICT(flag_key) DO UPDATE SET
	flag_value='1',
	updated_at=CURRENT_TIMESTAMP
`); err != nil {
		return fmt.Errorf("set wave3 cutover flag: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit wave3 issue cutover: %w", err)
	}
	rollback = false
	return nil
}

func ensureWave3IssueTables(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS issues (
	id            TEXT PRIMARY KEY,
	project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	session_id    TEXT REFERENCES chat_sessions(id) ON DELETE SET NULL,
	title         TEXT NOT NULL,
	body          TEXT NOT NULL DEFAULT '',
	labels        TEXT NOT NULL DEFAULT '[]',
	milestone_id  TEXT NOT NULL DEFAULT '',
	attachments   TEXT NOT NULL DEFAULT '[]',
	depends_on    TEXT NOT NULL DEFAULT '[]',
	blocks        TEXT NOT NULL DEFAULT '[]',
	priority      INTEGER NOT NULL DEFAULT 0,
	template      TEXT NOT NULL DEFAULT 'standard',
	auto_merge    INTEGER NOT NULL DEFAULT 1,
	state         TEXT NOT NULL DEFAULT 'open',
	status        TEXT NOT NULL DEFAULT 'draft',
	pipeline_id   TEXT,
	version       INTEGER NOT NULL DEFAULT 1,
	superseded_by TEXT NOT NULL DEFAULT '',
	external_id   TEXT,
	fail_policy   TEXT NOT NULL DEFAULT 'block',
	created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
	closed_at     DATETIME
);

CREATE TABLE IF NOT EXISTS issue_attachments (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id   TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
	path       TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS issue_changes (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id   TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
	field      TEXT NOT NULL,
	old_value  TEXT,
	new_value  TEXT,
	reason     TEXT NOT NULL DEFAULT '',
	changed_by TEXT NOT NULL DEFAULT '',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return fmt.Errorf("ensure wave3 issue tables: %w", err)
	}
	return nil
}

func migrateLegacyTaskRowsToIssues(tx *sql.Tx) error {
	taskPlansExists, err := hasTableTx(tx, "task_plans")
	if err != nil {
		return err
	}
	taskItemsExists, err := hasTableTx(tx, "task_items")
	if err != nil {
		return err
	}
	if !taskPlansExists || !taskItemsExists {
		return nil
	}

	taskPlanCols, err := tableColumnsTx(tx, "task_plans")
	if err != nil {
		return err
	}
	taskItemCols, err := tableColumnsTx(tx, "task_items")
	if err != nil {
		return err
	}
	tiExpr := func(column, fallback string) string {
		if taskItemCols[column] {
			return "ti." + column
		}
		return fallback
	}
	tpExpr := func(column, fallback string) string {
		if taskPlanCols[column] {
			return "tp." + column
		}
		return fallback
	}

	query := fmt.Sprintf(`
INSERT INTO issues (
	id, project_id, session_id, title, body, labels, milestone_id, attachments, depends_on, blocks,
	priority, template, auto_merge, state, status, pipeline_id, version, superseded_by, external_id, fail_policy,
	created_at, updated_at, closed_at
)
SELECT
	%s,
	%s,
	%s,
	%s,
	%s,
	COALESCE(%s, '[]'),
	'',
	'[]',
	COALESCE(%s, '[]'),
	'[]',
	0,
	COALESCE(NULLIF(TRIM(%s), ''), 'standard'),
	1,
	'open',
	COALESCE(NULLIF(TRIM(%s), ''), 'draft'),
	NULLIF(TRIM(COALESCE(%s, '')), ''),
	1,
	'',
	NULLIF(TRIM(COALESCE(%s, '')), ''),
	COALESCE(NULLIF(TRIM(%s), ''), 'block'),
	%s,
	%s,
	NULL
FROM task_items ti
JOIN task_plans tp ON tp.id = ti.plan_id
ON CONFLICT(id) DO NOTHING
`,
		tiExpr("id", "''"),
		tpExpr("project_id", "''"),
		tpExpr("session_id", "NULL"),
		tiExpr("title", "''"),
		tiExpr("description", "''"),
		tiExpr("labels", "'[]'"),
		tiExpr("depends_on", "'[]'"),
		tiExpr("template", "''"),
		tiExpr("status", "''"),
		tiExpr("pipeline_id", "''"),
		tiExpr("external_id", "''"),
		tpExpr("fail_policy", "'block'"),
		tiExpr("created_at", "CURRENT_TIMESTAMP"),
		tiExpr("updated_at", "CURRENT_TIMESTAMP"),
	)
	if _, err := tx.Exec(query); err != nil {
		return fmt.Errorf("migrate task_items into issues: %w", err)
	}
	return nil
}

func rebuildPipelinesForWave3(tx *sql.Tx) error {
	pipelinesExists, err := hasTableTx(tx, "pipelines")
	if err != nil {
		return err
	}
	if !pipelinesExists {
		return nil
	}

	cols, err := tableColumnsTx(tx, "pipelines")
	if err != nil {
		return err
	}
	if !cols["task_item_id"] {
		return nil
	}

	taskItemsExists, err := hasTableTx(tx, "task_items")
	if err != nil {
		return err
	}

	var issueParts []string
	if cols["issue_id"] {
		issueParts = append(issueParts, "NULLIF(TRIM(issue_id), '')")
	}
	if cols["task_item_id"] {
		issueParts = append(issueParts, "NULLIF(TRIM(task_item_id), '')")
	}
	if taskItemsExists {
		issueParts = append(issueParts, `(SELECT ti.id FROM task_items ti WHERE ti.pipeline_id = pipelines.id ORDER BY ti.created_at ASC, ti.id ASC LIMIT 1)`)
	}
	issueExpr := "NULL"
	if len(issueParts) == 1 {
		issueExpr = issueParts[0]
	} else if len(issueParts) > 1 {
		issueExpr = "COALESCE(" + strings.Join(issueParts, ", ") + ")"
	}

	pExpr := func(column, fallback string) string {
		if cols[column] {
			return column
		}
		return fallback
	}

	if _, err := tx.Exec(`
CREATE TABLE pipelines_wave3 (
	id                TEXT PRIMARY KEY,
	project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name              TEXT NOT NULL,
	description       TEXT,
	template          TEXT NOT NULL,
	status            TEXT NOT NULL DEFAULT 'created',
	current_stage     TEXT,
	stages_json       TEXT NOT NULL,
	artifacts_json    TEXT DEFAULT '{}',
	config_json       TEXT DEFAULT '{}',
	issue_number      INTEGER,
	pr_number         INTEGER,
	branch_name       TEXT,
	worktree_path     TEXT,
	error_message     TEXT,
	max_total_retries INTEGER DEFAULT 5,
	total_retries     INTEGER DEFAULT 0,
	run_count         INTEGER DEFAULT 0,
	last_error_type   TEXT,
	issue_id          TEXT,
	queued_at         DATETIME,
	last_heartbeat_at DATETIME,
	started_at        DATETIME,
	finished_at       DATETIME,
	created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
)
`); err != nil {
		return fmt.Errorf("create pipelines_wave3: %w", err)
	}

	insertQuery := fmt.Sprintf(`
INSERT INTO pipelines_wave3 (
	id, project_id, name, description, template, status, current_stage, stages_json, artifacts_json, config_json,
	issue_number, pr_number, branch_name, worktree_path, error_message, max_total_retries, total_retries, run_count,
	last_error_type, issue_id, queued_at, last_heartbeat_at, started_at, finished_at, created_at, updated_at
)
SELECT
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s
FROM pipelines
`,
		pExpr("id", "''"),
		pExpr("project_id", "''"),
		pExpr("name", "''"),
		pExpr("description", "NULL"),
		pExpr("template", "'standard'"),
		pExpr("status", "'created'"),
		pExpr("current_stage", "NULL"),
		pExpr("stages_json", "'[]'"),
		pExpr("artifacts_json", "'{}'"),
		pExpr("config_json", "'{}'"),
		pExpr("issue_number", "NULL"),
		pExpr("pr_number", "NULL"),
		pExpr("branch_name", "NULL"),
		pExpr("worktree_path", "NULL"),
		pExpr("error_message", "NULL"),
		pExpr("max_total_retries", "5"),
		pExpr("total_retries", "0"),
		pExpr("run_count", "0"),
		pExpr("last_error_type", "NULL"),
		issueExpr,
		pExpr("queued_at", "NULL"),
		pExpr("last_heartbeat_at", "NULL"),
		pExpr("started_at", "NULL"),
		pExpr("finished_at", "NULL"),
		pExpr("created_at", "CURRENT_TIMESTAMP"),
		pExpr("updated_at", "CURRENT_TIMESTAMP"),
	)
	if _, err := tx.Exec(insertQuery); err != nil {
		return fmt.Errorf("copy pipelines into pipelines_wave3: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE pipelines`); err != nil {
		return fmt.Errorf("drop legacy pipelines: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE pipelines_wave3 RENAME TO pipelines`); err != nil {
		return fmt.Errorf("rename pipelines_wave3: %w", err)
	}
	return nil
}

func rebuildReviewRecordsForWave3(tx *sql.Tx) error {
	reviewRecordsExists, err := hasTableTx(tx, "review_records")
	if err != nil {
		return err
	}
	if !reviewRecordsExists {
		return nil
	}

	cols, err := tableColumnsTx(tx, "review_records")
	if err != nil {
		return err
	}
	if cols["issue_id"] && !cols["plan_id"] {
		return nil
	}

	var issueParts []string
	if cols["issue_id"] {
		issueParts = append(issueParts, "NULLIF(TRIM(issue_id), '')")
	}
	if cols["plan_id"] {
		issueParts = append(issueParts, "NULLIF(TRIM(plan_id), '')")
	}
	if len(issueParts) == 0 {
		issueParts = append(issueParts, "''")
	}
	issueExpr := issueParts[0]
	if len(issueParts) > 1 {
		issueExpr = "COALESCE(" + strings.Join(issueParts, ", ") + ", '')"
	}

	rExpr := func(column, fallback string) string {
		if cols[column] {
			return column
		}
		return fallback
	}

	if _, err := tx.Exec(`
CREATE TABLE review_records_wave3 (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id   TEXT NOT NULL,
	round      INTEGER NOT NULL,
	reviewer   TEXT NOT NULL,
	verdict    TEXT NOT NULL,
	summary    TEXT NOT NULL DEFAULT '',
	raw_output TEXT NOT NULL DEFAULT '',
	issues     TEXT DEFAULT '[]',
	fixes      TEXT DEFAULT '[]',
	score      INTEGER,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
`); err != nil {
		return fmt.Errorf("create review_records_wave3: %w", err)
	}

	insertQuery := fmt.Sprintf(`
INSERT INTO review_records_wave3 (
	id, issue_id, round, reviewer, verdict, summary, raw_output, issues, fixes, score, created_at
)
SELECT
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s
FROM review_records
`,
		rExpr("id", "NULL"),
		issueExpr,
		rExpr("round", "0"),
		rExpr("reviewer", "''"),
		rExpr("verdict", "''"),
		rExpr("summary", "''"),
		rExpr("raw_output", "''"),
		rExpr("issues", "'[]'"),
		rExpr("fixes", "'[]'"),
		rExpr("score", "NULL"),
		rExpr("created_at", "CURRENT_TIMESTAMP"),
	)
	if _, err := tx.Exec(insertQuery); err != nil {
		return fmt.Errorf("copy review_records into review_records_wave3: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE review_records`); err != nil {
		return fmt.Errorf("drop legacy review_records: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE review_records_wave3 RENAME TO review_records`); err != nil {
		return fmt.Errorf("rename review_records_wave3: %w", err)
	}
	return nil
}

func dropLegacyTaskTables(tx *sql.Tx) error {
	taskItemsExists, err := hasTableTx(tx, "task_items")
	if err != nil {
		return err
	}
	if taskItemsExists {
		if _, err := tx.Exec(`DROP TABLE task_items`); err != nil {
			return fmt.Errorf("drop legacy task_items: %w", err)
		}
	}

	taskPlansExists, err := hasTableTx(tx, "task_plans")
	if err != nil {
		return err
	}
	if taskPlansExists {
		if _, err := tx.Exec(`DROP TABLE task_plans`); err != nil {
			return fmt.Errorf("drop legacy task_plans: %w", err)
		}
	}
	return nil
}

func ensureColumns(db *sql.DB, table string, columns map[string]string) error {
	for column, ddl := range columns {
		exists, err := hasColumn(db, table, column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + ddl); err != nil {
			return fmt.Errorf("add %s.%s: %w", table, column, err)
		}
	}
	return nil
}

func tableColumnsTx(tx *sql.Tx, table string) (map[string]bool, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := tx.Query(query)
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table_info(%s): %w", table, err)
	}
	return columns, nil
}

func hasTable(db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("check table %s: %w", table, err)
	}
	return count > 0, nil
}

func hasTableTx(tx *sql.Tx, table string) (bool, error) {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("check table %s: %w", table, err)
	}
	return count > 0, nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := db.Query(query)
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table_info(%s): %w", table, err)
	}
	return false, nil
}

# Decision 版本化实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为系统中每个 AI 决策建立独立的 `decisions` 表，记录 prompt/model/reasoning/output，实现决策可追溯。

**Architecture:** Decision 是独立实体，有专用表和强类型字段。与 TaskStep 通过 RefID 关联（TaskStep 记"发生了什么"，Decision 记"为什么这么决策"）。先覆盖 3 个关键决策点：审查/分解/Stage 执行。API 层提供按 Issue 查询 Decision 列表。

**Tech Stack:** Go 1.22+, SQLite (migration V11), chi router, React/TypeScript (前端展示)

---

### Task 1: Decision 核心模型

**Files:**
- Create: `internal/core/decision.go`

**Step 1: 创建 Decision 模型文件**

```go
package core

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Decision type constants.
const (
	DecisionTypeReview    = "review"
	DecisionTypeDecompose = "decompose"
	DecisionTypeStage     = "stage"
	DecisionTypeChat      = "chat"
)

// Decision records a single AI decision with full traceability.
type Decision struct {
	ID              string    `json:"id"`
	IssueID         string    `json:"issue_id"`
	RunID           string    `json:"run_id,omitempty"`
	StageID         StageID   `json:"stage_id,omitempty"`
	AgentID         string    `json:"agent_id"`
	Type            string    `json:"type"`
	PromptHash      string    `json:"prompt_hash"`
	PromptPreview   string    `json:"prompt_preview"`
	Model           string    `json:"model"`
	Template        string    `json:"template"`
	TemplateVersion string    `json:"template_version"`
	InputTokens     int       `json:"input_tokens"`
	Action          string    `json:"action"`
	Reasoning       string    `json:"reasoning"`
	Confidence      float64   `json:"confidence"`
	OutputTokens    int       `json:"output_tokens"`
	OutputData      string    `json:"output_data"`
	DurationMs      int64     `json:"duration_ms"`
	CreatedAt       time.Time `json:"created_at"`
}

// NewDecisionID generates an ID in format: dec-YYYYMMDD-HHMMSS-xxxxxxxx.
func NewDecisionID() string {
	return fmt.Sprintf("dec-%s-%s", time.Now().Format("20060102-150405"), randomHex(4))
}

// PromptHash returns the first 16 hex characters of the SHA-256 hash of prompt.
func PromptHash(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h[:8])
}

// TruncateString truncates s to at most maxLen characters.
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
```

**Step 2: 验证编译**

Run: `go build ./internal/core/...`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add internal/core/decision.go
git commit -m "feat(core): add Decision model with ID generation and prompt hashing"
```

---

### Task 2: Decision 模型单元测试

**Files:**
- Create: `internal/core/decision_test.go`

**Step 1: 写测试**

```go
package core

import (
	"strings"
	"testing"
)

func TestNewDecisionID(t *testing.T) {
	id := NewDecisionID()
	if !strings.HasPrefix(id, "dec-") {
		t.Errorf("expected prefix 'dec-', got %q", id)
	}
	// Format: dec-YYYYMMDD-HHMMSS-xxxxxxxx → 29 chars
	if len(id) != 29 {
		t.Errorf("expected length 29, got %d for %q", len(id), id)
	}
}

func TestPromptHash(t *testing.T) {
	hash := PromptHash("hello world")
	if len(hash) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %q", len(hash), hash)
	}
	// Same input should produce same hash.
	if PromptHash("hello world") != hash {
		t.Error("PromptHash should be deterministic")
	}
	// Different input should produce different hash.
	if PromptHash("goodbye world") == hash {
		t.Error("different inputs should produce different hashes")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"你好世界", 2, "你好"},
	}
	for _, tt := range tests {
		got := TruncateString(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
```

**Step 2: 运行测试**

Run: `go test ./internal/core/... -run TestNewDecisionID -v && go test ./internal/core/... -run TestPromptHash -v && go test ./internal/core/... -run TestTruncateString -v`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/core/decision_test.go
git commit -m "test(core): add unit tests for Decision ID, PromptHash, TruncateString"
```

---

### Task 3: Store 接口扩展

**Files:**
- Modify: `internal/core/store.go`

**Step 1: 在 Store 接口中添加 Decision 方法**

在 `SaveTaskStep` 相关方法之后、`ListEvents` 之前添加：

```go
	// Decision versioning.
	SaveDecision(d *Decision) error
	GetDecision(id string) (*Decision, error)
	ListDecisions(issueID string) ([]Decision, error)
```

**Step 2: 验证编译（此时会失败，因为 store-sqlite 未实现）**

Run: `go build ./internal/core/...`
Expected: BUILD SUCCESS（接口定义不影响编译）

**Step 3: Commit**

```bash
git add internal/core/store.go
git commit -m "feat(core): add Decision methods to Store interface"
```

---

### Task 4: SQLite Migration V11 — decisions 表

**Files:**
- Modify: `internal/plugins/store-sqlite/migrations.go`

**Step 1: 增加 migration 函数**

在 `migrateAddChatSessionAgentName` 函数之后添加：

```go
func migrateAddDecisions(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS decisions (
	id               TEXT PRIMARY KEY,
	issue_id         TEXT NOT NULL,
	run_id           TEXT NOT NULL DEFAULT '',
	stage_id         TEXT NOT NULL DEFAULT '',
	agent_id         TEXT NOT NULL DEFAULT '',
	type             TEXT NOT NULL,
	prompt_hash      TEXT NOT NULL,
	prompt_preview   TEXT NOT NULL DEFAULT '',
	model            TEXT NOT NULL DEFAULT '',
	template         TEXT NOT NULL DEFAULT '',
	template_version TEXT NOT NULL DEFAULT '',
	input_tokens     INTEGER NOT NULL DEFAULT 0,
	action           TEXT NOT NULL,
	reasoning        TEXT NOT NULL DEFAULT '',
	confidence       REAL NOT NULL DEFAULT 0,
	output_tokens    INTEGER NOT NULL DEFAULT 0,
	output_data      TEXT NOT NULL DEFAULT '{}',
	duration_ms      INTEGER NOT NULL DEFAULT 0,
	created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_decisions_issue ON decisions(issue_id, created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_type  ON decisions(type, created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_model ON decisions(model);
`)
	return err
}
```

**Step 2: 注册 migration，bump schemaVersion 到 11**

在 `applyMigrations` 函数中，`currentVersion < 10` 块之后添加：

```go
	if currentVersion < 11 {
		if err := migrateAddDecisions(db); err != nil {
			return fmt.Errorf("migration v11 (decisions): %w", err)
		}
	}
```

将 `const schemaVersion = 10` 改为 `const schemaVersion = 11`。

**Step 3: 验证编译**

Run: `go build ./internal/plugins/store-sqlite/...`
Expected: BUILD SUCCESS（此时还缺 Store 接口实现，先确保 migration 本身编译通过）

**Step 4: Commit**

```bash
git add internal/plugins/store-sqlite/migrations.go
git commit -m "feat(store): add migration V11 for decisions table"
```

---

### Task 5: SQLite Store 实现 — SaveDecision/GetDecision/ListDecisions

**Files:**
- Modify: `internal/plugins/store-sqlite/store.go`

**Step 1: 实现 SaveDecision**

在 `SaveReviewRecord` / `GetReviewRecords` 方法附近添加：

```go
func (s *SQLiteStore) SaveDecision(d *core.Decision) error {
	_, err := s.db.Exec(
		`INSERT INTO decisions (id, issue_id, run_id, stage_id, agent_id, type,
		 prompt_hash, prompt_preview, model, template, template_version, input_tokens,
		 action, reasoning, confidence, output_tokens, output_data, duration_ms, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.IssueID, d.RunID, string(d.StageID), d.AgentID, d.Type,
		d.PromptHash, d.PromptPreview, d.Model, d.Template, d.TemplateVersion, d.InputTokens,
		d.Action, d.Reasoning, d.Confidence, d.OutputTokens, d.OutputData, d.DurationMs, d.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetDecision(id string) (*core.Decision, error) {
	row := s.db.QueryRow(
		`SELECT id, issue_id, run_id, stage_id, agent_id, type,
		 prompt_hash, prompt_preview, model, template, template_version, input_tokens,
		 action, reasoning, confidence, output_tokens, output_data, duration_ms, created_at
		 FROM decisions WHERE id=?`, id,
	)
	var d core.Decision
	var stageID string
	err := row.Scan(
		&d.ID, &d.IssueID, &d.RunID, &stageID, &d.AgentID, &d.Type,
		&d.PromptHash, &d.PromptPreview, &d.Model, &d.Template, &d.TemplateVersion, &d.InputTokens,
		&d.Action, &d.Reasoning, &d.Confidence, &d.OutputTokens, &d.OutputData, &d.DurationMs, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.StageID = core.StageID(stageID)
	return &d, nil
}

func (s *SQLiteStore) ListDecisions(issueID string) ([]core.Decision, error) {
	rows, err := s.db.Query(
		`SELECT id, issue_id, run_id, stage_id, agent_id, type,
		 prompt_hash, prompt_preview, model, template, template_version, input_tokens,
		 action, reasoning, confidence, output_tokens, output_data, duration_ms, created_at
		 FROM decisions WHERE issue_id=? ORDER BY created_at`, issueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Decision
	for rows.Next() {
		var d core.Decision
		var stageID string
		if err := rows.Scan(
			&d.ID, &d.IssueID, &d.RunID, &stageID, &d.AgentID, &d.Type,
			&d.PromptHash, &d.PromptPreview, &d.Model, &d.Template, &d.TemplateVersion, &d.InputTokens,
			&d.Action, &d.Reasoning, &d.Confidence, &d.OutputTokens, &d.OutputData, &d.DurationMs, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		d.StageID = core.StageID(stageID)
		out = append(out, d)
	}
	return out, rows.Err()
}
```

**Step 2: 验证编译**

Run: `go build ./internal/plugins/store-sqlite/...`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add internal/plugins/store-sqlite/store.go
git commit -m "feat(store): implement SaveDecision/GetDecision/ListDecisions for SQLite"
```

---

### Task 6: Store Decision 集成测试

**Files:**
- Create: `internal/plugins/store-sqlite/decision_test.go`

**Step 1: 写集成测试**

```go
package storesqlite

import (
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestDecisionCRUD(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	project := &core.Project{ID: "proj-dec-test", Name: "test", RepoPath: "/tmp/dec-test"}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	issue := &core.Issue{
		ID:        "issue-dec-test",
		ProjectID: project.ID,
		Title:     "test decision",
		Status:    core.IssueStatusDraft,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	d := &core.Decision{
		ID:            core.NewDecisionID(),
		IssueID:       issue.ID,
		RunID:         "run-001",
		AgentID:       "reviewer-1",
		Type:          core.DecisionTypeReview,
		PromptHash:    core.PromptHash("test prompt"),
		PromptPreview: core.TruncateString("test prompt for review", 500),
		Model:         "claude-sonnet-4-20250514",
		Template:      "review",
		Action:        "approve",
		Reasoning:     "Code quality is good",
		Confidence:    0.85,
		OutputData:    `{"score":85}`,
		DurationMs:    3200,
		CreatedAt:     time.Now(),
	}

	// Save
	if err := store.SaveDecision(d); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	// Get
	got, err := store.GetDecision(d.ID)
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if got.IssueID != d.IssueID {
		t.Errorf("IssueID = %q, want %q", got.IssueID, d.IssueID)
	}
	if got.Type != core.DecisionTypeReview {
		t.Errorf("Type = %q, want %q", got.Type, core.DecisionTypeReview)
	}
	if got.Action != "approve" {
		t.Errorf("Action = %q, want %q", got.Action, "approve")
	}
	if got.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", got.Confidence)
	}
	if got.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", got.Model)
	}

	// Save a second decision
	d2 := &core.Decision{
		ID:         core.NewDecisionID(),
		IssueID:    issue.ID,
		AgentID:    "worker-1",
		Type:       core.DecisionTypeStage,
		PromptHash: core.PromptHash("implement prompt"),
		Action:     "completed",
		DurationMs: 15000,
		CreatedAt:  time.Now(),
	}
	if err := store.SaveDecision(d2); err != nil {
		t.Fatalf("SaveDecision(d2): %v", err)
	}

	// List
	list, err := store.ListDecisions(issue.ID)
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListDecisions: got %d, want 2", len(list))
	}
}
```

**Step 2: 运行测试**

Run: `go test ./internal/plugins/store-sqlite/... -run TestDecisionCRUD -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/plugins/store-sqlite/decision_test.go
git commit -m "test(store): add integration test for Decision CRUD operations"
```

---

### Task 7: REST API — Decision 列表与详情

**Files:**
- Create: `internal/web/handlers_decisions.go`
- Modify: `internal/web/handlers_v3.go` (注册路由)

**Step 1: 创建 handler 文件**

```go
package web

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

type decisionHandlers struct {
	store core.Store
}

func (h *decisionHandlers) listByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := strings.TrimSpace(chi.URLParam(r, "issueId"))
	if issueID == "" {
		issueID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if issueID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "issue id is required"})
		return
	}
	decisions, err := h.store.ListDecisions(issueID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if decisions == nil {
		decisions = []core.Decision{}
	}
	writeJSON(w, http.StatusOK, decisions)
}

func (h *decisionHandlers) getDecision(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "decision id is required"})
		return
	}
	d, err := h.store.GetDecision(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "decision not found"})
		return
	}
	writeJSON(w, http.StatusOK, d)
}
```

**Step 2: 在 registerV1Routes 中注册路由**

在 `internal/web/handlers_v3.go` 的 `registerV1Routes` 函数中，`r.With(RequireScope(ScopeIssuesRead)).Get("/projects/{projectId}/issues/{issueId}/timeline", ...)` 行之后添加：

```go
	decHandlers := &decisionHandlers{store: store}
	r.With(RequireScope(ScopeIssuesRead)).Get("/issues/{id}/decisions", decHandlers.listByIssue)
	r.With(RequireScope(ScopeIssuesRead)).Get("/projects/{projectId}/issues/{issueId}/decisions", decHandlers.listByIssue)
	r.With(RequireScope(ScopeIssuesRead)).Get("/decisions/{id}", decHandlers.getDecision)
```

**Step 3: 验证编译**

Run: `go build ./internal/web/...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add internal/web/handlers_decisions.go internal/web/handlers_v3.go
git commit -m "feat(api): add GET /issues/{id}/decisions and GET /decisions/{id} endpoints"
```

---

### Task 8: 前端 TypeScript 类型

**Files:**
- Modify: `web/src/types/workflow.ts` 或合适的类型文件

**Step 1: 添加 Decision 类型定义**

```typescript
export interface Decision {
  id: string;
  issue_id: string;
  run_id?: string;
  stage_id?: string;
  agent_id: string;
  type: "review" | "decompose" | "stage" | "chat";
  prompt_hash: string;
  prompt_preview: string;
  model: string;
  template: string;
  template_version: string;
  input_tokens: number;
  action: string;
  reasoning: string;
  confidence: number;
  output_tokens: number;
  output_data: string;
  duration_ms: number;
  created_at: string;
}
```

**Step 2: 添加 API 调用函数**

在 `web/src/lib/apiClient.ts` 中添加：

```typescript
export async function fetchIssueDecisions(issueId: string): Promise<Decision[]> {
  const resp = await apiFetch(`/issues/${issueId}/decisions`);
  return resp.json();
}

export async function fetchDecision(id: string): Promise<Decision> {
  const resp = await apiFetch(`/decisions/${id}`);
  return resp.json();
}
```

**Step 3: 验证类型检查**

Run: `npm --prefix web run typecheck`
Expected: NO ERRORS

**Step 4: Commit**

```bash
git add web/src/types/workflow.ts web/src/lib/apiClient.ts
git commit -m "feat(web): add Decision TypeScript type and API client functions"
```

---

### Task 9: 运行完整测试套件

**Step 1: 后端测试**

Run: `pwsh -NoProfile -File ./scripts/test/backend-all.ps1`
Expected: ALL PASS

**Step 2: 前端测试**

Run: `npm --prefix web run typecheck && npm --prefix web run test`
Expected: ALL PASS

**Step 3: 如有失败，修复并 commit**

```bash
git commit -m "fix(decision): address test failures from integration"
```

---

### Task 10: 审查决策点集成（review.go）— 可选后续

> 此任务在基础设施完成后执行。改造 `internal/teamleader/review.go` 的 `decideSession()` 函数，在审查完成时写入 Decision 记录。

**Files:**
- Modify: `internal/teamleader/review.go`

**改造要点:**

在 `decideSession()` 返回之前：

```go
decision := &core.Decision{
    ID:            core.NewDecisionID(),
    IssueID:       issue.ID,
    AgentID:       reviewerName,
    Type:          core.DecisionTypeReview,
    PromptHash:    core.PromptHash(reviewPrompt),
    PromptPreview: core.TruncateString(reviewPrompt, 500),
    Model:         modelID,
    Template:      "review",
    Action:        string(verdict),  // approve / fix / escalate
    Reasoning:     summary,
    Confidence:    float64(score) / 100.0,
    OutputData:    fmt.Sprintf(`{"score":%d}`, score),
    DurationMs:    elapsed.Milliseconds(),
    CreatedAt:     time.Now(),
}
_ = store.SaveDecision(decision)
```

同时在对应的 `recordTaskStep` 调用中，设置 `RefID = decision.ID`, `RefType = "decision"`。

**此任务需要仔细阅读 review.go 当前实现后再改造，不在本计划的"必做"范围内。**

# ChatView Agent Select + Style Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add agent runtime selection (claude/codex) when starting a chat session, lock it per-session, expose via API, and comprehensively optimize ChatView styling.

**Architecture:** Backend adds `agent_name` to ChatSession model + new `GET /api/v1/agents` endpoint. RoleResolver gets a `ListAgents()` method. ChatAssistant accepts an agent override that bypasses the role's default agent binding. Frontend adds a dropdown selector and full TUI style polish.

**Tech Stack:** Go 1.22, SQLite, chi router, React 18, TypeScript, Tailwind CSS

---

### Task 1: RoleResolver — add ListAgents method

**Files:**
- Modify: `internal/acpclient/role_resolver.go:51-103`
- Test: `internal/acpclient/role_resolver_test.go`

**Step 1: Write the test**

Add to `internal/acpclient/role_resolver_test.go`:

```go
func TestRoleResolver_ListAgents(t *testing.T) {
	agents := []AgentProfile{
		{ID: "claude", LaunchCommand: "npx", LaunchArgs: []string{"-y", "@zed-industries/claude-agent-acp"}},
		{ID: "codex", LaunchCommand: "npx", LaunchArgs: []string{"-y", "@zed-industries/codex-acp"}},
	}
	roles := []RoleProfile{{ID: "team_leader", AgentID: "claude"}}
	resolver := NewRoleResolver(agents, roles)

	result := resolver.ListAgents()
	if len(result) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result))
	}
	names := map[string]bool{}
	for _, a := range result {
		names[a.ID] = true
	}
	if !names["claude"] || !names["codex"] {
		t.Fatalf("expected claude and codex, got %v", names)
	}
}

func TestRoleResolver_GetAgent(t *testing.T) {
	agents := []AgentProfile{
		{ID: "claude", LaunchCommand: "npx"},
	}
	resolver := NewRoleResolver(agents, nil)

	a, err := resolver.GetAgent("claude")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "claude" {
		t.Fatalf("expected claude, got %s", a.ID)
	}

	_, err = resolver.GetAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd D:/project/ai-workflow && go test ./internal/acpclient/... -run TestRoleResolver_ListAgents -v`
Expected: FAIL — method not found

**Step 3: Implement**

Add to `internal/acpclient/role_resolver.go` after `Resolve()`:

```go
// ListAgents returns all registered agent profiles.
func (r *RoleResolver) ListAgents() []AgentProfile {
	if r == nil {
		return nil
	}
	result := make([]AgentProfile, 0, len(r.agents))
	for _, a := range r.agents {
		result = append(result, cloneAgentProfile(a))
	}
	return result
}

// GetAgent returns a single agent profile by ID.
func (r *RoleResolver) GetAgent(agentID string) (AgentProfile, error) {
	if r == nil {
		return AgentProfile{}, ErrRoleResolverNil
	}
	agent, ok := r.agents[agentID]
	if !ok {
		return AgentProfile{}, fmt.Errorf("%w: agent %q", ErrAgentNotFound, agentID)
	}
	return cloneAgentProfile(agent), nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd D:/project/ai-workflow && go test ./internal/acpclient/... -run "TestRoleResolver_ListAgents|TestRoleResolver_GetAgent" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/acpclient/role_resolver.go internal/acpclient/role_resolver_test.go
git commit -m "feat(acpclient): add ListAgents and GetAgent methods to RoleResolver"
```

---

### Task 2: ChatSession model — add AgentName field

**Files:**
- Modify: `internal/core/chat.go:10-18`

**Step 1: Add field**

Add `AgentName` field to `ChatSession` struct:

```go
type ChatSession struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	// AgentSessionID stores provider-native session id (e.g. Claude session_id) for multi-turn resume.
	AgentSessionID string        `json:"agent_session_id,omitempty"`
	AgentName      string        `json:"agent_name,omitempty"`
	Messages       []ChatMessage `json:"messages"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
```

**Step 2: Run existing tests**

Run: `cd D:/project/ai-workflow && go test ./internal/core/...`
Expected: PASS (no behavior change)

**Step 3: Commit**

```bash
git add internal/core/chat.go
git commit -m "feat(core): add AgentName field to ChatSession"
```

---

### Task 3: SQLite migration — add agent_name column

**Files:**
- Modify: `internal/plugins/store-sqlite/migrations.go`
- Modify: `internal/plugins/store-sqlite/store.go` (SQL queries)

**Step 1: Add migration function**

In `migrations.go`, after the last migration function, add:

```go
func migrateAddChatSessionAgentName(db *sql.DB) error {
	if hasColumn(db, "chat_sessions", "agent_name") {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE chat_sessions ADD COLUMN agent_name TEXT NOT NULL DEFAULT ''`)
	return err
}
```

**Step 2: Wire migration into migrate()**

In `migrate()` function, increment `schemaVersion` to 9 and add the call:

Find the section that checks version < current schemaVersion and add:

```go
if version < 9 {
	if err := migrateAddChatSessionAgentName(db); err != nil {
		return fmt.Errorf("migration v9 (chat_sessions.agent_name): %w", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 9`); err != nil {
		return err
	}
}
```

Update `schemaVersion = 9`.

**Step 3: Update SQL queries in store.go**

In `CreateChatSession` — add `agent_name` to INSERT:

```go
`INSERT INTO chat_sessions (id, project_id, agent_session_id, agent_name, messages) VALUES (?,?,?,?,?)`
session.ID, session.ProjectID, session.AgentSessionID, session.AgentName, messagesJSON
```

In `GetChatSession` — add `agent_name` to SELECT:

```go
`SELECT id, project_id, COALESCE(agent_session_id, ''), COALESCE(agent_name, ''), messages, created_at, updated_at FROM chat_sessions WHERE id=?`
```
And scan into `&s.AgentName`.

In `ListChatSessions` — same SELECT change, add `COALESCE(agent_name, '')` and scan.

Note: `UpdateChatSession` does NOT update `agent_name` — it's immutable after creation.

**Step 4: Run tests**

Run: `cd D:/project/ai-workflow && go test ./internal/plugins/store-sqlite/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/plugins/store-sqlite/migrations.go internal/plugins/store-sqlite/store.go
git commit -m "feat(store): add agent_name column to chat_sessions with migration v9"
```

---

### Task 4: Backend — GET /api/v1/agents endpoint

**Files:**
- Create: `internal/web/handlers_agents.go`
- Modify: `internal/web/handlers_v3.go` (route registration)
- Modify: `cmd/ai-flow/server.go` (pass RoleResolver to handler)

**Step 1: Create handler**

```go
// internal/web/handlers_agents.go
package web

import (
	"net/http"

	"github.com/anthropics/ai-workflow/internal/acpclient"
)

type agentHandlers struct {
	resolver *acpclient.RoleResolver
}

type agentInfo struct {
	Name string `json:"name"`
}

type listAgentsResponse struct {
	Agents []agentInfo `json:"agents"`
}

func (h *agentHandlers) list(w http.ResponseWriter, r *http.Request) {
	agents := h.resolver.ListAgents()
	items := make([]agentInfo, 0, len(agents))
	for _, a := range agents {
		items = append(items, agentInfo{Name: a.ID})
	}
	writeJSON(w, http.StatusOK, listAgentsResponse{Agents: items})
}
```

**Step 2: Register route**

In `handlers_v3.go`, inside `registerV1Routes()`, add:

```go
agentH := &agentHandlers{resolver: roleResolver}
r.With(RequireScope(ScopeChatRead)).Get("/agents", agentH.list)
```

This requires passing `roleResolver` into `registerV1Routes`. Check the current signature and add the parameter.

In `cmd/ai-flow/server.go`, where `registerV1Routes` is called, pass `bootstrapSet.RoleResolver`.

**Step 3: Run server tests**

Run: `cd D:/project/ai-workflow && go test ./internal/web/... -run TestAgent -v`
Expected: PASS (or no tests yet, add integration test if needed)

**Step 4: Commit**

```bash
git add internal/web/handlers_agents.go internal/web/handlers_v3.go cmd/ai-flow/server.go
git commit -m "feat(web): add GET /api/v1/agents endpoint"
```

---

### Task 5: Backend — accept agent_name in create session

**Files:**
- Modify: `internal/web/handlers_chat.go:40-44` (request struct)
- Modify: `internal/web/handlers_chat.go:138-253` (createSession method)

**Step 1: Extend request struct**

```go
type createChatSessionRequest struct {
	Message   string `json:"message"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	AgentName string `json:"agent_name"`
}
```

**Step 2: Update createSession logic**

In `createSession`, after creating a new session (around line 188-192):

```go
if isNewSession {
	session = &core.ChatSession{
		ID:        core.NewChatSessionID(),
		ProjectID: projectId,
		AgentName: strings.TrimSpace(req.AgentName),
	}
}
```

For existing sessions, ignore `req.AgentName` — the stored value is used.

**Step 3: Pass agent name to executeChatTurn**

Add `AgentName` to `chatRunInput` struct and populate it:

```go
type chatRunInput struct {
	ProjectID      string
	WorkDir        string
	Role           string
	Message        string
	SessionID      string
	AgentSessionID string
	AgentName      string
}
```

When building chatRunInput:
```go
agentName := session.AgentName
// ... pass to chatRunInput
```

**Step 4: Pass to ChatAssistantRequest**

In `executeChatTurn`, pass `AgentName` to the `ChatAssistantRequest`.

Add `AgentOverride string` to `ChatAssistantRequest` in `chat_assistant_claude.go`:

```go
type ChatAssistantRequest struct {
	Message        string
	Role           string
	WorkDir        string
	AgentSessionID string
	ProjectID      string
	ChatSessionID  string
	AgentOverride  string
}
```

In `executeChatTurn`:
```go
h.assistant.Reply(ctx, ChatAssistantRequest{
	// ... existing fields ...
	AgentOverride: input.AgentName,
})
```

**Step 5: Commit**

```bash
git add internal/web/handlers_chat.go internal/web/chat_assistant_claude.go
git commit -m "feat(web): accept agent_name in create chat session request"
```

---

### Task 6: Backend — ChatAssistant agent override

**Files:**
- Modify: `internal/web/chat_assistant_acp.go:187-231`

**Step 1: Update Reply method**

After resolving role (around line 218), add agent override logic:

```go
agent, role, err := roleResolver.Resolve(roleID)
if err != nil {
	return ChatAssistantResponse{}, fmt.Errorf("resolve chat role %q: %w", roleID, err)
}

// Agent override: use specified agent instead of role's default.
agentOverride := strings.TrimSpace(req.AgentOverride)
if agentOverride != "" && agentOverride != agent.ID {
	overrideAgent, oErr := roleResolver.GetAgent(agentOverride)
	if oErr != nil {
		return ChatAssistantResponse{}, fmt.Errorf("resolve agent override %q: %w", agentOverride, oErr)
	}
	agent = overrideAgent
}
```

**Step 2: Run tests**

Run: `cd D:/project/ai-workflow && go test ./internal/web/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/web/chat_assistant_acp.go
git commit -m "feat(web): support agent override in ChatAssistant Reply"
```

---

### Task 7: Backend — include agent_name in session response

**Files:**
- Modify: `internal/web/handlers_chat.go` — getSession handler and listSessionEvents response

**Step 1: Update createChatSessionResponse**

```go
type createChatSessionResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	AgentName string `json:"agent_name,omitempty"`
}
```

In `createSession`, populate:
```go
writeJSON(w, http.StatusAccepted, createChatSessionResponse{
	SessionID: session.ID,
	Status:    "accepted",
	AgentName: session.AgentName,
})
```

**Step 2: Update listSessionEvents response**

In `listChatSessionEventsResponse`, add `AgentName`:

```go
type listChatSessionEventsResponse struct {
	// ... existing fields ...
	AgentName string `json:"agent_name,omitempty"`
}
```

Populate in `listSessionEvents`:
```go
resp.AgentName = session.AgentName
```

**Step 3: Run backend tests**

Run: `cd D:/project/ai-workflow && go test ./...`
Expected: PASS (some tests may need update for new field)

**Step 4: Commit**

```bash
git add internal/web/handlers_chat.go
git commit -m "feat(web): include agent_name in chat session responses"
```

---

### Task 8: Frontend — API types and client

**Files:**
- Modify: `web/src/types/api.ts`
- Modify: `web/src/lib/apiClient.ts`

**Step 1: Add types**

In `web/src/types/api.ts`:

```typescript
export interface AgentInfo {
  name: string;
}

export interface AgentListResponse {
  agents: AgentInfo[];
}

export interface CreateChatRequest {
  message: string;
  session_id?: string;
  agent_name?: string;  // NEW
}

export interface CreateChatResponse {
  session_id: string;
  status: "accepted" | "running" | "queued" | string;
  agent_name?: string;  // NEW
}
```

**Step 2: Add listAgents to apiClient**

In `web/src/lib/apiClient.ts`, add to ApiClient interface and implementation:

```typescript
// Interface
listAgents(): Promise<AgentListResponse>;

// Implementation
listAgents: () =>
  request<AgentListResponse>({
    path: "/api/v1/agents",
    method: "GET",
  }),
```

**Step 3: Type check**

Run: `cd D:/project/ai-workflow/web && npx tsc --noEmit`
Expected: PASS

**Step 4: Commit**

```bash
git add web/src/types/api.ts web/src/lib/apiClient.ts
git commit -m "feat(web): add agent list API types and client method"
```

---

### Task 9: Frontend — agent selector + ChatView style overhaul

This is the large frontend task. ChatView gets:
1. Agent selector dropdown
2. Full-height flex layout (replace fixed `h-[30rem]`)
3. Mono font throughout
4. Compact sidebar styling
5. Improved input area with agent selector inline

**Files:**
- Modify: `web/src/views/ChatView.tsx`

**Step 1: Add state for agents**

Near other state declarations (around line 766):

```typescript
const [agents, setAgents] = useState<Array<{ name: string }>>([]);
const [selectedAgent, setSelectedAgent] = useState("claude");
const [agentsLoading, setAgentsLoading] = useState(false);
```

**Step 2: Fetch agents on mount**

Add useEffect after other mount effects:

```typescript
useEffect(() => {
  setAgentsLoading(true);
  void apiClient.listAgents().then((res) => {
    setAgents(res.agents ?? []);
    if (res.agents?.length > 0) {
      setSelectedAgent(res.agents[0].name);
    }
  }).catch(() => {}).finally(() => setAgentsLoading(false));
}, [apiClient]);
```

**Step 3: Pass agent_name in handleStartChat**

In `handleStartChat`, when building the createChat payload for a new session:

```typescript
const payload = currentSessionId
  ? { message, session_id: currentSessionId }
  : { message, agent_name: selectedAgent };
```

**Step 4: Update outer layout — full height flex**

Change the outer `<section>` from grid with fixed heights to full-height:

```tsx
<section className={`grid h-[calc(100vh-4rem)] gap-3 font-mono ${
  leftPanelOpen
    ? "lg:grid-cols-[240px_minmax(0,2fr)_280px]"
    : "lg:grid-cols-[minmax(0,2fr)_280px]"
}`}>
```

**Step 5: Left panel — compact**

Change left aside: `lg:min-h-[680px]` → remove (it fills via grid), reduce padding `p-4` → `p-3`, font sizes to `text-xs`.

**Step 6: Center column — flex layout**

Replace the center `<div>` from `rounded-xl border ... p-4 shadow-sm` to:

```tsx
<div className="flex min-w-0 flex-col overflow-hidden rounded-lg border border-slate-200 bg-white">
```

Remove header "Chat" title and description paragraph (the TUI style doesn't need them).

Chat timeline container: remove `mt-4 h-[30rem]` fixed height, use `flex-1 min-h-0`:

```tsx
<div className="flex flex-1 min-h-0 border-t border-slate-100">
  <div ref={timelineScrollRef} className="flex-1 overflow-y-auto font-mono text-sm" ...>
```

**Step 7: Input area — with agent selector**

Replace the input area with compact layout:

```tsx
<div className="border-t border-slate-200 p-3">
  <textarea ... className="min-h-[3rem] max-h-[8rem] w-full resize-y ..." />
  <div className="mt-2 flex items-center justify-between">
    <div className="flex items-center gap-2">
      {!sessionId ? (
        <select
          className="rounded border border-slate-300 bg-slate-50 px-2 py-1 font-mono text-xs"
          value={selectedAgent}
          onChange={(e) => setSelectedAgent(e.target.value)}
          disabled={chatLoading}
        >
          {agents.map((a) => (
            <option key={a.name} value={a.name}>{a.name}</option>
          ))}
        </select>
      ) : (
        <span className="rounded bg-slate-100 px-2 py-1 font-mono text-xs text-slate-500">
          {/* Display locked agent name from session */}
          agent: {selectedAgent}
        </span>
      )}
    </div>
    <button ... >{submitButtonLabel}</button>
  </div>
</div>
```

**Step 8: Right sidebar — compact terminal style**

Change right aside from `rounded-xl border ... p-4 shadow-sm` to:

```tsx
<aside className="flex flex-col overflow-hidden rounded-lg border border-slate-200 bg-white">
```

- Section headers: reduce to `text-xs font-semibold uppercase tracking-wider text-slate-400`
- Session list items: tighter padding `px-2 py-1.5 text-xs`
- Session ID display: `font-mono text-[10px]`
- Issue select and run events: `font-mono text-xs`
- Remove `shadow-sm` everywhere

**Step 9: Streaming cursor animation**

Add a blinking cursor to the streaming indicator:

```tsx
{isStreaming && streamingText.length > 0 && (
  <span className="inline-block h-4 w-1.5 animate-pulse bg-slate-400" />
)}
```

**Step 10: Type check and test**

Run: `cd D:/project/ai-workflow/web && npx tsc --noEmit`
Run: `cd D:/project/ai-workflow/web && npx vitest run`
Expected: PASS (some tests may need DOM query updates)

**Step 11: Commit**

```bash
git add web/src/views/ChatView.tsx
git commit -m "feat(web): add agent selector and overhaul ChatView to full-height TUI style"
```

---

### Task 10: Fix ChatView tests

**Files:**
- Modify: `web/src/views/ChatView.test.tsx`

**Step 1: Update test setup**

Tests that create ChatView need to mock the new `listAgents` API. Add to the mock apiClient:

```typescript
listAgents: vi.fn().mockResolvedValue({ agents: [{ name: "claude" }, { name: "codex" }] }),
```

**Step 2: Update DOM queries**

Fix any broken queries due to layout changes (removed header text, changed class names, etc.).

**Step 3: Run tests**

Run: `cd D:/project/ai-workflow/web && npx vitest run`
Expected: All PASS

**Step 4: Commit**

```bash
git add web/src/views/ChatView.test.tsx
git commit -m "test(web): update ChatView tests for agent selector and style changes"
```

---

### Task 11: Backend tests + final verification

**Files:**
- Modify: `cmd/ai-flow/commands_test.go` (if chat handler tests exist)

**Step 1: Run all backend tests**

Run: `cd D:/project/ai-workflow && go test ./...`
Expected: All PASS

**Step 2: Run all frontend tests**

Run: `cd D:/project/ai-workflow/web && npx vitest run`
Expected: All PASS

**Step 3: Build verification**

Run: `cd D:/project/ai-workflow/web && npm run build`
Run: `cd D:/project/ai-workflow && go build ./cmd/ai-flow`
Expected: Both succeed

**Step 4: Commit any remaining fixes**

```bash
git add -A
git commit -m "fix: address test failures from agent selector feature"
```

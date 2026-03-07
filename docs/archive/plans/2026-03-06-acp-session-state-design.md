# ACP Session State: Commands, ConfigOptions, and SessionUpdate Optimization

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Optimize SessionUpdate to carry raw JSON + structured commands/config, expose session-level commands and config options to frontend via API + WebSocket.

**Architecture:** Refactor `acpclient.SessionUpdate` to replace redundant string fields with `json.RawMessage` + typed slices. Add session state tracking in `pooledChatSession`. Expose via REST API and real-time WebSocket. Frontend stores and renders command palette + config selector.

**Tech Stack:** Go (acpclient, teamleader, web packages), ACP Go SDK, React + Zustand + TypeScript

**Date**: 2026-03-06
**Status**: Approved

## Summary

Three changes in one pass:

1. **SessionUpdate structure optimization** — replace `RawUpdateJSON`/`RawContentJSON` strings with a single `json.RawMessage` field; extract `Commands`/`ConfigOptions` at decode time
2. **Available commands** — capture `available_commands_update` into session state, expose via API and WebSocket, frontend `/` command palette
3. **Config options** — capture `config_option_update` and `session/new` response, expose via API and WebSocket, frontend config selector, support `session/set_config_option`

## Part 1: SessionUpdate Structure

### Before

```go
type SessionUpdate struct {
    SessionID      string `json:"sessionId"`
    Type           string `json:"type"`
    Text           string `json:"text,omitempty"`
    Status         string `json:"status,omitempty"`
    RawUpdateJSON  string `json:"rawUpdateJson,omitempty"`  // redundant string
    RawContentJSON string `json:"rawContentJson,omitempty"` // unused
}
```

### After

```go
type SessionUpdate struct {
    SessionID string            `json:"sessionId"`
    Type      string            `json:"type"`
    Text      string            `json:"text,omitempty"`
    Status    string            `json:"status,omitempty"`
    RawJSON   json.RawMessage   `json:"raw"`

    Commands      []acpproto.AvailableCommand          `json:"-"`
    ConfigOptions []acpproto.SessionConfigOptionSelect  `json:"-"`
}
```

### Changes

- Delete `RawUpdateJSON` (string) and `RawContentJSON` (string, never consumed)
- Add `RawJSON` (`json.RawMessage`) — zero-copy from `json.Marshal(notification.Update)`
- Add `Commands` — populated when `Type == "available_commands_update"`
- Add `ConfigOptions` — populated when `Type == "config_option_update"`
- `decodeACPNotificationFromStruct`: extract structured fields from SDK types
- Consumers: `json.Unmarshal([]byte(update.RawUpdateJSON), ...)` → `json.Unmarshal(update.RawJSON, ...)`

### Fixture format

Before (redundant):
```json
{
  "offset_ms": 2216,
  "update": {
    "sessionId": "...",
    "type": "agent_message_chunk",
    "text": "_OK",
    "rawUpdateJson": "{...}",
    "rawContentJson": "{...}"
  }
}
```

After (raw ACP notification only):
```json
{
  "offset_ms": 2216,
  "raw": {
    "sessionId": "...",
    "update": {
      "sessionUpdate": "agent_message_chunk",
      "content": {"type": "text", "text": "_OK"}
    }
  }
}
```

- fixture_agent sends `raw` directly as `session/update` params
- No extracted/duplicated fields

## Part 2: Available Commands

### Backend

**Session state**: `pooledChatSession` gains:
```go
commandsMu sync.RWMutex
commands   []acpproto.AvailableCommand
```

Updated when `ACPHandler.HandleSessionUpdate` receives `available_commands_update` (reads `update.Commands`).

**API**:
```
GET /api/v1/chat/sessions/{id}/commands → []AvailableCommand
```

Returns current commands list from session memory. 404 if session not found.

**WebSocket**: already broadcasts `RawJSON` via existing `EventRunUpdate` path — frontend receives the full `available_commands_update` payload.

### Frontend

**Store** (`chatStore`):
```typescript
commandsBySessionId: Record<string, AvailableCommand[]>
```

Updated on:
- WS message with `acp.sessionUpdate === "available_commands_update"`
- Initial fetch via GET on session connect/reconnect

**UI**: Input box `/` trigger → popup list of commands (name + description). Selection fills input with `/commandName [hint]`.

## Part 3: Config Options

### Backend

**Session state**: `pooledChatSession` gains:
```go
configOptions []acpproto.SessionConfigOptionSelect
```

Updated when:
- `config_option_update` notification received
- `session/new` or `session/load` response contains `configOptions`

**API**:
```
GET  /api/v1/chat/sessions/{id}/config-options → []ConfigOptionSelect
POST /api/v1/chat/sessions/{id}/config-options → {configId, value} → []ConfigOptionSelect
```

POST calls `client.SetConfigOption()` (new method on `acpclient.Client`), which sends `session/set_config_option` to the agent. Agent returns complete config state; backend updates session and returns it.

**`acpclient.Client` new method**:
```go
func (c *Client) SetConfigOption(ctx context.Context, req acpproto.SetConfigOptionRequest) ([]acpproto.SessionConfigOptionSelect, error)
```

### Frontend

**Store** (`chatStore`):
```typescript
configOptionsBySessionId: Record<string, ConfigOption[]>
```

**UI**: Config selector in chat header/toolbar. Each option rendered as a dropdown (`type: "select"`). Change triggers POST → updates store from response.

## LoadSession Suppress Refinement

The existing `ACPHandler.SetSuppressEvents(true)` blocks WS broadcast and persistence. But during `LoadSession`, replayed `available_commands_update` and `config_option_update` must still update session memory state.

Suppress behavior:
- suppress=true: **write** commands/configOptions to session state, **skip** publish + record
- suppress=false: write + publish + record (normal)

## Boundary Cases

| Scenario | Handling |
|----------|----------|
| `session/new` response includes `configOptions` | Parse and initialize session state |
| `LoadSession` replays `available_commands_update` | Write to state (suppress skips broadcast) |
| Agent pushes `config_option_update` (e.g. model fallback) | Update state + WS broadcast |
| `SetConfigOption` agent error | Return error to frontend, no local state change |
| Session not found or closed | 404 |
| Empty commands/configOptions arrays | Frontend hides respective UI |

## Out of Scope

- No DB persistence for commands/configOptions (memory, tied to session lifecycle)
- No command argument input UI (`input.hint` shown as placeholder text only)
- No custom configOption types (only `select` per protocol)
- No eventbus refactor for ACP notification handling

---

# Implementation Plan

## Task 1: Refactor SessionUpdate struct and decode path

**Files:**
- Modify: `internal/acpclient/protocol.go:24-31`
- Modify: `internal/acpclient/client.go:527-598` (decodeACPNotificationFromStruct)
- Modify: `internal/acpclient/client.go:389-454` (handleNotification legacy path)

**Step 1: Update SessionUpdate struct**

In `internal/acpclient/protocol.go`, replace the struct:

```go
type SessionUpdate struct {
	SessionID string          `json:"sessionId"`
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Status    string          `json:"status,omitempty"`
	RawJSON   json.RawMessage `json:"raw,omitempty"`

	// Structured fields — only populated for specific types, not serialized.
	Commands      []acpproto.AvailableCommand          `json:"-"`
	ConfigOptions []acpproto.SessionConfigOptionSelect  `json:"-"`
}
```

Add `"encoding/json"` and `acpproto` imports to protocol.go.

**Step 2: Update decodeACPNotificationFromStruct**

In `internal/acpclient/client.go:527-598`, change:
- `rawContent` variable and all `rawContentJson` assignments → delete
- `AvailableCommandsUpdate` case: extract `notification.Update.AvailableCommandsUpdate.AvailableCommands` into `Commands` field
- `ConfigOptionUpdate` case: extract config options — note `SessionConfigOptionUpdate.ConfigOptions` is `[]SessionConfigOption` (union), need to unwrap each `.Select` into `ConfigOptions` field
- At the bottom, replace `RawUpdateJSON: strings.TrimSpace(string(rawUpdate))` and `RawContentJSON: rawContent` with `RawJSON: rawUpdate` (already `[]byte` from `json.Marshal`)

**Step 3: Update handleNotification legacy path**

In `internal/acpclient/client.go:389-454`, change:
- Replace `RawUpdateJSON: strings.TrimSpace(string(in.Update))` with `RawJSON: json.RawMessage(in.Update)`
- Delete `RawContentJSON` assignment (line 445)

**Step 4: Build and verify compilation**

Run: `go build ./internal/acpclient/...`
Expected: Compilation errors in downstream consumers (engine, teamleader) — this is expected, fixed in Task 2.

**Step 5: Commit**

```bash
git add internal/acpclient/protocol.go internal/acpclient/client.go
git commit -m "refactor(acpclient): replace RawUpdateJSON/RawContentJSON with RawJSON + Commands/ConfigOptions"
```

---

## Task 2: Update all downstream consumers of RawUpdateJSON/RawContentJSON

**Files:**
- Modify: `internal/engine/executor_acp.go:518,546,579`
- Modify: `internal/teamleader/acp_handler.go:419,444,513`
- Modify: `internal/engine/executor_acp_test.go:86,100,112`
- Modify: `internal/teamleader/acp_handler_test.go:408-409,457,492,558,566,611,620`
- Modify: `internal/acpclient/client_test.go:142-143`

**Step 1: Fix engine/executor_acp.go**

Three methods reference `update.RawUpdateJSON`:
- `publishToolCall` (line 518): `json.Unmarshal([]byte(update.RawUpdateJSON), &parsed)` → `json.Unmarshal(update.RawJSON, &parsed)`
- `publishToolCallCompleted` (line 546): same change
- `publishUsageUpdate` (line 579): same change

**Step 2: Fix teamleader/acp_handler.go**

- Line 419: `update.RawUpdateJSON` → `string(update.RawJSON)` (for `data["acp_update_json"]`)
- Line 444: same pattern
- Line 513: `extractACPChunkText(update.RawUpdateJSON)` → `extractACPChunkText(string(update.RawJSON))`

**Step 3: Fix test files**

In all test files, replace `RawUpdateJSON:` with `RawJSON: json.RawMessage(...)` and delete any `RawContentJSON:` lines.

- `internal/acpclient/client_test.go:142-143`: change assertion from `update.RawContentJSON` to `update.RawJSON`
- `internal/engine/executor_acp_test.go:86,100,112`: `RawUpdateJSON: raw` → `RawJSON: json.RawMessage(raw)`
- `internal/teamleader/acp_handler_test.go`: all `RawUpdateJSON:` → `RawJSON: json.RawMessage(...)`, delete `RawContentJSON:` lines

**Step 4: Build and test**

Run: `go build ./... && go test ./internal/acpclient/... ./internal/engine/... ./internal/teamleader/... -timeout 60s`
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/engine/ internal/teamleader/ internal/acpclient/
git commit -m "refactor: migrate all consumers from RawUpdateJSON to RawJSON"
```

---

## Task 3: Update fixture format and fixture_agent

**Files:**
- Modify: `internal/acpclient/testdata/codex_fixtures.json` (regenerate)
- Modify: `internal/acpclient/testdata/fixture_agent.go:23-26,225-268`
- Modify: `internal/acpclient/fixture_test.go`
- Modify: `cmd/acp-probe/capture_test.go`

**Step 1: Update capture_test.go fixture format**

In `cmd/acp-probe/capture_test.go`, change `FixtureEvent` struct:
```go
type FixtureEvent struct {
	OffsetMs int64           `json:"offset_ms"`
	Raw      json.RawMessage `json:"raw"` // full ACP notification params: {sessionId, update}
}
```

Update the `captureRecorder` to store the raw ACP notification JSON. The `HandleSessionUpdate` receives a `SessionUpdate` with `RawJSON` — build the notification wrapper:
```go
func (r *captureRecorder) HandleSessionUpdate(_ context.Context, u acpclient.SessionUpdate) error {
	// Build the ACP notification params format: {sessionId, update}
	raw, _ := json.Marshal(map[string]any{
		"sessionId": u.SessionID,
		"update":    json.RawMessage(u.RawJSON),
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, FixtureEvent{
		OffsetMs: time.Since(r.start).Milliseconds(),
		Raw:      json.RawMessage(raw),
	})
	return nil
}
```

**Step 2: Regenerate codex_fixtures.json**

Run: `go test ./cmd/acp-probe/ -run TestCaptureACPEvents -v -timeout 300s`
Expected: New fixture file with `"raw": {sessionId, update}` format.

**Step 3: Update fixture_agent.go**

Change `fixtureEvent` struct:
```go
type fixtureEvent struct {
	OffsetMs int64           `json:"offset_ms"`
	Raw      json.RawMessage `json:"raw"`
}
```

Simplify `replayEvents`: instead of parsing `rawUpdateJson`, extract `update` from `raw` and send directly:
```go
func (s *server) replayEvents(sessionID string, sc fixtureScenario) error {
	var lastOffset int64
	for _, evt := range sc.Events {
		gap := evt.OffsetMs - lastOffset
		if gap > 10 { gap = 10 }
		if gap > 0 { time.Sleep(time.Duration(gap) * time.Millisecond) }
		lastOffset = evt.OffsetMs

		// Parse raw to extract "update" field, override sessionId.
		var notification map[string]json.RawMessage
		if err := json.Unmarshal(evt.Raw, &notification); err != nil {
			continue
		}
		updateRaw, ok := notification["update"]
		if !ok || len(updateRaw) == 0 {
			continue
		}
		msg := map[string]any{
			"jsonrpc": "2.0",
			"method":  "session/update",
			"params": map[string]any{
				"sessionId": sessionID,
				"update":    json.RawMessage(updateRaw),
			},
		}
		if err := s.write(msg); err != nil {
			return err
		}
	}
	return nil
}
```

**Step 4: Update fixture_test.go**

Tests should still pass since they assert on `Type`/`Text`/counts — the fixture_agent still sends the same ACP notifications, just sourced differently. Run and verify.

**Step 5: Run all fixture tests**

Run: `go test ./internal/acpclient/ -run TestFixture -v -timeout 60s`
Expected: All 5 tests pass.

**Step 6: Commit**

```bash
git add cmd/acp-probe/capture_test.go internal/acpclient/testdata/ internal/acpclient/fixture_test.go
git commit -m "refactor: simplify fixture format to raw ACP notification JSON"
```

---

## Task 4: Add Commands/ConfigOptions to session state + suppress refinement

**Files:**
- Modify: `internal/web/chat_assistant_acp.go:78-88` (pooledChatSession)
- Modify: `internal/teamleader/acp_handler.go:37-58` (ACPHandler struct)
- Modify: `internal/teamleader/acp_handler.go:391-475` (HandleSessionUpdate)

**Step 1: Add state callback to ACPHandler**

In `internal/teamleader/acp_handler.go`, add a callback type and field:

```go
// SessionStateCallback is invoked when commands or config options change.
type SessionStateCallback func(commands []acpproto.AvailableCommand, configOptions []acpproto.SessionConfigOptionSelect)
```

Add to `ACPHandler` struct:
```go
stateCallback SessionStateCallback
```

Add setter:
```go
func (h *ACPHandler) SetSessionStateCallback(cb SessionStateCallback) {
	if h == nil { return }
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateCallback = cb
}
```

**Step 2: Update HandleSessionUpdate to capture commands/configOptions**

In `HandleSessionUpdate`, after the suppress check but before publish/record, add:

```go
// Capture structured session state (even when suppressed).
if len(update.Commands) > 0 || len(update.ConfigOptions) > 0 {
	h.mu.Lock()
	cb := h.stateCallback
	h.mu.Unlock()
	if cb != nil {
		cb(update.Commands, update.ConfigOptions)
	}
}

if suppress {
	return nil
}
```

Move the suppress early-return **after** the state capture block.

**Step 3: Add state fields to pooledChatSession**

In `internal/web/chat_assistant_acp.go`, add to `pooledChatSession`:

```go
stateMu       sync.RWMutex
commands      []acpproto.AvailableCommand
configOptions []acpproto.SessionConfigOptionSelect
```

Add getter/setter methods:
```go
func (p *pooledChatSession) setCommands(cmds []acpproto.AvailableCommand) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.commands = cmds
}

func (p *pooledChatSession) getCommands() []acpproto.AvailableCommand {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	out := make([]acpproto.AvailableCommand, len(p.commands))
	copy(out, p.commands)
	return out
}

func (p *pooledChatSession) setConfigOptions(opts []acpproto.SessionConfigOptionSelect) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.configOptions = opts
}

func (p *pooledChatSession) getConfigOptions() []acpproto.SessionConfigOptionSelect {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	out := make([]acpproto.SessionConfigOptionSelect, len(p.configOptions))
	copy(out, p.configOptions)
	return out
}
```

**Step 4: Wire callback in Reply**

In `ACPChatAssistant.Reply`, after creating `pooled` (both new and reused paths), set the callback:

```go
if pooled.handler != nil {
	pooled.handler.SetSessionStateCallback(func(cmds []acpproto.AvailableCommand, opts []acpproto.SessionConfigOptionSelect) {
		if len(cmds) > 0 {
			pooled.setCommands(cmds)
		}
		if len(opts) > 0 {
			pooled.setConfigOptions(opts)
		}
	})
}
```

**Step 5: Parse configOptions from session/new and session/load responses**

In `acpclient/client.go`, update `NewSession` and `LoadSession` to extract `configOptions` from the response. Add a field to the Client or return them.

Simpler approach: parse in `startWebChatSession` after the call returns. The `NewSessionResponse` and `LoadSessionResponse` in the ACP SDK contain `ConfigOptions []SessionConfigOption`. We currently discard the full response. Change `NewSession`/`LoadSession` to return the config options too.

Add a new return type or extend the existing ones:
```go
type SessionResult struct {
	SessionID     acpproto.SessionId
	ConfigOptions []acpproto.SessionConfigOptionSelect
}
```

Update `NewSession` to return `SessionResult`:
- Parse `configOptions` from the response JSON
- Unwrap `SessionConfigOption.Select` into `[]SessionConfigOptionSelect`

Wire in `startWebChatSession`: after `NewSession`/`LoadSession`, call `pooled.setConfigOptions(result.ConfigOptions)`.

**Step 6: Build and test**

Run: `go build ./... && go test ./internal/teamleader/... ./internal/web/... -timeout 60s`
Expected: Pass (existing tests don't assert on commands/configOptions).

**Step 7: Commit**

```bash
git add internal/teamleader/acp_handler.go internal/web/chat_assistant_acp.go internal/acpclient/client.go
git commit -m "feat: capture commands and config options in session state"
```

---

## Task 5: Add Client.SetConfigOption method

**Files:**
- Modify: `internal/acpclient/client.go`
- Create: `internal/acpclient/client_setconfig_test.go`

**Step 1: Write test**

```go
func TestSetConfigOptionCallsTransport(t *testing.T) {
	// Use fake_agent — it returns -32601 for unknown methods.
	// We just verify the client sends the RPC and handles errors.
	cfg := testLaunchConfig(t)
	h := &recordingHandler{}
	client, err := New(cfg, h)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer client.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Initialize(ctx, ClientCapabilities{FSRead: true}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	_, err = client.SetConfigOption(ctx, acpproto.SetSessionConfigOptionRequest{
		SessionId: "fake-session-1",
		ConfigId:  "model",
		Value:     "model-2",
	})
	// fake_agent doesn't handle this method — expect error
	if err == nil {
		t.Fatal("expected error from fake agent for unsupported method")
	}
}
```

**Step 2: Implement SetConfigOption**

In `internal/acpclient/client.go`:

```go
func (c *Client) SetConfigOption(ctx context.Context, req acpproto.SetSessionConfigOptionRequest) ([]acpproto.SessionConfigOptionSelect, error) {
	raw, err := c.transport.Call(ctx, "session/set_config_option", req)
	if err != nil {
		return nil, err
	}
	var resp acpproto.SetSessionConfigOptionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode set_config_option response: %w", err)
	}
	var options []acpproto.SessionConfigOptionSelect
	for _, opt := range resp.ConfigOptions {
		if opt.Select != nil {
			options = append(options, *opt.Select)
		}
	}
	return options, nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/acpclient/ -run TestSetConfigOption -v`
Expected: Pass.

**Step 4: Commit**

```bash
git add internal/acpclient/client.go internal/acpclient/client_setconfig_test.go
git commit -m "feat(acpclient): add SetConfigOption method for session/set_config_option RPC"
```

---

## Task 6: Add REST API for commands and config options

**Files:**
- Modify: `internal/web/handlers_chat.go:85-100` (route registration)
- Modify: `internal/web/chat_assistant_acp.go` (add public methods to ACPChatAssistant)

**Step 1: Add public accessor methods to ACPChatAssistant**

```go
func (a *ACPChatAssistant) GetSessionCommands(chatSessionID string) ([]acpproto.AvailableCommand, error) {
	ps := a.getPooledSession(chatSessionID)
	if ps == nil {
		return nil, fmt.Errorf("session %q not found", chatSessionID)
	}
	return ps.getCommands(), nil
}

func (a *ACPChatAssistant) GetSessionConfigOptions(chatSessionID string) ([]acpproto.SessionConfigOptionSelect, error) {
	ps := a.getPooledSession(chatSessionID)
	if ps == nil {
		return nil, fmt.Errorf("session %q not found", chatSessionID)
	}
	return ps.getConfigOptions(), nil
}

func (a *ACPChatAssistant) SetSessionConfigOption(ctx context.Context, chatSessionID string, configID string, value string) ([]acpproto.SessionConfigOptionSelect, error) {
	ps := a.getPooledSession(chatSessionID)
	if ps == nil {
		return nil, fmt.Errorf("session %q not found", chatSessionID)
	}
	opts, err := ps.client.SetConfigOption(ctx, acpproto.SetSessionConfigOptionRequest{
		SessionId: ps.sessionID,
		ConfigId:  acpproto.SessionConfigId(configID),
		Value:     acpproto.SessionConfigValueId(value),
	})
	if err != nil {
		return nil, err
	}
	ps.setConfigOptions(opts)
	return opts, nil
}
```

**Step 2: Add ChatAssistant interface methods**

In `internal/web/handlers_chat.go`, the `ChatAssistant` interface needs extending. Check where it's defined and add the 3 new methods. If the interface is minimal (only `Reply` + `CancelChat`), add them.

**Step 3: Add route handlers**

In `registerChatRoutes`, add:
```go
r.With(RequireScope(ScopeChatRead)).Get("/projects/{projectID}/chat/{sessionID}/commands", h.getSessionCommands)
r.With(RequireScope(ScopeChatRead)).Get("/projects/{projectID}/chat/{sessionID}/config-options", h.getSessionConfigOptions)
r.With(RequireScope(ScopeChatWrite)).Post("/projects/{projectID}/chat/{sessionID}/config-options", h.setSessionConfigOption)
```

Handler implementations follow existing patterns in `handlers_chat.go`: extract `sessionID` from URL, call assistant method, `writeJSON` response.

**Step 4: Build and test**

Run: `go build ./... && go test ./internal/web/... -timeout 60s`
Expected: Pass.

**Step 5: Commit**

```bash
git add internal/web/handlers_chat.go internal/web/chat_assistant_acp.go
git commit -m "feat(api): add commands and config-options endpoints for chat sessions"
```

---

## Task 7: Frontend types and store

**Files:**
- Modify: `web/src/types/ws.ts:20-38`
- Modify: `web/src/stores/chatStore.ts:17-30,39-81`

**Step 1: Add TypeScript types**

In `web/src/types/ws.ts`, add:

```typescript
export interface AvailableCommand {
  name: string;
  description: string;
  input?: { hint?: string };
}

export interface ConfigOptionValue {
  value: string;
  name: string;
  description?: string;
}

export interface ConfigOption {
  id: string;
  name: string;
  description?: string;
  category?: string;
  type: "select";
  currentValue: string;
  options: ConfigOptionValue[];
}
```

**Step 2: Add store state**

In `web/src/stores/chatStore.ts`, add to state interface:

```typescript
commandsBySessionId: Record<string, AvailableCommand[]>;
configOptionsBySessionId: Record<string, ConfigOption[]>;
```

Add actions:
```typescript
setCommands: (sessionId: string, commands: AvailableCommand[]) => void;
setConfigOptions: (sessionId: string, options: ConfigOption[]) => void;
```

Initialize with empty objects in the store creator.

**Step 3: Handle WS updates**

Find where `ChatEventPayload` is processed (the WS message handler). Add cases:

```typescript
if (payload.acp?.sessionUpdate === "available_commands_update" && payload.acp?.availableCommands) {
  chatStore.getState().setCommands(sessionId, payload.acp.availableCommands);
}
if (payload.acp?.sessionUpdate === "config_option_update" && payload.acp?.configOptions) {
  chatStore.getState().setConfigOptions(sessionId, payload.acp.configOptions);
}
```

**Step 4: Add API client functions**

In `web/src/lib/apiClient.ts`, add:

```typescript
export async function getSessionCommands(projectId: string, sessionId: string): Promise<AvailableCommand[]> { ... }
export async function getSessionConfigOptions(projectId: string, sessionId: string): Promise<ConfigOption[]> { ... }
export async function setSessionConfigOption(projectId: string, sessionId: string, configId: string, value: string): Promise<ConfigOption[]> { ... }
```

**Step 5: Build**

Run: `npm --prefix web run typecheck`
Expected: Pass.

**Step 6: Commit**

```bash
git add web/src/types/ws.ts web/src/stores/chatStore.ts web/src/lib/apiClient.ts
git commit -m "feat(web): add commands and config options types, store, and API client"
```

---

## Task 8: Frontend command palette UI

**Files:**
- Create: `web/src/components/CommandPalette.tsx`
- Modify: `web/src/views/ChatView.tsx` (integrate with input box)

**Step 1: Create CommandPalette component**

A dropdown that:
- Shows when user types `/` at start of input
- Filters commands as user types after `/`
- Each item shows `name` and `description`
- Selection fills input with `/commandName ` (with trailing space for optional input)
- Dismiss on Escape or click outside

**Step 2: Integrate in ChatView**

Wire the input's `onChange` to detect `/` prefix, render `CommandPalette` positioned above the input, handle selection.

**Step 3: Build and verify**

Run: `npm --prefix web run build`
Expected: Pass.

**Step 4: Commit**

```bash
git add web/src/components/CommandPalette.tsx web/src/views/ChatView.tsx
git commit -m "feat(web): add /command palette in chat input"
```

---

## Task 9: Frontend config option selector UI

**Files:**
- Create: `web/src/components/ConfigSelector.tsx`
- Modify: `web/src/views/ChatView.tsx` (integrate in header/toolbar)

**Step 1: Create ConfigSelector component**

For each config option in store:
- Render a labeled dropdown (`option.name` as label)
- Show `option.options` as select items (`name` as display, `value` as key)
- Selected value = `option.currentValue`
- On change, call `setSessionConfigOption` API → update store from response
- Hide if `configOptions` is empty

Category-based ordering: render in array order (agent priority).

**Step 2: Integrate in ChatView**

Add `ConfigSelector` to the chat header/toolbar area.

**Step 3: Build and verify**

Run: `npm --prefix web run build`
Expected: Pass.

**Step 4: Commit**

```bash
git add web/src/components/ConfigSelector.tsx web/src/views/ChatView.tsx
git commit -m "feat(web): add config option selector in chat toolbar"
```

---

## Task 10: Integration test with fixture agent

**Files:**
- Modify: `internal/acpclient/testdata/fixture_agent.go` (add config_option_update and available_commands_update scenarios)
- Modify: `internal/acpclient/fixture_test.go` (add assertions for Commands/ConfigOptions fields)

**Step 1: Add fixture scenarios with commands and config options**

Add two new scenarios to `codex_fixtures.json` manually (or extend capture_test.go). For fast testing, add them by hand:

A `commands_and_config` scenario with events of types `available_commands_update` and `config_option_update`.

**Step 2: Write fixture test asserting structured fields**

```go
func TestFixtureCommandsAndConfigOptionsExtracted(t *testing.T) {
	// Use a fixture scenario that includes available_commands_update
	// Verify update.Commands is populated after decoding
}
```

**Step 3: Run tests**

Run: `go test ./internal/acpclient/ -run TestFixture -v -timeout 60s`
Expected: All pass including new test.

**Step 4: Commit**

```bash
git add internal/acpclient/testdata/ internal/acpclient/fixture_test.go
git commit -m "test: add fixture tests for Commands/ConfigOptions extraction"
```

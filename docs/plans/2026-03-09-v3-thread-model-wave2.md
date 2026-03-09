# Wave 2 Plan: Web And Runtime Adoption

## Wave Goal

让 web/chat/ws/assistant/decompose 进入 thread-backed 语义，同时保留现有 `/chat`、`session_id` 和现有前端行为的兼容壳。

## Tasks

### Task W2-T1: 新增 Thread 读取型 REST API

**Files:**
- Create: `internal/web/handlers_thread.go`
- Modify: `internal/web/handlers_v3.go`
- Test: `internal/web/handlers_thread_test.go`

**Depends on:** `[W1-T3]`

**Step 1: Write failing test**
```text
新增 thread handler 测试，覆盖：
- GET /projects/{projectID}/threads
- GET /projects/{projectID}/threads/{threadID}
- GET /projects/{projectID}/threads/{threadID}/events
断言返回包含 issue_id、topic、status、participants。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web/... -run "TestListThreads|TestGetThread|TestListThreadEvents" -count=1`
Expected: FAIL，route / handler / response shape 尚不存在。

**Step 3: Minimal implementation**
```text
新增 thread read API：
- 只读优先，不在本 task 里重做 chat turn 提交
- 复用现有 chat_run_events 读取链路
- 返回 thread 领域字段，不移除旧 chat API
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web/... -run "TestListThreads|TestGetThread|TestListThreadEvents" -count=1`
Expected: PASS，新 thread API 可读取 thread-backed 数据。

**Step 5: Commit**
```bash
git add internal/web/handlers_thread.go internal/web/handlers_v3.go internal/web/handlers_thread_test.go
git commit -m "feat(web): add thread read api"
```

### Task W2-T2: 让 `/chat` 路径成为 Thread 兼容壳

**Files:**
- Modify: `internal/web/handlers_chat.go`
- Modify: `internal/web/handlers_chat_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
给现有 chat handler 增加兼容断言：
- create chat response 增加 thread_id
- get/list chat session 结果能带出 thread topic/status/issue_id
- 旧 session_id 字段继续存在
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web/... -run "TestCreateChatSessionThenGetChatSession|TestListChatSessions|TestCreateChatSessionContinuesExistingSessionWithAssistant" -count=1`
Expected: FAIL，返回结构里没有 thread alias 或旧 handler 仍只按 ChatSession 理解数据。

**Step 3: Minimal implementation**
```text
改造 handlers_chat：
- 内部优先读取 Thread
- 对外仍保持 `/chat` 路径和 `session_id`
- 在响应体中新增 `thread_id`
- 需要时补充 topic/status/issue_id，但不得删旧字段
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web/... -run "TestCreateChatSessionThenGetChatSession|TestListChatSessions|TestCreateChatSessionContinuesExistingSessionWithAssistant" -count=1`
Expected: PASS，/chat 主路径兼容但已具备 thread-backed 语义。

**Step 5: Commit**
```bash
git add internal/web/handlers_chat.go internal/web/handlers_chat_test.go
git commit -m "feat(web): make chat handlers thread-backed"
```

### Task W2-T3: WebSocket、ACP assistant 与 decompose 进入 thread-aware 元数据

**Files:**
- Modify: `internal/web/ws.go`
- Modify: `internal/web/ws_test.go`
- Modify: `internal/web/chat_assistant_claude.go`
- Modify: `internal/web/chat_assistant_acp.go`
- Modify: `internal/web/chat_assistant_acp_test.go`
- Modify: `internal/web/handlers_decompose.go`
- Modify: `internal/web/handlers_decompose_test.go`

**Depends on:** `[W2-T2]`

**Step 1: Write failing test**
```text
新增和补强测试，覆盖：
- subscribe_thread 作为 subscribe_chat_session 的新别名
- WS 广播中能带 thread_id
- ChatAssistantRequest 支持 ThreadID，同时兼容 ChatSessionID
- decompose 创建的 synthetic session 可写入 thread topic/status/issue_id
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web/... -run "TestWSSubscribeThreadAlias|TestWSChatSessionSubscriptionRoutesChatEventsBySessionID|TestACPChatAssistantReplyUsesThreadID|TestDecomposeCreatesThreadBackedSession" -count=1`
Expected: FAIL，thread alias、assistant request 字段或 decompose thread 元数据尚不存在。

**Step 3: Minimal implementation**
```text
改造运行时元数据：
- ws: 增加 subscribe_thread / unsubscribe_thread 别名
- ws message / event payload 新增 thread_id（session_id 继续保留）
- ChatAssistantRequest 增加 ThreadID，ChatSessionID 作为兼容别名
- ACP assistant 内部优先按 thread_id 池化和记录
- decompose synthetic session 创建时填充 thread topic/status/issue_id
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web/... -run "TestWSSubscribeThreadAlias|TestWSChatSessionSubscriptionRoutesChatEventsBySessionID|TestACPChatAssistantReplyUsesThreadID|TestDecomposeCreatesThreadBackedSession" -count=1`
Expected: PASS，runtime 路径已识别 Thread，但旧 session 路径继续有效。

**Step 5: Commit**
```bash
git add internal/web/ws.go internal/web/ws_test.go internal/web/chat_assistant_claude.go internal/web/chat_assistant_acp.go internal/web/chat_assistant_acp_test.go internal/web/handlers_decompose.go internal/web/handlers_decompose_test.go
git commit -m "feat(web): propagate thread metadata through runtime paths"
```

## Test Strategy

- handler 测试覆盖 thread 读 API 和 chat 兼容路径。
- WS 测试同时验证旧订阅名和新订阅名。
- assistant / decompose 测试只验证 thread 元数据接入，不重测 ACP provider 细节。

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| `/chat` 响应结构变化导致前端回退 | 只增字段，不删旧字段；旧测试必须保留 |
| WS 新订阅名影响老客户端 | 新增 alias，不移除 `subscribe_chat_session` |
| assistant pooling 键切换导致会话复用异常 | `ThreadID` 和 `ChatSessionID` 在同一时期并存，测试覆盖 load/reuse/cancel |
| decompose synthetic session 没有 topic/status，线程列表变脏 | 在创建路径上立即补默认 topic/status，禁止留空脏数据 |

## Wave E2E / Smoke Cases

| Case | Entry Data | Command | Expected Signal |
|---|---|---|---|
| Thread 读取 API | thread-backed store fixture | `go test ./internal/web/... -run "TestListThreads|TestGetThread|TestListThreadEvents" -count=1` | REST 可列出 thread 字段 |
| Chat 兼容路径 | 现有 chat fixture | `go test ./internal/web/... -run "TestCreateChatSessionThenGetChatSession|TestListChatSessions" -count=1` | `/chat` 不回退且返回 thread_id |
| WS / assistant / decompose | ws fixture + assistant stub + decompose fixture | `go test ./internal/web/... -run "TestWSSubscribeThreadAlias|TestACPChatAssistantReplyUsesThreadID|TestDecomposeCreatesThreadBackedSession" -count=1` | runtime 路径识别 thread 语义 |

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 新增 thread read API，且 `/chat` 保持兼容。
  - [ ] WS、assistant、decompose 都能识别并透出 `thread_id` 元数据。
- Wave-specific verification:
  - [ ] `go test ./internal/web/... -run "TestListThreads|TestGetThread|TestListThreadEvents|TestCreateChatSessionThenGetChatSession|TestListChatSessions|TestWSSubscribeThreadAlias|TestACPChatAssistantReplyUsesThreadID|TestDecomposeCreatesThreadBackedSession" -count=1` 通过。
  - [ ] `go test ./internal/web/... -count=1` 通过。
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

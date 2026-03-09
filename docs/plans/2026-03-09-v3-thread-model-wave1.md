# Wave 1 Plan: Thread Domain And Store Foundation

## Wave Goal

建立 `Thread` 领域模型、Store 接口和 SQLite thread-backed 存储，同时保证现有 `ChatSession` CRUD 与历史数据读取不回退。

## Tasks

### Task W1-T1: 新增 Thread 核心领域模型

**Files:**
- Create: `internal/core/thread.go`
- Test: `internal/core/thread_test.go`

**Depends on:** `[]`

**Step 1: Write failing test**
```text
新增 Thread 领域单测，至少覆盖：
- Validate() 要求 project_id 非空
- status 仅允许 open / crystallized / closed
- participants 去重并保持稳定顺序
- issue_id 可空
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/core/... -run "TestThread|TestThreadStatus" -count=1`
Expected: 编译失败或测试失败，提示 `Thread` / `ThreadStatus` / `Validate` 未定义。

**Step 3: Minimal implementation**
```text
新增 `core.Thread`：
- 字段：ID、ProjectID、IssueID、Participants、Topic、Status、Messages、CreatedAt、UpdatedAt
- 新增 `ThreadStatus`
- 新增 `NewThreadID()` 和 `Validate()`
消息仍复用现有 `core.ChatMessage`，本 wave 不引入独立 Message 表。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/core/... -run "TestThread|TestThreadStatus" -count=1`
Expected: PASS，Thread 模型和校验规则稳定。

**Step 5: Commit**
```bash
git add internal/core/thread.go internal/core/thread_test.go
git commit -m "feat(core): add Thread domain model"
```

### Task W1-T2: 扩展 Store 接口与 SQLite thread-backed 持久化

**Files:**
- Modify: `internal/core/store.go`
- Modify: `internal/plugins/store-sqlite/migrations.go`
- Modify: `internal/plugins/store-sqlite/store.go`
- Test: `internal/plugins/store-sqlite/thread_store_test.go`

**Depends on:** `[W1-T1]`

**Step 1: Write failing test**
```text
新增 SQLite 集成测试，至少覆盖：
- CreateThread / GetThread / UpdateThread / ListThreads
- 旧 chat_sessions 行在未回填时也能被读取为默认 open Thread
- participants/topic/status/issue_id 可正确持久化
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/plugins/store-sqlite/... -run "TestThreadCRUD|TestLegacyChatSessionReadableAsThread" -count=1`
Expected: 编译失败或测试失败，提示 Thread Store 方法或 schema 字段不存在。

**Step 3: Minimal implementation**
```text
扩展 Store：
- CreateThread / GetThread / UpdateThread / ListThreads

SQLite 迁移策略：
- 继续复用 `chat_sessions` 表
- 增加 `issue_id`、`topic`、`thread_status`、`participants_json`
- 对旧数据使用安全默认值，不做破坏性 rename

实现要求：
- Thread 成为主读取模型
- 旧数据未显式回填时，默认 `status=open`、`participants=[]`
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/plugins/store-sqlite/... -run "TestThreadCRUD|TestLegacyChatSessionReadableAsThread" -count=1`
Expected: PASS，Thread CRUD 与默认回填行为稳定。

**Step 5: Commit**
```bash
git add internal/core/store.go internal/plugins/store-sqlite/migrations.go internal/plugins/store-sqlite/store.go internal/plugins/store-sqlite/thread_store_test.go
git commit -m "feat(store): add thread-backed sqlite persistence"
```

### Task W1-T3: 保持 ChatSession 兼容壳可用

**Files:**
- Create: `internal/core/chat_thread_compat.go`
- Test: `internal/core/chat_thread_compat_test.go`
- Modify: `internal/plugins/store-sqlite/store.go`
- Test: `internal/plugins/store-sqlite/secretary_store_test.go`

**Depends on:** `[W1-T2]`

**Step 1: Write failing test**
```text
新增兼容层测试，覆盖：
- ChatSession <-> Thread 转换保留 id/project/messages/agent_session_id/agent_name
- 既有 CreateChatSession / GetChatSession / ListChatSessions / UpdateChatSession 行为不变
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/core/... ./internal/plugins/store-sqlite/... -run "TestChatSessionThreadCompat|TestChatSessionCRUD" -count=1`
Expected: 失败，提示兼容转换 helper 未定义或旧 CRUD 被新字段破坏。

**Step 3: Minimal implementation**
```text
新增 ChatSession 兼容 helper：
- ThreadFromChatSession
- ChatSessionFromThread

要求：
- ChatSession 方法继续可用
- 内部可以路由到 thread-backed 存储
- 不改变现有 JSON 结构和测试夹具输入
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/core/... ./internal/plugins/store-sqlite/... -run "TestChatSessionThreadCompat|TestChatSessionCRUD|TestChatRunEventsCRUD" -count=1`
Expected: PASS，Thread 新能力不影响旧 ChatSession CRUD 与 chat_run_events。

**Step 5: Commit**
```bash
git add internal/core/chat_thread_compat.go internal/core/chat_thread_compat_test.go internal/plugins/store-sqlite/store.go internal/plugins/store-sqlite/secretary_store_test.go
git commit -m "feat(chat): keep ChatSession compatibility on thread store"
```

## Test Strategy

- `internal/core` 做模型和兼容层单测。
- `internal/plugins/store-sqlite` 做真实 SQLite CRUD 和迁移默认值测试。
- 明确验证“新 Thread 能力存在”和“旧 ChatSession 路径不回退”两条线。

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| 新增 schema 字段破坏旧 chat CRUD | 采用加法迁移；旧读取默认值兜底；旧测试必须全跑 |
| Thread 与 ChatSession 双模型语义漂移 | 明确 Thread 为主模型，ChatSession 只做兼容转换 |
| 一上来引入独立 message 表导致返工过大 | 本 wave 明确不做 message 表，只复用现有消息数组 |

## Wave E2E / Smoke Cases

| Case | Entry Data | Command | Expected Signal |
|---|---|---|---|
| Thread CRUD | SQLite 临时库 | `go test ./internal/plugins/store-sqlite/... -run "TestThreadCRUD" -count=1` | 可创建、更新、列出 Thread |
| 旧 chat 数据兼容 | 手工插入 legacy chat row | `go test ./internal/plugins/store-sqlite/... -run "TestLegacyChatSessionReadableAsThread|TestChatSessionCRUD" -count=1` | 旧行仍可被读取和更新 |

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] `Thread` 成为稳定领域模型并具备 SQLite CRUD。
  - [ ] 旧 `ChatSession` CRUD、chat_run_events 和历史数据读取不回退。
- Wave-specific verification:
  - [ ] `go test ./internal/core/... -run "TestThread|TestThreadStatus|TestChatSessionThreadCompat" -count=1` 通过。
  - [ ] `go test ./internal/plugins/store-sqlite/... -run "TestThreadCRUD|TestLegacyChatSessionReadableAsThread|TestChatSessionCRUD|TestChatRunEventsCRUD" -count=1` 通过。
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

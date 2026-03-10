# Wave 3 Plan: Consumer Contract Integration

## Wave Goal

把 `thread_id` 扩展到 MCP / 外部消费契约，并完成 thread / session 双语义并存下的集成回归与边界类型校准。

## Tasks

### Task W3-T1: 让 MCP 工具接受并返回 `thread_id` 兼容别名

**Files:**
- Modify: `internal/mcpserver/deps.go`
- Modify: `internal/mcpserver/tools_query.go`
- Modify: `internal/mcpserver/tools_issues.go`
- Test: `internal/mcpserver/tools_query_test.go`
- Test: `internal/mcpserver/tools_query_scenarios_test.go`

**Depends on:** `[W2-T3]`

**Step 1: Write failing test**
```text
新增 MCP 工具测试，覆盖：
- query_issues 接受 thread_id 作为 session_id 别名过滤
- create_issue 接受 thread_id 并归一化到现有 issue/session 绑定路径
- 结果在不删除 session_id 的前提下补出 thread_id
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run "TestQueryIssuesByThreadID|TestCreateIssueWithThreadIDAlias|TestQueryIssuesScenarios" -count=1`
Expected: FAIL，thread_id 输入输出语义尚不存在。

**Step 3: Minimal implementation**
```text
对 MCP 层只做加法兼容：
- 输入：thread_id 作为 session_id 别名
- 输出：同时给 session_id 和 thread_id
- 不在本 task 里删除旧字段，也不重写 issue 领域逻辑
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestQueryIssuesByThreadID|TestCreateIssueWithThreadIDAlias|TestQueryIssuesScenarios" -count=1`
Expected: PASS，MCP 消费面已能逐步切到 thread_id。

**Step 5: Commit**
```bash
git add internal/mcpserver/deps.go internal/mcpserver/tools_query.go internal/mcpserver/tools_issues.go internal/mcpserver/tools_query_test.go internal/mcpserver/tools_query_scenarios_test.go
git commit -m "feat(mcp): add thread id compatibility aliases"
```

### Task W3-T2: 增加 thread/session 共存的集成回归

**Files:**
- Create: `internal/web/e2e_thread_http_test.go`
- Modify: `internal/mcpserver/e2e_subprocess_test.go`

**Depends on:** `[W3-T1]`

**Step 1: Write failing test**
```text
新增集成 smoke，覆盖：
- 创建 thread-backed chat
- 通过 /threads 读取到 topic/status/issue_id
- 通过旧 /chat 路径继续读取
- MCP query 通过 thread_id 过滤 Issue
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/web/... ./internal/mcpserver/... -run "TestE2E_ThreadAndChatCompatibility|TestE2E_Subprocess_ThreadQueryAlias" -count=1`
Expected: FAIL，thread/session 双语义协同路径尚未全部打通。

**Step 3: Minimal implementation**
```text
只补集成缺口，不重做架构：
- 固化 thread + chat 双路径同时可读
- 固化 MCP subprocess 场景里的 thread_id alias
- 明确 thread 是主语义、chat/session 是兼容语义
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/web/... ./internal/mcpserver/... -run "TestE2E_ThreadAndChatCompatibility|TestE2E_Subprocess_ThreadQueryAlias" -count=1`
Expected: PASS，thread/session 共存链路稳定。

**Step 5: Commit**
```bash
git add internal/web/e2e_thread_http_test.go internal/mcpserver/e2e_subprocess_test.go
git commit -m "test(integration): cover thread and chat compatibility paths"
```

### Task W3-T3: 更新边界类型与消费契约

**Files:**
- Modify: `web/src/types/api.ts`
- Modify: `web/src/lib/apiClient.ts`

**Depends on:** `[W3-T2]`

**Step 1: Write failing test**
```text
让前端类型检查先失败，覆盖：
- 新 thread REST 响应
- chat 响应中的 thread_id / topic / thread_status
- 不破坏旧 session_id 读取
```

**Step 2: Run to confirm failure**
Run: `npm --prefix web run typecheck`
Expected: FAIL，前端 API 类型与后端新增字段不一致。

**Step 3: Minimal implementation**
```text
只做边界对齐：
- API type 增加 thread 字段
- apiClient 增加 thread read endpoints 或兼容类型
- 不做新的页面或 UI 重构
```

**Step 4: Run tests to confirm pass**
Run: `npm --prefix web run typecheck`
Expected: PASS，前端边界类型与后端 thread 契约一致。

**Step 5: Commit**
```bash
git add web/src/types/api.ts web/src/lib/apiClient.ts
git commit -m "feat(web): align api types with thread contract"
```

## Test Strategy

- MCP 工具层验证 thread_id 输入输出兼容，不重做业务逻辑。
- 集成测试验证 `/threads`、`/chat` 和 MCP query 三条链路同时成立。
- 前端只做类型和构建校验，不在本 wave 新做页面功能。

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| 外部消费面误以为 session_id 已废弃 | 明确采用双字段并存策略，thread_id 只做加法 |
| MCP alias 逻辑在不同工具间不一致 | 用同一归一化 helper 处理 session_id / thread_id |
| 前端类型未同步导致构建断裂 | Wave 3 强制执行 `typecheck` 和 `frontend-build` |

## Wave E2E / Smoke Cases

| Case | Entry Data | Command | Expected Signal |
|---|---|---|---|
| MCP thread alias | MCP subprocess fixture | `go test ./internal/mcpserver/... -run "TestQueryIssuesByThreadID|TestE2E_Subprocess_ThreadQueryAlias" -count=1` | thread_id 可被接受并正确过滤 |
| thread/chat 双路径集成 | HTTP server + SQLite fixture | `go test ./internal/web/... -run "TestE2E_ThreadAndChatCompatibility" -count=1` | `/threads` 与 `/chat` 同时可读 |
| 前端边界类型 | web API types | `npm --prefix web run typecheck` | 类型对齐完成 |

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] `thread_id` 已进入 MCP / web 消费契约，但 `session_id` 仍保留兼容。
  - [ ] thread/chat 双路径集成回归稳定。
- Wave-specific verification:
  - [ ] `go test ./internal/mcpserver/... -run "TestQueryIssuesByThreadID|TestCreateIssueWithThreadIDAlias|TestQueryIssuesScenarios|TestE2E_Subprocess_ThreadQueryAlias" -count=1` 通过。
  - [ ] `go test ./internal/web/... -run "TestE2E_ThreadAndChatCompatibility" -count=1` 通过。
  - [ ] `npm --prefix web run typecheck` 通过。
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1` 通过。
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- 计划完成；仅当 `executing-wave-plans` 给出 `Go` 或满足 `Conditional Go` 时，才允许开启下一份围绕 `acceptance_criteria`、Issue participants 或 thread-aware prompt / memory 的新计划。

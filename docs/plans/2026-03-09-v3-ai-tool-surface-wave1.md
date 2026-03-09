# Wave 1 Plan: Tool Envelope + Capability Catalog

## Wave Goal

为现有 MCP 工具建立统一返回壳、统一错误语义和能力发现目录，形成 Wave 2 外部能力 façade 接入的稳定基线。

## Tasks

### Task W1-T1: 提取统一 ToolResult 返回壳

**Files:**
- Create: `internal/mcpserver/tool_result.go`
- Modify: `internal/mcpserver/tools_query.go`
- Test: `internal/mcpserver/tool_result_test.go`

**Depends on:** `[]`

**Step 1: Write failing test**
```text
新增 ToolResult 单元测试，断言成功结果包含 ok/message/data/references/next_actions/warnings，失败结果包含 ok=false 和可读 message。
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run TestToolResult -v`
Expected: 编译失败或测试失败，提示 `ToolResult` / helper 未定义。

**Step 3: Minimal implementation**
```text
在 mcpserver 中新增统一结果类型和 helper：
- successResult(data, opts...)
- failureResult(message, opts...)
保留与当前 jsonResult/errorResult 的兼容过渡层，避免一次性改坏所有工具。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run TestToolResult -v`
Expected: PASS，统一结果 helper 可序列化并能生成 MCP 返回。

**Step 5: Commit**
```bash
git add internal/mcpserver/tool_result.go internal/mcpserver/tools_query.go internal/mcpserver/tool_result_test.go
git commit -m "feat(mcp): add unified tool result envelope"
```

### Task W1-T2: 迁移高频工具到统一返回壳

**Files:**
- Modify: `internal/mcpserver/tools_query.go`
- Modify: `internal/mcpserver/tools_issues.go`
- Modify: `internal/mcpserver/tools_runs.go`
- Modify: `internal/mcpserver/tools_submit.go`
- Test: `internal/mcpserver/tools_query_test.go`
- Test: `internal/mcpserver/tools_submit_test.go`

**Depends on:** `[W1-T1]`

**Step 1: Write failing test**
```text
给 query_issue_detail / submit_task / apply_issue_action / apply_run_action 添加断言：
- 返回体最外层有 ok/data/message
- detail 类工具附带 next_actions 或 references
- 错误路径不再只返回裸字符串
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run "TestQueryIssueDetail|TestSubmitTask|TestApplyRunAction" -v`
Expected: 现有断言不满足，测试失败。

**Step 3: Minimal implementation**
```text
逐个迁移高频工具到统一返回壳：
- 只读 detail 工具返回 references + next_actions
- 写工具返回 status_changed / references
- 保持原有 data 主体结构不被破坏，减少调用方迁移成本
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestQueryIssueDetail|TestSubmitTask|TestApplyRunAction" -v`
Expected: PASS，现有场景测试继续通过。

**Step 5: Commit**
```bash
git add internal/mcpserver/tools_query.go internal/mcpserver/tools_issues.go internal/mcpserver/tools_runs.go internal/mcpserver/tools_submit.go internal/mcpserver/tools_query_test.go internal/mcpserver/tools_submit_test.go
git commit -m "feat(mcp): normalize high-frequency tool outputs"
```

### Task W1-T3: 新增工具能力目录

**Files:**
- Create: `internal/mcpserver/tools_catalog.go`
- Modify: `internal/mcpserver/server.go`
- Test: `internal/mcpserver/e2e_subprocess_test.go`

**Depends on:** `[W1-T2]`

**Step 1: Write failing test**
```text
新增 catalog 工具测试，断言：
- 返回工具分组（query/task/orchestration/conversation/review/diagnostic/dev）
- 每个工具包含 name、group、summary、requires_write、status
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run TestE2E_Subprocess_ListTools -v`
Expected: 失败，`query_tool_catalog` 未注册。

**Step 3: Minimal implementation**
```text
增加只读工具 `query_tool_catalog`：
- 从服务注册表构造静态目录
- 先覆盖现有工具，并为 Wave 2 预留 orchestration / conversation / review / diagnostics 分组
- 不做动态反射，避免过度设计
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestE2E_Subprocess_ListTools|TestQueryToolCatalog" -v`
Expected: PASS，catalog 工具可列出已注册工具。

**Step 5: Commit**
```bash
git add internal/mcpserver/tools_catalog.go internal/mcpserver/server.go internal/mcpserver/e2e_subprocess_test.go
git commit -m "feat(mcp): add tool catalog discovery endpoint"
```

## Test Strategy

- 单元优先：`ToolResult` helper、catalog 构造逻辑单测先行。
- Handler 场景：覆盖 query/detail、submit_task、run_action 三条高频路径。
- E2E 冒烟：子进程 MCP server 列工具与调用 catalog。

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| 现有工具返回结构变化影响旧调用方 | 保持 `data` 主体兼容，旧字段位置不动，只加外层壳 |
| 一次迁移范围过大 | 先迁移高频工具，为 Wave 2 的 façade 接入预留统一壳和 catalog |
| Catalog 与真实注册表漂移 | 使用同一注册路径维护 catalog 条目，不复制多份字符串 |

## Wave E2E / Smoke Cases

| Case | Entry Data | Command | Expected Signal |
|---|---|---|---|
| Tool catalog 可发现能力 | 启动 MCP 子进程测试夹具 | `go test ./internal/mcpserver/... -run TestQueryToolCatalog -v` | 返回按 group 聚合的目录 |
| 高频工具统一返回壳 | 现有 issue/run 测试夹具 | `go test ./internal/mcpserver/... -run "TestQueryIssueDetail|TestSubmitTask|TestApplyRunAction" -v` | 所有返回含 `ok` 与 `data` |

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] `query_*`、`submit_task`、`apply_issue_action`、`apply_run_action` 全部切到统一返回壳。
  - [ ] `query_tool_catalog` 可稳定描述当前工具面，并为 Wave 2 的 façade 分组预留目录项。
- Wave-specific verification:
  - [ ] `go test ./internal/mcpserver/... -run "TestToolResult|TestQueryIssueDetail|TestSubmitTask|TestApplyRunAction|TestQueryToolCatalog" -v` 通过。
  - [ ] `go test ./internal/mcpserver/... -count=1` 通过。
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

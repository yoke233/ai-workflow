# Wave 2 Plan: External Capability Façade Integration

## Wave Goal

在 Wave 1 完成后，把外部已稳定能力统一接入到 MCP façade 中，覆盖三类动作：
- DAG 拆解与子任务创建
- 会话读取与任务结晶
- review / gate / blocker / failure diagnostics

本 wave 的原则是“谁 ready 就先接谁”，不因为某一个上游计划未完成而阻塞整个 wave；但每个 task 都必须先验证上游依赖已经 merged 或 API-stable。

## Blocked By Active Plans

- [issue-dag-decompose-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-issue-dag-decompose-plan.md) 必须先达到 merged 或至少接口冻结状态，才能做 DAG façades。
- [taskstep-event-sourcing-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-taskstep-event-sourcing-plan.md) 必须先达到 merged 或至少接口冻结状态，才能做 review / gate façades。
- [watchdog-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-watchdog-plan.md) 必须先达到 merged 或至少接口冻结状态，才能做 diagnostics façades。
- [decision-versioning-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-decision-versioning-plan.md) 必须先达到 merged 或至少接口冻结状态，才能把 decision summary 接入 review / diagnostics façades。

若某个依赖未满足，只阻塞对应 task，不阻塞整个 wave 中其他已 ready 的 task。

## Tasks

### Task W2-T1: 验证各外部能力的适配前提

**Files:**
- Modify: `docs/plans/2026-03-09-v3-ai-tool-surface-wave2.md`
- Test: `internal/mcpserver/tools_external_facade_test.go`

**Depends on:** `[]`

**Step 1: Write failing test**
```text
新增一组最小 precondition 测试，分别断言：
- DAG façade 依赖是否已注入
- conversation/crystallize façade 依赖是否已注入
- review/gate/diagnostics façade 依赖是否已注入
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run TestExternalFacadePreconditions -v`
Expected: FAIL，提示对应 façade 工具或依赖未注入。

**Step 3: Minimal implementation**
```text
在测试和计划备注中明确适配边界：
- 消费上游已提供的能力
- 不在本 wave 里新增 schema、Store 方法、状态机或业务内核
- 对未 ready 的依赖返回明确 blocked 信息
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run TestExternalFacadePreconditions -v`
Expected: PASS，测试夹具可明确上游已满足或明确阻塞原因。

**Step 5: Commit**
```bash
git add docs/plans/2026-03-09-v3-ai-tool-surface-wave2.md internal/mcpserver/tools_external_facade_test.go
git commit -m "test(mcp): add external facade precondition coverage"
```

### Task W2-T2: 新增 DAG façades

**Files:**
- Modify: `internal/mcpserver/deps.go`
- Create: `internal/mcpserver/tools_orchestration.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/tools_catalog.go`
- Test: `internal/mcpserver/tools_external_facade_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
新增工具测试，断言：
- `decompose_task` 只调用上游已存在的 decompose 能力
- `create_child_tasks` 只调用上游已存在的 child creation 能力
- 返回统一 envelope
- 返回中包含 proposal / child graph 摘要、references、next_actions
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run "TestDecomposeTaskFacade|TestCreateChildTasksFacade|TestQueryToolCatalog" -v`
Expected: FAIL，DAG façade 工具未注册或依赖接口未定义。

**Step 3: Minimal implementation**
```text
扩展 mcpserver 的依赖注入接口，只引入 façade 必需的最小方法。
handler 只做参数校验、统一返回壳组装和 references 拼装，不实现新的 DAG 业务逻辑。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestDecomposeTaskFacade|TestCreateChildTasksFacade|TestQueryToolCatalog" -v`
Expected: PASS，工具层不包含新的 DAG 内核逻辑，catalog 中出现 orchestration 条目。

**Step 5: Commit**
```bash
git add internal/mcpserver/deps.go internal/mcpserver/tools_orchestration.go internal/mcpserver/server.go internal/mcpserver/tools_catalog.go internal/mcpserver/tools_external_facade_test.go
git commit -m "feat(mcp): add DAG facade tools"
```

### Task W2-T3: 新增 conversation / crystallize façades

**Files:**
- Create: `internal/mcpserver/tools_conversation.go`
- Modify: `internal/mcpserver/deps.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/tools_catalog.go`
- Modify: `internal/mcpserver/tools_submit.go`
- Test: `internal/mcpserver/tools_external_facade_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
新增工具测试，断言：
- `list_thread_messages` 基于现有 ChatSession 输出 thread-like 摘要
- `crystallize_thread_to_task` 复用现有 submit_task / create_issue 路径
- catalog 中出现 conversation 分组条目
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run "TestListThreadMessagesFacade|TestCrystallizeThreadToTaskFacade|TestQueryToolCatalog" -v`
Expected: FAIL，conversation/crystallize 工具或 catalog 信息不存在。

**Step 3: Minimal implementation**
```text
增加 conversation façade：
- list_thread_messages
- crystallize_thread_to_task
- 不新增 Thread / Message 存储模型
- 只在返回里做 thread-like alias 与 references 封装
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestListThreadMessagesFacade|TestCrystallizeThreadToTaskFacade|TestQueryToolCatalog" -v`
Expected: PASS，conversation/crystallize façade 和能力目录同步可见。

**Step 5: Commit**
```bash
git add internal/mcpserver/tools_conversation.go internal/mcpserver/deps.go internal/mcpserver/server.go internal/mcpserver/tools_catalog.go internal/mcpserver/tools_submit.go internal/mcpserver/tools_external_facade_test.go
git commit -m "feat(mcp): add conversation and crystallize facade tools"
```

### Task W2-T4: 新增 review / gate / diagnostics façades

**Files:**
- Create: `internal/mcpserver/tools_review.go`
- Create: `internal/mcpserver/tools_diagnostics.go`
- Modify: `internal/mcpserver/deps.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/tools_catalog.go`
- Test: `internal/mcpserver/tools_external_facade_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
新增 façade 测试，断言：
- `request_review` 只触发上游 review 入口
- `query_review_gate_status` 汇总 review/taskstep/decision 摘要
- `list_blockers` / `diagnose_run_failure` 汇总 watchdog/run/decision 摘要
- catalog 中出现 review 和 diagnostics 分组条目
```

**Step 2: Run to confirm failure**
Run: `go test ./internal/mcpserver/... -run "TestRequestReviewFacade|TestQueryReviewGateStatusFacade|TestListBlockersFacade|TestDiagnoseRunFailureFacade|TestQueryToolCatalog" -v`
Expected: FAIL，review/diagnostics façade 工具或 catalog 信息不存在。

**Step 3: Minimal implementation**
```text
增加 façade：
- request_review
- query_review_gate_status
- list_blockers
- diagnose_run_failure
不引入 review_id、新 sign-off 状态机或新的 diagnostics 领域模型。
```

**Step 4: Run tests to confirm pass**
Run: `go test ./internal/mcpserver/... -run "TestRequestReviewFacade|TestQueryReviewGateStatusFacade|TestListBlockersFacade|TestDiagnoseRunFailureFacade|TestQueryToolCatalog" -v`
Expected: PASS，review / diagnostics façade 和能力目录同步可见。

**Step 5: Commit**
```bash
git add internal/mcpserver/tools_review.go internal/mcpserver/tools_diagnostics.go internal/mcpserver/deps.go internal/mcpserver/server.go internal/mcpserver/tools_catalog.go internal/mcpserver/tools_external_facade_test.go
git commit -m "feat(mcp): add review and diagnostics facade tools"
```

## Test Strategy

- 只测试 façade 契约，不重复测试外部计划的内核。
- 用 stub / fake 上游服务证明 handler 只做适配，不做重新编排。
- 通过 catalog 测试确保工具目录与注册同步。
- 对未 ready 的上游能力，验证 blocked 路径而不是强行实现替代逻辑。

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| 某一个上游计划未 ready 导致整 wave 停滞 | task 级 blocked，不阻塞其他已 ready façade |
| façade 反向侵入上游实现 | 依赖注入接口只暴露最小方法，不引用上游私有结构 |
| 与 Wave 1 返回壳风格漂移 | 强制复用 Wave 1 的统一结果 helper |

## Wave E2E / Smoke Cases

| Case | Entry Data | Command | Expected Signal |
|---|---|---|---|
| DAG façades | 上游 decompose / child creation stub | `go test ./internal/mcpserver/... -run "TestDecomposeTaskFacade|TestCreateChildTasksFacade" -v` | 返回 proposal 和 child graph 摘要 |
| conversation / crystallize façades | ChatSession fixture | `go test ./internal/mcpserver/... -run "TestListThreadMessagesFacade|TestCrystallizeThreadToTaskFacade" -v` | 返回 thread-like 摘要并成功结晶 |
| review / diagnostics façades | 上游 review / watchdog / decision stub | `go test ./internal/mcpserver/... -run "TestRequestReviewFacade|TestQueryReviewGateStatusFacade|TestListBlockersFacade|TestDiagnoseRunFailureFacade" -v` | 返回 façade 摘要与触发结果 |

## Wave Exit Gate
- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 外部能力接入统一收敛在一个 façade wave 中，不拆成新的底层实现计划。
  - [ ] 所有已 ready 的 façade 均进入 tool catalog，并复用 Wave 1 的统一返回壳。
- Wave-specific verification:
  - [ ] `go test ./internal/mcpserver/... -run "TestExternalFacadePreconditions|TestDecomposeTaskFacade|TestCreateChildTasksFacade|TestListThreadMessagesFacade|TestCrystallizeThreadToTaskFacade|TestRequestReviewFacade|TestQueryReviewGateStatusFacade|TestListBlockersFacade|TestDiagnoseRunFailureFacade|TestQueryToolCatalog" -v` 通过。
  - [ ] `go test ./internal/mcpserver/... -count=1` 通过。
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\backend-all.ps1` 通过。

## Next Wave Entry Condition
- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).

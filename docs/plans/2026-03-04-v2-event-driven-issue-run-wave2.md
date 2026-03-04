# Wave 2 Plan: 事件驱动后端主链路重构

## Wave Goal

删除 DAG/task-plan runtime 路径，落地 `Issue -> WorkflowRun` 的事件驱动执行链路，并引入 profile 驱动的审核编排（normal/strict/fast_release）。

## Tasks

### Task W2-T1: 删除 DAG 调度实现并替换为 Profile Queue Scheduler

**Files:**
- Delete: `internal/secretary/dag.go`
- Delete: `internal/secretary/dag_test.go`
- Modify: `internal/secretary/scheduler.go`
- Modify: `internal/secretary/scheduler_test.go`
- Modify: `internal/secretary/manager.go`

**Depends on:** `[W1-T3]`

**Step 1: Write failing test**
```text
新增调度测试：
1) Issue approved 后直接进入 profile queue
2) 按 profile 规则触发 run 创建
3) 不再使用 depends_on / in_degree / topo 校验路径
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/secretary -run "Scheduler|Queue|Profile|Run" -count=1 -timeout 60s`  
Expected: 旧 DAG 断言与新行为冲突导致失败。

**Step 3: Minimal implementation**
```text
移除 DAG Build/Validate/Reduce 调用与结构体。
调度改为 profile queue + run lifecycle listener。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/secretary -run "Scheduler|Queue|Profile|Run" -count=1 -timeout 60s`  
Expected: 新调度测试通过。

**Step 5: Commit**
```bash
git add internal/secretary/scheduler.go internal/secretary/scheduler_test.go internal/secretary/manager.go
git rm internal/secretary/dag.go internal/secretary/dag_test.go
git commit -m "refactor(secretary): remove dag runtime and switch to profile queue scheduler"
```

### Task W2-T2: 移除 task/plan API，统一 issue/profile/run API

**Files:**
- Modify: `internal/web/server.go`
- Modify: `internal/web/handlers_plan.go`
- Delete: `internal/web/handlers_task.go`
- Modify: `internal/web/handlers_pipeline.go`
- Modify: `cmd/ai-flow/commands.go`
- Test: `internal/web/handlers_plan_test.go`
- Test: `internal/web/handlers_pipeline_test.go`

**Depends on:** `[W2-T1]`

**Step 1: Write failing test**
```text
新增接口测试：
1) /api/v2/issues/* 可用
2) /api/v2/workflow-profiles/* 可用
3) /api/v2/runs/* 可用
4) /api/v1/projects/:pid/plans* 不再暴露
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/web -run "IssueAPI|ProfileAPI|RunAPI|NoPlansRoute" -count=1 -timeout 60s`  
Expected: 路由不匹配失败。

**Step 3: Minimal implementation**
```text
删除 plans/task 路由注册与兼容 alias。
新增 issue/profile/run 路由处理器并调整 handler 参数命名。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/web -run "IssueAPI|ProfileAPI|RunAPI|NoPlansRoute" -count=1 -timeout 60s`  
Expected: 接口测试通过。

**Step 5: Commit**
```bash
git add internal/web/server.go internal/web/handlers_plan.go internal/web/handlers_pipeline.go cmd/ai-flow/commands.go internal/web/handlers_plan_test.go internal/web/handlers_pipeline_test.go
git rm internal/web/handlers_task.go
git commit -m "refactor(web): replace plan-task routes with issue-profile-run v2 api"
```

### Task W2-T3: 引入 Profile 驱动审核编排（normal/strict/fast_release）

**Files:**
- Modify: `internal/secretary/review.go`
- Modify: `internal/secretary/default_review_panel.go`
- Modify: `internal/plugins/review-ai-panel/review_ai_panel.go`
- Test: `internal/secretary/review_test.go`
- Test: `internal/plugins/review-ai-panel/review_ai_panel_test.go`

**Depends on:** `[W2-T2]`

**Step 1: Write failing test**
```text
新增测试覆盖：
1) normal: 1 reviewer + 1 aggregator
2) strict: 3 reviewers 并行 + aggregator
3) fast_release: 轻量审核 + 快速结论
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/secretary ./internal/plugins/review-ai-panel -run "Profile|Strict|FastRelease|ReviewOrchestrator" -count=1 -timeout 60s`  
Expected: 角色数与流程断言失败。

**Step 3: Minimal implementation**
```text
将审核编排参数从固定 reviewer 集改为 WorkflowProfile 驱动。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/secretary ./internal/plugins/review-ai-panel -run "Profile|Strict|FastRelease|ReviewOrchestrator" -count=1 -timeout 60s`  
Expected: 测试通过。

**Step 5: Commit**
```bash
git add internal/secretary/review.go internal/secretary/default_review_panel.go internal/plugins/review-ai-panel/review_ai_panel.go internal/secretary/review_test.go internal/plugins/review-ai-panel/review_ai_panel_test.go
git commit -m "feat(review): make review orchestration profile-driven"
```

## Risks And Mitigations

- 风险: 删除 DAG 后可能丢失依赖执行能力。  
  缓解: V2 明确无依赖运行时语义；若后续需要，作为 Profile Gate 扩展而非恢复 DAG 模块。
- 风险: API 大规模断代导致编译与测试噪音高。  
  缓解: Wave2 内一次性断代，避免“新旧双栈”。

## Wave Exit Gate

- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] `internal/secretary/dag*.go` 完全移除。
  - [ ] 后端不再注册 plans/task 路由。
  - [ ] review 编排由 profile 驱动。
- Wave-specific verification:
  - [ ] `go test ./internal/secretary ./internal/web ./internal/plugins/review-ai-panel -count=1 -timeout 60s`
  - [ ] `go test ./cmd/ai-flow -count=1 -timeout 60s`
- Boundary-change verification (if triggered):
  - [ ] `go test ./... -count=1 -timeout 60s`

## Next Wave Entry Condition

- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).


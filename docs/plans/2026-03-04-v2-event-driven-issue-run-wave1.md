# Wave 1 Plan: V2 领域模型与命名收敛

## Wave Goal

建立 V2 的核心语义边界：`Issue + WorkflowProfile + WorkflowRun + TeamLeader`，并完成配置层命名切换，确保后续删除 DAG/task-plan 时有稳定基线。

## Tasks

### Task W1-T1: 定义 V2 领域模型（Profile/Run）

**Files:**
- Create: `internal/core/workflow_profile.go`
- Create: `internal/core/workflow_run.go`
- Modify: `internal/core/issue.go`
- Test: `internal/core/workflow_profile_test.go`
- Test: `internal/core/workflow_run_test.go`

**Depends on:** `[]`

**Step 1: Write failing test**
```text
新增测试覆盖：
1) WorkflowProfile 的规则校验（normal/strict/fast_release + sla_minutes）
2) WorkflowRun 状态流转校验（created/running/waiting_review/done/failed/timeout）
3) Issue 去除 task/plan 语义后仍可校验通过
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/core -run "WorkflowProfile|WorkflowRun|Issue" -count=1 -timeout 60s`  
Expected: 新增测试失败，报缺少类型或校验逻辑。

**Step 3: Minimal implementation**
```text
实现 WorkflowProfile/WorkflowRun 结构与 Validate 方法。
Issue 仅保留 V2 必要字段，不再表达 task/plan 子实体语义。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/core -run "WorkflowProfile|WorkflowRun|Issue" -count=1 -timeout 60s`  
Expected: 所有相关测试通过。

**Step 5: Commit**
```bash
git add internal/core/workflow_profile.go internal/core/workflow_run.go internal/core/issue.go internal/core/workflow_profile_test.go internal/core/workflow_run_test.go
git commit -m "feat(core): introduce v2 workflow profile and run domain models"
```

### Task W1-T2: 配置与角色绑定改名到 Team Leader

**Files:**
- Modify: `internal/config/types.go`
- Modify: `internal/config/defaults.go`
- Modify: `configs/defaults.yaml`
- Test: `internal/config/config_test.go`

**Depends on:** `[W1-T1]`

**Step 1: Write failing test**
```text
新增配置测试：
1) role_bindings.team_leader 可解析
2) 默认角色为 team_leader
3) 旧 secretary 字段不再接受（断代版本）
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/config -run "TeamLeader|RoleBinding|Defaults" -count=1 -timeout 60s`  
Expected: 解析失败或断言不通过。

**Step 3: Minimal implementation**
```text
重命名配置结构和默认值，移除 secretary 命名入口。
确保 role resolver 只走 team_leader。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/config -run "TeamLeader|RoleBinding|Defaults" -count=1 -timeout 60s`  
Expected: 测试全部通过。

**Step 5: Commit**
```bash
git add internal/config/types.go internal/config/defaults.go configs/defaults.yaml internal/config/config_test.go
git commit -m "refactor(config): rename secretary bindings to team_leader"
```

### Task W1-T3: 核心事件命名切换（team_leader/run）

**Files:**
- Modify: `internal/core/events.go`
- Test: `internal/core/events_test.go`

**Depends on:** `[W1-T2]`

**Step 1: Write failing test**
```text
新增事件枚举测试，断言：
1) 不再暴露 secretary_* 事件名
2) 暴露 team_leader_* 与 run_* 事件名
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/core -run "Event.*TeamLeader|Event.*Run" -count=1 -timeout 60s`  
Expected: 事件名不匹配导致失败。

**Step 3: Minimal implementation**
```text
替换事件常量与 Issue scoped 判定逻辑。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/core -run "Event.*TeamLeader|Event.*Run" -count=1 -timeout 60s`  
Expected: 测试通过。

**Step 5: Commit**
```bash
git add internal/core/events.go internal/core/events_test.go
git commit -m "refactor(core): rename secretary events to team_leader and run events"
```

## Risks And Mitigations

- 风险: 事件/配置改名影响面大，编译错误分散。  
  缓解: 先在 Wave1 只改域模型与配置，保持路由逻辑未切，集中修编译。
- 风险: 断代移除旧字段导致测试批量失败。  
  缓解: 同步删除旧断言，不保留兼容测试。

## Wave Exit Gate

- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] Core 层出现 WorkflowProfile/WorkflowRun 且通过校验测试。
  - [ ] 配置层默认角色与绑定切换到 team_leader。
  - [ ] 事件常量不再出现 secretary_*。
- Wave-specific verification:
  - [ ] `go test ./internal/core ./internal/config -count=1 -timeout 60s`
  - [ ] `go test ./... -run "TeamLeader|WorkflowRun|WorkflowProfile" -count=1 -timeout 60s`
- Boundary-change verification (if triggered):
  - [ ] `go test ./internal/web ./cmd/ai-flow -count=1 -timeout 60s`

## Next Wave Entry Condition

- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).


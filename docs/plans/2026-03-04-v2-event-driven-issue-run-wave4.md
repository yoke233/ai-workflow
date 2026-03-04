# Wave 4 Plan: 清理、回归与发布基线

## Wave Goal

清除所有 V1 兼容残留（task/plan/secretary 命名与迁移代码），固化 V2 测试与发布基线，形成可切换新版本的最小闭环。

## Tasks

### Task W4-T1: 删除旧迁移与兼容壳代码

**Files:**
- Modify: `internal/plugins/store-sqlite/migrations.go`
- Modify: `internal/plugins/store-sqlite/migrations_test.go`
- Modify: `internal/plugins/store-sqlite/cutover_test.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/handlers_plan.go`

**Depends on:** `[W3-T3]`

**Step 1: Write failing test**
```text
新增断言：
1) 新建数据库不再创建 task_plans/task_items 相关兼容逻辑
2) server 不再暴露 PlanManager deprecated 字段
3) 路由无 /plans 入口
```

**Step 2: Run to confirm failure**  
Run: `go test ./internal/plugins/store-sqlite ./internal/web -run "Migration|Cutover|NoPlans|NoPlanManager" -count=1 -timeout 60s`  
Expected: 旧逻辑仍存在导致失败。

**Step 3: Minimal implementation**
```text
删除 task_plans/task_items 迁移路径和兼容测试。
移除 PlanManager deprecated 字段与相关分支。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./internal/plugins/store-sqlite ./internal/web -run "Migration|Cutover|NoPlans|NoPlanManager" -count=1 -timeout 60s`  
Expected: 通过。

**Step 5: Commit**
```bash
git add internal/plugins/store-sqlite/migrations.go internal/plugins/store-sqlite/migrations_test.go internal/plugins/store-sqlite/cutover_test.go internal/web/server.go internal/web/handlers_plan.go
git commit -m "chore(v2): remove legacy task-plan migration and compatibility shells"
```

### Task W4-T2: Team Leader 命名收尾（包名/模板名/提示词）

**Files:**
- Move: `internal/secretary` -> `internal/teamleader`
- Move: `configs/prompts/secretary.tmpl` -> `configs/prompts/team_leader.tmpl`
- Modify: `cmd/ai-flow/commands.go`
- Modify: `internal/plugins/factory/factory.go`
- Modify: `internal/web/*`（引用路径）

**Depends on:** `[W4-T1]`

**Step 1: Write failing test**
```text
新增编译与引用测试：
1) 不再 import internal/secretary
2) 默认 prompt_template 为 team_leader
```

**Step 2: Run to confirm failure**  
Run: `go test ./... -run "TeamLeader|NoSecretaryImport|PromptTemplate" -count=1 -timeout 60s`  
Expected: import/template 引用失败。

**Step 3: Minimal implementation**
```text
执行包目录重命名与 import 批量替换。
更新配置默认模板与 role 默认值。
```

**Step 4: Run tests to confirm pass**  
Run: `go test ./... -run "TeamLeader|NoSecretaryImport|PromptTemplate" -count=1 -timeout 60s`  
Expected: 通过。

**Step 5: Commit**
```bash
git add internal/teamleader configs/prompts/team_leader.tmpl cmd/ai-flow/commands.go internal/plugins/factory/factory.go internal/web
git rm -r internal/secretary
git rm configs/prompts/secretary.tmpl
git commit -m "refactor(v2): rename secretary module to teamleader"
```

### Task W4-T3: V2 全链路 smoke 与发布基线

**Files:**
- Modify: `scripts/test/p3-integration.ps1`
- Create: `scripts/test/v2-smoke.ps1`
- Modify: `README.md`
- Modify: `docs/spec/*`（更新到 V2 语义）

**Depends on:** `[W4-T2]`

**Step 1: Write failing test**
```text
定义 smoke 预期：
1) 创建 issue
2) 绑定 profile
3) 触发 run
4) 收到 review/run 事件
```

**Step 2: Run to confirm failure**  
Run: `pwsh -NoProfile -File .\scripts\test\v2-smoke.ps1`  
Expected: 脚本失败（新脚本或新路径尚未完整）。

**Step 3: Minimal implementation**
```text
补全 v2-smoke.ps1 和文档命令，确保可一键验证。
```

**Step 4: Run tests to confirm pass**  
Run: `pwsh -NoProfile -File .\scripts\test\v2-smoke.ps1`  
Expected: smoke 通过，打印 run 完成信号。

**Step 5: Commit**
```bash
git add scripts/test/p3-integration.ps1 scripts/test/v2-smoke.ps1 README.md docs/spec
git commit -m "test(v2): add end-to-end smoke baseline for issue-profile-run flow"
```

## Risks And Mitigations

- 风险: 包重命名引发大规模 import 破裂。  
  缓解: 用 `rg` 全量替换并通过 `go test ./...` 兜底。
- 风险: 文档与代码语义不一致。  
  缓解: Wave4 强制同步 README/spec，禁止遗留旧术语。

## Wave Exit Gate

- Execute mandatory gate sequence via `executing-wave-plans`.
- Wave-specific acceptance:
  - [ ] 代码库不再包含 task_plans/task_items 迁移逻辑。
  - [ ] 代码库不再包含 secretary 包路径与模板命名。
  - [ ] v2-smoke 脚本可独立通过。
- Wave-specific verification:
  - [ ] `go test ./... -count=1 -timeout 60s`
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`
  - [ ] `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1`
  - [ ] `pwsh -NoProfile -File .\scripts\test\v2-smoke.ps1`
- Boundary-change verification (if triggered):
  - [ ] `pwsh -NoProfile -File .\scripts\test\p3-integration.ps1`

## Next Wave Entry Condition

- Governed by `executing-wave-plans` verdict rule (`Go` / satisfied `Conditional Go` only).


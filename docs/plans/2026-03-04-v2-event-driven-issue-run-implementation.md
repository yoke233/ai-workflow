# V2 Event-Driven Issue/Run Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** 交付一个全新 V2：事件驱动编排，业务域只保留 Issue，删除 task/plan 与 DAG 运行时调度，Pipeline 重构为 Run（执行实例），Secretary 重命名为 Team Leader。

**Architecture:** V2 采用 `Issue + WorkflowProfile + WorkflowRun` 三实体模型。Issue 作为唯一需求单，Profile 定义流程与审核规则，Run 承载执行状态机。Event Bus 负责解耦触发与推进，运行状态不再由 DAG 图驱动，而由 profile 规则驱动。

**Tech Stack:** Go 1.22+, chi, SQLite (modernc), React + TypeScript + Vite, ACP client.

---

## Context And Scope

### In Scope
- 删除 runtime DAG 调度能力与依赖驱动执行。
- 删除 task/plan 业务实体与对应 API/前端类型。
- 统一命名：Secretary -> Team Leader，Pipeline -> WorkflowRun（保留技术实现层可渐进重命名，但对外语义必须完成）。
- 引入 WorkflowProfile（normal/strict/fast_release）并驱动审核与执行监听。
- 新增 60 分钟 SLA 超时能力（profile 配置）。

### Out Of Scope
- 历史数据兼容与迁移保留。
- 与旧版本并行兼容路由、兼容字段、兼容事件名。
- GitHub 集成深度扩展（仅确保编译通过，不扩展新能力）。

### Assumptions
- V2 为断代版本，允许删除大量代码与测试。
- 现网继续跑旧版本，V2 不承担线上平滑迁移责任。
- 可以接受接口与前端路由完全变更。

## Dependency DAG Overview And Critical Path

实现依赖图（非运行时 DAG）：

1. 领域模型收敛（Issue/Profile/Run）  
2. 后端路由与调度重构（删除 task/plan 与 DAG）  
3. 前端数据模型与页面切换  
4. 清理与回归

Critical Path:
- Wave1 -> Wave2 -> Wave3 -> Wave4（强依赖串行）。

## Wave Map

1. Wave 1: V2 领域模型与命名收敛  
depends_on: []
2. Wave 2: 事件驱动后端主链路重构（删除 DAG/task-plan runtime）  
depends_on: [Wave 1]
3. Wave 3: 前端统一切换到 Issue/Profile/Run  
depends_on: [Wave 2]
4. Wave 4: 清理、压测与发布基线  
depends_on: [Wave 3]

## Global Quality Gates

- Functional Gate (F):
  - V2 仅暴露 Issue/Profile/Run 语义 API。
  - Team Leader 语义替换 Secretary（对外接口与文案）。
- Quality Gate (Q):
  - 所有新增/改动代码均有对应测试。
  - 删除旧能力时同步删除旧测试与死代码。
- Compatibility Gate (C):
  - 明确不做兼容，必须移除旧入口避免双栈漂移。
- Delivery Gate (D):
  - 提供一套最小 e2e/smoke 命令可重复执行。

## Wave Plan Links

- [Wave 1 Plan](./2026-03-04-v2-event-driven-issue-run-wave1.md)
- [Wave 2 Plan](./2026-03-04-v2-event-driven-issue-run-wave2.md)
- [Wave 3 Plan](./2026-03-04-v2-event-driven-issue-run-wave3.md)
- [Wave 4 Plan](./2026-03-04-v2-event-driven-issue-run-wave4.md)

## Full Regression Command Set

```powershell
go test ./internal/core ./internal/config -count=1 -timeout 60s
go test ./internal/secretary ./internal/engine ./internal/web ./internal/plugins/... -count=1 -timeout 60s
go test ./cmd/ai-flow -count=1 -timeout 60s
pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1
pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
```

## Test Policy

- 每个任务遵循 TDD：先写失败测试，再最小实现，再验证通过。
- 每个 Wave 至少包含一条端到端 smoke 测试。
- 触发边界变更（接口、事件、状态机）时补 integration/contract 测试。
- 单测统一 `-timeout 60s`，避免卡死。

## Workspace Strategy

- 使用一个计划级分支/工作区贯穿 Wave1..Wave4。
- 每个 wave 内允许临时并行分支，但 wave 退出前必须合并回计划主分支。
- 下一 wave 只从计划主分支起步。


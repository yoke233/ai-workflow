# V3 AI Tool Surface Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** 为当前 `ai-workflow` 构建一套贴近 v3 设计、便于 AI 稳定调用的 MCP 工具面，使 agent 能顺畅完成查询、任务结晶、拆解编排、会话收口、审阅总结与故障诊断。

**Architecture:** 本计划只做“工具面整合”，不争夺底层领域模型与持久化实现权。`internal/mcpserver` 负责统一返回壳、能力目录和 façade 工具；`internal/teamleader`、`internal/core`、`internal/plugins/store-sqlite` 的深层实现以其他正在推进的计划为准，本计划只在这些能力稳定后做最小适配。

**Tech Stack:** Go / modelcontextprotocol go-sdk / SQLite / TeamLeader orchestration / ACP client / PowerShell test scripts

---

## Context And Scope

### In Scope

- 为现有 MCP 工具统一返回结构，补充 `next_actions`、`blocking_reasons`、`references` 等 AI 友好字段。
- 建立工具能力目录，减少 AI 对工具名和参数的猜测。
- 为已合并或接口已稳定的外部能力增加 MCP façade：
  - DAG 拆解与子任务创建 façade
  - 会话读取与任务结晶 façade
  - review / gate / blocker / failure diagnostics façade

### Out Of Scope

- 不在本计划内实现完整的 Decision 版本化；仅消费其已合并输出。
- 不在本计划内实现新的 `Thread` / `Message` 存储模型；优先复用现有 `ChatSession` / `ChatRunEvent` 能力做 façade。
- 不在本计划内扩展 `core.Store`、SQLite migrations 或底层消息总线，除非上游计划已合并且只差极薄适配层。
- 不在本计划内实现新的 sign-off 状态机或独立 review aggregate。
- 不在本计划内建设新的 Web UI；仅在前后端契约或现有类型受影响时更新必要类型与回归。

### Current Codebase Constraints And Existing Capabilities

- 已有只读查询工具：`query_projects`、`query_project_detail`、`query_issues`、`query_issue_detail`、`query_runs`、`query_run_detail`、`query_run_events`、`query_project_stats`。
- 已有写工具：`create_issue`、`update_issue`、`apply_issue_action`、`add_issue_attachment`、`submit_task`、`apply_run_action`。
- `mcpserver.Deps` 当前仅暴露 `Store`、`IssueManager`、`RunExecutor`，因此任何新 façade 都必须先确认依赖注入点是否足够，避免在本计划里顺手新造业务内核。
- Store 目前已有 `ChatSession` 和 `ChatRunEvent`，足以承载“会话读取 + 结晶”型 façade；但还没有 v3.1 的独立 `Thread / Message` 模型。
- TaskStep、DAG 拆解、Watchdog、Decision 版本化均已有独立计划在推进。

### Source Specs / Docs

- [docs/v3/01-v3-main-architecture.zh-CN.md](/D:/project/ai-workflow/docs/v3/01-v3-main-architecture.zh-CN.md)
- [docs/v3/02-v3-post-iterations.zh-CN.md](/D:/project/ai-workflow/docs/v3/02-v3-post-iterations.zh-CN.md)
- [docs/v3/05-v3.1-thread-execution-context.zh-CN.md](/D:/project/ai-workflow/docs/v3/05-v3.1-thread-execution-context.zh-CN.md)
- [docs/v3/06-design-reflection-prompt-quality.zh-CN.md](/D:/project/ai-workflow/docs/v3/06-design-reflection-prompt-quality.zh-CN.md)
- [docs/v3/v3-evolution-roadmap.md](/D:/project/ai-workflow/docs/v3/v3-evolution-roadmap.md)

### Active Plan Dependencies

| External Plan | Status Assumption | This Plan Consumes |
|---|---|---|
| [issue-dag-decompose-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-issue-dag-decompose-plan.md) | merged or API-stable before Wave 2 tasks | Proposal / DAG creation / dependency-aware child creation |
| [taskstep-event-sourcing-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-taskstep-event-sourcing-plan.md) | merged or API-stable before Wave 2 review/gate tasks | issue lifecycle facts / gate status summary |
| [watchdog-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-watchdog-plan.md) | merged or API-stable before Wave 2 diagnostics tasks | stuck-run / queue-stale / sem-leak diagnostic signals |
| [decision-versioning-plan.md](/D:/project/ai-workflow/docs/plans/2026-03-09-decision-versioning-plan.md) | merged or API-stable before Wave 2 review/diagnostics tasks | review/decompose/stage decision summaries |

### V3 To Current Code Mapping

| v3 概念 | 当前代码落点 | 本计划策略 |
|---|---|---|
| Task | `core.Issue` | 不改名，只做 façade 映射 |
| TaskStep | `core.TaskStep` | 只消费已合并事实层，不重做事件模型 |
| Artifact | `Run.Artifacts` + `IssueAttachment` + Review 输出 | 只在工具返回里统一引用 |
| Thread | 当前近似物 `ChatSession` | 先做会话 façade，不新建存储模型 |
| Gate / Review | `teamleader/review*.go` + TaskStep / Decision 输出 | 只做工具面摘要与触发入口 |
| Crystallize | `submit_task` + 会话摘要 | 新增快捷工具，内部复用现有任务提交能力 |

## Goal And Non-Goals

### Goal

- 让零上下文执行者能实现一套 AI 可直接调用的工具面，并且不与正在进行的底层计划冲突。

### Non-Goals

- 不做全量 v3 一次性落地。
- 不为“模型完备性”增加暂时不会提升调用质量的复杂结构。
- 不在其他计划尚未收敛时抢先修改其领域边界。

## Assumptions

- 以 1 名主执行工程师为默认执行容量；如需并行，只允许 wave 内临时 lane，不允许跨 wave 分叉长期存在。
- 默认采用一个计划级分支/工作树：`plan/v3-ai-tool-surface`，所有 wave 复用同一工作树。
- 若某 wave 需要临时 lane worktree，则必须在该 wave Exit Gate 前合回 `plan/v3-ai-tool-surface`。
- Wave 1 可立即开工；Wave 2 的具体任务按外部计划成熟度逐项解锁。

## Dependency DAG Overview

```text
Wave 1: Tool Envelope + Capability Catalog
  └─ unlocks stable outputs for all later tools

Wave 2: External Capability Façade Integration
  └─ depends_on: Wave 1
  └─ blocked until required upstream plans are merged or API-stable
  └─ integrates DAG + conversation/crystallize + review/gate/diagnostics facades
```

**Critical Path:** `Wave 1 -> Wave 2`

Wave 1 是统一返回壳和工具目录基线。Wave 2 统一承接所有外部能力 façade，对上游计划的依赖通过 task 级 blocked 条件控制，而不是继续拆成多个独立 wave。

## Wave Map

| Wave | Goal | depends_on | Output |
|---|---|---|---|
| Wave 1 | 统一 MCP 工具返回壳与能力目录 | `[]` | [Wave 1 Plan](./2026-03-09-v3-ai-tool-surface-wave1.md) |
| Wave 2 | 为外部已稳定能力增加 façade：DAG、conversation/crystallize、review/gate/diagnostics | `[Wave 1, active plan dependencies as needed]` | [Wave 2 Plan](./2026-03-09-v3-ai-tool-surface-wave2.md) |

## Global Quality Gates

| Gate | Definition |
|---|---|
| F | 工具行为与 v3 目标动作一一对应，避免“需要 AI 自己猜下一步” |
| Q | 单元优先 TDD；新增工具必须有 handler 测试和至少一个场景测试 |
| C | 不侵入未完成的底层计划；已有 MCP 工具调用兼容，不得回退 |
| D | 文档、工具描述、返回字段、示例输入输出保持同步 |

## Per-Wave Output Files

- [docs/plans/2026-03-09-v3-ai-tool-surface-wave1.md](/D:/project/ai-workflow/docs/plans/2026-03-09-v3-ai-tool-surface-wave1.md)
- [docs/plans/2026-03-09-v3-ai-tool-surface-wave2.md](/D:/project/ai-workflow/docs/plans/2026-03-09-v3-ai-tool-surface-wave2.md)

## Full Regression Command Set

### Mandatory Backend Regression

```powershell
pwsh -NoProfile -File .\scripts\test\backend-all.ps1
```

### Mandatory Targeted Packages During Development

```powershell
go test ./internal/mcpserver/... -count=1
```

按 task 触发补充：

```powershell
go test ./internal/teamleader/... -count=1
go test ./internal/core/... -count=1
go test ./internal/plugins/store-sqlite/... -count=1
```

仅当当前 task 真的接触这些包时才执行。

### Boundary-Triggered Contract / Integration Regression

```powershell
pwsh -NoProfile -File .\scripts\test\p3-integration.ps1
```

触发条件：
- 修改了 MCP 工具注册、跨模块 orchestration 适配或现有集成链路
- 消费了外部计划新增的 API / store 字段 / 状态派生逻辑

### Frontend Boundary Regression

```powershell
npm --prefix web run typecheck
pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
```

触发条件：
- Web API 类型、返回壳、或工具/事件展示契约发生变化

## Test Policy

- 所有任务按 unit-first TDD 执行：先写失败测试，再写最小实现，再跑通过。
- 每个 wave 至少包含一个 MCP handler / façade smoke 场景，证明该 wave 的主能力可被工具面消费。
- `ai-tool-surface` 计划不负责为外部计划补底层测试；只负责新增 façade 的适配测试和契约测试。
- 若某个 façade task 需要引用外部计划的新接口，先加“依赖已满足”的验证步骤，再进入实现。

## Workspace Strategy

- 默认创建一个计划级分支/工作树：`plan/v3-ai-tool-surface`
- Wave 1 到 Wave 2 全部在同一工作树推进
- 若单个 wave 内需要并行 lane：
  - lane 命名：`plan/v3-ai-tool-surface-<wave>-<lane>`
  - lane 只能在当前 wave 内存在
  - Wave Exit Gate 前必须合并回 `plan/v3-ai-tool-surface`
  - 下一 wave 必须从合并后的计划级工作树继续

## Recommended Implementation Sequence

1. 先统一现有 MCP 工具返回壳和工具目录。
2. 在同一个集成 wave 中，按上游计划成熟度逐项接入：
   - DAG façade
   - conversation / crystallize façade
   - review / gate / diagnostics façade
3. 任何未 ready 的上游能力都留在 deferred hooks，不阻塞已 ready 的 façade 接入。

## Deferred Hooks

- 真正独立的 `Thread / Message` 存储模型
- 新的 sign-off 状态机
- 独立的 review aggregate / `review_id`
- 新的 diagnostics 领域模型或巡检规则

这些能力若未来需要，必须另起计划，不回灌到本计划的主执行波次。

## Acceptance Criteria

- AI 能通过稳定工具面完成以下动作，而无需拼装底层 CRUD：
  - 看清当前状态
  - 提交或结晶出任务
  - 调用已落地的 DAG 拆解与子任务创建能力
  - 读取会话上下文并发起结晶
  - 查询 review / gate / blocker / failure 摘要
- 新旧工具在返回结构和错误表达上保持一致。
- 本计划不引入与正在进行中的底层计划冲突的 schema / store / state-machine 变更。

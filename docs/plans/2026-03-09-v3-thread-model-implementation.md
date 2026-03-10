# V3 Thread Model Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** 在不破坏现有 `/chat`、`session_id`、ACP 会话复用和 DAG 结晶链路的前提下，把 `Thread` 引入为一等会话容器，并让当前后端从 `ChatSession` 过渡到 thread-backed conversation model。

**Architecture:** 本计划采用“领域先行、存储加法、接口兼容”的迁移方式。`core.Thread` 成为主领域模型，但底层优先复用现有 `chat_sessions` / `chat_run_events` 存储并做加法迁移，避免一次性改坏聊天、WebSocket、ACP session pooling 和 decompose 流程。`ChatSession` 在本计划内保留为兼容壳，`thread_id` 逐步进入 web / MCP / runtime 契约。

**Tech Stack:** Go / Chi / SQLite / Gorilla WebSocket / ACP client / TeamLeader / PowerShell test scripts

---

## Context And Scope

### In Scope

- 引入 `core.Thread` 一等模型，覆盖：
  - `id`
  - `project_id`
  - `issue_id`
  - `participants`
  - `topic`
  - `status`
  - `messages`
- 扩展 `core.Store` 与 SQLite，实现 Thread CRUD。
- 采用加法迁移方式扩展现有 `chat_sessions` 表，不做破坏性表替换。
- 保持现有 `ChatSession` CRUD、`/chat` REST、WebSocket chat 订阅、ACP assistant pooling 和 decompose 流程可继续工作。
- 新增 thread read API，并让现有 chat / MCP 消费面逐步接受 `thread_id` 语义。
- 为后续 `acceptance_criteria`、`participants`、thread-aware prompt / memory 做稳定数据边界。

### Out Of Scope

- 不在本计划内把 `Issue` 全量改名为 `Task`。
- 不在本计划内引入独立 `Message` 表，也不实现 `reply_to_msg_id` 对话链。
- 不在本计划内实现 thread broadcast bus、at-least-once 消息语义、`idempotency_key`。
- 不在本计划内补 `acceptance_criteria`、Issue/Task 级 `participants`、`tags`、`Schedule`、`Memory Compact`。
- 不在本计划内重做 provider-native `agent_session_id` 生命周期。

### Current Codebase Constraints And Existing Capabilities

- 当前会话核心模型为 [chat.go](/D:/project/ai-workflow/internal/core/chat.go)，`ChatSession` 把消息直接存进 `chat_sessions.messages` JSON。
- 运行时增量事件落在 `chat_run_events`，键仍是 `chat_session_id`，对应实现见 [store.go](/D:/project/ai-workflow/internal/plugins/store-sqlite/store.go)。
- Web chat 路由全部以 `/chat/{sessionID}` 语义暴露，核心实现见 [handlers_chat.go](/D:/project/ai-workflow/internal/web/handlers_chat.go)。
- WebSocket 当前只支持 `subscribe_chat_session`，缓存也按 `session_id` 路由，见 [ws.go](/D:/project/ai-workflow/internal/web/ws.go)。
- ACP chat assistant 的会话池、取消、命令查询都按 `ChatSessionID` 工作，见 [chat_assistant_claude.go](/D:/project/ai-workflow/internal/web/chat_assistant_claude.go) 和 [chat_assistant_acp.go](/D:/project/ai-workflow/internal/web/chat_assistant_acp.go)。
- DAG decompose 会创建 synthetic `ChatSession` 作为讨论容器，见 [handlers_decompose.go](/D:/project/ai-workflow/internal/web/handlers_decompose.go)。
- MCP 工具和 Issue 模型仍使用 `session_id` 过滤和绑定，见 [deps.go](/D:/project/ai-workflow/internal/mcpserver/deps.go)、[tools_query.go](/D:/project/ai-workflow/internal/mcpserver/tools_query.go)、[tools_issues.go](/D:/project/ai-workflow/internal/mcpserver/tools_issues.go)。

### Source Specs / Docs

- [docs/v3/05-v3.1-thread-execution-context.zh-CN.md](/D:/project/ai-workflow/docs/v3/05-v3.1-thread-execution-context.zh-CN.md)
- [docs/v3/06-design-reflection-prompt-quality.zh-CN.md](/D:/project/ai-workflow/docs/v3/06-design-reflection-prompt-quality.zh-CN.md)
- [docs/v3/07-openviking-thread-storage.zh-CN.md](/D:/project/ai-workflow/docs/v3/07-openviking-thread-storage.zh-CN.md)
- [docs/v3/v3-evolution-roadmap.md](/D:/project/ai-workflow/docs/v3/v3-evolution-roadmap.md)

### Goal And Non-Goals

#### Goal

- 先把 `Thread` 作为稳定容器落到后端主链路，再逐步把现有 `ChatSession` 消费面迁移到 thread-backed 语义。

#### Non-Goals

- 不追求本计划内一次性做完所有 v3.1 会话语义。
- 不为了模型完备性立即引入 `reply_to_msg_id`、独立 Message 表和总线广播。
- 不抢做后续 `acceptance_criteria`、Issue participants、Artifact 统一交付等其他结构性任务。

### Assumptions

- “下一步任务”按 [v3-evolution-roadmap.md](/D:/project/ai-workflow/docs/v3/v3-evolution-roadmap.md) 当前优先级，默认就是 `T1 Thread 会话容器`。
- 默认执行容量为 1 名工程师；如 wave 内需要并行 lane，只允许临时 lane worktree，不允许跨 wave 长期分叉。
- 默认使用一个计划级分支 / worktree：`plan/v3-thread-model`。
- 为降低返工风险，本计划优先在现有 `chat_sessions` 表上做加法迁移，而不是在第一步直接替换为全新 `threads` / `thread_messages` 表。

## Migration Strategy

1. `Thread` 先成为领域主模型，`ChatSession` 先成为兼容模型。
2. 底层存储优先扩展 `chat_sessions` 表，新增 thread 语义字段，旧数据通过默认值可直接读出。
3. `session_id` 在本计划内保留为兼容别名，不立即从 HTTP / MCP / WebSocket 契约中删除。
4. 新增 `thread_id` 后，所有对外契约采用“新增而非替换”的方式推进。
5. 只有在 Wave 3 全部通过后，后续计划才允许考虑把 `ChatSession` 彻底下沉为兼容层。

## Dependency DAG Overview

```text
Wave 1: Thread Domain + Store Foundation
  └─ unlocks all thread-backed runtime and API work

Wave 2: Web / Runtime Adoption
  └─ depends_on: Wave 1
  └─ unlocks thread-backed REST / WS / ACP compatibility

Wave 3: Consumer Contract Integration
  └─ depends_on: Wave 2
  └─ unlocks MCP aliases, external consumer compatibility, and final regression
```

**Critical Path:** `Wave 1 -> Wave 2 -> Wave 3`

Wave 1 是唯一的 schema / domain gate。只有 Thread store 基座稳定后，才能安全迁移 web、assistant、MCP 消费面。

## Wave Map

| Wave | Goal | depends_on | Output |
|---|---|---|---|
| Wave 1 | 建立 Thread 领域模型、Store 接口和 SQLite thread-backed 存储，同时保住 ChatSession 兼容性 | `[]` | [Wave 1 Plan](./2026-03-09-v3-thread-model-wave1.md) |
| Wave 2 | 让 web/chat/ws/assistant/decompose 进入 thread-backed 语义，同时保留 `/chat` 兼容壳 | `[Wave 1]` | [Wave 2 Plan](./2026-03-09-v3-thread-model-wave2.md) |
| Wave 3 | 把 `thread_id` 扩展到 MCP / 外部消费契约，并完成集成回归与边界类型校准 | `[Wave 2]` | [Wave 3 Plan](./2026-03-09-v3-thread-model-wave3.md) |

## Global Quality Gates

| Gate | Definition |
|---|---|
| F | Thread 成为一等容器，但现有 `/chat`、`session_id`、ACP pooling、decompose 流程不回退 |
| Q | 单元优先 TDD；每个 wave 至少包含 domain/store/web 或 consumer smoke 覆盖 |
| C | 迁移只做加法兼容；不允许破坏旧数据读取和现有 API 主路径 |
| D | 新增 `thread_id` 的所有对外契约必须同步更新测试、返回结构与计划文档 |

## Per-Wave Output Files

- [docs/plans/2026-03-09-v3-thread-model-wave1.md](/D:/project/ai-workflow/docs/plans/2026-03-09-v3-thread-model-wave1.md)
- [docs/plans/2026-03-09-v3-thread-model-wave2.md](/D:/project/ai-workflow/docs/plans/2026-03-09-v3-thread-model-wave2.md)
- [docs/plans/2026-03-09-v3-thread-model-wave3.md](/D:/project/ai-workflow/docs/plans/2026-03-09-v3-thread-model-wave3.md)

## Full Regression Command Set

### Mandatory Backend Regression

```powershell
pwsh -NoProfile -File .\scripts\test\backend-all.ps1
```

### Mandatory Targeted Packages During Development

```powershell
go test ./internal/core/... -count=1
go test ./internal/plugins/store-sqlite/... -count=1
go test ./internal/web/... -count=1
go test ./internal/mcpserver/... -count=1
```

只在对应 wave / task 触及时执行：

```powershell
go test ./internal/teamleader/... -count=1
```

### Boundary-Triggered Contract / Integration Regression

```powershell
go test ./internal/web/... -run "TestCreateChatSessionThenGetChatSession|TestListChatSessions|TestWSChatSessionSubscriptionRoutesChatEventsBySessionID|TestWSChatSessionSubscriptionBroadcastsToMultipleSubscribers" -count=1
go test ./internal/mcpserver/... -run "TestQueryIssues|TestQueryIssuesScenarios|TestE2E_Subprocess_ListTools" -count=1
```

触发条件：

- 修改了 chat / thread REST 契约
- 修改了 WebSocket session/thread 订阅语义
- 修改了 MCP 中的 `session_id` / `thread_id` 输入输出

### Frontend Boundary Regression

```powershell
npm --prefix web run typecheck
pwsh -NoProfile -File .\scripts\test\frontend-build.ps1
```

触发条件：

- 后端 JSON 响应新增 `thread_id`、`topic`、`thread_status` 等字段，并同步更新了 `web/src/types/api.ts` 或 `web/src/lib/apiClient.ts`

## Test Policy

- 所有任务按 unit-first TDD 执行：先写失败测试，再写最小实现，再确认通过。
- Wave 1 必须有 domain + store 级测试，验证新旧模型可以共存。
- Wave 2 必须有 web handler + WebSocket / ACP runtime smoke，验证线程语义进入运行时但不破坏旧路径。
- Wave 3 必须有 consumer / contract smoke，验证 `thread_id` 扩展没有破坏 MCP 和外部查询面。
- 任何改动如果触碰到历史兼容层，必须同时跑旧路径测试和新路径测试。

## Workspace Strategy

- 默认创建一个计划级分支 / worktree：`plan/v3-thread-model`
- Wave 1 到 Wave 3 全部复用同一 worktree
- 如单个 wave 内需要临时 lane：
  - lane 命名：`plan/v3-thread-model-<wave>-<lane>`
  - lane 只能在当前 wave 内存在
  - Wave Exit Gate 前必须合回 `plan/v3-thread-model`
  - 下一 wave 只能从合并后的计划级 worktree 开始

## Recommended Implementation Sequence

1. 先把 `Thread` 放进 core 和 store，并确认旧 `ChatSession` CRUD 不炸。
2. 再让 web/chat/ws/assistant/decompose 变成 thread-backed，但仍然保留 `/chat` 与 `session_id` 兼容壳。
3. 最后把 `thread_id` 扩展到 MCP / 外部查询面，并做契约回归。

## Acceptance Criteria

- 后端拥有一等 `Thread` 模型与持久化能力。
- 现有 `ChatSession` 路径继续可用，不要求前端或上游调用方立刻切换。
- Web / WebSocket / ACP runtime 可以识别 thread 语义。
- MCP / 外部消费面可以逐步接受 `thread_id`，且旧 `session_id` 不被立即破坏。
- 本计划完成后，可以安全开启下一份计划去做 `acceptance_criteria`、Issue participants 或 thread-aware prompt / memory。

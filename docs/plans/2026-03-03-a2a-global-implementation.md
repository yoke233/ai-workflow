# A2A 全局接入（前后端）Implementation Plan

> **For Agent:** REQUIRED SUB-SKILL: Use executing-wave-plans to implement this plan wave-by-wave.

**Goal:** 在不影响现有 `/api/v1` 主流程行为的前提下，把 `secretary` 能力通过官方 `a2a-go` 协议栈对外暴露，并在前端提供最小可用 A2A 交互。

**Architecture:** 不新增独立 A2A 领域模型，直接复用 `github.com/a2aproject/a2a-go` 的协议类型与调用约束；后端通过 `internal/web` 的 A2A JSON-RPC 入口桥接 `secretary` 现有能力；`secretary` 仅返回现有领域语义，`a2a-go` 协议对象组装收敛在 `internal/web` 适配层；A2A 路径默认关闭并强制 Bearer Token 鉴权；前端仅在 Chat 场景接入 `a2aClient`，不做全局协议工厂抽象。

**Tech Stack:** Go + chi + `a2a-go` + existing secretary manager/store + React/TypeScript/Vite + PowerShell 测试脚本。

---

## 1. Context And Scope

### In Scope
- 直接使用 `a2a-go` 完成 A2A 协议接入，不再自建同构模型层。
- 后端提供 `/.well-known/agent-card.json` 与 `/api/v1/a2a` JSON-RPC 入口。
- 最小方法集：`message/send`、`tasks/get`、`tasks/cancel`，并支持流式方法（`message/stream`）的基础能力。
- A2A 路径增加 Bearer Token 校验（默认严格模式：`a2a.enabled=true` 时必须显式配置 `a2a.token`，不隐式复用 `server.auth_token`）。
- `A2A_ENABLED=false` 时，`/api/v1/a2a` 与 `/.well-known/agent-card.json` 都必须硬 404，不允许落入 SPA fallback。
- 前端 Chat 页面支持 A2A 调用、状态展示与取消。
- 前端 A2A 认证复用现有 `VITE_API_TOKEN` 注入机制，不新增独立 A2A token 环境变量。
- 复用并参考已跑通的 `cmd/a2a-smoke` 作为协议烟测基线。

### Out Of Scope
- 不做 dark/shadow/live 三态治理。
- 不做 SLO guard、放量脚本、上线决策包。
- 不做全站双协议工厂与按项目动态路由策略。

## 2. ACP 与 A2A 边界

- ACP：当前系统内部 Agent 运行时协议（stdio/本地进程链路），保持不变。
- A2A：新增对外 Agent-to-Agent 协议入口（HTTP JSON-RPC）。
- 关系：ACP 与 A2A 共存；A2A 负责 northbound 对外，不替代 ACP 内部链路。
- 错误模型：A2A 对外遵循 JSON-RPC 错误码；ACP 维持现有内部语义。

## 3. Dependency DAG And Critical Path

### DAG（Wave 级）
- `Wave1 -> Wave2 -> Wave3`
- 关键依赖：
  - Wave2 依赖 Wave1 的鉴权、路由与协议接线。
  - Wave3 依赖 Wave2 的可用后端接口。

### Critical Path
1. 复用官方 `a2a-go` + A2A Token 鉴权落地（Wave1）
2. `secretary` 桥接最小方法集与流式能力（Wave2）
3. 前端最小接入 + 端到端烟测（Wave3）

## 4. Wave Map（执行顺序）

| Wave | 目标 | depends_on | 主要交付 |
|---|---|---|---|
| Wave 1 | A2A 协议接线与鉴权基础 | [] | `a2a-go` 对接、路由开关、Token 验证、AgentCard |
| Wave 2 | `secretary` 桥接与最小方法集 | [Wave 1] | `message/send`、`tasks/get`、`tasks/cancel`、`message/stream` |
| Wave 3 | 前端接入与回归收口 | [Wave 2] | `a2aClient`、Chat A2A UI、E2E/烟测 |

## 5. Global Quality Gates（跨 Wave）

### F - Functional Gate
- `A2A_ENABLED=false` 时，A2A 路由不可用且 legacy 行为不变；`/.well-known/agent-card.json` 返回 404（非 `index.html`）。
- `A2A_ENABLED=true` + 合法 token 时，最小方法集可用；缺失/错误 token 返回 401。

### Q - Quality Gate
- A2A 新增 handler/service 具备契约测试（成功 + 错误路径）。
- 前端新增 A2A 客户端与 Chat 交互有单元测试覆盖。

### C - Compatibility Gate
- 现有 `/api/v1/chat`、`/plans`、`/pipeline`、`/ws` 相关测试必须保持通过。
- 默认配置下前端行为与现状一致。

## 6. Wave Plan Files

- [Wave 1 Plan](./2026-03-03-a2a-global-wave1.md)
- [Wave 2 Plan](./2026-03-03-a2a-global-wave2.md)
- [Wave 3 Plan](./2026-03-03-a2a-global-wave3.md)

## 7. Baseline Smoke（参考已跑通）

> 当前仓库已包含并可运行 `cmd/a2a-smoke`，后续各 Wave 以其作为协议回归基线。

- 基线命令（无认证场景）：  
  `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3`
- 目标命令（认证场景，Wave1 增补 token 参数后）：  
  `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3 -token <A2A_TOKEN>`

## 8. Full Regression Command Set

### Backend
- `go test ./internal/web/... -count=1`
- `go test ./internal/secretary/... -count=1`
- `pwsh -NoProfile -File .\scripts\test\backend-all.ps1`

### Frontend
- `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`
- `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1`

### Integration
- `pwsh -NoProfile -File .\scripts\test\p3-integration.ps1`
- `go run ./cmd/a2a-smoke -card-base-url http://127.0.0.1:8080 -a2a-version 0.3`

## 9. Workspace Strategy

- 使用单一 plan 级 worktree/分支：`feat/a2a-global-rollout`。
- Wave 1..3 复用同一 worktree，避免跨 wave 偏移。

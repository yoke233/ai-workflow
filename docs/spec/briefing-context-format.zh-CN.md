# Briefing 上下文格式规格

> 状态：现行
>
> 最后按代码核对：2026-03-13
>
> 对应实现：`internal/application/flow/briefing_builder.go`、`internal/application/flow/pipeline.go`

## 1. 概述

每当 Agent 执行一个 Action（exec / gate / plan）时，引擎会自动组装一段 **Briefing 上下文**，作为 Agent 的输入。上下文由多个 **ContextRef**（上下文引用块）按优先级顺序拼接而成，每个引用块有独立的字符预算，最终整体不超过 12,000 字符。

本文档描述组装后的上下文完整格式，以便排障、日志审计和 Agent prompt 设计。

## 2. ContextRef 类型清单

| 类型 | 常量名 | 字符预算 | 状态 | 说明 |
|------|--------|----------|------|------|
| `project_brief` | `CtxProjectBrief` | 800 | ✅ 已实现 | 项目名称、类型、描述、资源绑定 |
| `issue_summary` | `CtxIssueSummary` | 800 | ✅ 已实现 | WorkItem 标题 + 正文摘要 |
| `progress_summary` | `CtxProgressSummary` | 800 | ✅ 已实现 | 当前 WorkItem 的 Action 执行进度 |
| `upstream_artifact` | `CtxUpstreamArtifact` | 4000 | ✅ 已实现 | 上游 Action 的产出物（L2 全文 / L0 摘要） |
| `feature_manifest` | `CtxFeatureManifest` | 2000 | ✅ 已实现 | 项目功能清单（compact JSON） |
| `skills_summary` | `CtxSkillsSummary` | 1000 | ✅ 已实现 | Agent Profile 可用技能概览 |
| `agent_memory` | `CtxAgentMemory` | 1500 | 🔲 未实现 | Agent 历史经验召回（Phase 2） |

**全局上限**：`maxInputTotalChars = 12,000` 字符。超出时按注入顺序截断后续块。

## 3. 注入顺序

ContextRef 按以下固定顺序注入，优先级从高到低：

```
1. project_brief      — 项目是什么
2. issue_summary       — 当前工作项是什么
3. progress_summary    — 执行到哪了
4. upstream_artifact   — 上游产出了什么
5. feature_manifest    — 项目功能全景
6. skills_summary      — Agent 有什么技能
```

这个顺序的设计逻辑：

- 先给 Agent 建立项目和任务方向感（project + work item）
- 再给执行进度感知（progress）
- 然后给具体输入材料（upstream deliverables）
- 最后补全景和能力感知（manifest + skills）

## 4. 渲染后的完整格式

下面是一个完整的渲染示例（带所有类型的 ContextRef）：

```markdown
Implement OAuth login flow for the web application

# Context

## project

**my-app** (dev)

A full-stack web application for task management.

Resources:
- main repo: https://github.com/example/my-app

## work item

**Add OAuth login support**

Users should be able to log in via GitHub OAuth. Include callback handling,
session creation, and error pages.

## execution progress

Progress: 2/4 actions completed
- [done] plan
- [done] implement-backend
- [running] implement-frontend ← current
- [pending] review

## upstream action 201 output

## OAuth Backend Implementation

Added the following files:
- `internal/auth/oauth.go` — GitHub OAuth client
- `internal/auth/callback.go` — callback handler
- `internal/auth/session.go` — session management

All backend tests pass. The `/auth/github` and `/auth/callback` endpoints
are ready for frontend integration.

## feature manifest

[{"key":"oauth-login","status":"pending","description":"GitHub OAuth login flow","work_item_id":42,"tags":["auth"]},{"key":"dashboard","status":"pass"},{"key":"settings","status":"pass"}]

## available skills

- **skill-go-conventions**: Go 编码规范与最佳实践
- **skill-testing-discipline**: 确保所有代码修改都附带相应的单元测试

# Acceptance Criteria

- Frontend login button triggers GitHub OAuth flow
- Callback correctly creates user session
- Error states show user-friendly messages
```

## 5. 各 ContextRef 类型的详细格式

### 5.1 project_brief

```markdown
**{project.Name}** ({project.Kind})

{project.Description}

Resources:
- {binding.Label}: {binding.URI}
- {binding.Label}: {binding.URI}
```

- 来源：`Store.GetProject()` + `Store.ListResourceBindings()`
- 条件：WorkItem 必须关联 ProjectID
- 如果项目没有描述，只输出名称和类型
- 如果没有资源绑定，省略 Resources 部分

### 5.2 issue_summary

```markdown
**{workItem.Title}**

{workItem.Body}  (最多 500 字符, 超出加 [...])
```

- 来源：`Store.GetWorkItem()`
- 条件：WorkItem 必须有非空 Title

### 5.3 progress_summary

```markdown
Progress: {doneCount}/{totalCount} actions completed
- [done] plan
- [done] implement-backend
- [running] implement-frontend ← current
- [pending] review
```

- 来源：`Store.ListActionsByWorkItem()`
- 条件：WorkItem 必须有 2 个以上 Action（单步不注入）
- 当前 Action 标记 `← current`
- 状态标记：`done` / `running` / `ready` / `failed` / `blocked` / `waiting` / `pending`

### 5.4 upstream_artifact

分两级注入：

**L2（直接前驱）**：完整 `ResultMarkdown`

```markdown
## upstream action {actionID} output

{deliverable.ResultMarkdown}
```

**L0（间接前驱）**：摘要优先

```markdown
## upstream action {actionID} summary

{deliverable.Metadata["summary"]}
```

如果没有 Metadata summary，fallback 到 ResultMarkdown 前 300 字符 + `[...]`。

### 5.5 feature_manifest

Compact JSON 数组，每个元素：

```json
{
  "key": "feature-key",
  "status": "pending|pass|fail|skipped",
  "description": "仅 fail/pending 有此字段",
  "work_item_id": 42,
  "tags": ["tag1"]
}
```

- 来源：`Store.GetFeatureManifestByProject()` + `Store.ListFeatureEntries()`
- pass/skipped 条目只包含 key + status，节省空间

### 5.6 skills_summary

```markdown
- **{skill.Name}**: {skill.Description}
- **{skill.Name}**: {skill.Description}
```

- 来源：Agent Profile 的 `Skills` 字段 + `skills.InspectSkill()` 读取 SKILL.md 元数据
- 条件：InputBuilder 必须配置 `WithRegistry()` + `WithSkillsRoot()`
- 跳过 description 为空或 "TODO" 的技能
- 匹配逻辑：通过 `AgentRegistry.ResolveForAction()` 解析 Profile，与 Resolver 使用相同的匹配策略

## 6. 日志记录

每次 `Build()` 完成后，会通过 `slog.Info` 输出结构化日志：

```
level=INFO msg="briefing context assembled"
  action_id=101
  work_item_id=42
  ref_count=5
  raw_chars=4847
  final_chars=3847
  refs="project_brief(10):245, issue_summary(42):312, progress_summary(42):180, upstream_artifact(1):2100, skills_summary(0):210"
```

字段说明：

| 字段 | 说明 |
|------|------|
| `action_id` | 当前正在构建输入的 Action ID |
| `work_item_id` | 所属 WorkItem ID |
| `ref_count` | 注入的 ContextRef 数量 |
| `raw_chars` | 所有 ContextRef 的原始字符总数（截断前） |
| `final_chars` | 渲染后的最终输入字符数（截断后） |
| `refs` | 格式 `{type}({refID}):{chars}`，每个 ref 的原始字符数，按注入顺序排列 |

## 7. 截断规则

1. 每个 ContextRef 先按自身 `refBudget()` 截断
2. 截断后检查剩余全局字符预算 `remaining`
3. 如果超出 remaining，再次截断到 remaining 大小
4. 截断后的文本如果为空，整个 ContextRef 跳过
5. 截断标记：`\n\n[truncated]`

## 8. 扩展点

### 8.1 agent_memory（Phase 2）

预留了 `CtxAgentMemory` 类型和 1,500 字符预算。实现后将在 `upstream_artifact` 之前注入，格式为：

```markdown
## relevant experience

- [case] 上次实现 OAuth 时忘了处理 token 过期，导致 Gate 被拒
- [pattern] Go HTTP handler 测试建议使用 httptest.NewServer
```

### 8.2 自定义注入

`DefaultInputBuilder` 支持通过 `InputBuilderOption` 扩展：

```go
builder := NewInputBuilder(store,
    WithRegistry(registry),      // 启用 skills 注入
    WithSkillsRoot(skillsRoot),  // skills 目录路径
)
```

未来可增加更多 Option，例如 `WithMemoryStore()` 用于 agent_memory 注入。

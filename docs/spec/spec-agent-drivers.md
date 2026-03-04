# V2 Agent Driver 规范（Role Binding + WorkflowProfile）

## 范围

本规范定义 Team Leader 体系下的 Agent 角色绑定、会话策略，
以及 WorkflowProfile 驱动的执行/审核要求。

## 角色绑定（Role Binding）

```yaml
role_bindings:
  team_leader:
    agent: codex
    capabilities: [read_repo, write_repo, call_tools]
    session:
      reuse: true
      reset_prompt: false

  reviewer:
    agent: codex
    capabilities: [read_repo, call_tools]
    session:
      reuse: true
      reset_prompt: true

  implementer:
    agent: codex
    capabilities: [read_repo, write_repo, call_tools]
    session:
      reuse: true
      reset_prompt: true
```

约束：

- 默认角色必须是 `team_leader`。
- 旧 `secretary` 字段不再接受。
- prompt 模板默认 `team_leader.tmpl`。

## WorkflowProfile 编排规则

### normal

- 1 reviewer + 1 aggregator
- 标准 SLA：`sla_minutes=60`

### strict

- 3 reviewers 并行 + 1 aggregator
- 更高通过阈值与更严格失败判定

### fast_release

- 轻量 reviewer + 快速 aggregator
- 允许更短反馈路径，但必须留审计痕迹

## 会话策略

- `reuse=true`：同一角色可复用会话。
- `reset_prompt=true`：复用前注入重置语义，避免上下文漂移。
- 会话失效（找不到/超时/权限异常）时自动新建。

## Run 事件要求

每个 run 至少发布：

1. `run_created`
2. `run_started`
3. `run_updated`（0..N）
4. 结束态之一：`run_completed | run_failed | run_timeout | run_cancelled`

最小持久化字段：

- `project_id`
- `session_id`
- `issue_id`
- `run_id`
- `event_type`
- `update_type`（update 时可用）
- `payload`
- `created_at`

## Review 事件要求

- review 结果必须写入 issue 时间线。
- `kinds=review` 查询必须返回摘要与原始输出片段。
- review 失败也必须留痕，禁止静默降级。
- issue 含 `review_scope.files` 时，review 输出必须标注覆盖文件集合，不得越界。

## 禁止项

- 禁止跨角色混用同一会话。
- 禁止丢失 run 结束态事件。
- 禁止只做内存事件不落库。
- 禁止继续使用 `chat_run_*` 或 `secretary_*` 作为 V2 标准事件名。

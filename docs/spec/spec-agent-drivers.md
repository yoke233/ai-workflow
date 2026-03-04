# V2 Agent Driver 规范（profile 驱动）

## 范围

本规范定义 Team Leader 与执行 Agent 的 profile 装配、会话策略、事件落库要求。

## Profile 模型

```yaml
profiles:
  - id: team_leader
    agent: codex
    capabilities: [read_repo, write_repo, call_tools]
    session:
      reuse: true
      reset_prompt: false

  - id: reviewer
    agent: codex
    capabilities: [read_repo, call_tools]
    session:
      reuse: true
      reset_prompt: true

  - id: implementer
    agent: codex
    capabilities: [read_repo, write_repo, call_tools]
    session:
      reuse: true
      reset_prompt: true
```

## Team Leader profile 选择

- 输入：用户消息 + issue 状态 + 最近 run 事件。
- 输出：`profile_id`。
- 默认策略：
  - 需求澄清与拆分：`team_leader`
  - 代码执行：`implementer`
  - 结果评审：`reviewer`

## 会话策略

- `reuse=true`：同一 `profile_id` 可复用 Agent 会话。
- `reset_prompt=true`：复用会话前注入重置语义，避免历史漂移。
- 会话失效（找不到/超时/权限异常）时自动新建。

## run 事件要求

每次 run 至少发布：

1. `chat_run_started`
2. `chat_run_update`（0..N）
3. 结束态之一：`chat_run_completed | chat_run_failed | chat_run_cancelled`

持久化字段最小集合：

- `project_id`
- `session_id`
- `event_type`
- `update_type`（update 事件时）
- `payload`
- `created_at`

## review 事件要求

- review 结果必须写入 issue 时间线。
- `kinds=review` 查询必须可返回摘要与原始输出片段。
- review 失败也必须留痕。

## 禁止项

- 禁止跨 profile 混用同一会话。
- 禁止丢失 run 结束态事件。
- 禁止只做内存事件而不落库。

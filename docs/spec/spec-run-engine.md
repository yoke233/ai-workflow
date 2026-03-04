# Run Engine 规范

## 目标

Run Engine 接收 `issue + profile`，执行一次 run 并提供可追踪事件。

## 执行输入

- `project_id`
- `issue_id`
- `profile_id`
- `workdir`
- `message/context`

## 执行状态

- `started`
- `running`
- `completed`
- `failed`
- `cancelled`

## 事件模型

### 必选事件

- `chat_run_started`
- `chat_run_update`
- `chat_run_completed` / `chat_run_failed` / `chat_run_cancelled`

### update 子类型

- `tool_call`
- `tool_call_update`
- `progress_map`（结构化步骤更新）
- 其他 ACP 原生类型

## 落库要求

- 非 chunk 更新必须入库。
- chunk 内容允许仅用于流式展示，不强制落库。
- 入库记录按 `created_at` 升序返回。

## 取消与恢复

- 取消：必须触发上下文 cancel，并写入取消事件。
- 恢复：由上层重新触发新的 run，不复用已结束 run 状态。

## 与 review 的关系

- run 输出会驱动 issue review。
- review 结论写入 issue 时间线，run 仅负责运行侧事件。

## 验收基线

至少通过以下烟雾验证：

1. issue 创建并进入 review。
2. 指定 profile 发起 run。
3. 可查询 run 事件列表。
4. 可查询 issue review 事件。

# WorkflowRun Engine 规范

## 目标

Run Engine 接收 `issue + workflow_profile`，驱动一次 `workflow_run`
状态机，并输出可追踪、可回放的运行事件。

## 输入契约

- `project_id`
- `issue_id`
- `session_id`
- `workflow_profile`: `normal | strict | fast_release`
- `workdir`
- `message` / `context`
- `trigger`: `user | system | github`（可扩展）

## 调度模型（替代 DAG）

- 使用 `profile queue scheduler`，不再构建或维护 DAG。
- issue 在 `ready` 后直接进入对应 profile 队列。
- run 生命周期由事件监听器推进，不依赖 `depends_on/in_degree/topo` 字段。

## 状态机

允许状态：

- `created`
- `running`
- `waiting_review`
- `done`
- `failed`
- `timeout`
- `cancelled`

推荐转移：

1. `created -> running`
2. `running -> waiting_review`（需要审核时）
3. `running -> done | failed | timeout | cancelled`
4. `waiting_review -> done | failed | timeout`

约束：

- 禁止从结束态转回运行态。
- `timeout` 与 `cancelled` 必须带原因字段。
- 任何异常退出都必须收敛到可观察结束态。

## 事件模型

### 事件命名

Run Engine 只产生 `run_*` 事件：

- `run_created`
- `run_started`
- `run_updated`
- `run_waiting_review`
- `run_completed`
- `run_failed`
- `run_timeout`
- `run_cancelled`

Team Leader 侧事件使用 `team_leader_*` 前缀，由上层模块发布。

### run_updated 子类型

- `tool_call`
- `tool_call_update`
- `progress_map`
- `log`
- `artifact`

## 落库要求

- 非 chunk 更新必须入库。
- chunk 可仅流式展示，不强制落库。
- 最小字段集：
  - `project_id`
  - `session_id`
  - `issue_id`
  - `run_id`
  - `event_type`
  - `update_type`（可空）
  - `payload`
  - `created_at`
- 查询默认按 `created_at ASC`。

## SLA 与超时

- `workflow_profile.sla_minutes` 默认 60 分钟。
- 超时后必须：
  1. 中断执行上下文；
  2. 写入 `run_timeout`；
  3. 回写 issue 时间线（含超时摘要）。

## 取消与恢复

- 取消：上层调用取消接口，必须写 `run_cancelled`。
- 恢复：通过新建 run 完成，不复用已结束 run。
- 重试：由 Team Leader/策略层决定，不在 Run Engine 内隐式自动重放。

## 与 Review 的关系

- run 只负责运行侧状态与事件。
- review 结论由 Review Orchestrator 写入 issue 时间线。
- `waiting_review` 是 run 与 review 的衔接态，不是独立业务实体。
- 当 issue 存在 `review_scope.files` 时，`waiting_review` 阶段必须校验 review 输出文件集合不越界。

## 验收基线

1. issue 进入 `ready` 后可触发 run 创建。
2. run 全程可查询事件并具备明确结束态。
3. `strict`、`normal`、`fast_release` 的审核分流可观测。
4. 60 分钟 SLA 超时路径可复现并留痕。

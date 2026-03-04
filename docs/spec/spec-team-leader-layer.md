# Team Leader 层规范

## 角色定位

Team Leader 是系统唯一用户入口，职责如下：

- 维护长期上下文与会话状态。
- 将用户目标沉淀为 issue。
- 为每个 issue 选择 profile 并触发 run。
- 聚合 run/review 事件并回显给用户。

## 输入输出

### 输入

- 用户消息
- 项目仓库上下文
- 历史 issue 状态
- 最近 run 事件

### 输出

- issue 变更
- run 启停命令
- review 结论
- 用户可读摘要

## Issue 生命周期

`draft -> reviewing -> ready -> executing -> done/failed/abandoned`

约束：

- 状态转换必须记录 `issue_changes`。
- review 结论必须写入 `review_records`。
- `executing` 状态下必须有可追踪 run 事件。

## profile 选择规则

1. 默认 profile 为 `team_leader`。
2. 当 issue 进入执行态，切换到 `implementer`。
3. 当 issue 需要审查，切换到 `reviewer`。

## run 协调规则

- 每次用户提交触发至多一个活跃 run。
- 同一 session 同时只允许一个运行态。
- 取消 run 后必须写入 `chat_run_cancelled`。

## 观测规则

Team Leader 必须支持两类查询：

- 会话运行事件：`/chat/{sessionID}/events`
- issue review 时间线：`/issues/{issueID}/timeline?kinds=review`

## 失败处理

- profile 不可用：返回明确错误并保持 issue 原状态。
- run 异常中断：写入 `chat_run_failed` 并记录错误摘要。
- review 写入失败：返回 5xx，禁止静默吞错。

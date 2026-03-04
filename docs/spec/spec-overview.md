# V2 总览规范（issue / profile / run / Team Leader）

## 目标

Wave4 起统一采用 `issue -> profile -> run` 模型：

- Team Leader 负责面向用户的持续对话与编排。
- issue 是唯一交付单元。
- profile 决定 Agent 的角色能力与执行策略。
- run 是执行与观测的最小实例。

## 架构分层

1. `Web API`：提供项目、issue、chat、事件查询接口。
2. `Team Leader`：接收用户输入，维护上下文，触发 issue 与 run。
3. `Issue Service`：issue 创建、review、变更追踪。
4. `Run Engine`：按 profile 执行 run，产出运行事件。
5. `Event Store`：持久化 `chat_run_*` 与 issue review 事件。
6. `Integration`：GitHub 等外部系统对接。

## 统一对象

### Issue

- `id`：`issue-*`
- `title/body`
- `status`：`draft | reviewing | ready | executing | done | failed | abandoned`
- `session_id`：所属 Team Leader 会话
- `auto_merge`：自动合并开关

### Profile

- `id`：如 `team_leader`、`reviewer`、`implementer`
- `capabilities`：工具权限与资源范围
- `session_policy`：是否复用会话、是否重置提示

### Run

- `run_id`（内部可由 session + 时间推导）
- `issue_id`
- `profile_id`
- `status`：`started | running | completed | failed | cancelled`
- `events[]`：流式更新与持久化事件

## 标准主链路

1. 用户向 Team Leader 发送消息。
2. Team Leader 选择 profile。
3. 系统创建或更新 issue。
4. 系统启动 run。
5. run 输出 `chat_run_started / chat_run_update / chat_run_completed` 等事件。
6. review 结果写入 issue 时间线，并可通过 API 查询。

## 事件观测基线

- 运行事件：`GET /api/v1/projects/{projectID}/chat/{sessionID}/events`
- review 事件：`GET /api/v1/projects/{projectID}/issues/{issueID}/timeline?kinds=review`

## 非目标

- 不再维护旧命名模型与旧文档叙述。
- 不提供兼容层说明。

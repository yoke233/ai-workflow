# V2 API 规范（issue / profile / run / Team Leader）

## 约定

- Base：`/api/v1`
- 鉴权：Bearer（按部署配置）
- 响应错误体：`{ "error": "...", "code": "..." }`

## 项目

### 创建项目

`POST /projects`

```json
{
  "name": "demo",
  "repo_path": "D:/repo/demo"
}
```

### 查询项目

- `GET /projects`
- `GET /projects/{projectID}`

## Team Leader 对话与 run

### 发起/续接对话

`POST /projects/{projectID}/chat`

```json
{
  "message": "请拆分 issue 并给出执行建议",
  "role": "team_leader",
  "session_id": ""
}
```

说明：

- `role` 作为 profile 输入。
- `session_id` 为空表示新会话；非空表示续接。

### 取消运行

`POST /projects/{projectID}/chat/{sessionID}/cancel`

### 查询运行事件

`GET /projects/{projectID}/chat/{sessionID}/events`

返回数组元素关键字段：

- `event_type`：`chat_run_started | chat_run_update | chat_run_completed | chat_run_failed | chat_run_cancelled`
- `update_type`：例如 `tool_call`
- `payload`：ACP 更新原文与扩展字段

## Issue

### 创建 issue

`POST /projects/{projectID}/issues`

```json
{
  "session_id": "chat-xxx",
  "name": "auth-refactor",
  "fail_policy": "block",
  "auto_merge": false
}
```

### 基于文件创建 issue 并进入 review

`POST /projects/{projectID}/issues/from-files`

```json
{
  "session_id": "chat-xxx",
  "name": "auth-refactor",
  "file_paths": ["docs/feature.md", "README.md"],
  "auto_merge": false
}
```

### Issue 查询

- `GET /projects/{projectID}/issues`
- `GET /projects/{projectID}/issues/{issueID}`
- `GET /projects/{projectID}/issues/{issueID}/reviews`
- `GET /projects/{projectID}/issues/{issueID}/changes`
- `GET /projects/{projectID}/issues/{issueID}/timeline`

### Issue 动作

- `POST /projects/{projectID}/issues/{issueID}/review`
- `POST /projects/{projectID}/issues/{issueID}/action`
- `POST /projects/{projectID}/issues/{issueID}/auto-merge`

## 时间线与 review 观测

`GET /projects/{projectID}/issues/{issueID}/timeline?kinds=review,log,checkpoint,action`

时间线元素统一字段：

- `event_id`
- `kind`
- `created_at`
- `actor_type`
- `title`
- `body`
- `status`
- `refs`
- `meta`

## 术语约束

- 文档、脚本、接口说明统一使用 `issue / profile / run / Team Leader`。
- 旧模型命名不再记录。

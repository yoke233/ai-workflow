# A2A Backend Smoke（Token 场景）

- 执行日期：2026-03-03
- 执行目录：`D:\project\ai-workflow\.worktrees\feat-a2a-global-rollout`
- 目标：验证 `cmd/a2a-smoke` 在 Bearer Token 场景下可完成 AgentCard + JSON-RPC `message/send` + `tasks/get` 基础链路。

## 预置条件

由于本机 `8080` 端口已有其他服务，本次 smoke 使用 `18080` 端口。

服务启动环境变量：

```powershell
$env:AI_WORKFLOW_A2A_ENABLED='true'
$env:AI_WORKFLOW_A2A_TOKEN='wave2-a2a-token'
$env:AI_WORKFLOW_A2A_VERSION='0.3'
```

服务启动命令：

```powershell
go run ./cmd/ai-flow server --port 18080
```

健康检查与 AgentCard 检查：

```text
GET /health -> 200 {"status":"ok"}
GET /.well-known/agent-card.json -> 200 (protocolVersion=0.3)
```

## Smoke 命令

```powershell
go run ./cmd/a2a-smoke `
  -card-base-url http://127.0.0.1:18080 `
  -a2a-version 0.3 `
  -token wave2-a2a-token `
  -project-id ai-workflow `
  -max-poll 1 `
  -allow-nonterminal `
  -timeout 180s
```

## 输出摘要

```text
rpc_url=http://127.0.0.1:18080/api/v1/a2a
card_protocol_version=0.3
send_result={"contextId":"","id":"plan-20260303-b36764f2","kind":"task","metadata":{"project_id":"ai-workflow"},"status":{"state":"input-required","timestamp":"2026-03-03T04:40:14Z"}}
task_result[1]={"contextId":"","id":"plan-20260303-b36764f2","kind":"task","metadata":{"project_id":"ai-workflow"},"status":{"state":"input-required","timestamp":"2026-03-03T04:40:14Z"}}
task_state=input-required
task_non_terminal=true
```

## 结论

- Token 鉴权链路正常：`a2a-smoke` 在带 `Bearer` token 情况下可成功访问 AgentCard 与 RPC 端点。
- 最小方法集链路正常：`message/send` 与 `tasks/get` 均返回可解析的 Task 响应。
- 当前任务状态为 `input-required`，属于非终态；本次 smoke 通过 `-allow-nonterminal` 保留状态证据并判定链路可用。

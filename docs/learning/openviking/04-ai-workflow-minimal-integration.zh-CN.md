# ai-workflow 最小集成方案（OpenViking）

最后更新：2026-03-03

## 1. 范围与原则

目标：先把 OpenViking 接入 `ai-workflow`，不引入复杂角色目录。

原则：

- 一个项目一个记忆池；
- `secretary` 统一写入；
- `worker/reviewer` 只读；
- 角色差异先放 metadata，不放目录层级。

## 2. 最小 URI 策略

默认加载范围：

1. `viking://resources/shared/`
2. `viking://resources/projects/{project_id}/`
3. `viking://memory/projects/{project_id}/`

按 mode 追加：

- `implement_backend` 追加 `.../backend/`
- `implement_frontend` 追加 `.../frontend/`
- `review` 追加 `.../api/`

## 3. 写入策略

- `secretary`: 允许 `commit`
- `worker/reviewer`: 禁止 `commit`，仅 `load`

写入类别限制：

- `preferences`（长期偏好）
- `patterns`（可复用套路）
- `events`（关键事件）

## 4. 代码层落点（当前仓库）

1. `internal/web/chat_assistant_acp.go`
- Chat 回合执行入口；
- 适合挂载 `secretary` 的 load/commit。

2. `internal/engine/executor.go`
- `worker/reviewer` 执行路径；
- 适合在 prompt 构建阶段注入只读记忆上下文。

3. `internal/secretary/mcp_tools.go`
- 当前 MCP tools 白名单在这里；
- 后续若扩展 OpenViking 工具，可在此做统一映射和权限门禁。

4. `cmd/viking/main.go`
- 本地 smoke 工具；
- 已提供 `plan`（策略输出）和 `probe`（服务探活）。

## 5. 当前阶段不做的事

- 不做 `company/team/project/role` 深目录；
- 不做多写入方；
- 不做复杂记忆分区；
- 不做一次性全量迁移。

## 6. 升级触发条件（再考虑角色目录）

满足任一条件再拆分：

1. 同项目多角色记忆明显互相干扰；
2. 权限隔离要求提高；
3. 检索命中率因记忆体量下降。

届时增量加：

```text
viking://memory/projects/{project_id}/by-role/{role}/
```

不影响现有项目级记忆池。

# ACP 学习记录

更新时间：2026-03-02
文档类型：学习记录（非项目规范）

## 1. ACP 是什么

ACP（Agent Client Protocol）是一个让「客户端（编辑器/IDE）」和「Agent（编码智能体）」标准化通信的协议。

可以把它理解成：AI 编码场景里的“通用对接层”。

- Client 负责：UI、权限决策、文件系统视图、终端能力等。
- Agent 负责：模型推理、计划、tool 执行编排、结果回传。
- 通信基础：JSON-RPC 2.0。
- 典型传输：stdio（最主流）；也在推进更多远程场景。

## 2. 为什么要用 ACP

不使用 ACP 时，通常是“一个 Agent 对一个编辑器写一套私有集成”，成本高且无法互通。  
使用 ACP 后，编辑器和 Agent 可以通过统一协议对接，复用生态能力。

## 3. 最小可用流程（怎么用）

### 3.1 初始化握手

1. Client -> Agent: `initialize`
2. 双方协商：
   - `protocolVersion`
   - `clientCapabilities`
   - `agentCapabilities`
   - `authMethods`（如果需要认证）

关键原则：

- capability 没声明就视为不支持。
- 不能调用对方未声明支持的方法。

### 3.2 可选认证

如果 Agent 需要登录：

1. Client 读取 `authMethods`
2. Client -> Agent: `authenticate(methodId)`
3. 成功后再创建 session

### 3.3 建立会话

Client -> Agent: `session/new`

核心参数：

- `cwd`：会话工作目录（必须绝对路径）
- `mcpServers`：可选 MCP 连接配置

注意：`cwd` 应作为文件工具操作边界。

### 3.3b 可选恢复会话

如果 Agent 声明支持 `loadSession`，可使用 `session/load` 恢复历史会话。

常见用法：

1. 先 `session/list`（如支持）让用户选择历史会话
2. 再 `session/load(sessionId, cwd, mcpServers)` 恢复上下文
3. Agent 通过 `session/update` 回放历史消息后进入可继续对话状态

### 3.4 发起一轮对话（Prompt Turn）

1. Client -> Agent: `session/prompt`
2. Agent -> Client: 多次 `session/update`（流式文本、计划、tool 状态）
3. 结束时 Agent 返回 `stopReason`

### 3.5 取消执行

Client -> Agent: `session/cancel`

Agent 必须返回语义化 `cancelled`，不能把取消直接当未处理异常抛给前端。

## 4. 权限模型（重点）

ACP 的“权限”不是单一开关，而是三层叠加：

### 4.1 能力权限（Capability Gate）

由 `initialize` 协商决定可调用面：

- 文件能力：`fs.readTextFile` / `fs.writeTextFile`
- 终端能力：`terminal`
- 其它能力（会话加载、内容类型等）

如果能力为 `false` 或缺失，Agent 必须不调用对应方法。

### 4.2 会话边界权限（Scope）

`session/new` 的 `cwd` 定义了会话文件系统上下文，协议建议将其作为工具边界。

### 4.3 运行时授权（Tool Permission）

Agent 在执行敏感 tool 前可请求授权：

- 方法：`session/request_permission`
- 选项类型：
  - `allow_once`
  - `allow_always`
  - `reject_once`
  - `reject_always`

Client 可以根据用户策略自动放行/拒绝，不必每次弹窗。

如果 prompt turn 被取消，Client 必须把所有 pending permission request 统一回 `cancelled`。

## 5. 文件读取与写入

### 5.1 读取：`fs/read_text_file`

- 支持读取编辑器中的未保存内容（这是 ACP 很实用的点）。
- `path` 必须绝对路径。
- 支持从某行开始和限制读取行数（行号 1-based）。

### 5.2 写入：`fs/write_text_file`

- `path` 必须绝对路径。
- 文件不存在时，Client 必须创建该文件。

## 6. Tool 调用与状态回传

Agent 通过 `session/update` 回传 tool 生命周期：

- `tool_call`：创建
- `tool_call_update`：更新

常见状态：

- `pending`
- `in_progress`
- `completed`
- `failed`

tool 结果可包含：

- 普通内容（文本/资源等）
- `diff`（代码变更）
- `terminal`（实时终端输出）

## 7. 终端执行（terminal/*）

常用方法：

- `terminal/create`
- `terminal/output`
- `terminal/wait_for_exit`
- `terminal/kill`
- `terminal/release`

关键约束：

- 未声明 `terminal` capability 时不能调用终端方法。
- `outputByteLimit` 可限制保留输出大小。
- Agent 使用完终端后必须 `terminal/release`，否则容易泄漏资源。
- `release` 后 terminalId 失效。

## 8. 常见问题与易踩坑

### 8.1 直接调用未协商能力的方法

现象：方法不存在/参数错误/行为不一致。  
原因：没有先看 `initialize` 返回能力。

### 8.2 取消处理不规范

现象：用户点取消后 UI 报红错误。  
原因：Agent 未把中断异常转换成 `stopReason=cancelled`。

### 8.3 路径不是绝对路径

现象：文件类操作失败或跨平台行为不一致。  
建议：协议层统一绝对路径，内部再做映射。

### 8.4 终端资源泄漏

现象：会话结束后仍有残留进程。  
原因：缺少 `terminal/release`。

### 8.5 把草案能力当稳定能力

例如 `session/list`、`usage_update`、更丰富 auth 交互等，很多仍在 Draft/Preview 演进中。  
建议：默认只承诺稳定规范；草案能力走 feature flag。

### 8.6 文档与 schema 个别字段命名差异

实践中遇到过以下差异（实现时应优先以 schema 为准）：

- `mcp` vs `mcpCapabilities`
- `modeId` vs `currentModeId`
- `config_options_update` vs `config_option_update`

## 9. 落地建议（给团队）

1. 先做“稳定最小集”：
   - `initialize`
   - `authenticate`（可选）
   - `session/new`
   - `session/prompt`
   - `session/cancel`
   - `session/update`
2. 权限策略默认保守：
   - 写文件/执行命令默认需授权
   - 提供一次性与长期授权两种选项
3. 把 capability 检查做成统一守卫，不要散落在业务代码里。
4. 把 `cancelled` 处理做成统一中间层。
5. 对 draft 能力单独开关，避免和稳定行为混用。

## 10. 给新同学的一句话

ACP 的核心不是“让 Agent 更聪明”，而是“让 Agent 与编辑器协作时可控、可观测、可互操作”。

## 11. 推荐阅读（本仓库内）

- `docs/vendor/acp-protocol-upstream-docs/get-started/introduction.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/overview.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/initialization.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/session-setup.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/prompt-turn.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/tool-calls.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/file-system.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/terminals.mdx`
- `docs/vendor/acp-protocol-upstream-docs/protocol/schema.mdx`
- `docs/vendor/acp-protocol-upstream-docs/rfds/about.mdx`

## 12. 版本与适用范围（先看这个）

本文是 ACP 学习记录，不是 ACP 官方规范，也不是本项目“强约束规范”。

### 12.1 适用范围

- 目标：帮助新同学快速理解 ACP 核心机制和落地风险。
- 适用对象：实现或接入 ACP Client/Agent 的开发者。
- 不适用对象：需要逐字段法律级/标准级约束的场景（应直接读 schema 与上游文档）。

### 12.2 稳定能力与草案能力边界

建议将能力分两层管理：

- 稳定层（默认启用）：`protocol/*.mdx` + `protocol/schema.mdx` 已稳定字段
- 草案层（默认关闭）：`protocol/draft/*.mdx` 与 `rfds/*.mdx` 中仍在演进的能力

工程建议：

1. 草案能力必须 feature flag 控制。
2. 对外承诺文档只写稳定层。
3. 版本升级时先做兼容性扫描，再打开草案能力。

### 12.3 传输层硬约束（stdio）

- JSON-RPC 消息 UTF-8 编码。
- 每条消息以 `\n` 分隔，消息体不能包含嵌入换行。
- Agent 的 `stdout` 只能输出有效 ACP 消息；日志应走 `stderr`。
- Client 的 `stdin` 写入也必须是有效 ACP 消息。

## 13. 方法与能力矩阵（实现参考）

说明：

- Direction：谁调用谁
- Capability Gate：是否必须先协商能力
- 级别：MUST / SHOULD / MAY 表示规范强度

| 方法 | Direction | Capability Gate | 级别 | 备注 |
|---|---|---|---|---|
| `initialize` | Client -> Agent | 无 | MUST | 连接后第一步，协商版本与能力 |
| `authenticate` | Client -> Agent | `authMethods` 非空时使用 | MAY | 未认证可能导致 `auth_required` |
| `session/new` | Client -> Agent | 无 | MUST | `cwd` 绝对路径 |
| `session/load` | Client -> Agent | `agentCapabilities.loadSession=true` | MAY | 仅当 Agent 支持 |
| `session/prompt` | Client -> Agent | 内容类型受 `promptCapabilities` 限制 | MUST | 一轮对话入口 |
| `session/cancel` | Client -> Agent | 无 | MAY | 取消当前 prompt turn |
| `session/update` | Agent -> Client | 无 | MUST | 流式更新、tool 状态、计划等 |
| `session/request_permission` | Agent -> Client | 无 | MAY | 敏感操作前请求授权 |
| `fs/read_text_file` | Agent -> Client | `clientCapabilities.fs.readTextFile=true` | MAY | 可读未保存编辑态 |
| `fs/write_text_file` | Agent -> Client | `clientCapabilities.fs.writeTextFile=true` | MAY | 不存在文件需创建 |
| `terminal/create` | Agent -> Client | `clientCapabilities.terminal=true` | MAY | 异步启动命令 |
| `terminal/output` | Agent -> Client | `clientCapabilities.terminal=true` | MAY | 拉取当前输出 |
| `terminal/wait_for_exit` | Agent -> Client | `clientCapabilities.terminal=true` | MAY | 等待命令退出 |
| `terminal/kill` | Agent -> Client | `clientCapabilities.terminal=true` | MAY | 终止命令但保留 terminal |
| `terminal/release` | Agent -> Client | `clientCapabilities.terminal=true` | MUST（使用终端后） | 释放资源，避免泄漏 |

### 13.1 Prompt 内容类型能力约束

客户端发送 `session/prompt` 时，必须遵守 Agent 声明的 `promptCapabilities`：

- 基线必须支持：`Text`、`ResourceLink`
- 可选支持：`Image`、`Audio`、`EmbeddedResource`

实现建议：

1. 在 Client 侧统一做“内容类型白名单过滤”。
2. 对不支持类型做 UI 兜底提示，不要直接发请求。

## 14. 错误码、重试与降级策略

### 14.1 常见错误码（最常用）

- `-32700` Parse error：JSON 解析失败
- `-32600` Invalid request：请求体不是合法 JSON-RPC Request
- `-32601` Method not found：方法不存在
- `-32602` Invalid params：参数不合法
- `-32603` Internal error：内部错误
- `-32000` Authentication required：需要认证
- `-32002` Resource not found：资源不存在（如文件）

### 14.2 客户端处理策略

1. `-32000`：引导用户完成认证流程，再重试原操作。
2. `-32601`：通常是能力没协商或版本不匹配，先检查 `initialize` 结果。
3. `-32602`：记录参数与 schema 差异，避免盲目重试。
4. `-32002`：区分“可恢复”（路径错误）与“不可恢复”（资源被删除）。
5. 取消场景：优先显示为“已取消”，不要显示为错误。

### 14.3 重试规则（建议）

- 幂等读操作可有限重试（指数退避）。
- 写操作默认不自动重试，除非具备幂等保障。
- 认证失败不应无限重试，避免循环弹窗。
- 网络/远程传输失败与协议语义失败要分开统计。

## 15. 安全基线与权限持久化规则

### 15.1 默认安全策略

1. 默认拒绝高风险操作（写文件、执行命令、删除/移动）。
2. 必须先做 capability gate，再做权限判定。
3. 所有路径标准化为绝对路径后再判断访问策略。
4. `cwd` 作为默认边界，越界访问需要显式授权策略。

### 15.2 权限选项落地规则

- `allow_once`：仅当前一次 tool call 生效，不跨 turn。
- `allow_always`：持久化到本地权限存储（建议按“项目 + 工具类别 + 路径范围”建键）。
- `reject_once`：当前一次拒绝，不记忆。
- `reject_always`：持久化拒绝规则，支持用户手动撤销。

建议权限存储键结构：

`{projectId}:{agentId}:{toolKind}:{scope}`

其中 `scope` 可为：

- `cwd`
- `path_prefix:/abs/path/...`
- `global`（不建议默认使用）

### 15.3 日志与审计建议

- 审计记录至少包含：`sessionId`、`toolCallId`、授权结果、操作者、时间戳。
- 不在日志中记录敏感凭据（token、API key、cookie）。
- 终端输出按敏感词规则脱敏后再持久化。

## 16. 上线前检查清单（Implementation Checklist）

上线前至少逐条确认：

1. `initialize` 已实现并正确处理未知 capability。
2. 未声明 capability 的方法不会被调用。
3. 所有文件路径在协议层均为绝对路径。
4. `session/cancel` 能可靠返回 `stopReason=cancelled`。
5. pending `session/request_permission` 在取消时会统一回 `cancelled`。
6. `fs/read_text_file` 支持读取未保存编辑态（若 Client 承诺支持）。
7. `fs/write_text_file` 对不存在文件会创建（若 Client 承诺支持）。
8. 所有 `terminal/*` 生命周期最终都会 `terminal/release`。
9. `allow_always/reject_always` 具备可撤销入口。
10. 草案能力都受 feature flag 控制，默认关闭。
11. 错误码处理与用户提示映射清晰（认证、参数、方法不存在、资源不存在）。
12. 关键流程有最小回归测试：握手、权限、读写、终端、取消。

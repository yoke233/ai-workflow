# Go 连接 ACP（codex-acp）实践心得

更新时间：2026-03-02

## 1. 本次实测结论（已跑通）

在本仓库新增了 `cmd/acp-smoke` 后，已通过 Go 真实完成以下链路：

1. 启动 Agent 进程：`npx -y @zed-industries/codex-acp@latest`
2. `initialize`
3. `session/new`
4. `session/prompt`
5. 接收 `session/update` 流式更新
6. 收到 `session/prompt` 最终响应（`stopReason=end_turn`）

实测输出里，Agent 分片返回了 `ACP_GO_OK`，最终聚合文本正确。

## 2. 我认为最关键的工程点

### 2.1 传输层要严格遵循 stdio 规则

- ACP stdio 传输是“**每条 JSON-RPC 消息一行**”。
- 消息必须 UTF-8。
- Agent 的 `stdout` 只能输出合法 ACP 消息；日志走 `stderr`。

这点看起来简单，但最容易出错。建议在客户端统一做两件事：

1. 写消息时统一 `json.Marshal + '\n'`；
2. 读消息时按行读取并做 JSON 反序列化校验。

### 2.2 会话内并发事件不是“可选项”，而是常态

`session/prompt` 过程中，不会只收到一个最终响应；中间会不断收到 `session/update`（例如文本分片、tool 状态、usage 更新）。  
因此客户端逻辑必须是：

- 一边等目标 request 的 response（按 `id` 匹配）；
- 一边处理中途 notification/request。

如果把它写成“发请求 -> 阻塞等响应（忽略中间消息）”，在复杂场景会卡死或丢状态。

### 2.3 `session/request_permission` 必须能闭环响应

即便最小 demo 不一定触发权限请求，生产实现也必须覆盖：

- Agent 发 `session/request_permission`（这是 request，不是 notification）；
- Client 需要返回 `RequestPermissionResponse`；
- 取消时要返回 `cancelled` 语义，而不是放着不回。

我在 `acp-smoke` 里加了“自动选 `allow_once`（否则选第一项）”的兜底，避免后续联调时卡在权限环节。

### 2.4 `cwd` 必须是绝对路径

`session/new` 的 `cwd` 要统一绝对路径。  
建议入口处一次性 `filepath.Abs`，不要在下游到处修补。

## 3. 关于“codex 可后期替换”的实现建议

你提到“codex 后期可替换”，这个方向很对。实践上建议把实现分三层：

1. **ACP 客户端层（稳定）**  
   只负责 JSON-RPC/stdio、request-response 对账、通知分发、权限回包。

2. **Agent 启动层（可替换）**  
   只负责“怎么启动某个 ACP Agent”。例如：
   - 当前：`npx -y @zed-industries/codex-acp@latest`
   - 将来：`<其他 agent 命令>`

3. **业务编排层（稳定）**  
   只依赖 ACP 抽象方法（initialize/session/new/session/prompt/cancel），不感知底层是 codex 还是其它实现。

这样替换 Agent 的代价就是改“启动命令与启动参数”，而不是改协议主流程代码。

## 4. 本次落地文件

- 新增命令：`cmd/acp-smoke/main.go`
- 本文档：`docs/learning/acp-go-codex-practice-notes.md`

## 5. 建议的下一步（从 demo 到可复用）

1. 把 `cmd/acp-smoke` 里的 JSON-RPC 收发逻辑抽到 `internal/acpclient` 包。
2. 加 3 组自动化测试：
   - 正常 turn 完成（`end_turn`）
   - 触发 `session/request_permission` 并正确回包
   - `session/cancel` 后返回 `cancelled`
3. 在配置层暴露 Agent 启动参数（命令、args、env），把 codex 从代码常量里移出去。

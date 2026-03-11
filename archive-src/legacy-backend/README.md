## legacy-backend

这里归档的是已经从默认运行链摘除的后端实现源码。

当前范围：

- `cmd-a2a-smoke/*`
- `cmd-ai-flow/commands_mcp.go`
- `internal-web-mcp/handlers_mcp.go`
- `internal-web-a2a/a2a_auth.go`
- `internal-web-a2a/handlers_a2a.go`
- `internal-web-a2a/handlers_a2a_protocol.go`
- `internal-web-a2a/handlers_a2a_stream.go`
- `internal-teamleader-a2a/a2a_bridge.go`
- `internal-teamleader-a2a/a2a_types.go`
- `internal-teamleader-mcp/mcp_tools.go`
- `internal-mcpserver/*`
- `tests/internal-web-a2a/*`
- `tests/internal-web-mcp/*`
- `tests/internal-teamleader-a2a/*`
- `tests/internal-teamleader-mcp/*`

说明：

- 这些文件保留仅用于历史对照和后续拆解，不再属于现行 `v2` 主线。
- 归档文件不应再被默认入口、默认 server runtime 或现行前端路由依赖。

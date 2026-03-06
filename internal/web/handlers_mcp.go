package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoke233/ai-workflow/internal/mcpserver"
)

// registerMCPRoutes mounts the MCP SSE endpoint on the router.
// The ACP agent (team_leader) can connect to this endpoint using SSE transport
// instead of spawning a stdio subprocess.
func registerMCPRoutes(r chi.Router, cfg Config) {
	if cfg.Store == nil {
		return
	}
	server := mcpserver.NewServer(cfg.Store, mcpserver.Options{
		DevMode:    cfg.MCPServerOpts.DevMode,
		SourceRoot: cfg.MCPServerOpts.SourceRoot,
		ServerAddr: cfg.MCPServerOpts.ServerAddr,
		ConfigDir:  cfg.MCPServerOpts.ConfigDir,
		DBPath:     cfg.MCPServerOpts.DBPath,
	})
	handler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)

	// SSE transport: GET creates session (SSE stream), POST sends messages.
	// Both use the same path with different methods/query params.
	r.Handle("/mcp", handler)
}

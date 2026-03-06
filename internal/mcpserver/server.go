package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoke233/ai-workflow/internal/core"
)

// Options controls MCP server behavior.
type Options struct {
	DevMode    bool
	SourceRoot string // go build working directory
	ServerAddr string // server HTTP address for self_restart
	ConfigDir  string // path to .ai-workflow/ directory
	DBPath     string // SQLite database path
}

// NewServer creates an MCP server exposing query tools over the given store.
// In dev mode, additional self-build/self-restart tools are registered.
func NewServer(store core.Store, opts Options) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "ai-workflow",
			Version: "0.1.0",
		},
		nil,
	)
	registerQueryTools(server, store)
	registerSystemInfoTool(server, opts)
	if opts.DevMode {
		registerDevTools(server, opts)
	}
	return server
}

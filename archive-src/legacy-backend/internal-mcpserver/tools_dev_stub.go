//go:build !dev

package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func registerDevTools(_ *mcp.Server, _ Options) {}

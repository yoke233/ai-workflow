package secretary

import (
	"strings"

	"github.com/user/ai-workflow/internal/acpclient"
)

const (
	internalMCPServerCommand = "internal"
	mcpToolEnvKey            = "AI_WORKFLOW_MCP_TOOL"
)

var supportedMCPQueryTools = map[string]struct{}{
	"query_plans":         {},
	"query_plan_detail":   {},
	"query_pipelines":     {},
	"query_pipeline_logs": {},
	"query_project_stats": {},
}

// MCPToolsFromRoleConfig maps role.mcp.tools to NewSessionRequest.MCPServers.
func MCPToolsFromRoleConfig(role acpclient.RoleProfile) []acpclient.MCPServerConfig {
	if len(role.MCPTools) == 0 {
		return nil
	}

	servers := make([]acpclient.MCPServerConfig, 0, len(role.MCPTools))
	seen := make(map[string]struct{}, len(role.MCPTools))

	for _, rawTool := range role.MCPTools {
		tool := strings.TrimSpace(rawTool)
		if tool == "" {
			continue
		}
		if _, ok := supportedMCPQueryTools[tool]; !ok {
			continue
		}
		if _, ok := seen[tool]; ok {
			continue
		}
		seen[tool] = struct{}{}

		servers = append(servers, acpclient.MCPServerConfig{
			Name:    "workflow-query-" + tool,
			Command: internalMCPServerCommand,
			Env: map[string]string{
				mcpToolEnvKey: tool,
			},
		})
	}

	if len(servers) == 0 {
		return nil
	}
	return servers
}

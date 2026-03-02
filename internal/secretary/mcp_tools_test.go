package secretary

import (
	"testing"

	"github.com/user/ai-workflow/internal/acpclient"
)

func TestMCPToolsFromRoleConfig(t *testing.T) {
	role := acpclient.RoleProfile{
		MCPTools: []string{
			" query_plans ",
			"query_plan_detail",
			"query_pipelines",
			"query_pipeline_logs",
			"query_project_stats",
			"query_plans",
			"unknown_tool",
		},
	}

	got := MCPToolsFromRoleConfig(role)
	if len(got) != 5 {
		t.Fatalf("expected 5 mcp servers, got %d", len(got))
	}

	wantByName := map[string]string{
		"workflow-query-query_plans":         "query_plans",
		"workflow-query-query_plan_detail":   "query_plan_detail",
		"workflow-query-query_pipelines":     "query_pipelines",
		"workflow-query-query_pipeline_logs": "query_pipeline_logs",
		"workflow-query-query_project_stats": "query_project_stats",
	}

	for _, server := range got {
		wantTool, ok := wantByName[server.Name]
		if !ok {
			t.Fatalf("unexpected server name: %q", server.Name)
		}
		if server.Command != "internal" {
			t.Fatalf("server %q command = %q, want %q", server.Name, server.Command, "internal")
		}
		if server.Env["AI_WORKFLOW_MCP_TOOL"] != wantTool {
			t.Fatalf("server %q tool = %q, want %q", server.Name, server.Env["AI_WORKFLOW_MCP_TOOL"], wantTool)
		}
	}
}

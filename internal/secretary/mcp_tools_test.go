package secretary

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/acpclient"
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
		if server.Stdio == nil {
			t.Fatalf("expected stdio server, got %#v", server)
		}

		wantTool, ok := wantByName[server.Stdio.Name]
		if !ok {
			t.Fatalf("unexpected server name: %q", server.Stdio.Name)
		}
		if server.Stdio.Command != "internal" {
			t.Fatalf("server %q command = %q, want %q", server.Stdio.Name, server.Stdio.Command, "internal")
		}
		if len(server.Stdio.Env) != 1 || server.Stdio.Env[0].Name != "AI_WORKFLOW_MCP_TOOL" || server.Stdio.Env[0].Value != wantTool {
			t.Fatalf("server %q env = %#v, want AI_WORKFLOW_MCP_TOOL=%q", server.Stdio.Name, server.Stdio.Env, wantTool)
		}
	}
}

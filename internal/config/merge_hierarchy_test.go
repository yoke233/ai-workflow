package config

import "testing"

func TestMergeHierarchy_GlobalProjectPipeline(t *testing.T) {
	global := &Config{
		Agents: AgentsConfig{
			Claude: &AgentConfig{
				Binary:       ptr("claude-global"),
				MaxTurns:     ptr(30),
				Model:        ptr("global-model"),
				DefaultTools: ptrSlice("Read(*)", "Write(*)"),
			},
		},
	}

	project := &ConfigLayer{
		Agents: &AgentsLayer{
			Claude: &AgentConfig{
				MaxTurns: ptr(50),
			},
		},
	}

	override := map[string]any{
		"agents": map[string]any{
			"claude": map[string]any{
				"binary":        "claude-pipeline",
				"default_tools": []any{},
			},
		},
	}

	merged, err := MergeForPipeline(global, project, override)
	if err != nil {
		t.Fatalf("MergeForPipeline returned error: %v", err)
	}

	if merged.Agents.Claude == nil {
		t.Fatal("expected merged agents.claude to be set")
	}
	if merged.Agents.Claude.Binary == nil || *merged.Agents.Claude.Binary != "claude-pipeline" {
		t.Fatalf("expected pipeline override binary, got %v", merged.Agents.Claude.Binary)
	}
	if merged.Agents.Claude.MaxTurns == nil || *merged.Agents.Claude.MaxTurns != 50 {
		t.Fatalf("expected project max_turns=50, got %v", merged.Agents.Claude.MaxTurns)
	}
	if merged.Agents.Claude.Model == nil || *merged.Agents.Claude.Model != "global-model" {
		t.Fatalf("expected nil inheritance for model, got %v", merged.Agents.Claude.Model)
	}
	if merged.Agents.Claude.DefaultTools == nil {
		t.Fatal("expected default_tools to be present after explicit empty override")
	}
	if len(*merged.Agents.Claude.DefaultTools) != 0 {
		t.Fatalf("expected empty array to clear inherited tools, got %v", *merged.Agents.Claude.DefaultTools)
	}
}

func ptrSlice(values ...string) *[]string {
	v := append([]string(nil), values...)
	return &v
}

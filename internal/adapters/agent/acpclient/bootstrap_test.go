package acpclient

import (
	"context"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestPrepareLaunch_AddsClaudeCodeEnvForClaudeACP(t *testing.T) {
	launch, err := PrepareLaunch(context.Background(), BootstrapConfig{
		Profile: &core.AgentProfile{
			Driver: core.DriverConfig{
				LaunchCommand: "npx",
				LaunchArgs:    []string{"-y", "@zed-industries/claude-agent-acp"},
			},
		},
		WorkDir: "D:\\project\\ai-workflow",
	})
	if err != nil {
		t.Fatalf("PrepareLaunch returned error: %v", err)
	}

	value, ok := launch.Env["CLAUDECODE"]
	if !ok {
		t.Fatal("expected CLAUDECODE env to be injected for claude-agent-acp")
	}
	if value != "" {
		t.Fatalf("CLAUDECODE = %q, want empty string", value)
	}
}

func TestPrepareLaunch_KeepsExistingClaudeCodeEnv(t *testing.T) {
	launch, err := PrepareLaunch(context.Background(), BootstrapConfig{
		Profile: &core.AgentProfile{
			Driver: core.DriverConfig{
				LaunchCommand: "npx",
				LaunchArgs:    []string{"-y", "@zed-industries/claude-agent-acp"},
				Env: map[string]string{
					"CLAUDECODE": "1",
				},
			},
		},
		WorkDir: "D:\\project\\ai-workflow",
	})
	if err != nil {
		t.Fatalf("PrepareLaunch returned error: %v", err)
	}

	if launch.Env["CLAUDECODE"] != "1" {
		t.Fatalf("CLAUDECODE = %q, want 1", launch.Env["CLAUDECODE"])
	}
}

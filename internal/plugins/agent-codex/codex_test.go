package agentcodex

import (
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestBuildCommand(t *testing.T) {
	a := New("codex", "gpt-5.3-codex", "high")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:  "fix the bug",
		WorkDir: "/tmp/project",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "exec") {
		t.Error("missing exec subcommand")
	}
	if !strings.Contains(joined, "-a never") {
		t.Error("missing approval flag")
	}
	if !strings.Contains(joined, "--json") {
		t.Error("missing json flag")
	}
	if !strings.Contains(joined, "-m gpt-5.3-codex") {
		t.Error("missing model flag")
	}
}

func TestBuildCommand_WithOutputSchema_DisablesShellTool(t *testing.T) {
	a := New("codex", "gpt-5.3-codex", "high")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:  "emit json",
		WorkDir: "/tmp/project",
		Env: map[string]string{
			"AI_WORKFLOW_CODEX_OUTPUT_SCHEMA": "/tmp/schema.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "--output-schema /tmp/schema.json") {
		t.Error("missing output schema flag")
	}
	if !strings.Contains(joined, "--disable shell_tool") {
		t.Error("missing disable shell_tool flag")
	}
}

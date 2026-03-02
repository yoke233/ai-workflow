package agentcodex

import (
	"context"
	"io"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

type CodexAgent struct {
	binary    string
	model     string
	reasoning string
}

func New(binary, model, reasoning string) *CodexAgent {
	return &CodexAgent{binary: binary, model: model, reasoning: reasoning}
}

func (a *CodexAgent) Name() string {
	return "codex"
}

func (a *CodexAgent) Init(_ context.Context) error {
	return nil
}

func (a *CodexAgent) Close() error {
	return nil
}

func (a *CodexAgent) BuildCommand(opts core.ExecOpts) ([]string, error) {
	prompt := opts.Prompt
	if opts.AppendContext != "" {
		prompt = opts.AppendContext + "\n\n" + prompt
	}

	args := []string{
		// `-a` is a global flag (before `exec`), otherwise `codex exec -a ...` fails.
		a.binary, "-a", "never",
		"exec",
		// Options must come before the prompt when `COMMAND` passthrough exists,
		// otherwise flags can be interpreted as the command to execute.
		"--json",
		"--color", "never",
		"-m", a.model,
		"-c", "model_reasoning_effort=" + a.reasoning,
	}

	// When a schema is supplied (typically for secretary decomposition), force a
	// schema-shaped final response and avoid tool use drifting.
	if opts.Env != nil {
		if schema := opts.Env["AI_WORKFLOW_CODEX_OUTPUT_SCHEMA"]; strings.TrimSpace(schema) != "" {
			args = append(args, "--disable", "shell_tool", "--output-schema", schema)
		}
	}
	if opts.WorkDir != "" {
		args = append(args, "-C", opts.WorkDir)
	}
	// Ensure prompt is parsed as a single positional argument.
	args = append(args, "--", prompt)
	return args, nil
}

func (a *CodexAgent) NewStreamParser(r io.Reader) core.StreamParser {
	return NewCodexStreamParser(r)
}

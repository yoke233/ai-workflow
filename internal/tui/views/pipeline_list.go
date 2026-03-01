package views

import (
	"fmt"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

func RenderPipelineList(pipelines []core.Pipeline, cursor int, styleStatus map[string]func(string) string) string {
	if len(pipelines) == 0 {
		return "No pipelines found. Use `ai-flow pipeline create` to get started.\n"
	}

	var b strings.Builder
	for i, p := range pipelines {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}

		status := string(p.Status)
		if fn, ok := styleStatus[status]; ok {
			status = fn(status)
		}
		currentStage := string(p.CurrentStage)
		if currentStage == "" {
			currentStage = "-"
		}
		b.WriteString(fmt.Sprintf("%s%-12s %-21s %-20s %-16s %s\n", prefix, p.ProjectID, p.ID, p.Name, currentStage, status))
	}
	return b.String()
}

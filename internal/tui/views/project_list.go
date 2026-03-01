package views

import (
	"fmt"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

func RenderProjectList(projects []core.Project, cursor int) string {
	if len(projects) == 0 {
		return "No projects found.\n"
	}

	var b strings.Builder
	for i, p := range projects {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-16s %s\n", prefix, p.ID, p.RepoPath))
	}
	return b.String()
}

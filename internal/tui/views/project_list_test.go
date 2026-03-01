package views

import (
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestRenderProjectListHighlightsCursor(t *testing.T) {
	out := RenderProjectList([]core.Project{
		{ID: "a", RepoPath: "D:/repo/a"},
		{ID: "b", RepoPath: "D:/repo/b"},
	}, 1)

	if !strings.Contains(out, "> b") {
		t.Fatalf("expected cursor to highlight project b, got: %s", out)
	}
}

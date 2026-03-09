package teamleader

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestAreDependenciesMet(t *testing.T) {
	issues := map[string]*core.Issue{
		"A": {ID: "A", Status: core.IssueStatusDone},
		"B": {ID: "B", Status: core.IssueStatusDone},
		"C": {ID: "C", Status: core.IssueStatusExecuting},
	}

	lookup := func(id string) *core.Issue {
		return issues[id]
	}

	if !areDependenciesMet([]string{"A", "B"}, lookup) {
		t.Fatal("expected deps A,B to be met")
	}
	if areDependenciesMet([]string{"A", "C"}, lookup) {
		t.Fatal("expected deps A,C to not be met")
	}
	if !areDependenciesMet(nil, lookup) {
		t.Fatal("expected nil deps to be met")
	}
	if areDependenciesMet([]string{"Z"}, lookup) {
		t.Fatal("expected unknown dep to not be met")
	}
}

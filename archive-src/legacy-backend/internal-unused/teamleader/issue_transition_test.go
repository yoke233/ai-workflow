package teamleader

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestTransitionIssueStatus_AllowsKnownTransition(t *testing.T) {
	issue := &core.Issue{
		ID:     "issue-1",
		Status: core.IssueStatusReviewing,
	}

	if err := transitionIssueStatus(issue, core.IssueStatusQueued); err != nil {
		t.Fatalf("transitionIssueStatus returned error: %v", err)
	}
	if issue.Status != core.IssueStatusQueued {
		t.Fatalf("issue.Status=%q, want queued", issue.Status)
	}
}

func TestTransitionIssueStatus_RejectsInvalidTransition(t *testing.T) {
	issue := &core.Issue{
		ID:     "issue-2",
		Status: core.IssueStatusDraft,
	}

	if err := transitionIssueStatus(issue, core.IssueStatusDone); err == nil {
		t.Fatalf("expected invalid transition error")
	}
	if issue.Status != core.IssueStatusDraft {
		t.Fatalf("issue.Status=%q, want draft", issue.Status)
	}
}

func TestTransitionIssueStatus_AllowsLegacyEmptySource(t *testing.T) {
	issue := &core.Issue{ID: "issue-3"}

	if err := transitionIssueStatus(issue, core.IssueStatusQueued); err != nil {
		t.Fatalf("transitionIssueStatus returned error: %v", err)
	}
	if issue.Status != core.IssueStatusQueued {
		t.Fatalf("issue.Status=%q, want queued", issue.Status)
	}
}

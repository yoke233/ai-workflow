package teamleader

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestTLTriageHandler_MergeConflictRequeuesWithRetryEvent(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	project := mustCreateManagerProject(t, store, "proj-triage-retry")
	issue := &core.Issue{
		ID:           "issue-triage-retry",
		ProjectID:    project.ID,
		Title:        "retry me",
		Body:         "retry me",
		Template:     "standard",
		State:        core.IssueStateOpen,
		Status:       core.IssueStatusMerging,
		RunID:        "run-triage-retry",
		FailPolicy:   core.FailBlock,
		MergeRetries: 0,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	var events []core.Event
	handler := NewTLTriageHandler(store, &capturePublisher{events: &events}, 3)
	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueMergeConflict,
		IssueID: issue.ID,
		RunID:   issue.RunID,
	})

	updated, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if updated.Status != core.IssueStatusQueued {
		t.Fatalf("issue status = %q, want queued", updated.Status)
	}
	if updated.MergeRetries != 1 {
		t.Fatalf("issue MergeRetries = %d, want 1", updated.MergeRetries)
	}
	if updated.RunID != "" {
		t.Fatalf("issue RunID = %q, want empty", updated.RunID)
	}

	if len(events) != 1 {
		t.Fatalf("published events = %d, want 1", len(events))
	}
	if events[0].Type != core.EventIssueMergeRetry {
		t.Fatalf("event type = %q, want %q", events[0].Type, core.EventIssueMergeRetry)
	}
	if events[0].IssueID != issue.ID {
		t.Fatalf("event issue_id = %q, want %q", events[0].IssueID, issue.ID)
	}
	if events[0].RunID != "run-triage-retry" {
		t.Fatalf("event run_id = %q, want %q", events[0].RunID, "run-triage-retry")
	}
}

func TestTLTriageHandler_ThirdConflictMarksFailed(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	project := mustCreateManagerProject(t, store, "proj-triage-failed")
	issue := &core.Issue{
		ID:           "issue-triage-failed",
		ProjectID:    project.ID,
		Title:        "fail me",
		Body:         "fail me",
		Template:     "standard",
		State:        core.IssueStateOpen,
		Status:       core.IssueStatusMerging,
		RunID:        "run-triage-failed",
		FailPolicy:   core.FailBlock,
		MergeRetries: 2,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	var events []core.Event
	handler := NewTLTriageHandler(store, &capturePublisher{events: &events}, 3)
	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueMergeConflict,
		IssueID: issue.ID,
		RunID:   issue.RunID,
	})

	updated, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if updated.Status != core.IssueStatusFailed {
		t.Fatalf("issue status = %q, want failed", updated.Status)
	}
	if updated.MergeRetries != 3 {
		t.Fatalf("issue MergeRetries = %d, want 3", updated.MergeRetries)
	}

	if len(events) != 1 {
		t.Fatalf("published events = %d, want 1", len(events))
	}
	if events[0].Type != core.EventIssueFailed {
		t.Fatalf("event type = %q, want %q", events[0].Type, core.EventIssueFailed)
	}
}

func TestTLTriageHandler_IgnoresNonMergingIssue(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	project := mustCreateManagerProject(t, store, "proj-triage-ignore")
	issue := &core.Issue{
		ID:           "issue-triage-ignore",
		ProjectID:    project.ID,
		Title:        "ignore me",
		Body:         "ignore me",
		Template:     "standard",
		State:        core.IssueStateOpen,
		Status:       core.IssueStatusExecuting,
		RunID:        "run-triage-ignore",
		FailPolicy:   core.FailBlock,
		MergeRetries: 1,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	var events []core.Event
	handler := NewTLTriageHandler(store, &capturePublisher{events: &events}, 3)
	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueMergeConflict,
		IssueID: issue.ID,
		RunID:   issue.RunID,
	})

	updated, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if updated.Status != core.IssueStatusExecuting {
		t.Fatalf("issue status = %q, want executing", updated.Status)
	}
	if updated.MergeRetries != 1 {
		t.Fatalf("issue MergeRetries = %d, want 1", updated.MergeRetries)
	}
	if len(events) != 0 {
		t.Fatalf("published events = %d, want 0", len(events))
	}
}

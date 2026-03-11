package teamleader

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestChildCompletion_AllChildrenDoneClosesParent(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-1", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDecomposed,
		FailPolicy: core.FailBlock,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-a", ProjectID: "proj-1", ParentID: "parent-1",
		Title: "A", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-b", ProjectID: "proj-1", ParentID: "parent-1",
		Title: "B", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueDone,
		IssueID: "child-b",
	})

	parent, _ := store.GetIssue("parent-1")
	if parent.Status != core.IssueStatusDone {
		t.Errorf("parent status = %q, want done", parent.Status)
	}
	if parent.State != core.IssueStateClosed {
		t.Errorf("parent state = %q, want closed", parent.State)
	}
	if parent.ClosedAt == nil {
		t.Error("parent ClosedAt should be set")
	}

	found := false
	for _, evt := range published {
		if evt.Type == core.EventIssueDone && evt.IssueID == "parent-1" {
			found = true
		}
	}
	if !found {
		t.Error("EventIssueDone not published for parent")
	}
}

func TestChildCompletion_PendingChildDoesNotCloseParent(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-2", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDecomposed,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-c", ProjectID: "proj-1", ParentID: "parent-2",
		Title: "C", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-d", ProjectID: "proj-1", ParentID: "parent-2",
		Title: "D", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusExecuting,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueDone,
		IssueID: "child-c",
	})

	parent, _ := store.GetIssue("parent-2")
	if parent.Status != core.IssueStatusDecomposed {
		t.Errorf("parent status = %q, want decomposed (unchanged)", parent.Status)
	}
	if len(published) != 0 {
		t.Error("no events should be published when children still pending")
	}
}

func TestChildCompletion_FailedChildBlockPolicy(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-3", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDecomposed,
		FailPolicy: core.FailBlock,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-e", ProjectID: "proj-1", ParentID: "parent-3",
		Title: "E", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-f", ProjectID: "proj-1", ParentID: "parent-3",
		Title: "F", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusFailed,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueFailed,
		IssueID: "child-f",
	})

	parent, _ := store.GetIssue("parent-3")
	if parent.Status != core.IssueStatusFailed {
		t.Errorf("parent status = %q, want failed", parent.Status)
	}

	found := false
	for _, evt := range published {
		if evt.Type == core.EventIssueFailed && evt.IssueID == "parent-3" {
			found = true
		}
	}
	if !found {
		t.Error("EventIssueFailed not published for parent")
	}
}

func TestChildCompletion_FailedChildSkipPolicy(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-4", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDecomposed,
		FailPolicy: core.FailSkip,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-g", ProjectID: "proj-1", ParentID: "parent-4",
		Title: "G", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-h", ProjectID: "proj-1", ParentID: "parent-4",
		Title: "H", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusFailed,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueFailed,
		IssueID: "child-h",
	})

	parent, _ := store.GetIssue("parent-4")
	if parent.Status != core.IssueStatusDone {
		t.Errorf("parent status = %q, want done (skip policy)", parent.Status)
	}
}

func TestChildCompletion_AbandonedChildSkipPolicy(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-4b", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDecomposed,
		FailPolicy: core.FailSkip,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-gb", ProjectID: "proj-1", ParentID: "parent-4b",
		Title: "G", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-hb", ProjectID: "proj-1", ParentID: "parent-4b",
		Title: "H", Template: "standard", State: core.IssueStateClosed, Status: core.IssueStatusAbandoned,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueDone,
		IssueID: "child-gb",
	})

	parent, _ := store.GetIssue("parent-4b")
	if parent.Status != core.IssueStatusDone {
		t.Errorf("parent status = %q, want done (skip policy with abandoned child)", parent.Status)
	}
}

func TestChildCompletion_IgnoresNonChildIssue(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "standalone", ProjectID: "proj-1", Title: "Regular",
		Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueDone,
		IssueID: "standalone",
	})

	if len(published) != 0 {
		t.Error("no events should be published for standalone issue")
	}
}

func TestChildCompletion_IgnoresNonDecomposedParent(t *testing.T) {
	store := newManagerTestStore(t)
	t.Cleanup(func() { _ = store.Close() })
	store.CreateProject(&core.Project{ID: "proj-1", Name: "svc", RepoPath: t.TempDir()})
	store.CreateIssue(&core.Issue{
		ID: "parent-5", ProjectID: "proj-1", Title: "Epic",
		Template: "epic", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})
	store.CreateIssue(&core.Issue{
		ID: "child-i", ProjectID: "proj-1", ParentID: "parent-5",
		Title: "I", Template: "standard", State: core.IssueStateOpen, Status: core.IssueStatusDone,
	})

	var published []core.Event
	handler := NewChildCompletionHandler(store, &capturePublisher{events: &published})

	handler.OnEvent(context.Background(), core.Event{
		Type:    core.EventIssueDone,
		IssueID: "child-i",
	})

	if len(published) != 0 {
		t.Error("no events should be published when parent not in decomposed state")
	}
}

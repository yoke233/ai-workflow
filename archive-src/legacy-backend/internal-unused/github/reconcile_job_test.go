package github

import (
	"context"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
	storesqlite "github.com/yoke233/ai-workflow/internal/plugins/store-sqlite"
)

func TestReconcileJob_FixesBlockedTaskWhenDependencyAlreadyDone(t *testing.T) {
	store := newReconcileTestStore(t)
	defer store.Close()

	createTestIssues(t, store,
		core.Issue{
			ID:         "issue-upstream",
			Title:      "upstream done",
			Body:       "upstream done",
			Status:     core.IssueStatusDone,
			State:      core.IssueStateClosed,
			ExternalID: "11",
		},
		core.Issue{
			ID:         "issue-downstream",
			Title:      "downstream blocked",
			Body:       "downstream blocked",
			Status:     core.IssueStatusQueued,
			DependsOn:  []string{"issue-upstream"},
			ExternalID: "12",
		},
	)

	job := NewReconcileJob(store, nil)
	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	issue, err := store.GetIssue("issue-downstream")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if issue.Status != core.IssueStatusReady {
		t.Fatalf("issue status = %s, want %s", issue.Status, core.IssueStatusReady)
	}
}

func TestReconcileJob_RepairsIssueLabelDrift(t *testing.T) {
	store := newReconcileTestStore(t)
	defer store.Close()

	createTestIssues(t, store,
		core.Issue{
			ID:         "issue-only",
			Title:      "sync me",
			Body:       "sync me",
			Status:     core.IssueStatusExecuting,
			ExternalID: "22",
		},
	)

	tracker := &fakeTrackerMirror{}
	syncer := NewStatusSyncer(tracker)
	job := NewReconcileJob(store, syncer)
	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if tracker.updateCalls != 1 {
		t.Fatalf("expected 1 update status call, got %d", tracker.updateCalls)
	}
	if tracker.syncDependencyCalls != 1 {
		t.Fatalf("expected 1 sync dependencies call, got %d", tracker.syncDependencyCalls)
	}
}

func TestReconcileJob_MissedWebhookRecoveredWithinInterval(t *testing.T) {
	store := newReconcileTestStore(t)
	defer store.Close()

	now := time.Unix(100, 0)
	recoverCalls := 0

	job := NewReconcileJob(store, nil)
	job.Interval = 10 * time.Minute
	job.Now = func() time.Time { return now }
	job.RecoverMissedWebhook = func(context.Context) error {
		recoverCalls++
		return nil
	}

	ran, err := job.RunIfDue(context.Background())
	if err != nil {
		t.Fatalf("first RunIfDue() error = %v", err)
	}
	if !ran {
		t.Fatal("expected first RunIfDue() to run")
	}

	now = now.Add(5 * time.Minute)
	ran, err = job.RunIfDue(context.Background())
	if err != nil {
		t.Fatalf("second RunIfDue() error = %v", err)
	}
	if ran {
		t.Fatal("expected second RunIfDue() to skip before interval")
	}

	now = now.Add(5 * time.Minute)
	ran, err = job.RunIfDue(context.Background())
	if err != nil {
		t.Fatalf("third RunIfDue() error = %v", err)
	}
	if !ran {
		t.Fatal("expected third RunIfDue() to run at interval")
	}
	if recoverCalls != 2 {
		t.Fatalf("expected recover hook called twice, got %d", recoverCalls)
	}
}

func newReconcileTestStore(t *testing.T) *storesqlite.SQLiteStore {
	t.Helper()
	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	return store
}

func createTestIssues(t *testing.T, store core.Store, issues ...core.Issue) {
	t.Helper()

	project := &core.Project{ID: "proj-reconcile", Name: "proj-reconcile", RepoPath: t.TempDir()}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	for i := range issues {
		issue := issues[i]
		if issue.ProjectID == "" {
			issue.ProjectID = project.ID
		}
		if issue.Template == "" {
			issue.Template = "standard"
		}
		if err := store.SaveIssue(&issue); err != nil {
			t.Fatalf("SaveIssue(%s) error = %v", issue.ID, err)
		}
	}
}

type fakeTrackerMirror struct {
	updateCalls         int
	syncDependencyCalls int
}

func (f *fakeTrackerMirror) UpdateStatus(context.Context, string, core.IssueStatus) error {
	f.updateCalls++
	return nil
}

func (f *fakeTrackerMirror) SyncDependencies(context.Context, *core.Issue, []*core.Issue) error {
	f.syncDependencyCalls++
	return nil
}

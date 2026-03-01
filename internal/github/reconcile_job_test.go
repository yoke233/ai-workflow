package github

import (
	"context"
	"testing"
	"time"

	"github.com/user/ai-workflow/internal/core"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
)

func TestReconcileJob_FixesBlockedTaskWhenDependencyAlreadyDone(t *testing.T) {
	store := newReconcileTestStore(t)
	defer store.Close()

	createTestPlanAndTasks(t, store,
		core.TaskItem{
			ID:          "task-upstream",
			PlanID:      "plan-reconcile-1",
			Title:       "upstream done",
			Description: "upstream done",
			Status:      core.ItemDone,
			ExternalID:  "11",
		},
		core.TaskItem{
			ID:          "task-downstream",
			PlanID:      "plan-reconcile-1",
			Title:       "downstream blocked",
			Description: "downstream blocked",
			Status:      core.ItemBlockedByFailure,
			DependsOn:   []string{"task-upstream"},
			ExternalID:  "12",
		},
	)

	job := NewReconcileJob(store, nil)
	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	task, err := store.GetTaskItem("task-downstream")
	if err != nil {
		t.Fatalf("GetTaskItem() error = %v", err)
	}
	if task.Status != core.ItemReady {
		t.Fatalf("task status = %s, want %s", task.Status, core.ItemReady)
	}
}

func TestReconcileJob_RepairsIssueLabelDrift(t *testing.T) {
	store := newReconcileTestStore(t)
	defer store.Close()

	createTestPlanAndTasks(t, store,
		core.TaskItem{
			ID:          "task-only",
			PlanID:      "plan-reconcile-2",
			Title:       "sync me",
			Description: "sync me",
			Status:      core.ItemRunning,
			ExternalID:  "22",
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

func createTestPlanAndTasks(t *testing.T, store core.Store, tasks ...core.TaskItem) {
	t.Helper()

	project := &core.Project{ID: "proj-reconcile", Name: "proj-reconcile", RepoPath: t.TempDir()}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	plan := &core.TaskPlan{
		ID:         tasks[0].PlanID,
		ProjectID:  project.ID,
		Name:       "reconcile",
		Status:     core.PlanExecuting,
		FailPolicy: core.FailBlock,
	}
	if err := store.SaveTaskPlan(plan); err != nil {
		t.Fatalf("SaveTaskPlan() error = %v", err)
	}

	for i := range tasks {
		task := tasks[i]
		if task.PlanID == "" {
			task.PlanID = plan.ID
		}
		if task.Template == "" {
			task.Template = "standard"
		}
		if err := store.SaveTaskItem(&task); err != nil {
			t.Fatalf("SaveTaskItem(%s) error = %v", task.ID, err)
		}
	}
}

type fakeTrackerMirror struct {
	updateCalls         int
	syncDependencyCalls int
}

func (f *fakeTrackerMirror) UpdateStatus(context.Context, string, core.TaskItemStatus) error {
	f.updateCalls++
	return nil
}

func (f *fakeTrackerMirror) SyncDependencies(context.Context, *core.TaskItem, []core.TaskItem) error {
	f.syncDependencyCalls++
	return nil
}

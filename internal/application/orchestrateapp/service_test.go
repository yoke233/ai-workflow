package orchestrateapp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/application/planning"
	"github.com/yoke233/zhanggui/internal/application/workitemapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type testEnv struct {
	store *sqlite.Store
	svc   *Service
}

type fakePlanner struct {
	dag *planning.GeneratedDAG
	err error
}

func (p *fakePlanner) Generate(context.Context, planning.GenerateInput) (*planning.GeneratedDAG, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.dag, nil
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "orchestrateapp-test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	workItems := workitemapp.New(workitemapp.Config{Store: store})
	svc := New(Config{
		Store:           store,
		WorkItemCreator: workItems,
		Planner: &fakePlanner{dag: &planning.GeneratedDAG{
			Actions: []planning.GeneratedAction{
				{Name: "implement", Type: "exec", AgentRole: "worker"},
			},
		}},
	})
	return &testEnv{store: store, svc: svc}
}

func TestServiceCreateTaskReturnsExistingOpenWorkItemForSameDedupeKey(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	first, err := env.svc.CreateTask(context.Background(), CreateTaskInput{
		Title:     "CEO bootstrap",
		DedupeKey: "chat:42:goal:bootstrap",
	})
	if err != nil {
		t.Fatalf("CreateTask(first) error = %v", err)
	}

	second, err := env.svc.CreateTask(context.Background(), CreateTaskInput{
		Title:     "CEO bootstrap",
		DedupeKey: "chat:42:goal:bootstrap",
	})
	if err != nil {
		t.Fatalf("CreateTask(second) error = %v", err)
	}

	if second.WorkItem.ID != first.WorkItem.ID {
		t.Fatalf("CreateTask(second).WorkItem.ID = %d, want %d", second.WorkItem.ID, first.WorkItem.ID)
	}
	if second.Created {
		t.Fatal("CreateTask(second).Created = true, want false")
	}
}

func TestServiceFollowUpTaskReturnsAssignedProfileAndNextStep(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	workItemID, err := env.store.CreateWorkItem(context.Background(), &core.WorkItem{
		Title:    "assigned task",
		Status:   core.WorkItemOpen,
		Priority: core.PriorityMedium,
		Metadata: map[string]any{
			"ceo": map[string]any{"assigned_profile": "worker"},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	actionID, err := env.store.CreateAction(context.Background(), &core.Action{
		WorkItemID: workItemID,
		Name:       "implement",
		Type:       core.ActionExec,
		Status:     core.ActionPending,
		Position:   0,
	})
	if err != nil {
		t.Fatalf("CreateAction() error = %v", err)
	}
	_, err = env.store.CreateRun(context.Background(), &core.Run{
		ActionID:       actionID,
		WorkItemID:     workItemID,
		Status:         core.RunSucceeded,
		Attempt:        1,
		ResultMarkdown: "Implemented initial version successfully",
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := env.svc.FollowUpTask(context.Background(), FollowUpTaskInput{WorkItemID: workItemID})
	if err != nil {
		t.Fatalf("FollowUpTask() error = %v", err)
	}
	if result.AssignedProfile != "worker" {
		t.Fatalf("AssignedProfile = %q, want worker", result.AssignedProfile)
	}
	if result.RecommendedNextStep != "run_work_item" {
		t.Fatalf("RecommendedNextStep = %q, want run_work_item", result.RecommendedNextStep)
	}
	if result.LatestRunSummary == "" {
		t.Fatal("LatestRunSummary is empty, want non-empty summary")
	}
}

func TestServiceReassignAppendsCEOJournal(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	workItemID, err := env.store.CreateWorkItem(context.Background(), &core.WorkItem{
		Title:    "reassign me",
		Status:   core.WorkItemOpen,
		Priority: core.PriorityMedium,
		Metadata: map[string]any{
			"ceo": map[string]any{"assigned_profile": "planner"},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	result, err := env.svc.ReassignTask(context.Background(), ReassignTaskInput{
		WorkItemID:    workItemID,
		NewProfile:    "worker",
		Reason:        "planner stalled",
		ActorProfile:  "ceo",
		SourceSession: "chat-42",
	})
	if err != nil {
		t.Fatalf("ReassignTask() error = %v", err)
	}
	if result.OldProfile != "planner" || result.NewProfile != "worker" {
		t.Fatalf("unexpected reassign result: %+v", result)
	}
	if len(result.JournalEntries) != 1 {
		t.Fatalf("JournalEntries len = %d, want 1", len(result.JournalEntries))
	}

	workItem, err := env.store.GetWorkItem(context.Background(), workItemID)
	if err != nil {
		t.Fatalf("GetWorkItem() error = %v", err)
	}
	if got := metadataValue(workItem.Metadata, "ceo", "assigned_profile"); got != "worker" {
		t.Fatalf("assigned profile = %q, want worker", got)
	}
	journal, ok := workItem.Metadata["ceo_journal"].([]any)
	if !ok || len(journal) != 1 {
		t.Fatalf("ceo_journal = %#v, want single entry", workItem.Metadata["ceo_journal"])
	}
}

func TestServiceDecomposeRejectsOverwriteWhenActiveActionsExist(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	workItemID, err := env.store.CreateWorkItem(context.Background(), &core.WorkItem{
		Title:    "replan me",
		Status:   core.WorkItemOpen,
		Priority: core.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	_, err = env.store.CreateAction(context.Background(), &core.Action{
		WorkItemID: workItemID,
		Name:       "running-action",
		Type:       core.ActionExec,
		Status:     core.ActionRunning,
		Position:   0,
	})
	if err != nil {
		t.Fatalf("CreateAction() error = %v", err)
	}

	_, err = env.svc.DecomposeTask(context.Background(), DecomposeTaskInput{
		WorkItemID:        workItemID,
		Objective:         "replan",
		OverwriteExisting: true,
	})
	if CodeOf(err) != CodeDecomposeConflict {
		t.Fatalf("CodeOf(err) = %q, want %q (err=%v)", CodeOf(err), CodeDecomposeConflict, err)
	}
}

func TestServiceDecomposePropagatesAssignedProfileToCreatedActions(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	workItemID, err := env.store.CreateWorkItem(context.Background(), &core.WorkItem{
		Title:    "assigned decompose",
		Status:   core.WorkItemOpen,
		Priority: core.PriorityMedium,
		Metadata: map[string]any{
			"ceo": map[string]any{"assigned_profile": "worker-b"},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}

	result, err := env.svc.DecomposeTask(context.Background(), DecomposeTaskInput{
		WorkItemID: workItemID,
		Objective:  "build implementation plan",
	})
	if err != nil {
		t.Fatalf("DecomposeTask() error = %v", err)
	}
	if result.ActionCount != 1 {
		t.Fatalf("ActionCount = %d, want 1", result.ActionCount)
	}
	actions, err := env.store.ListActionsByWorkItem(context.Background(), workItemID)
	if err != nil {
		t.Fatalf("ListActionsByWorkItem() error = %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(actions))
	}
	if actions[0].Config["preferred_profile_id"] != "worker-b" {
		t.Fatalf("preferred_profile_id = %v, want worker-b", actions[0].Config["preferred_profile_id"])
	}
}

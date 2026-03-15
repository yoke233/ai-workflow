package flow

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

// --- Position-mode tests (backward compatibility) ---

func TestEntryActions_ByPosition(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A", Position: 0},
		{ID: 2, Name: "B", Position: 1},
		{ID: 3, Name: "C", Position: 2},
	}
	entries := EntryActions(actions)
	if len(entries) != 1 || entries[0].ID != 1 {
		t.Fatalf("expected only action 1 entry, got %v", entries)
	}
}

func TestPromotableActions_ByPosition(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A", Status: core.ActionDone, Position: 0},
		{ID: 2, Name: "B", Status: core.ActionPending, Position: 1},
		{ID: 3, Name: "C", Status: core.ActionPending, Position: 2},
	}
	promotable := PromotableActions(actions)
	if len(promotable) != 1 || promotable[0].ID != 2 {
		t.Fatalf("expected only action 2 promotable, got %v", promotable)
	}
}

func TestRunnableActions(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A", Status: core.ActionReady},
		{ID: 2, Name: "B", Status: core.ActionPending},
		{ID: 3, Name: "C", Status: core.ActionReady},
	}
	runnable := RunnableActions(actions)
	if len(runnable) != 2 {
		t.Fatalf("expected 2 runnable, got %d", len(runnable))
	}
}

func TestPredecessorActionIDs(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Position: 0},
		{ID: 2, Position: 1},
		{ID: 3, Position: 2},
	}
	ids := predecessorActionIDs(actions, actions[2])
	if len(ids) != 2 {
		t.Fatalf("expected 2 predecessors, got %d", len(ids))
	}
}

func TestImmediatePredecessorActionIDs(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Position: 0},
		{ID: 2, Position: 1},
		{ID: 3, Position: 2},
	}
	ids := immediatePredecessorActionIDs(actions, actions[2])
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only action 2 as immediate predecessor, got %v", ids)
	}
}

func TestValidateActions_RejectsDuplicatePosition(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Position: 0},
		{ID: 2, Position: 0},
	}
	if err := ValidateActions(actions); err == nil {
		t.Fatal("expected duplicate position validation error")
	}
}

func TestValidateActions_RejectsNegativePosition(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Position: -1},
	}
	if err := ValidateActions(actions); err == nil {
		t.Fatal("expected negative position validation error")
	}
}

// --- DAG-mode tests ---

func TestHasDependsOn(t *testing.T) {
	t.Run("no deps", func(t *testing.T) {
		actions := []*core.Action{
			{ID: 1}, {ID: 2},
		}
		if hasDependsOn(actions) {
			t.Fatal("expected false when no action has DependsOn")
		}
	})

	t.Run("empty deps slice", func(t *testing.T) {
		actions := []*core.Action{
			{ID: 1, DependsOn: []int64{}},
			{ID: 2},
		}
		if hasDependsOn(actions) {
			t.Fatal("expected false when DependsOn is empty slice")
		}
	})

	t.Run("has deps", func(t *testing.T) {
		actions := []*core.Action{
			{ID: 1},
			{ID: 2, DependsOn: []int64{1}},
		}
		if !hasDependsOn(actions) {
			t.Fatal("expected true when any action has DependsOn")
		}
	})
}

func TestEntryActions_ByDependsOn(t *testing.T) {
	// A has no deps (entry), B depends on A, C depends on A.
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{1}},
	}
	entries := EntryActions(actions)
	if len(entries) != 1 || entries[0].ID != 1 {
		t.Fatalf("expected only action A as entry, got %v", entries)
	}
}

func TestEntryActions_ByDependsOn_MultipleRoots(t *testing.T) {
	// A and B are both roots, C depends on both.
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B"},
		{ID: 3, Name: "C", DependsOn: []int64{1, 2}},
	}
	entries := EntryActions(actions)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entry actions, got %d", len(entries))
	}
}

func TestPromotableActions_FanOut(t *testing.T) {
	// A → {B, C}. A done → both B and C promotable.
	actions := []*core.Action{
		{ID: 1, Name: "A", Status: core.ActionDone},
		{ID: 2, Name: "B", Status: core.ActionPending, DependsOn: []int64{1}},
		{ID: 3, Name: "C", Status: core.ActionPending, DependsOn: []int64{1}},
	}
	promotable := PromotableActions(actions)
	if len(promotable) != 2 {
		t.Fatalf("expected 2 promotable (B,C), got %d", len(promotable))
	}
}

func TestPromotableActions_Diamond(t *testing.T) {
	// A → {B, C} → D. B done, C not done → D not promotable.
	actions := []*core.Action{
		{ID: 1, Name: "A", Status: core.ActionDone},
		{ID: 2, Name: "B", Status: core.ActionDone, DependsOn: []int64{1}},
		{ID: 3, Name: "C", Status: core.ActionPending, DependsOn: []int64{1}},
		{ID: 4, Name: "D", Status: core.ActionPending, DependsOn: []int64{2, 3}},
	}
	promotable := PromotableActions(actions)
	// Only C should be promotable (A is done, C's dep A is done).
	// D should NOT be promotable (C is not done).
	if len(promotable) != 1 || promotable[0].ID != 3 {
		ids := make([]int64, len(promotable))
		for i, p := range promotable {
			ids[i] = p.ID
		}
		t.Fatalf("expected only action C (3) promotable, got %v", ids)
	}
}

func TestPromotableActions_Diamond_AllReady(t *testing.T) {
	// A → {B, C} → D. A, B, C all done → D promotable.
	actions := []*core.Action{
		{ID: 1, Name: "A", Status: core.ActionDone},
		{ID: 2, Name: "B", Status: core.ActionDone, DependsOn: []int64{1}},
		{ID: 3, Name: "C", Status: core.ActionDone, DependsOn: []int64{1}},
		{ID: 4, Name: "D", Status: core.ActionPending, DependsOn: []int64{2, 3}},
	}
	promotable := PromotableActions(actions)
	if len(promotable) != 1 || promotable[0].ID != 4 {
		t.Fatalf("expected only action D (4) promotable, got %v", promotable)
	}
}

func TestPredecessorActionIDs_DAG(t *testing.T) {
	// A → B → C. Predecessors of C should be {A, B} (transitive closure).
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{2}},
	}
	ids := predecessorActionIDs(actions, actions[2])
	idSet := make(map[int64]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet[1] || !idSet[2] {
		t.Fatalf("expected predecessors {1, 2}, got %v", ids)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 predecessors, got %d", len(ids))
	}
}

func TestImmediatePredecessorActionIDs_DAG(t *testing.T) {
	// A → B → C. Immediate predecessors of C should be {B} only.
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{2}},
	}
	ids := immediatePredecessorActionIDs(actions, actions[2])
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only action 2 as immediate predecessor, got %v", ids)
	}
}

func TestImmediatePredecessorActionIDs_DAG_FanIn(t *testing.T) {
	// A, B → C. Immediate predecessors of C should be {A, B}.
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B"},
		{ID: 3, Name: "C", DependsOn: []int64{1, 2}},
	}
	ids := immediatePredecessorActionIDs(actions, actions[2])
	if len(ids) != 2 {
		t.Fatalf("expected 2 immediate predecessors, got %v", ids)
	}
}

func TestValidateActions_DAG_Valid(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{1}},
		{ID: 4, Name: "D", DependsOn: []int64{2, 3}},
	}
	if err := ValidateActions(actions); err != nil {
		t.Fatalf("expected valid DAG, got error: %v", err)
	}
}

func TestValidateActions_DAG_Cycle(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A", DependsOn: []int64{3}},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{2}},
	}
	if err := ValidateActions(actions); err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestValidateActions_DAG_SelfDep(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A", DependsOn: []int64{1}},
	}
	if err := ValidateActions(actions); err == nil {
		t.Fatal("expected self-dependency error")
	}
}

func TestValidateActions_DAG_MissingDep(t *testing.T) {
	actions := []*core.Action{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{999}},
	}
	if err := ValidateActions(actions); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestValidateActions_DAG_DupPositionOK(t *testing.T) {
	// In DAG mode, duplicate positions are allowed (Position is just a display hint).
	actions := []*core.Action{
		{ID: 1, Name: "A", Position: 0},
		{ID: 2, Name: "B", Position: 0, DependsOn: []int64{1}},
	}
	if err := ValidateActions(actions); err != nil {
		t.Fatalf("DAG mode should allow duplicate positions, got error: %v", err)
	}
}

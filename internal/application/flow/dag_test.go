package flow

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestValidateDAG_NoCycle(t *testing.T) {
	steps := []*core.Step{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{1}},
		{ID: 4, Name: "D", DependsOn: []int64{2, 3}},
	}
	if err := ValidateDAG(steps); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	steps := []*core.Step{
		{ID: 1, Name: "A", DependsOn: []int64{3}},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C", DependsOn: []int64{2}},
	}
	if err := ValidateDAG(steps); err != core.ErrCycleDetected {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestEntrySteps(t *testing.T) {
	steps := []*core.Step{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B", DependsOn: []int64{1}},
		{ID: 3, Name: "C"},
	}
	entries := EntrySteps(steps)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestPromotableSteps(t *testing.T) {
	steps := []*core.Step{
		{ID: 1, Name: "A", Status: core.StepDone},
		{ID: 2, Name: "B", Status: core.StepPending, DependsOn: []int64{1}},
		{ID: 3, Name: "C", Status: core.StepPending, DependsOn: []int64{1, 2}},
	}
	promotable := PromotableSteps(steps)
	if len(promotable) != 1 || promotable[0].ID != 2 {
		t.Fatalf("expected only step 2 promotable, got %v", promotable)
	}
}


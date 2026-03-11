package storesqlite

import (
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestDecisionCRUD(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	d := &core.Decision{
		ID:            core.NewDecisionID(),
		IssueID:       "issue-dec-test",
		RunID:         "run-1",
		StageID:       "implement",
		AgentID:       "agent-1",
		Type:          core.DecisionTypeGateCheck,
		PromptHash:    core.PromptHash("test prompt"),
		PromptPreview: "test prompt",
		Model:         "claude-3",
		Template:      "review",
		Action:        "pass",
		Reasoning:     "looks good",
		Confidence:    0.95,
		OutputData:    "{}",
		CreatedAt:     time.Now(),
	}

	if err := store.SaveDecision(d); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	got, err := store.GetDecision(d.ID)
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if got.Action != "pass" {
		t.Errorf("expected action 'pass', got %q", got.Action)
	}
	if got.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", got.Confidence)
	}

	list, err := store.ListDecisions("issue-dec-test")
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(list))
	}

	// nonexistent
	_, err = store.GetDecision("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent decision")
	}
}

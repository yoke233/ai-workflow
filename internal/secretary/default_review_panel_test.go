package secretary

import (
	"context"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestNewDefaultReviewPanelApproveFlow(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewPanel(store)

	plan := &core.TaskPlan{
		ID:         "plan-default-review-approve",
		ProjectID:  "proj-1",
		Name:       "demo",
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
		Tasks: []core.TaskItem{
			{
				ID:          "task-1",
				PlanID:      "plan-default-review-approve",
				Title:       "task one",
				Description: "task one description",
				Template:    "standard",
			},
		},
	}

	result, err := panel.Run(context.Background(), plan, ReviewInput{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}
	if result.Plan.Status != core.PlanWaitingHuman {
		t.Fatalf("status = %q, want %q", result.Plan.Status, core.PlanWaitingHuman)
	}
	if result.Plan.WaitReason != core.WaitFinalApproval {
		t.Fatalf("wait_reason = %q, want %q", result.Plan.WaitReason, core.WaitFinalApproval)
	}
}

func TestNewDefaultReviewPanelEscalateFlow(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewPanel(store)

	plan := &core.TaskPlan{
		ID:         "plan-default-review-escalate",
		ProjectID:  "proj-1",
		Name:       "demo",
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
		Tasks: []core.TaskItem{
			{
				ID:          "task-1",
				PlanID:      "plan-default-review-escalate",
				Title:       "task one",
				Description: "task one description",
				Template:    "custom-template",
			},
		},
	}

	result, err := panel.Run(context.Background(), plan, ReviewInput{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionEscalate {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionEscalate)
	}
	if result.Plan.Status != core.PlanWaitingHuman {
		t.Fatalf("status = %q, want %q", result.Plan.Status, core.PlanWaitingHuman)
	}
	if result.Plan.WaitReason != core.WaitFeedbackReq {
		t.Fatalf("wait_reason = %q, want %q", result.Plan.WaitReason, core.WaitFeedbackReq)
	}
}

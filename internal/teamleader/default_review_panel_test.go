package teamleader

import (
	"context"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

func TestNewDefaultReviewOrchestratorApproveFlow(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)
	panel.Reviewer = stubDemandReviewer{
		fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 96}, nil
		},
	}

	issues := []*core.Issue{
		newReviewTestIssue("issue-default-review-approve"),
	}
	result, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}
	if result.Status != core.ReviewStatusApproved {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusApproved)
	}
	if !result.AutoApproved {
		t.Fatal("auto_approved = false, want true")
	}
	if len(result.Verdicts) != 1 {
		t.Fatalf("verdict count = %d, want 1", len(result.Verdicts))
	}
	if _, ok := result.Verdicts[issues[0].ID]; !ok {
		t.Fatalf("verdict missing issue id %q", issues[0].ID)
	}
}

func TestNewDefaultReviewOrchestratorEscalateFlow(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)
	panel.Reviewer = stubDemandReviewer{
		fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 93}, nil
		},
	}
	panel.Analyzer = stubDependencyAnalyzer{
		fn: func(_ context.Context, issues []*core.Issue) (*DependencyAnalysis, error) {
			return &DependencyAnalysis{
				Conflicts: []ConflictInfo{
					{
						IssueIDs: []string{issues[0].ID, issues[1].ID},
						Resource: "shared-env",
					},
				},
			}, nil
		},
	}

	issues := []*core.Issue{
		newReviewTestIssue("issue-default-review-escalate-1"),
		newReviewTestIssue("issue-default-review-escalate-2"),
	}
	result, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionEscalate {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionEscalate)
	}
	if result.Status != core.ReviewStatusRejected {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusRejected)
	}
	if result.AutoApproved {
		t.Fatal("auto_approved = true, want false")
	}
	if result.DAG == nil || len(result.DAG.Conflicts) != 1 {
		t.Fatalf("dag conflicts = %d, want 1", len(result.DAG.Conflicts))
	}
}

func TestNewDefaultReviewOrchestratorThresholdTriggersFixFlow(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)
	panel.Reviewer = stubDemandReviewer{
		fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 72}, nil
		},
	}
	panel.AutoApproveThreshold = 80

	issues := []*core.Issue{
		newReviewTestIssue("issue-default-review-threshold"),
	}
	result, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionFix {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionFix)
	}
	if result.Status != core.ReviewStatusChangesRequested {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusChangesRequested)
	}
	if result.AutoApproved {
		t.Fatal("auto_approved = true, want false")
	}
}

func TestNewDefaultReviewOrchestratorRequiresNonNilIssueEntries(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)
	panel.Reviewer = stubDemandReviewer{}

	_, err := panel.Run(context.Background(), []*core.Issue{nil})
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "issue[0] is nil") {
		t.Fatalf("error = %q, want contains %q", err.Error(), "issue[0] is nil")
	}
}

func TestNewDefaultReviewOrchestratorRejectsDuplicateIssueIDs(t *testing.T) {
	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)
	panel.Reviewer = stubDemandReviewer{}

	issues := []*core.Issue{
		newReviewTestIssue("issue-default-review-dup"),
		newReviewTestIssue("issue-default-review-dup"),
	}
	_, err := panel.Run(context.Background(), issues)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate issue id") {
		t.Fatalf("error = %q, want contains %q", err.Error(), "duplicate issue id")
	}
}

func TestNewDefaultReviewOrchestratorFromBindingsUsesResolvedRuntime(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	resolver := acpclient.NewRoleResolver(
		[]acpclient.AgentProfile{
			{
				ID: "codex",
				CapabilitiesMax: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]acpclient.RoleProfile{
			{
				ID:      "senior-reviewer",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:       false,
					ResetPrompt: false,
				},
			},
			{
				ID:      "chief-aggregator",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:       false,
					ResetPrompt: false,
				},
			},
		},
	)

	panel, err := NewDefaultReviewOrchestratorFromBindings(store, ReviewRoleBindingInput{
		Reviewers: map[string]string{
			"completeness": "senior-reviewer",
			"dependency":   "senior-reviewer",
			"feasibility":  "senior-reviewer",
		},
		Aggregator: "chief-aggregator",
	}, resolver)
	if err != nil {
		t.Fatalf("NewDefaultReviewOrchestratorFromBindings() error = %v", err)
	}
	if panel.RoleRuntime == nil {
		t.Fatal("expected role runtime on review panel")
	}

	for _, reviewer := range []string{"completeness", "dependency", "feasibility"} {
		if got := panel.RoleRuntime.ReviewerRoles[reviewer]; got != "senior-reviewer" {
			t.Fatalf("role runtime reviewer role %s = %q, want %q", reviewer, got, "senior-reviewer")
		}
		policy := panel.RoleRuntime.ReviewerSessionPolicies[reviewer]
		if !policy.Reuse {
			t.Fatalf("role runtime reviewer %s should default reuse=true", reviewer)
		}
		if !policy.ResetPrompt {
			t.Fatalf("role runtime reviewer %s should default reset_prompt=true", reviewer)
		}
	}

	if got := panel.RoleRuntime.AggregatorRole; got != "chief-aggregator" {
		t.Fatalf("role runtime aggregator role = %q, want %q", got, "chief-aggregator")
	}
	if !panel.RoleRuntime.AggregatorSessionPolicy.Reuse {
		t.Fatal("role runtime aggregator should default reuse=true")
	}
	if !panel.RoleRuntime.AggregatorSessionPolicy.ResetPrompt {
		t.Fatal("role runtime aggregator should default reset_prompt=true")
	}
}

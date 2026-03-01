package secretary

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

const (
	defaultCompletenessReviewerName = "completeness"
	defaultDependencyReviewerName   = "dependency"
	defaultFeasibilityReviewerName  = "feasibility"
)

// NewDefaultReviewPanel builds a runnable review panel with rule-based reviewers.
// This keeps P2 production path working without requiring external AI backends.
func NewDefaultReviewPanel(store ReviewStore) *ReviewPanel {
	return &ReviewPanel{
		Store: store,
		Reviewers: []Reviewer{
			newCompletenessReviewer(),
			newDependencyReviewer(),
			newFeasibilityReviewer(),
		},
		Aggregator: newRuleAggregator(),
		MaxRounds:  defaultReviewMaxRounds,
	}
}

type ruleReviewer struct {
	name string
	run  func(plan *core.TaskPlan) []core.ReviewIssue
}

func (r ruleReviewer) Name() string {
	return r.name
}

func (r ruleReviewer) Review(_ context.Context, input ReviewerInput) (core.ReviewVerdict, error) {
	if input.Plan == nil {
		return core.ReviewVerdict{}, fmt.Errorf("reviewer %s: plan is nil", r.name)
	}
	issues := r.run(input.Plan)
	status := "pass"
	score := 100
	if len(issues) > 0 {
		status = "issues_found"
		score = 60
	}
	return core.ReviewVerdict{
		Reviewer: r.name,
		Status:   status,
		Issues:   issues,
		Score:    score,
	}, nil
}

func newCompletenessReviewer() Reviewer {
	return ruleReviewer{
		name: defaultCompletenessReviewerName,
		run: func(plan *core.TaskPlan) []core.ReviewIssue {
			issues := make([]core.ReviewIssue, 0)
			if len(plan.Tasks) == 0 {
				return []core.ReviewIssue{
					{
						Severity:    "error",
						Description: "task plan has no tasks",
						Suggestion:  "add at least one executable task",
						TaskID:      "",
					},
				}
			}

			for i := range plan.Tasks {
				task := plan.Tasks[i]
				if strings.TrimSpace(task.Title) == "" {
					issues = append(issues, core.ReviewIssue{
						Severity:    "error",
						Description: "task title is required",
						Suggestion:  "provide a clear task title",
						TaskID:      strings.TrimSpace(task.ID),
					})
				}
				if strings.TrimSpace(task.Description) == "" {
					issues = append(issues, core.ReviewIssue{
						Severity:    "error",
						Description: "task description is required",
						Suggestion:  "provide acceptance criteria in description",
						TaskID:      strings.TrimSpace(task.ID),
					})
				}
			}

			return issues
		},
	}
}

func newDependencyReviewer() Reviewer {
	return ruleReviewer{
		name: defaultDependencyReviewerName,
		run: func(plan *core.TaskPlan) []core.ReviewIssue {
			dag := Build(plan.Tasks)
			if err := dag.Validate(); err != nil {
				return []core.ReviewIssue{
					{
						Severity:    "error",
						Description: err.Error(),
						Suggestion:  "fix dependency graph to satisfy DAG constraints",
					},
				}
			}
			return nil
		},
	}
}

func newFeasibilityReviewer() Reviewer {
	return ruleReviewer{
		name: defaultFeasibilityReviewerName,
		run: func(plan *core.TaskPlan) []core.ReviewIssue {
			issues := make([]core.ReviewIssue, 0)
			for i := range plan.Tasks {
				task := plan.Tasks[i]
				template := strings.TrimSpace(task.Template)
				if template == "" {
					continue
				}
				if _, ok := allowedPipelineTemplates[template]; ok {
					continue
				}
				issues = append(issues, core.ReviewIssue{
					Severity:    "warning",
					Description: fmt.Sprintf("unknown template %q", template),
					Suggestion:  "use one of: full/standard/quick/hotfix",
					TaskID:      strings.TrimSpace(task.ID),
				})
			}
			return issues
		},
	}
}

var allowedPipelineTemplates = map[string]struct{}{
	"full":     {},
	"standard": {},
	"quick":    {},
	"hotfix":   {},
}

type ruleAggregator struct{}

func newRuleAggregator() Aggregator {
	return ruleAggregator{}
}

func (a ruleAggregator) Decide(_ context.Context, input AggregatorInput) (AggregatorDecision, error) {
	allIssues := collectAllIssues(input.Verdicts)
	if len(allIssues) == 0 {
		return AggregatorDecision{
			Decision: DecisionApprove,
		}, nil
	}

	reason := "review issues found"
	for i := range allIssues {
		issue := allIssues[i]
		if strings.TrimSpace(issue.Description) != "" {
			reason = issue.Description
			break
		}
	}

	// V1 runtime path: when issues exist we escalate to human feedback directly.
	return AggregatorDecision{
		Decision: DecisionEscalate,
		Reason:   reason,
	}, nil
}

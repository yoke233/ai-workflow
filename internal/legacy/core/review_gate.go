package core

import "context"

// ReviewGate performs Issue review and returns review decisions.
type ReviewGate interface {
	Plugin
	Submit(ctx context.Context, issues []*Issue) (reviewID string, err error)
	Check(ctx context.Context, reviewID string) (*ReviewResult, error)
	Cancel(ctx context.Context, reviewID string) error
}

const (
	ReviewStatusPending          = "pending"
	ReviewStatusApproved         = "approved"
	ReviewStatusRejected         = "rejected"
	ReviewStatusChangesRequested = "changes_requested"
	ReviewStatusCancelled        = "cancelled"
)

const (
	ReviewDecisionPending   = "pending"
	ReviewDecisionApprove   = "approve"
	ReviewDecisionReject    = "reject"
	ReviewDecisionFix       = "fix"
	ReviewDecisionCancelled = "cancelled"
)

type ReviewResult struct {
	Status   string          `json:"status"`
	Verdicts []ReviewVerdict `json:"verdicts"`
	Decision string          `json:"decision"`
	Comments []string        `json:"comments,omitempty"`
}

type ReviewVerdict struct {
	Reviewer  string        `json:"reviewer"`
	Status    string        `json:"status"`
	Summary   string        `json:"summary,omitempty"`
	RawOutput string        `json:"raw_output,omitempty"`
	Issues    []ReviewIssue `json:"issues"`
	Score     int           `json:"score"`
}

type ReviewIssue struct {
	Severity    string `json:"severity"`
	IssueID     string `json:"issue_id"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

package core

import "time"

// ReviewRecord stores one persisted reviewer/aggregator output for audit trail.
type ReviewRecord struct {
	ID        int64         `json:"id"`
	IssueID   string        `json:"issue_id"`
	Round     int           `json:"round"`
	Reviewer  string        `json:"reviewer"`
	Verdict   string        `json:"verdict"`
	Summary   string        `json:"summary,omitempty"`
	RawOutput string        `json:"raw_output,omitempty"`
	Issues    []ReviewIssue `json:"issues"`
	Fixes     []ProposedFix `json:"fixes"`
	Score     *int          `json:"score,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// ProposedFix is a normalized shape for review-driven issue adjustments.
type ProposedFix struct {
	IssueID     string `json:"issue_id,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

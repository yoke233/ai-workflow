package core

import (
	"context"
	"time"
)

// IssueStatus represents the unified lifecycle state of an Issue.
// It covers both planning (open/accepted) and execution (queued/running/done/failed).
type IssueStatus string

const (
	IssueOpen      IssueStatus = "open"
	IssueAccepted  IssueStatus = "accepted"
	IssueQueued    IssueStatus = "queued"
	IssueRunning   IssueStatus = "running"
	IssueBlocked   IssueStatus = "blocked"
	IssueFailed    IssueStatus = "failed"
	IssueDone      IssueStatus = "done"
	IssueCancelled IssueStatus = "cancelled"
	IssueClosed    IssueStatus = "closed"
)

// IssuePriority represents the urgency of an Issue.
type IssuePriority string

const (
	PriorityLow    IssuePriority = "low"
	PriorityMedium IssuePriority = "medium"
	PriorityHigh   IssuePriority = "high"
	PriorityUrgent IssuePriority = "urgent"
)

// Issue is the unified work unit: it combines the planning intent (title, body,
// priority, labels) with the execution context (status lifecycle, steps, workspace).
// It replaces the former Flow + Issue pair.
//
// An Issue optionally belongs to a Project and can be bound to a specific
// ResourceBinding (repo) for workspace isolation. Issues may declare
// dependencies on other Issues via DependsOn, forming a DAG at the project level.
type Issue struct {
	ID                int64  `json:"id"`
	ProjectID         *int64 `json:"project_id,omitempty"`
	ResourceBindingID *int64 `json:"resource_binding_id,omitempty"` // which repo/resource to work on

	// Planning fields
	Title    string        `json:"title"`
	Body     string        `json:"body"`
	Priority IssuePriority `json:"priority"`
	Labels   []string      `json:"labels,omitempty"`

	// DAG: issue-level dependencies (replaces step-level DependsOn)
	DependsOn []int64 `json:"depends_on,omitempty"`

	// Execution fields (absorbed from former Flow)
	Status   IssueStatus    `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`

	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// IssueFilter constrains Issue queries.
type IssueFilter struct {
	ProjectID *int64
	Status    *IssueStatus
	Priority  *IssuePriority
	Archived  *bool
	Limit     int
	Offset    int
}

// IssueStore persists Issue aggregates.
type IssueStore interface {
	CreateIssue(ctx context.Context, issue *Issue) (int64, error)
	GetIssue(ctx context.Context, id int64) (*Issue, error)
	ListIssues(ctx context.Context, filter IssueFilter) ([]*Issue, error)
	UpdateIssue(ctx context.Context, issue *Issue) error
	UpdateIssueStatus(ctx context.Context, id int64, status IssueStatus) error
	UpdateIssueMetadata(ctx context.Context, id int64, metadata map[string]any) error
	PrepareIssueRun(ctx context.Context, id int64, queuedStatus IssueStatus) error
	SetIssueArchived(ctx context.Context, id int64, archived bool) error
	DeleteIssue(ctx context.Context, id int64) error
}

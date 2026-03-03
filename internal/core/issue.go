package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// IssueState represents whether the issue is open or closed in tracker terms.
type IssueState string

const (
	IssueStateOpen   IssueState = "open"
	IssueStateClosed IssueState = "closed"
)

// IssueStatus represents internal issue lifecycle status.
type IssueStatus string

const (
	IssueStatusDraft      IssueStatus = "draft"
	IssueStatusReviewing  IssueStatus = "reviewing"
	IssueStatusQueued     IssueStatus = "queued"
	IssueStatusReady      IssueStatus = "ready"
	IssueStatusExecuting  IssueStatus = "executing"
	IssueStatusDone       IssueStatus = "done"
	IssueStatusFailed     IssueStatus = "failed"
	IssueStatusSuperseded IssueStatus = "superseded"
	IssueStatusAbandoned  IssueStatus = "abandoned"
)

type FailurePolicy string

const (
	FailBlock FailurePolicy = "block"
	FailSkip  FailurePolicy = "skip"
	FailHuman FailurePolicy = "human"
)

// Issue represents one persistent task/track item for execution.
type Issue struct {
	ID           string        `json:"id"`
	ProjectID    string        `json:"project_id"`
	SessionID    string        `json:"session_id"`
	Title        string        `json:"title"`
	Body         string        `json:"body"`
	Labels       []string      `json:"labels"`
	MilestoneID  string        `json:"milestone_id"`
	Attachments  []string      `json:"attachments"`
	DependsOn    []string      `json:"depends_on"`
	Blocks       []string      `json:"blocks"`
	Priority     int           `json:"priority"`
	Template     string        `json:"template"`
	AutoMerge    bool          `json:"auto_merge"`
	State        IssueState    `json:"state"`
	Status       IssueStatus   `json:"status"`
	PipelineID   string        `json:"pipeline_id"`
	Version      int           `json:"version"`
	SupersededBy string        `json:"superseded_by"`
	ExternalID   string        `json:"external_id"`
	FailPolicy   FailurePolicy `json:"fail_policy"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ClosedAt     *time.Time    `json:"closed_at,omitempty"`
}

// NewIssueID generates an ID in format: issue-YYYYMMDD-xxxxxxxx.
func NewIssueID() string {
	return fmt.Sprintf("issue-%s-%s", time.Now().Format("20060102"), randomHex(4))
}

// Validate checks required Issue fields at the domain-model layer.
func (i Issue) Validate() error {
	if strings.TrimSpace(i.Title) == "" {
		return errors.New("issue title is required")
	}
	if strings.TrimSpace(i.Template) == "" {
		return errors.New("issue template is required")
	}
	if strings.ContainsAny(i.Template, " \t\r\n") {
		return errors.New("issue template must not contain spaces")
	}
	return nil
}

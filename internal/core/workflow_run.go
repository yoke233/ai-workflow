package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// WorkflowRunStatus represents lifecycle states of a V2 workflow run.
type WorkflowRunStatus string

const (
	WorkflowRunStatusCreated       WorkflowRunStatus = "created"
	WorkflowRunStatusRunning       WorkflowRunStatus = "running"
	WorkflowRunStatusWaitingReview WorkflowRunStatus = "waiting_review"
	WorkflowRunStatusDone          WorkflowRunStatus = "done"
	WorkflowRunStatusFailed        WorkflowRunStatus = "failed"
	WorkflowRunStatusTimeout       WorkflowRunStatus = "timeout"
)

var validWorkflowRunStatuses = map[WorkflowRunStatus]struct{}{
	WorkflowRunStatusCreated:       {},
	WorkflowRunStatusRunning:       {},
	WorkflowRunStatusWaitingReview: {},
	WorkflowRunStatusDone:          {},
	WorkflowRunStatusFailed:        {},
	WorkflowRunStatusTimeout:       {},
}

var validWorkflowRunTransitions = map[WorkflowRunStatus]map[WorkflowRunStatus]struct{}{
	WorkflowRunStatusCreated: {
		WorkflowRunStatusRunning: {},
	},
	WorkflowRunStatusRunning: {
		WorkflowRunStatusWaitingReview: {},
		WorkflowRunStatusDone:          {},
		WorkflowRunStatusFailed:        {},
		WorkflowRunStatusTimeout:       {},
	},
	WorkflowRunStatusWaitingReview: {
		WorkflowRunStatusRunning: {},
		WorkflowRunStatusDone:    {},
		WorkflowRunStatusFailed:  {},
		WorkflowRunStatusTimeout: {},
	},
}

// Validate checks whether the workflow run status is one of V2 supported values.
func (s WorkflowRunStatus) Validate() error {
	if _, ok := validWorkflowRunStatuses[s]; !ok {
		return fmt.Errorf("invalid workflow run status %q", s)
	}
	return nil
}

// ValidateWorkflowRunTransition checks whether from -> to is a legal status transition.
func ValidateWorkflowRunTransition(from, to WorkflowRunStatus) error {
	if err := from.Validate(); err != nil {
		return err
	}
	if err := to.Validate(); err != nil {
		return err
	}

	targets, ok := validWorkflowRunTransitions[from]
	if !ok {
		return fmt.Errorf("no transitions allowed from %q", from)
	}
	if _, ok := targets[to]; !ok {
		return fmt.Errorf("invalid transition: %q -> %q", from, to)
	}
	return nil
}

// WorkflowRun is the V2 execution aggregate bound to one issue.
type WorkflowRun struct {
	ID         string            `json:"id"`
	IssueID    string            `json:"issue_id"`
	Profile    string            `json:"profile"`
	Status     WorkflowRunStatus `json:"status"`
	Error      string            `json:"error,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
}

// NewWorkflowRunID generates an ID in format: run-YYYYMMDD-xxxxxxxx.
func NewWorkflowRunID() string {
	return fmt.Sprintf("run-%s-%s", time.Now().Format("20060102"), randomHex(4))
}

// Validate checks required WorkflowRun fields and timestamp constraints.
func (r WorkflowRun) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("run id is required")
	}
	if strings.TrimSpace(r.IssueID) == "" {
		return errors.New("run issue_id is required")
	}
	if strings.TrimSpace(r.Profile) == "" {
		return errors.New("run profile is required")
	}
	if err := r.Status.Validate(); err != nil {
		return err
	}

	if r.FinishedAt != nil && r.FinishedAt.Before(r.CreatedAt) {
		return errors.New("run finished_at must not be earlier than created_at")
	}
	if r.StartedAt != nil && r.FinishedAt != nil && r.FinishedAt.Before(*r.StartedAt) {
		return errors.New("run finished_at must not be earlier than started_at")
	}
	if isTerminalWorkflowRunStatus(r.Status) && r.FinishedAt == nil {
		return errors.New("run finished_at is required for terminal status")
	}
	return nil
}

func isTerminalWorkflowRunStatus(status WorkflowRunStatus) bool {
	switch status {
	case WorkflowRunStatusDone, WorkflowRunStatusFailed, WorkflowRunStatusTimeout:
		return true
	default:
		return false
	}
}

package teamleader

import (
	"errors"
	"time"
)

var (
	ErrA2AInvalidInput = errors.New("a2a invalid input")
	ErrA2ATaskNotFound = errors.New("a2a task not found")
	ErrA2AProjectScope = errors.New("a2a project scope mismatch")
)

type A2ATaskState string

const (
	A2ATaskStateUnknown       A2ATaskState = "unknown"
	A2ATaskStateSubmitted     A2ATaskState = "submitted"
	A2ATaskStateWorking       A2ATaskState = "working"
	A2ATaskStateInputRequired A2ATaskState = "input-required"
	A2ATaskStateCompleted     A2ATaskState = "completed"
	A2ATaskStateFailed        A2ATaskState = "failed"
	A2ATaskStateCanceled      A2ATaskState = "canceled"
)

type A2ASendMessageInput struct {
	ProjectID    string
	SessionID    string
	TaskID       string
	Conversation string
}

type A2AGetTaskInput struct {
	ProjectID string
	TaskID    string
}

type A2ACancelTaskInput struct {
	ProjectID string
	TaskID    string
}

type A2AListTasksInput struct {
	ProjectID string
	SessionID string
	State     A2ATaskState
	PageSize  int
	PageToken string
}

type A2ATaskSnapshot struct {
	TaskID     string
	ProjectID  string
	SessionID  string
	State      A2ATaskState
	Error      string
	UpdatedAt  time.Time
	BranchName string
	Artifacts  map[string]string
}

type A2ATaskList struct {
	Tasks         []*A2ATaskSnapshot
	TotalSize     int
	PageSize      int
	NextPageToken string
}

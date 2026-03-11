package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Tracker mirrors Issue state into external systems.
type Tracker interface {
	Plugin
	CreateIssue(ctx context.Context, issue *Issue) (externalID string, err error)
	UpdateStatus(ctx context.Context, externalID string, status IssueStatus) error
	SyncDependencies(ctx context.Context, issue *Issue, allIssues []*Issue) error
	OnExternalComplete(ctx context.Context, externalID string) error
}

// TrackerWarning marks a non-fatal external sync failure.
type TrackerWarning struct {
	Operation string
	Err       error
}

func (w *TrackerWarning) Error() string {
	if w == nil {
		return "tracker warning"
	}
	operation := strings.TrimSpace(w.Operation)
	if operation == "" {
		operation = "operation"
	}
	if w.Err == nil {
		return fmt.Sprintf("tracker warning: %s", operation)
	}
	return fmt.Sprintf("tracker warning: %s: %v", operation, w.Err)
}

func (w *TrackerWarning) Unwrap() error {
	if w == nil {
		return nil
	}
	return w.Err
}

// NewTrackerWarning wraps an error as a non-fatal tracker warning.
func NewTrackerWarning(operation string, err error) error {
	if err == nil {
		return nil
	}
	var warn *TrackerWarning
	if errors.As(err, &warn) {
		return err
	}
	return &TrackerWarning{
		Operation: operation,
		Err:       err,
	}
}

// IsTrackerWarning reports whether err is a non-fatal tracker warning.
func IsTrackerWarning(err error) bool {
	var warn *TrackerWarning
	return errors.As(err, &warn)
}

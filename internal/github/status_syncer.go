package github

import (
	"context"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

type taskStatusMirror interface {
	UpdateStatus(ctx context.Context, externalID string, status core.TaskItemStatus) error
	SyncDependencies(ctx context.Context, item *core.TaskItem, allItems []core.TaskItem) error
}

// StatusSyncer repairs final-state drift between local task status and GitHub issue labels.
type StatusSyncer struct {
	tracker taskStatusMirror
}

func NewStatusSyncer(tracker taskStatusMirror) *StatusSyncer {
	if tracker == nil {
		return &StatusSyncer{}
	}
	return &StatusSyncer{tracker: tracker}
}

// RepairTask syncs final status and dependency labels for one task item.
func (s *StatusSyncer) RepairTask(ctx context.Context, item *core.TaskItem, allItems []core.TaskItem) error {
	if s == nil || s.tracker == nil || item == nil {
		return nil
	}
	if strings.TrimSpace(item.ExternalID) == "" {
		return nil
	}

	if err := s.tracker.UpdateStatus(ctx, item.ExternalID, item.Status); err != nil {
		return err
	}
	return s.tracker.SyncDependencies(ctx, item, allItems)
}

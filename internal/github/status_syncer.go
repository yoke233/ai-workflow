package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

type taskStatusMirror interface {
	UpdateStatus(ctx context.Context, externalID string, status core.TaskItemStatus) error
	SyncDependencies(ctx context.Context, item *core.TaskItem, allItems []core.TaskItem) error
}

type pipelineIssueSyncClient interface {
	UpdateIssueLabels(ctx context.Context, issueNumber int, labels []string) error
	AddIssueComment(ctx context.Context, issueNumber int, body string) error
}

// StatusSyncer repairs final-state drift between local task status and GitHub issue labels.
type StatusSyncer struct {
	tracker taskStatusMirror
	issues  pipelineIssueSyncClient
}

func NewPipelineStatusSyncer(issues pipelineIssueSyncClient) *StatusSyncer {
	return &StatusSyncer{issues: issues}
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

// SyncPipelineEvent mirrors pipeline lifecycle events to GitHub issue labels/comments.
// Synchronization failures are intentionally non-blocking and return nil.
func (s *StatusSyncer) SyncPipelineEvent(ctx context.Context, evt core.Event) error {
	if s == nil || s.issues == nil {
		return nil
	}

	issueNumber := parseIssueNumberFromEventData(evt.Data)
	if issueNumber <= 0 {
		return nil
	}

	switch evt.Type {
	case core.EventStageStart:
		return s.syncStatusLabel(ctx, issueNumber, []string{stageStatusLabel(evt.Stage)})
	case core.EventPipelineDone:
		return s.syncStatusLabel(ctx, issueNumber, []string{"status: pipeline_done"})
	case core.EventPipelineFailed:
		return s.syncStatusLabel(ctx, issueNumber, []string{"status: pipeline_failed"})
	case core.EventHumanRequired:
		return s.syncHumanRequiredComment(ctx, issueNumber, evt)
	default:
		return nil
	}
}

func (s *StatusSyncer) syncStatusLabel(ctx context.Context, issueNumber int, labels []string) error {
	if err := s.issues.UpdateIssueLabels(ctx, issueNumber, labels); err != nil {
		return nil
	}
	return nil
}

func (s *StatusSyncer) syncHumanRequiredComment(ctx context.Context, issueNumber int, evt core.Event) error {
	stage := strings.TrimSpace(string(evt.Stage))
	if stage == "" {
		stage = "unknown"
	}
	comment := fmt.Sprintf(
		"Pipeline `%s` 在阶段 `%s` 等待人工操作。可用命令：`/approve`、`/reject %s <reason>`、`/abort`、`/status`。",
		strings.TrimSpace(evt.PipelineID),
		stage,
		stage,
	)
	if err := s.issues.AddIssueComment(ctx, issueNumber, comment); err != nil {
		return nil
	}
	return nil
}

func stageStatusLabel(stage core.StageID) string {
	stageID := strings.TrimSpace(string(stage))
	if stageID == "" {
		return "status: pipeline_active"
	}
	return "status: pipeline_active:" + stageID
}

func parseIssueNumberFromEventData(data map[string]string) int {
	if len(data) == 0 {
		return 0
	}
	keys := []string{"issue_number", "github_issue_number", "issue"}
	for _, key := range keys {
		raw := strings.TrimSpace(data[key])
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			return value
		}
	}
	return 0
}

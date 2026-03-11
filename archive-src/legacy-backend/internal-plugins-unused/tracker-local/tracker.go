package trackerlocal

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/core"
)

// LocalTracker is a no-op tracker implementation that keeps all operations local.
type LocalTracker struct{}

func New() *LocalTracker {
	return &LocalTracker{}
}

func (t *LocalTracker) Name() string {
	return "local"
}

func (t *LocalTracker) Init(context.Context) error {
	return nil
}

func (t *LocalTracker) Close() error {
	return nil
}

func (t *LocalTracker) CreateIssue(_ context.Context, issue *core.Issue) (string, error) {
	if issue == nil {
		return "", nil
	}
	if issue.ExternalID != "" {
		return issue.ExternalID, nil
	}
	return issue.ID, nil
}

func (t *LocalTracker) UpdateStatus(context.Context, string, core.IssueStatus) error {
	return nil
}

func (t *LocalTracker) SyncDependencies(context.Context, *core.Issue, []*core.Issue) error {
	return nil
}

func (t *LocalTracker) OnExternalComplete(context.Context, string) error {
	return nil
}

var _ core.Tracker = (*LocalTracker)(nil)

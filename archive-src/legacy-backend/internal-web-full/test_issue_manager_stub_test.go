package web

import (
	"context"
	"errors"

	"github.com/yoke233/ai-workflow/internal/core"
)

type testPlanManager struct {
	createIssuesFn     func(ctx context.Context, input IssueCreateInput) ([]core.Issue, error)
	createDraftFn      func(ctx context.Context, input IssueCreateInput) ([]core.Issue, error)
	submitForReviewFn  func(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error)
	submitReviewFn     func(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error)
	applyIssueActionFn func(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error)
	applyActionFn      func(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error)
}

func (m *testPlanManager) CreateIssues(ctx context.Context, input IssueCreateInput) ([]core.Issue, error) {
	switch {
	case m.createIssuesFn != nil:
		return m.createIssuesFn(ctx, input)
	case m.createDraftFn != nil:
		return m.createDraftFn(ctx, input)
	default:
		return nil, errors.New("create issues not implemented")
	}
}

func (m *testPlanManager) SubmitForReview(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error) {
	switch {
	case m.submitForReviewFn != nil:
		return m.submitForReviewFn(ctx, issueID, input)
	case m.submitReviewFn != nil:
		return m.submitReviewFn(ctx, issueID, input)
	default:
		return nil, errors.New("submit for review not implemented")
	}
}

func (m *testPlanManager) ApplyIssueAction(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error) {
	switch {
	case m.applyIssueActionFn != nil:
		return m.applyIssueActionFn(ctx, issueID, action)
	case m.applyActionFn != nil:
		return m.applyActionFn(ctx, issueID, action)
	default:
		return nil, errors.New("apply issue action not implemented")
	}
}

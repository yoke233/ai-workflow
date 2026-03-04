package reviewaipanel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
	storesqlite "github.com/yoke233/ai-workflow/internal/plugins/store-sqlite"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

func TestAIReviewGate_UnknownReview(t *testing.T) {
	store, _ := newTestStoreWithIssue(t)
	gate := New(store, fakePanel{})

	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := gate.Check(context.Background(), "issue-unknown"); err == nil {
		t.Fatalf("expected Check(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), "issue-unknown"); err == nil {
		t.Fatalf("expected Cancel(unknown reviewID) to fail")
	}
}

func TestAIReviewGate_ProfileDefaultsToNormal(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	issue.Labels = nil
	issue.Template = "standard"

	done := make(chan struct{})
	capturedLabels := []string{}
	panel := fakePanel{
		run: func(_ context.Context, issues []*core.Issue) (*teamleader.ReviewSessionResult, error) {
			if len(issues) != 1 {
				return nil, errors.New("expected one issue")
			}
			capturedLabels = append([]string(nil), issues[0].Labels...)
			close(done)
			return &teamleader.ReviewSessionResult{
				Status:   core.ReviewStatusApproved,
				Decision: teamleader.DecisionApprove,
			}, nil
		},
	}
	gate := New(store, panel)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("review run did not complete")
	}

	if profile := resolveIssueProfile(&core.Issue{Labels: capturedLabels}); profile != core.WorkflowProfileNormal {
		t.Fatalf("normalized profile = %q, want %q", profile, core.WorkflowProfileNormal)
	}
}

func TestAIReviewGate_ProfileStrictAndFastReleasePassThrough(t *testing.T) {
	tests := []struct {
		name    string
		profile core.WorkflowProfileType
	}{
		{name: "strict", profile: core.WorkflowProfileStrict},
		{name: "fast_release", profile: core.WorkflowProfileFastRelease},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store, issue := newTestStoreWithIssue(t)
			issue.Labels = []string{"profile:" + string(tc.profile)}

			done := make(chan struct{})
			capturedLabels := []string{}
			panel := fakePanel{
				run: func(_ context.Context, issues []*core.Issue) (*teamleader.ReviewSessionResult, error) {
					if len(issues) != 1 {
						return nil, errors.New("expected one issue")
					}
					capturedLabels = append([]string(nil), issues[0].Labels...)
					close(done)
					return &teamleader.ReviewSessionResult{
						Status:   core.ReviewStatusApproved,
						Decision: teamleader.DecisionApprove,
					}, nil
				},
			}
			gate := New(store, panel)
			if err := gate.Init(context.Background()); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err != nil {
				t.Fatalf("Submit() error = %v", err)
			}

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("review run did not complete")
			}

			if profile := resolveIssueProfile(&core.Issue{Labels: capturedLabels}); profile != tc.profile {
				t.Fatalf("normalized profile = %q, want %q", profile, tc.profile)
			}
		})
	}
}

func TestAIReviewGate_CancelWinsOverAsyncError(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	started := make(chan struct{})

	panel := fakePanel{
		run: func(ctx context.Context, _ []*core.Issue) (*teamleader.ReviewSessionResult, error) {
			close(started)
			<-ctx.Done()
			return nil, errors.New("runner returned non-context error after cancellation")
		},
	}
	gate := New(store, panel)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}

	if err := gate.Cancel(context.Background(), issue.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := gate.Check(context.Background(), issue.ID)
		if err == nil && got.Status == "cancelled" && got.Decision == "cancelled" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected cancelled status to win after cancel+async-error race")
}

func TestAIReviewGate_SubmitPendingDuplicateAndCancelIdempotent(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	started := make(chan struct{})
	panel := fakePanel{
		run: func(ctx context.Context, _ []*core.Issue) (*teamleader.ReviewSessionResult, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	gate := New(store, panel)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	reviewID, err := gate.Submit(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if reviewID != issue.ID {
		t.Fatalf("Submit() reviewID = %q, want %q", reviewID, issue.ID)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("runner did not start")
	}

	if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err == nil {
		t.Fatalf("expected duplicate Submit() to fail while review is running")
	}

	pending, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() while running error = %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("Check().Status = %q, want pending", pending.Status)
	}
	if pending.Decision != "pending" {
		t.Fatalf("Check().Decision = %q, want pending", pending.Decision)
	}

	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("first Cancel() error = %v", err)
	}
	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("second Cancel() should be idempotent, got error = %v", err)
	}

	cancelled, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after cancel error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Status)
	}
	if cancelled.Decision != "cancelled" {
		t.Fatalf("cancelled decision = %q, want cancelled", cancelled.Decision)
	}
}

func TestAIReviewGate_CheckCompletedStatusMapping(t *testing.T) {
	tests := []struct {
		name         string
		verdict      string
		wantStatus   string
		wantDecision string
	}{
		{
			name:         "approved from pass verdict",
			verdict:      "pass",
			wantStatus:   "approved",
			wantDecision: "approve",
		},
		{
			name:         "rejected from escalate verdict",
			verdict:      "escalate",
			wantStatus:   "rejected",
			wantDecision: "escalate",
		},
		{
			name:         "changes requested from fix verdict",
			verdict:      "fix",
			wantStatus:   "changes_requested",
			wantDecision: "fix",
		},
		{
			name:         "cancelled",
			verdict:      "cancelled",
			wantStatus:   "cancelled",
			wantDecision: "cancelled",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store, issue := newTestStoreWithIssue(t)
			done := make(chan struct{})
			panel := fakePanel{
				run: func(_ context.Context, _ []*core.Issue) (*teamleader.ReviewSessionResult, error) {
					if err := store.SaveReviewRecord(&core.ReviewRecord{
						IssueID:  issue.ID,
						Round:    1,
						Reviewer: "aggregator",
						Verdict:  tc.verdict,
					}); err != nil {
						t.Fatalf("SaveReviewRecord() error = %v", err)
					}
					close(done)
					return nil, nil
				},
			}
			gate := New(store, panel)
			if err := gate.Init(context.Background()); err != nil {
				t.Fatalf("Init() error = %v", err)
			}

			if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err != nil {
				t.Fatalf("Submit() error = %v", err)
			}

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("review run did not complete")
			}

			got, err := gate.Check(context.Background(), issue.ID)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("Check().Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Decision != tc.wantDecision {
				t.Fatalf("Check().Decision = %q, want %q", got.Decision, tc.wantDecision)
			}
			for _, verdict := range got.Verdicts {
				if verdict.Reviewer == gateReviewer {
					t.Fatalf("Check().Verdicts should not include gate reviewer marker %q", gateReviewer)
				}
			}
		})
	}
}

func TestAIReviewGate_SessionDecisionPersistsFinalVerdict(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	done := make(chan struct{})
	panel := fakePanel{
		run: func(_ context.Context, _ []*core.Issue) (*teamleader.ReviewSessionResult, error) {
			if err := store.SaveReviewRecord(&core.ReviewRecord{
				IssueID:  issue.ID,
				Round:    1,
				Reviewer: "aggregator",
				Verdict:  "issues_found",
			}); err != nil {
				t.Fatalf("SaveReviewRecord() error = %v", err)
			}
			close(done)
			return &teamleader.ReviewSessionResult{
				Status:   core.ReviewStatusRejected,
				Decision: teamleader.DecisionEscalate,
				Verdicts: map[string]core.ReviewVerdict{
					issue.ID: {
						Reviewer: "aggregator",
						Status:   "issues_found",
						Issues: []core.ReviewIssue{
							{
								Severity:    "warning",
								IssueID:     issue.ID,
								Description: "dependency conflict",
							},
						},
						Score: 60,
					},
				},
			}, nil
		},
	}
	gate := New(store, panel)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := gate.Submit(context.Background(), []*core.Issue{issue}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("review run did not complete")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := gate.Check(context.Background(), issue.ID)
		if err == nil && got.Status == "rejected" && got.Decision == "escalate" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected session decision persisted as rejected/escalate")
}

type fakePanel struct {
	run func(ctx context.Context, issues []*core.Issue) (*teamleader.ReviewSessionResult, error)
}

func (f fakePanel) Run(ctx context.Context, issues []*core.Issue) (*teamleader.ReviewSessionResult, error) {
	if f.run == nil {
		return &teamleader.ReviewSessionResult{
			Status:   core.ReviewStatusApproved,
			Decision: teamleader.DecisionApprove,
		}, nil
	}
	return f.run(ctx, cloneIssues(issues))
}

func newTestStoreWithIssue(t *testing.T) (core.Store, *core.Issue) {
	t.Helper()

	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("storesqlite.New(:memory:) error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	project := &core.Project{
		ID:       "proj-review-ai-panel",
		Name:     "review-ai-panel",
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	issue := &core.Issue{
		ID:         "issue-20260301-reviewaipanel",
		ProjectID:  project.ID,
		Title:      "ai-panel-review",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateIssue(issue); err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	return store, issue
}

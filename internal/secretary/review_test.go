package secretary

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

func TestReviewOrchestratorRunApprovePath(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 92}, nil
		}},
	}

	issues := []*core.Issue{
		newReviewTestIssue("issue-review-approve-1"),
		newReviewTestIssue("issue-review-approve-2"),
	}

	result, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}
	if result.Status != core.ReviewStatusApproved {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusApproved)
	}
	if !result.AutoApproved {
		t.Fatal("auto_approved = false, want true")
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("verdict count = %d, want 2", len(result.Verdicts))
	}

	for _, issue := range issues {
		records, err := store.GetReviewRecords(issue.ID)
		if err != nil {
			t.Fatalf("GetReviewRecords(%s) error = %v", issue.ID, err)
		}
		if len(records) != 2 {
			t.Fatalf("record count for issue %s = %d, want 2", issue.ID, len(records))
		}
		if got := collectReviewers(records); !slices.Equal(got, []string{phase2ReviewerName, phase1ReviewerName}) {
			t.Fatalf("reviewers = %v, want [%s %s]", got, phase2ReviewerName, phase1ReviewerName)
		}
	}
}

func TestReviewOrchestratorProfileStrictRunsThreeReviewersAndAggregator(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 91}, nil
		}},
	}

	issue := newReviewTestIssue("issue-review-strict")
	issue.Labels = []string{"profile:strict"}

	result, err := panel.Run(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("record count = %d, want 4 (3 strict reviewers + 1 aggregator)", len(records))
	}
	if got := collectReviewers(records); !slices.Equal(got, []string{
		phase2ReviewerName,
		strictReviewerNamePrefix + "_1",
		strictReviewerNamePrefix + "_2",
		strictReviewerNamePrefix + "_3",
	}) {
		t.Fatalf("reviewers = %v, want strict trio + aggregator", got)
	}
}

func TestReviewOrchestratorProfileNormalUsesSingleReviewerAndAggregator(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 90}, nil
		}},
	}

	issue := newReviewTestIssue("issue-review-normal")
	issue.Labels = []string{"profile:normal"}

	if _, err := panel.Run(context.Background(), []*core.Issue{issue}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if got := collectReviewers(records); !slices.Equal(got, []string{
		phase2ReviewerName,
		phase1ReviewerName,
	}) {
		t.Fatalf("reviewers = %v, want [%s %s]", got, phase2ReviewerName, phase1ReviewerName)
	}
}

func TestReviewOrchestratorProfileFastReleaseBypassesThreshold(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store:                store,
		AutoApproveThreshold: 95,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 70}, nil
		}},
	}

	issue := newReviewTestIssue("issue-review-fast-release")
	issue.Labels = []string{"profile:fast_release"}

	result, err := panel.Run(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}
	if result.Status != core.ReviewStatusApproved {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusApproved)
	}
}

func TestReviewOrchestratorRunFixPathWhenVerdictHasIssues(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{
				Reviewer: "completeness",
				Status:   "approved",
				Score:    80,
				Issues: []core.ReviewIssue{
					{
						Severity:    "warning",
						IssueID:     "",
						Description: "覆盖不足",
						Suggestion:  "补齐测试",
					},
				},
			}, nil
		}},
	}

	issue := newReviewTestIssue("issue-review-fix")
	result, err := panel.Run(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionFix {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionFix)
	}
	if result.Status != core.ReviewStatusChangesRequested {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusChangesRequested)
	}
	if result.AutoApproved {
		t.Fatal("auto_approved = true, want false")
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if records[0].Verdict != "issues_found" {
		t.Fatalf("phase1 verdict = %q, want %q", records[0].Verdict, "issues_found")
	}
	if len(records[0].Issues) != 1 {
		t.Fatalf("phase1 issues count = %d, want 1", len(records[0].Issues))
	}
	if records[0].Issues[0].IssueID != issue.ID {
		t.Fatalf("phase1 issue_id = %q, want %q", records[0].Issues[0].IssueID, issue.ID)
	}
}

func TestReviewOrchestratorRunEscalatePathOnDependencyConflict(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, issue *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Reviewer: "demand", Status: "pass", Score: 95, Issues: nil}, nil
		}},
		Analyzer: stubDependencyAnalyzer{fn: func(_ context.Context, issues []*core.Issue) (*DependencyAnalysis, error) {
			if len(issues) != 2 {
				return nil, errors.New("analyzer should receive 2 issues")
			}
			return &DependencyAnalysis{
				Conflicts: []ConflictInfo{
					{
						IssueIDs:   []string{issues[0].ID, issues[1].ID},
						Resource:   "db-lock",
						Suggestion: "串行化写入顺序",
					},
				},
			}, nil
		}},
	}

	issues := []*core.Issue{
		newReviewTestIssue("issue-review-conflict-1"),
		newReviewTestIssue("issue-review-conflict-2"),
	}

	result, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionEscalate {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionEscalate)
	}
	if result.Status != core.ReviewStatusRejected {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusRejected)
	}
	if result.AutoApproved {
		t.Fatal("auto_approved = true, want false")
	}
	if result.DAG == nil || len(result.DAG.Conflicts) != 1 {
		t.Fatalf("dag conflicts = %#v, want 1 conflict", result.DAG)
	}

	for _, issue := range issues {
		records, err := store.GetReviewRecords(issue.ID)
		if err != nil {
			t.Fatalf("GetReviewRecords(%s) error = %v", issue.ID, err)
		}
		if len(records) != 2 {
			t.Fatalf("record count for issue %s = %d, want 2", issue.ID, len(records))
		}
		phase2 := records[1]
		if phase2.Verdict != "issues_found" {
			t.Fatalf("phase2 verdict for issue %s = %q, want issues_found", issue.ID, phase2.Verdict)
		}
		if len(phase2.Issues) != 1 {
			t.Fatalf("phase2 issues count for issue %s = %d, want 1", issue.ID, len(phase2.Issues))
		}
		if !strings.Contains(phase2.Issues[0].Description, "db-lock") {
			t.Fatalf("phase2 description = %q, want contains db-lock", phase2.Issues[0].Description)
		}
	}
}

func TestReviewOrchestratorRunFixPathWhenBelowThreshold(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store:                store,
		AutoApproveThreshold: 85,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 84}, nil
		}},
	}

	result, err := panel.Run(context.Background(), []*core.Issue{newReviewTestIssue("issue-review-threshold")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionFix {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionFix)
	}
	if result.Status != core.ReviewStatusChangesRequested {
		t.Fatalf("status = %q, want %q", result.Status, core.ReviewStatusChangesRequested)
	}
	if result.AutoApproved {
		t.Fatal("auto_approved = true, want false")
	}
}

func TestReviewOrchestratorRunUsesLegacyReviewerAdapter(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewers: []Reviewer{
			stubLegacyReviewer{
				name: "legacy-reviewer",
				fn: func(_ context.Context, input ReviewerInput) (core.ReviewVerdict, error) {
					if input.Issue == nil {
						return core.ReviewVerdict{}, errors.New("legacy reviewer should receive issue")
					}
					input.Issue.Title = "changed-inside-reviewer"
					return core.ReviewVerdict{Reviewer: "legacy-reviewer", Status: "pass", Score: 91}, nil
				},
			},
		},
	}

	issue := newReviewTestIssue("issue-review-legacy")
	originalTitle := issue.Title

	result, err := panel.Run(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Decision != DecisionApprove {
		t.Fatalf("decision = %q, want %q", result.Decision, DecisionApprove)
	}
	if issue.Title != originalTitle {
		t.Fatalf("original issue title mutated = %q, want %q", issue.Title, originalTitle)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected persisted records")
	}
	if records[0].Reviewer != "legacy-reviewer" {
		t.Fatalf("phase1 reviewer = %q, want legacy-reviewer", records[0].Reviewer)
	}
}

func TestReviewOrchestratorSubmitForReviewDelegatesRun(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := ReviewOrchestrator{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 90}, nil
		}},
	}

	err := panel.SubmitForReview(context.Background(), []*core.Issue{newReviewTestIssue("issue-review-submit")})
	if err != nil {
		t.Fatalf("SubmitForReview() error = %v", err)
	}
}

func TestDefaultReviewOrchestratorSubmitForReviewAcceptsSingleIssue(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	panel := NewDefaultReviewOrchestrator(store)

	issue := newReviewTestIssue("issue-review-default-submit")
	if err := panel.SubmitForReview(context.Background(), []*core.Issue{issue}); err != nil {
		t.Fatalf("SubmitForReview() error = %v", err)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords(%s) error = %v", issue.ID, err)
	}
	if len(records) == 0 {
		t.Fatalf("expected review records for %s", issue.ID)
	}
}

func TestTwoPhaseReviewRunValidatesInput(t *testing.T) {
	t.Parallel()

	base := TwoPhaseReview{
		Store: newMockReviewStore(),
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 90}, nil
		}},
	}

	tests := []struct {
		name   string
		panel  TwoPhaseReview
		issues []*core.Issue
		want   string
	}{
		{
			name:   "missing store",
			panel:  TwoPhaseReview{Reviewer: base.Reviewer},
			issues: []*core.Issue{newReviewTestIssue("issue-review-validate-1")},
			want:   "review store is required",
		},
		{
			name:   "missing reviewer",
			panel:  TwoPhaseReview{Store: base.Store},
			issues: []*core.Issue{newReviewTestIssue("issue-review-validate-2")},
			want:   "demand reviewer is required",
		},
		{
			name:   "empty issues",
			panel:  base,
			issues: nil,
			want:   "issues are required",
		},
		{
			name:  "nil issue",
			panel: base,
			issues: []*core.Issue{
				nil,
			},
			want: "issue[0] is nil",
		},
		{
			name:  "blank id",
			panel: base,
			issues: []*core.Issue{
				newReviewTestIssue("   "),
			},
			want: "issue[0] id is required",
		},
		{
			name:  "duplicate id",
			panel: base,
			issues: []*core.Issue{
				newReviewTestIssue("issue-review-dup"),
				newReviewTestIssue("issue-review-dup"),
			},
			want: `duplicate issue id "issue-review-dup"`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.panel.Run(context.Background(), tc.issues)
			if err == nil {
				t.Fatalf("Run() expected error contains %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestTwoPhaseReviewRunUsesNextRoundFromHistory(t *testing.T) {
	t.Parallel()

	store := newMockReviewStore()
	if err := store.SaveReviewRecord(&core.ReviewRecord{IssueID: "issue-review-round-1", Round: 2, Reviewer: "x", Verdict: "pass"}); err != nil {
		t.Fatalf("seed record issue-1 error = %v", err)
	}
	if err := store.SaveReviewRecord(&core.ReviewRecord{IssueID: "issue-review-round-2", Round: 4, Reviewer: "x", Verdict: "pass"}); err != nil {
		t.Fatalf("seed record issue-2 error = %v", err)
	}

	panel := TwoPhaseReview{
		Store: store,
		Reviewer: stubDemandReviewer{fn: func(_ context.Context, _ *core.Issue) (core.ReviewVerdict, error) {
			return core.ReviewVerdict{Status: "pass", Score: 96}, nil
		}},
	}

	issues := []*core.Issue{
		newReviewTestIssue("issue-review-round-1"),
		newReviewTestIssue("issue-review-round-2"),
	}
	_, err := panel.Run(context.Background(), issues)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, issue := range issues {
		records, err := store.GetReviewRecords(issue.ID)
		if err != nil {
			t.Fatalf("GetReviewRecords(%s) error = %v", issue.ID, err)
		}
		var latestRound int
		for _, record := range records {
			if record.Round > latestRound {
				latestRound = record.Round
			}
		}
		if latestRound != 5 {
			t.Fatalf("latest round for issue %s = %d, want 5", issue.ID, latestRound)
		}
	}
}

func TestNormalizeIssuesTrimsIDAndClones(t *testing.T) {
	t.Parallel()

	origin := newReviewTestIssue(" issue-review-normalize ")
	origin.DependsOn = []string{"upstream-1"}

	out, err := normalizeIssues([]*core.Issue{origin})
	if err != nil {
		t.Fatalf("normalizeIssues() error = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("normalize output len = %d, want 1", len(out))
	}
	if out[0].ID != "issue-review-normalize" {
		t.Fatalf("normalized id = %q, want %q", out[0].ID, "issue-review-normalize")
	}

	out[0].Title = "mutated"
	out[0].DependsOn[0] = "mutated-dependency"
	if strings.TrimSpace(origin.ID) != "issue-review-normalize" {
		t.Fatalf("origin id unexpectedly changed = %q", origin.ID)
	}
	if origin.Title == "mutated" {
		t.Fatal("origin title should not be mutated")
	}
	if origin.DependsOn[0] != "upstream-1" {
		t.Fatalf("origin depends_on[0] = %q, want %q", origin.DependsOn[0], "upstream-1")
	}
}

func TestNormalizeVerdictDefaultsAndBounds(t *testing.T) {
	t.Parallel()

	pass := normalizeVerdict("issue-review-verdict", core.ReviewVerdict{})
	if pass.Reviewer != phase1ReviewerName {
		t.Fatalf("reviewer = %q, want %q", pass.Reviewer, phase1ReviewerName)
	}
	if pass.Status != "pass" {
		t.Fatalf("status = %q, want pass", pass.Status)
	}
	if pass.Score != 100 {
		t.Fatalf("score = %d, want 100", pass.Score)
	}

	withIssues := normalizeVerdict("issue-review-verdict", core.ReviewVerdict{
		Reviewer: " custom-reviewer ",
		Status:   "approved",
		Score:    200,
		Issues: []core.ReviewIssue{
			{Severity: "warning", Description: "x"},
		},
	})
	if withIssues.Reviewer != "custom-reviewer" {
		t.Fatalf("reviewer = %q, want custom-reviewer", withIssues.Reviewer)
	}
	if withIssues.Status != "issues_found" {
		t.Fatalf("status = %q, want issues_found", withIssues.Status)
	}
	if withIssues.Score != 100 {
		t.Fatalf("score = %d, want 100", withIssues.Score)
	}
	if withIssues.Issues[0].IssueID != "issue-review-verdict" {
		t.Fatalf("issue_id = %q, want issue-review-verdict", withIssues.Issues[0].IssueID)
	}
}

func TestDependencyIssuesForIssueMapsConflicts(t *testing.T) {
	t.Parallel()

	analysis := &DependencyAnalysis{
		Conflicts: []ConflictInfo{
			{IssueIDs: []string{"issue-1", "issue-2"}, Resource: "shared-db", Suggestion: "serial execution"},
			{IssueIDs: []string{"issue-3"}, Resource: "cache"},
		},
	}

	got := dependencyIssuesForIssue("issue-1", analysis)
	if len(got) != 1 {
		t.Fatalf("issues len = %d, want 1", len(got))
	}
	if got[0].IssueID != "issue-1" {
		t.Fatalf("issue_id = %q, want %q", got[0].IssueID, "issue-1")
	}
	if !strings.Contains(got[0].Description, "shared-db") {
		t.Fatalf("description = %q, want contains shared-db", got[0].Description)
	}
	if got[0].Suggestion != "serial execution" {
		t.Fatalf("suggestion = %q, want %q", got[0].Suggestion, "serial execution")
	}
}

func TestReviewOrchestratorUsesRoleBindings(t *testing.T) {
	t.Parallel()

	resolver := acpclient.NewRoleResolver(
		[]acpclient.AgentProfile{
			{
				ID: "codex",
				CapabilitiesMax: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]acpclient.RoleProfile{
			{
				ID:      "reviewer",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:       false,
					ResetPrompt: false,
				},
			},
			{
				ID:      "aggregator",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:       false,
					ResetPrompt: false,
				},
			},
		},
	)

	runtime, err := ResolveReviewOrchestratorRoles(ReviewRoleBindingInput{
		Reviewers: map[string]string{
			"completeness": "reviewer",
			"dependency":   "reviewer",
			"feasibility":  "reviewer",
		},
		Aggregator: "aggregator",
	}, resolver)
	if err != nil {
		t.Fatalf("ResolveReviewOrchestratorRoles() error = %v", err)
	}
	if runtime.AggregatorRole != "aggregator" {
		t.Fatalf("aggregator role = %q, want %q", runtime.AggregatorRole, "aggregator")
	}
	for _, reviewer := range []string{"completeness", "dependency", "feasibility"} {
		if got := runtime.ReviewerRoles[reviewer]; got != "reviewer" {
			t.Fatalf("reviewer role %s = %q, want %q", reviewer, got, "reviewer")
		}
		policy := runtime.ReviewerSessionPolicies[reviewer]
		if !policy.Reuse {
			t.Fatalf("reviewer %s reuse should default true", reviewer)
		}
		if !policy.ResetPrompt {
			t.Fatalf("reviewer %s reset_prompt should default true", reviewer)
		}
	}
	if !runtime.AggregatorSessionPolicy.Reuse {
		t.Fatal("aggregator reuse should default true")
	}
	if !runtime.AggregatorSessionPolicy.ResetPrompt {
		t.Fatal("aggregator reset_prompt should default true")
	}
}

type mockReviewStore struct {
	mu      sync.Mutex
	records []core.ReviewRecord
}

func newMockReviewStore() *mockReviewStore {
	return &mockReviewStore{}
}

func (s *mockReviewStore) SaveReviewRecord(r *core.ReviewRecord) error {
	if r == nil {
		return errors.New("review record is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := *r
	record.Issues = append([]core.ReviewIssue(nil), r.Issues...)
	record.Fixes = append([]core.ProposedFix(nil), r.Fixes...)
	s.records = append(s.records, record)
	return nil
}

func (s *mockReviewStore) GetReviewRecords(issueID string) ([]core.ReviewRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(issueID)
	filtered := make([]core.ReviewRecord, 0)
	for i := range s.records {
		record := s.records[i]
		if strings.TrimSpace(record.IssueID) != key {
			continue
		}
		cp := record
		cp.Issues = append([]core.ReviewIssue(nil), record.Issues...)
		cp.Fixes = append([]core.ProposedFix(nil), record.Fixes...)
		filtered = append(filtered, cp)
	}
	return filtered, nil
}

type stubDemandReviewer struct {
	fn func(ctx context.Context, issue *core.Issue) (core.ReviewVerdict, error)
}

func (r stubDemandReviewer) Review(ctx context.Context, issue *core.Issue) (core.ReviewVerdict, error) {
	if r.fn != nil {
		return r.fn(ctx, issue)
	}
	return core.ReviewVerdict{Status: "pass", Score: 90}, nil
}

type stubDependencyAnalyzer struct {
	fn func(ctx context.Context, issues []*core.Issue) (*DependencyAnalysis, error)
}

func (a stubDependencyAnalyzer) Analyze(ctx context.Context, issues []*core.Issue) (*DependencyAnalysis, error) {
	if a.fn != nil {
		return a.fn(ctx, issues)
	}
	return &DependencyAnalysis{}, nil
}

type stubLegacyReviewer struct {
	name string
	fn   func(ctx context.Context, input ReviewerInput) (core.ReviewVerdict, error)
}

func (r stubLegacyReviewer) Name() string { return r.name }

func (r stubLegacyReviewer) Review(ctx context.Context, input ReviewerInput) (core.ReviewVerdict, error) {
	if r.fn != nil {
		return r.fn(ctx, input)
	}
	return core.ReviewVerdict{Reviewer: r.name, Status: "pass", Score: 80}, nil
}

func newReviewTestIssue(id string) *core.Issue {
	return &core.Issue{
		ID:         id,
		ProjectID:  "proj-review",
		SessionID:  "session-review",
		Title:      "实现功能A",
		Body:       "完成功能A并补充测试",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	}
}

func collectReviewers(records []core.ReviewRecord) []string {
	set := map[string]struct{}{}
	for _, record := range records {
		set[record.Reviewer] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for reviewer := range set {
		out = append(out, reviewer)
	}
	slices.Sort(out)
	return out
}

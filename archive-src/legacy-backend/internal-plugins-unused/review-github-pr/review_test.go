package reviewgithubpr

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	ghapi "github.com/google/go-github/v68/github"
	"github.com/yoke233/ai-workflow/internal/core"
	ghsvc "github.com/yoke233/ai-workflow/internal/github"
)

func TestGitHubPRReview_Submit_CreatesReviewPRFromIssue(t *testing.T) {
	store := newFakeReviewStore()
	client := &fakePRClient{
		createPR: &ghapi.PullRequest{
			Number:  ghapi.Int(77),
			HTMLURL: ghapi.String("https://github.com/acme/ai-workflow/pull/77"),
		},
	}

	gate := New(store, client)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	issue := &core.Issue{
		ID:    "issue-gh-review-1",
		Title: "Refactor scheduler retry policy",
		Body:  "确保失败重试和超时策略一致。",
	}
	reviewID, err := gate.Submit(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if reviewID != issue.ID {
		t.Fatalf("Submit() reviewID = %q, want %q", reviewID, issue.ID)
	}
	if client.createCalls != 1 {
		t.Fatalf("expected CreatePR called once, got %d", client.createCalls)
	}
	if client.lastCreateInput.Title != "[Review] "+issue.Title {
		t.Fatalf("create pr title = %q, want %q", client.lastCreateInput.Title, "[Review] "+issue.Title)
	}
	if !strings.Contains(client.lastCreateInput.Body, issue.ID) {
		t.Fatalf("create pr body should contain issue id, body=%q", client.lastCreateInput.Body)
	}
	if !strings.Contains(client.lastCreateInput.Body, issue.Body) {
		t.Fatalf("create pr body should contain issue body, body=%q", client.lastCreateInput.Body)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one review record, got %d", len(records))
	}
	if records[0].IssueID != issue.ID {
		t.Fatalf("review record issueID = %q, want %q", records[0].IssueID, issue.ID)
	}
	if records[0].Verdict != "pending" {
		t.Fatalf("expected pending verdict, got %q", records[0].Verdict)
	}
	if got := extractPRNumber(records[0].Fixes); got != 77 {
		t.Fatalf("extractPRNumber() = %d, want %d", got, 77)
	}
}

func TestGitHubPRReview_Check_MapsReviewStatesByIssueID(t *testing.T) {
	store := newFakeReviewStore()
	gate := New(store, nil)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	issueID := "issue-gh-review-2"
	seedReviewRecord(t, store, core.ReviewRecord{
		IssueID:  issueID,
		Round:    1,
		Reviewer: reviewerName,
		Verdict:  "changes_requested",
	})

	result, err := gate.Check(context.Background(), issueID)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != "changes_requested" {
		t.Fatalf("expected status changes_requested, got %q", result.Status)
	}
	if result.Decision != "fix" {
		t.Fatalf("expected decision fix, got %q", result.Decision)
	}

	queries := store.queriedIssueIDs()
	if len(queries) == 0 || queries[len(queries)-1] != issueID {
		t.Fatalf("expected Check() to query issueID %q, queries=%v", issueID, queries)
	}
}

func TestGitHubPRReview_Cancel_ClosesPRByIssueID(t *testing.T) {
	store := newFakeReviewStore()
	client := &fakePRClient{}
	gate := New(store, client)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	issueID := "issue-gh-review-3"
	seedReviewRecord(t, store, core.ReviewRecord{
		IssueID:  issueID,
		Round:    1,
		Reviewer: reviewerName,
		Verdict:  "pending",
		Fixes: []core.ProposedFix{
			{
				Description: "pr_number",
				Suggestion:  "88",
			},
		},
	})

	if err := gate.Cancel(context.Background(), issueID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if client.updateCalls != 1 {
		t.Fatalf("expected UpdatePR called once, got %d", client.updateCalls)
	}
	if client.lastUpdateNumber != 88 {
		t.Fatalf("expected close pr number 88, got %d", client.lastUpdateNumber)
	}

	records, err := store.GetReviewRecords(issueID)
	if err != nil {
		t.Fatalf("GetReviewRecords() error = %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected cancel record appended, got %d records", len(records))
	}
	latest := records[len(records)-1]
	if latest.IssueID != issueID {
		t.Fatalf("expected cancel record issueID %q, got %q", issueID, latest.IssueID)
	}
	if latest.Verdict != "cancelled" {
		t.Fatalf("expected cancelled verdict, got %q", latest.Verdict)
	}

	queries := store.queriedIssueIDs()
	if len(queries) == 0 || queries[len(queries)-1] != issueID {
		t.Fatalf("expected Cancel() to query issueID %q, queries=%v", issueID, queries)
	}
}

func TestGitHubPRReview_Submit_ValidatesIssues(t *testing.T) {
	store := newFakeReviewStore()
	client := &fakePRClient{}
	gate := New(store, client)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cases := []struct {
		name   string
		issues []*core.Issue
	}{
		{name: "nil issues", issues: nil},
		{name: "empty issues", issues: []*core.Issue{}},
		{name: "nil issue", issues: []*core.Issue{nil}},
		{name: "missing issue id", issues: []*core.Issue{{Title: "missing id"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := gate.Submit(context.Background(), tc.issues); err == nil {
				t.Fatalf("Submit() expected error for case %q", tc.name)
			}
		})
	}
}

func seedReviewRecord(t *testing.T, store core.Store, record core.ReviewRecord) {
	t.Helper()
	if err := store.SaveReviewRecord(&record); err != nil {
		t.Fatalf("SaveReviewRecord() error = %v", err)
	}
}

type fakePRClient struct {
	createPR *ghapi.PullRequest

	createCalls      int
	updateCalls      int
	lastUpdateNumber int
	lastCreateInput  ghsvc.CreatePRInput
	lastUpdateInput  ghsvc.UpdatePRInput
}

func (f *fakePRClient) CreatePR(_ context.Context, input ghsvc.CreatePRInput) (*ghapi.PullRequest, error) {
	f.createCalls++
	f.lastCreateInput = input
	if f.createPR == nil {
		return &ghapi.PullRequest{}, nil
	}
	return f.createPR, nil
}

func (f *fakePRClient) UpdatePR(_ context.Context, number int, input ghsvc.UpdatePRInput) (*ghapi.PullRequest, error) {
	f.updateCalls++
	f.lastUpdateNumber = number
	f.lastUpdateInput = input
	return &ghapi.PullRequest{}, nil
}

type fakeReviewStore struct {
	core.Store

	mu      sync.Mutex
	records map[string][]core.ReviewRecord
	queries []string
}

func newFakeReviewStore() *fakeReviewStore {
	return &fakeReviewStore{
		records: make(map[string][]core.ReviewRecord),
	}
}

func (s *fakeReviewStore) SaveReviewRecord(record *core.ReviewRecord) error {
	if record == nil {
		return errors.New("record is nil")
	}
	issueID := strings.TrimSpace(record.IssueID)
	if issueID == "" {
		return errors.New("issue id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := *record
	cloned.IssueID = issueID
	cloned.Issues = append([]core.ReviewIssue(nil), record.Issues...)
	cloned.Fixes = append([]core.ProposedFix(nil), record.Fixes...)
	s.records[issueID] = append(s.records[issueID], cloned)
	return nil
}

func (s *fakeReviewStore) GetReviewRecords(issueID string) ([]core.ReviewRecord, error) {
	normalized := strings.TrimSpace(issueID)
	if normalized == "" {
		return nil, errors.New("issue id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.queries = append(s.queries, normalized)

	records := s.records[normalized]
	out := make([]core.ReviewRecord, 0, len(records))
	for _, record := range records {
		cp := record
		cp.Issues = append([]core.ReviewIssue(nil), record.Issues...)
		cp.Fixes = append([]core.ProposedFix(nil), record.Fixes...)
		out = append(out, cp)
	}
	return out, nil
}

func (s *fakeReviewStore) queriedIssueIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, len(s.queries))
	copy(out, s.queries)
	return out
}

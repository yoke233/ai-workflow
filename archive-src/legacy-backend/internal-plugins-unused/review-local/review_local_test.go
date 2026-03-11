package reviewlocal

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestLocalReviewGate_NameInitClose(t *testing.T) {
	store, _ := newTestStoreWithIssue(t)
	gate := New(store)

	if got := gate.Name(); got != "local" {
		t.Fatalf("Name() = %q, want %q", got, "local")
	}
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := gate.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLocalReviewGate_SubmitCheckCancelFlow(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	gate := New(store)

	reviewID, err := gate.Submit(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if reviewID != issue.ID {
		t.Fatalf("Submit() reviewID = %q, want %q", reviewID, issue.ID)
	}

	pending, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after submit error = %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("pending status = %q, want %q", pending.Status, "pending")
	}
	if pending.Decision != "pending" {
		t.Fatalf("pending decision = %q, want %q", pending.Decision, "pending")
	}
	if len(pending.Verdicts) != 1 || pending.Verdicts[0].Status != "pending" {
		t.Fatalf("pending verdicts = %#v, want one pending verdict", pending.Verdicts)
	}

	records, err := store.GetReviewRecords(issue.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() after submit error = %v", err)
	}
	if len(records) != 1 || records[0].Verdict != "pending" {
		t.Fatalf("review records after submit = %#v, want one pending record", records)
	}
	if records[0].IssueID != issue.ID {
		t.Fatalf("review record issueID = %q, want %q", records[0].IssueID, issue.ID)
	}

	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	cancelled, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after cancel error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want %q", cancelled.Status, "cancelled")
	}
	if cancelled.Decision != "cancelled" {
		t.Fatalf("cancelled decision = %q, want %q", cancelled.Decision, "cancelled")
	}
}

func TestLocalReviewGate_CheckReadsLatestVerdict(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	gate := New(store)

	reviewID, err := gate.Submit(context.Background(), []*core.Issue{issue})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := store.SaveReviewRecord(&core.ReviewRecord{
		IssueID:  issue.ID,
		Round:    1,
		Reviewer: "local_human",
		Verdict:  "approved",
	}); err != nil {
		t.Fatalf("SaveReviewRecord(approved) error = %v", err)
	}

	result, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != "approved" {
		t.Fatalf("status = %q, want %q", result.Status, "approved")
	}
	if result.Decision != "approve" {
		t.Fatalf("decision = %q, want %q", result.Decision, "approve")
	}
}

func TestLocalReviewGate_Boundaries(t *testing.T) {
	store, issue := newTestStoreWithIssue(t)
	gate := New(store)

	if _, err := gate.Submit(context.Background(), nil); err == nil {
		t.Fatalf("expected Submit(nil) to fail")
	}
	if _, err := gate.Submit(context.Background(), []*core.Issue{}); err == nil {
		t.Fatalf("expected Submit(empty issues) to fail")
	}
	if _, err := gate.Submit(context.Background(), []*core.Issue{nil}); err == nil {
		t.Fatalf("expected Submit(nil issue) to fail")
	}
	if _, err := gate.Submit(context.Background(), []*core.Issue{{}}); err == nil {
		t.Fatalf("expected Submit(issue without id) to fail")
	}
	if _, err := gate.Check(context.Background(), ""); err == nil {
		t.Fatalf("expected Check(empty reviewID) to fail")
	}
	if _, err := gate.Check(context.Background(), "issue-unknown"); err == nil {
		t.Fatalf("expected Check(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), ""); err == nil {
		t.Fatalf("expected Cancel(empty reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), "issue-unknown"); err == nil {
		t.Fatalf("expected Cancel(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), issue.ID); err == nil {
		t.Fatalf("expected Cancel(review without submit) to fail")
	}
}

func newTestStoreWithIssue(t *testing.T) (*fakeReviewStore, *core.Issue) {
	t.Helper()

	store := newFakeReviewStore()
	issue := &core.Issue{
		ID:        "issue-20260302-localreview",
		SessionID: "sess-localreview",
		Title:     "local review issue",
		Template:  "default",
	}
	return store, issue
}

type fakeReviewStore struct {
	mu      sync.Mutex
	records map[string][]core.ReviewRecord
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

	records := s.records[normalized]
	out := make([]core.ReviewRecord, len(records))
	copy(out, records)
	return out, nil
}

package trackergithub

import (
	"context"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestGitHubTracker_CreateIssue_CreatesIssueAndExternalID(t *testing.T) {
	stub := &stubIssueService{
		createIssueNumber: 101,
	}
	tracker := newWithIssueService(stub)

	issue := &core.Issue{
		ID:          "issue-gh-5",
		Title:       "实现 tracker-github",
		Body:        "把 issue 状态镜像到 GitHub issue",
		Template:    "standard",
		Status:      core.IssueStatusReady,
		Labels:      []string{"type:feature"},
		Attachments: []string{"logs/build.log", "artifacts/report.json"},
	}

	externalID, err := tracker.CreateIssue(context.Background(), issue)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if externalID != "101" {
		t.Fatalf("CreateIssue() externalID = %q, want %q", externalID, "101")
	}

	if stub.createTitle != issue.Title {
		t.Fatalf("CreateIssue() title = %q, want %q", stub.createTitle, issue.Title)
	}
	if !strings.Contains(stub.createBody, issue.Body) {
		t.Fatalf("CreateIssue() body = %q, should contain %q", stub.createBody, issue.Body)
	}
	if !strings.Contains(stub.createBody, "Attachments:") {
		t.Fatalf("CreateIssue() body = %q, should contain attachment summary", stub.createBody)
	}
	assertLabelsContain(t, stub.createLabels, "type:feature", "template: standard", "status: ready")
}

func TestGitHubTracker_UpdateStatus_Done_ClosesIssue(t *testing.T) {
	stub := &stubIssueService{}
	tracker := newWithIssueService(stub)

	if err := tracker.UpdateStatus(context.Background(), "42", core.IssueStatusDone); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if len(stub.updateLabelsCalls) != 1 {
		t.Fatalf("UpdateIssueLabels() calls = %d, want 1", len(stub.updateLabelsCalls))
	}
	call := stub.updateLabelsCalls[0]
	if call.issueNumber != 42 {
		t.Fatalf("UpdateIssueLabels() issueNumber = %d, want 42", call.issueNumber)
	}
	assertLabelsContain(t, call.labels, "status: done")

	if len(stub.closeIssueCalls) != 1 {
		t.Fatalf("CloseIssue() calls = %d, want 1", len(stub.closeIssueCalls))
	}
	if stub.closeIssueCalls[0] != 42 {
		t.Fatalf("CloseIssue() issueNumber = %d, want 42", stub.closeIssueCalls[0])
	}
}

func TestGitHubTracker_UpdateStatus_Failed_SetsFailedLabel(t *testing.T) {
	stub := &stubIssueService{}
	tracker := newWithIssueService(stub)

	if err := tracker.UpdateStatus(context.Background(), "77", core.IssueStatusFailed); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if len(stub.updateLabelsCalls) != 1 {
		t.Fatalf("UpdateIssueLabels() calls = %d, want 1", len(stub.updateLabelsCalls))
	}
	call := stub.updateLabelsCalls[0]
	if call.issueNumber != 77 {
		t.Fatalf("UpdateIssueLabels() issueNumber = %d, want 77", call.issueNumber)
	}
	assertLabelsContain(t, call.labels, "status: failed")

	if len(stub.closeIssueCalls) != 0 {
		t.Fatalf("CloseIssue() calls = %d, want 0", len(stub.closeIssueCalls))
	}
}

func TestGitHubTracker_SyncDependencies_ReadyAndBlockedLabels(t *testing.T) {
	stub := &stubIssueService{}
	tracker := newWithIssueService(stub)

	issue := &core.Issue{
		ID:         "task-main",
		ExternalID: "200",
		DependsOn:  []string{"task-a", "task-b"},
	}
	allIssues := []*core.Issue{
		{ID: "task-a", ExternalID: "11", Status: core.IssueStatusDone},
		{ID: "task-b", ExternalID: "12", Status: core.IssueStatusQueued},
		{ID: "task-main", ExternalID: "200", Status: core.IssueStatusQueued},
	}

	if err := tracker.SyncDependencies(context.Background(), issue, allIssues); err != nil {
		t.Fatalf("SyncDependencies(blocked) error = %v", err)
	}

	if len(stub.updateLabelsCalls) != 1 {
		t.Fatalf("UpdateIssueLabels() blocked calls = %d, want 1", len(stub.updateLabelsCalls))
	}
	first := stub.updateLabelsCalls[0]
	assertLabelsContain(t, first.labels, "depends-on-#11", "depends-on-#12", "status: blocked")

	allIssues[1].Status = core.IssueStatusDone
	if err := tracker.SyncDependencies(context.Background(), issue, allIssues); err != nil {
		t.Fatalf("SyncDependencies(ready) error = %v", err)
	}

	if len(stub.updateLabelsCalls) != 2 {
		t.Fatalf("UpdateIssueLabels() total calls = %d, want 2", len(stub.updateLabelsCalls))
	}
	second := stub.updateLabelsCalls[1]
	assertLabelsContain(t, second.labels, "depends-on-#11", "depends-on-#12", "status: ready")
}

type updateLabelsCall struct {
	issueNumber int
	labels      []string
}

type stubIssueService struct {
	createIssueNumber int
	createIssueErr    error

	updateIssueErr error
	closeIssueErr  error

	createTitle  string
	createBody   string
	createLabels []string

	updateLabelsCalls []updateLabelsCall
	closeIssueCalls   []int
}

func (s *stubIssueService) CreateIssue(_ context.Context, title, body string, labels []string) (int, error) {
	s.createTitle = title
	s.createBody = body
	s.createLabels = append([]string(nil), labels...)
	if s.createIssueErr != nil {
		return 0, s.createIssueErr
	}
	return s.createIssueNumber, nil
}

func (s *stubIssueService) UpdateIssueLabels(_ context.Context, issueNumber int, labels []string) error {
	s.updateLabelsCalls = append(s.updateLabelsCalls, updateLabelsCall{
		issueNumber: issueNumber,
		labels:      append([]string(nil), labels...),
	})
	return s.updateIssueErr
}

func (s *stubIssueService) CloseIssue(_ context.Context, issueNumber int) error {
	s.closeIssueCalls = append(s.closeIssueCalls, issueNumber)
	return s.closeIssueErr
}

func assertLabelsContain(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, label := range want {
		if !containsLabel(got, label) {
			t.Fatalf("labels = %v, missing %q", got, label)
		}
	}
}

func containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

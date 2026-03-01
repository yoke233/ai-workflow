package trackergithub

import (
	"context"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestGitHubTracker_CreateTask_CreatesIssueAndExternalID(t *testing.T) {
	stub := &stubIssueService{
		createIssueNumber: 101,
	}
	tracker := newWithIssueService(stub)

	item := &core.TaskItem{
		ID:          "task-gh-5",
		PlanID:      "plan-wave2",
		Title:       "实现 tracker-github",
		Description: "把 task 状态镜像到 GitHub issue",
		Template:    "standard",
		Status:      core.ItemReady,
		Labels:      []string{"type:feature"},
	}

	externalID, err := tracker.CreateTask(context.Background(), item)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if externalID != "101" {
		t.Fatalf("CreateTask() externalID = %q, want %q", externalID, "101")
	}

	if stub.createTitle != item.Title {
		t.Fatalf("CreateTask() title = %q, want %q", stub.createTitle, item.Title)
	}
	if stub.createBody != item.Description {
		t.Fatalf("CreateTask() body = %q, want %q", stub.createBody, item.Description)
	}
	assertLabelsContain(t, stub.createLabels, "type:feature", "plan: plan-wave2", "template: standard", "status: ready")
}

func TestGitHubTracker_UpdateStatus_Done_ClosesIssue(t *testing.T) {
	stub := &stubIssueService{}
	tracker := newWithIssueService(stub)

	if err := tracker.UpdateStatus(context.Background(), "42", core.ItemDone); err != nil {
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

	if err := tracker.UpdateStatus(context.Background(), "77", core.ItemFailed); err != nil {
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

	item := &core.TaskItem{
		ID:         "task-main",
		ExternalID: "200",
		DependsOn:  []string{"task-a", "task-b"},
	}
	allItems := []core.TaskItem{
		{ID: "task-a", ExternalID: "11", Status: core.ItemDone},
		{ID: "task-b", ExternalID: "12", Status: core.ItemPending},
		{ID: "task-main", ExternalID: "200", Status: core.ItemPending},
	}

	if err := tracker.SyncDependencies(context.Background(), item, allItems); err != nil {
		t.Fatalf("SyncDependencies(blocked) error = %v", err)
	}

	if len(stub.updateLabelsCalls) != 1 {
		t.Fatalf("UpdateIssueLabels() blocked calls = %d, want 1", len(stub.updateLabelsCalls))
	}
	first := stub.updateLabelsCalls[0]
	assertLabelsContain(t, first.labels, "depends-on-#11", "depends-on-#12", "status: blocked")

	allItems[1].Status = core.ItemDone
	if err := tracker.SyncDependencies(context.Background(), item, allItems); err != nil {
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

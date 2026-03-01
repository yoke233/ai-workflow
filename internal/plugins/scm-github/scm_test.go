package scmgithub

import (
	"context"
	"strings"
	"testing"

	ghapi "github.com/google/go-github/v68/github"
	"github.com/user/ai-workflow/internal/core"
	ghsvc "github.com/user/ai-workflow/internal/github"
)

func TestGitHubSCM_CreatePR_DraftSuccess(t *testing.T) {
	local := &mockLocalSCM{}
	service := &mockPRService{
		createPRResult: &ghapi.PullRequest{
			Number:  ghapi.Ptr(42),
			HTMLURL: ghapi.Ptr("https://github.com/acme/demo/pull/42"),
		},
	}

	scm := NewWithDeps(local, service, Options{
		DefaultDraft:     true,
		DefaultReviewers: []string{"alice", "bob"},
	})

	prURL, err := scm.CreatePR(context.Background(), core.PullRequest{
		Title: "feat: add gh-6",
		Body:  "implement scm-github",
		Head:  "feature/gh-6",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if prURL != "https://github.com/acme/demo/pull/42" {
		t.Fatalf("expected PR URL %q, got %q", "https://github.com/acme/demo/pull/42", prURL)
	}
	if !service.createPRInput.Draft {
		t.Fatal("expected default draft=true to be applied")
	}
	if service.addIssueCommentNumber != 42 {
		t.Fatalf("expected reviewers comment on PR #42, got #%d", service.addIssueCommentNumber)
	}
	if !strings.Contains(service.addIssueCommentBody, "@alice") || !strings.Contains(service.addIssueCommentBody, "@bob") {
		t.Fatalf("expected reviewers in comment body, got %q", service.addIssueCommentBody)
	}
}

func TestGitHubSCM_UpdatePR_AddComment(t *testing.T) {
	local := &mockLocalSCM{}
	service := &mockPRService{}
	scm := NewWithDeps(local, service, Options{})

	nextTitle := "feat: gh-6 v2"
	nextBody := "append updates"
	err := scm.UpdatePR(context.Background(), core.PullRequestUpdate{
		Number:     12,
		Title:      &nextTitle,
		Body:       &nextBody,
		AddComment: "补充说明：已完成 fixup。",
	})
	if err != nil {
		t.Fatalf("UpdatePR returned error: %v", err)
	}
	if service.updatePRNumber != 12 {
		t.Fatalf("expected update on PR #12, got #%d", service.updatePRNumber)
	}
	if service.updatePRInput.Title == nil || *service.updatePRInput.Title != nextTitle {
		t.Fatalf("expected title update %q, got %#v", nextTitle, service.updatePRInput.Title)
	}
	if service.updatePRInput.Body == nil || *service.updatePRInput.Body != nextBody {
		t.Fatalf("expected body update %q, got %#v", nextBody, service.updatePRInput.Body)
	}
	if service.addIssueCommentNumber != 12 {
		t.Fatalf("expected comment on PR #12, got #%d", service.addIssueCommentNumber)
	}
	if service.addIssueCommentBody != "补充说明：已完成 fixup。" {
		t.Fatalf("expected comment body to match, got %q", service.addIssueCommentBody)
	}
}

func TestGitHubSCM_ConvertToReady_Success(t *testing.T) {
	local := &mockLocalSCM{}
	service := &mockPRService{}
	scm := NewWithDeps(local, service, Options{})

	if err := scm.ConvertToReady(context.Background(), 7); err != nil {
		t.Fatalf("ConvertToReady returned error: %v", err)
	}
	if service.updatePRNumber != 7 {
		t.Fatalf("expected UpdatePR called with #7, got #%d", service.updatePRNumber)
	}
	if service.updatePRInput.State == nil || *service.updatePRInput.State != "open" {
		t.Fatalf("expected ConvertToReady to set state=open, got %#v", service.updatePRInput.State)
	}
}

func TestGitHubSCM_MergePR_Success(t *testing.T) {
	local := &mockLocalSCM{}
	service := &mockPRService{}
	scm := NewWithDeps(local, service, Options{})

	err := scm.MergePR(context.Background(), core.PullRequestMerge{
		Number:        99,
		CommitTitle:   "feat: merge gh-6",
		CommitMessage: "squash merge by workflow",
		Method:        "squash",
		SHA:           "abc123",
	})
	if err != nil {
		t.Fatalf("MergePR returned error: %v", err)
	}
	if service.mergePRNumber != 99 {
		t.Fatalf("expected merge on PR #99, got #%d", service.mergePRNumber)
	}
	if service.mergePRInput.CommitTitle != "feat: merge gh-6" {
		t.Fatalf("expected commit title propagated, got %q", service.mergePRInput.CommitTitle)
	}
	if service.mergePRInput.CommitMessage != "squash merge by workflow" {
		t.Fatalf("expected commit message propagated, got %q", service.mergePRInput.CommitMessage)
	}
	if service.mergePRInput.MergeMethod != "squash" {
		t.Fatalf("expected merge method squash, got %q", service.mergePRInput.MergeMethod)
	}
	if service.mergePRInput.SHA != "abc123" {
		t.Fatalf("expected SHA abc123, got %q", service.mergePRInput.SHA)
	}
}

type mockLocalSCM struct{}

func (m *mockLocalSCM) Name() string { return "mock-local" }

func (m *mockLocalSCM) Init(context.Context) error { return nil }

func (m *mockLocalSCM) Close() error { return nil }

func (m *mockLocalSCM) CreateBranch(context.Context, string) error { return nil }

func (m *mockLocalSCM) Commit(context.Context, string) (string, error) { return "", nil }

func (m *mockLocalSCM) Push(context.Context, string, string) error { return nil }

func (m *mockLocalSCM) Merge(context.Context, string) (string, error) { return "", nil }

func (m *mockLocalSCM) CreatePR(context.Context, core.PullRequest) (string, error) { return "", nil }

func (m *mockLocalSCM) UpdatePR(context.Context, core.PullRequestUpdate) error { return nil }

func (m *mockLocalSCM) ConvertToReady(context.Context, int) error { return nil }

func (m *mockLocalSCM) MergePR(context.Context, core.PullRequestMerge) error { return nil }

type mockPRService struct {
	createPRInput         ghsvc.CreatePRInput
	createPRResult        *ghapi.PullRequest
	createPRErr           error
	updatePRNumber        int
	updatePRInput         ghsvc.UpdatePRInput
	updatePRResult        *ghapi.PullRequest
	updatePRErr           error
	mergePRNumber         int
	mergePRInput          ghsvc.MergePRInput
	mergePRResult         *ghapi.PullRequestMergeResult
	mergePRErr            error
	addIssueCommentNumber int
	addIssueCommentBody   string
	addIssueCommentErr    error
}

func (m *mockPRService) CreatePR(_ context.Context, input ghsvc.CreatePRInput) (*ghapi.PullRequest, error) {
	m.createPRInput = input
	if m.createPRResult == nil {
		m.createPRResult = &ghapi.PullRequest{
			Number:  ghapi.Ptr(1),
			HTMLURL: ghapi.Ptr("https://github.com/acme/demo/pull/1"),
		}
	}
	return m.createPRResult, m.createPRErr
}

func (m *mockPRService) UpdatePR(_ context.Context, number int, input ghsvc.UpdatePRInput) (*ghapi.PullRequest, error) {
	m.updatePRNumber = number
	m.updatePRInput = input
	if m.updatePRResult == nil {
		m.updatePRResult = &ghapi.PullRequest{
			Number: ghapi.Ptr(number),
		}
	}
	return m.updatePRResult, m.updatePRErr
}

func (m *mockPRService) MergePR(_ context.Context, number int, input ghsvc.MergePRInput) (*ghapi.PullRequestMergeResult, error) {
	m.mergePRNumber = number
	m.mergePRInput = input
	if m.mergePRResult == nil {
		m.mergePRResult = &ghapi.PullRequestMergeResult{
			Merged: ghapi.Ptr(true),
		}
	}
	return m.mergePRResult, m.mergePRErr
}

func (m *mockPRService) AddIssueComment(_ context.Context, issueNumber int, body string) (*ghapi.IssueComment, error) {
	m.addIssueCommentNumber = issueNumber
	m.addIssueCommentBody = body
	return &ghapi.IssueComment{}, m.addIssueCommentErr
}

package scmgithub

import (
	"context"
	"fmt"
	"strings"

	ghapi "github.com/google/go-github/v68/github"
	"github.com/user/ai-workflow/internal/core"
	ghsvc "github.com/user/ai-workflow/internal/github"
	scmlocalgit "github.com/user/ai-workflow/internal/plugins/scm-local-git"
)

var _ core.SCM = (*GitHubSCM)(nil)

type localGitOps interface {
	Name() string
	Init(ctx context.Context) error
	Close() error
	CreateBranch(ctx context.Context, branch string) error
	Commit(ctx context.Context, message string) (string, error)
	Push(ctx context.Context, remote string, branch string) error
	Merge(ctx context.Context, branch string) (string, error)
}

type prService interface {
	CreatePR(ctx context.Context, input ghsvc.CreatePRInput) (*ghapi.PullRequest, error)
	UpdatePR(ctx context.Context, number int, input ghsvc.UpdatePRInput) (*ghapi.PullRequest, error)
	MergePR(ctx context.Context, number int, input ghsvc.MergePRInput) (*ghapi.PullRequestMergeResult, error)
	AddIssueComment(ctx context.Context, issueNumber int, body string) (*ghapi.IssueComment, error)
}

type Options struct {
	DefaultDraft     bool
	DefaultReviewers []string
}

type GitHubSCM struct {
	local            localGitOps
	service          prService
	defaultDraft     bool
	defaultReviewers []string
}

func New(repoDir string, service *ghsvc.GitHubService, opts Options) *GitHubSCM {
	return NewWithDeps(scmlocalgit.New(repoDir), service, opts)
}

func NewWithDeps(local localGitOps, service prService, opts Options) *GitHubSCM {
	if local == nil {
		local = scmlocalgit.New(".")
	}
	return &GitHubSCM{
		local:            local,
		service:          service,
		defaultDraft:     opts.DefaultDraft,
		defaultReviewers: normalizeReviewers(opts.DefaultReviewers),
	}
}

func (s *GitHubSCM) Name() string {
	return "scm-github"
}

func (s *GitHubSCM) Init(ctx context.Context) error {
	if s.local == nil {
		return fmt.Errorf("github scm init: local git delegate is required")
	}
	if err := s.local.Init(ctx); err != nil {
		return err
	}
	return s.requireService("init")
}

func (s *GitHubSCM) Close() error {
	if s.local == nil {
		return nil
	}
	return s.local.Close()
}

func (s *GitHubSCM) CreateBranch(ctx context.Context, branch string) error {
	if s.local == nil {
		return fmt.Errorf("github scm create branch: local git delegate is required")
	}
	return s.local.CreateBranch(ctx, branch)
}

func (s *GitHubSCM) Commit(ctx context.Context, message string) (string, error) {
	if s.local == nil {
		return "", fmt.Errorf("github scm commit: local git delegate is required")
	}
	return s.local.Commit(ctx, message)
}

func (s *GitHubSCM) Push(ctx context.Context, remote string, branch string) error {
	if s.local == nil {
		return fmt.Errorf("github scm push: local git delegate is required")
	}
	return s.local.Push(ctx, remote, branch)
}

func (s *GitHubSCM) Merge(ctx context.Context, branch string) (string, error) {
	if s.local == nil {
		return "", fmt.Errorf("github scm merge: local git delegate is required")
	}
	return s.local.Merge(ctx, branch)
}

func (s *GitHubSCM) CreatePR(ctx context.Context, req core.PullRequest) (string, error) {
	if err := s.requireService("create pr"); err != nil {
		return "", err
	}

	draft := s.defaultDraft
	if req.Draft != nil {
		draft = *req.Draft
	}

	pr, err := s.service.CreatePR(ctx, ghsvc.CreatePRInput{
		Title: req.Title,
		Body:  req.Body,
		Head:  req.Head,
		Base:  req.Base,
		Draft: draft,
	})
	if err != nil {
		return "", err
	}

	reviewers := normalizeReviewers(req.Reviewers)
	if len(reviewers) == 0 {
		reviewers = s.defaultReviewers
	}
	if len(reviewers) > 0 && pr.GetNumber() > 0 {
		if _, err := s.service.AddIssueComment(ctx, pr.GetNumber(), formatReviewerComment(reviewers)); err != nil {
			return "", err
		}
	}

	return strings.TrimSpace(pr.GetHTMLURL()), nil
}

func (s *GitHubSCM) UpdatePR(ctx context.Context, req core.PullRequestUpdate) error {
	if err := s.requireService("update pr"); err != nil {
		return err
	}
	if req.Number <= 0 {
		return fmt.Errorf("github scm update pr: number must be positive")
	}

	if hasPRUpdates(req) {
		if _, err := s.service.UpdatePR(ctx, req.Number, ghsvc.UpdatePRInput{
			Title:               req.Title,
			Body:                req.Body,
			Base:                req.Base,
			State:               req.State,
			MaintainerCanModify: req.MaintainerCanModify,
		}); err != nil {
			return err
		}
	}

	if comment := strings.TrimSpace(req.AddComment); comment != "" {
		if _, err := s.service.AddIssueComment(ctx, req.Number, comment); err != nil {
			return err
		}
	}
	return nil
}

func (s *GitHubSCM) ConvertToReady(ctx context.Context, number int) error {
	if err := s.requireService("convert to ready"); err != nil {
		return err
	}
	if number <= 0 {
		return fmt.Errorf("github scm convert to ready: number must be positive")
	}

	stateOpen := "open"
	_, err := s.service.UpdatePR(ctx, number, ghsvc.UpdatePRInput{
		State: &stateOpen,
	})
	return err
}

func (s *GitHubSCM) MergePR(ctx context.Context, req core.PullRequestMerge) error {
	if err := s.requireService("merge pr"); err != nil {
		return err
	}
	if req.Number <= 0 {
		return fmt.Errorf("github scm merge pr: number must be positive")
	}

	_, err := s.service.MergePR(ctx, req.Number, ghsvc.MergePRInput{
		CommitTitle:        req.CommitTitle,
		CommitMessage:      req.CommitMessage,
		MergeMethod:        req.Method,
		SHA:                req.SHA,
		DontDefaultIfBlank: req.DontDefaultIfBlank,
	})
	return err
}

func (s *GitHubSCM) requireService(operation string) error {
	if s == nil || s.service == nil {
		return fmt.Errorf("github scm %s: github service is required", operation)
	}
	return nil
}

func hasPRUpdates(req core.PullRequestUpdate) bool {
	return req.Title != nil || req.Body != nil || req.Base != nil || req.State != nil || req.MaintainerCanModify != nil
}

func normalizeReviewers(reviewers []string) []string {
	if len(reviewers) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(reviewers))
	out := make([]string, 0, len(reviewers))
	for _, reviewer := range reviewers {
		trimmed := strings.TrimSpace(strings.TrimPrefix(reviewer, "@"))
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func formatReviewerComment(reviewers []string) string {
	mentions := make([]string, 0, len(reviewers))
	for _, reviewer := range reviewers {
		mentions = append(mentions, "@"+reviewer)
	}
	return "Reviewers: " + strings.Join(mentions, " ")
}

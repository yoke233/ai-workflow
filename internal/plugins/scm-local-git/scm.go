package scmlocalgit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/user/ai-workflow/internal/core"
	gitops "github.com/user/ai-workflow/internal/git"
)

var _ core.SCM = (*LocalGitSCM)(nil)

type LocalGitSCM struct {
	repoDir string
	runner  *gitops.Runner
}

func New(repoDir string) *LocalGitSCM {
	return &LocalGitSCM{
		repoDir: repoDir,
		runner:  gitops.NewRunner(repoDir),
	}
}

func (s *LocalGitSCM) Name() string {
	return "local-git"
}

func (s *LocalGitSCM) Init(ctx context.Context) error {
	_, err := s.run(ctx, "rev-parse", "--is-inside-work-tree")
	return err
}

func (s *LocalGitSCM) Close() error {
	return nil
}

func (s *LocalGitSCM) CreateBranch(ctx context.Context, branch string) error {
	name := strings.TrimSpace(branch)
	if name == "" {
		return fmt.Errorf("branch is empty")
	}
	_, err := s.run(ctx, "checkout", "-b", name)
	return err
}

func (s *LocalGitSCM) Commit(ctx context.Context, message string) (string, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("commit message is empty")
	}
	if _, err := s.run(ctx, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := s.run(ctx, "commit", "-m", msg); err != nil {
		return "", err
	}
	return s.run(ctx, "rev-parse", "HEAD")
}

func (s *LocalGitSCM) Push(ctx context.Context, remote string, branch string) error {
	remoteName := strings.TrimSpace(remote)
	branchName := strings.TrimSpace(branch)
	if remoteName == "" {
		return fmt.Errorf("remote is empty")
	}
	if branchName == "" {
		return fmt.Errorf("branch is empty")
	}
	_, err := s.run(ctx, "push", remoteName, branchName)
	return err
}

func (s *LocalGitSCM) Merge(ctx context.Context, branch string) (string, error) {
	name := strings.TrimSpace(branch)
	if name == "" {
		return "", fmt.Errorf("branch is empty")
	}
	if _, err := s.runner.Merge(name); err != nil {
		return "", err
	}
	return s.run(ctx, "rev-parse", "HEAD")
}

func (s *LocalGitSCM) CreatePR(context.Context, core.PullRequest) (string, error) {
	return "", fmt.Errorf("local-git does not support pull request creation")
}

func (s *LocalGitSCM) UpdatePR(context.Context, core.PullRequestUpdate) error {
	return fmt.Errorf("local-git does not support pull request update")
}

func (s *LocalGitSCM) ConvertToReady(context.Context, int) error {
	return fmt.Errorf("local-git does not support pull request ready transition")
}

func (s *LocalGitSCM) MergePR(context.Context, core.PullRequestMerge) error {
	return fmt.Errorf("local-git does not support pull request merge")
}

func (s *LocalGitSCM) run(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", s.repoDir}, args...)...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

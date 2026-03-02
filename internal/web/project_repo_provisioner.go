package web

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	workspaceclone "github.com/user/ai-workflow/internal/plugins/workspace-clone"
)

const (
	projectSourceTypeLocalPath   = "local_path"
	projectSourceTypeLocalNew    = "local_new"
	projectSourceTypeGitHubClone = "github_clone"
)

var (
	reProjectSlug = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// ProjectRepoProvisionInput carries source-specific fields for repository preparation.
type ProjectRepoProvisionInput struct {
	SourceType string

	RepoPath string
	Slug     string

	RemoteURL string
	Ref       string

	Progress func(step, message string)
}

// ProjectRepoProvisionResult contains repository path plus source metadata.
type ProjectRepoProvisionResult struct {
	RepoPath string

	GitHubOwner string
	GitHubRepo  string
}

// ProjectRepoProvisioner prepares a usable repository for project creation.
type ProjectRepoProvisioner interface {
	Provision(ctx context.Context, input ProjectRepoProvisionInput) (ProjectRepoProvisionResult, error)
}

type gitCommandRunner interface {
	Run(ctx context.Context, args ...string) error
}

type shellGitCommandRunner struct{}

func (r shellGitCommandRunner) Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

type projectRepoProvisioner struct {
	reposRoot   string
	gitRunner   gitCommandRunner
	clonePlugin remoteClonePlugin
}

type remoteClonePlugin interface {
	Clone(ctx context.Context, req workspaceclone.CloneRequest) (workspaceclone.CloneResult, error)
}

// NewProjectRepoProvisioner creates a default provisioner. reposRoot can be empty to use ~/.ai-workflow/repos.
func NewProjectRepoProvisioner(reposRoot string) ProjectRepoProvisioner {
	return newProjectRepoProvisioner(reposRoot, shellGitCommandRunner{})
}

func newProjectRepoProvisioner(reposRoot string, gitRunner gitCommandRunner) *projectRepoProvisioner {
	if gitRunner == nil {
		gitRunner = shellGitCommandRunner{}
	}
	return &projectRepoProvisioner{
		reposRoot:   strings.TrimSpace(reposRoot),
		gitRunner:   gitRunner,
		clonePlugin: workspaceclone.NewWithRunner(gitRunner),
	}
}

func (p *projectRepoProvisioner) Provision(ctx context.Context, input ProjectRepoProvisionInput) (ProjectRepoProvisionResult, error) {
	sourceType := strings.TrimSpace(input.SourceType)
	switch sourceType {
	case projectSourceTypeLocalPath:
		return p.provisionLocalPath(input)
	case projectSourceTypeLocalNew:
		return p.provisionLocalNew(ctx, input)
	case projectSourceTypeGitHubClone:
		return p.provisionGitHubClone(ctx, input)
	default:
		return ProjectRepoProvisionResult{}, fmt.Errorf("unsupported source_type: %s", sourceType)
	}
}

func (p *projectRepoProvisioner) provisionLocalPath(input ProjectRepoProvisionInput) (ProjectRepoProvisionResult, error) {
	repoPath := strings.TrimSpace(input.RepoPath)
	if repoPath == "" {
		return ProjectRepoProvisionResult{}, fmt.Errorf("repo_path is required for local_path")
	}
	notifyProvisionProgress(input.Progress, "resolve_local_path", "using submitted local repository path")
	return ProjectRepoProvisionResult{
		RepoPath: filepath.Clean(repoPath),
	}, nil
}

func (p *projectRepoProvisioner) provisionLocalNew(ctx context.Context, input ProjectRepoProvisionInput) (ProjectRepoProvisionResult, error) {
	slug := normalizeProjectSlug(input.Slug)
	if slug == "" {
		return ProjectRepoProvisionResult{}, fmt.Errorf("slug is required for local_new")
	}

	reposRoot, err := p.resolveReposRoot()
	if err != nil {
		return ProjectRepoProvisionResult{}, err
	}
	notifyProvisionProgress(input.Progress, "ensure_repo_root", "ensuring repository root directory")
	if err := os.MkdirAll(reposRoot, 0o755); err != nil {
		return ProjectRepoProvisionResult{}, fmt.Errorf("create repos root: %w", err)
	}

	repoPath := filepath.Join(reposRoot, slug)
	notifyProvisionProgress(input.Progress, "create_directory", "creating local repository directory")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return ProjectRepoProvisionResult{}, fmt.Errorf("create local repository directory: %w", err)
	}

	notifyProvisionProgress(input.Progress, "git_init", "initializing git repository")
	if err := p.gitRunner.Run(ctx, "init", repoPath); err != nil {
		return ProjectRepoProvisionResult{}, err
	}

	return ProjectRepoProvisionResult{
		RepoPath: repoPath,
	}, nil
}

func (p *projectRepoProvisioner) provisionGitHubClone(ctx context.Context, input ProjectRepoProvisionInput) (ProjectRepoProvisionResult, error) {
	remoteURL := strings.TrimSpace(input.RemoteURL)
	ref := strings.TrimSpace(input.Ref)
	if remoteURL == "" {
		return ProjectRepoProvisionResult{}, fmt.Errorf("remote_url is required for github_clone")
	}

	remote, err := workspaceclone.ParseRemoteURL(remoteURL)
	if err != nil {
		return ProjectRepoProvisionResult{}, err
	}

	reposRoot, err := p.resolveReposRoot()
	if err != nil {
		return ProjectRepoProvisionResult{}, err
	}
	notifyProvisionProgress(input.Progress, "ensure_repo_root", "ensuring repository root directory")
	if err := os.MkdirAll(reposRoot, 0o755); err != nil {
		return ProjectRepoProvisionResult{}, fmt.Errorf("create repos root: %w", err)
	}

	targetPath := filepath.Join(reposRoot, buildRemoteRepoKey(remote.Host, remote.Owner, remote.Repo))
	if pathExists(filepath.Join(targetPath, ".git")) {
		notifyProvisionProgress(input.Progress, "update_repository", "updating existing repository from GitHub")
	} else {
		notifyProvisionProgress(input.Progress, "clone_repository", "cloning repository from GitHub")
	}
	if ref != "" {
		notifyProvisionProgress(input.Progress, "checkout_ref", "checking out requested ref")
	}
	cloneResult, err := p.clonePlugin.Clone(ctx, workspaceclone.CloneRequest{
		RemoteURL:  remoteURL,
		TargetPath: targetPath,
		Ref:        ref,
	})
	if err != nil {
		return ProjectRepoProvisionResult{}, err
	}

	repoPath := strings.TrimSpace(cloneResult.RepoPath)
	if repoPath == "" {
		repoPath = targetPath
	}
	owner := strings.TrimSpace(cloneResult.Owner)
	if owner == "" {
		owner = remote.Owner
	}
	repo := strings.TrimSpace(cloneResult.Repo)
	if repo == "" {
		repo = remote.Repo
	}

	return ProjectRepoProvisionResult{
		RepoPath:    repoPath,
		GitHubOwner: owner,
		GitHubRepo:  repo,
	}, nil
}

func (p *projectRepoProvisioner) resolveReposRoot() (string, error) {
	if p.reposRoot != "" {
		return filepath.Clean(p.reposRoot), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for repos root: %w", err)
	}
	return filepath.Join(homeDir, ".ai-workflow", "repos"), nil
}

func normalizeProjectSlug(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}
	normalized = reProjectSlug.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-_")
	return normalized
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func buildRemoteRepoKey(host, owner, repo string) string {
	return normalizeRepoKeyPart(host) + "__" + normalizeRepoKeyPart(owner) + "__" + normalizeRepoKeyPart(repo)
}

func normalizeRepoKeyPart(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, ch := range trimmed {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' {
			b.WriteRune(ch)
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func notifyProvisionProgress(progress func(step, message string), step, message string) {
	if progress == nil {
		return
	}
	progress(strings.TrimSpace(step), strings.TrimSpace(message))
}

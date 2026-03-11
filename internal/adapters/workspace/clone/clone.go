package workspaceclone

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRequest describes the remote repository and local target path.
type CloneRequest struct {
	RemoteURL  string
	TargetPath string
	Ref        string
}

// CloneResult reports clone destination and parsed remote metadata.
type CloneResult struct {
	RepoPath string
	Host     string
	Owner    string
	Repo     string
}

// RemoteMetadata describes owner/repo parsed from remote_url.
type RemoteMetadata struct {
	Host  string
	Owner string
	Repo  string
}

// CommandRunner abstracts git command execution for testing.
type CommandRunner interface {
	Run(ctx context.Context, args ...string) error
}

// ClonePlugin clones or updates repository content from remote_url.
type ClonePlugin struct {
	runner CommandRunner
}

type shellGitRunner struct{}

func (r shellGitRunner) Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return nil
}

// New creates a clone plugin with shell git execution.
func New() *ClonePlugin {
	return NewWithRunner(shellGitRunner{})
}

// NewWithRunner creates a clone plugin with injected git runner.
func NewWithRunner(runner CommandRunner) *ClonePlugin {
	if runner == nil {
		runner = shellGitRunner{}
	}
	return &ClonePlugin{runner: runner}
}

// ParseRemoteURL validates remote_url and extracts repository owner/repo.
func ParseRemoteURL(remoteURL string) (RemoteMetadata, error) {
	info, err := parseRemoteURL(remoteURL)
	if err != nil {
		return RemoteMetadata{}, err
	}
	return RemoteMetadata{
		Host:  info.Host,
		Owner: info.Owner,
		Repo:  info.Repo,
	}, nil
}

// Clone clones remote_url into target path, or fetches updates if target already exists.
func (p *ClonePlugin) Clone(ctx context.Context, req CloneRequest) (CloneResult, error) {
	info, err := parseRemoteURL(req.RemoteURL)
	if err != nil {
		return CloneResult{}, err
	}

	targetPath := strings.TrimSpace(req.TargetPath)
	if targetPath == "" {
		return CloneResult{}, fmt.Errorf("target_path is required")
	}
	targetPath = filepath.Clean(targetPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return CloneResult{}, fmt.Errorf("ensure clone target parent directory: %w", err)
	}

	if pathExists(filepath.Join(targetPath, ".git")) {
		if err := p.runner.Run(ctx, "-C", targetPath, "fetch", "--all", "--prune"); err != nil {
			return CloneResult{}, err
		}
	} else if pathExists(targetPath) {
		return CloneResult{}, fmt.Errorf("clone target exists but is not a git repository: %s", targetPath)
	} else {
		if err := p.runner.Run(ctx, "clone", info.OriginalRemote, targetPath); err != nil {
			return CloneResult{}, err
		}
	}

	if ref := strings.TrimSpace(req.Ref); ref != "" {
		if err := p.runner.Run(ctx, "-C", targetPath, "checkout", ref); err != nil {
			return CloneResult{}, err
		}
	}

	return CloneResult{
		RepoPath: targetPath,
		Host:     info.Host,
		Owner:    info.Owner,
		Repo:     info.Repo,
	}, nil
}

type remoteInfo struct {
	OriginalRemote string
	Host           string
	Owner          string
	Repo           string
}

func parseRemoteURL(raw string) (remoteInfo, error) {
	remoteURL := strings.TrimSpace(raw)
	if remoteURL == "" {
		return remoteInfo{}, fmt.Errorf("remote_url is required")
	}

	if strings.Contains(remoteURL, "://") {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return remoteInfo{}, fmt.Errorf("invalid remote_url %q: %w", remoteURL, err)
		}
		scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
		if scheme != "https" && scheme != "ssh" {
			return remoteInfo{}, fmt.Errorf("invalid remote_url %q: unsupported scheme %q (expect ssh or https)", remoteURL, scheme)
		}
		if strings.TrimSpace(u.Host) == "" {
			return remoteInfo{}, fmt.Errorf("invalid remote_url %q: host is required", remoteURL)
		}
		return buildRemoteInfo(remoteURL, u.Path, strings.ToLower(strings.TrimSpace(u.Host)))
	}

	atIndex := strings.Index(remoteURL, "@")
	colonIndex := strings.LastIndex(remoteURL, ":")
	if atIndex <= 0 || colonIndex <= atIndex+1 || colonIndex >= len(remoteURL)-1 {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: expect ssh/https repository URL", remoteURL)
	}
	host := strings.TrimSpace(remoteURL[atIndex+1 : colonIndex])
	if host == "" {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: host is required", remoteURL)
	}
	pathPart := remoteURL[colonIndex+1:]
	return buildRemoteInfo(remoteURL, pathPart, strings.ToLower(host))
}

func buildRemoteInfo(originalRemote, pathPart, host string) (remoteInfo, error) {
	pathPart = strings.Trim(strings.TrimSpace(pathPart), "/")
	if pathPart == "" {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: repository path is empty", originalRemote)
	}

	rawSegments := strings.Split(pathPart, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		segments = append(segments, trimmed)
	}
	if len(segments) < 2 {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: repository path must include owner/repo", originalRemote)
	}

	repoName := strings.TrimSuffix(segments[len(segments)-1], ".git")
	repoName = strings.TrimSpace(repoName)
	if repoName == "" {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: repository name is empty", originalRemote)
	}
	ownerName := strings.TrimSpace(segments[len(segments)-2])
	if ownerName == "" {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: repository owner is empty", originalRemote)
	}
	if strings.TrimSpace(host) == "" {
		return remoteInfo{}, fmt.Errorf("invalid remote_url %q: host is required", originalRemote)
	}

	return remoteInfo{
		OriginalRemote: originalRemote,
		Host:           host,
		Owner:          ownerName,
		Repo:           repoName,
	}, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

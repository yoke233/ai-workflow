package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	repoDir string
}

func NewRunner(repoDir string) *Runner {
	return &Runner{repoDir: repoDir}
}

// RemoteURL returns the fetch URL for the given remote name (e.g. "origin").
func (r *Runner) RemoteURL(name string) (string, error) {
	return r.run("remote", "get-url", name)
}

// HasRemote checks whether the named remote (e.g. "origin") exists.
func (r *Runner) HasRemote(name string) bool {
	_, err := r.run("remote", "get-url", name)
	return err == nil
}

// Fetch fetches all refs from the named remote (e.g. "origin").
// It's a no-op if the remote doesn't exist (pure local repo).
func (r *Runner) Fetch(remote string) error {
	if !r.HasRemote(remote) {
		return nil
	}
	_, err := r.run("fetch", remote, "--prune")
	return err
}

// RefExists checks whether the given ref (branch, tag, etc.) exists locally.
func (r *Runner) RefExists(ref string) bool {
	_, err := r.run("rev-parse", "--verify", ref)
	return err == nil
}

func (r *Runner) run(args ...string) (string, error) {
	stdout, stderr, _, err := r.runRaw(args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr), err)
	}
	return strings.TrimSpace(stdout), nil
}

func (r *Runner) runRaw(args ...string) (string, string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", r.repoDir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), err
		}
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

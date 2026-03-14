package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WorktreeAdd creates a worktree at path on the given branch.
//
// If startPoint is non-empty (e.g. "origin/main"), the new branch is created
// from that ref, ensuring the worktree starts from the latest remote base.
// When the branch already exists, the worktree is checked out and reset to
// the start point so it stays up-to-date.
func (r *Runner) WorktreeAdd(path, branch, startPoint string) error {
	// Build "git worktree add -b <branch> <path> [startPoint]"
	args := []string{"worktree", "add", "-b", branch, path}
	if startPoint != "" {
		args = append(args, startPoint)
	}

	_, err := r.run(args...)
	if err == nil {
		return nil
	}

	// If the branch already exists, remove orphaned directory (if any) and
	// check out the existing branch: "git worktree add <path> <branch>".
	if strings.Contains(err.Error(), "already exists") {
		_ = os.RemoveAll(path) // remove stale empty dir that blocks git
		_, retryErr := r.run("worktree", "add", path, branch)
		if retryErr != nil {
			return retryErr
		}
		// If a start point was given, reset the existing branch to it
		// so the worktree reflects the latest remote base.
		if startPoint != "" {
			_, resetErr := r.runInDir(path, "reset", "--hard", startPoint)
			if resetErr != nil {
				return fmt.Errorf("reset existing worktree to %s: %w", startPoint, resetErr)
			}
		}
		return nil
	}
	return err
}

// runInDir runs a git command in a specific directory rather than r.repoDir.
func (r *Runner) runInDir(dir string, args ...string) (string, error) {
	stdout, stderr, _, err := r.runRawInDir(dir, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr), err)
	}
	return strings.TrimSpace(stdout), nil
}

func (r *Runner) runRawInDir(dir string, args ...string) (string, string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
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

func (r *Runner) WorktreeRemove(path string) error {
	_, err := r.run("worktree", "remove", path, "--force")
	return err
}

func (r *Runner) WorktreeClean(path string) error {
	cmd1 := exec.Command("git", "-C", path, "checkout", ".")
	if err := cmd1.Run(); err != nil {
		return err
	}
	cmd2 := exec.Command("git", "-C", path, "clean", "-fd")
	return cmd2.Run()
}

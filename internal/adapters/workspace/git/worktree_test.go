package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "test-user"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, cmd := range commands {
		c := exec.Command(cmd[0], cmd[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %s (%v)", cmd, out, err)
		}
	}
	return dir
}

func TestWorktreeCreateAndRemove(t *testing.T) {
	repo := setupTestRepo(t)
	runner := NewRunner(repo)

	wtPath := filepath.Join(t.TempDir(), "wt-test")
	branch := "feature/test-wt"

	if err := runner.WorktreeAdd(wtPath, branch); err != nil {
		t.Fatalf("worktree add: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	if err := runner.WorktreeRemove(wtPath); err != nil {
		t.Fatalf("worktree remove: %v", err)
	}
}

package scmlocalgit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalGitSCMNameInitClose(t *testing.T) {
	repoDir := setupTempRepo(t)
	scm := New(repoDir)

	if scm.Name() != "local-git" {
		t.Fatalf("unexpected plugin name: %q", scm.Name())
	}
	if err := scm.Init(context.Background()); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if err := scm.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestLocalGitSCMCreateBranch(t *testing.T) {
	repoDir := setupTempRepo(t)
	scm := New(repoDir)

	branch := "feature/local-git"
	if err := scm.CreateBranch(context.Background(), branch); err != nil {
		t.Fatalf("create branch failed: %v", err)
	}

	current := runGit(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if current != branch {
		t.Fatalf("expected current branch %q, got %q", branch, current)
	}
	exists := runGit(t, repoDir, "branch", "--list", branch)
	if strings.TrimSpace(exists) == "" {
		t.Fatalf("branch %q not found after create", branch)
	}
}

func TestLocalGitSCMCommit(t *testing.T) {
	repoDir := setupTempRepo(t)
	scm := New(repoDir)

	fileName := "feature.txt"
	filePath := filepath.Join(repoDir, fileName)
	if err := os.WriteFile(filePath, []byte("hello local git\n"), 0o644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}

	commitMsg := "feat: add feature file"
	hash, err := scm.Commit(context.Background(), commitMsg)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if hash == "" {
		t.Fatalf("empty commit hash")
	}

	head := runGit(t, repoDir, "rev-parse", "HEAD")
	if head != hash {
		t.Fatalf("expected commit hash %q, got %q", head, hash)
	}
	gotMsg := runGit(t, repoDir, "log", "-1", "--pretty=%s")
	if gotMsg != commitMsg {
		t.Fatalf("expected commit message %q, got %q", commitMsg, gotMsg)
	}
	changedFiles := runGit(t, repoDir, "show", "--pretty=format:", "--name-only", "HEAD")
	if !strings.Contains(changedFiles, fileName) {
		t.Fatalf("expected changed files to include %q, got %q", fileName, changedFiles)
	}
}

func setupTempRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runCmd(t, "git", "init", repoDir)
	runCmd(t, "git", "-C", repoDir, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", repoDir, "config", "user.name", "test-user")

	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write initial file failed: %v", err)
	}
	runCmd(t, "git", "-C", repoDir, "add", "README.md")
	runCmd(t, "git", "-C", repoDir, "commit", "-m", "init")

	return repoDir
}

func runGit(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
	}
	return strings.TrimSpace(string(out))
}

func runCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s (%v)", name, args, string(out), err)
	}
}

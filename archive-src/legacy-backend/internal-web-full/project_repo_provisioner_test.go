package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestProjectRepoProvisionerLocalPathUsesSubmittedRepoPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos-root")
	runner := &recordingGitRunner{}
	provisioner := newProjectRepoProvisioner(root, runner)

	inputPath := filepath.Join(t.TempDir(), "repo-local-path")
	got, err := provisioner.Provision(context.Background(), ProjectRepoProvisionInput{
		SourceType: string(projectSourceTypeLocalPath),
		RepoPath:   inputPath,
	})
	if err != nil {
		t.Fatalf("Provision(local_path) error = %v", err)
	}
	if got.RepoPath != inputPath {
		t.Fatalf("expected repo_path %s, got %s", inputPath, got.RepoPath)
	}
	if len(runner.Calls()) != 0 {
		t.Fatalf("expected no git calls for local_path, got %v", runner.Calls())
	}
}

func TestProjectRepoProvisionerResolveReposRootDefaultsToWorkingDirectory(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)

	provisioner := newProjectRepoProvisioner("", &recordingGitRunner{})
	got, err := provisioner.resolveReposRoot()
	if err != nil {
		t.Fatalf("resolveReposRoot() error = %v", err)
	}

	want := filepath.Join(workingDir, ".ai-workflow", "repos")
	if got != want {
		t.Fatalf("resolveReposRoot() = %s, want %s", got, want)
	}
}

func TestProjectRepoProvisionerLocalNewCreatesGitRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos-root")
	runner := &recordingGitRunner{
		runFn: func(args []string) error {
			if len(args) >= 2 && args[0] == "init" {
				repoPath := args[1]
				return os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)
			}
			return nil
		},
	}
	provisioner := newProjectRepoProvisioner(root, runner)

	got, err := provisioner.Provision(context.Background(), ProjectRepoProvisionInput{
		SourceType: string(projectSourceTypeLocalNew),
		Slug:       "demo-new",
	})
	if err != nil {
		t.Fatalf("Provision(local_new) error = %v", err)
	}

	wantPath := filepath.Join(root, "demo-new")
	if got.RepoPath != wantPath {
		t.Fatalf("expected repo_path %s, got %s", wantPath, got.RepoPath)
	}

	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Fatalf("expected .git directory, stat error: %v", err)
	}

	calls := runner.Calls()
	if !hasGitCall(calls, func(call []string) bool {
		return len(call) == 2 && call[0] == "init" && call[1] == wantPath
	}) {
		t.Fatalf("expected git init %s call, got %v", wantPath, calls)
	}
}

func TestProjectRepoProvisionerLocalNewRequiresSlug(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos-root")
	provisioner := newProjectRepoProvisioner(root, &recordingGitRunner{})

	_, err := provisioner.Provision(context.Background(), ProjectRepoProvisionInput{
		SourceType: string(projectSourceTypeLocalNew),
	})
	if err == nil {
		t.Fatal("expected error when local_new slug is empty")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "slug") {
		t.Fatalf("expected slug validation error, got %v", err)
	}
}

func TestProjectRepoProvisionerGitHubCloneValidatesRemoteURL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos-root")
	provisioner := newProjectRepoProvisioner(root, &recordingGitRunner{})

	cases := []struct {
		name  string
		input ProjectRepoProvisionInput
	}{
		{
			name: "missing remote url",
			input: ProjectRepoProvisionInput{
				SourceType: string(projectSourceTypeGitHubClone),
			},
		},
		{
			name: "unsupported remote scheme",
			input: ProjectRepoProvisionInput{
				SourceType: string(projectSourceTypeGitHubClone),
				RemoteURL:  "ftp://github.com/acme/demo.git",
			},
		},
		{
			name: "missing owner/repo path",
			input: ProjectRepoProvisionInput{
				SourceType: string(projectSourceTypeGitHubClone),
				RemoteURL:  "https://github.com/acme",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := provisioner.Provision(context.Background(), tc.input)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "remote_url") {
				t.Fatalf("expected remote_url diagnostic error, got %v", err)
			}
		})
	}
}

func TestProjectRepoProvisionerGitHubCloneUpdatesExistingRepo(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos-root")
	runner := &recordingGitRunner{
		runFn: func(args []string) error {
			switch {
			case len(args) >= 1 && args[0] == "clone":
				target := args[len(args)-1]
				return os.MkdirAll(filepath.Join(target, ".git"), 0o755)
			case len(args) >= 4 && args[0] == "-C" && args[2] == "fetch":
				return nil
			case len(args) >= 4 && args[0] == "-C" && args[2] == "checkout":
				return nil
			default:
				return nil
			}
		},
	}
	provisioner := newProjectRepoProvisioner(root, runner)
	input := ProjectRepoProvisionInput{
		SourceType: string(projectSourceTypeGitHubClone),
		RemoteURL:  "https://github.com/acme/demo.git",
		Ref:        "main",
	}

	got, err := provisioner.Provision(context.Background(), input)
	if err != nil {
		t.Fatalf("first github clone provision failed: %v", err)
	}
	wantPath := filepath.Join(root, "github.com__acme__demo")
	if got.RepoPath != wantPath {
		t.Fatalf("expected repo_path %s, got %s", wantPath, got.RepoPath)
	}
	firstCalls := runner.Calls()
	if !hasGitCall(firstCalls, func(call []string) bool {
		return len(call) == 3 && call[0] == "clone" && call[1] == input.RemoteURL && call[2] == wantPath
	}) {
		t.Fatalf("expected clone call during first provision, got %v", firstCalls)
	}
	if !hasGitCall(firstCalls, func(call []string) bool {
		return len(call) == 4 && call[0] == "-C" && call[1] == wantPath && call[2] == "checkout" && call[3] == "main"
	}) {
		t.Fatalf("expected checkout call during first provision, got %v", firstCalls)
	}

	runner.Reset()
	got, err = provisioner.Provision(context.Background(), input)
	if err != nil {
		t.Fatalf("second github clone provision failed: %v", err)
	}
	if got.RepoPath != wantPath {
		t.Fatalf("expected repo_path %s, got %s", wantPath, got.RepoPath)
	}
	secondCalls := runner.Calls()
	if hasGitCall(secondCalls, func(call []string) bool {
		return len(call) >= 1 && call[0] == "clone"
	}) {
		t.Fatalf("expected no clone call during update, got %v", secondCalls)
	}
	if !hasGitCall(secondCalls, func(call []string) bool {
		return len(call) == 5 && call[0] == "-C" && call[1] == wantPath && call[2] == "fetch" && call[3] == "--all" && call[4] == "--prune"
	}) {
		t.Fatalf("expected fetch update call, got %v", secondCalls)
	}
	if !hasGitCall(secondCalls, func(call []string) bool {
		return len(call) == 4 && call[0] == "-C" && call[1] == wantPath && call[2] == "checkout" && call[3] == "main"
	}) {
		t.Fatalf("expected checkout call during update, got %v", secondCalls)
	}
}

func hasGitCall(calls [][]string, match func(call []string) bool) bool {
	for _, call := range calls {
		if match(call) {
			return true
		}
	}
	return false
}

type recordingGitRunner struct {
	mu    sync.Mutex
	calls [][]string
	runFn func(args []string) error
}

func (r *recordingGitRunner) Run(_ context.Context, args ...string) error {
	r.mu.Lock()
	cloned := make([]string, len(args))
	copy(cloned, args)
	r.calls = append(r.calls, cloned)
	runFn := r.runFn
	r.mu.Unlock()

	if runFn != nil {
		return runFn(cloned)
	}
	return nil
}

func (r *recordingGitRunner) Calls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, 0, len(r.calls))
	for _, call := range r.calls {
		copied := make([]string, len(call))
		copy(copied, call)
		out = append(out, copied)
	}
	return out
}

func (r *recordingGitRunner) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

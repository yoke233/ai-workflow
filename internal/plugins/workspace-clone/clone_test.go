package workspaceclone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestClonePluginCloneHTTPSRepository(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "repos", "acme__demo")
	runner := &recordingRunner{
		runFn: func(args []string) error {
			if len(args) >= 3 && args[0] == "clone" {
				return os.MkdirAll(filepath.Join(targetPath, ".git"), 0o755)
			}
			return nil
		},
	}
	plugin := NewWithRunner(runner)

	got, err := plugin.Clone(context.Background(), CloneRequest{
		RemoteURL:  "https://github.com/acme/demo.git",
		TargetPath: targetPath,
		Ref:        "main",
	})
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	if got.RepoPath != targetPath {
		t.Fatalf("RepoPath = %q, want %q", got.RepoPath, targetPath)
	}
	if got.Owner != "acme" {
		t.Fatalf("Owner = %q, want %q", got.Owner, "acme")
	}
	if got.Repo != "demo" {
		t.Fatalf("Repo = %q, want %q", got.Repo, "demo")
	}

	calls := runner.Calls()
	if !hasCall(calls, func(call []string) bool {
		return len(call) == 3 &&
			call[0] == "clone" &&
			call[1] == "https://github.com/acme/demo.git" &&
			call[2] == targetPath
	}) {
		t.Fatalf("expected clone call, got %v", calls)
	}
	if !hasCall(calls, func(call []string) bool {
		return len(call) == 4 &&
			call[0] == "-C" &&
			call[1] == targetPath &&
			call[2] == "checkout" &&
			call[3] == "main"
	}) {
		t.Fatalf("expected checkout call, got %v", calls)
	}
}

func TestClonePluginCloneAcceptsSSHRemoteURL(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "repos", "acme__demo")
	runner := &recordingRunner{
		runFn: func(args []string) error {
			if len(args) >= 3 && args[0] == "clone" {
				return os.MkdirAll(filepath.Join(targetPath, ".git"), 0o755)
			}
			return nil
		},
	}
	plugin := NewWithRunner(runner)

	got, err := plugin.Clone(context.Background(), CloneRequest{
		RemoteURL:  "git@github.com:acme/demo.git",
		TargetPath: targetPath,
	})
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	if got.Owner != "acme" || got.Repo != "demo" {
		t.Fatalf("unexpected remote metadata: %+v", got)
	}
}

func TestClonePluginUpdateExistingRepository(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "repos", "acme__demo")
	if err := os.MkdirAll(filepath.Join(targetPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	runner := &recordingRunner{}
	plugin := NewWithRunner(runner)

	_, err := plugin.Clone(context.Background(), CloneRequest{
		RemoteURL:  "https://github.com/acme/demo.git",
		TargetPath: targetPath,
		Ref:        "release/v1",
	})
	if err != nil {
		t.Fatalf("Clone(update) error = %v", err)
	}

	calls := runner.Calls()
	if hasCall(calls, func(call []string) bool { return len(call) > 0 && call[0] == "clone" }) {
		t.Fatalf("expected no clone call for existing repo, got %v", calls)
	}
	if !hasCall(calls, func(call []string) bool {
		return len(call) == 5 &&
			call[0] == "-C" &&
			call[1] == targetPath &&
			call[2] == "fetch" &&
			call[3] == "--all" &&
			call[4] == "--prune"
	}) {
		t.Fatalf("expected fetch call, got %v", calls)
	}
	if !hasCall(calls, func(call []string) bool {
		return len(call) == 4 &&
			call[0] == "-C" &&
			call[1] == targetPath &&
			call[2] == "checkout" &&
			call[3] == "release/v1"
	}) {
		t.Fatalf("expected checkout call, got %v", calls)
	}
}

func TestClonePluginRejectsInvalidRemoteURL(t *testing.T) {
	plugin := New()
	cases := []string{
		"",
		"ftp://github.com/acme/demo.git",
		"github.com/acme/demo",
		"git@github.com",
	}

	for _, remoteURL := range cases {
		t.Run(remoteURL, func(t *testing.T) {
			_, err := plugin.Clone(context.Background(), CloneRequest{
				RemoteURL:  remoteURL,
				TargetPath: filepath.Join(t.TempDir(), "repos", "demo"),
			})
			if err == nil {
				t.Fatal("expected invalid remote_url error")
			}
			message := strings.ToLower(err.Error())
			if !strings.Contains(message, "remote_url") {
				t.Fatalf("error = %v, want diagnostic message containing remote_url", err)
			}
		})
	}
}

func hasCall(calls [][]string, match func([]string) bool) bool {
	for _, call := range calls {
		if match(call) {
			return true
		}
	}
	return false
}

type recordingRunner struct {
	mu    sync.Mutex
	calls [][]string
	runFn func(args []string) error
}

func (r *recordingRunner) Run(_ context.Context, args ...string) error {
	r.mu.Lock()
	copied := make([]string, len(args))
	copy(copied, args)
	r.calls = append(r.calls, copied)
	runFn := r.runFn
	r.mu.Unlock()

	if runFn != nil {
		return runFn(copied)
	}
	return nil
}

func (r *recordingRunner) Calls() [][]string {
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

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
)

func TestBoxLiteSandboxPrepareWrapsLaunch(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	tmpDir := filepath.Join(t.TempDir(), "tmp")
	workDir := filepath.Join(t.TempDir(), "repo")
	for _, dir := range []string{homeDir, tmpDir, workDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	sb := BoxLiteSandbox{
		Base:    NoopSandbox{},
		Command: "boxlite",
		Image:   "ghcr.io/boxlite-ai/macos-dev:latest",
		RunArgs: []string{"--debug"},
		CPUs:    "2",
		Memory:  "4g",
		Network: "shared",
	}
	got, err := sb.Prepare(context.Background(), PrepareInput{
		Launch: acpclient.LaunchConfig{
			Command: "codex-acp",
			Args:    []string{"serve", "--stdio"},
			WorkDir: workDir,
			Env: map[string]string{
				"CODEX_HOME": homeDir,
				"TMPDIR":     tmpDir,
				"TMP":        tmpDir,
				"TEMP":       tmpDir,
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if got.Command != "boxlite" {
		t.Fatalf("wrapped command = %q, want boxlite", got.Command)
	}
	if got.WorkDir != "" {
		t.Fatalf("wrapped workdir = %q, want empty", got.WorkDir)
	}
	if got.Env != nil {
		t.Fatalf("wrapped env should be cleared after -e projection, got=%v", got.Env)
	}
	if !contains(got.Args, "run") || !contains(got.Args, "--rm") || !contains(got.Args, "-i") {
		t.Fatalf("wrapped args missing run flags: %v", got.Args)
	}
	if !containsPair(got.Args, "--cpus", "2") || !containsPair(got.Args, "--memory", "4g") || !containsPair(got.Args, "--network", "shared") {
		t.Fatalf("wrapped args missing resource flags: %v", got.Args)
	}
	if !containsPair(got.Args, "-w", containerWorkDir) {
		t.Fatalf("wrapped args missing rewritten workdir: %v", got.Args)
	}
	if !containsPair(got.Args, "-e", "CODEX_HOME="+containerHomeDir) {
		t.Fatalf("wrapped args missing container home env: %v", got.Args)
	}
	if !containsPair(got.Args, "-e", "TMPDIR="+containerTempDir) {
		t.Fatalf("wrapped args missing container temp env: %v", got.Args)
	}
	if !containsPair(got.Args, "-v", homeDir+":"+containerHomeDir) {
		t.Fatalf("wrapped args missing home mount: %v", got.Args)
	}
	if !containsPair(got.Args, "-v", workDir+":"+containerWorkDir) {
		t.Fatalf("wrapped args missing workdir mount: %v", got.Args)
	}
	if !contains(got.Args, "ghcr.io/boxlite-ai/macos-dev:latest") || !contains(got.Args, "codex-acp") {
		t.Fatalf("wrapped args missing image/program tail: %v", got.Args)
	}
}

func TestBoxLiteSandboxPrepareRequiresImage(t *testing.T) {
	t.Parallel()

	_, err := (BoxLiteSandbox{Command: "boxlite"}).Prepare(context.Background(), PrepareInput{
		Launch: acpclient.LaunchConfig{Command: "codex-acp"},
	})
	if err == nil {
		t.Fatal("Prepare() error = nil, want validation error")
	}
}

package acphandler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	acpproto "github.com/coder/acp-go-sdk"
)

func TestACPHandlerResolveThreadPaths(t *testing.T) {
	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, "workspace")
	archiveDir := filepath.Join(baseDir, "archive")
	mountDir := filepath.Join(baseDir, "project-alpha")
	for _, dir := range []string{workspaceDir, archiveDir, mountDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	h := NewACPHandler(workspaceDir, "", nil)
	h.SetThreadWorkspace(ThreadWorkspaceConfig{
		ThreadID:     1,
		WorkspaceDir: workspaceDir,
		ArchiveDir:   archiveDir,
		Mounts: []ThreadMount{
			{Alias: "project-alpha", TargetPath: mountDir, Access: "check", CheckCommands: []string{"go test ./..."}},
		},
	})

	if _, err := h.resolvePath("notes/todo.md", accessWrite); err != nil {
		t.Fatalf("workspace write should be allowed: %v", err)
	}
	if resolved, err := h.resolvePath("../archive/snapshot.txt", accessRead); err != nil {
		t.Fatalf("archive read should be allowed: %v", err)
	} else if resolved.Zone != pathZoneArchive {
		t.Fatalf("expected archive zone, got %q", resolved.Zone)
	}
	if _, err := h.resolvePath("mounts/project-alpha/README.md", accessRead); err != nil {
		t.Fatalf("mount read should be allowed: %v", err)
	}
	if _, err := h.resolvePath("mounts/project-alpha/README.md", accessWrite); err == nil {
		t.Fatal("mount write should be rejected for check access")
	}
	if _, err := h.resolvePath("../archive/snapshot.txt", accessWrite); err == nil {
		t.Fatal("archive write should be rejected")
	}
}

func TestACPHandlerCreateTerminalChecksWhitelist(t *testing.T) {
	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, "workspace")
	mountDir := filepath.Join(baseDir, "project-alpha")
	for _, dir := range []string{workspaceDir, mountDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	h := NewACPHandler(workspaceDir, "", nil)
	h.SetThreadWorkspace(ThreadWorkspaceConfig{
		ThreadID:     1,
		WorkspaceDir: workspaceDir,
		Mounts: []ThreadMount{
			{Alias: "project-alpha", TargetPath: mountDir, Access: "check", CheckCommands: []string{"go test ./..."}},
		},
	})

	if _, err := h.CreateTerminal(context.Background(), acpproto.CreateTerminalRequest{
		Command: "go",
		Args:    []string{"version"},
		Cwd:     stringPtr("mounts/project-alpha"),
	}); err == nil {
		t.Fatal("expected non-whitelisted command to be rejected")
	}
}

func TestMountAllowsCommand(t *testing.T) {
	mount := &ThreadMount{
		Alias:         "project-alpha",
		Access:        "check",
		CheckCommands: []string{"go test ./...", "npm test"},
	}
	if !mountAllowsCommand(mount, "go", []string{"test", "./..."}) {
		t.Fatal("expected go test ./... to be allowed")
	}
	if !mountAllowsCommand(mount, "go.exe", []string{"test", "./..."}) {
		t.Fatal("expected go.exe test ./... to be allowed")
	}
	if mountAllowsCommand(mount, "go", []string{"build", "./..."}) {
		t.Fatal("expected go build ./... to be rejected")
	}
}

func stringPtr(value string) *string {
	return &value
}

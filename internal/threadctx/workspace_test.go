package threadctx

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	"github.com/yoke233/ai-workflow/internal/core"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(filepath.Join(t.TempDir(), "threadctx.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestResolveMount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	projectID, err := store.CreateProject(ctx, &core.Project{Name: "Project Alpha", Kind: core.ProjectGeneral})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectDir := t.TempDir()
	if _, err := store.CreateResourceBinding(ctx, &core.ResourceBinding{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		URI:       projectDir,
		Config: map[string]any{
			"check_commands": []string{"go test ./..."},
		},
	}); err != nil {
		t.Fatalf("create resource binding: %v", err)
	}

	mount, err := ResolveMount(ctx, store, &core.ThreadContextRef{
		ThreadID:  1,
		ProjectID: projectID,
		Access:    core.ContextAccessCheck,
	})
	if err != nil {
		t.Fatalf("ResolveMount: %v", err)
	}
	if mount.Slug != "project-alpha" {
		t.Fatalf("expected slug project-alpha, got %q", mount.Slug)
	}
	if mount.TargetPath != projectDir {
		t.Fatalf("expected target path %q, got %q", projectDir, mount.TargetPath)
	}
	if len(mount.CheckCommands) != 1 || mount.CheckCommands[0] != "go test ./..." {
		t.Fatalf("unexpected check commands: %+v", mount.CheckCommands)
	}
}

func TestBuildWorkspaceContext(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "Thread Alpha", OwnerID: "owner-1"})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	if _, err := store.AddThreadMember(ctx, &core.ThreadMember{
		ThreadID: threadID,
		Kind:     core.ThreadMemberKindHuman,
		UserID:   "owner-1",
		Role:     "owner",
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	projectID, err := store.CreateProject(ctx, &core.Project{Name: "Project Beta", Kind: core.ProjectGeneral})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := store.CreateResourceBinding(ctx, &core.ResourceBinding{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		URI:       t.TempDir(),
	}); err != nil {
		t.Fatalf("create resource binding: %v", err)
	}
	if _, err := store.CreateThreadContextRef(ctx, &core.ThreadContextRef{
		ThreadID:  threadID,
		ProjectID: projectID,
		Access:    core.ContextAccessRead,
	}); err != nil {
		t.Fatalf("create context ref: %v", err)
	}

	payload, err := BuildWorkspaceContext(ctx, store, t.TempDir(), threadID)
	if err != nil {
		t.Fatalf("BuildWorkspaceContext: %v", err)
	}
	if payload.ThreadID != threadID {
		t.Fatalf("unexpected thread id: %d", payload.ThreadID)
	}
	if payload.Workspace != "." || payload.Archive != "../archive" {
		t.Fatalf("unexpected workspace payload: %+v", payload)
	}
	mount, ok := payload.Mounts["project-beta"]
	if !ok {
		t.Fatalf("expected project-beta mount, got %+v", payload.Mounts)
	}
	if mount.Access != core.ContextAccessRead {
		t.Fatalf("expected read access, got %q", mount.Access)
	}
	if len(payload.Members) != 1 || payload.Members[0] != "owner-1" {
		t.Fatalf("unexpected members: %+v", payload.Members)
	}
}

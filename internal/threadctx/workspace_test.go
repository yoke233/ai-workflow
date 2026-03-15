package threadctx

import (
	"context"
	"errors"
	"os"
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
	if _, err := store.CreateResourceSpace(ctx, &core.ResourceSpace{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		RootURI:   projectDir,
		Config: map[string]any{
			"check_commands": []string{"go test ./..."},
		},
	}); err != nil {
		t.Fatalf("create resource space: %v", err)
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
	if _, err := store.CreateResourceSpace(ctx, &core.ResourceSpace{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		RootURI:   t.TempDir(),
	}); err != nil {
		t.Fatalf("create resource space: %v", err)
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
	if payload.Workspace != "." {
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

func TestSyncContextFileAndLoadContextFileRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	dataDir := t.TempDir()

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

	projectID, _ := store.CreateProject(ctx, &core.Project{Name: "Project Gamma", Kind: core.ProjectGeneral})
	if _, err := store.CreateResourceSpace(ctx, &core.ResourceSpace{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		RootURI:   t.TempDir(),
		Config: map[string]any{
			"check_commands": []any{"go test ./...", "npm test"},
		},
	}); err != nil {
		t.Fatalf("create resource space: %v", err)
	}
	if _, err := store.CreateThreadContextRef(ctx, &core.ThreadContextRef{
		ThreadID:  threadID,
		ProjectID: projectID,
		Access:    core.ContextAccessCheck,
	}); err != nil {
		t.Fatalf("create context ref: %v", err)
	}

	if _, err := SyncContextFile(ctx, store, dataDir, threadID); err != nil {
		t.Fatalf("SyncContextFile: %v", err)
	}
	loaded, err := LoadContextFile(dataDir, threadID)
	if err != nil {
		t.Fatalf("LoadContextFile: %v", err)
	}
	if loaded.ThreadID != threadID {
		t.Fatalf("unexpected thread id: %d", loaded.ThreadID)
	}
	if len(loaded.Mounts["project-gamma"].CheckCommands) != 2 {
		t.Fatalf("expected 2 check commands, got %+v", loaded.Mounts["project-gamma"].CheckCommands)
	}
}

func TestResolveMountUsesGitCloneDirForRemoteBinding(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	projectID, _ := store.CreateProject(ctx, &core.Project{Name: "Remote Repo", Kind: core.ProjectGeneral})
	cloneDir := t.TempDir()
	if _, err := store.CreateResourceSpace(ctx, &core.ResourceSpace{
		ProjectID: projectID,
		Kind:      core.ResourceKindGit,
		RootURI:   "https://github.com/acme/demo.git",
		Config: map[string]any{
			"clone_dir":      cloneDir,
			"check_commands": []string{"go test ./..."},
		},
	}); err != nil {
		t.Fatalf("create git resource space: %v", err)
	}

	mount, err := ResolveMount(ctx, store, &core.ThreadContextRef{
		ThreadID:  1,
		ProjectID: projectID,
		Access:    core.ContextAccessCheck,
	})
	if err != nil {
		t.Fatalf("ResolveMount: %v", err)
	}
	if mount.TargetPath != cloneDir {
		t.Fatalf("expected clone_dir %q, got %q", cloneDir, mount.TargetPath)
	}
}

type threadctxStoreStub struct {
	getThreadErr         error
	getProjectErr        error
	listMembers          []*core.ThreadMember
	listThreadContextRef []*core.ThreadContextRef
	listSpaces           []*core.ResourceSpace
	listMembersErr       error
	listRefsErr          error
	listSpacesErr        error
	project              *core.Project
	projectsByID         map[int64]*core.Project
	spacesByProject      map[int64][]*core.ResourceSpace
	getProjectCalls      int
	listSpacesCalls      int
	batchProjectCalls    int
	batchSpacesCalls     int
}

func (s *threadctxStoreStub) GetThread(context.Context, int64) (*core.Thread, error) {
	if s.getThreadErr != nil {
		return nil, s.getThreadErr
	}
	return &core.Thread{ID: 1, Title: "thread"}, nil
}

func (s *threadctxStoreStub) GetProject(_ context.Context, id int64) (*core.Project, error) {
	s.getProjectCalls++
	if s.getProjectErr != nil {
		return nil, s.getProjectErr
	}
	if len(s.projectsByID) > 0 {
		return s.projectsByID[id], nil
	}
	if s.project != nil {
		return s.project, nil
	}
	return &core.Project{ID: 1, Name: "Project"}, nil
}

func (s *threadctxStoreStub) GetProjectsByID(_ context.Context, ids []int64) (map[int64]*core.Project, error) {
	s.batchProjectCalls++
	out := make(map[int64]*core.Project, len(ids))
	for _, id := range ids {
		if project, ok := s.projectsByID[id]; ok {
			out[id] = project
		}
	}
	return out, s.getProjectErr
}

func (s *threadctxStoreStub) ListThreadMembers(context.Context, int64) ([]*core.ThreadMember, error) {
	return s.listMembers, s.listMembersErr
}

func (s *threadctxStoreStub) ListThreadContextRefs(context.Context, int64) ([]*core.ThreadContextRef, error) {
	return s.listThreadContextRef, s.listRefsErr
}

func (s *threadctxStoreStub) ListThreadAttachments(context.Context, int64) ([]*core.ThreadAttachment, error) {
	return nil, nil
}

func (s *threadctxStoreStub) ListResourceSpaces(context.Context, int64) ([]*core.ResourceSpace, error) {
	s.listSpacesCalls++
	return s.listSpaces, s.listSpacesErr
}

func (s *threadctxStoreStub) ListResourceSpacesByProjects(_ context.Context, projectIDs []int64) (map[int64][]*core.ResourceSpace, error) {
	s.batchSpacesCalls++
	out := make(map[int64][]*core.ResourceSpace, len(projectIDs))
	for _, id := range projectIDs {
		if spaces, ok := s.spacesByProject[id]; ok {
			out[id] = spaces
		}
	}
	return out, s.listSpacesErr
}

func TestBuildWorkspaceContextUsesBatchMountLoaders(t *testing.T) {
	store := &threadctxStoreStub{
		listMembers: []*core.ThreadMember{{UserID: "owner-1"}},
		listThreadContextRef: []*core.ThreadContextRef{
			{ProjectID: 1, Access: core.ContextAccessRead},
			{ProjectID: 2, Access: core.ContextAccessCheck},
		},
		projectsByID: map[int64]*core.Project{
			1: &core.Project{ID: 1, Name: "Project One"},
			2: &core.Project{ID: 2, Name: "Project Two"},
		},
		spacesByProject: map[int64][]*core.ResourceSpace{
			1: {&core.ResourceSpace{ProjectID: 1, Kind: core.ResourceKindLocalFS, RootURI: t.TempDir()}},
			2: {&core.ResourceSpace{ProjectID: 2, Kind: core.ResourceKindLocalFS, RootURI: t.TempDir()}},
		},
	}

	payload, err := BuildWorkspaceContext(context.Background(), store, t.TempDir(), 1)
	if err != nil {
		t.Fatalf("BuildWorkspaceContext() error = %v", err)
	}
	if len(payload.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %+v", payload.Mounts)
	}
	if store.batchProjectCalls != 1 || store.batchSpacesCalls != 1 {
		t.Fatalf("expected batch loaders to be used once, got projects=%d spaces=%d", store.batchProjectCalls, store.batchSpacesCalls)
	}
	if store.getProjectCalls != 0 || store.listSpacesCalls != 0 {
		t.Fatalf("expected fallback loaders to be skipped, got getProject=%d listSpaces=%d", store.getProjectCalls, store.listSpacesCalls)
	}
}

func TestSyncContextFileAvoidsDuplicateMountResolution(t *testing.T) {
	store := &threadctxStoreStub{
		listMembers: []*core.ThreadMember{{UserID: "owner-1"}},
		listThreadContextRef: []*core.ThreadContextRef{
			{ProjectID: 7, Access: core.ContextAccessRead},
		},
		projectsByID: map[int64]*core.Project{
			7: &core.Project{ID: 7, Name: "Project Seven"},
		},
		spacesByProject: map[int64][]*core.ResourceSpace{
			7: {&core.ResourceSpace{ProjectID: 7, Kind: core.ResourceKindLocalFS, RootURI: t.TempDir()}},
		},
	}

	if _, err := SyncContextFile(context.Background(), store, t.TempDir(), 1); err != nil {
		t.Fatalf("SyncContextFile() error = %v", err)
	}
	if store.batchProjectCalls != 1 || store.batchSpacesCalls != 1 {
		t.Fatalf("expected a single mount preload, got projects=%d spaces=%d", store.batchProjectCalls, store.batchSpacesCalls)
	}
	if store.getProjectCalls != 0 || store.listSpacesCalls != 0 {
		t.Fatalf("expected fallback loaders to be skipped, got getProject=%d listSpaces=%d", store.getProjectCalls, store.listSpacesCalls)
	}
}

func TestPathsAndEnsureLayout(t *testing.T) {
	paths := Paths("  "+t.TempDir()+"  ", 42)
	if filepath.Base(paths.ThreadDir) != "42" {
		t.Fatalf("unexpected thread dir: %q", paths.ThreadDir)
	}
	if filepath.Base(paths.ContextFile) != ".context.json" {
		t.Fatalf("unexpected context file: %q", paths.ContextFile)
	}

	got, err := EnsureLayout("", 42)
	if err != nil {
		t.Fatalf("EnsureLayout(empty) error = %v", err)
	}
	if got != (PathsInfo{}) {
		t.Fatalf("EnsureLayout(empty) = %+v, want zero value", got)
	}

	dataDir := t.TempDir()
	got, err = EnsureLayout(dataDir, 99)
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	for _, dir := range []string{got.ThreadDir, got.ProjectsDir, got.AttachmentsDir} {
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("expected directory %q to exist, err=%v", dir, statErr)
		}
	}
}

func TestSyncMountAliasDirsRemovesStaleDirs(t *testing.T) {
	projectsDir := filepath.Join(t.TempDir(), "projects")
	if err := os.MkdirAll(filepath.Join(projectsDir, "stale-project"), 0o755); err != nil {
		t.Fatalf("mkdir stale dir: %v", err)
	}
	payload := &core.ThreadWorkspaceContext{
		Mounts: map[string]core.ThreadWorkspaceMount{
			"project-alpha": {Path: "projects/project-alpha"},
		},
	}
	if err := syncMountAliasDirs(projectsDir, payload, nil); err != nil {
		t.Fatalf("syncMountAliasDirs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectsDir, "project-alpha")); err != nil {
		t.Fatalf("expected fresh mount dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectsDir, "stale-project")); !os.IsNotExist(err) {
		t.Fatalf("expected stale mount dir removed, err=%v", err)
	}
}

func TestSyncMountAliasDirsCreatesMountDirForResolvedTarget(t *testing.T) {
	projectsDir := filepath.Join(t.TempDir(), "projects")
	targetDir := filepath.Join(t.TempDir(), "project-alpha")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	payload := &core.ThreadWorkspaceContext{
		Mounts: map[string]core.ThreadWorkspaceMount{
			"project-alpha": {Path: "projects/project-alpha"},
		},
	}
	if err := syncMountAliasDirs(projectsDir, payload, map[string]string{"project-alpha": targetDir}); err != nil {
		t.Fatalf("syncMountAliasDirs: %v", err)
	}
	info, err := os.Stat(filepath.Join(projectsDir, "project-alpha"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected mount alias dir, err=%v", err)
	}
}

func TestLoadContextFileAndBuildWorkspaceContextErrors(t *testing.T) {
	if _, err := LoadContextFile(t.TempDir(), 1); err == nil {
		t.Fatal("expected missing context file to fail")
	}

	dataDir := t.TempDir()
	paths, err := EnsureLayout(dataDir, 2)
	if err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if err := os.WriteFile(paths.ContextFile, []byte("{broken"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := LoadContextFile(dataDir, 2); err == nil {
		t.Fatal("expected broken context file to fail")
	}

	if _, err := BuildWorkspaceContext(context.Background(), nil, dataDir, 1); err == nil {
		t.Fatal("expected nil store to fail")
	}
	if _, err := SyncContextFile(context.Background(), nil, dataDir, 1); err == nil {
		t.Fatal("expected nil store sync to fail")
	}

	store := &threadctxStoreStub{getThreadErr: core.ErrNotFound}
	if _, err := BuildWorkspaceContext(context.Background(), store, dataDir, 1); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected thread lookup error, got %v", err)
	}
}

func TestBuildWorkspaceContextSkipsBrokenMountsAndDeduplicatesMembers(t *testing.T) {
	store := &threadctxStoreStub{
		project: &core.Project{ID: 7, Name: "Project Name"},
		listMembers: []*core.ThreadMember{
			nil,
			{UserID: "owner-1"},
			{UserID: "owner-1"},
			{UserID: "member-2"},
			{UserID: "   "},
		},
		listThreadContextRef: []*core.ThreadContextRef{
			{ThreadID: 1, ProjectID: 7, Access: core.ContextAccessCheck},
			{ThreadID: 1, ProjectID: 8, Access: core.ContextAccessRead},
		},
		listSpaces: []*core.ResourceSpace{
			{ProjectID: 7, Kind: core.ResourceKindLocalFS, RootURI: t.TempDir()},
		},
	}

	payload, err := BuildWorkspaceContext(context.Background(), store, t.TempDir(), 1)
	if err != nil {
		t.Fatalf("BuildWorkspaceContext() error = %v", err)
	}
	if len(payload.Members) != 2 || payload.Members[0] != "member-2" || payload.Members[1] != "owner-1" {
		t.Fatalf("unexpected members: %+v", payload.Members)
	}
	if len(payload.Mounts) != 1 {
		t.Fatalf("expected broken mount to be skipped, got %+v", payload.Mounts)
	}
}

func TestResolveMountAndHelpersErrors(t *testing.T) {
	store := &threadctxStoreStub{}
	if _, err := ResolveMount(context.Background(), nil, &core.ThreadContextRef{}); err == nil {
		t.Fatal("expected nil store to fail")
	}
	if _, err := ResolveMount(context.Background(), store, nil); err == nil {
		t.Fatal("expected nil ref to fail")
	}

	store.project = &core.Project{ID: 3, Name: "Project"}
	store.listSpaces = []*core.ResourceSpace{{Kind: core.ResourceKindGit, RootURI: "https://example.com/repo.git"}}
	if _, err := ResolveMount(context.Background(), store, &core.ThreadContextRef{ProjectID: 3, Access: core.ContextAccessRead}); err == nil {
		t.Fatal("expected unresolved binding to fail")
	}

	if path, checks := resolveSpaceTarget([]*core.ResourceSpace{
		nil,
		{Kind: core.ResourceKindGit, RootURI: "git@github.com:org/repo.git", Config: map[string]any{"clone_dir": "C:/repo", "check_commands": []any{"go test ./...", "  ", 123}}},
	}); path != "C:/repo" || len(checks) != 1 || checks[0] != "go test ./..." {
		t.Fatalf("unexpected resolveSpaceTarget result: path=%q checks=%v", path, checks)
	}
	if path := resolveGitSpacePath(&core.ResourceSpace{Kind: core.ResourceKindGit, RootURI: "C:/repo"}); path != "C:/repo" {
		t.Fatalf("resolveGitSpacePath(local) = %q", path)
	}
	if !looksLikeRemoteGitURI("git@github.com:org/repo.git") || !looksLikeRemoteGitURI("https://github.com/org/repo.git") {
		t.Fatal("expected remote git uris to be detected")
	}
	if looksLikeRemoteGitURI("C:/repo") {
		t.Fatal("expected local path not to be treated as remote")
	}
	if got := readCheckCommands(map[string]any{"check_commands": []string{"go test ./...", "  "}}); len(got) != 1 {
		t.Fatalf("unexpected []string check commands: %v", got)
	}
	if got := readCheckCommands(map[string]any{"check_commands": "go test ./..."}); got != nil {
		t.Fatalf("expected unsupported check_commands type to be ignored, got %v", got)
	}
}

func TestProjectSlugFallbacks(t *testing.T) {
	if got := projectSlug(nil); got != "project" {
		t.Fatalf("projectSlug(nil) = %q", got)
	}
	if got := projectSlug(&core.Project{ID: 12, Name: "  "}); got != "project-12" {
		t.Fatalf("projectSlug(blank) = %q", got)
	}
	if got := projectSlug(&core.Project{ID: 13, Name: "Hello__World !!"}); got != "hello-world" {
		t.Fatalf("projectSlug(normalized) = %q", got)
	}
}

func TestSyncContextFileEmptyDataDirAndResolveMountErrors(t *testing.T) {
	store := &threadctxStoreStub{}
	payload, err := SyncContextFile(context.Background(), store, "", 1)
	if err != nil {
		t.Fatalf("SyncContextFile(empty data dir) error = %v", err)
	}
	if payload != nil {
		t.Fatalf("SyncContextFile(empty data dir) = %+v, want nil", payload)
	}

	store = &threadctxStoreStub{
		getProjectErr: core.ErrNotFound,
		listSpacesErr: errors.New("bindings failed"),
	}
	if _, err := ResolveMount(context.Background(), store, &core.ThreadContextRef{ProjectID: 1}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected project lookup error, got %v", err)
	}

	store = &threadctxStoreStub{
		project:       &core.Project{ID: 1, Name: "Project"},
		listSpacesErr: errors.New("bindings failed"),
	}
	if _, err := ResolveMount(context.Background(), store, &core.ThreadContextRef{ProjectID: 1}); err == nil || err.Error() != "bindings failed" {
		t.Fatalf("expected space list error, got %v", err)
	}

	if got := resolveGitSpacePath(&core.ResourceSpace{Kind: core.ResourceKindGit, RootURI: "https://example.com/repo.git"}); got != "" {
		t.Fatalf("resolveGitSpacePath(remote without clone dir) = %q, want empty", got)
	}
	if got := resolveGitSpacePath(nil); got != "" {
		t.Fatalf("resolveGitSpacePath(nil) = %q, want empty", got)
	}
}

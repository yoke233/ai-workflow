//go:build probe

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	acphandler "github.com/yoke233/ai-workflow/internal/adapters/agent/acp"
	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/threadctx"
)

func TestThreadWorkspaceRealACP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real ACP thread workspace test in short mode")
	}
	runThreadWorkspaceRealACP(t, codexLaunchConfig)
}

func runThreadWorkspaceRealACP(t *testing.T, build func(string) acpclient.LaunchConfig) {
	t.Helper()

	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "data")
	dbPath := filepath.Join(baseDir, "thread-workspace-real.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "real-thread", OwnerID: "owner-1"})
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

	projectDir := filepath.Join(baseDir, "project-alpha")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("Project Alpha Readme\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/projectalpha\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main_test.go"), []byte("package projectalpha\n\nimport \"testing\"\n\nfunc TestWorkspace(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatalf("write main_test.go: %v", err)
	}

	projectID, err := store.CreateProject(ctx, &core.Project{Name: "Project Alpha", Kind: core.ProjectGeneral})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := store.CreateResourceBinding(ctx, &core.ResourceBinding{
		ProjectID: projectID,
		Kind:      core.ResourceKindLocalFS,
		URI:       projectDir,
		Label:     "workspace",
		Config: map[string]any{
			"check_commands": []string{"go test ./..."},
		},
	}); err != nil {
		t.Fatalf("create resource binding: %v", err)
	}
	if _, err := store.CreateThreadContextRef(ctx, &core.ThreadContextRef{
		ThreadID:  threadID,
		ProjectID: projectID,
		Access:    core.ContextAccessCheck,
	}); err != nil {
		t.Fatalf("create thread context ref: %v", err)
	}

	paths, err := threadctx.EnsureLayout(dataDir, threadID)
	if err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.WorkspaceDir, "notes.md"), []byte("workspace-note"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.WorkspaceDir, "history.md"), []byte("history-note"), 0o644); err != nil {
		t.Fatalf("write history.md: %v", err)
	}

	archiveDate := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if err := threadctx.SyncDailyArchive(paths, archiveDate); err != nil {
		t.Fatalf("SyncDailyArchive: %v", err)
	}
	if _, err := threadctx.SyncContextFile(ctx, store, dataDir, threadID); err != nil {
		t.Fatalf("SyncContextFile: %v", err)
	}

	scopeCfg, err := buildThreadWorkspaceConfig(ctx, store, dataDir, threadID)
	if err != nil {
		t.Fatalf("buildThreadWorkspaceConfig: %v", err)
	}

	handler := acphandler.NewACPHandler(paths.WorkspaceDir, "", nil)
	handler.SetThreadWorkspace(scopeCfg)
	handler.SetSuppressEvents(true)

	launchCfg := build(paths.WorkspaceDir)
	client, err := acpclient.New(launchCfg, handler)
	if err != nil {
		t.Fatalf("new acp client: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Close(closeCtx)
	}()

	runCtx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	if err := client.Initialize(runCtx, acpclient.ClientCapabilities{
		FSRead:   true,
		FSWrite:  true,
		Terminal: true,
	}); err != nil {
		t.Fatalf("initialize client: %v", err)
	}
	sessionID, err := client.NewSession(runCtx, acpproto.NewSessionRequest{Cwd: paths.WorkspaceDir})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	promptAndVerify(t, runCtx, client, sessionID,
		strings.Join([]string{
			"Use file tools, not just a text reply.",
			"Read ./.context.json.",
			"Read ./notes.md from the current workspace.",
			"Write exactly this content to ./workspace-step1.txt : WORKSPACE=workspace-note",
			"Then read back ./workspace-step1.txt to confirm it was written.",
			"Final reply must be exactly: STEP1_OK",
		}, "\n"),
		"STEP1_OK",
		filepath.Join(paths.WorkspaceDir, "workspace-step1.txt"),
		"WORKSPACE=workspace-note",
	)

	promptAndVerify(t, runCtx, client, sessionID,
		strings.Join([]string{
			"Use file tools, not just a text reply.",
			fmt.Sprintf("Read ../archive/%s/.manifest.json.", archiveDate.Format("2006-01-02")),
			"Verify it mentions notes.md and history.md.",
			"Write exactly this content to ./archive-step2.txt : ARCHIVE=notes.md,history.md",
			"Then read back ./archive-step2.txt to confirm it was written.",
			"Final reply must be exactly: STEP2_OK",
		}, "\n"),
		"STEP2_OK",
		filepath.Join(paths.WorkspaceDir, "archive-step2.txt"),
		"ARCHIVE=notes.md,history.md",
	)

	promptAndVerify(t, runCtx, client, sessionID,
		strings.Join([]string{
			"Use file tools, not just a text reply.",
			"Read ../mounts/project-alpha/README.md.",
			"Write exactly this content to ./mount-step3.txt : MOUNT=Project Alpha Readme",
			"Then read back ./mount-step3.txt to confirm it was written.",
			"Final reply must be exactly: STEP3_OK",
		}, "\n"),
		"STEP3_OK",
		filepath.Join(paths.WorkspaceDir, "mount-step3.txt"),
		"MOUNT=Project Alpha Readme",
	)

	promptAndVerify(t, runCtx, client, sessionID,
		strings.Join([]string{
			"Use the terminal tool.",
			"Use cwd exactly as: ../mounts/project-alpha",
			"In ../mounts/project-alpha run exactly this terminal command: go test ./...",
			"Do not run any other terminal command.",
			"After it succeeds, write exactly this content to ./terminal-step4.txt : TERMINAL=go test ok",
			"Then read back ./terminal-step4.txt to confirm it was written.",
			"Final reply must be exactly: STEP4_OK",
		}, "\n"),
		"STEP4_OK",
		filepath.Join(paths.WorkspaceDir, "terminal-step4.txt"),
		"TERMINAL=go test ok",
	)
}

func buildThreadWorkspaceConfig(ctx context.Context, store *sqlite.Store, dataDir string, threadID int64) (acphandler.ThreadWorkspaceConfig, error) {
	paths, err := threadctx.EnsureLayout(dataDir, threadID)
	if err != nil {
		return acphandler.ThreadWorkspaceConfig{}, err
	}
	cfg := acphandler.ThreadWorkspaceConfig{
		ThreadID:     threadID,
		WorkspaceDir: paths.WorkspaceDir,
		ArchiveDir:   paths.ArchiveDir,
	}
	refs, err := store.ListThreadContextRefs(ctx, threadID)
	if err != nil {
		return acphandler.ThreadWorkspaceConfig{}, err
	}
	for _, ref := range refs {
		mount, err := threadctx.ResolveMount(ctx, store, ref)
		if err != nil || mount == nil {
			continue
		}
		cfg.Mounts = append(cfg.Mounts, acphandler.ThreadMount{
			Alias:         mount.Slug,
			TargetPath:    mount.TargetPath,
			Access:        string(mount.Access),
			CheckCommands: append([]string(nil), mount.CheckCommands...),
		})
	}
	return cfg, nil
}

func promptAndVerify(
	t *testing.T,
	ctx context.Context,
	client *acpclient.Client,
	sessionID acpproto.SessionId,
	prompt string,
	replyToken string,
	filePath string,
	wantContent string,
) {
	t.Helper()

	result, err := client.Prompt(ctx, acpproto.PromptRequest{
		SessionId: sessionID,
		Prompt: []acpproto.ContentBlock{
			{Text: &acpproto.ContentBlockText{Text: prompt}},
		},
	})
	if err != nil {
		t.Fatalf("prompt client failed for %s: %v", replyToken, err)
	}
	if result == nil || !strings.Contains(result.Text, replyToken) {
		t.Fatalf("unexpected prompt result for %s: %+v", replyToken, result)
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v; result=%+v", filepath.Base(filePath), err, result)
	}
	if strings.TrimSpace(string(raw)) != wantContent {
		t.Fatalf("unexpected content in %s: %q", filepath.Base(filePath), string(raw))
	}
}

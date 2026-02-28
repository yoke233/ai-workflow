package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

type noopExecutor struct{}

func (noopExecutor) CreatePipeline(projectID, name, description, template string) (*core.Pipeline, error) {
	return &core.Pipeline{}, nil
}

func (noopExecutor) Run(ctx context.Context, pipelineID string) error {
	return nil
}

type noopStore struct{}

func (noopStore) ListProjects(filter core.ProjectFilter) ([]core.Project, error) {
	return nil, nil
}

func (noopStore) GetProject(id string) (*core.Project, error) {
	return nil, nil
}

func (noopStore) CreateProject(p *core.Project) error {
	return nil
}

func (noopStore) UpdateProject(p *core.Project) error {
	return nil
}

func (noopStore) DeleteProject(id string) error {
	return nil
}

func (noopStore) ListPipelines(projectID string, filter core.PipelineFilter) ([]core.Pipeline, error) {
	return nil, nil
}

func (noopStore) GetPipeline(id string) (*core.Pipeline, error) {
	return nil, nil
}

func (noopStore) SavePipeline(p *core.Pipeline) error {
	return nil
}

func (noopStore) GetActivePipelines() ([]core.Pipeline, error) {
	return nil, nil
}

func (noopStore) SaveCheckpoint(cp *core.Checkpoint) error {
	return nil
}

func (noopStore) GetCheckpoints(pipelineID string) ([]core.Checkpoint, error) {
	return nil, nil
}

func (noopStore) GetLastSuccessCheckpoint(pipelineID string) (*core.Checkpoint, error) {
	return nil, nil
}

func (noopStore) AppendLog(entry core.LogEntry) error {
	return nil
}

func (noopStore) GetLogs(pipelineID string, stage string, limit int, offset int) ([]core.LogEntry, int, error) {
	return nil, 0, nil
}

func (noopStore) RecordAction(action core.HumanAction) error {
	return nil
}

func (noopStore) GetActions(pipelineID string) ([]core.HumanAction, error) {
	return nil, nil
}

func (noopStore) Close() error {
	return nil
}

type createSpyStore struct {
	noopStore
	created []core.Project
}

func (s *createSpyStore) CreateProject(p *core.Project) error {
	s.created = append(s.created, *p)
	return nil
}

func TestSplitArgsQuoted(t *testing.T) {
	args, err := splitArgs(`pipeline create demo auth "实现 登录 与 注册" quick`)
	if err != nil {
		t.Fatalf("split args failed: %v", err)
	}

	want := []string{"pipeline", "create", "demo", "auth", "实现 登录 与 注册", "quick"}
	if len(args) != len(want) {
		t.Fatalf("unexpected args length: got=%d want=%d (%v)", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got=%q want=%q", i, args[i], want[i])
		}
	}
}

func TestSplitArgsUnclosedQuote(t *testing.T) {
	_, err := splitArgs(`pipeline create demo auth "bad`)
	if err == nil {
		t.Fatal("expected unclosed quote error, got nil")
	}
}

func TestRunCommandHelp(t *testing.T) {
	out, err := runCommand(context.Background(), noopStore{}, noopExecutor{}, "help")
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	if !strings.Contains(out, "/pipeline start <pipeline-id>") {
		t.Fatalf("help output missing pipeline start command: %s", out)
	}
}

func TestResolveChatInputSingleProject(t *testing.T) {
	msg, proj, err := resolveChatInput("请整理需求", []core.Project{
		{ID: "demo", RepoPath: "D:/repo/demo"},
	}, "D:/repo/any")
	if err != nil {
		t.Fatalf("resolve chat input failed: %v", err)
	}
	if msg != "请整理需求" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if proj.ID != "demo" {
		t.Fatalf("unexpected project: %s", proj.ID)
	}
}

func TestResolveChatInputMultipleProjectsNeedPrefix(t *testing.T) {
	_, _, err := resolveChatInput("请整理需求", []core.Project{
		{ID: "a", RepoPath: "D:/repo/a"},
		{ID: "b", RepoPath: "D:/repo/b"},
	}, "D:/repo/unknown")
	if err == nil {
		t.Fatal("expected error when multiple projects and no @prefix")
	}
}

func TestResolveChatInputWithPrefix(t *testing.T) {
	msg, proj, err := resolveChatInput("@b 请整理需求", []core.Project{
		{ID: "a", RepoPath: "D:/repo/a"},
		{ID: "b", RepoPath: "D:/repo/b"},
	}, "D:/repo/a")
	if err != nil {
		t.Fatalf("resolve prefixed chat input failed: %v", err)
	}
	if msg != "请整理需求" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if proj.ID != "b" {
		t.Fatalf("unexpected project: %s", proj.ID)
	}
}

func TestResolveChatInputAutoInferByDir(t *testing.T) {
	msg, proj, err := resolveChatInput("讨论需求", []core.Project{
		{ID: "a", RepoPath: "D:/repo/a"},
		{ID: "b", RepoPath: "D:/repo/b"},
	}, "D:/repo/b/service/api")
	if err != nil {
		t.Fatalf("resolve auto infer failed: %v", err)
	}
	if msg != "讨论需求" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if proj.ID != "b" {
		t.Fatalf("expected inferred project b, got %s", proj.ID)
	}
}

func TestResolveChatInputUnknownPrefixFallbackToDir(t *testing.T) {
	msg, proj, err := resolveChatInput("@demo 讨论需求", []core.Project{
		{ID: "a", RepoPath: "D:/repo/a"},
		{ID: "b", RepoPath: "D:/repo/b"},
	}, "D:/repo/a")
	if err != nil {
		t.Fatalf("resolve fallback failed: %v", err)
	}
	if msg != "讨论需求" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if proj.ID != "a" {
		t.Fatalf("expected inferred project a, got %s", proj.ID)
	}
}

func TestEnsureProjectForWorkDirCreatesDefaultProject(t *testing.T) {
	store := &createSpyStore{}
	proj, created, err := ensureProjectForWorkDir(store, []core.Project{
		{ID: "demo", RepoPath: "D:/repo/demo"},
	}, "D:/project/ai-workflow")
	if err != nil {
		t.Fatalf("ensure project failed: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if proj.ID != "ai-workflow" {
		t.Fatalf("expected id ai-workflow, got %s", proj.ID)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected one create call, got %d", len(store.created))
	}
}

func TestEnsureProjectForWorkDirCreateWithSuffixWhenIDExists(t *testing.T) {
	store := &createSpyStore{}
	proj, created, err := ensureProjectForWorkDir(store, []core.Project{
		{ID: "ai-workflow", RepoPath: "D:/other/path"},
	}, "D:/project/ai-workflow")
	if err != nil {
		t.Fatalf("ensure project failed: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if proj.ID != "ai-workflow-2" {
		t.Fatalf("expected id ai-workflow-2, got %s", proj.ID)
	}
}

func TestCanAttemptAutoCreateProject(t *testing.T) {
	if !canAttemptAutoCreateProject("讨论需求") {
		t.Fatal("expected plain message to allow auto-create")
	}
	if canAttemptAutoCreateProject("@demo") {
		t.Fatal("expected malformed @prefix to block auto-create")
	}
	if !canAttemptAutoCreateProject("@demo 讨论需求") {
		t.Fatal("expected valid @prefix to allow auto-create")
	}
}

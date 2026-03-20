package requirementapp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	threadapp "github.com/yoke233/zhanggui/internal/application/threadapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func newRequirementStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(filepath.Join(t.TempDir(), "requirement.db"))
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestServiceAnalyzeHeuristicUsesProjectMetadata(t *testing.T) {
	store := newRequirementStore(t)
	ctx := context.Background()
	projectID, err := store.CreateProject(ctx, &core.Project{
		Name:        "backend-api",
		Kind:        core.ProjectDev,
		Description: "认证服务",
		Metadata: map[string]string{
			core.ProjectMetaScope:      "用户认证、登录、两步验证",
			core.ProjectMetaKeywords:   "auth, login, otp",
			core.ProjectMetaAgentHints: "backend-dev, arch-reviewer",
		},
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "backend-dev", Role: core.RoleWorker, Capabilities: []string{"backend", "auth"}},
		{ID: "arch-reviewer", Role: core.RoleLead, Capabilities: []string{"architecture", "review"}},
	})

	svc := New(Config{Store: store, Registry: registry})
	result, err := svc.Analyze(ctx, AnalyzeInput{
		Description: "给登录系统增加两步验证和 OTP 流程",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Analysis.MatchedProjects) == 0 || result.Analysis.MatchedProjects[0].ProjectID != projectID {
		t.Fatalf("matched_projects = %+v", result.Analysis.MatchedProjects)
	}
	if result.SuggestedThread.MeetingMode == "" {
		t.Fatal("expected suggested meeting mode")
	}
	if len(result.SuggestedThread.Agents) == 0 {
		t.Fatal("expected suggested agents from metadata and capabilities")
	}
}

func TestServiceCreateThreadCreatesRefsAndNormalizesConfig(t *testing.T) {
	store := newRequirementStore(t)
	ctx := context.Background()
	projectID, err := store.CreateProject(ctx, &core.Project{Name: "frontend-web", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := store.CreateResourceSpace(ctx, &core.ResourceSpace{
		ProjectID: projectID,
		Kind:      "local_fs",
		RootURI:   "D:/workspace/frontend-web",
	}); err != nil {
		t.Fatalf("CreateResourceSpace: %v", err)
	}

	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "frontend-dev", Role: core.RoleWorker, Capabilities: []string{"frontend"}},
	})

	threadSvc := threadapp.New(threadapp.Config{Store: store})
	svc := New(Config{
		Store:         store,
		Registry:      registry,
		ThreadService: threadSvc,
	})

	result, err := svc.CreateThread(ctx, CreateThreadInput{
		Description: "重做登录页交互",
		OwnerID:     "user-1",
		ThreadConfig: SuggestedThread{
			ContextRefs:      []SuggestedContextRef{{ProjectID: projectID}},
			Agents:           []string{"frontend-dev", "frontend-dev"},
			MeetingMode:      "group_chat",
			MeetingMaxRounds: 20,
		},
	})
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if result.Thread == nil || result.Thread.ID <= 0 {
		t.Fatalf("thread = %+v", result.Thread)
	}
	if got := result.Thread.Metadata["meeting_mode"]; got != "group_chat" {
		t.Fatalf("meeting_mode = %v", got)
	}
	if got := result.Thread.Metadata["meeting_max_rounds"]; got != 12 {
		t.Fatalf("meeting_max_rounds = %v, want 12", got)
	}
	if len(result.ContextRefs) != 1 || result.ContextRefs[0].ProjectID != projectID {
		t.Fatalf("context_refs = %+v", result.ContextRefs)
	}
	if len(result.AgentIDs) != 1 || result.AgentIDs[0] != "frontend-dev" {
		t.Fatalf("agents = %+v", result.AgentIDs)
	}
}

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	planningapp "github.com/yoke233/zhanggui/internal/application/planning"
	requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type stubRequirementCompleter struct {
	raw json.RawMessage
	err error
}

func (s stubRequirementCompleter) Complete(context.Context, string, []planningapp.ToolDef) (json.RawMessage, error) {
	return s.raw, s.err
}

func TestAPI_AnalyzeRequirement(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()
	projectID, err := h.store.CreateProject(ctx, &core.Project{Name: "backend-api", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "backend-dev", Role: core.RoleWorker, Capabilities: []string{"backend"}},
	})
	h.registry = registry
	h.requirementLLM = stubRequirementCompleter{raw: mustMarshalJSONRaw(t, map[string]any{
		"summary": "新增登录风控能力",
		"type":    "single_project",
		"matched_projects": []map[string]any{
			{"project_id": projectID, "reason": "涉及后端鉴权", "relevance": "high"},
		},
		"suggested_agents": []map[string]any{
			{"profile_id": "backend-dev", "reason": "处理后端接口与鉴权"},
		},
		"complexity":             "medium",
		"suggested_meeting_mode": "concurrent",
		"risks":                  []string{"需要兼容旧登录流程"},
		"suggested_thread": map[string]any{
			"title":              "讨论：登录风控",
			"context_refs":       []map[string]any{{"project_id": projectID, "access": "read"}},
			"agents":             []string{"backend-dev"},
			"meeting_mode":       "direct",
			"meeting_max_rounds": 4,
		},
	})}

	resp, err := post(ts, "/requirements/analyze", map[string]any{
		"description": "想给登录系统增加风控与 OTP 校验",
	})
	if err != nil {
		t.Fatalf("analyze requirement: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got requirementapp.AnalyzeResult
	if err := decodeJSON(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Analysis.Summary != "新增登录风控能力" {
		t.Fatalf("summary = %q", got.Analysis.Summary)
	}
	if len(got.Analysis.MatchedProjects) != 1 || got.Analysis.MatchedProjects[0].ProjectID != projectID {
		t.Fatalf("matched_projects = %+v", got.Analysis.MatchedProjects)
	}
	if len(got.SuggestedThread.Agents) != 1 || got.SuggestedThread.Agents[0] != "backend-dev" {
		t.Fatalf("agents = %+v", got.SuggestedThread.Agents)
	}
}

func TestAPI_CreateThreadFromRequirement(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()
	projectID, err := h.store.CreateProject(ctx, &core.Project{Name: "frontend-web", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := h.store.CreateResourceSpace(ctx, &core.ResourceSpace{
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
	h.registry = registry
	threadPool := &stubThreadAgentRuntime{}
	h.threadPool = threadPool

	resp, err := post(ts, "/requirements/create-thread", map[string]any{
		"description": "重做登录页 MFA 开关和 OTP 提示",
		"context":     "需要兼容旧 UI",
		"owner_id":    "alice",
		"analysis": map[string]any{
			"summary":                "登录页 MFA 改造",
			"type":                   "single_project",
			"suggested_meeting_mode": "group_chat",
			"suggested_agents":       []map[string]any{{"profile_id": "frontend-dev", "reason": "负责前端实现"}},
			"matched_projects":       []map[string]any{{"project_id": projectID, "project_name": "frontend-web"}},
			"risks":                  []string{"注意旧版入口兼容"},
		},
		"thread_config": map[string]any{
			"title":              "讨论：登录页 MFA",
			"context_refs":       []map[string]any{{"project_id": projectID, "access": "read"}},
			"agents":             []string{"frontend-dev"},
			"meeting_mode":       "group_chat",
			"meeting_max_rounds": 5,
		},
	})
	if err != nil {
		t.Fatalf("create requirement thread: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("status = %d, want 201, body=%s", resp.StatusCode, string(body))
	}
	var got struct {
		Thread      core.Thread             `json:"thread"`
		ContextRefs []core.ThreadContextRef `json:"context_refs"`
		Agents      []string                `json:"agents"`
		Message     core.ThreadMessage      `json:"message"`
	}
	if err := decodeJSON(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Thread.Title != "讨论：登录页 MFA" {
		t.Fatalf("thread title = %q", got.Thread.Title)
	}
	if len(got.ContextRefs) != 1 || got.ContextRefs[0].ProjectID != projectID {
		t.Fatalf("context_refs = %+v", got.ContextRefs)
	}
	if len(got.Agents) != 1 || got.Agents[0] != "frontend-dev" {
		t.Fatalf("agents = %+v", got.Agents)
	}
	if got.Message.ThreadID != got.Thread.ID || got.Message.Role != "human" {
		t.Fatalf("message = %+v", got.Message)
	}

	invites := threadPool.snapshotInviteCalls()
	if len(invites) != 1 || invites[0].profileID != "frontend-dev" {
		t.Fatalf("invite calls = %+v", invites)
	}
	sends := threadPool.snapshotSendCalls()
	if len(sends) != 1 || sends[0].profileID != "frontend-dev" {
		t.Fatalf("send calls = %+v", sends)
	}
	if got := got.Thread.Metadata["meeting_mode"]; got != "group_chat" {
		t.Fatalf("meeting_mode metadata = %v", got)
	}
}

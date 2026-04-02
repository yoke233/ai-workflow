package api

import (
	"context"
	"net/http"
	"testing"

	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	"github.com/yoke233/zhanggui/internal/application/ceoapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func TestCEOSubmitReturnsDirectExecutionPayload(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()

	projectID, err := h.store.CreateProject(ctx, &core.Project{Name: "backend-api", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	h.requirementLLM = stubRequirementCompleter{raw: mustMarshalJSONRaw(t, map[string]any{
		"summary": "Backend OTP",
		"type":    "single_project",
		"matched_projects": []map[string]any{
			{"project_id": projectID, "reason": "backend work", "relevance": "high"},
		},
		"complexity":             "medium",
		"suggested_meeting_mode": "direct",
		"suggested_thread": map[string]any{
			"title":              "Discuss Backend OTP",
			"context_refs":       []map[string]any{{"project_id": projectID, "access": "read"}},
			"meeting_mode":       "direct",
			"meeting_max_rounds": 4,
		},
	})}

	resp, err := post(ts, "/ceo/submit", map[string]any{
		"description": "Add backend OTP support",
	})
	if err != nil {
		t.Fatalf("submit ceo requirement: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	var got ceoSubmitResponse
	if err := decodeJSON(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Mode != ceoapp.ModeDirectExecution {
		t.Fatalf("Mode = %q, want %q", got.Mode, ceoapp.ModeDirectExecution)
	}
	if got.WorkItemID <= 0 {
		t.Fatalf("WorkItemID = %d, want > 0", got.WorkItemID)
	}
	if got.ActionCount == 0 {
		t.Fatalf("ActionCount = %d, want > 0", got.ActionCount)
	}
	if got.NextStep != "run_work_item" {
		t.Fatalf("NextStep = %q, want run_work_item", got.NextStep)
	}
}

func TestCEOSubmitReturnsDiscussionThreadPayload(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()

	backendID, err := h.store.CreateProject(ctx, &core.Project{Name: "backend-api", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject(backend): %v", err)
	}
	frontendID, err := h.store.CreateProject(ctx, &core.Project{Name: "frontend-web", Kind: core.ProjectDev})
	if err != nil {
		t.Fatalf("CreateProject(frontend): %v", err)
	}
	for _, item := range []struct {
		projectID int64
		rootURI   string
	}{
		{projectID: backendID, rootURI: "D:/workspace/backend-api"},
		{projectID: frontendID, rootURI: "D:/workspace/frontend-web"},
	} {
		if _, err := h.store.CreateResourceSpace(ctx, &core.ResourceSpace{
			ProjectID: item.projectID,
			Kind:      "local_fs",
			RootURI:   item.rootURI,
		}); err != nil {
			t.Fatalf("CreateResourceSpace(%d): %v", item.projectID, err)
		}
	}

	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "lead", Role: core.RoleLead, Capabilities: []string{"planning"}},
	})
	h.registry = registry
	h.requirementLLM = stubRequirementCompleter{raw: mustMarshalJSONRaw(t, map[string]any{
		"summary": "OTP rollout",
		"type":    "cross_project",
		"matched_projects": []map[string]any{
			{"project_id": backendID, "reason": "backend work", "relevance": "high"},
			{"project_id": frontendID, "reason": "frontend work", "relevance": "high"},
		},
		"suggested_agents": []map[string]any{
			{"profile_id": "lead", "reason": "coordinate"},
		},
		"complexity":             "high",
		"suggested_meeting_mode": "group_chat",
		"suggested_thread": map[string]any{
			"title":              "Discuss OTP rollout",
			"context_refs":       []map[string]any{{"project_id": backendID, "access": "read"}, {"project_id": frontendID, "access": "read"}},
			"agents":             []string{"lead"},
			"meeting_mode":       "group_chat",
			"meeting_max_rounds": 6,
		},
	})}

	resp, err := post(ts, "/ceo/submit", map[string]any{
		"description": "Roll out OTP across backend and frontend",
		"owner_id":    "alice",
	})
	if err != nil {
		t.Fatalf("submit ceo requirement: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	var got ceoSubmitResponse
	if err := decodeJSON(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Mode != ceoapp.ModeDiscussion {
		t.Fatalf("Mode = %q, want %q", got.Mode, ceoapp.ModeDiscussion)
	}
	if got.ThreadID <= 0 {
		t.Fatalf("ThreadID = %d, want > 0", got.ThreadID)
	}
	if got.Message == nil || got.Message.ID <= 0 {
		t.Fatalf("Message = %+v, want created thread message", got.Message)
	}
	if len(got.ContextRefs) != 2 {
		t.Fatalf("ContextRefs = %d, want 2", len(got.ContextRefs))
	}
}

func TestCEOSubmitRejectsEmptyDescription(t *testing.T) {
	_, ts := setupAPI(t)

	resp, err := post(ts, "/ceo/submit", map[string]any{})
	if err != nil {
		t.Fatalf("submit ceo requirement: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

package api

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	initiativeapp "github.com/yoke233/zhanggui/internal/application/initiativeapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func TestCEOFlow_DiscussionPathRequiresInitiativeProposeAndApprove(t *testing.T) {
	var store core.Store
	var backendRuns int32
	var frontendRuns int32

	executor := func(ctx context.Context, step *core.Action, exec *core.Run) error {
		workItem, err := store.GetWorkItem(ctx, step.WorkItemID)
		if err != nil {
			return err
		}
		switch {
		case strings.Contains(workItem.Title, "后端"):
			atomic.AddInt32(&backendRuns, 1)
		case strings.Contains(workItem.Title, "前端"):
			atomic.AddInt32(&frontendRuns, 1)
		}
		exec.Output = map[string]any{"result": "ok"}
		return nil
	}

	env := setupIntegration(t, executor)
	store = env.store
	h := env.handler
	ts := env.server
	ctx := context.Background()

	backendID, err := h.store.CreateProject(ctx, &core.Project{Name: "backend-api", Kind: core.ProjectDev, Description: "认证后端"})
	if err != nil {
		t.Fatalf("CreateProject(backend): %v", err)
	}
	frontendID, err := h.store.CreateProject(ctx, &core.Project{Name: "frontend-web", Kind: core.ProjectDev, Description: "登录前端"})
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

	h.requirementLLM = stubRequirementCompleter{raw: mustMarshalJSONRaw(t, map[string]any{
		"summary": "OTP rollout",
		"type":    "cross_project",
		"matched_projects": []map[string]any{
			{"project_id": backendID, "reason": "需要后端支持", "relevance": "high"},
			{"project_id": frontendID, "reason": "需要前端支持", "relevance": "high"},
		},
		"complexity":             "high",
		"suggested_meeting_mode": "group_chat",
		"suggested_thread": map[string]any{
			"title":              "讨论：OTP rollout",
			"context_refs":       []map[string]any{{"project_id": backendID, "access": "read"}, {"project_id": frontendID, "access": "read"}},
			"meeting_mode":       "group_chat",
			"meeting_max_rounds": 6,
		},
	})}

	resp, err := postJSON(ts, "/ceo/submit", map[string]any{
		"description": "给登录系统增加 OTP，两端一起改。",
		"owner_id":    "alice",
	})
	if err != nil {
		t.Fatalf("ceo submit: %v", err)
	}
	requireStatus(t, resp, 202)
	created := decode[ceoSubmitResponse](t, resp)
	if created.Mode != "discussion_thread" || created.ThreadID <= 0 {
		t.Fatalf("created = %+v", created)
	}

	resp, err = postJSON(ts, fmt.Sprintf("/threads/%d/proposals", created.ThreadID), map[string]any{
		"title":       "OTP rollout proposal",
		"summary":     "拆成后端和前端两个 work item",
		"content":     "先后端后前端",
		"proposed_by": "alice",
		"work_item_drafts": []map[string]any{
			{"temp_id": "backend", "project_id": backendID, "title": "实现 OTP 后端支持", "priority": "high"},
			{"temp_id": "frontend", "project_id": frontendID, "title": "接入 OTP 前端交互", "depends_on": []string{"backend"}, "priority": "high"},
		},
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	requireStatus(t, resp, 201)
	proposal := decode[core.ThreadProposal](t, resp)

	resp, err = postJSON(ts, fmt.Sprintf("/proposals/%d/submit", proposal.ID), map[string]any{})
	if err != nil {
		t.Fatalf("submit proposal: %v", err)
	}
	requireStatus(t, resp, 200)

	resp, err = postJSON(ts, fmt.Sprintf("/proposals/%d/approve", proposal.ID), map[string]any{
		"reviewed_by": "alice",
	})
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	requireStatus(t, resp, 200)
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalMerged || proposal.InitiativeID == nil {
		t.Fatalf("proposal = %+v", proposal)
	}

	resp, err = getJSON(ts, fmt.Sprintf("/initiatives/%d", *proposal.InitiativeID))
	if err != nil {
		t.Fatalf("get initiative detail: %v", err)
	}
	requireStatus(t, resp, 200)
	detail := decode[initiativeapp.InitiativeDetail](t, resp)
	if detail.Initiative.Status != core.InitiativeDraft {
		t.Fatalf("initiative status = %s, want draft", detail.Initiative.Status)
	}
	if len(detail.WorkItems) != 2 {
		t.Fatalf("work items = %d, want 2", len(detail.WorkItems))
	}

	workItemsByTitle := make(map[string]*core.WorkItem, len(detail.WorkItems))
	for _, workItem := range detail.WorkItems {
		if workItem != nil {
			workItemsByTitle[workItem.Title] = workItem
		}
	}
	backend := workItemsByTitle["实现 OTP 后端支持"]
	frontend := workItemsByTitle["接入 OTP 前端交互"]
	if backend == nil || frontend == nil {
		t.Fatalf("unexpected work items: %+v", detail.WorkItems)
	}

	for _, req := range []struct {
		workItemID int64
		name       string
	}{
		{workItemID: backend.ID, name: "backend-impl"},
		{workItemID: frontend.ID, name: "frontend-impl"},
	} {
		resp, err := postJSON(ts, fmt.Sprintf("/work-items/%d/actions", req.workItemID), map[string]any{
			"name":       req.name,
			"type":       "exec",
			"agent_role": "worker",
		})
		if err != nil {
			t.Fatalf("create action for %d: %v", req.workItemID, err)
		}
		requireStatus(t, resp, 201)
	}

	resp, err = postJSON(ts, fmt.Sprintf("/initiatives/%d/propose", detail.Initiative.ID), map[string]any{})
	if err != nil {
		t.Fatalf("propose initiative: %v", err)
	}
	requireStatus(t, resp, 200)
	initiative := decode[core.Initiative](t, resp)
	if initiative.Status != core.InitiativeProposed {
		t.Fatalf("initiative status = %s, want proposed", initiative.Status)
	}

	resp, err = postJSON(ts, fmt.Sprintf("/initiatives/%d/approve", detail.Initiative.ID), map[string]any{
		"approved_by": "alice",
	})
	if err != nil {
		t.Fatalf("approve initiative: %v", err)
	}
	requireStatus(t, resp, 200)
	initiative = decode[core.Initiative](t, resp)
	if initiative.Status != core.InitiativeExecuting {
		t.Fatalf("initiative status = %s, want executing", initiative.Status)
	}

	pollWorkItemStatus(t, ts, backend.ID, core.WorkItemDone, 5*time.Second)
	pollWorkItemStatus(t, ts, frontend.ID, core.WorkItemDone, 5*time.Second)

	if got := atomic.LoadInt32(&backendRuns); got != 1 {
		t.Fatalf("backend run count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&frontendRuns); got != 1 {
		t.Fatalf("frontend run count = %d, want 1", got)
	}
}

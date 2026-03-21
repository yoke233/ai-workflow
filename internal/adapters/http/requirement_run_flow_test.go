package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	initiativeapp "github.com/yoke233/zhanggui/internal/application/initiativeapp"
	requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func TestIntegration_RequirementToWorkItemRunFlow(t *testing.T) {
	var store core.Store
	var backendRunCount int32
	var frontendRunCount int32
	var frontendGateRuns int32
	var qaRunCount int32

	executor := func(ctx context.Context, step *core.Action, exec *core.Run) error {
		workItem, err := store.GetWorkItem(ctx, step.WorkItemID)
		if err != nil {
			return err
		}
		switch {
		case strings.Contains(workItem.Title, "后端"):
			time.Sleep(80 * time.Millisecond)
			atomic.AddInt32(&backendRunCount, 1)
			exec.Output = map[string]any{"result": "backend-ready"}
			return nil
		case strings.Contains(workItem.Title, "登录交互") && step.Type == core.ActionGate:
			n := atomic.AddInt32(&frontendGateRuns, 1)
			verdict := "reject"
			reason := "OTP 错误提示和重试状态还不完整"
			if n > 1 {
				verdict = "pass"
				reason = "OTP 交互与异常态都已补齐"
			}
			exec.ResultMarkdown = reason
			exec.ResultMetadata = map[string]any{"verdict": verdict, "reason": reason}
			exec.Output = map[string]any{"verdict": verdict, "reason": reason}
			return nil
		case strings.Contains(workItem.Title, "登录交互"):
			atomic.AddInt32(&frontendRunCount, 1)
			exec.Output = map[string]any{"result": "frontend-updated"}
			return nil
		case strings.Contains(workItem.Title, "验收"):
			atomic.AddInt32(&qaRunCount, 1)
			exec.Output = map[string]any{"result": "qa-verified"}
			return nil
		default:
			exec.Output = map[string]any{"result": "ok"}
			return nil
		}
	}

	env := setupIntegration(t, executor)
	store = env.store
	h := env.handler
	ts := env.server
	ctx := context.Background()

	projectBackend, err := h.store.CreateProject(ctx, &core.Project{
		Name:        "backend-api",
		Kind:        core.ProjectDev,
		Description: "认证后端",
		Metadata: map[string]string{
			core.ProjectMetaScope:      "负责登录、鉴权与 OTP 校验",
			core.ProjectMetaKeywords:   "auth, otp, login",
			core.ProjectMetaAgentHints: "backend-dev, arch-reviewer",
		},
	})
	if err != nil {
		t.Fatalf("CreateProject(backend): %v", err)
	}
	projectFrontend, err := h.store.CreateProject(ctx, &core.Project{
		Name:        "frontend-web",
		Kind:        core.ProjectDev,
		Description: "登录前端",
		Metadata: map[string]string{
			core.ProjectMetaScope:      "负责登录页面与交互",
			core.ProjectMetaKeywords:   "frontend, login, web",
			core.ProjectMetaAgentHints: "frontend-dev",
		},
	})
	if err != nil {
		t.Fatalf("CreateProject(frontend): %v", err)
	}
	projectQA, err := h.store.CreateProject(ctx, &core.Project{
		Name:        "qa-verification",
		Kind:        core.ProjectGeneral,
		Description: "验收与联调",
		Metadata: map[string]string{
			core.ProjectMetaScope:      "负责跨端验收与冒烟",
			core.ProjectMetaKeywords:   "qa, verification, smoke",
			core.ProjectMetaAgentHints: "qa-reviewer",
		},
	})
	if err != nil {
		t.Fatalf("CreateProject(qa): %v", err)
	}

	for _, item := range []struct {
		projectID int64
		rootURI   string
	}{
		{projectID: projectBackend, rootURI: "D:/workspace/backend-api"},
		{projectID: projectFrontend, rootURI: "D:/workspace/frontend-web"},
		{projectID: projectQA, rootURI: "D:/workspace/qa-verification"},
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
		{ID: "arch-reviewer", Role: core.RoleLead, Capabilities: []string{"architecture", "review"}},
		{ID: "backend-dev", Role: core.RoleWorker, Capabilities: []string{"backend", "auth"}},
		{ID: "frontend-dev", Role: core.RoleWorker, Capabilities: []string{"frontend", "ui"}},
		{ID: "qa-reviewer", Role: core.RoleGate, Capabilities: []string{"qa", "review"}},
	})
	h.registry = registry
	threadPool := &stubThreadAgentRuntime{
		promptReplies: map[string]string{
			"arch-reviewer": "先确定 OTP 边界，再把后端、前端、验收拆开。",
			"backend-dev":   "后端先补 OTP 校验接口与兼容策略。",
			"frontend-dev":  "[FINAL] 前端跟进登录交互，验收另起一个 work item 兜底。",
		},
	}
	h.threadPool = threadPool
	h.requirementLLM = stubRequirementCompleter{raw: mustMarshalJSONRaw(t, map[string]any{
		"summary": "给登录系统增加 OTP 两步验证",
		"type":    "cross_project",
		"matched_projects": []map[string]any{
			{"project_id": projectBackend, "reason": "需要后端 OTP 校验与接口变更", "relevance": "high"},
			{"project_id": projectFrontend, "reason": "需要登录页补充 OTP 输入与状态反馈", "relevance": "high"},
			{"project_id": projectQA, "reason": "需要跨端冒烟与验收", "relevance": "medium"},
		},
		"suggested_agents": []map[string]any{
			{"profile_id": "arch-reviewer", "reason": "负责收敛方案"},
			{"profile_id": "backend-dev", "reason": "负责接口与鉴权逻辑"},
			{"profile_id": "frontend-dev", "reason": "负责前端交互"},
		},
		"complexity":             "high",
		"suggested_meeting_mode": "group_chat",
		"risks":                  []string{"需要兼容旧登录流程"},
		"suggested_thread": map[string]any{
			"title": "讨论：登录 OTP 两步验证",
			"context_refs": []map[string]any{
				{"project_id": projectBackend, "access": "read"},
				{"project_id": projectFrontend, "access": "read"},
				{"project_id": projectQA, "access": "read"},
			},
			"agents":             []string{"arch-reviewer", "backend-dev", "frontend-dev"},
			"meeting_mode":       "group_chat",
			"meeting_max_rounds": 6,
		},
	})}

	resp, err := postJSON(ts, "/requirements/analyze", map[string]any{
		"description": "给用户登录系统增加 OTP 两步验证，后端要校验，前端要新增输入流程，并补一轮验收。",
	})
	if err != nil {
		t.Fatalf("analyze requirement: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	analysis := decode[requirementapp.AnalyzeResult](t, resp)

	resp, err = postJSON(ts, "/requirements/create-thread", map[string]any{
		"description":   "给用户登录系统增加 OTP 两步验证，后端要校验，前端要新增输入流程，并补一轮验收。",
		"context":       "本次要求模拟 proposal 审核返工，再进入 work item 执行闭环。",
		"owner_id":      "alice",
		"analysis":      analysis.Analysis,
		"thread_config": analysis.SuggestedThread,
	})
	if err != nil {
		t.Fatalf("create requirement thread: %v", err)
	}
	requireStatus(t, resp, http.StatusCreated)
	var created struct {
		Thread  core.Thread        `json:"thread"`
		Agents  []string           `json:"agents"`
		Message core.ThreadMessage `json:"message"`
	}
	if err := decodeJSON(resp, &created); err != nil {
		t.Fatalf("decode created thread: %v", err)
	}

	waitForThreadCondition(t, 2*time.Second, func() error {
		if got := len(threadPool.snapshotPromptCalls()); got != 3 {
			return fmt.Errorf("prompt calls = %d, want 3", got)
		}
		msgs, err := h.store.ListThreadMessages(ctx, created.Thread.ID, 20, 0)
		if err != nil {
			return err
		}
		if len(msgs) < 5 {
			return fmt.Errorf("messages = %d, want at least 5", len(msgs))
		}
		return nil
	})

	resp, err = postJSON(ts, "/threads/"+itoa64(created.Thread.ID)+"/proposals", map[string]any{
		"title":             "登录 OTP 两步验证方案",
		"summary":           "先后端后前端",
		"content":           "先落后端接口，再接前端",
		"proposed_by":       "arch-reviewer",
		"source_message_id": created.Message.ID,
		"work_item_drafts": []map[string]any{
			{"temp_id": "backend", "project_id": projectBackend, "title": "实现 OTP 后端校验接口", "priority": "high"},
			{"temp_id": "frontend", "project_id": projectFrontend, "title": "接入 OTP 登录交互", "depends_on": []string{"backend"}, "priority": "high"},
		},
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusCreated)
	proposal := decode[core.ThreadProposal](t, resp)

	resp, err = postJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/submit", map[string]any{})
	if err != nil {
		t.Fatalf("submit proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalOpen {
		t.Fatalf("proposal status = %s, want open", proposal.Status)
	}

	resp, err = postJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/reject", map[string]any{
		"reviewed_by": "alice",
		"review_note": "需要把验收环节拆成独立 work item，并明确前端返工预期。",
	})
	if err != nil {
		t.Fatalf("reject proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalRejected {
		t.Fatalf("proposal status = %s, want rejected", proposal.Status)
	}

	resp, err = postJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/revise", map[string]any{
		"reviewed_by": "alice",
		"review_note": "允许修改后重新提交。",
	})
	if err != nil {
		t.Fatalf("revise proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalRevised {
		t.Fatalf("proposal status = %s, want revised", proposal.Status)
	}

	resp, err = putJSON(ts, "/proposals/"+itoa64(proposal.ID), map[string]any{
		"summary": "拆成后端、前端、验收三个 work item，保留依赖链",
		"content": "后端先交付 OTP 接口，前端完成交互与错误提示，最后单独验收。",
	})
	if err != nil {
		t.Fatalf("update proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)

	resp, err = putJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/drafts", map[string]any{
		"work_item_drafts": []map[string]any{
			{"temp_id": "backend", "project_id": projectBackend, "title": "实现 OTP 后端校验接口", "priority": "high"},
			{"temp_id": "frontend", "project_id": projectFrontend, "title": "接入 OTP 登录交互并补错误提示", "depends_on": []string{"backend"}, "priority": "high"},
			{"temp_id": "qa", "project_id": projectQA, "title": "补充 OTP 联调验收", "depends_on": []string{"frontend"}, "priority": "medium"},
		},
	})
	if err != nil {
		t.Fatalf("replace proposal drafts: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	proposal = decode[core.ThreadProposal](t, resp)
	if len(proposal.WorkItemDrafts) != 3 {
		t.Fatalf("proposal drafts = %d, want 3", len(proposal.WorkItemDrafts))
	}

	resp, err = postJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/submit", map[string]any{})
	if err != nil {
		t.Fatalf("resubmit proposal: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalOpen {
		t.Fatalf("proposal status = %s, want open after resubmit", proposal.Status)
	}

	resp, err = postJSON(ts, "/proposals/"+itoa64(proposal.ID)+"/approve", map[string]any{
		"reviewed_by": "alice",
		"review_note": "按三段 work item 关系组执行",
	})
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve proposal status = %d, body = %s", resp.StatusCode, readBody(resp))
	}
	proposal = decode[core.ThreadProposal](t, resp)
	if proposal.Status != core.ProposalMerged || proposal.InitiativeID == nil {
		t.Fatalf("proposal = %+v", proposal)
	}

	resp, err = getJSON(ts, fmt.Sprintf("/initiatives/%d", *proposal.InitiativeID))
	if err != nil {
		t.Fatalf("get initiative detail: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	detail := decode[initiativeapp.InitiativeDetail](t, resp)
	if detail.Initiative.Status != core.InitiativeDraft {
		t.Fatalf("initiative status = %s, want draft", detail.Initiative.Status)
	}
	if len(detail.WorkItems) != 3 {
		t.Fatalf("work items = %d, want 3", len(detail.WorkItems))
	}

	workItemsByTitle := make(map[string]*core.WorkItem, len(detail.WorkItems))
	for _, workItem := range detail.WorkItems {
		if workItem != nil {
			workItemsByTitle[workItem.Title] = workItem
		}
	}
	backend := workItemsByTitle["实现 OTP 后端校验接口"]
	frontend := workItemsByTitle["接入 OTP 登录交互并补错误提示"]
	qa := workItemsByTitle["补充 OTP 联调验收"]
	if backend == nil || frontend == nil || qa == nil {
		t.Fatalf("unexpected work items: %+v", detail.WorkItems)
	}
	if len(frontend.DependsOn) != 1 || frontend.DependsOn[0] != backend.ID {
		t.Fatalf("frontend depends_on = %+v, want [%d]", frontend.DependsOn, backend.ID)
	}
	if len(qa.DependsOn) != 1 || qa.DependsOn[0] != frontend.ID {
		t.Fatalf("qa depends_on = %+v, want [%d]", qa.DependsOn, frontend.ID)
	}

	for _, req := range []struct {
		workItemID int64
		body       map[string]any
	}{
		{
			workItemID: backend.ID,
			body:       map[string]any{"name": "backend-impl", "type": "exec", "agent_role": "worker"},
		},
		{
			workItemID: frontend.ID,
			body:       map[string]any{"name": "frontend-impl", "type": "exec", "agent_role": "worker"},
		},
		{
			workItemID: frontend.ID,
			body: map[string]any{
				"name":                "frontend-review",
				"type":                "gate",
				"agent_role":          "gate",
				"acceptance_criteria": []string{"OTP 错误提示完整", "重试状态明确"},
			},
		},
		{
			workItemID: qa.ID,
			body:       map[string]any{"name": "qa-verify", "type": "exec", "agent_role": "worker"},
		},
	} {
		resp, err := postJSON(ts, fmt.Sprintf("/work-items/%d/actions", req.workItemID), req.body)
		if err != nil {
			t.Fatalf("create step for work item %d: %v", req.workItemID, err)
		}
		requireStatus(t, resp, http.StatusCreated)
	}

	resp, err = postJSON(ts, fmt.Sprintf("/initiatives/%d/propose", detail.Initiative.ID), map[string]any{})
	if err != nil {
		t.Fatalf("propose initiative: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)

	resp, err = postJSON(ts, fmt.Sprintf("/initiatives/%d/approve", detail.Initiative.ID), map[string]any{
		"approved_by": "alice",
	})
	if err != nil {
		t.Fatalf("approve initiative: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	initiative := decode[core.Initiative](t, resp)
	if initiative.Status != core.InitiativeExecuting {
		t.Fatalf("initiative status = %s, want executing", initiative.Status)
	}

	resp, err = getJSON(ts, fmt.Sprintf("/work-items/%d", frontend.ID))
	if err != nil {
		t.Fatalf("get frontend work item: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	frontendState := decode[core.WorkItem](t, resp)
	frontend = &frontendState
	if frontend.Status != core.WorkItemAccepted {
		t.Fatalf("frontend status = %s, want accepted before dependency release", frontend.Status)
	}

	resp, err = getJSON(ts, fmt.Sprintf("/work-items/%d", qa.ID))
	if err != nil {
		t.Fatalf("get qa work item: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	qaState := decode[core.WorkItem](t, resp)
	qa = &qaState
	if qa.Status != core.WorkItemAccepted {
		t.Fatalf("qa status = %s, want accepted before dependency release", qa.Status)
	}

	pollWorkItemStatus(t, ts, backend.ID, core.WorkItemDone, 5*time.Second)
	pollWorkItemStatus(t, ts, frontend.ID, core.WorkItemDone, 10*time.Second)
	pollWorkItemStatus(t, ts, qa.ID, core.WorkItemDone, 5*time.Second)

	resp, err = getJSON(ts, fmt.Sprintf("/initiatives/%d", detail.Initiative.ID))
	if err != nil {
		t.Fatalf("get final initiative detail: %v", err)
	}
	requireStatus(t, resp, http.StatusOK)
	detail = decode[initiativeapp.InitiativeDetail](t, resp)
	if detail.Initiative.Status != core.InitiativeDone {
		t.Fatalf("initiative status = %s, want done", detail.Initiative.Status)
	}
	if detail.Progress.Total != 3 || detail.Progress.Done != 3 {
		t.Fatalf("initiative progress = %+v, want total=3 done=3", detail.Progress)
	}

	if got := atomic.LoadInt32(&backendRunCount); got != 1 {
		t.Fatalf("backend run count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&frontendRunCount); got != 2 {
		t.Fatalf("frontend run count = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&frontendGateRuns); got != 2 {
		t.Fatalf("frontend gate runs = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&qaRunCount); got != 1 {
		t.Fatalf("qa run count = %d, want 1", got)
	}

	msgs, err := h.store.ListThreadMessages(ctx, created.Thread.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListThreadMessages: %v", err)
	}
	seenSystemTypes := map[string]bool{}
	for _, msg := range msgs {
		if msg.Role != "system" || msg.Metadata == nil {
			continue
		}
		if typ, _ := msg.Metadata["type"].(string); typ != "" {
			seenSystemTypes[typ] = true
		}
	}
	for _, typ := range []string{"meeting_summary", "proposal_rejected", "proposal_revised", "proposal_merged"} {
		if !seenSystemTypes[typ] {
			t.Fatalf("missing system message type %q in thread messages", typ)
		}
	}
}

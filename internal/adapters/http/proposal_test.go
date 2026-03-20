package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestAPI_ProposalLifecycleMaterializesInitiative(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()

	projectA, err := h.store.CreateProject(ctx, &core.Project{Name: "project-a"})
	if err != nil {
		t.Fatalf("CreateProject(project-a): %v", err)
	}
	projectB, err := h.store.CreateProject(ctx, &core.Project{Name: "project-b"})
	if err != nil {
		t.Fatalf("CreateProject(project-b): %v", err)
	}
	threadID, err := h.store.CreateThread(ctx, &core.Thread{
		Title:   "proposal thread",
		Status:  core.ThreadActive,
		OwnerID: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	resp, err := post(ts, "/threads/"+itoa64(threadID)+"/proposals", map[string]any{
		"title":       "跨项目 rollout",
		"summary":     "先后端后前端",
		"content":     "拆成两个 work item",
		"proposed_by": "lead-1",
		"work_item_drafts": []map[string]any{
			{"temp_id": "backend", "project_id": projectA, "title": "后端改造", "priority": "high"},
			{"temp_id": "frontend", "project_id": projectB, "title": "前端接入", "depends_on": []string{"backend"}},
		},
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create proposal status = %d, want 201", resp.StatusCode)
	}
	var proposal core.ThreadProposal
	if err := decodeJSON(resp, &proposal); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}

	resp, err = post(ts, "/proposals/"+itoa64(proposal.ID)+"/submit", map[string]any{})
	if err != nil {
		t.Fatalf("submit proposal: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("submit proposal status = %d, want 200", resp.StatusCode)
	}

	resp, err = post(ts, "/proposals/"+itoa64(proposal.ID)+"/approve", map[string]any{
		"reviewed_by": "reviewer-1",
		"review_note": "可以推进",
	})
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve proposal status = %d, want 200", resp.StatusCode)
	}
	if err := decodeJSON(resp, &proposal); err != nil {
		t.Fatalf("decode approved proposal: %v", err)
	}
	if proposal.Status != core.ProposalMerged {
		t.Fatalf("proposal status = %s, want merged", proposal.Status)
	}
	if proposal.InitiativeID == nil {
		t.Fatal("proposal initiative_id should not be nil after approve")
	}

	resp, err = get(ts, "/proposals/"+itoa64(proposal.ID))
	if err != nil {
		t.Fatalf("get proposal: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get proposal status = %d, want 200", resp.StatusCode)
	}

	resp, err = get(ts, "/threads/"+itoa64(threadID)+"/proposals?status=merged")
	if err != nil {
		t.Fatalf("list proposals: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list proposals status = %d, want 200", resp.StatusCode)
	}
	var proposals []core.ThreadProposal
	if err := decodeJSON(resp, &proposals); err != nil {
		t.Fatalf("decode proposal list: %v", err)
	}
	if len(proposals) != 1 || proposals[0].ID != proposal.ID {
		t.Fatalf("proposal list = %+v", proposals)
	}

	initiative, err := h.store.GetInitiative(ctx, *proposal.InitiativeID)
	if err != nil {
		t.Fatalf("GetInitiative: %v", err)
	}
	if initiative.Status != core.InitiativeDraft {
		t.Fatalf("initiative status = %s, want draft", initiative.Status)
	}

	items, err := h.store.ListInitiativeItems(ctx, initiative.ID)
	if err != nil {
		t.Fatalf("ListInitiativeItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("initiative items = %d, want 2", len(items))
	}
}

package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func newProposalTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(filepath.Join(t.TempDir(), "proposal-test.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreThreadProposalCRUD(t *testing.T) {
	store := newProposalTestStore(t)
	ctx := context.Background()

	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "proposal thread", Status: core.ThreadActive, OwnerID: "user-1"})
	if err != nil {
		t.Fatalf("CreateThread() error = %v", err)
	}

	proposalID, err := store.CreateThreadProposal(ctx, &core.ThreadProposal{
		ThreadID:   threadID,
		Title:      "跨项目方案",
		Summary:    "先做后端再做前端",
		ProposedBy: "lead-1",
		Status:     core.ProposalDraft,
		WorkItemDrafts: []core.ProposalWorkItemDraft{
			{TempID: "backend", Title: "后端改造"},
			{TempID: "frontend", Title: "前端接入", DependsOn: []string{"backend"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateThreadProposal() error = %v", err)
	}

	proposal, err := store.GetThreadProposal(ctx, proposalID)
	if err != nil {
		t.Fatalf("GetThreadProposal() error = %v", err)
	}
	if proposal.Title != "跨项目方案" || proposal.ThreadID != threadID {
		t.Fatalf("GetThreadProposal() = %+v", proposal)
	}

	proposal.Status = core.ProposalOpen
	proposal.ReviewNote = "等待审批"
	if err := store.UpdateThreadProposal(ctx, proposal); err != nil {
		t.Fatalf("UpdateThreadProposal() error = %v", err)
	}

	items, err := store.ListThreadProposals(ctx, core.ProposalFilter{ThreadID: &threadID})
	if err != nil {
		t.Fatalf("ListThreadProposals() error = %v", err)
	}
	if len(items) != 1 || items[0].Status != core.ProposalOpen {
		t.Fatalf("ListThreadProposals() = %+v", items)
	}

	if err := store.DeleteThreadProposal(ctx, proposalID); err != nil {
		t.Fatalf("DeleteThreadProposal() error = %v", err)
	}
	if _, err := store.GetThreadProposal(ctx, proposalID); err != core.ErrNotFound {
		t.Fatalf("GetThreadProposal() after delete error = %v, want not found", err)
	}
}

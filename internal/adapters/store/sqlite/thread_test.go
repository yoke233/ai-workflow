package sqlite

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestThreadCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	thread := &core.Thread{
		Title:    "design discussion",
		OwnerID:  "user-1",
		Metadata: map[string]any{"topic": "architecture"},
	}
	id, err := s.CreateThread(ctx, thread)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive id")
	}
	if thread.Status != core.ThreadActive {
		t.Fatalf("expected active status, got %s", thread.Status)
	}

	got, err := s.GetThread(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "design discussion" || got.Status != core.ThreadActive {
		t.Fatalf("unexpected thread: %+v", got)
	}
	if got.OwnerID != "user-1" {
		t.Fatalf("owner_id not preserved: %s", got.OwnerID)
	}
	if got.Metadata["topic"] != "architecture" {
		t.Fatalf("metadata not preserved: %v", got.Metadata)
	}

	// Update
	got.Title = "updated title"
	got.Summary = "summary of discussion"
	got.Status = core.ThreadClosed
	if err := s.UpdateThread(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetThread(ctx, id)
	if got.Title != "updated title" || got.Status != core.ThreadClosed {
		t.Fatalf("update not applied: %+v", got)
	}
	if got.Summary != "summary of discussion" {
		t.Fatalf("summary not updated: %s", got.Summary)
	}

	// List
	threads, err := s.ListThreads(ctx, core.ThreadFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}

	// List with status filter
	active := core.ThreadActive
	threads, err = s.ListThreads(ctx, core.ThreadFilter{Status: &active, Limit: 10})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("expected 0 active threads, got %d", len(threads))
	}

	// Delete
	if err := s.DeleteThread(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetThread(ctx, id)
	if err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestThreadNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetThread(context.Background(), 9999)
	if err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestThreadDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteThread(context.Background(), 9999); err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestThreadUpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateThread(context.Background(), &core.Thread{ID: 9999, Title: "x"})
	if err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestThreadTitleRequired(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateThread(context.Background(), &core.Thread{Title: "  "})
	if err == nil {
		t.Fatal("expected error for blank title")
	}
}

func TestThreadMessageCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create thread first.
	thread := &core.Thread{Title: "msg-test"}
	threadID, err := s.CreateThread(ctx, thread)
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	// Create message.
	msg := &core.ThreadMessage{
		ThreadID: threadID,
		SenderID: "user-1",
		Role:     "human",
		Content:  "hello world",
		Metadata: map[string]any{"source": "test"},
	}
	msgID, err := s.CreateThreadMessage(ctx, msg)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if msgID <= 0 {
		t.Fatal("expected positive message id")
	}

	// Create second message.
	msg2 := &core.ThreadMessage{
		ThreadID: threadID,
		SenderID: "agent-1",
		Role:     "agent",
		Content:  "hi there",
	}
	if _, err := s.CreateThreadMessage(ctx, msg2); err != nil {
		t.Fatalf("create message 2: %v", err)
	}

	// List messages.
	msgs, err := s.ListThreadMessages(ctx, threadID, 10, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello world" {
		t.Fatalf("expected first message content 'hello world', got %q", msgs[0].Content)
	}
	if msgs[1].Role != "agent" {
		t.Fatalf("expected second message role 'agent', got %q", msgs[1].Role)
	}
}

func TestThreadWorkItemLinkCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create thread and issue (work item).
	thread := &core.Thread{Title: "link-test"}
	threadID, err := s.CreateThread(ctx, thread)
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	issue := &core.Issue{Title: "work-item-1", Status: core.IssueOpen}
	issueID, err := s.CreateIssue(ctx, issue)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	// Create link.
	link := &core.ThreadWorkItemLink{
		ThreadID:     threadID,
		WorkItemID:   issueID,
		RelationType: "related",
		IsPrimary:    true,
	}
	linkID, err := s.CreateThreadWorkItemLink(ctx, link)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if linkID <= 0 {
		t.Fatal("expected positive link id")
	}

	// List by thread.
	links, err := s.ListWorkItemsByThread(ctx, threadID)
	if err != nil {
		t.Fatalf("list by thread: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].WorkItemID != issueID || links[0].IsPrimary != true {
		t.Fatalf("unexpected link: %+v", links[0])
	}

	// List by work item.
	links2, err := s.ListThreadsByWorkItem(ctx, issueID)
	if err != nil {
		t.Fatalf("list by work item: %v", err)
	}
	if len(links2) != 1 || links2[0].ThreadID != threadID {
		t.Fatalf("unexpected reverse link: %+v", links2)
	}

	// Duplicate link should fail (UNIQUE constraint).
	dup := &core.ThreadWorkItemLink{
		ThreadID:     threadID,
		WorkItemID:   issueID,
		RelationType: "drives",
	}
	if _, err := s.CreateThreadWorkItemLink(ctx, dup); err == nil {
		t.Fatal("expected error for duplicate link")
	}

	// Delete specific link.
	if err := s.DeleteThreadWorkItemLink(ctx, threadID, issueID); err != nil {
		t.Fatalf("delete link: %v", err)
	}
	links, _ = s.ListWorkItemsByThread(ctx, threadID)
	if len(links) != 0 {
		t.Fatalf("expected 0 links after delete, got %d", len(links))
	}
}

func TestThreadWorkItemLinkCleanup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Setup: thread + 2 issues + links.
	thread := &core.Thread{Title: "cleanup-test"}
	threadID, _ := s.CreateThread(ctx, thread)

	issue1 := &core.Issue{Title: "wi-1", Status: core.IssueOpen}
	issueID1, _ := s.CreateIssue(ctx, issue1)
	issue2 := &core.Issue{Title: "wi-2", Status: core.IssueOpen}
	issueID2, _ := s.CreateIssue(ctx, issue2)

	s.CreateThreadWorkItemLink(ctx, &core.ThreadWorkItemLink{ThreadID: threadID, WorkItemID: issueID1, RelationType: "related"})
	s.CreateThreadWorkItemLink(ctx, &core.ThreadWorkItemLink{ThreadID: threadID, WorkItemID: issueID2, RelationType: "drives"})

	// Cleanup by thread: deletes all links for that thread.
	if err := s.DeleteThreadWorkItemLinksByThread(ctx, threadID); err != nil {
		t.Fatalf("cleanup by thread: %v", err)
	}
	links, _ := s.ListWorkItemsByThread(ctx, threadID)
	if len(links) != 0 {
		t.Fatalf("expected 0 links after thread cleanup, got %d", len(links))
	}

	// Re-create links for work-item cleanup test.
	s.CreateThreadWorkItemLink(ctx, &core.ThreadWorkItemLink{ThreadID: threadID, WorkItemID: issueID1, RelationType: "related"})

	// Cleanup by work item.
	if err := s.DeleteThreadWorkItemLinksByWorkItem(ctx, issueID1); err != nil {
		t.Fatalf("cleanup by work item: %v", err)
	}
	links2, _ := s.ListThreadsByWorkItem(ctx, issueID1)
	if len(links2) != 0 {
		t.Fatalf("expected 0 links after work item cleanup, got %d", len(links2))
	}
}

func TestThreadParticipantCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create thread.
	thread := &core.Thread{Title: "participant-test"}
	threadID, err := s.CreateThread(ctx, thread)
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	// Add participant.
	p := &core.ThreadParticipant{
		ThreadID: threadID,
		UserID:   "user-1",
		Role:     "owner",
	}
	pID, err := s.AddThreadParticipant(ctx, p)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if pID <= 0 {
		t.Fatal("expected positive participant id")
	}

	// Add second participant.
	p2 := &core.ThreadParticipant{
		ThreadID: threadID,
		UserID:   "agent-1",
		Role:     "agent",
	}
	if _, err := s.AddThreadParticipant(ctx, p2); err != nil {
		t.Fatalf("add participant 2: %v", err)
	}

	// List participants.
	participants, err := s.ListThreadParticipants(ctx, threadID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}

	// Remove participant.
	if err := s.RemoveThreadParticipant(ctx, threadID, "user-1"); err != nil {
		t.Fatalf("remove participant: %v", err)
	}

	participants, _ = s.ListThreadParticipants(ctx, threadID)
	if len(participants) != 1 {
		t.Fatalf("expected 1 participant after remove, got %d", len(participants))
	}

	// Remove non-existent.
	if err := s.RemoveThreadParticipant(ctx, threadID, "nobody"); err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

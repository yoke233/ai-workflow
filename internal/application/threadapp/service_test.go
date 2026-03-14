package threadapp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	"github.com/yoke233/ai-workflow/internal/core"
)

type runtimeStub struct {
	err          error
	calls        int
	lastThreadID int64
}

func (r *runtimeStub) CleanupThread(_ context.Context, threadID int64) error {
	r.calls++
	r.lastThreadID = threadID
	return r.err
}

type sqliteTxAdapter struct {
	base core.TransactionalStore
	wrap func(core.Store) (TxStore, error)
}

func (a sqliteTxAdapter) InTx(ctx context.Context, fn func(ctx context.Context, store TxStore) error) error {
	return a.base.InTx(ctx, func(store core.Store) error {
		txStore, err := a.bind(store)
		if err != nil {
			return err
		}
		return fn(ctx, txStore)
	})
}

func (a sqliteTxAdapter) bind(store core.Store) (TxStore, error) {
	if a.wrap != nil {
		return a.wrap(store)
	}
	txStore, ok := store.(TxStore)
	if !ok {
		return nil, fmt.Errorf("unexpected tx store type %T", store)
	}
	return txStore, nil
}

type failingLinkStore struct {
	*sqlite.Store
	failCreateLink bool
}

func (s *failingLinkStore) CreateThreadWorkItemLink(ctx context.Context, link *core.ThreadWorkItemLink) (int64, error) {
	if s.failCreateLink {
		return 0, errors.New("create link failed")
	}
	return s.Store.CreateThreadWorkItemLink(ctx, link)
}

type failingDeleteStore struct {
	*sqlite.Store
	failDeleteMembers  bool
	failDeleteMessages bool
}

func (s *failingDeleteStore) DeleteThreadMembersByThread(ctx context.Context, threadID int64) error {
	if s.failDeleteMembers {
		return errors.New("delete members failed")
	}
	return s.Store.DeleteThreadMembersByThread(ctx, threadID)
}

func (s *failingDeleteStore) DeleteThreadMessagesByThread(ctx context.Context, threadID int64) error {
	if s.failDeleteMessages {
		return errors.New("delete messages failed")
	}
	return s.Store.DeleteThreadMessagesByThread(ctx, threadID)
}

func newThreadAppTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "threadapp-test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newSQLiteThreadAppService(store Store, tx Tx, runtime Runtime) *Service {
	return New(Config{
		Store:   store,
		Tx:      tx,
		Runtime: runtime,
	})
}

func newSQLiteTxAdapter(store *sqlite.Store, wrap func(core.Store) (TxStore, error)) Tx {
	return sqliteTxAdapter{
		base: store,
		wrap: wrap,
	}
}

func createThreadFixture(t *testing.T, store *sqlite.Store, withMessage bool, withLink bool) (threadID int64, workItemID int64) {
	t.Helper()
	ctx := context.Background()

	threadID, err := store.CreateThread(ctx, &core.Thread{
		Title:   "fixture-thread",
		Summary: "fixture summary",
		Status:  core.ThreadActive,
		OwnerID: "owner-1",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	if _, err := store.AddThreadMember(ctx, &core.ThreadMember{
		ThreadID: threadID,
		Kind:     core.ThreadMemberKindHuman,
		UserID:   "owner-1",
		Role:     "owner",
	}); err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	if _, err := store.AddThreadMember(ctx, &core.ThreadMember{
		ThreadID: threadID,
		Kind:     core.ThreadMemberKindHuman,
		UserID:   "member-2",
		Role:     "member",
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if withMessage {
		if _, err := store.CreateThreadMessage(ctx, &core.ThreadMessage{
			ThreadID: threadID,
			SenderID: "owner-1",
			Role:     "human",
			Content:  "hello",
		}); err != nil {
			t.Fatalf("create thread message: %v", err)
		}
	}
	if withLink {
		workItemID, err = store.CreateWorkItem(ctx, &core.WorkItem{
			Title:    "linked work item",
			Body:     "linked body",
			Status:   core.WorkItemOpen,
			Priority: core.PriorityMedium,
		})
		if err != nil {
			t.Fatalf("create work item: %v", err)
		}
		if _, err := store.CreateThreadWorkItemLink(ctx, &core.ThreadWorkItemLink{
			ThreadID:     threadID,
			WorkItemID:   workItemID,
			RelationType: "related",
			IsPrimary:    true,
		}); err != nil {
			t.Fatalf("create link: %v", err)
		}
	}
	return threadID, workItemID
}

func TestServiceCreateWorkItemFromThreadUsesSummaryAndCreatesPrimaryLink(t *testing.T) {
	store := newThreadAppTestStore(t)
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), nil)
	ctx := context.Background()

	thread := &core.Thread{Title: "thread-1", Summary: "Ship the feature from summary."}
	threadID, err := store.CreateThread(ctx, thread)
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	thread.ID = threadID

	result, err := svc.CreateWorkItemFromThread(ctx, CreateWorkItemFromThreadInput{
		ThreadID:      thread.ID,
		WorkItemTitle: "spawned work item",
	})
	if err != nil {
		t.Fatalf("CreateWorkItemFromThread: %v", err)
	}
	if result.Thread == nil || result.Thread.ID != thread.ID {
		t.Fatalf("unexpected thread result: %+v", result.Thread)
	}
	if result.WorkItem == nil {
		t.Fatal("expected work item result")
	}
	if result.WorkItem.Body != "Ship the feature from summary." {
		t.Fatalf("expected summary-backed body, got %q", result.WorkItem.Body)
	}
	if result.WorkItem.Metadata["source_type"] != "thread_summary" {
		t.Fatalf("expected source_type thread_summary, got %#v", result.WorkItem.Metadata["source_type"])
	}
	if result.Link == nil || result.Link.ThreadID != thread.ID || result.Link.WorkItemID != result.WorkItem.ID {
		t.Fatalf("unexpected link result: %+v", result.Link)
	}
	if !result.Link.IsPrimary || result.Link.RelationType != "drives" {
		t.Fatalf("unexpected link metadata: %+v", result.Link)
	}

	links, err := store.ListWorkItemsByThread(ctx, thread.ID)
	if err != nil {
		t.Fatalf("list links by thread: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 persisted link, got %d", len(links))
	}
}

func TestServiceCreateWorkItemFromThreadExplicitBodyMarksManualSource(t *testing.T) {
	store := newThreadAppTestStore(t)
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), nil)
	ctx := context.Background()

	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "thread-2"})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	result, err := svc.CreateWorkItemFromThread(ctx, CreateWorkItemFromThreadInput{
		ThreadID:      threadID,
		WorkItemTitle: "spawned work item",
		WorkItemBody:  "Manual body from request.",
	})
	if err != nil {
		t.Fatalf("CreateWorkItemFromThread: %v", err)
	}
	if result.WorkItem == nil {
		t.Fatal("expected work item result")
	}
	if result.WorkItem.Body != "Manual body from request." {
		t.Fatalf("unexpected work item body: %q", result.WorkItem.Body)
	}
	if result.WorkItem.Metadata["source_type"] != "thread_manual" {
		t.Fatalf("expected source_type thread_manual, got %#v", result.WorkItem.Metadata["source_type"])
	}
	if result.WorkItem.Metadata["body_from_summary"] != false {
		t.Fatalf("expected body_from_summary false, got %#v", result.WorkItem.Metadata["body_from_summary"])
	}
}

func TestServiceCreateWorkItemFromThreadRequiresSummaryWhenBodyMissing(t *testing.T) {
	store := newThreadAppTestStore(t)
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), nil)
	ctx := context.Background()

	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "thread-3"})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	_, err = svc.CreateWorkItemFromThread(ctx, CreateWorkItemFromThreadInput{
		ThreadID:      threadID,
		WorkItemTitle: "spawned work item",
	})
	if CodeOf(err) != CodeMissingThreadSummary {
		t.Fatalf("expected %s, got %v", CodeMissingThreadSummary, err)
	}
}

func TestServiceCreateWorkItemFromThreadRollsBackOnLinkFailure(t *testing.T) {
	base := newThreadAppTestStore(t)
	store := &failingLinkStore{Store: base, failCreateLink: true}
	tx := newSQLiteTxAdapter(base, func(txStore core.Store) (TxStore, error) {
		sqliteStore, ok := txStore.(*sqlite.Store)
		if !ok {
			return nil, fmt.Errorf("unexpected tx store type %T", txStore)
		}
		return &failingLinkStore{Store: sqliteStore, failCreateLink: true}, nil
	})
	svc := newSQLiteThreadAppService(store, tx, nil)
	ctx := context.Background()

	threadID, err := base.CreateThread(ctx, &core.Thread{
		Title:   "thread-4",
		Summary: "Use me as the body.",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	_, err = svc.CreateWorkItemFromThread(ctx, CreateWorkItemFromThreadInput{
		ThreadID:      threadID,
		WorkItemTitle: "spawned work item",
	})
	if err == nil {
		t.Fatal("expected create work item from thread to fail")
	}

	items, err := base.ListWorkItems(ctx, core.WorkItemFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 work items after rollback, got %d", len(items))
	}

	links, err := base.ListWorkItemsByThread(ctx, threadID)
	if err != nil {
		t.Fatalf("list thread links: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected 0 links after rollback, got %d", len(links))
	}
}

func TestServiceDeleteThreadFailsFastWhenRuntimeCleanupFails(t *testing.T) {
	store := newThreadAppTestStore(t)
	threadID, _ := createThreadFixture(t, store, true, true)
	runtime := &runtimeStub{err: errors.New("cleanup failed")}
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), runtime)
	ctx := context.Background()

	err := svc.DeleteThread(ctx, threadID)
	if CodeOf(err) != CodeCleanupThreadFailed {
		t.Fatalf("expected %s, got %v", CodeCleanupThreadFailed, err)
	}
	if runtime.calls != 1 || runtime.lastThreadID != threadID {
		t.Fatalf("unexpected runtime cleanup call state: %+v", runtime)
	}

	if _, err := store.GetThread(ctx, threadID); err != nil {
		t.Fatalf("expected thread to remain after runtime cleanup failure: %v", err)
	}
	messages, err := store.ListThreadMessages(ctx, threadID, 20, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected messages to remain, got %d", len(messages))
	}
	members, err := store.ListThreadMembers(ctx, threadID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected members to remain, got %d", len(members))
	}
	links, err := store.ListWorkItemsByThread(ctx, threadID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected links to remain, got %d", len(links))
	}
}

func TestServiceDeleteThreadRollsBackWhenAggregateDeleteFails(t *testing.T) {
	base := newThreadAppTestStore(t)
	threadID, _ := createThreadFixture(t, base, true, true)
	store := &failingDeleteStore{Store: base, failDeleteMembers: true}
	tx := newSQLiteTxAdapter(base, func(txStore core.Store) (TxStore, error) {
		sqliteStore, ok := txStore.(*sqlite.Store)
		if !ok {
			return nil, fmt.Errorf("unexpected tx store type %T", txStore)
		}
		return &failingDeleteStore{Store: sqliteStore, failDeleteMembers: true}, nil
	})
	svc := newSQLiteThreadAppService(store, tx, nil)
	ctx := context.Background()

	err := svc.DeleteThread(ctx, threadID)
	if err == nil {
		t.Fatal("expected delete thread to fail")
	}

	if _, err := base.GetThread(ctx, threadID); err != nil {
		t.Fatalf("expected thread to remain after rollback: %v", err)
	}
	messages, err := base.ListThreadMessages(ctx, threadID, 20, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected messages rollback, got %d", len(messages))
	}
	members, err := base.ListThreadMembers(ctx, threadID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected members rollback, got %d", len(members))
	}
	links, err := base.ListWorkItemsByThread(ctx, threadID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected links rollback, got %d", len(links))
	}
}

func TestServiceCrystallizeChatSessionCreatesThreadOnly(t *testing.T) {
	store := newThreadAppTestStore(t)
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), nil)
	ctx := context.Background()

	result, err := svc.CrystallizeChatSession(ctx, CrystallizeChatSessionInput{
		SessionID:          "chat-1",
		ThreadTitle:        "Design API shape",
		ThreadSummary:      "Discuss API structure",
		OwnerID:            "owner-1",
		ParticipantUserIDs: []string{"owner-1", "member-2"},
	})
	if err != nil {
		t.Fatalf("CrystallizeChatSession: %v", err)
	}
	if result.Thread == nil || result.Thread.ID == 0 {
		t.Fatalf("expected persisted thread, got %+v", result.Thread)
	}
	if result.WorkItem != nil {
		t.Fatalf("expected no work item, got %+v", result.WorkItem)
	}
	if result.Thread.Metadata["source_chat_session_id"] != "chat-1" {
		t.Fatalf("unexpected thread metadata: %#v", result.Thread.Metadata)
	}
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
	members, err := store.ListThreadMembers(ctx, result.Thread.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 persisted members, got %d", len(members))
	}
	items, err := store.ListWorkItems(ctx, core.WorkItemFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no work items, got %d", len(items))
	}
}

func TestServiceCrystallizeChatSessionCreatesThreadAndWorkItem(t *testing.T) {
	store := newThreadAppTestStore(t)
	svc := newSQLiteThreadAppService(store, newSQLiteTxAdapter(store, nil), nil)
	ctx := context.Background()

	result, err := svc.CrystallizeChatSession(ctx, CrystallizeChatSessionInput{
		SessionID:      "chat-2",
		ThreadTitle:    "Ship feature",
		ThreadSummary:  "Use summary as work item body",
		OwnerID:        "owner-1",
		CreateWorkItem: true,
		WorkItemTitle:  "Implement feature",
	})
	if err != nil {
		t.Fatalf("CrystallizeChatSession: %v", err)
	}
	if result.Thread == nil || result.Thread.ID == 0 {
		t.Fatalf("expected thread result, got %+v", result.Thread)
	}
	if result.WorkItem == nil || result.WorkItem.ID == 0 {
		t.Fatalf("expected work item result, got %+v", result.WorkItem)
	}
	if result.WorkItem.Body != "Use summary as work item body" {
		t.Fatalf("expected summary-backed work item body, got %q", result.WorkItem.Body)
	}
	links, err := store.ListWorkItemsByThread(ctx, result.Thread.ID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 persisted link, got %d", len(links))
	}
	if !links[0].IsPrimary || links[0].RelationType != "drives" {
		t.Fatalf("unexpected link: %+v", links[0])
	}
}

func TestServiceCrystallizeChatSessionRollsBackWhenLinkCreationFails(t *testing.T) {
	base := newThreadAppTestStore(t)
	store := &failingLinkStore{Store: base, failCreateLink: true}
	tx := newSQLiteTxAdapter(base, func(txStore core.Store) (TxStore, error) {
		sqliteStore, ok := txStore.(*sqlite.Store)
		if !ok {
			return nil, fmt.Errorf("unexpected tx store type %T", txStore)
		}
		return &failingLinkStore{Store: sqliteStore, failCreateLink: true}, nil
	})
	svc := newSQLiteThreadAppService(store, tx, nil)
	ctx := context.Background()

	_, err := svc.CrystallizeChatSession(ctx, CrystallizeChatSessionInput{
		SessionID:      "chat-3",
		ThreadTitle:    "Broken materialization",
		ThreadSummary:  "summary body",
		OwnerID:        "owner-1",
		CreateWorkItem: true,
		WorkItemTitle:  "Should rollback",
	})
	if err == nil {
		t.Fatal("expected crystallize chat session to fail")
	}

	threads, err := base.ListThreads(ctx, core.ThreadFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("expected 0 threads after rollback, got %d", len(threads))
	}
	items, err := base.ListWorkItems(ctx, core.WorkItemFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 work items after rollback, got %d", len(items))
	}
}

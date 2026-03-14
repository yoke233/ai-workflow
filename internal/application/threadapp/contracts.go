package threadapp

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/core"
)

type ThreadReader interface {
	GetThread(ctx context.Context, id int64) (*core.Thread, error)
}

type WorkItemReader interface {
	GetWorkItem(ctx context.Context, id int64) (*core.WorkItem, error)
}

type ThreadWriter interface {
	CreateThread(ctx context.Context, thread *core.Thread) (int64, error)
	DeleteThread(ctx context.Context, id int64) error
}

type ThreadMessageWriter interface {
	DeleteThreadMessagesByThread(ctx context.Context, threadID int64) error
}

type ThreadMemberWriter interface {
	AddThreadMember(ctx context.Context, member *core.ThreadMember) (int64, error)
	DeleteThreadMembersByThread(ctx context.Context, threadID int64) error
}

type ThreadLinkWriter interface {
	CreateThreadWorkItemLink(ctx context.Context, link *core.ThreadWorkItemLink) (int64, error)
	DeleteThreadWorkItemLink(ctx context.Context, threadID, workItemID int64) error
	DeleteThreadWorkItemLinksByThread(ctx context.Context, threadID int64) error
}

type WorkItemWriter interface {
	CreateWorkItem(ctx context.Context, workItem *core.WorkItem) (int64, error)
	DeleteWorkItem(ctx context.Context, id int64) error
}

// Store is the application-facing persistence port for thread workflows.
type Store interface {
	ThreadReader
	WorkItemReader
	ThreadWriter
	ThreadMessageWriter
	ThreadMemberWriter
	ThreadLinkWriter
	WorkItemWriter
}

type TxStore interface {
	Store
}

type Tx interface {
	InTx(ctx context.Context, fn func(ctx context.Context, store TxStore) error) error
}

// Runtime is the optional runtime port for cleaning up live thread sessions.
type Runtime interface {
	CleanupThread(ctx context.Context, threadID int64) error
}

type CreateThreadInput struct {
	Title              string
	OwnerID            string
	Summary            string
	Metadata           map[string]any
	ParticipantUserIDs []string
}

type CreateThreadResult struct {
	Thread       *core.Thread
	Participants []*core.ThreadMember
}

type LinkThreadWorkItemInput struct {
	ThreadID     int64
	WorkItemID   int64
	RelationType string
	IsPrimary    bool
}

type CreateWorkItemFromThreadInput struct {
	ThreadID      int64
	WorkItemTitle string
	WorkItemBody  string
	ProjectID     *int64
}

type CreateWorkItemFromThreadResult struct {
	Thread   *core.Thread
	WorkItem *core.WorkItem
	Link     *core.ThreadWorkItemLink
}

type CrystallizeChatSessionInput struct {
	SessionID          string
	ThreadTitle        string
	ThreadSummary      string
	OwnerID            string
	ParticipantUserIDs []string
	CreateWorkItem     bool
	WorkItemTitle      string
	WorkItemBody       string
	ProjectID          *int64
}

type CrystallizeChatSessionResult struct {
	Thread       *core.Thread
	WorkItem     *core.WorkItem
	Participants []*core.ThreadMember
}

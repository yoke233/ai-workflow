package core

import "context"

// ContextStore is the Go client abstraction for context storage (OpenViking / mock / sqlite).
// All URI parameters use the viking:// scheme.
type ContextStore interface {
	Plugin

	// Basic CRUD
	Read(ctx context.Context, uri string) ([]byte, error)
	Write(ctx context.Context, uri string, content []byte) error
	List(ctx context.Context, uri string) ([]ContextEntry, error)
	Remove(ctx context.Context, uri string) error

	// Layered queries (L0/L1)
	Abstract(ctx context.Context, uri string) (string, error)
	Overview(ctx context.Context, uri string) (string, error)

	// Semantic search
	Find(ctx context.Context, query string, opts FindOpts) ([]ContextResult, error)
	Search(ctx context.Context, query string, sessionID string, opts SearchOpts) ([]ContextResult, error)

	// Resource management
	AddResource(ctx context.Context, path string, opts AddResourceOpts) error
	Link(ctx context.Context, from string, to []string, reason string) error

	// Session
	CreateSession(ctx context.Context, id string) (ContextSession, error)
	GetSession(ctx context.Context, id string) (ContextSession, error)

	// Materialize to local directory (coder-only)
	Materialize(ctx context.Context, uri, targetDir string) error
}

// ContextSession tracks dialogue within an ACP session.
type ContextSession interface {
	ID() string
	AddMessage(role string, parts []MessagePart) error
	Used(contexts []string) error
	Commit() (CommitResult, error)
}

// ContextEntry represents a file or directory in the context store.
type ContextEntry struct {
	URI   string
	Name  string
	IsDir bool
}

// ContextResult represents a search or find result.
type ContextResult struct {
	URI     string
	Score   float64
	Content string
}

// FindOpts controls the scope of a Find query.
type FindOpts struct {
	TargetURI string
	Limit     int
}

// SearchOpts controls the scope of a Search query.
type SearchOpts struct {
	TargetURI string
	Limit     int
}

// AddResourceOpts configures resource import.
type AddResourceOpts struct {
	TargetURI string
	Reason    string
	Wait      bool
}

// MessagePart is a component of a session message.
type MessagePart struct {
	Type    string // "text" | "context" | "tool"
	Content string
	URI     string // populated for "context" type
}

// CommitResult is the outcome of committing a session.
type CommitResult struct {
	Status            string // "committed"
	MemoriesExtracted int
	Archived          bool
}

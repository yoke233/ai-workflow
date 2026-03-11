package flow

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/core"
)

// Store is the application-facing persistence port required by flow orchestration.
// It intentionally exposes only the sub-stores used by the flow application layer.
type Store interface {
	core.ProjectStore
	core.ResourceBindingStore
	core.FlowStore
	core.StepStore
	core.ExecutionStore
	core.ArtifactStore
	core.BriefingStore
}

// EventStore is the persistence port required for persisting emitted events.
type EventStore interface {
	core.EventStore
}

// EventPublisher is the minimal outbound event port required by flow orchestration.
type EventPublisher interface {
	Publish(ctx context.Context, event core.Event)
}

// EventBus is the subscribe-capable event port used by background consumers.
type EventBus interface {
	EventPublisher
	Subscribe(opts core.SubscribeOpts) *core.Subscription
}

// WorkspaceProvider prepares and releases isolated workspaces for a flow run.
type WorkspaceProvider interface {
	Prepare(ctx context.Context, project *core.Project, bindings []*core.ResourceBinding, flowID int64) (*core.Workspace, error)
	Release(ctx context.Context, ws *core.Workspace) error
}

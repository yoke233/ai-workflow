package probe

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/core"
)

// Store is the application-facing persistence port required by execution probe use cases.
type Store interface {
	core.ExecutionStore
	core.EventStore
	core.ExecutionProbeStore
}

// EventPublisher is the minimal outbound event port required by probe workflows.
type EventPublisher interface {
	Publish(ctx context.Context, event core.Event)
}

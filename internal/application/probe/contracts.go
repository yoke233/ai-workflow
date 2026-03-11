package probe

import (
	"context"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// Service is the minimal application contract required by transport adapters.
type Service interface {
	ListExecutionProbes(ctx context.Context, executionID int64) ([]*core.ExecutionProbe, error)
	GetLatestExecutionProbe(ctx context.Context, executionID int64) (*core.ExecutionProbe, error)
	RequestExecutionProbe(ctx context.Context, executionID int64, source core.ExecutionProbeTriggerSource, question string, timeout time.Duration) (*core.ExecutionProbe, error)
}

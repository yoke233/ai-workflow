package flow

import "context"

// Runner is the minimal application contract required to execute and cancel flows.
type Runner interface {
	Run(ctx context.Context, flowID int64) error
	Cancel(ctx context.Context, flowID int64) error
}

// Scheduler is the minimal application contract required by transport adapters.
type Scheduler interface {
	Submit(ctx context.Context, flowID int64) error
	Cancel(ctx context.Context, flowID int64) error
	Stats() SchedulerStats
}

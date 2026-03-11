package core

import (
	"context"
	"time"
)

// AnalyticsStore provides aggregated analytics queries.
type AnalyticsStore interface {
	// ProjectErrorRanking returns projects ordered by failure count (desc).
	ProjectErrorRanking(ctx context.Context, filter AnalyticsFilter) ([]ProjectErrorRank, error)

	// FlowBottleneckSteps returns steps that are slowest or fail most within flows.
	FlowBottleneckSteps(ctx context.Context, filter AnalyticsFilter) ([]StepBottleneck, error)

	// ExecutionDurationStats returns execution duration percentiles per flow.
	ExecutionDurationStats(ctx context.Context, filter AnalyticsFilter) ([]FlowDurationStat, error)

	// ErrorBreakdown returns error counts grouped by error_kind.
	ErrorBreakdown(ctx context.Context, filter AnalyticsFilter) ([]ErrorKindCount, error)

	// RecentFailures returns the most recent failed executions with context.
	RecentFailures(ctx context.Context, filter AnalyticsFilter) ([]FailureRecord, error)

	// FlowStatusDistribution returns flow counts grouped by status.
	FlowStatusDistribution(ctx context.Context, filter AnalyticsFilter) ([]StatusCount, error)
}

// AnalyticsFilter constrains analytics queries.
type AnalyticsFilter struct {
	ProjectID *int64     `json:"project_id,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	Limit     int        `json:"limit,omitempty"`
}

// ProjectErrorRank represents a project's error ranking.
type ProjectErrorRank struct {
	ProjectID   int64   `json:"project_id"`
	ProjectName string  `json:"project_name"`
	TotalFlows  int     `json:"total_flows"`
	FailedFlows int     `json:"failed_flows"`
	FailureRate float64 `json:"failure_rate"`
	FailedExecs int     `json:"failed_execs"`
}

// StepBottleneck represents a step that is a bottleneck in flow execution.
type StepBottleneck struct {
	StepID       int64   `json:"step_id"`
	StepName     string  `json:"step_name"`
	FlowID       int64   `json:"flow_id"`
	FlowName     string  `json:"flow_name"`
	ProjectID    *int64  `json:"project_id,omitempty"`
	AvgDurationS float64 `json:"avg_duration_s"`
	MaxDurationS float64 `json:"max_duration_s"`
	ExecCount    int     `json:"exec_count"`
	FailCount    int     `json:"fail_count"`
	RetryCount   int     `json:"retry_count"`
	FailRate     float64 `json:"fail_rate"`
}

// FlowDurationStat provides duration statistics for a flow.
type FlowDurationStat struct {
	FlowID       int64   `json:"flow_id"`
	FlowName     string  `json:"flow_name"`
	ProjectID    *int64  `json:"project_id,omitempty"`
	ExecCount    int     `json:"exec_count"`
	AvgDurationS float64 `json:"avg_duration_s"`
	MinDurationS float64 `json:"min_duration_s"`
	MaxDurationS float64 `json:"max_duration_s"`
	P50DurationS float64 `json:"p50_duration_s"`
}

// ErrorKindCount counts errors by classification.
type ErrorKindCount struct {
	ErrorKind ErrorKind `json:"error_kind"`
	Count     int       `json:"count"`
	Pct       float64   `json:"pct"`
}

// FailureRecord is a recent failed execution with context.
type FailureRecord struct {
	ExecID       int64     `json:"exec_id"`
	StepID       int64     `json:"step_id"`
	StepName     string    `json:"step_name"`
	FlowID       int64     `json:"flow_id"`
	FlowName     string    `json:"flow_name"`
	ProjectID    *int64    `json:"project_id,omitempty"`
	ProjectName  string    `json:"project_name,omitempty"`
	ErrorMessage string    `json:"error_message"`
	ErrorKind    ErrorKind `json:"error_kind"`
	Attempt      int       `json:"attempt"`
	DurationS    float64   `json:"duration_s"`
	FailedAt     time.Time `json:"failed_at"`
}

// StatusCount counts flows by status.
type StatusCount struct {
	Status FlowStatus `json:"status"`
	Count  int        `json:"count"`
}

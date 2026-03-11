package probe

import (
	"context"
	"time"
)

// Runtime sends a diagnostic probe to a running execution through the active runtime.
type Runtime interface {
	ProbeExecution(ctx context.Context, req ExecutionProbeRuntimeRequest) (*ExecutionProbeRuntimeResult, error)
}

// ExecutionProbeRuntimeRequest contains the routing data needed to send a probe.
type ExecutionProbeRuntimeRequest struct {
	ExecutionID  int64
	InvocationID string
	SessionID    string
	OwnerID      string
	Question     string
	Timeout      time.Duration
}

// ExecutionProbeRuntimeResult is the low-level runtime response for a probe request.
type ExecutionProbeRuntimeResult struct {
	Reachable  bool
	Answered   bool
	ReplyText  string
	Error      string
	ObservedAt time.Time
}

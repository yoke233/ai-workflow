package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

type ExecutionProbeWatchdogConfig struct {
	Enabled      bool
	Interval     time.Duration
	ProbeAfter   time.Duration
	IdleAfter    time.Duration
	ProbeTimeout time.Duration
	MaxAttempts  int
}

type ExecutionProbeWatchdog struct {
	store   core.Store
	service *ExecutionProbeService
	cfg     ExecutionProbeWatchdogConfig
}

func NewExecutionProbeWatchdog(store core.Store, service *ExecutionProbeService, cfg ExecutionProbeWatchdogConfig) *ExecutionProbeWatchdog {
	return &ExecutionProbeWatchdog{store: store, service: service, cfg: cfg}
}

func (w *ExecutionProbeWatchdog) Start(ctx context.Context) {
	if w == nil || !w.cfg.Enabled || w.service == nil || w.store == nil {
		return
	}

	interval := w.cfg.Interval
	if interval <= 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		w.runOnce(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *ExecutionProbeWatchdog) runOnce(ctx context.Context) {
	running, err := w.store.ListExecutionsByStatus(ctx, core.ExecRunning)
	if err != nil {
		slog.Warn("execution probe watchdog: list running executions failed", "error", err)
		return
	}

	now := time.Now().UTC()
	for _, execRec := range running {
		if execRec == nil {
			continue
		}
		if !w.shouldProbeExecution(ctx, now, execRec) {
			continue
		}
		if _, err := w.service.RequestExecutionProbe(ctx, execRec.ID, core.ExecutionProbeTriggerWatchdog, "", w.cfg.ProbeTimeout); err != nil && err != ErrExecutionProbeConflict && err != ErrExecutionNotRunning {
			slog.Warn("execution probe watchdog: request probe failed", "exec_id", execRec.ID, "error", err)
		}
	}
}

func (w *ExecutionProbeWatchdog) shouldProbeExecution(ctx context.Context, now time.Time, execRec *core.Execution) bool {
	startedAt := execRec.CreatedAt
	if execRec.StartedAt != nil {
		startedAt = *execRec.StartedAt
	}
	if w.cfg.ProbeAfter > 0 && now.Sub(startedAt) < w.cfg.ProbeAfter {
		return false
	}

	if active, err := w.store.GetActiveExecutionProbe(ctx, execRec.ID); err == nil && active != nil {
		return false
	} else if err != nil && err != core.ErrNotFound {
		slog.Warn("execution probe watchdog: read active probe failed", "exec_id", execRec.ID, "error", err)
		return false
	}

	probes, err := w.store.ListExecutionProbesByExecution(ctx, execRec.ID)
	if err != nil {
		slog.Warn("execution probe watchdog: list probes failed", "exec_id", execRec.ID, "error", err)
		return false
	}
	if w.cfg.MaxAttempts > 0 && len(probes) >= w.cfg.MaxAttempts {
		return false
	}

	lastActivity := startedAt
	latestEventAt, err := w.store.GetLatestExecutionEventTime(ctx, execRec.ID, core.EventExecAgentOutput)
	if err != nil {
		slog.Warn("execution probe watchdog: latest activity lookup failed", "exec_id", execRec.ID, "error", err)
		return false
	}
	if latestEventAt != nil {
		lastActivity = *latestEventAt
	}
	if w.cfg.IdleAfter > 0 && now.Sub(lastActivity) < w.cfg.IdleAfter {
		return false
	}

	return true
}

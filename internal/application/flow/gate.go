package flow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// GateResult represents the outcome of a gate evaluation.
type GateResult struct {
	Passed  bool
	Reason  string
	ResetTo []int64 // Step IDs to reset on reject (upstream rework)
	// Metadata is copied from the gate step's Artifact metadata when available.
	// It may include fields like pr_number, pr_url, reject_targets, etc.
	Metadata map[string]any
}

// ProcessGate handles a gate Step: pass → downstream continue, reject → reset upstream + gate re-enters loop.
func (e *IssueEngine) ProcessGate(ctx context.Context, step *core.Step, result GateResult) error {
	if step.Type != core.StepGate {
		return fmt.Errorf("step %d is not a gate (type=%s)", step.ID, step.Type)
	}

	if result.Passed {
		if err := e.transitionStep(ctx, step, core.StepDone); err != nil {
			return err
		}
		e.bus.Publish(ctx, core.Event{
			Type:      core.EventGatePassed,
			IssueID:   step.IssueID,
			StepID:    step.ID,
			Timestamp: time.Now().UTC(),
			Data:      map[string]any{"reason": result.Reason},
		})
		return nil
	}

	// Gate rejected — check rework round limit before cycling.
	maxReworkRounds := 3 // default
	if step.Config != nil {
		if v, ok := step.Config["max_rework_rounds"].(float64); ok && v > 0 {
			maxReworkRounds = int(v)
		}
	}

	// Read rework_count from signal count (single source of truth).
	reworkCount := 0
	if cnt, err := e.store.CountStepSignals(ctx, step.ID, core.SignalReject); err == nil {
		reworkCount = cnt
	}

	if reworkCount >= maxReworkRounds {
		// Rework limit reached — caller will transition to blocked.
		e.bus.Publish(ctx, core.Event{
			Type:      core.EventGateReworkLimitReached,
			IssueID:   step.IssueID,
			StepID:    step.ID,
			Timestamp: time.Now().UTC(),
			Data: map[string]any{
				"reason":            result.Reason,
				"rework_count":      reworkCount,
				"max_rework_rounds": maxReworkRounds,
			},
		})
		return core.ErrMaxRetriesExceeded
	}

	// Record a SignalReject on the gate step — single source of truth for rework_count.
	if _, err := e.store.CreateStepSignal(ctx, &core.StepSignal{
		StepID:    step.ID,
		IssueID:   step.IssueID,
		Type:      core.SignalReject,
		Source:    core.SignalSourceSystem,
		Summary:   strings.TrimSpace(result.Reason),
		Payload:   result.Metadata,
		Actor:     "gate",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		slog.Error("failed to record gate reject signal", "step_id", step.ID, "error", err)
	}

	e.bus.Publish(ctx, core.Event{
		Type:      core.EventGateRejected,
		IssueID:   step.IssueID,
		StepID:    step.ID,
		Timestamp: time.Now().UTC(),
		Data:      map[string]any{"reason": result.Reason, "rework_round": reworkCount + 1},
	})

	// Reset upstream steps for rework — persist retry_count via UpdateStep.
	for _, upID := range result.ResetTo {
		up, err := e.store.GetStep(ctx, upID)
		if err != nil {
			return fmt.Errorf("get upstream step %d: %w", upID, err)
		}
		if up.MaxRetries > 0 && up.RetryCount >= up.MaxRetries {
			return core.ErrMaxRetriesExceeded
		}
		e.recordGateRework(ctx, up, step.ID, result.Reason, result.Metadata)
		up.RetryCount++
		up.Status = core.StepPending
		if err := e.store.UpdateStep(ctx, up); err != nil {
			return fmt.Errorf("reset step %d: %w", upID, err)
		}
	}

	// Gate itself → pending (will be re-promoted after upstream completes).
	return e.transitionStep(ctx, step, core.StepPending)
}

// processGateReject delegates to ProcessGate and handles ErrMaxRetriesExceeded
// by transitioning the step to blocked (instead of propagating the error).
func (e *IssueEngine) processGateReject(ctx context.Context, step *core.Step, result GateResult) error {
	rejectErr := e.ProcessGate(ctx, step, result)
	if rejectErr == core.ErrMaxRetriesExceeded {
		_ = e.transitionStep(ctx, step, core.StepBlocked)
		return nil
	}
	return rejectErr
}

// GateVerdict represents the outcome of a single gate evaluator.
type GateVerdict struct {
	Decided  bool              // true if this evaluator made a decision
	Passed   bool              // pass or reject (only meaningful when Decided)
	Reason   string            // human-readable reason
	ResetTo  []int64           // step IDs to reset on reject
	Metadata map[string]any    // source context (art.Metadata, signal.Payload); used for merge failure recovery
	Signal   *core.StepSignal  // non-nil for signal-driven verdicts (carries Source for event data)
}

// GateEvaluator evaluates a gate step and optionally returns a verdict.
// If Decided is false, the next evaluator in the chain is tried.
type GateEvaluator func(ctx context.Context, step *core.Step) (GateVerdict, error)

// finalizeGate is called after a gate step's executor succeeds.
// It runs the evaluator chain in order; the first evaluator that returns Decided=true wins.
// Default chain: StepSignal (MCP/HTTP) → Manifest check → Artifact metadata.
func (e *IssueEngine) finalizeGate(ctx context.Context, step *core.Step) error {
	evaluators := e.gateEvaluators
	if len(evaluators) == 0 {
		evaluators = []GateEvaluator{
			e.evalSignalVerdict,
			e.evalManifestCheck,
			e.evalArtifactMetadata,
		}
	}
	for _, eval := range evaluators {
		v, err := eval(ctx, step)
		if err != nil {
			return err
		}
		if v.Decided {
			return e.applyGateVerdict(ctx, step, v)
		}
	}
	// No evaluator decided — default pass.
	return e.applyGatePass(ctx, step, GateVerdict{})
}

// applyGateVerdict dispatches a decided verdict to the pass or reject path.
func (e *IssueEngine) applyGateVerdict(ctx context.Context, step *core.Step, v GateVerdict) error {
	if v.Passed {
		return e.applyGatePass(ctx, step, v)
	}
	return e.processGateReject(ctx, step, GateResult{
		Passed:   false,
		Reason:   v.Reason,
		ResetTo:  v.ResetTo,
		Metadata: v.Metadata,
	})
}

// applyGatePass handles gate pass: merge PR (if configured), emit event, transition done.
func (e *IssueEngine) applyGatePass(ctx context.Context, step *core.Step, v GateVerdict) error {
	if err := e.mergePRIfConfigured(ctx, step); err != nil {
		if e.handleMergeConflictBlock(ctx, step, err) {
			return nil
		}
		mergeReason, mergeMetadata := e.formatMergeFailureFeedback(step, err)
		resetTo, _ := e.defaultGateResetTargets(ctx, step, v.Metadata)
		return e.processGateReject(ctx, step, GateResult{
			Passed:   false,
			Reason:   mergeReason,
			ResetTo:  resetTo,
			Metadata: mergeMetadata,
		})
	}

	// Build event data — signal-driven verdicts carry extra fields.
	var data map[string]any
	if v.Signal != nil {
		data = map[string]any{"signal_source": string(v.Signal.Source)}
		if v.Reason != "" {
			data["reason"] = v.Reason
		}
	}
	e.bus.Publish(ctx, core.Event{
		Type:      core.EventGatePassed,
		IssueID:   step.IssueID,
		StepID:    step.ID,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
	return e.transitionStep(ctx, step, core.StepDone)
}

// --- Gate Evaluators ---

// evalSignalVerdict checks for an explicit StepSignal (MCP tool call or human HTTP API).
// System-sourced signals are skipped — those are internal bookkeeping.
func (e *IssueEngine) evalSignalVerdict(ctx context.Context, step *core.Step) (GateVerdict, error) {
	signal, _ := e.store.GetLatestStepSignal(ctx, step.ID, core.SignalApprove, core.SignalReject)
	if signal == nil || signal.Source == core.SignalSourceSystem {
		return GateVerdict{}, nil
	}

	if signal.Type == core.SignalApprove {
		reason, _ := signal.Payload["reason"].(string)
		return GateVerdict{
			Decided:  true,
			Passed:   true,
			Reason:   reason,
			Metadata: signal.Payload,
			Signal:   signal,
		}, nil
	}

	// SignalReject
	reason, _ := signal.Payload["reason"].(string)
	if strings.TrimSpace(reason) == "" {
		reason = "gate rejected"
	}
	resetTo := e.immediatePredecessorIDs(ctx, step)
	resetTo = extractResetTargets(signal.Payload, resetTo)
	return GateVerdict{
		Decided:  true,
		Passed:   false,
		Reason:   reason,
		ResetTo:  resetTo,
		Metadata: signal.Payload,
		Signal:   signal,
	}, nil
}

// evalManifestCheck evaluates the feature manifest if manifest_check is enabled.
func (e *IssueEngine) evalManifestCheck(ctx context.Context, step *core.Step) (GateVerdict, error) {
	if !manifestCheckEnabled(step) {
		return GateVerdict{}, nil
	}
	passed, reason, err := e.checkManifestEntries(ctx, step)
	if err != nil {
		return GateVerdict{}, fmt.Errorf("manifest check: %w", err)
	}
	if passed {
		return GateVerdict{}, nil // manifest passed — continue to next evaluator
	}
	return GateVerdict{
		Decided: true,
		Passed:  false,
		Reason:  reason,
		ResetTo: e.immediatePredecessorIDs(ctx, step),
	}, nil
}

// evalArtifactMetadata checks the gate step's artifact for a verdict field.
func (e *IssueEngine) evalArtifactMetadata(ctx context.Context, step *core.Step) (GateVerdict, error) {
	art, err := e.store.GetLatestArtifactByStep(ctx, step.ID)
	if err == core.ErrNotFound {
		return GateVerdict{}, nil // no artifact → continue to default pass
	}
	if err != nil {
		return GateVerdict{}, fmt.Errorf("get gate artifact for step %d: %w", step.ID, err)
	}

	verdict, _ := art.Metadata["verdict"].(string)
	if verdict != "reject" {
		// "pass" or unrecognized → pass
		return GateVerdict{Decided: true, Passed: true, Metadata: art.Metadata}, nil
	}

	resetTo, reason := e.defaultGateResetTargets(ctx, step, art.Metadata)
	return GateVerdict{
		Decided:  true,
		Passed:   false,
		Reason:   reason,
		ResetTo:  resetTo,
		Metadata: art.Metadata,
	}, nil
}

// extractResetTargets reads reject_targets from metadata, falling back to predecessors.
func extractResetTargets(metadata map[string]any, fallback []int64) []int64 {
	targets, ok := metadata["reject_targets"].([]any)
	if !ok || len(targets) == 0 {
		return fallback
	}
	var result []int64
	for _, t := range targets {
		if id, ok := toInt64(t); ok {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}


// recordGateRework creates a SignalFeedback on the upstream step,
// recording the gate rejection as a structured signal.
func (e *IssueEngine) recordGateRework(ctx context.Context, upstreamStep *core.Step, gateStepID int64, reason string, metadata map[string]any) {
	summary := strings.TrimSpace(reason)
	if summary == "" {
		summary = "gate rejected"
	}

	// Build formatted content.
	var content strings.Builder
	content.WriteString("Reason: ")
	content.WriteString(summary)
	if metadata != nil {
		if prURL, ok := metadata["pr_url"].(string); ok && strings.TrimSpace(prURL) != "" {
			content.WriteString("\nPR: ")
			content.WriteString(strings.TrimSpace(prURL))
		}
		if n, ok := metadata["pr_number"]; ok {
			content.WriteString("\nPR Number: ")
			content.WriteString(fmt.Sprint(n))
		}
		if hint, ok := metadata["merge_action_hint"].(string); ok && strings.TrimSpace(hint) != "" {
			content.WriteString("\nHint: ")
			content.WriteString(strings.TrimSpace(hint))
		}
	}

	sig := &core.StepSignal{
		StepID:       upstreamStep.ID,
		IssueID:      upstreamStep.IssueID,
		Type:         core.SignalFeedback,
		Source:       core.SignalSourceSystem,
		Summary:      summary,
		Content:      content.String(),
		SourceStepID: gateStepID,
		Payload:      metadata,
		Actor:        "gate",
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := e.store.CreateStepSignal(ctx, sig); err != nil {
		slog.Error("failed to record gate rework signal", "step_id", upstreamStep.ID, "error", err)
	}
}

func (e *IssueEngine) defaultGateResetTargets(ctx context.Context, step *core.Step, metadata map[string]any) (resetTo []int64, reason string) {
	// By default only reset the closest upstream position.
	// Full upstream closure is opt-in via reset_upstream_closure.
	immediatePredecessors := e.immediatePredecessorIDs(ctx, step)
	resetTo = extractResetTargets(metadata, immediatePredecessors)
	if len(resetTo) == 0 {
		resetTo = append([]int64(nil), immediatePredecessors...)
	}
	if step.Config != nil {
		if v, ok := step.Config["reset_upstream_closure"].(bool); ok && v {
			resetTo = e.predecessorIDs(ctx, step)
		}
	}
	reason, _ = metadata["reason"].(string)
	if strings.TrimSpace(reason) == "" {
		reason = "gate rejected"
	}
	return resetTo, reason
}

// predecessorIDs returns IDs of all steps with lower Position in the same issue.
func (e *IssueEngine) predecessorIDs(ctx context.Context, step *core.Step) []int64 {
	steps, err := e.store.ListStepsByIssue(ctx, step.IssueID)
	if err != nil || len(steps) == 0 {
		return nil
	}
	return predecessorStepIDs(steps, step)
}

func (e *IssueEngine) immediatePredecessorIDs(ctx context.Context, step *core.Step) []int64 {
	steps, err := e.store.ListStepsByIssue(ctx, step.IssueID)
	if err != nil || len(steps) == 0 {
		return nil
	}
	return immediatePredecessorStepIDs(steps, step)
}

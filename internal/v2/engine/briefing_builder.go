package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// DefaultBriefingBuilder assembles a Briefing by reading upstream Artifacts
// and step configuration.
type DefaultBriefingBuilder struct {
	store core.Store
}

// NewBriefingBuilder creates a BriefingBuilder backed by the given store.
func NewBriefingBuilder(store core.Store) *DefaultBriefingBuilder {
	return &DefaultBriefingBuilder{store: store}
}

// Build constructs a Briefing for the given step.
func (b *DefaultBriefingBuilder) Build(ctx context.Context, step *core.Step) (*core.Briefing, error) {
	briefing := &core.Briefing{
		StepID:      step.ID,
		Objective:   buildObjective(step),
		Constraints: step.AcceptanceCriteria,
	}

	// Collect upstream artifact references.
	//
	// For gate steps, include transitive upstream artifacts when configured so the reviewer can
	// see the implementer's output (e.g. test commands/results) even when the gate only depends
	// on later automation steps like "open_pr".
	depIDs := append([]int64(nil), step.DependsOn...)
	if step.Type == core.StepGate && step.Config != nil {
		if v, ok := step.Config["reset_upstream_closure"].(bool); ok && v {
			closure, err := upstreamClosure(ctx, b.store, step.FlowID, step.ID)
			if err == nil && len(closure) > 0 {
				depIDs = closure
			}
		}
	}

	for _, depID := range depIDs {
		art, err := b.store.GetLatestArtifactByStep(ctx, depID)
		if err == core.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get upstream artifact for step %d: %w", depID, err)
		}
		briefing.ContextRefs = append(briefing.ContextRefs, core.ContextRef{
			Type:   core.CtxUpstreamArtifact,
			RefID:  art.ID,
			Label:  fmt.Sprintf("upstream step %d output", depID),
			Inline: art.ResultMarkdown,
		})
	}

	return briefing, nil
}

func upstreamClosure(ctx context.Context, store core.Store, flowID int64, stepID int64) ([]int64, error) {
	steps, err := store.ListStepsByFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	depsByID := make(map[int64][]int64, len(steps))
	for _, s := range steps {
		if s == nil {
			continue
		}
		depsByID[s.ID] = append([]int64(nil), s.DependsOn...)
	}
	seen := map[int64]struct{}{}
	var stack []int64
	stack = append(stack, depsByID[stepID]...)
	var result []int64
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		result = append(result, n)
		stack = append(stack, depsByID[n]...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

// buildObjective derives a brief objective string from step config or name.
func buildObjective(step *core.Step) string {
	if step.Config != nil {
		if obj, ok := step.Config["objective"].(string); ok && obj != "" {
			return obj
		}
	}
	return fmt.Sprintf("Execute step: %s", step.Name)
}

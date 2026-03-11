package flow

import "github.com/yoke233/ai-workflow/internal/core"

// ValidateDAG checks that the Steps form a valid DAG (no cycles).
// Returns core.ErrCycleDetected if a cycle is found.
func ValidateDAG(steps []*core.Step) error {
	// Build adjacency: step ID → downstream step IDs
	// Also track in-degree for topological sort.
	idSet := make(map[int64]struct{}, len(steps))
	inDegree := make(map[int64]int, len(steps))
	downstream := make(map[int64][]int64, len(steps))

	for _, s := range steps {
		idSet[s.ID] = struct{}{}
		inDegree[s.ID] = 0
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			downstream[dep] = append(downstream[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	// Kahn's algorithm
	var queue []int64
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range downstream[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(steps) {
		return core.ErrCycleDetected
	}
	return nil
}

// EntrySteps returns steps that have no upstream dependencies (DependsOn is empty).
func EntrySteps(steps []*core.Step) []*core.Step {
	var entries []*core.Step
	for _, s := range steps {
		if len(s.DependsOn) == 0 {
			entries = append(entries, s)
		}
	}
	return entries
}

// PromotableSteps returns steps that are pending and whose upstream dependencies are all done.
// These should be promoted to "ready" status by the engine.
func PromotableSteps(steps []*core.Step) []*core.Step {
	doneSet := make(map[int64]bool, len(steps))
	for _, s := range steps {
		if s.Status == core.StepDone {
			doneSet[s.ID] = true
		}
	}

	var promotable []*core.Step
	for _, s := range steps {
		if s.Status != core.StepPending {
			continue
		}
		allDone := true
		for _, dep := range s.DependsOn {
			if !doneSet[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			promotable = append(promotable, s)
		}
	}
	return promotable
}

// RunnableSteps returns steps that have status "ready" and can be dispatched for execution.
func RunnableSteps(steps []*core.Step) []*core.Step {
	var runnable []*core.Step
	for _, s := range steps {
		if s.Status == core.StepReady {
			runnable = append(runnable, s)
		}
	}
	return runnable
}


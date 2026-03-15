package flow

import (
	"fmt"
	"math"
	"sort"

	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/dagutil"
)

// hasDependsOn returns true if any action in the set has a non-empty DependsOn list.
// When true, the entire WorkItem uses DAG-mode scheduling; otherwise Position-mode.
func hasDependsOn(actions []*core.Action) bool {
	for _, a := range actions {
		if len(a.DependsOn) > 0 {
			return true
		}
	}
	return false
}

// detectCycle checks for cycles in the DependsOn graph using DFS three-color marking.
// Returns an error describing the cycle if one is found.
func detectCycle(actions []*core.Action) error {
	// Build adjacency: id → list of dependency IDs (backward edges).
	adj := make(map[int64][]int64, len(actions))
	for _, a := range actions {
		adj[a.ID] = a.DependsOn
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in current DFS path
		black = 2 // fully processed
	)
	color := make(map[int64]int, len(actions))

	var dfs func(id int64) error
	dfs = func(id int64) error {
		color[id] = gray
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("cycle detected: action %d → %d", id, dep)
			case white:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for _, a := range actions {
		if color[a.ID] == white {
			if err := dfs(a.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateActions checks that actions have valid ordering.
// Position-mode: positions must be non-negative and unique.
// DAG-mode: DependsOn IDs must exist and the graph must be acyclic; Position uniqueness is not required.
func ValidateActions(actions []*core.Action) error {
	for _, action := range actions {
		if action == nil {
			return fmt.Errorf("action is nil")
		}
	}

	if hasDependsOn(actions) {
		// DAG mode: validate DependsOn references and acyclicity.
		idSet := make(map[int64]bool, len(actions))
		for _, a := range actions {
			idSet[a.ID] = true
		}
		for _, a := range actions {
			for _, dep := range a.DependsOn {
				if !idSet[dep] {
					return fmt.Errorf("action %d depends on non-existent action %d", a.ID, dep)
				}
				if dep == a.ID {
					return fmt.Errorf("action %d depends on itself", a.ID)
				}
			}
		}
		if err := detectCycle(actions); err != nil {
			return err
		}
		return nil
	}

	// Position mode: non-negative + unique.
	seen := make(map[int]struct{}, len(actions))
	for _, action := range actions {
		if action.Position < 0 {
			return fmt.Errorf("action %d has negative position %d", action.ID, action.Position)
		}
		if _, ok := seen[action.Position]; ok {
			return fmt.Errorf("duplicate action position %d", action.Position)
		}
		seen[action.Position] = struct{}{}
	}
	return nil
}

// EntryActions returns actions that should run first.
// DAG-mode: actions with empty DependsOn.
// Position-mode: actions with the lowest Position.
func EntryActions(actions []*core.Action) []*core.Action {
	if len(actions) == 0 {
		return nil
	}

	if hasDependsOn(actions) {
		var entries []*core.Action
		for _, a := range actions {
			if len(a.DependsOn) == 0 {
				entries = append(entries, a)
			}
		}
		return entries
	}

	// Position mode: smallest position.
	sorted := make([]*core.Action, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	minPos := sorted[0].Position
	var entries []*core.Action
	for _, a := range sorted {
		if a.Position == minPos {
			entries = append(entries, a)
		}
	}
	return entries
}

// PromotableActions returns actions that are pending and whose predecessors are all done.
// DAG-mode: all DependsOn actions are done.
// Position-mode: all actions with lower Position are done.
func PromotableActions(actions []*core.Action) []*core.Action {
	doneSet := make(map[int64]bool, len(actions))
	for _, a := range actions {
		if a.Status == core.ActionDone {
			doneSet[a.ID] = true
		}
	}

	dagMode := hasDependsOn(actions)

	var promotable []*core.Action
	for _, a := range actions {
		if a.Status != core.ActionPending {
			continue
		}

		if dagMode {
			if dagutil.AllDepsResolved(a.DependsOn, doneSet) {
				promotable = append(promotable, a)
			}
		} else {
			allPriorDone := true
			for _, other := range actions {
				if other.Position < a.Position && !doneSet[other.ID] {
					allPriorDone = false
					break
				}
			}
			if allPriorDone {
				promotable = append(promotable, a)
			}
		}
	}
	return promotable
}

// RunnableActions returns actions that have status "ready" and can be dispatched for execution.
func RunnableActions(actions []*core.Action) []*core.Action {
	var runnable []*core.Action
	for _, a := range actions {
		if a.Status == core.ActionReady {
			runnable = append(runnable, a)
		}
	}
	return runnable
}

// predecessorActionIDs returns the IDs of all transitive predecessors of the given action.
// DAG-mode: BFS traversal of DependsOn (transitive closure).
// Position-mode: all actions with Position strictly less than the given action.
func predecessorActionIDs(actions []*core.Action, action *core.Action) []int64 {
	if hasDependsOn(actions) {
		// DAG mode: BFS transitive closure over DependsOn.
		byID := make(map[int64]*core.Action, len(actions))
		for _, a := range actions {
			byID[a.ID] = a
		}
		visited := make(map[int64]bool)
		queue := make([]int64, len(action.DependsOn))
		copy(queue, action.DependsOn)
		for _, id := range action.DependsOn {
			visited[id] = true
		}
		var ids []int64
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			ids = append(ids, cur)
			if dep, ok := byID[cur]; ok {
				for _, pid := range dep.DependsOn {
					if !visited[pid] {
						visited[pid] = true
						queue = append(queue, pid)
					}
				}
			}
		}
		return ids
	}

	// Position mode.
	var ids []int64
	for _, a := range actions {
		if a.Position < action.Position {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// immediatePredecessorActionIDs returns the IDs of direct predecessors.
// DAG-mode: returns action.DependsOn directly.
// Position-mode: actions at the closest lower Position.
func immediatePredecessorActionIDs(actions []*core.Action, action *core.Action) []int64 {
	if hasDependsOn(actions) {
		// DAG mode: direct dependencies.
		if len(action.DependsOn) == 0 {
			return nil
		}
		result := make([]int64, len(action.DependsOn))
		copy(result, action.DependsOn)
		return result
	}

	// Position mode: closest lower position.
	closest := math.MinInt
	for _, a := range actions {
		if a.Position < action.Position && a.Position > closest {
			closest = a.Position
		}
	}
	if closest == math.MinInt {
		return nil
	}

	var ids []int64
	for _, a := range actions {
		if a.Position == closest {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

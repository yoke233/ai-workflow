package executor

import (
	"context"
	"fmt"
	"time"

	flowapp "github.com/yoke233/zhanggui/internal/application/flow"
	"github.com/yoke233/zhanggui/internal/core"
)

// NewMockActionExecutor returns a ActionExecutor that does not spawn ACP agents.
// It stores a small markdown artifact and publishes a single "done" agent_output event.
//
// Intended for local smoke tests and CI where external agent credentials are unavailable.
func NewMockActionExecutor(store core.Store, bus core.EventBus) flowapp.ActionExecutor {
	return func(ctx context.Context, action *core.Action, run *core.Run) error {
		workDir := ""
		if ws := flowapp.WorkspaceFromContext(ctx); ws != nil {
			workDir = ws.Path
		}

		now := time.Now().UTC()
		reply := fmt.Sprintf(
			"## Mock executor\n\n- action_id: %d\n- work_item_id: %d\n- action_type: %s\n- agent_role: %s\n- work_dir: %s\n- time_utc: %s\n",
			action.ID, action.WorkItemID, action.Type, action.AgentRole, workDir, now.Format(time.RFC3339),
		)

		// Publish done event with full reply (matches ACP bridge "done" shape).
		if bus != nil {
			bus.Publish(ctx, core.Event{
				Type:       core.EventRunAgentOutput,
				WorkItemID: action.WorkItemID,
				ActionID:   action.ID,
				RunID:      run.ID,
				Timestamp:  now,
				Data: map[string]any{
					"type":    "done",
					"content": reply,
				},
			})
		}

		// Store result inline on the Run.
		run.ResultMarkdown = reply

		run.Output = map[string]any{
			"text":        reply,
			"stop_reason": "mock",
		}

		return nil
	}
}

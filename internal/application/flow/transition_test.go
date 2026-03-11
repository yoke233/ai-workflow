package flow

import (
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestValidFlowTransitions(t *testing.T) {
	cases := []struct {
		from, to core.FlowStatus
		valid    bool
	}{
		{core.FlowPending, core.FlowRunning, true},
		{core.FlowPending, core.FlowCancelled, true},
		{core.FlowPending, core.FlowDone, false},
		{core.FlowRunning, core.FlowDone, true},
		{core.FlowRunning, core.FlowFailed, true},
		{core.FlowRunning, core.FlowPending, false},
		{core.FlowDone, core.FlowRunning, false},
	}
	for _, tc := range cases {
		got := ValidFlowTransition(tc.from, tc.to)
		if got != tc.valid {
			t.Errorf("Flow %s → %s: expected %v, got %v", tc.from, tc.to, tc.valid, got)
		}
	}
}

func TestValidStepTransitions(t *testing.T) {
	cases := []struct {
		from, to core.StepStatus
		valid    bool
	}{
		{core.StepPending, core.StepReady, true},
		{core.StepReady, core.StepRunning, true},
		{core.StepRunning, core.StepDone, true},
		{core.StepRunning, core.StepFailed, true},
		{core.StepDone, core.StepRunning, false},
		{core.StepFailed, core.StepPending, true}, // retry
		{core.StepBlocked, core.StepReady, true},
	}
	for _, tc := range cases {
		got := ValidStepTransition(tc.from, tc.to)
		if got != tc.valid {
			t.Errorf("Step %s → %s: expected %v, got %v", tc.from, tc.to, tc.valid, got)
		}
	}
}

func TestValidExecTransitions(t *testing.T) {
	cases := []struct {
		from, to core.ExecutionStatus
		valid    bool
	}{
		{core.ExecCreated, core.ExecRunning, true},
		{core.ExecRunning, core.ExecSucceeded, true},
		{core.ExecRunning, core.ExecFailed, true},
		{core.ExecSucceeded, core.ExecRunning, false},
	}
	for _, tc := range cases {
		got := ValidExecTransition(tc.from, tc.to)
		if got != tc.valid {
			t.Errorf("Exec %s → %s: expected %v, got %v", tc.from, tc.to, tc.valid, got)
		}
	}
}


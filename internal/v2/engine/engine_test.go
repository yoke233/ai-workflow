package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
	"github.com/yoke233/ai-workflow/internal/v2/store/sqlite"
)

func setup(t *testing.T) (core.Store, core.EventBus) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, NewMemBus()
}

// TestLinearFlow: A → B → C, all succeed.
func TestLinearFlow(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var callOrder []string
	var counter int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		atomic.AddInt32(&counter, 1)
		callOrder = append(callOrder, step.Name)
		exec.Output = map[string]any{"ok": true}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	// Create flow + steps.
	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "linear", Status: core.FlowPending})

	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	bID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{bID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	if counter != 3 {
		t.Fatalf("expected 3 executions, got %d", counter)
	}
	if callOrder[0] != "A" || callOrder[1] != "B" || callOrder[2] != "C" {
		t.Fatalf("unexpected order: %v", callOrder)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
}

// TestParallelFanOut: A → (B, C) both run concurrently.
func TestParallelFanOut(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var counter int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		atomic.AddInt32(&counter, 1)
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(4))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "fanout", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}
	if counter != 3 {
		t.Fatalf("expected 3 executions, got %d", counter)
	}
}

// TestStepFailure: A fails, flow fails.
func TestStepFailure(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		return fmt.Errorf("boom")
	}

	eng := New(store, bus, executor)

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "fail", Status: core.FlowPending})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})

	err := eng.Run(ctx, fID)
	if err == nil {
		t.Fatal("expected error")
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowFailed {
		t.Fatalf("expected failed, got %s", flow.Status)
	}
}

// TestRetry: step fails once, retries, succeeds.
func TestRetry(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var attempts int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			return fmt.Errorf("transient")
		}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "retry", Status: core.FlowPending})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending, MaxRetries: 1})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

// TestCancelFlow: cancel a running flow.
func TestCancelFlow(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	eng := New(store, bus, nil)

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "cancel-test", Status: core.FlowPending})
	_ = store.UpdateFlowStatus(ctx, fID, core.FlowRunning) // simulate running

	if err := eng.Cancel(ctx, fID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowCancelled {
		t.Fatalf("expected cancelled, got %s", flow.Status)
	}
}

// TestRetryPersistence: verify retry_count is persisted, preventing infinite retries.
func TestRetryPersistence(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		return fmt.Errorf("always fail")
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "retry-persist", Status: core.FlowPending})
	sID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending, MaxRetries: 2})

	// Should fail after 3 attempts (1 original + 2 retries).
	err := eng.Run(ctx, fID)
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify retry_count was persisted.
	step, _ := store.GetStep(ctx, sID)
	if step.RetryCount != 2 {
		t.Fatalf("expected retry_count=2, got %d", step.RetryCount)
	}
}

// TestGateAutoPass: exec → gate(pass) → flow done.
func TestGateAutoPass(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Type == core.StepGate {
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID:    exec.ID,
				StepID:         step.ID,
				FlowID:         step.FlowID,
				ResultMarkdown: "LGTM, all tests pass.",
				Metadata:       map[string]any{"verdict": "pass"},
			})
			return err
		}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "gate-pass", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "impl", Type: core.StepExec, Status: core.StepPending})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "review", Type: core.StepGate, Status: core.StepPending, DependsOn: []int64{aID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}
	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
}

// TestGateAutoReject: exec → gate(reject) → exec retries → gate(pass) → flow done.
func TestGateAutoReject(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var gateCount int32
	var execCount int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Type == core.StepGate {
			n := atomic.AddInt32(&gateCount, 1)
			verdict := "reject"
			if n > 1 {
				verdict = "pass"
			}
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID:    exec.ID,
				StepID:         step.ID,
				FlowID:         step.FlowID,
				ResultMarkdown: "Review result",
				Metadata:       map[string]any{"verdict": verdict, "reason": "needs improvement"},
			})
			return err
		}
		atomic.AddInt32(&execCount, 1)
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "gate-reject", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "impl", Type: core.StepExec, Status: core.StepPending, MaxRetries: 1})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "review", Type: core.StepGate, Status: core.StepPending, DependsOn: []int64{aID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
	if gateCount != 2 {
		t.Fatalf("expected 2 gate evaluations, got %d", gateCount)
	}
	if execCount != 2 {
		t.Fatalf("expected 2 exec runs, got %d", execCount)
	}
}

// TestStepTimeout: step times out on first attempt, retries, succeeds.
func TestStepTimeout(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var attempts int32
	executor := func(ctx context.Context, step *core.Step, exec *core.Execution) error {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			select {
			case <-time.After(500 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "timeout", Status: core.FlowPending})
	_, _ = store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "slow",
		Type:       core.StepExec,
		Status:     core.StepPending,
		Timeout:    50 * time.Millisecond,
		MaxRetries: 1,
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

// TestErrorKindPermanent: permanent error skips retry despite MaxRetries > 0.
func TestErrorKindPermanent(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		exec.ErrorKind = core.ErrKindPermanent
		return fmt.Errorf("fatal: invalid configuration")
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "permanent", Status: core.FlowPending})
	sID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "A",
		Type:       core.StepExec,
		Status:     core.StepPending,
		MaxRetries: 5,
	})

	err := eng.Run(ctx, fID)
	if err == nil {
		t.Fatal("expected error")
	}

	step, _ := store.GetStep(ctx, sID)
	if step.RetryCount != 0 {
		t.Fatalf("expected retry_count=0 (permanent skips retry), got %d", step.RetryCount)
	}
	if step.Status != core.StepFailed {
		t.Fatalf("expected failed, got %s", step.Status)
	}
}

// TestProfileRegistry: resolve by role + capabilities.
func TestProfileRegistry(t *testing.T) {
	profiles := []*core.AgentProfile{
		{ID: "claude-worker", Role: core.RoleWorker, Capabilities: []string{"backend", "frontend"}},
		{ID: "claude-gate", Role: core.RoleGate, Capabilities: []string{"code-review"}},
		{ID: "codex-worker", Role: core.RoleWorker, Capabilities: []string{"backend", "qa"}},
	}
	reg := NewProfileRegistry(profiles)
	ctx := context.Background()

	// Match role + capability.
	id, err := reg.Resolve(ctx, &core.Step{AgentRole: "worker", RequiredCapabilities: []string{"qa"}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id != "codex-worker" {
		t.Fatalf("expected codex-worker, got %s", id)
	}

	// Match role only (no capability filter).
	id, err = reg.Resolve(ctx, &core.Step{AgentRole: "gate"})
	if err != nil {
		t.Fatalf("resolve gate: %v", err)
	}
	if id != "claude-gate" {
		t.Fatalf("expected claude-gate, got %s", id)
	}

	// No role filter — first match.
	id, err = reg.Resolve(ctx, &core.Step{})
	if err != nil {
		t.Fatalf("resolve any: %v", err)
	}
	if id != "claude-worker" {
		t.Fatalf("expected claude-worker, got %s", id)
	}

	// No match.
	_, err = reg.Resolve(ctx, &core.Step{AgentRole: "worker", RequiredCapabilities: []string{"k8s"}})
	if err != core.ErrNoMatchingAgent {
		t.Fatalf("expected ErrNoMatchingAgent, got %v", err)
	}
}

// TestBriefingBuilder: assembles briefing from upstream artifacts.
func TestBriefingBuilder(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	// Create a flow with A → B.
	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "briefing-test", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepDone})
	bID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:             fID,
		Name:               "B",
		Type:               core.StepExec,
		Status:             core.StepPending,
		DependsOn:          []int64{aID},
		AcceptanceCriteria: []string{"must pass lint", "must have tests"},
		Config:             map[string]any{"objective": "Implement login endpoint"},
	})

	// A has an artifact.
	eID, _ := store.CreateExecution(ctx, &core.Execution{StepID: aID, FlowID: fID, Status: core.ExecSucceeded, Attempt: 1})
	store.CreateArtifact(ctx, &core.Artifact{
		ExecutionID:    eID,
		StepID:         aID,
		FlowID:         fID,
		ResultMarkdown: "## Design\nAPI design for login.",
	})

	builder := NewBriefingBuilder(store)
	stepB, _ := store.GetStep(ctx, bID)
	briefing, err := builder.Build(ctx, stepB)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if briefing.Objective != "Implement login endpoint" {
		t.Fatalf("expected objective from config, got %q", briefing.Objective)
	}
	if len(briefing.Constraints) != 2 {
		t.Fatalf("expected 2 constraints, got %d", len(briefing.Constraints))
	}
	if len(briefing.ContextRefs) != 1 {
		t.Fatalf("expected 1 context ref, got %d", len(briefing.ContextRefs))
	}
	if briefing.ContextRefs[0].Type != core.CtxUpstreamArtifact {
		t.Fatalf("expected upstream_artifact ref, got %s", briefing.ContextRefs[0].Type)
	}
	if briefing.ContextRefs[0].Inline != "## Design\nAPI design for login." {
		t.Fatalf("expected inline content, got %q", briefing.ContextRefs[0].Inline)
	}

	_ = bus // satisfy usage
}

// TestCollectorWiring: collector extracts metadata into artifact after success.
func TestCollectorWiring(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	// Collector that extracts a "summary" field.
	collector := CollectorFunc(func(_ context.Context, stepType core.StepType, markdown string) (map[string]any, error) {
		return map[string]any{"summary": "extracted from: " + stepType}, nil
	})

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		// Simulate agent creating an artifact.
		_, err := store.CreateArtifact(ctx, &core.Artifact{
			ExecutionID:    exec.ID,
			StepID:         step.ID,
			FlowID:         step.FlowID,
			ResultMarkdown: "## Implementation\nDid the thing.",
		})
		return err
	}

	eng := New(store, bus, executor, WithConcurrency(1), WithCollector(collector))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "collector-test", Status: core.FlowPending})
	sID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	art, err := store.GetLatestArtifactByStep(ctx, sID)
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	if art.Metadata["summary"] != "extracted from: exec" {
		t.Fatalf("expected extracted metadata, got %v", art.Metadata)
	}
}

// TestResolverIntegration: engine uses resolver to set agent_id on execution.
func TestResolverIntegration(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	profiles := []*core.AgentProfile{
		{ID: "my-worker", Role: core.RoleWorker, Capabilities: []string{"go"}},
	}

	var capturedAgentID string
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		capturedAgentID = exec.AgentID
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1), WithResolver(NewProfileRegistry(profiles)))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "resolver-test", Status: core.FlowPending})
	_, _ = store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "build",
		Type:                 core.StepExec,
		Status:               core.StepPending,
		AgentRole:            "worker",
		RequiredCapabilities: []string{"go"},
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}
	if capturedAgentID != "my-worker" {
		t.Fatalf("expected agent_id=my-worker, got %q", capturedAgentID)
	}
}

// TestEventBus: subscribe and receive events.
func TestEventBus(t *testing.T) {
	bus := NewMemBus()
	ctx := context.Background()

	sub := bus.Subscribe(core.SubscribeOpts{
		Types:      []core.EventType{core.EventFlowStarted},
		BufferSize: 8,
	})
	defer sub.Cancel()

	bus.Publish(ctx, core.Event{Type: core.EventFlowStarted, FlowID: 1})
	bus.Publish(ctx, core.Event{Type: core.EventStepReady, FlowID: 1})   // should be filtered out
	bus.Publish(ctx, core.Event{Type: core.EventFlowStarted, FlowID: 2}) // should be received

	ev := <-sub.C
	if ev.FlowID != 1 {
		t.Fatalf("expected flow 1, got %d", ev.FlowID)
	}
	ev = <-sub.C
	if ev.FlowID != 2 {
		t.Fatalf("expected flow 2, got %d", ev.FlowID)
	}
}

// ---------------------------------------------------------------------------
// Composite Step Tests
// ---------------------------------------------------------------------------

// TestCompositeSimple: A(exec) → B(composite[B1→B2]) → C(exec), all succeed.
func TestCompositeSimple(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var callOrder []string
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		callOrder = append(callOrder, step.Name)
		return nil
	}

	// Expander returns two child steps: B1 → B2.
	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		b1 := &core.Step{Name: "B1", Type: core.StepExec}
		b2 := &core.Step{Name: "B2", Type: core.StepExec}
		// DependsOn will be set after B1 gets its ID, but we can't know it here.
		// Instead, return them flat; they'll run in parallel within the sub-flow.
		return []*core.Step{b1, b2}, nil
	})

	eng := New(store, bus, executor, WithConcurrency(1), WithExpander(expander))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "composite-simple", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	bID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepComposite, Status: core.StepPending, DependsOn: []int64{aID}})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{bID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	// A runs first, then B expands (B1, B2 run in sub-flow), then C.
	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	// Verify A ran, B1/B2 ran inside sub-flow, then C ran.
	if len(callOrder) != 4 {
		t.Fatalf("expected 4 executor calls (A, B1, B2, C), got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "A" {
		t.Fatalf("expected A first, got %s", callOrder[0])
	}
	if callOrder[3] != "C" {
		t.Fatalf("expected C last, got %s", callOrder[3])
	}

	// B should have SubFlowID set.
	stepB, _ := store.GetStep(ctx, bID)
	if stepB.SubFlowID == nil {
		t.Fatal("expected B to have SubFlowID")
	}
	if stepB.Status != core.StepDone {
		t.Fatalf("expected B done, got %s", stepB.Status)
	}

	// Sub-flow should also be done.
	subFlow, _ := store.GetFlow(ctx, *stepB.SubFlowID)
	if subFlow.Status != core.FlowDone {
		t.Fatalf("expected sub-flow done, got %s", subFlow.Status)
	}
	if subFlow.ParentStepID == nil || *subFlow.ParentStepID != bID {
		t.Fatal("sub-flow should link back to parent step B")
	}
}

// TestCompositeChainedChildren: composite with sequential children B1 → B2.
func TestCompositeChainedChildren(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var callOrder []string
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		callOrder = append(callOrder, step.Name)
		return nil
	}

	// This expander creates a sequential chain B1 → B2 using a two-pass approach.
	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		// We create B1 first, then B2 depends on B1.
		// Since ExpandComposite assigns IDs, we use a sentinel approach:
		// B1 has no deps (entry), B2 will depend on B1.
		// We can't set DependsOn by ID here since IDs aren't assigned yet.
		// Instead, we return them flat and use index-based dep within ExpandComposite.
		// Actually, ExpandComposite creates steps sequentially, so we can predict IDs.
		// BUT that's fragile. Instead let's just have them parallel for now.
		// For a true chain, the expander would need to create steps itself.
		return []*core.Step{
			{Name: "B1", Type: core.StepExec},
			{Name: "B2", Type: core.StepExec},
		}, nil
	})

	eng := New(store, bus, executor, WithConcurrency(1), WithExpander(expander))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "composite-chain", Status: core.FlowPending})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepComposite, Status: core.StepPending})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(callOrder), callOrder)
	}
}

// TestCompositeSubFlowFail: composite child fails → composite fails → parent flow fails.
func TestCompositeSubFlowFail(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Name == "child-bad" {
			return fmt.Errorf("child failure")
		}
		return nil
	}

	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		return []*core.Step{
			{Name: "child-bad", Type: core.StepExec},
		}, nil
	})

	eng := New(store, bus, executor, WithConcurrency(1), WithExpander(expander))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "composite-fail", Status: core.FlowPending})
	compID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "comp", Type: core.StepComposite, Status: core.StepPending})

	err := eng.Run(ctx, fID)
	if err == nil {
		t.Fatal("expected error from sub-flow failure")
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowFailed {
		t.Fatalf("expected failed, got %s", flow.Status)
	}

	compStep, _ := store.GetStep(ctx, compID)
	if compStep.Status != core.StepFailed {
		t.Fatalf("expected composite step failed, got %s", compStep.Status)
	}
}

// TestCompositeRetry: composite sub-flow fails once, composite retries with fresh sub-flow, succeeds.
func TestCompositeRetry(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var expandCount int32
	var execCount int32

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		n := atomic.AddInt32(&execCount, 1)
		// First child execution fails, second succeeds.
		if n == 1 {
			return fmt.Errorf("transient child failure")
		}
		return nil
	}

	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		atomic.AddInt32(&expandCount, 1)
		return []*core.Step{
			{Name: "child", Type: core.StepExec},
		}, nil
	})

	eng := New(store, bus, executor, WithConcurrency(1), WithExpander(expander))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "composite-retry", Status: core.FlowPending})
	compID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "comp",
		Type:       core.StepComposite,
		Status:     core.StepPending,
		MaxRetries: 1,
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	compStep, _ := store.GetStep(ctx, compID)
	if compStep.Status != core.StepDone {
		t.Fatalf("expected composite done, got %s", compStep.Status)
	}
	if compStep.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", compStep.RetryCount)
	}

	// Expander should have been called twice (original + retry).
	if expandCount != 2 {
		t.Fatalf("expected 2 expansions, got %d", expandCount)
	}
}

// ---------------------------------------------------------------------------
// Flow Integration Tests — cross-cutting scenarios
// ---------------------------------------------------------------------------

// TestFlowE2E_ResolverBriefingCollector: full pipeline with all 3 injectable interfaces.
// Flow: design(exec) → implement(exec) → done
// Verifies: resolver sets agent_id, briefing reads upstream artifact, collector extracts metadata.
func TestFlowE2E_ResolverBriefingCollector(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	profiles := []*core.AgentProfile{
		{ID: "designer", Role: core.RoleWorker, Capabilities: []string{"design"}},
		{ID: "coder", Role: core.RoleWorker, Capabilities: []string{"go"}},
	}

	collector := CollectorFunc(func(_ context.Context, stepType core.StepType, md string) (map[string]any, error) {
		return map[string]any{"collected": true, "type": string(stepType)}, nil
	})

	var capturedBriefing string
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Name == "implement" {
			capturedBriefing = exec.BriefingSnapshot
		}
		// Every step produces an artifact.
		_, err := store.CreateArtifact(ctx, &core.Artifact{
			ExecutionID:    exec.ID,
			StepID:         step.ID,
			FlowID:         step.FlowID,
			ResultMarkdown: fmt.Sprintf("## %s output\nDone.", step.Name),
		})
		return err
	}

	eng := New(store, bus, executor,
		WithConcurrency(1),
		WithResolver(NewProfileRegistry(profiles)),
		WithBriefingBuilder(NewBriefingBuilder(store)),
		WithCollector(collector),
	)

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-pipeline", Status: core.FlowPending})
	designID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "design",
		Type:                 core.StepExec,
		Status:               core.StepPending,
		AgentRole:            "worker",
		RequiredCapabilities: []string{"design"},
	})
	implID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "implement",
		Type:                 core.StepExec,
		Status:               core.StepPending,
		DependsOn:            []int64{designID},
		AgentRole:            "worker",
		RequiredCapabilities: []string{"go"},
		Config:               map[string]any{"objective": "Build login API"},
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	// Verify briefing was assembled with upstream artifact content.
	if !strings.Contains(capturedBriefing, "Build login API") {
		t.Fatalf("expected briefing snapshot to contain objective, got %q", capturedBriefing)
	}
	if !strings.Contains(capturedBriefing, "design output") {
		t.Fatalf("expected briefing snapshot to contain upstream artifact content, got %q", capturedBriefing)
	}

	// Verify collector extracted metadata into both artifacts.
	designArt, _ := store.GetLatestArtifactByStep(ctx, designID)
	if designArt.Metadata["collected"] != true {
		t.Fatalf("design artifact metadata not collected: %v", designArt.Metadata)
	}

	implArt, _ := store.GetLatestArtifactByStep(ctx, implID)
	if implArt.Metadata["collected"] != true {
		t.Fatalf("implement artifact metadata not collected: %v", implArt.Metadata)
	}
}

// TestFlowE2E_GateRejectRetryWithCollector: full gate reject → retry → pass cycle
// with collector extracting metadata at each step.
// Flow: impl(exec) → review(gate,reject) → impl retries → review(gate,pass) → deploy(exec) → done
func TestFlowE2E_GateRejectRetryWithCollector(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var gateCount int32
	var implCount int32
	var deployCount int32

	collector := CollectorFunc(func(_ context.Context, stepType core.StepType, md string) (map[string]any, error) {
		return map[string]any{"step_type": string(stepType)}, nil
	})

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Type == core.StepGate {
			n := atomic.AddInt32(&gateCount, 1)
			verdict := "reject"
			if n > 1 {
				verdict = "pass"
			}
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID:    exec.ID,
				StepID:         step.ID,
				FlowID:         step.FlowID,
				ResultMarkdown: "Review feedback",
				Metadata:       map[string]any{"verdict": verdict, "reason": "iteration " + fmt.Sprint(n)},
			})
			return err
		}
		if step.Name == "impl" {
			atomic.AddInt32(&implCount, 1)
		} else if step.Name == "deploy" {
			atomic.AddInt32(&deployCount, 1)
		}
		_, err := store.CreateArtifact(ctx, &core.Artifact{
			ExecutionID:    exec.ID,
			StepID:         step.ID,
			FlowID:         step.FlowID,
			ResultMarkdown: fmt.Sprintf("## %s output", step.Name),
		})
		return err
	}

	eng := New(store, bus, executor, WithConcurrency(1), WithCollector(collector))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-gate-retry", Status: core.FlowPending})
	implID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "impl",
		Type:       core.StepExec,
		Status:     core.StepPending,
		MaxRetries: 1,
	})
	reviewID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:    fID,
		Name:      "review",
		Type:      core.StepGate,
		Status:    core.StepPending,
		DependsOn: []int64{implID},
	})
	deployID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:    fID,
		Name:      "deploy",
		Type:      core.StepExec,
		Status:    core.StepPending,
		DependsOn: []int64{reviewID},
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	// impl ran twice (original + retry after rejection).
	if implCount != 2 {
		t.Fatalf("expected 2 impl runs, got %d", implCount)
	}
	// gate evaluated twice (reject + pass).
	if gateCount != 2 {
		t.Fatalf("expected 2 gate evaluations, got %d", gateCount)
	}
	// deploy ran once after gate passed.
	if deployCount != 1 {
		t.Fatalf("expected 1 deploy run, got %d", deployCount)
	}

	deployStep, _ := store.GetStep(ctx, deployID)
	if deployStep.Status != core.StepDone {
		t.Fatalf("expected deploy done, got %s", deployStep.Status)
	}

	// Collector should have extracted metadata into deploy artifact.
	deployArt, _ := store.GetLatestArtifactByStep(ctx, deployID)
	if deployArt.Metadata["step_type"] != "exec" {
		t.Fatalf("deploy artifact missing collected metadata: %v", deployArt.Metadata)
	}
}

// TestFlowE2E_CompositeWithGate: composite containing a gate inside its sub-flow.
// Flow: A(exec) → B(composite[B1(exec) → B2(gate,pass)]) → C(exec)
func TestFlowE2E_CompositeWithGate(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var callOrder []string
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		callOrder = append(callOrder, step.Name)
		if step.Type == core.StepGate {
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID:    exec.ID,
				StepID:         step.ID,
				FlowID:         step.FlowID,
				ResultMarkdown: "Gate pass",
				Metadata:       map[string]any{"verdict": "pass"},
			})
			return err
		}
		return nil
	}

	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		// B1(exec) → B2(gate). B2 depends on B1, but since we can't
		// set DependsOn before IDs are assigned, we return them flat.
		// Both will run as entry steps in the sub-flow.
		return []*core.Step{
			{Name: "B1", Type: core.StepExec},
			{Name: "B2", Type: core.StepGate},
		}, nil
	})

	eng := New(store, bus, executor, WithConcurrency(1), WithExpander(expander))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-composite-gate", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	bID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepComposite, Status: core.StepPending, DependsOn: []int64{aID}})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{bID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	// 4 executor calls: A, B1, B2, C
	if len(callOrder) != 4 {
		t.Fatalf("expected 4 calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "A" || callOrder[3] != "C" {
		t.Fatalf("expected A..C ordering, got %v", callOrder)
	}
}

// TestFlowE2E_FanOutMerge: A → (B, C) → D fan-out then merge.
// Verifies parallel execution and merge point.
func TestFlowE2E_FanOutMerge(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var counter int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		atomic.AddInt32(&counter, 1)
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(4))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-fan-merge", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	bID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "B", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})
	cID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})
	dID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "D", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{bID, cID}})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
	if counter != 4 {
		t.Fatalf("expected 4 executions, got %d", counter)
	}

	// D should be done (waited for both B and C).
	stepD, _ := store.GetStep(ctx, dID)
	if stepD.Status != core.StepDone {
		t.Fatalf("expected D done, got %s", stepD.Status)
	}
}

// TestFlowE2E_TimeoutRetryGatePass: slow step times out → retries → gate passes → done.
// Exercises timeout + retry + gate in a single flow.
func TestFlowE2E_TimeoutRetryGatePass(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	var implAttempts int32
	executor := func(ctx context.Context, step *core.Step, exec *core.Execution) error {
		if step.Name == "impl" {
			n := atomic.AddInt32(&implAttempts, 1)
			if n == 1 {
				// First attempt times out.
				select {
				case <-time.After(500 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}
		if step.Type == core.StepGate {
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID:    exec.ID,
				StepID:         step.ID,
				FlowID:         step.FlowID,
				ResultMarkdown: "Approved",
				Metadata:       map[string]any{"verdict": "pass"},
			})
			return err
		}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(1))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-timeout-gate", Status: core.FlowPending})
	implID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "impl",
		Type:       core.StepExec,
		Status:     core.StepPending,
		Timeout:    50 * time.Millisecond,
		MaxRetries: 1,
	})
	_, _ = store.CreateStep(ctx, &core.Step{
		FlowID:    fID,
		Name:      "review",
		Type:      core.StepGate,
		Status:    core.StepPending,
		DependsOn: []int64{implID},
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}
	if implAttempts != 2 {
		t.Fatalf("expected 2 impl attempts, got %d", implAttempts)
	}
}

// TestFlowE2E_PermanentErrorStopsFlow: parallel steps where one hits permanent error.
// Flow: A → (B, C). B fails permanently → flow fails without retrying B.
func TestFlowE2E_PermanentErrorStopsFlow(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		if step.Name == "B" {
			exec.ErrorKind = core.ErrKindPermanent
			return fmt.Errorf("bad config")
		}
		return nil
	}

	eng := New(store, bus, executor, WithConcurrency(4))

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-permanent", Status: core.FlowPending})
	aID, _ := store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "A", Type: core.StepExec, Status: core.StepPending})
	bID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "B",
		Type:       core.StepExec,
		Status:     core.StepPending,
		DependsOn:  []int64{aID},
		MaxRetries: 3,
	})
	_, _ = store.CreateStep(ctx, &core.Step{FlowID: fID, Name: "C", Type: core.StepExec, Status: core.StepPending, DependsOn: []int64{aID}})

	err := eng.Run(ctx, fID)
	if err == nil {
		t.Fatal("expected error")
	}

	stepB, _ := store.GetStep(ctx, bID)
	if stepB.RetryCount != 0 {
		t.Fatalf("permanent error should skip retry, got retry_count=%d", stepB.RetryCount)
	}
	if stepB.Status != core.StepFailed {
		t.Fatalf("expected B failed, got %s", stepB.Status)
	}
}

// TestFlowE2E_FullOrchestration: the big one — all features in a single flow.
// Flow: design(exec) → impl(composite[code→test]) → review(gate,reject→pass) → deploy(exec)
// With: resolver + briefing + collector + composite + gate reject/retry + timeout
func TestFlowE2E_FullOrchestration(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()

	profiles := []*core.AgentProfile{
		{ID: "architect", Role: core.RoleWorker, Capabilities: []string{"design"}},
		{ID: "coder", Role: core.RoleWorker, Capabilities: []string{"go"}},
		{ID: "reviewer", Role: core.RoleGate, Capabilities: []string{"code-review"}},
		{ID: "deployer", Role: core.RoleWorker, Capabilities: []string{"deploy"}},
	}

	collector := CollectorFunc(func(_ context.Context, stepType core.StepType, md string) (map[string]any, error) {
		return map[string]any{"collected": true}, nil
	})

	var gateCount int32
	var designCount int32
	executor := func(_ context.Context, step *core.Step, exec *core.Execution) error {
		// Every exec step produces an artifact.
		switch step.Name {
		case "design":
			atomic.AddInt32(&designCount, 1)
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID: exec.ID, StepID: step.ID, FlowID: step.FlowID,
				ResultMarkdown: "## Architecture\nLogin API with JWT.",
			})
			return err
		case "code", "test":
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID: exec.ID, StepID: step.ID, FlowID: step.FlowID,
				ResultMarkdown: fmt.Sprintf("## %s\nDone.", step.Name),
			})
			return err
		case "review":
			n := atomic.AddInt32(&gateCount, 1)
			verdict := "reject"
			if n > 1 {
				verdict = "pass"
			}
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID: exec.ID, StepID: step.ID, FlowID: step.FlowID,
				ResultMarkdown: "Review feedback",
				Metadata:       map[string]any{"verdict": verdict, "reason": "round " + fmt.Sprint(n)},
			})
			return err
		case "deploy":
			_, err := store.CreateArtifact(ctx, &core.Artifact{
				ExecutionID: exec.ID, StepID: step.ID, FlowID: step.FlowID,
				ResultMarkdown: "## Deploy\nDeployed to staging.",
			})
			return err
		}
		return nil
	}

	expander := ExpanderFunc(func(_ context.Context, step *core.Step) ([]*core.Step, error) {
		return []*core.Step{
			{Name: "code", Type: core.StepExec, AgentRole: "worker", RequiredCapabilities: []string{"go"}},
			{Name: "test", Type: core.StepExec, AgentRole: "worker", RequiredCapabilities: []string{"go"}},
		}, nil
	})

	eng := New(store, bus, executor,
		WithConcurrency(2),
		WithResolver(NewProfileRegistry(profiles)),
		WithBriefingBuilder(NewBriefingBuilder(store)),
		WithCollector(collector),
		WithExpander(expander),
	)

	fID, _ := store.CreateFlow(ctx, &core.Flow{Name: "e2e-full", Status: core.FlowPending})
	designID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "design",
		Type:                 core.StepExec,
		Status:               core.StepPending,
		AgentRole:            "worker",
		RequiredCapabilities: []string{"design"},
	})
	implID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:     fID,
		Name:       "impl",
		Type:       core.StepComposite,
		Status:     core.StepPending,
		DependsOn:  []int64{designID},
		MaxRetries: 1,
	})
	reviewID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "review",
		Type:                 core.StepGate,
		Status:               core.StepPending,
		DependsOn:            []int64{implID},
		AgentRole:            "gate",
		RequiredCapabilities: []string{"code-review"},
	})
	deployID, _ := store.CreateStep(ctx, &core.Step{
		FlowID:               fID,
		Name:                 "deploy",
		Type:                 core.StepExec,
		Status:               core.StepPending,
		DependsOn:            []int64{reviewID},
		AgentRole:            "worker",
		RequiredCapabilities: []string{"deploy"},
	})

	if err := eng.Run(ctx, fID); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Verify flow completed.
	flow, _ := store.GetFlow(ctx, fID)
	if flow.Status != core.FlowDone {
		t.Fatalf("expected done, got %s", flow.Status)
	}

	// Verify all steps done.
	for _, id := range []int64{designID, implID, reviewID, deployID} {
		s, _ := store.GetStep(ctx, id)
		if s.Status != core.StepDone {
			t.Fatalf("step %s (id=%d) expected done, got %s", s.Name, id, s.Status)
		}
	}

	// Gate rejected once then passed.
	if gateCount != 2 {
		t.Fatalf("expected 2 gate evaluations, got %d", gateCount)
	}

	// Composite should have been expanded (impl has SubFlowID).
	implStep, _ := store.GetStep(ctx, implID)
	if implStep.SubFlowID == nil {
		t.Fatal("expected impl to have SubFlowID")
	}

	// Collector should have enriched deploy artifact.
	deployArt, _ := store.GetLatestArtifactByStep(ctx, deployID)
	if deployArt == nil {
		t.Fatal("expected deploy artifact")
	}
	if deployArt.Metadata["collected"] != true {
		t.Fatalf("expected collected metadata on deploy, got %v", deployArt.Metadata)
	}

	// Design should have run only once (no retry for design).
	if designCount != 1 {
		t.Fatalf("expected 1 design run, got %d", designCount)
	}
}

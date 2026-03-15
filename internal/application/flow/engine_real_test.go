//go:build real

package flow_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	executoradapter "github.com/yoke233/ai-workflow/internal/adapters/executor"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	agentapp "github.com/yoke233/ai-workflow/internal/application/agent"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
	agentruntime "github.com/yoke233/ai-workflow/internal/runtime/agent"
)

// TestReal_WorkItemActionRun runs a single-action WorkItem through the full
// ACP execution pipeline: WorkItemEngine.Run → ACPActionExecutor → LocalSessionManager
// → real ACP agent.
//
// This test verifies the end-to-end "workitem action run" flow with a real agent
// process (no mocks). It mirrors the ThreadSessionPool real test pattern.
//
// Prerequisites:
//   - Set AI_WORKFLOW_REAL_WORKITEM=1 to enable
//   - Requires a valid .ai-workflow/config.toml with ACP driver configured
//   - Requires an API key for the configured agent (e.g. OPENAI_API_KEY)
//
// Run:
//
//	AI_WORKFLOW_REAL_WORKITEM=1 go test -tags real -run TestReal_WorkItemActionRun -v -timeout 300s ./internal/application/flow/...
func TestReal_WorkItemActionRun(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_WORKITEM") == "" {
		t.Skip("set AI_WORKFLOW_REAL_WORKITEM=1 to run")
	}

	env := setupRealEnv(t)

	// --- create work item + action ---
	ctx := context.Background()
	workItemID, err := env.store.CreateWorkItem(ctx, &core.WorkItem{
		Title:  "Real ACP WorkItem Test",
		Status: core.WorkItemOpen,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	t.Logf("work item id=%d", workItemID)

	actionID, err := env.store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "simple-task",
		Type:       core.ActionExec,
		Status:     core.ActionPending,
		Position:   0,
		AgentRole:  string(env.profile.Role),
		Config: map[string]any{
			"profile_id": env.profile.ID,
			"objective":  "Reply with exactly: HELLO_FROM_ACP. Do not write any files. Just reply with that text.",
		},
	})
	if err != nil {
		t.Fatalf("create action: %v", err)
	}
	t.Logf("action id=%d", actionID)

	// --- run ---
	t.Log(">>> starting WorkItemEngine.Run (may take a while for agent bootstrap)...")
	runCtx, runCancel := context.WithTimeout(ctx, 180*time.Second)
	defer runCancel()

	start := time.Now()
	runErr := env.engine.Run(runCtx, workItemID)
	elapsed := time.Since(start)
	t.Logf("engine.Run completed in %s", elapsed.Round(time.Millisecond))

	if runErr != nil {
		t.Fatalf("engine.Run failed: %v", runErr)
	}

	// --- verify ---

	// 1. WorkItem → done
	workItem, err := env.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	t.Logf("work item status: %s", workItem.Status)
	if workItem.Status != core.WorkItemDone {
		t.Fatalf("expected work item done, got %s", workItem.Status)
	}

	// 2. Action → done
	actions, err := env.store.ListActionsByWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	t.Logf("action status: %s", actions[0].Status)
	if actions[0].Status != core.ActionDone {
		t.Fatalf("expected action done, got %s", actions[0].Status)
	}

	// 3. Run → succeeded with output
	run, err := env.store.GetLatestRunWithResult(ctx, actionID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	t.Logf("run status: %s, agent: %s", run.Status, run.AgentID)
	if run.Status != core.RunSucceeded {
		t.Fatalf("expected run succeeded, got %s", run.Status)
	}

	if run.ResultMarkdown == "" {
		t.Fatal("expected non-empty result markdown")
	}
	t.Logf("result (%d chars): %s", len(run.ResultMarkdown), realTruncate(run.ResultMarkdown, 200))

	if !strings.Contains(run.ResultMarkdown, "HELLO_FROM_ACP") {
		t.Logf("WARNING: result does not contain 'HELLO_FROM_ACP' (agent may not follow instructions exactly)")
	}

	// 4. Token usage (may be zero for some agents — codex doesn't report usage).
	if run.Output != nil {
		inputTokens, _ := run.Output["input_tokens"].(float64)
		outputTokens, _ := run.Output["output_tokens"].(float64)
		t.Logf("token usage: input=%d output=%d", int64(inputTokens), int64(outputTokens))
		if inputTokens == 0 && outputTokens == 0 {
			t.Log("NOTE: agent reported zero token usage (some ACP agents do not expose usage)")
		}
	}

	// 5. Timing
	if run.StartedAt != nil && run.FinishedAt != nil {
		dur := run.FinishedAt.Sub(*run.StartedAt)
		t.Logf("run duration: %s", dur.Round(time.Millisecond))
	}

	t.Log(">>> PASS: real ACP workitem action run completed")
}

// TestReal_WorkItemMultiAction runs a two-action WorkItem sequentially through
// real ACP, verifying action ordering and completion.
//
// Run:
//
//	AI_WORKFLOW_REAL_WORKITEM=1 go test -tags real -run TestReal_WorkItemMultiAction -v -timeout 300s ./internal/application/flow/...
func TestReal_WorkItemMultiAction(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_WORKITEM") == "" {
		t.Skip("set AI_WORKFLOW_REAL_WORKITEM=1 to run")
	}

	env := setupRealEnv(t)
	ctx := context.Background()

	// Create work item with two sequential actions.
	workItemID, err := env.store.CreateWorkItem(ctx, &core.WorkItem{
		Title:  "Real ACP Multi-Action Test",
		Status: core.WorkItemOpen,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	action1ID, _ := env.store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "step-1-greet",
		Type:       core.ActionExec,
		Status:     core.ActionPending,
		Position:   0,
		AgentRole:  string(env.profile.Role),
		Config: map[string]any{
			"profile_id": env.profile.ID,
			"objective":  "Reply with exactly: STEP_1_DONE. Do not write any files.",
		},
	})

	action2ID, _ := env.store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "step-2-finish",
		Type:       core.ActionExec,
		Status:     core.ActionPending,
		Position:   1,
		AgentRole:  string(env.profile.Role),
		Config: map[string]any{
			"profile_id": env.profile.ID,
			"objective":  "Reply with exactly: STEP_2_DONE. Do not write any files.",
		},
	})

	t.Logf("work item=%d, action1=%d, action2=%d", workItemID, action1ID, action2ID)

	// Run.
	t.Log(">>> starting multi-action run...")
	runCtx, runCancel := context.WithTimeout(ctx, 300*time.Second)
	defer runCancel()

	start := time.Now()
	if err := env.engine.Run(runCtx, workItemID); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}
	t.Logf("engine.Run completed in %s", time.Since(start).Round(time.Millisecond))

	// Verify both actions done.
	for _, id := range []int64{action1ID, action2ID} {
		run, err := env.store.GetLatestRunWithResult(ctx, id)
		if err != nil {
			t.Fatalf("get run for action %d: %v", id, err)
		}
		t.Logf("action %d: run status=%s, result=%s",
			id, run.Status, realTruncate(run.ResultMarkdown, 100))
		if run.Status != core.RunSucceeded {
			t.Fatalf("expected run succeeded for action %d, got %s", id, run.Status)
		}
	}

	workItem, _ := env.store.GetWorkItem(ctx, workItemID)
	if workItem.Status != core.WorkItemDone {
		t.Fatalf("expected work item done, got %s", workItem.Status)
	}

	t.Log(">>> PASS: real ACP multi-action workitem completed")
}

// TestReal_WorkItemFallbackSignal tests that the fallback signal extraction
// from agent output works with a real ACP agent.
//
// Run:
//
//	AI_WORKFLOW_REAL_WORKITEM=1 go test -tags real -run TestReal_WorkItemFallbackSignal -v -timeout 300s ./internal/application/flow/...
func TestReal_WorkItemFallbackSignal(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_WORKITEM") == "" {
		t.Skip("set AI_WORKFLOW_REAL_WORKITEM=1 to run")
	}

	env := setupRealEnv(t)
	ctx := context.Background()

	workItemID, _ := env.store.CreateWorkItem(ctx, &core.WorkItem{
		Title:  "Real ACP Fallback Signal Test",
		Status: core.WorkItemOpen,
	})

	actionID, _ := env.store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "signal-task",
		Type:       core.ActionExec,
		Status:     core.ActionPending,
		Position:   0,
		AgentRole:  string(env.profile.Role),
		Config: map[string]any{
			"profile_id": env.profile.ID,
			"objective": `Complete the following task and at the end of your reply, emit a signal line.

Task: Say "hello world".

IMPORTANT: Your reply MUST end with exactly this line (no extra text after it):
AI_WORKFLOW_SIGNAL: {"decision":"complete","reason":"task done"}`,
		},
	})

	t.Logf("work item=%d, action=%d", workItemID, actionID)

	runCtx, runCancel := context.WithTimeout(ctx, 180*time.Second)
	defer runCancel()

	t.Log(">>> starting fallback signal test...")
	if err := env.engine.Run(runCtx, workItemID); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	run, _ := env.store.GetLatestRunWithResult(ctx, actionID)
	t.Logf("result: %s", realTruncate(run.ResultMarkdown, 300))

	if strings.Contains(run.ResultMarkdown, "AI_WORKFLOW_SIGNAL") {
		t.Log("agent emitted signal line in output")
	} else {
		t.Log("NOTE: agent did not emit signal line (may not follow instructions exactly)")
	}

	// Check if a fallback signal was created.
	sig, _ := env.store.GetLatestActionSignal(ctx, actionID, core.SignalComplete)
	if sig != nil {
		t.Logf("fallback signal created: type=%s source=%s", sig.Type, sig.Source)
	} else {
		t.Log("NOTE: no fallback signal created (agent may not have emitted signal line)")
	}

	workItem, _ := env.store.GetWorkItem(ctx, workItemID)
	if workItem.Status != core.WorkItemDone {
		t.Fatalf("expected work item done, got %s", workItem.Status)
	}

	t.Log(">>> PASS: real ACP fallback signal test completed")
}

// ---------------------------------------------------------------------------
// shared test environment
// ---------------------------------------------------------------------------

type realEnv struct {
	store   core.Store
	bus     core.EventBus
	engine  *flowapp.WorkItemEngine
	profile *core.AgentProfile
}

func setupRealEnv(t *testing.T) *realEnv {
	t.Helper()

	cfgPath := os.Getenv("AI_WORKFLOW_REAL_CONFIG")
	if cfgPath == "" {
		cfgPath = realFindConfig(t)
	}
	cfg, err := config.LoadGlobal(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "real-workitem.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	bus := membus.NewBus()
	ctx := context.Background()

	// Start event persister so action events are recorded.
	persister := flowapp.NewEventPersister(store, bus)
	persisterCtx, persisterCancel := context.WithCancel(ctx)
	if err := persister.Start(persisterCtx); err != nil {
		persisterCancel()
		t.Fatalf("start persister: %v", err)
	}
	t.Cleanup(func() {
		persisterCancel()
		persister.Stop()
	})

	// Build agent registry from config.
	registry := agentapp.NewConfigRegistry()
	profiles := configruntime.BuildAgents(cfg)
	if len(profiles) == 0 {
		t.Skip("no agent profiles configured")
	}
	registry.LoadProfiles(profiles)
	testProfile := realPickWorker(profiles)
	t.Logf("using profile: %s (role=%s, cmd=%s)",
		testProfile.ID, testProfile.Role, testProfile.Driver.LaunchCommand)

	// Work directory for agent sandbox.
	workDir := filepath.Join(baseDir, "workspace")
	_ = os.MkdirAll(workDir, 0o755)

	// Build the ACP execution stack: Pool → SessionManager → Executor.
	acpPool := agentruntime.NewACPSessionPool(store, bus)
	sessionMgr := agentruntime.NewLocalSessionManager(acpPool, store, nil) // no sandbox for test
	t.Cleanup(sessionMgr.Close)

	executor := executoradapter.NewACPActionExecutor(executoradapter.ACPExecutorConfig{
		Registry:       registry,
		Store:          store,
		Bus:            bus,
		SessionManager: sessionMgr,
		DefaultWorkDir: workDir,
	})

	eng := flowapp.New(store, bus, executor,
		flowapp.WithConcurrency(2),
		flowapp.WithResolver(registry),
	)

	return &realEnv{
		store:   store,
		bus:     bus,
		engine:  eng,
		profile: testProfile,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func realFindConfig(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, ".ai-workflow", "config.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("no .ai-workflow/config.toml found")
	return ""
}

func realPickWorker(profiles []*core.AgentProfile) *core.AgentProfile {
	for _, p := range profiles {
		id := strings.ToLower(p.ID)
		if id == "worker" || strings.Contains(id, "worker") {
			return p
		}
	}
	for _, p := range profiles {
		if p.Role == core.RoleWorker {
			return p
		}
	}
	return profiles[0]
}

func realTruncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

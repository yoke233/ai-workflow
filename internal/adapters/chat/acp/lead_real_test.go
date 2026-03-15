//go:build real

package acp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	agentapp "github.com/yoke233/ai-workflow/internal/application/agent"
	chatapp "github.com/yoke233/ai-workflow/internal/application/chat"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

// TestReal_LeadChat_SingleTurn tests the LeadAgent.Chat() flow with a real ACP agent:
// create session → send message → receive reply → verify session state.
//
// Prerequisites:
//   - Set AI_WORKFLOW_REAL_LEAD=1 to enable
//   - Requires a valid .ai-workflow/config.toml with a "lead" profile configured
//
// Run:
//
//	AI_WORKFLOW_REAL_LEAD=1 go test -tags real -run TestReal_LeadChat_SingleTurn -v -timeout 300s ./internal/adapters/chat/acp/...
func TestReal_LeadChat_SingleTurn(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_LEAD") == "" {
		t.Skip("set AI_WORKFLOW_REAL_LEAD=1 to run")
	}

	env := setupLeadRealEnv(t)

	ctx := context.Background()
	t.Log(">>> sending first message (may take a while for agent bootstrap)...")
	start := time.Now()
	resp, err := env.agent.Chat(ctx, chatapp.Request{
		Message: "Reply with exactly: LEAD_CHAT_OK. Do not write any files.",
		WorkDir: env.workDir,
	})
	elapsed := time.Since(start)
	t.Logf("Chat completed in %s", elapsed.Round(time.Millisecond))

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	t.Logf("session_id: %s", resp.SessionID)
	t.Logf("reply (%d chars): %s", len(resp.Reply), leadTruncate(resp.Reply, 200))
	t.Logf("ws_path: %s", resp.WSPath)

	if !strings.Contains(resp.Reply, "LEAD_CHAT_OK") {
		t.Log("WARNING: reply does not contain 'LEAD_CHAT_OK' (agent may not follow instructions exactly)")
	}

	// Verify session is persisted.
	sessions, err := env.agent.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	t.Logf("session status: %s, title: %q", sessions[0].Status, sessions[0].Title)

	// Verify session detail has message history.
	detail, err := env.agent.GetSession(ctx, resp.SessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (user+assistant), got %d", len(detail.Messages))
	}
	t.Logf("messages: %d (roles: %s)", len(detail.Messages), leadMessageRoles(detail.Messages))
	if detail.Messages[0].Role != "user" {
		t.Fatalf("first message should be user, got %q", detail.Messages[0].Role)
	}
	if detail.Messages[1].Role != "assistant" {
		t.Fatalf("second message should be assistant, got %q", detail.Messages[1].Role)
	}

	// Verify session alive status.
	if !env.agent.IsSessionAlive(resp.SessionID) {
		t.Fatal("expected session to be alive")
	}

	env.agent.Shutdown()
	t.Log(">>> PASS: real lead chat single turn completed")
}

// TestReal_LeadChat_MultiTurn tests multi-turn conversation with session reuse.
//
// Run:
//
//	AI_WORKFLOW_REAL_LEAD=1 go test -tags real -run TestReal_LeadChat_MultiTurn -v -timeout 300s ./internal/adapters/chat/acp/...
func TestReal_LeadChat_MultiTurn(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_LEAD") == "" {
		t.Skip("set AI_WORKFLOW_REAL_LEAD=1 to run")
	}

	env := setupLeadRealEnv(t)

	ctx := context.Background()

	// Turn 1: create session.
	t.Log(">>> turn 1: creating session...")
	resp1, err := env.agent.Chat(ctx, chatapp.Request{
		Message: "Remember this number: 42. Reply with: REMEMBERED",
		WorkDir: env.workDir,
	})
	if err != nil {
		t.Fatalf("turn 1 failed: %v", err)
	}
	t.Logf("turn 1 reply: %s", leadTruncate(resp1.Reply, 150))

	// Turn 2: reuse session, test conversational memory.
	t.Log(">>> turn 2: follow-up on same session...")
	resp2, err := env.agent.Chat(ctx, chatapp.Request{
		SessionID: resp1.SessionID,
		Message:   "What number did I ask you to remember? Reply with just the number.",
	})
	if err != nil {
		t.Fatalf("turn 2 failed: %v", err)
	}
	t.Logf("turn 2 reply: %s", leadTruncate(resp2.Reply, 150))

	if resp2.SessionID != resp1.SessionID {
		t.Fatalf("session ID changed: %s → %s", resp1.SessionID, resp2.SessionID)
	}

	if strings.Contains(resp2.Reply, "42") {
		t.Log("agent correctly recalled the number 42")
	} else {
		t.Log("WARNING: agent did not recall '42' (conversational memory may vary)")
	}

	// Verify 4 messages in history (2 user + 2 assistant).
	detail, err := env.agent.GetSession(ctx, resp1.SessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	t.Logf("messages: %d (roles: %s)", len(detail.Messages), leadMessageRoles(detail.Messages))
	if len(detail.Messages) < 4 {
		t.Fatalf("expected at least 4 messages, got %d", len(detail.Messages))
	}

	env.agent.Shutdown()
	t.Log(">>> PASS: real lead chat multi-turn completed")
}

// TestReal_LeadChat_SessionRestore tests that a session can be restored
// after agent shutdown (persisted session → reload).
//
// Run:
//
//	AI_WORKFLOW_REAL_LEAD=1 go test -tags real -run TestReal_LeadChat_SessionRestore -v -timeout 300s ./internal/adapters/chat/acp/...
func TestReal_LeadChat_SessionRestore(t *testing.T) {
	if os.Getenv("AI_WORKFLOW_REAL_LEAD") == "" {
		t.Skip("set AI_WORKFLOW_REAL_LEAD=1 to run")
	}

	env := setupLeadRealEnv(t)
	ctx := context.Background()

	// Turn 1: create session.
	t.Log(">>> turn 1: creating session...")
	resp1, err := env.agent.Chat(ctx, chatapp.Request{
		Message: "Say hello. Reply with: HELLO_LEAD",
		WorkDir: env.workDir,
	})
	if err != nil {
		t.Fatalf("turn 1 failed: %v", err)
	}
	sessionID := resp1.SessionID
	t.Logf("session_id: %s, reply: %s", sessionID, leadTruncate(resp1.Reply, 100))

	// Shutdown and recreate agent (simulates server restart).
	t.Log(">>> shutting down agent (simulating restart)...")
	env.agent.Shutdown()

	// Verify session is no longer alive in memory but catalog persists.
	agent2 := NewLeadAgent(env.agentCfg)

	sessions, err := agent2.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions after restart: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(sessions))
	}
	t.Logf("persisted session: %s (status=%s)", sessions[0].SessionID, sessions[0].Status)

	// Turn 2: resume on restored session.
	t.Log(">>> turn 2: resuming session after restart...")
	resp2, err := agent2.Chat(ctx, chatapp.Request{
		SessionID: sessionID,
		Message:   "What was the first thing I said? Reply briefly.",
	})
	if err != nil {
		t.Fatalf("turn 2 (restore) failed: %v", err)
	}
	t.Logf("turn 2 reply: %s", leadTruncate(resp2.Reply, 150))

	if resp2.SessionID != sessionID {
		t.Fatalf("session ID changed on restore: %s → %s", sessionID, resp2.SessionID)
	}

	// Verify session is alive again.
	if !agent2.IsSessionAlive(sessionID) {
		t.Fatal("expected restored session to be alive")
	}

	detail, err := agent2.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	t.Logf("messages after restore: %d (roles: %s)",
		len(detail.Messages), leadMessageRoles(detail.Messages))

	agent2.Shutdown()
	t.Log(">>> PASS: real lead chat session restore completed")
}

// ---------------------------------------------------------------------------
// shared test environment
// ---------------------------------------------------------------------------

type leadRealEnv struct {
	agent    *LeadAgent
	agentCfg LeadAgentConfig
	workDir  string
}

func setupLeadRealEnv(t *testing.T) *leadRealEnv {
	t.Helper()

	cfgPath := os.Getenv("AI_WORKFLOW_REAL_CONFIG")
	if cfgPath == "" {
		cfgPath = leadFindConfig(t)
	}
	cfg, err := config.LoadGlobal(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	registry := agentapp.NewConfigRegistry()
	profiles := configruntime.BuildAgents(cfg)
	if len(profiles) == 0 {
		t.Skip("no agent profiles configured")
	}
	registry.LoadProfiles(profiles)

	// Find a lead profile, fallback to any worker.
	testProfile := leadPickProfile(profiles)
	t.Logf("using profile: %s (role=%s, cmd=%s)",
		testProfile.ID, testProfile.Role, testProfile.Driver.LaunchCommand)

	dataDir := filepath.Join(t.TempDir(), "lead-data")
	_ = os.MkdirAll(dataDir, 0o755)

	workDir := filepath.Join(t.TempDir(), "lead-workspace")
	_ = os.MkdirAll(workDir, 0o755)

	agentCfg := LeadAgentConfig{
		Registry:  registry,
		Bus:       membus.NewBus(),
		Sandbox:   v2sandbox.NoopSandbox{},
		DataDir:   dataDir,
		ProfileID: testProfile.ID,
		Timeout:   120 * time.Second,
	}

	agent := NewLeadAgent(agentCfg)

	return &leadRealEnv{
		agent:    agent,
		agentCfg: agentCfg,
		workDir:  workDir,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func leadFindConfig(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		p := filepath.Join(dir, ".ai-workflow", "config.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("no .ai-workflow/config.toml found")
	return ""
}

func leadPickProfile(profiles []*core.AgentProfile) *core.AgentProfile {
	// If AI_WORKFLOW_REAL_LEAD_PROFILE is set, use that specific profile.
	if envProfile := os.Getenv("AI_WORKFLOW_REAL_LEAD_PROFILE"); envProfile != "" {
		for _, p := range profiles {
			if p.ID == envProfile {
				return p
			}
		}
	}
	// Prefer a profile with "lead" in the ID.
	for _, p := range profiles {
		id := strings.ToLower(p.ID)
		if id == "lead" || strings.Contains(id, "lead") {
			return p
		}
	}
	// Fallback: any RoleLead.
	for _, p := range profiles {
		if p.Role == core.RoleLead {
			return p
		}
	}
	// Fallback: worker.
	for _, p := range profiles {
		if p.Role == core.RoleWorker {
			return p
		}
	}
	return profiles[0]
}

func leadTruncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func leadMessageRoles(msgs []chatapp.Message) string {
	parts := make([]string, len(msgs))
	for i, m := range msgs {
		parts[i] = m.Role
	}
	return strings.Join(parts, ", ")
}

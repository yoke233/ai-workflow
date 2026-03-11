package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// ACPExecutorConfig configures the ACP step executor.
type ACPExecutorConfig struct {
	Registry                 core.AgentRegistry
	Store                    core.Store
	Bus                      core.EventBus
	DefaultWorkDir           string
	MCPEnv                   teamleader.MCPEnvConfig
	MCPResolver              func(profileID string, agentSupportsSSE bool) []acpproto.McpServer
	SessionManager           SessionManager
	ReworkFollowupTemplate   string
	ContinueFollowupTemplate string
}

// NewACPStepExecutor creates a StepExecutor that uses a SessionManager for ACP agent execution.
// It resolves step → AgentProfile + AgentDriver via the AgentRegistry, acquires a session,
// starts the execution, watches for completion, then stores the result.
func NewACPStepExecutor(cfg ACPExecutorConfig) StepExecutor {
	return func(ctx context.Context, step *core.Step, exec *core.Execution) error {
		if cfg.SessionManager == nil {
			return fmt.Errorf("session manager is not configured")
		}

		profile, driver, err := cfg.Registry.ResolveForStep(ctx, step)
		if err != nil {
			return fmt.Errorf("resolve agent for step %d: %w", step.ID, err)
		}
		exec.AgentID = profile.ID

		workDir := cfg.DefaultWorkDir
		if ws := WorkspaceFromContext(ctx); ws != nil {
			workDir = ws.Path
		}
		if v, ok := step.Config["work_dir"].(string); ok && v != "" {
			workDir = v
		}

		launchCfg := acpclient.LaunchConfig{
			Command: driver.LaunchCommand,
			Args:    driver.LaunchArgs,
			WorkDir: workDir,
			Env:     cloneEnv(driver.Env),
		}

		bridge := NewEventBridge(cfg.Bus, core.EventExecAgentOutput, EventBridgeScope{
			FlowID: step.FlowID,
			StepID: step.ID,
			ExecID: exec.ID,
		})

		caps := profile.EffectiveCapabilities()
		acpCaps := acpclient.ClientCapabilities{
			FSRead:   caps.FSRead,
			FSWrite:  caps.FSWrite,
			Terminal: caps.Terminal,
		}

		reuse := profile.Session.Reuse

		handle, err := cfg.SessionManager.Acquire(ctx, SessionAcquireInput{
			Profile: profile,
			Driver:  driver,
			Launch:  launchCfg,
			Caps:    acpCaps,
			WorkDir: workDir,
			MCPFactory: func(agentSupportsSSE bool) []acpproto.McpServer {
				if cfg.MCPResolver != nil {
					return cfg.MCPResolver(profile.ID, agentSupportsSSE)
				}
				roleProfile := acpclient.RoleProfile{
					ID:         profile.ID,
					MCPEnabled: profile.MCP.Enabled,
					MCPTools:   append([]string(nil), profile.MCP.Tools...),
				}
				return teamleader.MCPToolsFromRoleConfig(roleProfile, cfg.MCPEnv, agentSupportsSSE)
			},
			FlowID:   step.FlowID,
			StepID:   step.ID,
			ExecID:   exec.ID,
			Reuse:    reuse,
			IdleTTL:  profile.Session.IdleTTL,
			MaxTurns: profile.Session.MaxTurns,
		})
		if err != nil {
			return fmt.Errorf("acquire session: %w", err)
		}
		defer func() {
			if !reuse {
				_ = cfg.SessionManager.Release(ctx, handle)
			}
		}()

		if handle.AgentContextID != nil {
			exec.AgentContextID = handle.AgentContextID
		}

		executionInput := buildExecutionInputForStep(profile, exec.BriefingSnapshot, step, handle.HasPriorTurns, cfg.ReworkFollowupTemplate, cfg.ContinueFollowupTemplate)

		invocationID, err := cfg.SessionManager.StartExecution(ctx, handle, executionInput)
		if err != nil {
			return fmt.Errorf("start execution: %w", err)
		}

		result, err := cfg.SessionManager.WatchExecution(ctx, invocationID, 0, bridge)
		if err != nil {
			return fmt.Errorf("watch execution: %w", err)
		}
		if result != nil && result.AgentContextID != nil {
			exec.AgentContextID = result.AgentContextID
		}

		// Flush any remaining accumulated chunks.
		bridge.FlushPending(ctx)

		// Publish done event with full reply.
		replyText := strings.TrimSpace(result.Text)
		bridge.PublishData(ctx, map[string]any{
			"type":    "done",
			"content": replyText,
		})

		// Store agent output as an Artifact.
		art := &core.Artifact{
			ExecutionID:    exec.ID,
			StepID:         step.ID,
			FlowID:         step.FlowID,
			ResultMarkdown: replyText,
		}
		if step.Type == core.StepGate {
			art.Metadata = extractGateMetadata(replyText)
		}
		artID, err := cfg.Store.CreateArtifact(ctx, art)
		if err != nil {
			return fmt.Errorf("store artifact: %w", err)
		}
		exec.ArtifactID = &artID
		exec.Output = map[string]any{
			"text":        replyText,
			"stop_reason": result.StopReason,
		}
		if result.InputTokens > 0 || result.OutputTokens > 0 {
			exec.Output["input_tokens"] = result.InputTokens
			exec.Output["output_tokens"] = result.OutputTokens
		}

		slog.Info("v2 ACP step executed",
			"step_id", step.ID, "agent", profile.ID,
			"output_len", len(replyText),
			"stop_reason", result.StopReason)

		return nil
	}
}

var reGateJSONLine = regexp.MustCompile(`(?m)^AI_WORKFLOW_GATE_JSON:\s*(\{.*\})\s*$`)

// extractGateMetadata parses a deterministic JSON line emitted by the reviewer agent.
// Expected format (single line):
//
//	AI_WORKFLOW_GATE_JSON: {"verdict":"pass"|"reject","reason":"...","reject_targets":[1,2,3]}
func extractGateMetadata(markdown string) map[string]any {
	m := reGateJSONLine.FindAllStringSubmatch(markdown, -1)
	if len(m) == 0 {
		return map[string]any{"verdict": "pass"}
	}
	raw := strings.TrimSpace(m[len(m)-1][1])
	if raw == "" {
		return map[string]any{"verdict": "pass"}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]any{"verdict": "reject", "reason": "invalid gate json"}
	}
	verdict, _ := parsed["verdict"].(string)
	verdict = strings.ToLower(strings.TrimSpace(verdict))
	if verdict != "reject" {
		verdict = "pass"
	}
	parsed["verdict"] = verdict
	return parsed
}

// buildExecutionInputFromBriefing constructs the execution input sent to the ACP agent.
func buildExecutionInputFromBriefing(snapshot string, step *core.Step) string {
	var sb strings.Builder
	sb.WriteString("# Task\n\n")
	sb.WriteString(snapshot)

	if len(step.AcceptanceCriteria) > 0 {
		sb.WriteString("\n\n# Acceptance Criteria\n\n")
		for _, c := range step.AcceptanceCriteria {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func cloneEnv(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildExecutionInputForStep(profile *core.AgentProfile, snapshot string, step *core.Step, hasPriorTurns bool, reworkTmpl string, continueTmpl string) string {
	// For gate steps, always repeat the full instruction block to ensure deterministic output.
	if step != nil && step.Type == core.StepGate {
		return buildExecutionInputFromBriefing(snapshot, step)
	}

	feedback := latestGateFeedback(step)
	// If the agent is in a reused session and we already have prior turns, send only the incremental
	// feedback to preserve execution context caching and leverage the existing context window.
	if profile != nil && profile.Session.Reuse && hasPriorTurns {
		if feedback != "" {
			return renderFollowupExecutionMessage(reworkTmpl, followupVars{Feedback: feedback, StepName: stepName(step)})
		}
		// No explicit feedback — continue without re-sending the full base instruction block.
		return renderFollowupExecutionMessage(continueTmpl, followupVars{StepName: stepName(step)})
	}

	// Default: full base instruction block + optional feedback section.
	base := buildExecutionInputFromBriefing(snapshot, step)
	if feedback == "" {
		return base
	}
	return base + "\n\n# Gate Feedback (Rework)\n\n" + feedback + "\n"
}

func latestGateFeedback(step *core.Step) string {
	if step == nil || step.Config == nil {
		return ""
	}
	last, _ := step.Config["last_gate_feedback"].(map[string]any)
	if last == nil {
		// Fall back to the end of rework_history.
		if arr, ok := step.Config["rework_history"].([]any); ok && len(arr) > 0 {
			if m, ok := arr[len(arr)-1].(map[string]any); ok {
				last = m
			}
		}
	}
	if last == nil {
		return ""
	}
	reason, _ := last["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Reason: ")
	sb.WriteString(reason)
	if prURL, ok := last["pr_url"].(string); ok && strings.TrimSpace(prURL) != "" {
		sb.WriteString("\nPR: ")
		sb.WriteString(strings.TrimSpace(prURL))
	}
	if n, ok := last["pr_number"]; ok {
		sb.WriteString("\nPR Number: ")
		sb.WriteString(fmt.Sprint(n))
	}
	return sb.String()
}

type followupVars struct {
	Feedback string
	StepName string
}

func stepName(step *core.Step) string {
	if step == nil {
		return ""
	}
	return strings.TrimSpace(step.Name)
}

func renderFollowupExecutionMessage(tmplText string, vars followupVars) string {
	// Safe fallback: no template provided.
	if strings.TrimSpace(tmplText) == "" {
		if strings.TrimSpace(vars.Feedback) == "" {
			if vars.StepName == "" {
				return "# Continue\n\n请继续完成当前任务（复用已有上下文）。\n"
			}
			return "# Continue\n\n请继续完成本 step（复用已有上下文）： " + vars.StepName + "\n"
		}
		if vars.StepName == "" {
			return "# Rework Requested\n\n反馈：\n" + vars.Feedback + "\n"
		}
		return "# Rework Requested\n\n(step: " + vars.StepName + ")\n\n反馈：\n" + vars.Feedback + "\n"
	}

	tmpl, err := template.New("v2-followup").Parse(tmplText)
	if err != nil {
		// Never fail the execution due to follow-up template issues.
		slog.Warn("v2 followup execution message: invalid template", "error", err)
		return "# Rework Requested\n\n反馈：\n" + vars.Feedback + "\n"
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, vars); err != nil {
		slog.Warn("v2 followup execution message: render failed", "error", err)
		return "# Rework Requested\n\n反馈：\n" + vars.Feedback + "\n"
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "# Rework Requested\n\n反馈：\n" + vars.Feedback + "\n"
	}
	return out
}

// ---------------------------------------------------------------------------
// EventBridge — chunk aggregation, tool call extraction, usage tracking
// ---------------------------------------------------------------------------

// EventBridgeScope identifies the context of events published by the bridge.
type EventBridgeScope struct {
	FlowID    int64
	StepID    int64
	ExecID    int64
	SessionID string // used for chat events (no flow/step/exec)
}

// EventBridge converts ACP SessionUpdate events into v2 core.Event published on the bus.
// Chunk types are accumulated and flushed as complete content on type boundaries.
// Matches v1 stageEventBridge semantics.
type EventBridge struct {
	bus       core.EventBus
	eventType core.EventType
	scope     EventBridgeScope

	lastActivity atomic.Int64 // unix nano

	mu             sync.Mutex
	pendingThought strings.Builder
	pendingMessage strings.Builder
}

// NewEventBridge creates an EventBridge for publishing ACP events.
func NewEventBridge(bus core.EventBus, eventType core.EventType, scope EventBridgeScope) *EventBridge {
	b := &EventBridge{
		bus:       bus,
		eventType: eventType,
		scope:     scope,
	}
	b.lastActivity.Store(time.Now().UnixNano())
	return b
}

// LastActivity returns the time of the last ACP event received.
func (b *EventBridge) LastActivity() time.Time {
	return time.Unix(0, b.lastActivity.Load())
}

// HandleSessionUpdate implements acpclient.EventHandler.
func (b *EventBridge) HandleSessionUpdate(ctx context.Context, update acpclient.SessionUpdate) error {
	b.lastActivity.Store(time.Now().UnixNano())

	// Flush accumulated chunks when the incoming type differs (preserves boundaries).
	switch update.Type {
	case "agent_thought_chunk":
		b.flushMessage(ctx)
	case "agent_message_chunk":
		b.flushThought(ctx)
	default:
		b.FlushPending(ctx)
	}

	switch update.Type {
	case "agent_thought_chunk":
		b.mu.Lock()
		b.pendingThought.WriteString(update.Text)
		b.mu.Unlock()
		b.publishChunk(ctx, update)

	case "agent_message_chunk":
		b.mu.Lock()
		b.pendingMessage.WriteString(update.Text)
		b.mu.Unlock()
		b.publishChunk(ctx, update)

	case "tool_call":
		b.publishToolCall(ctx, update)

	case "tool_call_update":
		if update.Status == "completed" {
			b.publishToolCallCompleted(ctx, update)
		}

	case "usage_update":
		b.publishUsageUpdate(ctx, update)

	default:
		b.publishChunk(ctx, update)
	}

	return nil
}

// FlushPending flushes all accumulated thought and message chunks.
func (b *EventBridge) FlushPending(ctx context.Context) {
	b.flushThought(ctx)
	b.flushMessage(ctx)
}

// PublishData publishes an event with arbitrary data (used for done and runtime events).
func (b *EventBridge) PublishData(ctx context.Context, data map[string]any) {
	b.publish(ctx, data)
}

func (b *EventBridge) flushThought(ctx context.Context) {
	b.mu.Lock()
	thought := b.pendingThought.String()
	b.pendingThought.Reset()
	b.mu.Unlock()
	if thought != "" {
		b.publish(ctx, map[string]any{
			"type":    "agent_thought",
			"content": thought,
		})
	}
}

func (b *EventBridge) flushMessage(ctx context.Context) {
	b.mu.Lock()
	message := b.pendingMessage.String()
	b.pendingMessage.Reset()
	b.mu.Unlock()
	if message != "" {
		b.publish(ctx, map[string]any{
			"type":    "agent_message",
			"content": message,
		})
	}
}

// publishChunk sends a streaming event for WS broadcast (persister will skip it).
func (b *EventBridge) publishChunk(ctx context.Context, update acpclient.SessionUpdate) {
	if update.Text == "" {
		return
	}
	b.publish(ctx, map[string]any{
		"type":    update.Type,
		"content": update.Text,
	})
}

func (b *EventBridge) publishToolCall(ctx context.Context, update acpclient.SessionUpdate) {
	data := map[string]any{"type": "tool_call"}
	var parsed struct {
		Title      string `json:"title"`
		ToolCallID string `json:"toolCallId"`
	}
	if json.Unmarshal(update.RawJSON, &parsed) == nil {
		if parsed.Title != "" {
			data["content"] = parsed.Title
		}
		if parsed.ToolCallID != "" {
			data["tool_call_id"] = parsed.ToolCallID
		}
	}
	b.publish(ctx, data)
}

func (b *EventBridge) publishToolCallCompleted(ctx context.Context, update acpclient.SessionUpdate) {
	data := map[string]any{"type": "tool_call_completed"}
	var parsed struct {
		ToolCallID string `json:"toolCallId"`
		RawOutput  struct {
			ExitCode int    `json:"exit_code"`
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
		} `json:"rawOutput"`
	}
	if json.Unmarshal(update.RawJSON, &parsed) == nil {
		data["tool_call_id"] = parsed.ToolCallID
		data["exit_code"] = parsed.RawOutput.ExitCode
		stdout := parsed.RawOutput.Stdout
		if len(stdout) > 2000 {
			stdout = stdout[:2000] + "...(truncated)"
		}
		data["content"] = stdout
		if parsed.RawOutput.Stderr != "" {
			stderr := parsed.RawOutput.Stderr
			if len(stderr) > 2000 {
				stderr = stderr[:2000] + "...(truncated)"
			}
			data["stderr"] = stderr
		}
	}
	b.publish(ctx, data)
}

func (b *EventBridge) publishUsageUpdate(ctx context.Context, update acpclient.SessionUpdate) {
	data := map[string]any{"type": "usage_update"}
	var usage struct {
		Size int64 `json:"size"`
		Used int64 `json:"used"`
	}
	if json.Unmarshal(update.RawJSON, &usage) == nil {
		data["usage_size"] = usage.Size
		data["usage_used"] = usage.Used
	}
	b.publish(ctx, data)
}

func (b *EventBridge) publish(ctx context.Context, data map[string]any) {
	ev := core.Event{
		Type:      b.eventType,
		FlowID:    b.scope.FlowID,
		StepID:    b.scope.StepID,
		ExecID:    b.scope.ExecID,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
	if b.scope.SessionID != "" {
		if ev.Data == nil {
			ev.Data = map[string]any{}
		}
		ev.Data["session_id"] = b.scope.SessionID
	}
	b.bus.Publish(ctx, ev)
}

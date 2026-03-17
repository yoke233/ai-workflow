package acpclient

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	agentsdkapi "github.com/cexll/agentsdk-go/pkg/api"
	agentsdkmodel "github.com/cexll/agentsdk-go/pkg/model"
	acpproto "github.com/coder/acp-go-sdk"
)

func TestAgentSDKInProcLifecycleAndStreaming(t *testing.T) {
	originalBuilder := buildAgentSDKOptionsFromLaunch
	buildAgentSDKOptionsFromLaunch = func(cfg LaunchConfig) (agentsdkapi.Options, error) {
		return agentsdkapi.Options{
			ProjectRoot: cfg.WorkDir,
			ModelFactory: agentsdkapi.ModelFactoryFunc(func(context.Context) (agentsdkmodel.Model, error) {
				return stubAgentSDKModel{}, nil
			}),
		}, nil
	}
	t.Cleanup(func() {
		buildAgentSDKOptionsFromLaunch = originalBuilder
	})

	root := t.TempDir()
	handler := &agentsdkInProcEventRecorder{}
	client, err := New(LaunchConfig{
		Command: "agentsdk-go",
		WorkDir: root,
		Env: map[string]string{
			"AGENTSDK_PROVIDER": "openai_response",
			"AGENTSDK_MODEL":    "gpt-4.1-mini",
		},
	}, handler, WithEventHandler(handler))
	if err != nil {
		t.Fatalf("create in-proc agentsdk client: %v", err)
	}
	defer func() {
		if err := client.Close(context.Background()); err != nil {
			t.Fatalf("close client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Initialize(ctx, ClientCapabilities{FSRead: true, FSWrite: true, Terminal: true}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	session, err := client.NewSessionResult(ctx, acpproto.NewSessionRequest{
		Cwd:        root,
		McpServers: []acpproto.McpServer{},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if session.SessionID == "" {
		t.Fatal("expected non-empty session id")
	}
	if session.Modes == nil || len(session.Modes.AvailableModes) == 0 {
		t.Fatal("expected available session modes from agentsdk-go adapter")
	}
	if len(session.ConfigOptions) == 0 {
		t.Fatal("expected config options from agentsdk-go adapter")
	}

	promptResult, err := client.Prompt(ctx, acpproto.PromptRequest{
		SessionId: session.SessionID,
		Prompt:    []acpproto.ContentBlock{acpproto.TextBlock("hello")},
	})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if promptResult == nil {
		t.Fatal("expected non-nil prompt result")
	}
	if promptResult.StopReason != acpproto.StopReasonEndTurn {
		t.Fatalf("stop reason = %q, want %q", promptResult.StopReason, acpproto.StopReasonEndTurn)
	}
	if promptResult.Text != "ok" {
		t.Fatalf("prompt text = %q, want %q", promptResult.Text, "ok")
	}

	modeID := chooseAlternateModeID(session.Modes)
	if modeID == "" {
		t.Fatal("expected alternate mode id from agentsdk-go adapter")
	}
	if err := client.SetSessionMode(ctx, acpproto.SetSessionModeRequest{
		SessionId: session.SessionID,
		ModeId:    modeID,
	}); err != nil {
		t.Fatalf("set session mode: %v", err)
	}

	configID, configValue := chooseAlternateConfigValue(session.ConfigOptions)
	if configID == "" || configValue == "" {
		t.Fatal("expected selectable config option from agentsdk-go adapter")
	}
	configOptions, err := client.SetConfigOption(ctx, acpproto.SetSessionConfigOptionRequest{
		SessionId: session.SessionID,
		ConfigId:  configID,
		Value:     configValue,
	})
	if err != nil {
		t.Fatalf("set config option: %v", err)
	}
	if len(configOptions) == 0 {
		t.Fatal("expected config option response after update")
	}

	requireEventually(t, 2*time.Second, func() bool {
		return handler.countType("agent_message_chunk") > 0 &&
			handler.countType("current_mode_update") > 0 &&
			handler.countType("config_option_update") > 0
	}, "agentsdk in-proc ACP updates")
}

func TestAgentSDKStreamDebugEnvParsing(t *testing.T) {
	if isAgentSDKStreamDebugEnabled(nil) {
		t.Fatal("expected nil env to disable stream debug")
	}
	if !isAgentSDKStreamDebugEnabled(map[string]string{agentSDKDebugStreamEnv: "true"}) {
		t.Fatal("expected primary env to enable stream debug")
	}
	if !isAgentSDKStreamDebugEnabled(map[string]string{agentSDKDebugStreamLegacyEnv: "1"}) {
		t.Fatal("expected legacy env to enable stream debug")
	}
	if isAgentSDKStreamDebugEnabled(map[string]string{agentSDKDebugStreamEnv: "false"}) {
		t.Fatal("expected false to disable stream debug")
	}
}

func TestAgentSDKDebugModelLogsRawStreamEvents(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	model := &agentSDKDebugModel{
		base: stubAgentSDKModel{},
		provider: agentSDKProviderConfig{
			ProviderType: "anthropic",
			Model:        "claude-opus-4-6",
			BaseURL:      "https://example.invalid/v1",
		},
		logger: logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callCount := 0
	err := model.CompleteStream(ctx, agentsdkmodel.Request{SessionID: "sess-debug"}, func(sr agentsdkmodel.StreamResult) error {
		callCount++
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteStream returned error: %v", err)
	}
	if callCount == 0 {
		t.Fatal("expected wrapped callback to receive stream results")
	}

	logs := buf.String()
	if !strings.Contains(logs, "acpclient: agentsdk-go raw stream event") {
		t.Fatalf("expected raw stream event log, got %q", logs)
	}
	if !strings.Contains(logs, "\\\"Delta\\\":\\\"ok\\\"") {
		t.Fatalf("expected delta payload in logs, got %q", logs)
	}
	if !strings.Contains(logs, "claude-opus-4-6") {
		t.Fatalf("expected provider metadata in logs, got %q", logs)
	}
}

type stubAgentSDKModel struct{}

func (stubAgentSDKModel) Complete(_ context.Context, _ agentsdkmodel.Request) (*agentsdkmodel.Response, error) {
	return &agentsdkmodel.Response{
		Message: agentsdkmodel.Message{
			Role:    "assistant",
			Content: "ok",
		},
		StopReason: "end_turn",
	}, nil
}

func (stubAgentSDKModel) CompleteStream(_ context.Context, _ agentsdkmodel.Request, cb agentsdkmodel.StreamHandler) error {
	if cb == nil {
		return nil
	}
	return cb(agentsdkmodel.StreamResult{
		Delta: "ok",
		Final: true,
		Response: &agentsdkmodel.Response{
			Message: agentsdkmodel.Message{
				Role:    "assistant",
				Content: "ok",
			},
			StopReason: "end_turn",
		},
	})
}

type agentsdkInProcEventRecorder struct {
	recordingHandler

	mu      sync.Mutex
	updates []SessionUpdate
}

func (r *agentsdkInProcEventRecorder) HandleSessionUpdate(ctx context.Context, update SessionUpdate) error {
	if err := r.recordingHandler.HandleSessionUpdate(ctx, update); err != nil {
		return err
	}
	r.mu.Lock()
	r.updates = append(r.updates, update)
	r.mu.Unlock()
	return nil
}

func (r *agentsdkInProcEventRecorder) countType(kind string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, update := range r.updates {
		if update.Type == kind {
			count++
		}
	}
	return count
}

func chooseAlternateModeID(state *acpproto.SessionModeState) acpproto.SessionModeId {
	if state == nil {
		return ""
	}
	for _, mode := range state.AvailableModes {
		if mode.Id != state.CurrentModeId {
			return mode.Id
		}
	}
	if len(state.AvailableModes) > 0 {
		return state.AvailableModes[0].Id
	}
	return ""
}

func chooseAlternateConfigValue(options []acpproto.SessionConfigOptionSelect) (acpproto.SessionConfigId, acpproto.SessionConfigValueId) {
	for _, option := range options {
		if option.Options.Ungrouped != nil {
			for _, value := range *option.Options.Ungrouped {
				if value.Value != option.CurrentValue {
					return option.Id, value.Value
				}
			}
		}
		if option.Options.Grouped != nil {
			for _, group := range *option.Options.Grouped {
				for _, value := range group.Options {
					if value.Value != option.CurrentValue {
						return option.Id, value.Value
					}
				}
			}
		}
	}
	return "", ""
}

func requireEventually(t *testing.T, timeout time.Duration, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

package web

import (
	"context"
	"testing"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

func TestShouldLoadPersistedChatSession(t *testing.T) {
	tests := []struct {
		name              string
		policy            acpclient.SessionPolicy
		persistedSession  string
		wantLoadPersisted bool
	}{
		{
			name:              "empty session id",
			policy:            acpclient.SessionPolicy{Reuse: true, PreferLoadSession: true},
			persistedSession:  " ",
			wantLoadPersisted: false,
		},
		{
			name:              "reuse disabled",
			policy:            acpclient.SessionPolicy{Reuse: false, PreferLoadSession: true},
			persistedSession:  "sid-old",
			wantLoadPersisted: false,
		},
		{
			name:              "prefer load disabled",
			policy:            acpclient.SessionPolicy{Reuse: true, PreferLoadSession: false},
			persistedSession:  "sid-old",
			wantLoadPersisted: false,
		},
		{
			name:              "reuse and prefer load enabled",
			policy:            acpclient.SessionPolicy{Reuse: true, PreferLoadSession: true},
			persistedSession:  "sid-old",
			wantLoadPersisted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldLoadPersistedChatSession(tt.policy, tt.persistedSession)
			if got != tt.wantLoadPersisted {
				t.Fatalf("shouldLoadPersistedChatSession() = %v, want %v", got, tt.wantLoadPersisted)
			}
		})
	}
}

func TestStartWebChatSessionSkipsLoadWhenReuseDisabled(t *testing.T) {
	client := &stubACPClient{
		loadResp: acpproto.SessionId("sid-loaded"),
		newResp:  acpproto.SessionId("sid-new"),
	}
	role := acpclient.RoleProfile{
		SessionPolicy: acpclient.SessionPolicy{
			Reuse:             false,
			PreferLoadSession: true,
		},
	}

	session, err := startWebChatSession(
		context.Background(),
		client,
		"team_leader",
		role,
		"sid-old",
		"D:/repo/demo",
		teamleader.MCPEnvConfig{},
		nil,
	)
	if err != nil {
		t.Fatalf("startWebChatSession() error = %v", err)
	}
	if string(session.SessionID) != "sid-new" {
		t.Fatalf("session id = %q, want %q", string(session.SessionID), "sid-new")
	}
	if len(client.loadReqs) != 0 {
		t.Fatalf("LoadSession calls = %d, want 0", len(client.loadReqs))
	}
	if len(client.newReqs) != 1 {
		t.Fatalf("NewSession calls = %d, want 1", len(client.newReqs))
	}
}

func TestStartWebChatSessionSkipsLoadWhenPreferLoadDisabled(t *testing.T) {
	client := &stubACPClient{
		loadResp: acpproto.SessionId("sid-loaded"),
		newResp:  acpproto.SessionId("sid-new"),
	}
	role := acpclient.RoleProfile{
		SessionPolicy: acpclient.SessionPolicy{
			Reuse:             true,
			PreferLoadSession: false,
		},
	}

	session, err := startWebChatSession(
		context.Background(),
		client,
		"team_leader",
		role,
		"sid-old",
		"D:/repo/demo",
		teamleader.MCPEnvConfig{},
		nil,
	)
	if err != nil {
		t.Fatalf("startWebChatSession() error = %v", err)
	}
	if string(session.SessionID) != "sid-new" {
		t.Fatalf("session id = %q, want %q", string(session.SessionID), "sid-new")
	}
	if len(client.loadReqs) != 0 {
		t.Fatalf("LoadSession calls = %d, want 0", len(client.loadReqs))
	}
	if len(client.newReqs) != 1 {
		t.Fatalf("NewSession calls = %d, want 1", len(client.newReqs))
	}
}

func TestACPChatAssistantTracksSessionCommandsAndConfigOptions(t *testing.T) {
	configOptions := []acpproto.SessionConfigOptionSelect{
		{
			Type:         "select",
			Id:           acpproto.SessionConfigId("model"),
			Name:         "Model",
			CurrentValue: acpproto.SessionConfigValueId("model-1"),
			Options: acpproto.SessionConfigSelectOptions{
				Ungrouped: &acpproto.SessionConfigSelectOptionsUngrouped{
					{Name: "Model 1", Value: acpproto.SessionConfigValueId("model-1")},
					{Name: "Model 2", Value: acpproto.SessionConfigValueId("model-2")},
				},
			},
		},
	}
	updatedConfigOptions := []acpproto.SessionConfigOptionSelect{
		{
			Type:         "select",
			Id:           acpproto.SessionConfigId("model"),
			Name:         "Model",
			CurrentValue: acpproto.SessionConfigValueId("model-2"),
			Options: acpproto.SessionConfigSelectOptions{
				Ungrouped: &acpproto.SessionConfigSelectOptionsUngrouped{
					{Name: "Model 1", Value: acpproto.SessionConfigValueId("model-1")},
					{Name: "Model 2", Value: acpproto.SessionConfigValueId("model-2")},
				},
			},
		},
	}
	client := &stubACPClient{
		newResult: acpclient.SessionResult{
			SessionID:     acpproto.SessionId("sid-new"),
			ConfigOptions: configOptions,
		},
		promptResp:    &acpclient.PromptResult{Text: "hello from acp"},
		setConfigResp: updatedConfigOptions,
	}
	resolver := &stubChatRoleResolver{
		agent: acpclient.AgentProfile{ID: "codex", LaunchCommand: "codex"},
		roles: map[string]acpclient.RoleProfile{
			"team_leader": {
				ID:      "team_leader",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{Reuse: true, PreferLoadSession: true},
			},
		},
	}
	factory := &recordingACPClientFactory{client: client}
	assistant := newACPChatAssistant(ACPChatAssistantDeps{
		DefaultRoleID: "team_leader",
		RoleResolver:  resolver,
		ClientFactory: factory,
	})

	got, err := assistant.Reply(context.Background(), ChatAssistantRequest{
		Message:       "hello",
		ProjectID:     "proj-1",
		ChatSessionID: "chat-1",
		WorkDir:       t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.Reply != "hello from acp" {
		t.Fatalf("reply = %q, want %q", got.Reply, "hello from acp")
	}

	opts, err := assistant.GetSessionConfigOptions("chat-1")
	if err != nil {
		t.Fatalf("GetSessionConfigOptions returned error: %v", err)
	}
	if len(opts) != 1 || opts[0].CurrentValue != acpproto.SessionConfigValueId("model-1") {
		t.Fatalf("initial config options = %#v, want model-1", opts)
	}

	ps := assistant.getPooledSession("chat-1")
	if ps == nil || ps.handler == nil {
		t.Fatal("expected pooled session handler")
	}
	ps.handler.SetSuppressEvents(true)
	if err := ps.handler.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		SessionID: "sid-new",
		Type:      "available_commands_update",
		Commands: []acpproto.AvailableCommand{
			{Name: "review", Description: "Review current changes"},
		},
	}); err != nil {
		t.Fatalf("HandleSessionUpdate(commands) error: %v", err)
	}

	commands, err := assistant.GetSessionCommands("chat-1")
	if err != nil {
		t.Fatalf("GetSessionCommands returned error: %v", err)
	}
	if len(commands) != 1 || commands[0].Name != "review" {
		t.Fatalf("commands = %#v, want review", commands)
	}

	opts, err = assistant.SetSessionConfigOption(context.Background(), "chat-1", "model", "model-2")
	if err != nil {
		t.Fatalf("SetSessionConfigOption returned error: %v", err)
	}
	if len(opts) != 1 || opts[0].CurrentValue != acpproto.SessionConfigValueId("model-2") {
		t.Fatalf("updated config options = %#v, want model-2", opts)
	}
	if len(client.setConfigReqs) != 1 {
		t.Fatalf("SetConfigOption calls = %d, want 1", len(client.setConfigReqs))
	}
}

func TestACPChatAssistantExposesSessionStateWhilePromptRunning(t *testing.T) {
	configOptions := []acpproto.SessionConfigOptionSelect{
		{
			Type:         "select",
			Id:           acpproto.SessionConfigId("model"),
			Name:         "Model",
			CurrentValue: acpproto.SessionConfigValueId("model-1"),
			Options: acpproto.SessionConfigSelectOptions{
				Ungrouped: &acpproto.SessionConfigSelectOptionsUngrouped{
					{Name: "Model 1", Value: acpproto.SessionConfigValueId("model-1")},
					{Name: "Model 2", Value: acpproto.SessionConfigValueId("model-2")},
				},
			},
		},
	}
	client := &stubACPClient{
		newResult: acpclient.SessionResult{
			SessionID:     acpproto.SessionId("sid-running"),
			ConfigOptions: configOptions,
		},
		promptResp:    &acpclient.PromptResult{Text: "hello from acp"},
		promptStarted: make(chan struct{}),
		promptRelease: make(chan struct{}),
		promptUpdates: []acpclient.SessionUpdate{
			{
				SessionID: "sid-running",
				Type:      "available_commands_update",
				Commands: []acpproto.AvailableCommand{
					{Name: "review", Description: "Review current changes"},
				},
			},
		},
	}
	resolver := &stubChatRoleResolver{
		agent: acpclient.AgentProfile{ID: "codex", LaunchCommand: "codex"},
		roles: map[string]acpclient.RoleProfile{
			"team_leader": {
				ID:      "team_leader",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{Reuse: true, PreferLoadSession: true},
			},
		},
	}
	factory := &recordingACPClientFactory{client: client}
	assistant := newACPChatAssistant(ACPChatAssistantDeps{
		DefaultRoleID: "team_leader",
		RoleResolver:  resolver,
		ClientFactory: factory,
	})

	replyDone := make(chan error, 1)
	go func() {
		_, err := assistant.Reply(context.Background(), ChatAssistantRequest{
			Message:       "hello",
			ProjectID:     "proj-1",
			ChatSessionID: "chat-1",
			WorkDir:       t.TempDir(),
		})
		replyDone <- err
	}()

	select {
	case <-client.promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("prompt did not start in time")
	}

	commands, err := assistant.GetSessionCommands("chat-1")
	if err != nil {
		t.Fatalf("GetSessionCommands while running returned error: %v", err)
	}
	if len(commands) != 1 || commands[0].Name != "review" {
		t.Fatalf("commands while running = %#v, want review", commands)
	}

	opts, err := assistant.GetSessionConfigOptions("chat-1")
	if err != nil {
		t.Fatalf("GetSessionConfigOptions while running returned error: %v", err)
	}
	if len(opts) != 1 || opts[0].CurrentValue != acpproto.SessionConfigValueId("model-1") {
		t.Fatalf("config options while running = %#v, want model-1", opts)
	}

	close(client.promptRelease)
	select {
	case err := <-replyDone:
		if err != nil {
			t.Fatalf("Reply returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Reply did not finish in time")
	}
}

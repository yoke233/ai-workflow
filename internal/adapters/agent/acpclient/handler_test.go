package acpclient

import (
	"context"
	"testing"

	acpproto "github.com/coder/acp-go-sdk"
)

func TestNopHandlerImplementsHandler(t *testing.T) {
	var h acpproto.Client = &NopHandler{}
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNopHandlerImplementsEventHandler(t *testing.T) {
	var h EventHandler = &NopHandler{}
	if h == nil {
		t.Fatal("expected non-nil event handler")
	}
}

func TestNopHandlerCallbacksReturnZeroValues(t *testing.T) {
	ctx := context.Background()
	h := &NopHandler{}

	if _, err := h.ReadTextFile(ctx, acpproto.ReadTextFileRequest{Path: "demo.txt"}); err != nil {
		t.Fatalf("ReadTextFile returned error: %v", err)
	}
	if _, err := h.WriteTextFile(ctx, acpproto.WriteTextFileRequest{Path: "demo.txt", Content: "hello"}); err != nil {
		t.Fatalf("WriteTextFile returned error: %v", err)
	}
	if _, err := h.RequestPermission(ctx, acpproto.RequestPermissionRequest{}); err != nil {
		t.Fatalf("RequestPermission returned error: %v", err)
	}
	if _, err := h.CreateTerminal(ctx, acpproto.CreateTerminalRequest{Command: "echo", Args: []string{"ok"}}); err != nil {
		t.Fatalf("CreateTerminal returned error: %v", err)
	}
	if _, err := h.KillTerminalCommand(ctx, acpproto.KillTerminalCommandRequest{TerminalId: "t1"}); err != nil {
		t.Fatalf("KillTerminalCommand returned error: %v", err)
	}
	if _, err := h.TerminalOutput(ctx, acpproto.TerminalOutputRequest{TerminalId: "t1"}); err != nil {
		t.Fatalf("TerminalOutput returned error: %v", err)
	}
	if _, err := h.ReleaseTerminal(ctx, acpproto.ReleaseTerminalRequest{TerminalId: "t1"}); err != nil {
		t.Fatalf("ReleaseTerminal returned error: %v", err)
	}
	if _, err := h.WaitForTerminalExit(ctx, acpproto.WaitForTerminalExitRequest{TerminalId: "t1"}); err != nil {
		t.Fatalf("WaitForTerminalExit returned error: %v", err)
	}
	if err := h.SessionUpdate(ctx, acpproto.SessionNotification{
		SessionId: "s1",
		Update: acpproto.SessionUpdate{
			AgentMessageChunk: &acpproto.SessionUpdateAgentMessageChunk{
				Content: acpproto.ContentBlock{Text: &acpproto.ContentBlockText{Text: "hello"}},
			},
		},
	}); err != nil {
		t.Fatalf("SessionUpdate returned error: %v", err)
	}
	if err := h.HandleSessionUpdate(ctx, SessionUpdate{SessionID: "s1", Type: "agent_message_chunk"}); err != nil {
		t.Fatalf("HandleSessionUpdate returned error: %v", err)
	}
}

package secretary

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

type recordingACPEventPublisher struct {
	mu     sync.Mutex
	events []core.Event
}

func (r *recordingACPEventPublisher) Publish(evt core.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
}

func (r *recordingACPEventPublisher) Events() []core.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]core.Event, len(r.events))
	copy(out, r.events)
	return out
}

type recordingChatRunEventRecorder struct {
	mu     sync.Mutex
	events []core.ChatRunEvent
}

func (r *recordingChatRunEventRecorder) AppendChatRunEvent(event core.ChatRunEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *recordingChatRunEventRecorder) Events() []core.ChatRunEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]core.ChatRunEvent, len(r.events))
	copy(out, r.events)
	return out
}

func TestHandleWriteFilePublishesChangedEvent(t *testing.T) {
	cwd := t.TempDir()
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(cwd, "chat-1", pub)

	req := acpproto.WriteTextFileRequest{
		Path:    "./plans/plan-a.md",
		Content: "hello secretary",
	}
	_, err := handler.WriteTextFile(context.Background(), req)
	if err != nil {
		t.Fatalf("WriteTextFile() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cwd, "plans", "plan-a.md"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(raw) != req.Content {
		t.Fatalf("written content = %q, want %q", string(raw), req.Content)
	}

	events := pub.Events()
	if len(events) != 1 {
		t.Fatalf("published events = %d, want 1", len(events))
	}
	if events[0].Type != core.EventSecretaryFilesChanged {
		t.Fatalf("event type = %q, want %q", events[0].Type, core.EventSecretaryFilesChanged)
	}
	if events[0].Data["session_id"] != "chat-1" {
		t.Fatalf("event session_id = %q, want %q", events[0].Data["session_id"], "chat-1")
	}
	if !strings.Contains(events[0].Data["file_paths"], "plans/plan-a.md") {
		t.Fatalf("event file_paths = %q, should contain %q", events[0].Data["file_paths"], "plans/plan-a.md")
	}

	sessionCtx := handler.SessionContext()
	if sessionCtx.SessionID != "chat-1" {
		t.Fatalf("session context id = %q, want %q", sessionCtx.SessionID, "chat-1")
	}
	if len(sessionCtx.ChangedFiles) != 1 || sessionCtx.ChangedFiles[0] != "plans/plan-a.md" {
		t.Fatalf("changed files = %#v, want [%q]", sessionCtx.ChangedFiles, "plans/plan-a.md")
	}
}

func TestHandleReadFileSupportsLineWindow(t *testing.T) {
	cwd := t.TempDir()
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(cwd, "chat-1", pub)

	filePath := filepath.Join(cwd, "notes.txt")
	content := "line-1\nline-2\nline-3\nline-4"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	start := 2
	limit := 2
	got, err := handler.ReadTextFile(context.Background(), acpproto.ReadTextFileRequest{
		Path:  filePath,
		Line:  &start,
		Limit: &limit,
	})
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if got.Content != "line-2\nline-3" {
		t.Fatalf("read content = %q, want %q", got.Content, "line-2\nline-3")
	}
}

func TestHandleRequestPermissionChoosesAllowOption(t *testing.T) {
	handler := NewACPHandler(t.TempDir(), "chat-1", nil)
	decision, err := handler.RequestPermission(context.Background(), acpproto.RequestPermissionRequest{
		Options: []acpproto.PermissionOption{
			{OptionId: "reject_once"},
			{OptionId: "allow_once"},
		},
	})
	if err != nil {
		t.Fatalf("RequestPermission() error = %v", err)
	}
	if decision.Outcome.Selected == nil || string(decision.Outcome.Selected.OptionId) != "allow_once" {
		t.Fatalf("decision option id = %#v, want %q", decision.Outcome.Selected, "allow_once")
	}
}

func TestHandleTerminalLifecycle(t *testing.T) {
	handler := NewACPHandler(t.TempDir(), "chat-1", nil)

	command := []string{"sh", "-c", "echo terminal-ok"}
	if runtime.GOOS == "windows" {
		command = []string{"cmd", "/C", "echo terminal-ok"}
	}
	createReq := acpproto.CreateTerminalRequest{
		Command: command[0],
		Args:    command[1:],
	}
	createReq.Cwd = &handler.cwd
	createRes, err := handler.CreateTerminal(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateTerminal() error = %v", err)
	}
	if strings.TrimSpace(createRes.TerminalId) == "" {
		t.Fatalf("expected non-empty terminal id")
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	waitRes, err := handler.WaitForTerminalExit(waitCtx, acpproto.WaitForTerminalExitRequest{
		TerminalId: createRes.TerminalId,
	})
	if err != nil {
		t.Fatalf("WaitForTerminalExit() error = %v", err)
	}
	if waitRes.ExitCode == nil || *waitRes.ExitCode != 0 {
		t.Fatalf("exit code = %#v, want 0", waitRes.ExitCode)
	}

	outputRes, err := handler.TerminalOutput(context.Background(), acpproto.TerminalOutputRequest{
		TerminalId: createRes.TerminalId,
	})
	if err != nil {
		t.Fatalf("TerminalOutput() error = %v", err)
	}
	if !strings.Contains(outputRes.Output, "terminal-ok") {
		t.Fatalf("terminal output = %q, want contains %q", outputRes.Output, "terminal-ok")
	}

	if _, err := handler.ReleaseTerminal(context.Background(), acpproto.ReleaseTerminalRequest{
		TerminalId: createRes.TerminalId,
	}); err != nil {
		t.Fatalf("ReleaseTerminal() error = %v", err)
	}
}

func TestHandleWriteFileRejectsPathOutsideScope(t *testing.T) {
	cwd := t.TempDir()
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(cwd, "chat-1", pub)

	outsidePath := filepath.Join("..", "escape.md")
	if _, err := handler.WriteTextFile(context.Background(), acpproto.WriteTextFileRequest{
		Path:    outsidePath,
		Content: "x",
	}); err == nil {
		t.Fatalf("expected out-of-scope error for path %q", outsidePath)
	}

	if len(pub.Events()) != 0 {
		t.Fatalf("no event should be published when write fails")
	}
}

func TestHandleSessionUpdatePublishesMinimalData(t *testing.T) {
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(t.TempDir(), "agent-session-1", pub)
	handler.SetProjectID("proj-1")
	handler.SetChatSessionID("chat-session-1")

	rawUpdate := `{"type":"agent_message","content":[{"type":"text","text":"hello"}]}`
	err := handler.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		SessionID:      "acp-session-fallback",
		Type:           "agent_message",
		Text:           "hello",
		Status:         "running",
		RawUpdateJSON:  rawUpdate,
		RawContentJSON: `{"text":"ignore-me"}`,
	})
	if err != nil {
		t.Fatalf("HandleSessionUpdate() error = %v", err)
	}

	events := pub.Events()
	if len(events) != 1 {
		t.Fatalf("published events = %d, want 1", len(events))
	}
	if events[0].Type != core.EventChatRunUpdate {
		t.Fatalf("event type = %q, want %q", events[0].Type, core.EventChatRunUpdate)
	}

	wantData := map[string]string{
		"session_id":       "chat-session-1",
		"agent_session_id": "agent-session-1",
		"acp_update_json":  rawUpdate,
	}
	if len(events[0].Data) != len(wantData) {
		t.Fatalf("event data size = %d, want %d, data=%#v", len(events[0].Data), len(wantData), events[0].Data)
	}
	for key, wantValue := range wantData {
		if got := events[0].Data[key]; got != wantValue {
			t.Fatalf("event data[%q] = %q, want %q", key, got, wantValue)
		}
	}

	unexpectedKeys := []string{"update_type", "text", "status", "acp_content_json"}
	for _, key := range unexpectedKeys {
		if _, ok := events[0].Data[key]; ok {
			t.Fatalf("event data should not contain %q, data=%#v", key, events[0].Data)
		}
	}
}

func TestHandleSessionUpdatePersistsNonChunkEvent(t *testing.T) {
	pub := &recordingACPEventPublisher{}
	recorder := &recordingChatRunEventRecorder{}
	handler := NewACPHandler(t.TempDir(), "agent-session-1", pub)
	handler.SetProjectID("proj-1")
	handler.SetChatSessionID("chat-session-1")
	handler.SetRunEventRecorder(recorder)

	if err := handler.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		SessionID:     "acp-session-fallback",
		Type:          "tool_call",
		Status:        "pending",
		RawUpdateJSON: `{"sessionUpdate":"tool_call","title":"Terminal","status":"pending"}`,
	}); err != nil {
		t.Fatalf("HandleSessionUpdate() error = %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("persisted events = %d, want 1", len(events))
	}
	if events[0].SessionID != "chat-session-1" || events[0].ProjectID != "proj-1" {
		t.Fatalf("unexpected persisted event identity: %#v", events[0])
	}
	if events[0].EventType != "chat_run_update" || events[0].UpdateType != "tool_call" {
		t.Fatalf("unexpected persisted event type fields: %#v", events[0])
	}
	if events[0].Payload == nil {
		t.Fatalf("expected persisted payload")
	}
	if _, ok := events[0].Payload["acp"]; !ok {
		t.Fatalf("expected payload.acp to exist, got=%#v", events[0].Payload)
	}
}

func TestHandleSessionUpdateSkipsChunkPersistence(t *testing.T) {
	pub := &recordingACPEventPublisher{}
	recorder := &recordingChatRunEventRecorder{}
	handler := NewACPHandler(t.TempDir(), "agent-session-1", pub)
	handler.SetProjectID("proj-1")
	handler.SetChatSessionID("chat-session-1")
	handler.SetRunEventRecorder(recorder)

	if err := handler.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		SessionID:     "acp-session-fallback",
		Type:          "agent_message_chunk",
		Text:          "hello",
		RawUpdateJSON: `{"sessionUpdate":"agent_message_chunk","content":{"text":"hello"}}`,
	}); err != nil {
		t.Fatalf("HandleSessionUpdate() error = %v", err)
	}

	if got := len(recorder.Events()); got != 0 {
		t.Fatalf("persisted chunk events = %d, want 0", got)
	}
}

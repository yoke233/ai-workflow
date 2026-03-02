package secretary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/user/ai-workflow/internal/acpclient"
	"github.com/user/ai-workflow/internal/core"
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

func TestHandleWriteFilePublishesChangedEvent(t *testing.T) {
	cwd := t.TempDir()
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(cwd, "chat-1", pub)

	req := acpclient.WriteFileRequest{
		Path:    "./plans/plan-a.md",
		Content: "hello secretary",
	}
	result, err := handler.HandleWriteFile(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleWriteFile() error = %v", err)
	}
	if result.BytesWritten != len([]byte(req.Content)) {
		t.Fatalf("bytes written = %d, want %d", result.BytesWritten, len([]byte(req.Content)))
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

func TestHandleWriteFileRejectsPathOutsideScope(t *testing.T) {
	cwd := t.TempDir()
	pub := &recordingACPEventPublisher{}
	handler := NewACPHandler(cwd, "chat-1", pub)

	outsidePath := filepath.Join("..", "escape.md")
	if _, err := handler.HandleWriteFile(context.Background(), acpclient.WriteFileRequest{
		Path:    outsidePath,
		Content: "x",
	}); err == nil {
		t.Fatalf("expected out-of-scope error for path %q", outsidePath)
	}

	if len(pub.Events()) != 0 {
		t.Fatalf("no event should be published when write fails")
	}
}

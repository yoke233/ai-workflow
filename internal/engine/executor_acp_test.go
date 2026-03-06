package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

// collectBus captures all published events in order.
type collectBus struct {
	mu     sync.Mutex
	events []core.Event
}

func (b *collectBus) Publish(_ context.Context, evt core.Event) error {
	b.mu.Lock()
	b.events = append(b.events, evt)
	b.mu.Unlock()
	return nil
}

func (b *collectBus) Subscribe(...core.SubOption) (*core.Subscription, error) {
	return nil, nil
}

func (b *collectBus) Close() error { return nil }

func (b *collectBus) collected() []core.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]core.Event, len(b.events))
	copy(out, b.events)
	return out
}

// persistable filters collected events the same way the real persister does.
func (b *collectBus) persistable() []core.Event {
	var out []core.Event
	for _, evt := range b.collected() {
		if evt.Type != core.EventAgentOutput {
			out = append(out, evt)
			continue
		}
		switch evt.Data["type"] {
		case "done", "agent_thought", "agent_message", "tool_call", "tool_call_completed", "usage_update":
			out = append(out, evt)
		}
	}
	return out
}

func newTestBridge(bus core.EventBus) *stageEventBridge {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	exec := &Executor{bus: bus, logger: logger}
	return &stageEventBridge{
		executor:  exec,
		runID:     "run-1",
		stage:     "implement",
		agentName: "codex",
	}
}

func sendUpdate(t *testing.T, bridge *stageEventBridge, typ, text string) {
	t.Helper()
	err := bridge.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		Type: typ,
		Text: text,
	})
	if err != nil {
		t.Fatalf("HandleSessionUpdate(%s) error: %v", typ, err)
	}
}

func sendToolCall(t *testing.T, bridge *stageEventBridge, title string) {
	t.Helper()
	raw := `{"title":"` + title + `","toolCallId":"call-1","sessionUpdate":"tool_call","status":"in_progress"}`
	err := bridge.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		Type:          "tool_call",
		RawUpdateJSON: raw,
	})
	if err != nil {
		t.Fatalf("HandleSessionUpdate(tool_call) error: %v", err)
	}
}

func sendToolCallCompleted(t *testing.T, bridge *stageEventBridge, stdout string, exitCode int) {
	t.Helper()
	raw := `{"toolCallId":"call-1","sessionUpdate":"tool_call_update","status":"completed","rawOutput":{"exit_code":` +
		intStr(exitCode) + `,"stdout":"` + stdout + `","stderr":""}}`
	err := bridge.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		Type:          "tool_call_update",
		Status:        "completed",
		RawUpdateJSON: raw,
	})
	if err != nil {
		t.Fatalf("HandleSessionUpdate(tool_call_update) error: %v", err)
	}
}

func sendUsage(t *testing.T, bridge *stageEventBridge, size, used int64) {
	t.Helper()
	raw := `{"sessionUpdate":"usage_update","size":` + intStr64(size) + `,"used":` + intStr64(used) + `}`
	err := bridge.HandleSessionUpdate(context.Background(), acpclient.SessionUpdate{
		Type:          "usage_update",
		RawUpdateJSON: raw,
	})
	if err != nil {
		t.Fatalf("HandleSessionUpdate(usage_update) error: %v", err)
	}
}

func intStr(n int) string   { return fmt.Sprintf("%d", n) }
func intStr64(n int64) string { return fmt.Sprintf("%d", n) }

// TestBridge_ThoughtFlushOnMessageChunk verifies thought is flushed
// when message chunks start, even without a tool_call in between.
func TestBridge_ThoughtFlushOnMessageChunk(t *testing.T) {
	bus := &collectBus{}
	bridge := newTestBridge(bus)

	sendUpdate(t, bridge, "agent_thought_chunk", "thinking...")
	sendUpdate(t, bridge, "agent_thought_chunk", " more thinking")
	// Now message chunks arrive — should flush thought first
	sendUpdate(t, bridge, "agent_message_chunk", "Hello")
	sendUpdate(t, bridge, "agent_message_chunk", " world")

	bridge.flushPending(context.Background())

	persisted := bus.persistable()
	if len(persisted) < 2 {
		t.Fatalf("expected ≥2 persisted events, got %d: %+v", len(persisted), typesOf(persisted))
	}

	// First persisted must be the complete thought
	if persisted[0].Data["type"] != "agent_thought" {
		t.Errorf("expected first persisted type=agent_thought, got %s", persisted[0].Data["type"])
	}
	if persisted[0].Data["content"] != "thinking... more thinking" {
		t.Errorf("thought content = %q", persisted[0].Data["content"])
	}

	// Second must be the complete message
	if persisted[1].Data["type"] != "agent_message" {
		t.Errorf("expected second persisted type=agent_message, got %s", persisted[1].Data["type"])
	}
	if persisted[1].Data["content"] != "Hello world" {
		t.Errorf("message content = %q", persisted[1].Data["content"])
	}
}

// TestBridge_ThoughtFlushOnToolCall verifies thought is flushed before tool_call.
func TestBridge_ThoughtFlushOnToolCall(t *testing.T) {
	bus := &collectBus{}
	bridge := newTestBridge(bus)

	sendUpdate(t, bridge, "agent_thought_chunk", "let me run a command")
	sendToolCall(t, bridge, "Run ls -la")

	persisted := bus.persistable()
	if len(persisted) < 2 {
		t.Fatalf("expected ≥2 persisted events, got %d: %+v", len(persisted), typesOf(persisted))
	}
	if persisted[0].Data["type"] != "agent_thought" {
		t.Errorf("expected agent_thought before tool_call, got %s", persisted[0].Data["type"])
	}
	if persisted[1].Data["type"] != "tool_call" {
		t.Errorf("expected tool_call second, got %s", persisted[1].Data["type"])
	}
	if persisted[1].Data["content"] != "Run ls -la" {
		t.Errorf("tool_call content = %q", persisted[1].Data["content"])
	}
}

// TestBridge_FullSequence simulates a real coding flow:
// thought → tool_call → tool_result → thought → message → usage
func TestBridge_FullSequence(t *testing.T) {
	bus := &collectBus{}
	bridge := newTestBridge(bus)

	// Round 1: think, then execute
	sendUpdate(t, bridge, "agent_thought_chunk", "I need to create ")
	sendUpdate(t, bridge, "agent_thought_chunk", "the file")
	sendToolCall(t, bridge, "Run touch hello.txt")
	sendToolCallCompleted(t, bridge, "ok", 0)

	// Round 2: think again, then respond
	sendUpdate(t, bridge, "agent_thought_chunk", "file created, let me confirm")
	sendUpdate(t, bridge, "agent_message_chunk", "Done! ")
	sendUpdate(t, bridge, "agent_message_chunk", "File created.")

	sendUsage(t, bridge, 258400, 10000)
	sendUsage(t, bridge, 258400, 11000)

	bridge.flushPending(context.Background())

	persisted := bus.persistable()
	expected := []string{
		"agent_thought",      // "I need to create the file"
		"tool_call",          // "Run touch hello.txt"
		"tool_call_completed", // exit_code=0
		"agent_thought",      // "file created, let me confirm"
		"agent_message",      // "Done! File created."
		"usage_update",       // size=258400, used=10000
		"usage_update",       // size=258400, used=11000
	}

	if len(persisted) != len(expected) {
		t.Fatalf("expected %d persisted events, got %d:\n%s", len(expected), len(persisted), dumpEvents(persisted))
	}
	for i, exp := range expected {
		got := persisted[i].Data["type"]
		if got != exp {
			t.Errorf("persisted[%d] type = %q, want %q\nAll: %s", i, got, exp, dumpEvents(persisted))
		}
	}

	// Verify content of specific events
	if c := persisted[0].Data["content"]; c != "I need to create the file" {
		t.Errorf("thought[0] content = %q", c)
	}
	if c := persisted[4].Data["content"]; c != "Done! File created." {
		t.Errorf("message content = %q", c)
	}
	if persisted[2].Data["exit_code"] != "0" {
		t.Errorf("tool_call_completed exit_code = %q", persisted[2].Data["exit_code"])
	}

	// Verify usage events store size/used
	if persisted[5].Data["usage_used"] != "10000" {
		t.Errorf("usage_update[0] used = %q, want 10000", persisted[5].Data["usage_used"])
	}
	if persisted[6].Data["usage_used"] != "11000" {
		t.Errorf("usage_update[1] used = %q, want 11000", persisted[6].Data["usage_used"])
	}
}

// TestBridge_FlushOnUnknownType verifies unknown types also trigger flush.
func TestBridge_FlushOnUnknownType(t *testing.T) {
	bus := &collectBus{}
	bridge := newTestBridge(bus)

	sendUpdate(t, bridge, "agent_thought_chunk", "thinking")
	// Some unknown/new ACP event type arrives
	sendUpdate(t, bridge, "some_new_acp_event", "")

	persisted := bus.persistable()
	found := false
	for _, e := range persisted {
		if e.Data["type"] == "agent_thought" && e.Data["content"] == "thinking" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected thought to be flushed on unknown event type, persisted: %s", dumpEvents(persisted))
	}
}

// TestBridge_EmptyChunksNoSpuriousEvents verifies no events for empty content.
func TestBridge_EmptyChunksNoSpuriousEvents(t *testing.T) {
	bus := &collectBus{}
	bridge := newTestBridge(bus)

	bridge.flushPending(context.Background())
	persisted := bus.persistable()
	if len(persisted) != 0 {
		t.Errorf("expected 0 persisted events from empty bridge, got %d", len(persisted))
	}
}

func typesOf(events []core.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Data["type"])
	}
	return out
}

func dumpEvents(events []core.Event) string {
	var sb strings.Builder
	for i, e := range events {
		fmt.Fprintf(&sb, "  [%d] type=%s content=%q\n", i, e.Data["type"], truncate(e.Data["content"], 60))
	}
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

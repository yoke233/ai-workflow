package core

import "testing"

func TestParseThreadStatus(t *testing.T) {
	status, err := ParseThreadStatus("active")
	if err != nil {
		t.Fatalf("parse active: %v", err)
	}
	if status != ThreadActive {
		t.Fatalf("expected active, got %q", status)
	}

	if _, err := ParseThreadStatus("broken"); err == nil {
		t.Fatal("expected invalid thread status error")
	}
}

func TestCanTransitionThreadStatus(t *testing.T) {
	if !CanTransitionThreadStatus(ThreadActive, ThreadClosed) {
		t.Fatal("expected active -> closed to be allowed")
	}
	if CanTransitionThreadStatus(ThreadArchived, ThreadActive) {
		t.Fatal("expected archived -> active to be rejected")
	}
}

func TestParseThreadAgentStatus(t *testing.T) {
	status, err := ParseThreadAgentStatus("paused")
	if err != nil {
		t.Fatalf("parse paused: %v", err)
	}
	if status != ThreadAgentPaused {
		t.Fatalf("expected paused, got %q", status)
	}

	if _, err := ParseThreadAgentStatus("broken"); err == nil {
		t.Fatal("expected invalid thread agent status error")
	}
}

func TestCanTransitionThreadAgentStatus(t *testing.T) {
	if !CanTransitionThreadAgentStatus(ThreadAgentActive, ThreadAgentPaused) {
		t.Fatal("expected active -> paused to be allowed")
	}
	if CanTransitionThreadAgentStatus(ThreadAgentLeft, ThreadAgentActive) {
		t.Fatal("expected left -> active to be rejected")
	}
}

func TestParseContextAccess(t *testing.T) {
	access, err := ParseContextAccess("check")
	if err != nil {
		t.Fatalf("parse check: %v", err)
	}
	if access != ContextAccessCheck {
		t.Fatalf("expected check, got %q", access)
	}
	if _, err := ParseContextAccess("broken"); err == nil {
		t.Fatal("expected invalid context access error")
	}
}

func TestContextAccessCapabilities(t *testing.T) {
	if !ContextAccessCheck.AllowsCheck() {
		t.Fatal("expected check access to allow checks")
	}
	if ContextAccessCheck.AllowsWrite() {
		t.Fatal("expected check access to reject writes")
	}
	if !ContextAccessWrite.AllowsWrite() {
		t.Fatal("expected write access to allow writes")
	}
}

func TestThreadFocusHelpers(t *testing.T) {
	thread := &Thread{}
	if _, ok := ReadThreadFocus(thread); ok {
		t.Fatal("expected empty thread focus to be absent")
	}

	SetThreadFocusProjectID(thread, 42)
	if got, ok := ReadThreadFocusProjectID(thread); !ok || got != 42 {
		t.Fatalf("ReadThreadFocusProjectID() = (%d, %v), want (42, true)", got, ok)
	}

	thread.Metadata["other"] = "value"
	ClearThreadFocus(thread)
	if _, ok := ReadThreadFocus(thread); ok {
		t.Fatal("expected focus to be cleared")
	}
	if thread.Metadata["other"] != "value" {
		t.Fatalf("expected unrelated metadata to remain, got %+v", thread.Metadata)
	}
}

func TestReadThreadFocusSupportsDecodedMetadata(t *testing.T) {
	thread := &Thread{
		Metadata: map[string]any{
			"focus": map[string]any{"project_id": float64(7)},
		},
	}
	focus, ok := ReadThreadFocus(thread)
	if !ok || focus == nil || focus.ProjectID != 7 {
		t.Fatalf("ReadThreadFocus() = %+v, %v", focus, ok)
	}
}

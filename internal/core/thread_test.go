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

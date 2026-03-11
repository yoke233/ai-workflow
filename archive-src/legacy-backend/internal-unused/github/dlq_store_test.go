package github

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryDLQStore_PushAndGetByDeliveryID(t *testing.T) {
	store := NewInMemoryDLQStore()

	entry := DLQEntry{
		DeliveryID:  "delivery-1",
		ProjectID:   "proj-1",
		EventType:   "issues",
		Action:      "opened",
		IssueNumber: 101,
		TraceID:     "trace-1",
		Payload:     []byte(`{"action":"opened"}`),
		LastError:   "dispatcher failed",
	}
	if err := store.Push(context.Background(), entry); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	got, err := store.GetByDeliveryID(context.Background(), "delivery-1")
	if err != nil {
		t.Fatalf("GetByDeliveryID() error = %v", err)
	}
	if got.DeliveryID != "delivery-1" {
		t.Fatalf("DeliveryID = %q, want %q", got.DeliveryID, "delivery-1")
	}
	if got.ProjectID != "proj-1" || got.EventType != "issues" || got.IssueNumber != 101 {
		t.Fatalf("unexpected entry payload: %+v", got)
	}
}

func TestInMemoryDLQStore_MarkReplayed(t *testing.T) {
	store := NewInMemoryDLQStore()

	if err := store.Push(context.Background(), DLQEntry{DeliveryID: "delivery-2"}); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	if err := store.MarkReplayed(context.Background(), "delivery-2"); err != nil {
		t.Fatalf("MarkReplayed() error = %v", err)
	}

	got, err := store.GetByDeliveryID(context.Background(), "delivery-2")
	if err != nil {
		t.Fatalf("GetByDeliveryID() error = %v", err)
	}
	if !got.Replayed {
		t.Fatal("expected entry to be marked as replayed")
	}
	if got.ReplayCount != 1 {
		t.Fatalf("ReplayCount = %d, want 1", got.ReplayCount)
	}
}

func TestInMemoryDLQStore_GetByDeliveryID_NotFound(t *testing.T) {
	store := NewInMemoryDLQStore()

	_, err := store.GetByDeliveryID(context.Background(), "missing")
	if !errors.Is(err, ErrDLQEntryNotFound) {
		t.Fatalf("expected ErrDLQEntryNotFound, got %v", err)
	}
}

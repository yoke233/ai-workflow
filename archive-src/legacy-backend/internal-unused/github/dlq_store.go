package github

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrDLQEntryNotFound = errors.New("dlq entry not found")

// DLQEntry stores one failed webhook delivery for later replay.
type DLQEntry struct {
	DeliveryID  string
	ProjectID   string
	EventType   string
	Action      string
	IssueNumber int
	TraceID     string
	Payload     []byte
	FailedAt    time.Time
	LastError   string
	ReplayCount int
	Replayed    bool
}

// DLQStore persists failed webhook deliveries.
type DLQStore interface {
	Push(context.Context, DLQEntry) error
	GetByDeliveryID(context.Context, string) (DLQEntry, error)
	MarkReplayed(context.Context, string) error
}

type inMemoryDLQStore struct {
	mu      sync.Mutex
	entries map[string]DLQEntry
}

var (
	defaultDLQStoreMu sync.RWMutex
	defaultDLQStore   DLQStore = NewInMemoryDLQStore()
)

// NewInMemoryDLQStore creates a process-local DLQ store implementation.
func NewInMemoryDLQStore() DLQStore {
	return &inMemoryDLQStore{
		entries: make(map[string]DLQEntry),
	}
}

// DefaultDLQStore returns shared webhook DLQ storage.
func DefaultDLQStore() DLQStore {
	defaultDLQStoreMu.RLock()
	defer defaultDLQStoreMu.RUnlock()
	return defaultDLQStore
}

// SetDefaultDLQStore replaces the shared DLQ store (used by tests and bootstrap wiring).
func SetDefaultDLQStore(store DLQStore) {
	if store == nil {
		store = NewInMemoryDLQStore()
	}
	defaultDLQStoreMu.Lock()
	defer defaultDLQStoreMu.Unlock()
	defaultDLQStore = store
}

func (s *inMemoryDLQStore) Push(_ context.Context, entry DLQEntry) error {
	if s == nil {
		return nil
	}
	deliveryID := strings.TrimSpace(entry.DeliveryID)
	if deliveryID == "" {
		return errors.New("dlq push requires delivery id")
	}
	if entry.FailedAt.IsZero() {
		entry.FailedAt = time.Now()
	}
	entry.DeliveryID = deliveryID
	entry.Payload = append([]byte(nil), entry.Payload...)

	s.mu.Lock()
	defer s.mu.Unlock()
	existing, exists := s.entries[deliveryID]
	if exists {
		entry.ReplayCount = existing.ReplayCount
		entry.Replayed = existing.Replayed
	}
	s.entries[deliveryID] = entry
	return nil
}

func (s *inMemoryDLQStore) GetByDeliveryID(_ context.Context, deliveryID string) (DLQEntry, error) {
	if s == nil {
		return DLQEntry{}, ErrDLQEntryNotFound
	}
	key := strings.TrimSpace(deliveryID)
	if key == "" {
		return DLQEntry{}, ErrDLQEntryNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return DLQEntry{}, ErrDLQEntryNotFound
	}
	entry.Payload = append([]byte(nil), entry.Payload...)
	return entry, nil
}

func (s *inMemoryDLQStore) MarkReplayed(_ context.Context, deliveryID string) error {
	if s == nil {
		return ErrDLQEntryNotFound
	}
	key := strings.TrimSpace(deliveryID)
	if key == "" {
		return ErrDLQEntryNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return ErrDLQEntryNotFound
	}
	entry.Replayed = true
	entry.ReplayCount++
	s.entries[key] = entry
	return nil
}

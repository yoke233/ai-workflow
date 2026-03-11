package memory

import (
	"context"
	"sync"

	"github.com/yoke233/ai-workflow/internal/core"
)

// Bus is an in-memory channel-based EventBus implementation.
type Bus struct {
	mu   sync.RWMutex
	subs []*sub
}

type sub struct {
	types  map[core.EventType]struct{}
	ch     chan core.Event
	cancel func()
	done   bool
}

// NewBus creates a new in-memory EventBus.
func NewBus() *Bus {
	return &Bus{}
}

// Publish sends an event to all matching subscribers.
func (b *Bus) Publish(_ context.Context, event core.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subs {
		if sub.done {
			continue
		}
		if len(sub.types) > 0 {
			if _, ok := sub.types[event.Type]; !ok {
				continue
			}
		}
		select {
		case sub.ch <- event:
		default:
		}
	}
}

// Subscribe creates a new subscription. If opts.Types is empty, all events are received.
func (b *Bus) Subscribe(opts core.SubscribeOpts) *core.Subscription {
	bufSize := opts.BufferSize
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan core.Event, bufSize)

	types := make(map[core.EventType]struct{}, len(opts.Types))
	for _, t := range opts.Types {
		types[t] = struct{}{}
	}

	sub := &sub{
		types: types,
		ch:    ch,
	}

	b.mu.Lock()
	sub.cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		sub.done = true
		close(ch)
	}
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	return &core.Subscription{
		C:      ch,
		Cancel: sub.cancel,
	}
}

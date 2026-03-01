package eventbus

import (
	"sync"

	"github.com/user/ai-workflow/internal/core"
)

type Bus struct {
	mu   sync.RWMutex
	subs map[chan core.Event]struct{}
}

var (
	defaultBusMu sync.RWMutex
	defaultBus   *Bus
)

func New() *Bus {
	bus := &Bus{subs: make(map[chan core.Event]struct{})}

	defaultBusMu.Lock()
	if defaultBus == nil {
		defaultBus = bus
	}
	defaultBusMu.Unlock()

	return bus
}

func (b *Bus) Subscribe() chan core.Event {
	ch := make(chan core.Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan core.Event) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *Bus) Publish(evt core.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
}

// Default returns the process-wide default event bus.
func Default() *Bus {
	defaultBusMu.RLock()
	defer defaultBusMu.RUnlock()
	return defaultBus
}

// SetDefault overrides the process-wide default event bus.
func SetDefault(bus *Bus) {
	defaultBusMu.Lock()
	defer defaultBusMu.Unlock()
	defaultBus = bus
}

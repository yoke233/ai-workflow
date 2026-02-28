package eventbus

import (
	"sync"

	"github.com/user/ai-workflow/internal/core"
)

type Bus struct {
	mu   sync.RWMutex
	subs map[chan core.Event]struct{}
}

func New() *Bus {
	return &Bus{subs: make(map[chan core.Event]struct{})}
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

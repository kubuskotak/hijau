package server

import "sync"

// broker is a tiny in-process pub/sub for live updates: SSE connections
// subscribe per project, and translation writes publish to them. Sends are
// non-blocking (a slow subscriber drops events rather than stalling writers).
type broker struct {
	mu   sync.Mutex
	subs map[string]map[chan any]struct{} // projectID -> subscriber channels
}

func newBroker() *broker {
	return &broker{subs: make(map[string]map[chan any]struct{})}
}

func (b *broker) subscribe(projectID string) (<-chan any, func()) {
	ch := make(chan any, 16)
	b.mu.Lock()
	if b.subs[projectID] == nil {
		b.subs[projectID] = make(map[chan any]struct{})
	}
	b.subs[projectID][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			if m := b.subs[projectID]; m != nil {
				delete(m, ch)
				if len(m) == 0 {
					delete(b.subs, projectID)
				}
			}
			b.mu.Unlock()
			close(ch)
		})
	}
	return ch, unsubscribe
}

func (b *broker) publish(projectID string, msg any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[projectID] {
		select {
		case ch <- msg:
		default: // subscriber buffer full — drop rather than block the writer
		}
	}
}

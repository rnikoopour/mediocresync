package sse

import "sync"

const subBufferSize = 50

// Broker fan-outs events from active sync runs to zero or more HTTP clients.
// Publish is non-blocking: a slow or disconnected subscriber drops events
// rather than stalling the sync engine.
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]chan Event // key: runID
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[string][]chan Event)}
}

// Subscribe registers a listener for the given runID. The returned channel
// receives events until the run finishes or the caller invokes the returned
// unsubscribe function.
func (b *Broker) Subscribe(runID string) (<-chan Event, func()) {
	ch := make(chan Event, subBufferSize)

	b.mu.Lock()
	b.subs[runID] = append(b.subs[runID], ch)
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		chans := b.subs[runID]
		for i, c := range chans {
			if c == ch {
				b.subs[runID] = append(chans[:i], chans[i+1:]...)
				break
			}
		}
	}
	return ch, unsub
}

// Publish sends an event to all current subscribers for the run.
// Non-blocking: full subscriber channels are skipped.
func (b *Broker) Publish(runID string, ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[runID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close signals all subscribers for a run that the run is finished by
// closing their channels, then removes the run from the broker.
func (b *Broker) Close(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[runID] {
		close(ch)
	}
	delete(b.subs, runID)
}

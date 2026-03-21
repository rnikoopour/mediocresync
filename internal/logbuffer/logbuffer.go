package logbuffer

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const DefaultSize = 500

// Entry is a captured log record.
type Entry struct {
	Time  time.Time      `json:"time"`
	Level string         `json:"level"`
	Msg   string         `json:"msg"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// state holds the shared ring buffer and subscriber set used by all
// derived handlers (via WithAttrs / WithGroup).
type state struct {
	mu      sync.Mutex
	entries []Entry
	size    int
	head    int
	count   int
	subs    map[chan Entry]struct{}
}

func (s *state) append(e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[s.head] = e
	s.head = (s.head + 1) % s.size
	if s.count < s.size {
		s.count++
	}
	for ch := range s.subs {
		select {
		case ch <- e:
		default: // drop if subscriber is slow
		}
	}
}

func (s *state) snapshot() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count == 0 {
		return nil
	}
	out := make([]Entry, s.count)
	start := (s.head - s.count + s.size) % s.size
	for i := range s.count {
		out[i] = s.entries[(start+i)%s.size]
	}
	return out
}

func (s *state) subscribe() (<-chan Entry, func()) {
	ch := make(chan Entry, 64)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
		for len(ch) > 0 {
			<-ch
		}
	}
}

// Buffer is a slog.Handler that captures records into a fixed-size ring
// buffer and fans them out to live subscribers, while also forwarding
// every record to a wrapped handler (e.g. os.Stderr text output).
type Buffer struct {
	st      *state
	wrapped slog.Handler
	attrs   []slog.Attr
}

// New creates a Buffer with the given capacity, wrapping wrapped for
// actual log output.
func New(size int, wrapped slog.Handler) *Buffer {
	return &Buffer{
		st: &state{
			size:    size,
			entries: make([]Entry, size),
			subs:    make(map[chan Entry]struct{}),
		},
		wrapped: wrapped,
	}
}

// Entries returns a snapshot of all buffered entries in chronological order.
func (b *Buffer) Entries() []Entry { return b.st.snapshot() }

// Subscribe returns a channel that receives new log entries and a cleanup
// function to unsubscribe. The channel is buffered; slow consumers may drop.
func (b *Buffer) Subscribe() (<-chan Entry, func()) { return b.st.subscribe() }

// slog.Handler interface

func (b *Buffer) Enabled(ctx context.Context, level slog.Level) bool {
	return b.wrapped.Enabled(ctx, level)
}

func jsonValue(v slog.Value) any {
	a := v.Any()
	if err, ok := a.(error); ok {
		return err.Error()
	}
	return a
}

func (b *Buffer) Handle(ctx context.Context, r slog.Record) error {
	entry := Entry{
		Time:  r.Time,
		Level: r.Level.String(),
		Msg:   r.Message,
	}
	total := len(b.attrs) + r.NumAttrs()
	if total > 0 {
		attrs := make(map[string]any, total)
		for _, a := range b.attrs {
			attrs[a.Key] = jsonValue(a.Value)
		}
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = jsonValue(a.Value)
			return true
		})
		entry.Attrs = attrs
	}
	b.st.append(entry)
	return b.wrapped.Handle(ctx, r)
}

func (b *Buffer) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(b.attrs)+len(attrs))
	copy(combined, b.attrs)
	copy(combined[len(b.attrs):], attrs)
	return &Buffer{st: b.st, wrapped: b.wrapped.WithAttrs(attrs), attrs: combined}
}

func (b *Buffer) WithGroup(name string) slog.Handler {
	return &Buffer{st: b.st, wrapped: b.wrapped.WithGroup(name), attrs: b.attrs}
}

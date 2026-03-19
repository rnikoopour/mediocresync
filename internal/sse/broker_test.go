package sse

import (
	"testing"
	"time"
)

func TestSubscribeReceive(t *testing.T) {
	b := NewBroker()
	ch, unsub := b.Subscribe("run-1")
	defer unsub()

	ev := Event{RunID: "run-1", Status: "in_progress"}
	b.Publish("run-1", ev)

	select {
	case got := <-ch:
		if got.Status != "in_progress" {
			t.Errorf("got status %q, want in_progress", got.Status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	b := NewBroker()
	ch, unsub := b.Subscribe("run-2")
	unsub()

	b.Publish("run-2", Event{RunID: "run-2", Status: "done"})

	select {
	case <-ch:
		t.Error("should not receive after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestCloseSignalsSubscribers(t *testing.T) {
	b := NewBroker()
	ch, _ := b.Subscribe("run-3")

	b.Close("run-3")

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed, not have a value")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for channel close")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := NewBroker()
	ch1, unsub1 := b.Subscribe("run-4")
	ch2, unsub2 := b.Subscribe("run-4")
	defer unsub1()
	defer unsub2()

	b.Publish("run-4", Event{RunID: "run-4", Status: "done"})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d timed out", i+1)
		}
	}
}

func TestPublishNonBlockingOnFullBuffer(t *testing.T) {
	b := NewBroker()
	ch, unsub := b.Subscribe("run-5")
	defer unsub()

	// Fill the buffer without reading
	for i := range subBufferSize + 10 {
		b.Publish("run-5", Event{RunID: "run-5", Status: "in_progress", BytesXferred: int64(i)})
	}

	// Drain what made it in
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			if drained != subBufferSize {
				t.Errorf("drained %d events, want %d (buffer size)", drained, subBufferSize)
			}
			return
		}
	}
}

func TestPublishToUnknownRunIsNoop(t *testing.T) {
	b := NewBroker()
	// Should not panic
	b.Publish("nonexistent-run", Event{Status: "done"})
}

package pubsub

import (
	"sync"
	"testing"
	"time"
)

func TestHub_FanOutToSameOwner(t *testing.T) {
	h := NewHub(4)
	a := h.Subscribe("u-1")
	b := h.Subscribe("u-1")
	t.Cleanup(func() {
		a.Close()
		b.Close()
	})

	delivered, dropped := h.Publish(Event{OwnerID: "u-1", DocumentID: "d-1", Status: "ready"})
	if delivered != 2 || dropped != 0 {
		t.Fatalf("publish counts: got delivered=%d dropped=%d, want 2/0", delivered, dropped)
	}
	want := Event{OwnerID: "u-1", DocumentID: "d-1", Status: "ready"}
	mustReceive(t, a.C, want)
	mustReceive(t, b.C, want)
}

func TestHub_OwnerIsolation(t *testing.T) {
	h := NewHub(4)
	mine := h.Subscribe("u-1")
	other := h.Subscribe("u-2")
	t.Cleanup(func() {
		mine.Close()
		other.Close()
	})

	h.Publish(Event{OwnerID: "u-1", DocumentID: "d-1", Status: "queued"})

	mustReceive(t, mine.C, Event{OwnerID: "u-1", DocumentID: "d-1", Status: "queued"})
	select {
	case ev := <-other.C:
		t.Fatalf("other owner received event: %+v", ev)
	case <-time.After(20 * time.Millisecond):
		// expected: no leak across owners
	}
}

func TestHub_DropsOnFullChannel(t *testing.T) {
	h := NewHub(1)
	sub := h.Subscribe("u-1")
	t.Cleanup(sub.Close)

	// Fill, then publish two more — should drop both.
	if d, dr := h.Publish(Event{OwnerID: "u-1", Status: "queued"}); d != 1 || dr != 0 {
		t.Fatalf("first publish: got d=%d dr=%d, want 1/0", d, dr)
	}
	if d, dr := h.Publish(Event{OwnerID: "u-1", Status: "extracting"}); d != 0 || dr != 1 {
		t.Fatalf("second publish: got d=%d dr=%d, want 0/1", d, dr)
	}
	if d, dr := h.Publish(Event{OwnerID: "u-1", Status: "ready"}); d != 0 || dr != 1 {
		t.Fatalf("third publish: got d=%d dr=%d, want 0/1", d, dr)
	}
}

func TestHub_CloseRemovesSubscription(t *testing.T) {
	h := NewHub(4)
	sub := h.Subscribe("u-1")
	if got := h.Subscribers("u-1"); got != 1 {
		t.Fatalf("subscribers before close: got %d, want 1", got)
	}
	sub.Close()
	if got := h.Subscribers("u-1"); got != 0 {
		t.Fatalf("subscribers after close: got %d, want 0", got)
	}
	// Re-close is a no-op.
	sub.Close()

	// Publish to no-one is harmless.
	if d, dr := h.Publish(Event{OwnerID: "u-1", Status: "ready"}); d != 0 || dr != 0 {
		t.Fatalf("publish after close: got d=%d dr=%d, want 0/0", d, dr)
	}
}

func TestHub_ConcurrentSubscribePublish(t *testing.T) {
	h := NewHub(64)
	const N = 32
	var wg sync.WaitGroup
	wg.Add(N)
	subs := make([]*Subscription, N)
	for i := 0; i < N; i++ {
		subs[i] = h.Subscribe("u-1")
	}
	defer func() {
		for _, s := range subs {
			s.Close()
		}
	}()

	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			h.Publish(Event{OwnerID: "u-1", Status: "queued"})
		}()
	}
	wg.Wait()

	// We don't assert exact counts because publishes race: each
	// subscriber sees between 1 and N events depending on timing.
	// The contract is: no panics and no corruption.
}

func mustReceive(t *testing.T, c <-chan Event, want Event) {
	t.Helper()
	select {
	case got := <-c:
		if got != want {
			t.Fatalf("event mismatch: got %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event %+v", want)
	}
}

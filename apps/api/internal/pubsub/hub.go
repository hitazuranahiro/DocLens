package pubsub

import (
	"sync"
)

// Hub is the in-process fanout for document_status events.
//
// Subscribers are keyed by ownerID. Each subscription has a buffered
// channel; a slow consumer is dropped rather than allowed to block
// the listener goroutine.
//
// Hub is safe for concurrent use.
type Hub struct {
	// bufferPerSub is the size of each subscriber's channel. Picked so
	// a temporary stall (network blip, paused tab) doesn't drop events
	// for normal status churn (a few transitions per document).
	bufferPerSub int

	mu          sync.RWMutex
	subscribers map[string]map[*Subscription]struct{}
}

// NewHub returns a Hub with a per-subscriber channel buffer of `buf`.
// Pass 0 to use the default of 16.
func NewHub(buf int) *Hub {
	if buf <= 0 {
		buf = 16
	}
	return &Hub{
		bufferPerSub: buf,
		subscribers:  make(map[string]map[*Subscription]struct{}),
	}
}

// Subscription is a single SSE connection's view of the hub.
//
// The HTTP handler reads from C until the request context is
// cancelled, then calls Close.
type Subscription struct {
	OwnerID string
	C       chan Event

	hub    *Hub
	once   sync.Once
	closed chan struct{}
}

// Subscribe registers a subscription scoped to ownerID. The caller
// must call Close (or its deferred wrapper) when done.
func (h *Hub) Subscribe(ownerID string) *Subscription {
	sub := &Subscription{
		OwnerID: ownerID,
		C:       make(chan Event, h.bufferPerSub),
		hub:     h,
		closed:  make(chan struct{}),
	}
	h.mu.Lock()
	if h.subscribers[ownerID] == nil {
		h.subscribers[ownerID] = make(map[*Subscription]struct{})
	}
	h.subscribers[ownerID][sub] = struct{}{}
	h.mu.Unlock()
	return sub
}

// Close removes the subscription from the hub and closes its channel.
// Safe to call more than once.
func (s *Subscription) Close() {
	s.once.Do(func() {
		s.hub.mu.Lock()
		if set, ok := s.hub.subscribers[s.OwnerID]; ok {
			delete(set, s)
			if len(set) == 0 {
				delete(s.hub.subscribers, s.OwnerID)
			}
		}
		s.hub.mu.Unlock()
		close(s.closed)
		close(s.C)
	})
}

// Publish fans out one event to every subscriber matching ev.OwnerID.
//
// Returns (delivered, dropped) so the caller can emit a metric. A
// drop happens when the subscriber channel is full — we never block
// the producer (which is the single Postgres listener goroutine).
func (h *Hub) Publish(ev Event) (delivered, dropped int) {
	if ev.OwnerID == "" {
		return 0, 0
	}
	h.mu.RLock()
	set := h.subscribers[ev.OwnerID]
	// Snapshot under the read lock so subscribers cannot be Closed
	// mid-iteration; channel sends below are non-blocking so a
	// concurrent Close that closes a channel will only race a
	// best-effort drop, which is fine.
	subs := make([]*Subscription, 0, len(set))
	for s := range set {
		subs = append(subs, s)
	}
	h.mu.RUnlock()

	for _, s := range subs {
		select {
		case s.C <- ev:
			delivered++
		default:
			dropped++
		}
	}
	return delivered, dropped
}

// Subscribers returns the number of active subscriptions for ownerID.
// Used by tests; no production callers today.
func (h *Hub) Subscribers(ownerID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[ownerID])
}

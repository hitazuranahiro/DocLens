// Package inmem provides a JobBus adapter that records calls in
// memory rather than enqueueing to a real broker.
//
// It is the default in tests and dev environments where Redis is
// unavailable; callers can also use it to write deterministic
// integration tests for code that enqueues jobs.
package inmem

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/shared/jobs"
)

// Bus is an in-memory JobBus. Safe for concurrent use.
type Bus struct {
	mu        sync.Mutex
	enqueued  []jobs.Task
	failNext  bool
	failError error
	uniqueIDs map[string]struct{}
}

// NewBus returns an empty Bus.
func NewBus() *Bus { return &Bus{uniqueIDs: make(map[string]struct{})} }

// Enqueue records the task and returns a receipt with a synthetic ID.
// Honors UniqueKey by returning jobs.ErrDuplicate on second use.
func (b *Bus) Enqueue(_ context.Context, t jobs.Task) (*jobs.Receipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failNext {
		b.failNext = false
		err := b.failError
		if err == nil {
			err = errors.New("inmem: forced failure")
		}
		return nil, err
	}
	if t.UniqueKey != "" {
		if _, dup := b.uniqueIDs[t.UniqueKey]; dup {
			return nil, jobs.ErrDuplicate
		}
		b.uniqueIDs[t.UniqueKey] = struct{}{}
	}
	b.enqueued = append(b.enqueued, t)
	queue := t.Queue
	if queue == "" {
		queue = "default"
	}
	return &jobs.Receipt{
		TaskID: uuid.NewString(),
		Queue:  queue,
	}, nil
}

// Tasks returns a snapshot of every enqueued task in order.
func (b *Bus) Tasks() []jobs.Task {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]jobs.Task, len(b.enqueued))
	copy(out, b.enqueued)
	return out
}

// FailOnce makes the next Enqueue return err. Useful for testing
// caller error handling.
func (b *Bus) FailOnce(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failNext = true
	b.failError = err
}

// Package jobs defines the JobBus port that bounded contexts use to
// enqueue background work. The asynq adapter lives in jobs/asynq;
// tests use jobs/inmem.
//
// Per ADR 0005, handlers (and thus the API) must not import asynq
// directly. Engine-specific concerns (queue names, priority weights,
// retry curves) live in the adapter; the port stays neutral so a
// future migration to River or another backend doesn't ripple
// through every caller.
package jobs

import (
	"context"
	"errors"
	"time"
)

// Task is everything the bus needs to enqueue one job.
//
// Payload is JSON-serialized by the adapter. Keep it small: queue
// brokers limit message size, and our payloads are typed pointers
// (e.g. just a documentId), not whole document bodies.
type Task struct {
	// Type is the task name, e.g. "extract.document". Workers
	// register handlers by Type.
	Type string

	// Payload is the JSON-serializable arguments for the task.
	Payload any

	// Queue selects the named asynq queue. Empty means "default".
	// Use this to give small documents priority over large ones.
	Queue string

	// MaxRetries is the retry ceiling. 0 means use the bus default
	// (typically "no retries" or whatever the adapter ships with).
	MaxRetries int

	// Timeout caps a single attempt. Zero = adapter default.
	Timeout time.Duration

	// Deadline, when non-zero, is a hard stop independent of retries.
	Deadline time.Time

	// UniqueKey, when non-empty, deduplicates pending tasks. Asynq's
	// implementation is based on Redis SETNX; the adapter docs which
	// states are deduped against.
	UniqueKey string

	// UniqueTTL bounds how long the dedupe lock holds. Required when
	// UniqueKey is set.
	UniqueTTL time.Duration
}

// Receipt is what the bus returns after a successful enqueue.
type Receipt struct {
	// TaskID is the bus-assigned ID. Used by ops dashboards and
	// reflected on extraction.jobs.task_id so we can correlate.
	TaskID string

	// Queue is the queue the task landed in.
	Queue string
}

// JobBus is the interface every adapter implements.
//
// Enqueue MUST be safe to call concurrently. Adapters may but are not
// required to provide stronger guarantees (idempotent enqueue under
// network partitions, etc.).
type JobBus interface {
	Enqueue(ctx context.Context, t Task) (*Receipt, error)
}

// ErrDuplicate is the canonical error adapters return when a unique
// task collision was rejected.
var ErrDuplicate = errors.New("jobs: duplicate task")

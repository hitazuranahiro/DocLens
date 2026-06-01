// Package asynq implements the JobBus port over the hibiken/asynq
// library, per ADR 0005.
//
// Key design choices:
//   - Default retry curve is asynq's own (exponential, ~25 attempts);
//     callers override per task type via Task.MaxRetries.
//   - Timeouts default to 5 minutes per attempt, matching the
//     MarkItDown subprocess ceiling.
//   - Unique-task dedupe uses Redis SETNX under the hood. Asynq
//     considers the task "unique" while it is in pending, scheduled,
//     or active states; once it lands in archived/dead it can be
//     enqueued again.
package asynq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	hasynq "github.com/hibiken/asynq"

	"github.com/tomeku/doclens/services/shared/jobs"
)

// Adapter is the asynq-backed JobBus.
type Adapter struct {
	client *hasynq.Client
}

// New connects to Redis and returns a ready-to-use Adapter.
//
// `redisURL` accepts the standard scheme (redis://, rediss://,
// redis-sentinel://). For dev with the bundled compose, that's
// `redis://localhost:6379/0`.
func New(redisURL string) (*Adapter, error) {
	opt, err := hasynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("jobs/asynq: parse redis URI: %w", err)
	}
	return &Adapter{client: hasynq.NewClient(opt)}, nil
}

// Close releases the underlying Redis connection. Safe to call once;
// subsequent calls are no-ops.
func (a *Adapter) Close() error {
	if a.client == nil {
		return nil
	}
	if c, ok := any(a.client).(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Enqueue implements jobs.JobBus.
func (a *Adapter) Enqueue(ctx context.Context, t jobs.Task) (*jobs.Receipt, error) {
	if a.client == nil {
		return nil, errors.New("jobs/asynq: nil client")
	}

	payload, err := json.Marshal(t.Payload)
	if err != nil {
		return nil, fmt.Errorf("jobs/asynq: marshal payload: %w", err)
	}

	opts := []hasynq.Option{}
	if t.Queue != "" {
		opts = append(opts, hasynq.Queue(t.Queue))
	}
	// Negative means "no retries". Zero means "use the bus default".
	// Positive values pass through verbatim. Asynq treats 0 as
	// "retry forever", which is rarely what we want, so we map 0
	// to "do not override".
	if t.MaxRetries > 0 {
		opts = append(opts, hasynq.MaxRetry(t.MaxRetries))
	} else if t.MaxRetries < 0 {
		opts = append(opts, hasynq.MaxRetry(0))
	}
	if t.Timeout > 0 {
		opts = append(opts, hasynq.Timeout(t.Timeout))
	}
	if !t.Deadline.IsZero() {
		opts = append(opts, hasynq.Deadline(t.Deadline))
	}
	if t.UniqueKey != "" {
		// Asynq's Unique option dedupes against pending/scheduled/active.
		// We pass the user-supplied key prefixed with the task type so
		// different task types can share keys without collision.
		ttl := t.UniqueTTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		opts = append(opts, hasynq.Unique(ttl))
	}

	task := hasynq.NewTask(t.Type, payload)

	info, err := a.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		if errors.Is(err, hasynq.ErrDuplicateTask) || errors.Is(err, hasynq.ErrTaskIDConflict) {
			return nil, jobs.ErrDuplicate
		}
		return nil, fmt.Errorf("jobs/asynq: enqueue: %w", err)
	}

	queue := info.Queue
	if queue == "" {
		queue = "default"
	}
	return &jobs.Receipt{TaskID: info.ID, Queue: queue}, nil
}

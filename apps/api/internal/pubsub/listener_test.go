package pubsub

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestListener_DispatchesNotificationToHub(t *testing.T) {
	hub := NewHub(4)
	sub := hub.Subscribe("u-1")
	t.Cleanup(sub.Close)

	conn := &fakeConn{
		notifications: []*pgconn.Notification{
			{Channel: Channel, Payload: `{"event":"INSERT","owner_id":"u-1","document_id":"d-1","status":"queued","updated_at":"2026-06-01T10:30:00.000000Z"}`},
		},
	}
	connector := func(ctx context.Context) (pgConn, error) { return conn, nil }
	l := NewListener(connector, hub, Options{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		MinBackoff: time.Millisecond,
		MaxBackoff: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	select {
	case ev := <-sub.C:
		if ev.OwnerID != "u-1" || ev.DocumentID != "d-1" || ev.Status != "queued" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for event")
	}

	if got := conn.listenCount.Load(); got == 0 {
		t.Fatalf("expected LISTEN to be issued")
	}
}

func TestListener_ReconnectsOnError(t *testing.T) {
	hub := NewHub(4)

	var calls atomic.Int32
	connector := func(ctx context.Context) (pgConn, error) {
		n := calls.Add(1)
		if n == 1 {
			return nil, errors.New("first connect fails")
		}
		// Second attempt: deliver one notification then return EOF
		// to force another reconnect cycle.
		return &fakeConn{
			notifications: []*pgconn.Notification{
				{Channel: Channel, Payload: `{"owner_id":"u-1","document_id":"d-1","status":"ready","updated_at":"2026-06-01T10:30:00.000000Z"}`},
			},
		}, nil
	}
	l := NewListener(connector, hub, Options{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		MinBackoff: time.Millisecond,
		MaxBackoff: 5 * time.Millisecond,
	})

	sub := hub.Subscribe("u-1")
	t.Cleanup(sub.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	select {
	case ev := <-sub.C:
		if ev.Status != "ready" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-ctx.Done():
		t.Fatalf("listener did not recover; calls=%d", calls.Load())
	}
	if calls.Load() < 2 {
		t.Fatalf("expected listener to retry connect at least twice; got %d", calls.Load())
	}
}

func TestListener_IgnoresMalformedPayload(t *testing.T) {
	hub := NewHub(4)
	sub := hub.Subscribe("u-1")
	t.Cleanup(sub.Close)

	conn := &fakeConn{
		notifications: []*pgconn.Notification{
			{Channel: Channel, Payload: "not json"},
			{Channel: Channel, Payload: `{"owner_id":"u-1","document_id":"d-1","status":"queued","updated_at":"2026-06-01T10:30:00.000000Z"}`},
		},
	}
	connector := func(ctx context.Context) (pgConn, error) { return conn, nil }
	l := NewListener(connector, hub, Options{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		MinBackoff: time.Millisecond,
		MaxBackoff: 5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	select {
	case ev := <-sub.C:
		if ev.Status != "queued" {
			t.Fatalf("expected the well-formed event; got %+v", ev)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for well-formed event")
	}
}

// --- fake -----------------------------------------------------------------

type fakeConn struct {
	notifications []*pgconn.Notification
	idx           int

	listenCount atomic.Int32
	closed      atomic.Bool
}

func (f *fakeConn) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if len(sql) >= 6 && sql[:6] == "LISTEN" {
		f.listenCount.Add(1)
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeConn) WaitForNotification(ctx context.Context) (*pgconn.Notification, error) {
	if f.idx < len(f.notifications) {
		n := f.notifications[f.idx]
		f.idx++
		return n, nil
	}
	// Block on ctx so the test can cancel cleanly.
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeConn) Close(_ context.Context) error {
	f.closed.Store(true)
	return nil
}

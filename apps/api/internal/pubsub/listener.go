package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Channel is the Postgres NOTIFY channel the trigger writes to.
// Migration 0003 owns this string; keep them in sync.
const Channel = "document_status"

// Listener owns a single dedicated pgx.Conn (not a pool — LISTEN
// state is per-connection) and forwards NOTIFY payloads to the Hub.
//
// Run is a long-lived goroutine. On any error other than ctx
// cancellation it backs off and reconnects.
type Listener struct {
	connect Connector
	hub     *Hub
	logger  *slog.Logger

	// Reconnect backoff bounds. Short floor so a one-off blip
	// reconnects in roughly the next heartbeat tick; ceiling keeps
	// us from spinning hot when Postgres stays down.
	minBackoff time.Duration
	maxBackoff time.Duration
}

// Connector is the dependency the listener uses to acquire a fresh
// pgx.Conn. The production wiring passes a closure that calls
// pgx.Connect(ctx, databaseURL); tests pass a fake.
type Connector func(ctx context.Context) (pgConn, error)

// pgConn is the narrow set of pgx.Conn methods we use. Letting tests
// inject a fake without pulling in testcontainers.
type pgConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	WaitForNotification(ctx context.Context) (*pgconn.Notification, error)
	Close(ctx context.Context) error
}

// Options bundles tuneables that aren't required at construction.
type Options struct {
	Logger     *slog.Logger
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

// NewListener returns a Listener wired to hub via connect.
func NewListener(connect Connector, hub *Hub, opts Options) *Listener {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	min := opts.MinBackoff
	if min <= 0 {
		min = 250 * time.Millisecond
	}
	max := opts.MaxBackoff
	if max <= 0 {
		max = 10 * time.Second
	}
	return &Listener{
		connect:    connect,
		hub:        hub,
		logger:     logger,
		minBackoff: min,
		maxBackoff: max,
	}
}

// Run blocks until ctx is cancelled, reconnecting on transient errors
// with exponential backoff capped at maxBackoff. Returns ctx.Err()
// when shutting down, or the underlying error if the connector keeps
// failing (caller may use this for a fail-fast bootstrap mode).
func (l *Listener) Run(ctx context.Context) error {
	backoff := l.minBackoff
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := l.runOnce(ctx)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ctx.Err()
		}

		l.logger.Warn("pubsub: listener disconnected; reconnecting",
			"err", err,
			"backoff", backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > l.maxBackoff {
			backoff = l.maxBackoff
		}
	}
}

// runOnce establishes a connection, issues LISTEN, then loops
// receiving notifications. Returns nil on graceful ctx shutdown,
// non-nil otherwise (Run will reconnect on non-nil).
func (l *Listener) runOnce(ctx context.Context) error {
	conn, err := l.connect(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+Channel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	l.logger.Info("pubsub: listening", "channel", Channel)

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if n == nil || n.Channel != Channel {
			continue
		}

		var ev Event
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil {
			l.logger.Warn("pubsub: malformed payload",
				"err", err,
				"payload_len", len(n.Payload))
			continue
		}
		delivered, dropped := l.hub.Publish(ev)
		if dropped > 0 {
			l.logger.Warn("pubsub: dropped events on slow subscriber",
				"owner_id", ev.OwnerID,
				"document_id", ev.DocumentID,
				"delivered", delivered,
				"dropped", dropped)
		}
	}
}

// PgxConnector returns a Connector that opens a single pgx.Conn from
// the given database URL. The returned Connector is the production
// wiring used by main.go; the resulting connection is dedicated to
// the Listener (LISTEN state is per-connection and not safe to share
// with the application's pgxpool).
func PgxConnector(databaseURL string) Connector {
	return func(ctx context.Context) (pgConn, error) {
		c, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			return nil, err
		}
		return &pgxConnAdapter{c: c}, nil
	}
}

type pgxConnAdapter struct{ c *pgx.Conn }

func (a *pgxConnAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.c.Exec(ctx, sql, args...)
}

func (a *pgxConnAdapter) WaitForNotification(ctx context.Context) (*pgconn.Notification, error) {
	return a.c.WaitForNotification(ctx)
}

func (a *pgxConnAdapter) Close(ctx context.Context) error { return a.c.Close(ctx) }

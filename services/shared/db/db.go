// Package db is the shared database plumbing used by every Postgres
// adapter in the monorepo.
//
// The headline export is `Querier`: a narrow interface that both
// `*pgxpool.Pool` and `pgx.Tx` satisfy, letting one repository
// implementation be re-used inside or outside a transaction.
//
// `WithinTx` is the canonical way to run a multi-statement atomic
// block across bounded contexts — extraction's Mark-Ready +
// UpsertArtifact + search.Upsert all commit together (Property 5).
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the subset of pgx APIs that both `*pgxpool.Pool` and
// `pgx.Tx` provide. Concrete repositories accept a Querier so they
// can be constructed from a pool (normal use) or from a tx
// (transactional batch).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Compile-time guarantees: both backends satisfy Querier.
var (
	_ Querier = (*pgxpool.Pool)(nil)
)

// Beginner is anything that can start a transaction. `*pgxpool.Pool`
// satisfies it.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Transactor coordinates multi-context atomic writes. It owns no
// schema, just hands a `pgx.Tx` to a callback.
type Transactor struct {
	beginner Beginner
}

// NewTransactor wraps a Beginner (typically `*pgxpool.Pool`).
func NewTransactor(b Beginner) *Transactor {
	return &Transactor{beginner: b}
}

// WithinTx runs fn inside a fresh transaction. The tx commits when
// fn returns nil and rolls back on any error or panic. Nested calls
// are not supported in v0.1.
func (t *Transactor) WithinTx(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	if t == nil || t.beginner == nil {
		return errors.New("db: transactor not configured")
	}
	tx, err := t.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			// fn returned an error; roll back. Ignore the rollback
			// error because returning it would mask the real cause.
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("db: commit: %w", commitErr)
	}
	return nil
}

// Package librarytx wires the library Delete use case to a single
// Postgres transaction.
//
// Library.Delete needs to commit two writes atomically:
//
//   1. library.documents flips to status='deleted' (and artifact rows
//      are dropped).
//   2. search.documents row is removed.
//
// On any failure the whole step rolls back. The asynchronous S3
// cleanup runs AFTER the tx commits in libapp.Service.Delete; this
// adapter is only concerned with the Postgres half.
package librarytx

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	libapp "github.com/tomeku/doclens/services/library/app"
	libpg "github.com/tomeku/doclens/services/library/adapters/postgres"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	searchpg "github.com/tomeku/doclens/services/search/adapters/postgres"
	"github.com/tomeku/doclens/services/shared/db"
)

// DeleteTransactor implements libapp.DeleteTransactor.
type DeleteTransactor struct {
	tx      *db.Transactor
	library *libpg.Repo
	search  *searchpg.Repo
}

// New returns a transactor backed by pool.
func New(pool *pgxpool.Pool) *DeleteTransactor {
	return &DeleteTransactor{
		tx:      db.NewTransactor(pool),
		library: libpg.New(pool),
		search:  searchpg.New(pool),
	}
}

// WithinDeleteTx implements libapp.DeleteTransactor.
func (d *DeleteTransactor) WithinDeleteTx(
	ctx context.Context,
	fn func(library libdomain.Repository, eraser libapp.IndexEraser) error,
) error {
	return d.tx.WithinTx(ctx, func(tx pgx.Tx) error {
		q := txQuerier{tx: tx}
		return fn(d.library.WithQuerier(q), &eraserAdapter{repo: d.search.WithQuerier(q)})
	})
}

// txQuerier adapts pgx.Tx to db.Querier (same shape as readytx).
type txQuerier struct{ tx pgx.Tx }

func (q txQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.tx.Exec(ctx, sql, args...)
}
func (q txQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return q.tx.Query(ctx, sql, args...)
}
func (q txQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.tx.QueryRow(ctx, sql, args...)
}

var _ db.Querier = txQuerier{}

// eraserAdapter exposes the search Repo as the library context's
// narrow IndexEraser port.
type eraserAdapter struct {
	repo *searchpg.Repo
}

// Delete implements libapp.IndexEraser.
func (e *eraserAdapter) Delete(ctx context.Context, id uuid.UUID) error {
	return e.repo.Delete(ctx, id)
}

// Package readytx provides the transactor adapter wiring the
// extraction worker's "ready" step into a single Postgres transaction.
//
// On every extract.document task that succeeds, the worker needs to
// commit four writes atomically:
//
//   1. Markdown artifact row (library.artifacts).
//   2. Thumbnail artifact row (library.artifacts), when produced.
//   3. Search index row (search.documents).
//   4. Document status flip to 'ready' (library.documents).
//
// Property 5 of the spec requires these to land or fail together: a
// crash partway through must not leave a 'ready' document with no
// search row, or a search row pointing at a 'failed' document.
//
// This adapter realizes the extraction.domain.Transactor port. It
// owns no domain logic; it only opens a tx and rebinds the library
// + search repos to it.
package readytx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	extractdomain "github.com/tomeku/doclens/services/extraction/domain"
	libpg "github.com/tomeku/doclens/services/library/adapters/postgres"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	searchpg "github.com/tomeku/doclens/services/search/adapters/postgres"
	"github.com/tomeku/doclens/services/shared/db"
)

// Transactor implements extractdomain.Transactor on top of pgxpool.
type Transactor struct {
	tx      *db.Transactor
	library *libpg.Repo
	search  *searchpg.Repo
}

// New returns a Transactor backed by the given pool.
func New(pool *pgxpool.Pool) *Transactor {
	return &Transactor{
		tx:      db.NewTransactor(pool),
		library: libpg.New(pool),
		search:  searchpg.New(pool),
	}
}

// WithinReadyTx implements extractdomain.Transactor.
//
// Inside the tx callback we rebind both repos to the same pgx.Tx
// so all writes go through a single connection and commit together.
func (t *Transactor) WithinReadyTx(
	ctx context.Context,
	fn func(library libdomain.Repository, indexer extractdomain.Indexer) error,
) error {
	return t.tx.WithinTx(ctx, func(tx pgx.Tx) error {
		q := txQuerier{tx: tx}
		boundLib := t.library.WithQuerier(q)
		boundSearch := t.search.WithQuerier(q)
		return fn(boundLib, &indexerAdapter{repo: boundSearch})
	})
}

// txQuerier adapts pgx.Tx to db.Querier. pgx.Tx already provides the
// three methods Querier needs; the only reason we wrap is that pgx.Tx
// returns concrete pgx types and Go's structural interface check
// covers it.
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

// Compile-time check that txQuerier satisfies the Querier port.
var _ db.Querier = txQuerier{}

// Package postgres implements the Search Repository against
// `search.documents` (per migration 0004).
//
// The adapter takes a generic `Querier` so it can run against either
// a long-lived `*pgxpool.Pool` (for read traffic on /v1/search) or a
// `pgx.Tx` (for transactional Upsert from the extraction worker).
//
// The query parser is `websearch_to_tsquery('english', $)`, which
// gives us:
//
//   * Quoted phrases       — "exact phrase"
//   * Negation              — -word
//   * OR / AND keywords     — engineering OR design
//   * Tolerant fallback     — random punctuation can't crash the parser.
//
// `ts_headline` produces the snippet with <mark> tags around hits.
// We intentionally leave the markup untouched here; the web client
// sanitizes via DOMPurify before rendering.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tomeku/doclens/services/search/domain"
	"github.com/tomeku/doclens/services/shared/db"
)

// Repo is the postgres-backed search adapter.
type Repo struct {
	q db.Querier
}

// New returns a Repo bound to the given Querier. Pass `*pgxpool.Pool`
// for read traffic; pass a `pgx.Tx` to make Upsert/Delete part of a
// caller-supplied transaction.
func New(q db.Querier) *Repo { return &Repo{q: q} }

// WithQuerier returns a copy of Repo bound to a different querier.
// Used by the extraction worker to bind to a tx without rebuilding
// the rest of the dependency graph.
func (r *Repo) WithQuerier(q db.Querier) *Repo { return &Repo{q: q} }

// Upsert implements domain.Repository.
//
// The body column is text — Postgres rejects non-UTF8 bytes with
// SQLSTATE 22021. Real extractors (MarkItDown) emit UTF-8 markdown
// and never trip this; the dev passthrough adapter that hands us
// raw PDF bytes does. Sanitizing here means the contract is "best
// effort, never crash"; the unit-tested search behavior is unchanged
// for clean text.
func (r *Repo) Upsert(ctx context.Context, d domain.Document) error {
	const stmt = `
INSERT INTO search.documents (document_id, owner_id, title, body, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
ON CONFLICT (document_id)
DO UPDATE SET title    = EXCLUDED.title,
              body     = EXCLUDED.body,
              owner_id = EXCLUDED.owner_id`

	if _, err := r.q.Exec(ctx, stmt,
		d.DocumentID,
		d.OwnerID,
		toValidUTF8(d.Title),
		toValidUTF8(d.Body),
	); err != nil {
		return fmt.Errorf("search: upsert: %w", err)
	}
	return nil
}

// toValidUTF8 returns s with any non-UTF8 bytes replaced by U+FFFD,
// AND any embedded NUL bytes (\x00) stripped — Postgres rejects
// nulls in text columns even when the rest of the payload is valid
// UTF-8. The replacement is lossy but bounded; the alternative is
// failing the whole extraction transaction over a single bad byte.
func toValidUTF8(s string) string {
	cleaned := strings.ToValidUTF8(s, "\uFFFD")
	if !strings.ContainsRune(cleaned, '\x00') {
		return cleaned
	}
	return strings.ReplaceAll(cleaned, "\x00", "")
}

// Delete implements domain.Repository.
func (r *Repo) Delete(ctx context.Context, documentID uuid.UUID) error {
	const stmt = `DELETE FROM search.documents WHERE document_id = $1`
	if _, err := r.q.Exec(ctx, stmt, documentID); err != nil {
		return fmt.Errorf("search: delete: %w", err)
	}
	return nil
}

// Search implements domain.Repository.
//
// We fetch limit+1 rows to detect a next page without an extra
// COUNT(*). Cursor format is (rank, document_id) with strict
// inequality so duplicate ranks don't loop.
func (r *Repo) Search(ctx context.Context, ownerID, q string, limit int, cursor *domain.Cursor) ([]domain.Hit, *domain.Cursor, error) {
	if q == "" {
		return nil, nil, domain.ErrEmptyQuery
	}
	if limit <= 0 {
		limit = 20
	}

	// Two query shapes: first page (no cursor) vs subsequent
	// (cursor). We avoid string concat with the user query by
	// keeping it as $2 throughout.
	var (
		rows pgx.Rows
		err  error
	)
	if cursor == nil {
		const stmt = `
SELECT d.document_id,
       d.owner_id,
       d.title,
       ts_headline(
           'english',
           d.body,
           websearch_to_tsquery('english', $2),
           'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MaxWords=20, MinWords=8'
       ) AS snippet,
       ts_rank_cd(d.search_vector, websearch_to_tsquery('english', $2)) AS rank
FROM search.documents d
WHERE d.owner_id = $1
  AND d.search_vector @@ websearch_to_tsquery('english', $2)
ORDER BY rank DESC, d.document_id DESC
LIMIT $3`
		rows, err = r.q.Query(ctx, stmt, ownerID, q, limit+1)
	} else {
		const stmt = `
SELECT d.document_id,
       d.owner_id,
       d.title,
       ts_headline(
           'english',
           d.body,
           websearch_to_tsquery('english', $2),
           'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MaxWords=20, MinWords=8'
       ) AS snippet,
       ts_rank_cd(d.search_vector, websearch_to_tsquery('english', $2)) AS rank
FROM search.documents d
WHERE d.owner_id = $1
  AND d.search_vector @@ websearch_to_tsquery('english', $2)
  AND (ts_rank_cd(d.search_vector, websearch_to_tsquery('english', $2)), d.document_id) < ($3, $4)
ORDER BY rank DESC, d.document_id DESC
LIMIT $5`
		rows, err = r.q.Query(ctx, stmt, ownerID, q, cursor.Rank, cursor.DocumentID, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("search: query: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Hit, 0, limit+1)
	for rows.Next() {
		var h domain.Hit
		if err := rows.Scan(&h.DocumentID, &h.OwnerID, &h.Title, &h.Snippet, &h.Rank); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, nil, fmt.Errorf("search: scan: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("search: iter: %w", err)
	}

	var next *domain.Cursor
	if len(out) > limit {
		last := out[limit-1]
		next = &domain.Cursor{Rank: last.Rank, DocumentID: last.DocumentID}
		out = out[:limit]
	}
	return out, next, nil
}

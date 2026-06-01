// Package app holds the Search use cases. v0.1 has just one
// (Search) — Indexing happens transactionally inside the extraction
// worker via the Repository port directly.
package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/search/domain"
)

// DefaultPageSize keeps /v1/search responses small and bounded.
const DefaultPageSize = 20

// MaxQueryLen rejects pathologically long queries before they hit
// Postgres. websearch_to_tsquery handles weird input fine, but a
// 50KB blob of text is almost certainly an attack.
const MaxQueryLen = 256

// Service is the read-side use case for /v1/search.
type Service struct {
	repo domain.Repository
}

// NewService constructs a Service.
func NewService(repo domain.Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("search: nil repository")
	}
	return &Service{repo: repo}, nil
}

// Page is the result of Search.
type Page struct {
	Hits       []domain.Hit
	NextCursor string // empty when no more pages
}

// ErrInvalidCursor mirrors the library equivalent for handler use.
var ErrInvalidCursor = errors.New("search: invalid cursor")

// Search runs the FTS query, owner-scoped to id.
func (s *Service) Search(ctx context.Context, ownerID, q, rawCursor string) (*Page, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, domain.ErrEmptyQuery
	}
	if len(q) > MaxQueryLen {
		q = q[:MaxQueryLen]
	}

	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}

	hits, next, err := s.repo.Search(ctx, ownerID, q, DefaultPageSize, cursor)
	if err != nil {
		return nil, err
	}
	out := &Page{Hits: hits}
	if next != nil {
		out.NextCursor = encodeCursor(*next)
	}
	return out, nil
}

// --- cursor encoding ------------------------------------------------------

// Wire format: base64url("rankFloat|uuid"). Floats are formatted with
// strconv.FormatFloat(f, 'g', -1, 64) so a roundtrip preserves the
// exact bits (within float64 precision).
func encodeCursor(c domain.Cursor) string {
	raw := strconv.FormatFloat(c.Rank, 'g', -1, 64) + "|" + c.DocumentID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(token string) (*domain.Cursor, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidCursor
	}
	rank, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("%w: rank: %v", ErrInvalidCursor, err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: id: %v", ErrInvalidCursor, err)
	}
	return &domain.Cursor{Rank: rank, DocumentID: id}, nil
}

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/search/domain"
)

func TestService_Search_RejectsEmptyQuery(t *testing.T) {
	repo := &fakeRepo{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := svc.Search(context.Background(), "u-1", "   ", ""); !errors.Is(err, domain.ErrEmptyQuery) {
		t.Fatalf("got err=%v, want ErrEmptyQuery", err)
	}
}

func TestService_Search_TruncatesLongQuery(t *testing.T) {
	repo := &fakeRepo{}
	svc, _ := NewService(repo)
	long := strings.Repeat("a", MaxQueryLen+50)
	if _, err := svc.Search(context.Background(), "u-1", long, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := len(repo.gotQuery); got != MaxQueryLen {
		t.Fatalf("query len: got %d, want %d", got, MaxQueryLen)
	}
}

func TestService_Search_PassesOwnerScope(t *testing.T) {
	repo := &fakeRepo{}
	svc, _ := NewService(repo)
	_, err := svc.Search(context.Background(), "user_42", "hello", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo.gotOwnerID != "user_42" {
		t.Fatalf("owner id: got %q, want user_42", repo.gotOwnerID)
	}
	if repo.gotLimit != DefaultPageSize {
		t.Fatalf("limit: got %d, want %d", repo.gotLimit, DefaultPageSize)
	}
}

func TestService_Search_ReturnsEncodedNextCursor(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repo := &fakeRepo{
		hits: []domain.Hit{{DocumentID: id, Title: "Doc", Snippet: "<mark>hello</mark>", Rank: 0.85}},
		next: &domain.Cursor{Rank: 0.85, DocumentID: id},
	}
	svc, _ := NewService(repo)
	page, err := svc.Search(context.Background(), "u-1", "hello", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(page.Hits) != 1 {
		t.Fatalf("hits: %d, want 1", len(page.Hits))
	}
	if page.NextCursor == "" {
		t.Fatalf("expected next cursor")
	}
	// Round-trip: decoding the returned token must produce the same cursor.
	c, err := decodeCursor(page.NextCursor)
	if err != nil || c == nil {
		t.Fatalf("decode: cursor=%v err=%v", c, err)
	}
	if c.DocumentID != id {
		t.Fatalf("cursor id: got %v, want %v", c.DocumentID, id)
	}
	if c.Rank != 0.85 {
		t.Fatalf("cursor rank: got %v, want 0.85", c.Rank)
	}
}

func TestService_Search_RejectsInvalidCursor(t *testing.T) {
	svc, _ := NewService(&fakeRepo{})
	cases := []string{
		"not-base64!!!",
		"YWJjZA",     // base64 of "abcd" — no separator
		"YWJjfGRlZg", // "abc|def" — not numeric / not uuid
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := svc.Search(context.Background(), "u-1", "hello", raw); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("got err=%v, want ErrInvalidCursor", err)
			}
		})
	}
}

func TestCursorRoundTrip(t *testing.T) {
	id := uuid.New()
	c := domain.Cursor{Rank: 0.123456789, DocumentID: id}
	got, err := decodeCursor(encodeCursor(c))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Rank != c.Rank || got.DocumentID != c.DocumentID {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", got, c)
	}
}

// --- fake -----------------------------------------------------------------

type fakeRepo struct {
	gotOwnerID string
	gotQuery   string
	gotLimit   int
	hits       []domain.Hit
	next       *domain.Cursor
}

func (f *fakeRepo) Upsert(ctx context.Context, d domain.Document) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error      { return nil }
func (f *fakeRepo) Search(_ context.Context, ownerID, q string, limit int, _ *domain.Cursor) ([]domain.Hit, *domain.Cursor, error) {
	f.gotOwnerID = ownerID
	f.gotQuery = q
	f.gotLimit = limit
	return f.hits, f.next, nil
}

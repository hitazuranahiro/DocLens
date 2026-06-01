// Package app holds Library read-side use cases. The HTTP layer in
// apps/api/internal/handlers calls these methods.
//
// The web reader needs four shapes (list, detail, presigned URL,
// streaming bytes). Each gets its own use case so the handlers stay
// thin and the access policy lives in one place per concern.
package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// DefaultPageSize is the fixed page size for /v1/documents in v0.1.
const DefaultPageSize = 20

// Service exposes the M5 read use cases.
type Service struct {
	repo            domain.Repository
	store           storage.ObjectStore
	rawBucket       string
	artifactsBucket string
}

// NewService constructs a Service.
func NewService(repo domain.Repository, store storage.ObjectStore, rawBucket, artifactsBucket string) (*Service, error) {
	if repo == nil || store == nil {
		return nil, errors.New("library: nil dependency")
	}
	if rawBucket == "" || artifactsBucket == "" {
		return nil, errors.New("library: empty bucket name")
	}
	return &Service{
		repo:            repo,
		store:           store,
		rawBucket:       rawBucket,
		artifactsBucket: artifactsBucket,
	}, nil
}

// Page is the result of List.
type Page struct {
	Items      []*domain.Document
	NextCursor string // empty when no more pages
}

// List returns one page of the caller's documents, newest first.
//
// `rawCursor` is the opaque token from a previous response (or empty
// for the first page).
func (s *Service) List(ctx context.Context, ownerID, rawCursor string) (*Page, error) {
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, fmt.Errorf("library: %w", err)
	}
	docs, next, err := s.repo.ListByOwner(ctx, ownerID, DefaultPageSize, cursor)
	if err != nil {
		return nil, err
	}
	out := &Page{Items: docs}
	if next != nil {
		out.NextCursor = encodeCursor(*next)
	}
	return out, nil
}

// Detail bundles a document with its artifacts.
type Detail struct {
	Document  *domain.Document
	Artifacts []*domain.Artifact
}

// Get returns the document + every artifact, owner-scoped.
func (s *Service) Get(ctx context.Context, ownerID string, id uuid.UUID) (*Detail, error) {
	doc, err := s.repo.FindByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	arts, err := s.repo.FindArtifacts(ctx, doc.ID)
	if err != nil {
		return nil, err
	}
	return &Detail{Document: doc, Artifacts: arts}, nil
}

// ErrArtifactNotFound is returned when the requested artifact kind
// hasn't been produced yet (extraction still running, or thumbnail
// step skipped).
var ErrArtifactNotFound = errors.New("library: artifact not found")

// ErrNotReady is returned by Markdown when the document hasn't
// finished extracting.
var ErrNotReady = errors.New("library: document not ready")

// MarkdownStream returns the extracted Markdown bytes streaming.
// The caller MUST close the returned reader.
func (s *Service) MarkdownStream(ctx context.Context, ownerID string, id uuid.UUID) (io.ReadCloser, int64, error) {
	doc, err := s.repo.FindByID(ctx, ownerID, id)
	if err != nil {
		return nil, 0, err
	}
	if doc.Status != domain.StatusReady {
		return nil, 0, ErrNotReady
	}
	art, err := s.findArtifact(ctx, doc.ID, domain.ArtifactMarkdown)
	if err != nil {
		return nil, 0, err
	}
	rc, err := s.store.Get(ctx, s.artifactsBucket, art.ObjectKey)
	if err != nil {
		return nil, 0, err
	}
	return rc, art.ByteSize, nil
}

// ThumbnailStream returns the page-1 thumbnail image. Returns
// ErrArtifactNotFound when no thumbnail was produced (e.g. Poppler
// unavailable on the worker).
func (s *Service) ThumbnailStream(ctx context.Context, ownerID string, id uuid.UUID) (io.ReadCloser, int64, error) {
	doc, err := s.repo.FindByID(ctx, ownerID, id)
	if err != nil {
		return nil, 0, err
	}
	art, err := s.findArtifact(ctx, doc.ID, domain.ArtifactThumbnail)
	if err != nil {
		return nil, 0, err
	}
	rc, err := s.store.Get(ctx, s.artifactsBucket, art.ObjectKey)
	if err != nil {
		return nil, 0, err
	}
	return rc, art.ByteSize, nil
}

// RawPresignedURL returns a 5-minute presigned URL pointing at the
// original uploaded bytes. The TTL ceiling is enforced by the
// storage adapter (Property 7 / Req 7.8).
func (s *Service) RawPresignedURL(ctx context.Context, ownerID string, id uuid.UUID) (storage.PresignedURL, error) {
	doc, err := s.repo.FindByID(ctx, ownerID, id)
	if err != nil {
		return storage.PresignedURL{}, err
	}
	return s.store.PresignGet(ctx, s.rawBucket, doc.RawObjectKey, storage.MaxPresignTTL)
}

// findArtifact returns the artifact of the given kind. The repo
// returns artifacts in `kind` order; we walk the slice rather than
// adding a dedicated query.
func (s *Service) findArtifact(ctx context.Context, documentID uuid.UUID, kind domain.ArtifactKind) (*domain.Artifact, error) {
	arts, err := s.repo.FindArtifacts(ctx, documentID)
	if err != nil {
		return nil, err
	}
	for _, a := range arts {
		if a.Kind == kind {
			return a, nil
		}
	}
	return nil, ErrArtifactNotFound
}

// --- cursor encoding ------------------------------------------------------

// Cursor wire format: base64url("createdAtRFC3339Nano|uuid").
//
// We use a separator-delimited string because both fields are
// already-safe ASCII; this avoids pulling in encoding/json or
// gob just for two fields. Any caller-supplied cursor that fails
// to decode is rejected with a domain error mapped to HTTP 400.
func encodeCursor(c domain.Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ErrInvalidCursor is returned when a cursor token cannot be parsed.
var ErrInvalidCursor = errors.New("library: invalid cursor")

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
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, ErrInvalidCursor
	}
	return &domain.Cursor{CreatedAt: ts, ID: id}, nil
}

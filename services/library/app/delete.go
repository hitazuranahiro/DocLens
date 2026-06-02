package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/library/domain"
)

// IndexEraser is the search-side narrow port for deletion. The
// library service depends on this rather than the search domain
// directly so the two contexts stay decoupled (mirrors the Indexer
// port the extraction context owns).
type IndexEraser interface {
	Delete(ctx context.Context, documentID uuid.UUID) error
}

// DeleteTransactor coordinates the atomic soft-delete step.
//
// Inside the callback we run library.SoftDelete + IndexEraser.Delete
// against the same Postgres tx so a partial failure rolls back both.
// The S3 cleanup is intentionally NOT in the tx — it lands as
// fire-and-forget afterwards, with a sweeper to mop up.
type DeleteTransactor interface {
	WithinDeleteTx(ctx context.Context, fn func(library domain.Repository, eraser IndexEraser) error) error
}

// DeleteResult is what Delete returns to the handler.
type DeleteResult struct {
	// AlreadyDeleted is true when the document was already in the
	// 'deleted' state. Handlers can still return 204 in this case
	// (idempotent), or treat it as a hint that the user double-clicked.
	AlreadyDeleted bool
}

// SetDeleteDeps wires the optional collaborators needed for Delete.
// Pass nil to skip the corresponding step. In production the API's
// main wires all three; tests can leave them nil and use the
// non-tx fallback.
func (s *Service) SetDeleteDeps(tx DeleteTransactor, eraser IndexEraser, store DeleteObjectStore, logger *slog.Logger) {
	s.deleteTx = tx
	s.indexEraser = eraser
	s.deleteStore = store
	if logger != nil {
		s.deleteLog = logger
	}
}

// DeleteObjectStore is the narrow ObjectStore subset Delete needs.
// We don't import the full storage package here because (a) we only
// need Delete and (b) wrapping it lets us swap the impl in tests.
type DeleteObjectStore interface {
	Delete(ctx context.Context, bucket, key string) error
}

// Delete soft-deletes the document, removes it from the search
// index, and asynchronously hard-deletes its S3 objects.
//
// Concurrency contract:
//   - SoftDelete + IndexEraser.Delete commit together (Property 5).
//   - S3 cleanup runs AFTER the tx commits. A failure here leaves
//     orphan S3 objects, which the storage-cleanup sweeper picks up.
//   - In-flight extraction jobs no-op when they next touch the row
//     (MarkExtracting refuses status='deleted'); we don't need to
//     cancel asynq tasks proactively (Req 6.2).
func (s *Service) Delete(ctx context.Context, ownerID string, id uuid.UUID) (DeleteResult, error) {
	if s.deleteTx == nil {
		return s.deleteWithoutTx(ctx, ownerID, id)
	}

	var (
		rawKey   string
		artKeys  []string
		already  bool
		commitOK bool
	)

	err := s.deleteTx.WithinDeleteTx(ctx, func(library domain.Repository, eraser IndexEraser) error {
		rk, ak, err := library.SoftDelete(ctx, ownerID, id)
		if err != nil {
			return err
		}
		rawKey = rk
		artKeys = ak

		if eraser != nil {
			if err := eraser.Delete(ctx, id); err != nil {
				return fmt.Errorf("delete from search index: %w", err)
			}
		}
		commitOK = true
		return nil
	})
	if err != nil {
		return DeleteResult{}, err
	}
	if !commitOK {
		// Defensive: tx callback returned nil but didn't reach the
		// success path. Should be unreachable.
		return DeleteResult{}, errors.New("library: delete tx finished without commit signal")
	}

	// AlreadyDeleted is best-effort: SoftDelete returns rawKey but
	// no artifacts when the row was already 'deleted'. This is fine
	// for the response code (204 in both cases).
	already = (len(artKeys) == 0 && rawKey != "")

	// Asynchronous S3 cleanup. We launch a goroutine so the HTTP
	// handler returns immediately. A failed cleanup is logged; the
	// storage sweeper finds it later.
	if s.deleteStore != nil && rawKey != "" {
		s.scheduleHardDelete(rawKey, artKeys)
	}

	return DeleteResult{AlreadyDeleted: already}, nil
}

// deleteWithoutTx is the fallback used in tests + when no transactor
// is wired. SoftDelete and IndexEraser run sequentially; a partial
// failure leaves an orphan search row that the next extraction or
// manual run cleans up.
func (s *Service) deleteWithoutTx(ctx context.Context, ownerID string, id uuid.UUID) (DeleteResult, error) {
	rawKey, artKeys, err := s.repo.SoftDelete(ctx, ownerID, id)
	if err != nil {
		return DeleteResult{}, err
	}
	if s.indexEraser != nil {
		if err := s.indexEraser.Delete(ctx, id); err != nil {
			return DeleteResult{}, fmt.Errorf("delete from search index: %w", err)
		}
	}
	if s.deleteStore != nil && rawKey != "" {
		s.scheduleHardDelete(rawKey, artKeys)
	}
	return DeleteResult{AlreadyDeleted: len(artKeys) == 0}, nil
}

// scheduleHardDelete launches a detached goroutine that hard-deletes
// the raw upload + every artifact object. We use context.Background
// because the request context dies as soon as the handler returns;
// long-tail S3 latency must not abort the cleanup.
func (s *Service) scheduleHardDelete(rawKey string, artKeys []string) {
	keys := make([]string, 0, 1+len(artKeys))
	keys = append(keys, rawKey)
	keys = append(keys, artKeys...)
	rawBucket := s.rawBucket
	artBucket := s.artifactsBucket
	store := s.deleteStore
	logger := s.deleteLog

	go func() {
		ctx := context.Background()
		for i, k := range keys {
			bucket := artBucket
			if i == 0 {
				bucket = rawBucket
			}
			if err := store.Delete(ctx, bucket, k); err != nil {
				logger.Warn("library: hard-delete failed; sweeper will retry",
					"bucket", bucket, "key", k, "err", err)
			}
		}
	}()
}

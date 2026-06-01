package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// SweepResult records what one sweep pass did.
type SweepResult struct {
	ExpiredCount int
	DeletedCount int
}

// SweepExpiredUploads removes orphaned uploads — pending rows whose
// presigned URL has already expired and whose object never landed.
//
// Per Property 1, the sweep:
//   1. picks up pending rows with expires_at < now
//   2. tries to DELETE the corresponding object (no-op if missing)
//   3. flips the upload row status to 'expired'
//
// `batchSize` keeps each pass bounded so a long backlog doesn't block
// the asynq worker for too long.
func (s *Service) SweepExpiredUploads(ctx context.Context, batchSize int) (*SweepResult, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	expired, err := s.uploads.ListExpired(ctx, s.clock.Now(), batchSize)
	if err != nil {
		return nil, fmt.Errorf("ingestion: list expired: %w", err)
	}
	if len(expired) == 0 {
		return &SweepResult{}, nil
	}

	deleted := 0
	ids := make([]uuid.UUID, 0, len(expired))
	for _, u := range expired {
		// Delete is idempotent in the storage port (missing == ok).
		if err := s.store.Delete(ctx, u.Bucket, u.ObjectKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			// One failed delete should not block the rest of the
			// batch. Skip the DB flip so the row is retried next pass.
			continue
		}
		deleted++
		ids = append(ids, u.ID)
	}

	if err := s.uploads.MarkExpired(ctx, ids); err != nil {
		return nil, fmt.Errorf("ingestion: mark expired: %w", err)
	}

	return &SweepResult{
		ExpiredCount: len(expired),
		DeletedCount: deleted,
	}, nil
}

// Compile-time assert the upload-status constants are reachable from
// here so refactors don't accidentally break the sweep import path.
var _ = ingdomain.UploadStatusPending

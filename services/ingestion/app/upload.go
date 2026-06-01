// Package app holds the upload use cases.
//
// CreateUpload validates an Intent, dedupes against the Library, and
// (when not deduped) issues a presigned PUT URL pointing at a
// deterministic key in the raw bucket.
//
// FinalizeUpload verifies the byte landed, creates a Document with
// status=queued, enqueues an extraction job, and marks the upload
// row finalized.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	extractiondomain "github.com/tomeku/doclens/services/extraction/domain"
	"github.com/tomeku/doclens/services/shared/jobs"
	"github.com/tomeku/doclens/services/shared/storage"
)

// Clock lets tests stub time.Now without monkey-patching the package.
type Clock interface {
	Now() time.Time
}

// SystemClock returns wall-clock time.
type SystemClock struct{}

// Now returns time.Now().UTC().
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// Service exposes the M3 upload use cases.
type Service struct {
	uploads     ingdomain.Repository
	library     libdomain.Repository
	store       storage.ObjectStore
	bus         jobs.JobBus
	rawBucket   string
	presignTTL  time.Duration
	enabledMime map[string]struct{}
	clock       Clock
	logger      *slog.Logger
}

// Options bundles the optional knobs.
type Options struct {
	// PresignTTL caps the URL lifetime; the storage adapter further
	// caps to 5 minutes.
	PresignTTL time.Duration
	// EnabledMime lists the MIME types the API accepts. Defaults to
	// {"application/pdf"}.
	EnabledMime []string
	// Bus is the JobBus used to enqueue extract.document. May be nil;
	// when nil, FinalizeUpload still creates the document row and
	// records the missing enqueue as a warning. PR 2 elevates this
	// to a hard requirement.
	Bus jobs.JobBus
	// Clock overrides the wall clock for tests.
	Clock Clock
	// Logger captures soft failures (skipped enqueue, finalize
	// bookkeeping). Defaults to slog.Default().
	Logger *slog.Logger
}

// NewService constructs a Service. The required arguments cannot be nil.
func NewService(
	uploads ingdomain.Repository,
	library libdomain.Repository,
	store storage.ObjectStore,
	rawBucket string,
	opts Options,
) (*Service, error) {
	if uploads == nil || library == nil || store == nil {
		return nil, errors.New("ingestion: nil dependency")
	}
	if rawBucket == "" {
		return nil, errors.New("ingestion: empty raw bucket")
	}
	ttl := opts.PresignTTL
	if ttl <= 0 {
		ttl = storage.MaxPresignTTL
	}
	enabled := make(map[string]struct{})
	if len(opts.EnabledMime) == 0 {
		enabled["application/pdf"] = struct{}{}
	} else {
		for _, m := range opts.EnabledMime {
			enabled[m] = struct{}{}
		}
	}
	clock := opts.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		uploads:     uploads,
		library:     library,
		store:       store,
		bus:         opts.Bus,
		rawBucket:   rawBucket,
		presignTTL:  ttl,
		enabledMime: enabled,
		clock:       clock,
		logger:      logger,
	}, nil
}

// EnabledMime returns a copy of the active allow-list. Callers use this
// for problem-document detail messages.
func (s *Service) EnabledMime() []string {
	out := make([]string, 0, len(s.enabledMime))
	for k := range s.enabledMime {
		out = append(out, k)
	}
	return out
}

// MaxUploadBytes exposes the size cap for problem-document detail.
func (s *Service) MaxUploadBytes() int64 { return ingdomain.MaxUploadBytes }

// CreateUploadResult is what the API returns from POST /v1/uploads.
type CreateUploadResult struct {
	// DocumentID is the ID the client should reference. For new uploads
	// it is allocated up-front (so the raw object key is stable);
	// for duplicates it is the existing Library row.
	DocumentID uuid.UUID
	// UploadID is set only when a fresh upload row was created.
	UploadID *uuid.UUID
	// PresignedURL is set only when a fresh upload was created.
	// Duplicates return nil and the client never PUTs.
	PresignedURL *storage.PresignedURL
	// Status is the current document status for duplicates, or
	// "pending" for fresh uploads.
	Status string
	// Duplicate is true when the (ownerId, sha256) pair already has a
	// Library row; the API maps this to HTTP 200 vs 201.
	Duplicate bool
}

// CreateUpload runs the validation / dedupe / presign workflow.
func (s *Service) CreateUpload(
	ctx context.Context,
	ownerID, filename, mimeType string,
	byteSize int64,
	sha256, title string,
) (*CreateUploadResult, error) {
	intent, err := ingdomain.NewIntent(
		ownerID, filename, mimeType, byteSize, sha256, title, s.enabledMime,
	)
	if err != nil {
		return nil, err
	}

	// Dedupe path. A live document with the same (owner, sha) means
	// "you already have this file" — return it without minting an
	// upload URL. The API maps Duplicate=true to HTTP 200 per Property 6.
	existing, err := s.library.FindAliveByOwnerSHA(ctx, intent.OwnerID, intent.SHA256)
	if err == nil {
		return &CreateUploadResult{
			DocumentID: existing.ID,
			Status:     string(existing.Status),
			Duplicate:  true,
		}, nil
	}
	if !errors.Is(err, libdomain.ErrDocumentNotFound) {
		return nil, fmt.Errorf("ingestion: dedupe lookup: %w", err)
	}

	// Fresh upload: allocate the document ID now so the raw key is
	// stable, mint the presigned PUT URL, and record the pending row.
	documentID := uuid.New()
	objectKey := buildRawKey(intent.OwnerID, documentID, intent.Filename)

	signed, err := s.store.PresignPut(ctx, s.rawBucket, objectKey, storage.PresignPutOptions{
		TTL:           s.presignTTL,
		ContentType:   intent.MimeType,
		ContentLength: intent.ByteSize,
	})
	if err != nil {
		return nil, fmt.Errorf("ingestion: presign put: %w", err)
	}

	now := s.clock.Now()
	upload := &ingdomain.Upload{
		ID:             uuid.New(),
		OwnerID:        intent.OwnerID,
		// Pin the documentID at upload time so /finalize uses the same
		// UUID the client received in CreateUploadResult — without
		// this, the client would have to round-trip /finalize to
		// learn the real documentID. Stored as a pointer to allow
		// nil for legacy/expired rows.
		DocumentID:     &documentID,
		ObjectKey:      objectKey,
		Bucket:         s.rawBucket,
		SHA256:         intent.SHA256,
		MimeType:       intent.MimeType,
		ByteSize:       intent.ByteSize,
		SourceFilename: intent.Filename,
		Title:          intent.Title,
		Status:         ingdomain.UploadStatusPending,
		ExpiresAt:      signed.ExpiresAt,
		CreatedAt:      now,
	}
	if err := s.uploads.Insert(ctx, upload); err != nil {
		return nil, fmt.Errorf("ingestion: insert upload: %w", err)
	}

	return &CreateUploadResult{
		DocumentID:   documentID,
		UploadID:     uuidPtr(upload.ID),
		PresignedURL: &signed,
		Status:       string(ingdomain.UploadStatusPending),
		Duplicate:    false,
	}, nil
}

// FinalizeUploadResult is the post-finalize document, ready for the API
// to serialize.
type FinalizeUploadResult struct {
	Document *libdomain.Document
}

// FinalizeUpload verifies the upload row + object both exist, then
// inserts a Library row in status=queued. Idempotent: calling twice for
// a finalized upload returns the existing document.
func (s *Service) FinalizeUpload(
	ctx context.Context,
	ownerID string,
	uploadID uuid.UUID,
) (*FinalizeUploadResult, error) {
	upload, err := s.uploads.FindByID(ctx, ownerID, uploadID)
	if err != nil {
		return nil, err
	}

	switch upload.Status {
	case ingdomain.UploadStatusFinalized:
		// Already finalized. Idempotent path: look up the document we
		// produced before and hand it back.
		if upload.DocumentID == nil {
			return nil, fmt.Errorf("ingestion: finalized upload %s missing document_id", upload.ID)
		}
		doc, err := s.library.FindByID(ctx, ownerID, *upload.DocumentID)
		if err != nil {
			return nil, fmt.Errorf("ingestion: lookup finalized doc: %w", err)
		}
		return &FinalizeUploadResult{Document: doc}, nil
	case ingdomain.UploadStatusExpired:
		return nil, ingdomain.ErrUploadNotFound
	}

	// Verify the bytes landed.
	info, err := s.store.Head(ctx, upload.Bucket, upload.ObjectKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ingdomain.ErrObjectMissing
		}
		return nil, fmt.Errorf("ingestion: head object: %w", err)
	}
	// Defensive: if the size differs wildly from what the user
	// promised, refuse. We accept small drift because some S3-compatible
	// stores normalize content-length differently than the client did.
	if info.ByteSize > 0 && info.ByteSize != upload.ByteSize {
		return nil, fmt.Errorf(
			"ingestion: object size %d != intent size %d: %w",
			info.ByteSize, upload.ByteSize, ingdomain.ErrObjectMissing,
		)
	}

	// Insert the Library row. Reuse the pre-allocated documentID so
	// the value the client got from CreateUpload matches what lands
	// in the database. The unique index on (owner_id, sha256)
	// catches a race against a parallel upload of the same file.
	doc := &libdomain.Document{
		OwnerID:        upload.OwnerID,
		Title:          upload.Title,
		SourceFilename: upload.SourceFilename,
		SHA256:         upload.SHA256,
		ByteSize:       upload.ByteSize,
		MimeType:       upload.MimeType,
		Status:         libdomain.StatusQueued,
		RawObjectKey:   upload.ObjectKey,
	}
	if upload.DocumentID != nil {
		doc.ID = *upload.DocumentID
	}
	if err := s.library.Insert(ctx, doc); err != nil {
		if errors.Is(err, libdomain.ErrDuplicateDocument) {
			// A concurrent finalize won; return whatever Library has.
			existing, lookupErr := s.library.FindAliveByOwnerSHA(ctx, upload.OwnerID, upload.SHA256)
			if lookupErr != nil {
				return nil, fmt.Errorf("ingestion: race recovery lookup: %w", lookupErr)
			}
			doc = existing
		} else {
			return nil, fmt.Errorf("ingestion: insert document: %w", err)
		}
	}

	if err := s.uploads.MarkFinalized(ctx, upload.ID, doc.ID, s.clock.Now()); err != nil {
		// Document is already in Library; finalization bookkeeping is
		// best-effort. Log and keep going so the user gets their
		// document back even if the upload row update raced.
		s.logger.Warn("upload bookkeeping failed",
			"upload_id", upload.ID,
			"document_id", doc.ID,
			"err", err,
		)
	}

	// Enqueue the extraction job. We do this last so a queue outage
	// doesn't block the user from seeing their document — the row
	// stays in 'queued' and a future retry endpoint (PR 2) can fan
	// it out manually.
	if s.bus != nil {
		_, err := s.bus.Enqueue(ctx, jobs.Task{
			Type: extractiondomain.TaskTypeExtractDocument,
			Payload: extractiondomain.ExtractDocumentPayload{
				DocumentID: doc.ID.String(),
				OwnerID:    doc.OwnerID,
			},
			// Owner-scoped unique key collapses double-finalize
			// races inside the dedupe TTL window.
			UniqueKey: "extract:" + doc.ID.String(),
			UniqueTTL: 5 * time.Minute,
			MaxRetries: 5,
			Timeout:    5 * time.Minute,
		})
		if err != nil && !errors.Is(err, jobs.ErrDuplicate) {
			s.logger.Error("enqueue extract.document failed",
				"document_id", doc.ID,
				"err", err,
			)
		}
	} else {
		s.logger.Warn("no JobBus configured; document will not auto-extract",
			"document_id", doc.ID,
		)
	}

	return &FinalizeUploadResult{Document: doc}, nil
}

func buildRawKey(ownerID string, documentID uuid.UUID, filename string) string {
	return fmt.Sprintf("raw/%s/%s/%s", ownerID, documentID, filename)
}

func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }


// NewServiceMust panics on error. Convenience for bootstrap code that
// has already validated its inputs.
func NewServiceMust(
	uploads ingdomain.Repository,
	library libdomain.Repository,
	store storage.ObjectStore,
	rawBucket string,
	opts Options,
) *Service {
	svc, err := NewService(uploads, library, store, rawBucket, opts)
	if err != nil {
		panic(err)
	}
	return svc
}


// RetryDocument transitions a 'failed' document back to 'queued' and
// re-enqueues the extract.document job. Owner-scoped.
//
// Errors:
//   - libdomain.ErrDocumentNotFound: no such doc, or owned by someone
//     else (Req 7.9).
//   - libdomain.ErrInvalidTransition: the document is not in 'failed'.
func (s *Service) RetryDocument(ctx context.Context, ownerID string, id uuid.UUID) (*libdomain.Document, error) {
	if err := s.library.MarkRetry(ctx, ownerID, id); err != nil {
		return nil, err
	}
	doc, err := s.library.FindByID(ctx, ownerID, id)
	if err != nil {
		return nil, fmt.Errorf("ingestion: lookup retried doc: %w", err)
	}

	if s.bus != nil {
		_, err := s.bus.Enqueue(ctx, jobs.Task{
			Type: extractiondomain.TaskTypeExtractDocument,
			Payload: extractiondomain.ExtractDocumentPayload{
				DocumentID: doc.ID.String(),
				OwnerID:    doc.OwnerID,
			},
			UniqueKey:  "extract:" + doc.ID.String(),
			UniqueTTL:  5 * time.Minute,
			MaxRetries: 5,
			Timeout:    5 * time.Minute,
		})
		if err != nil && !errors.Is(err, jobs.ErrDuplicate) {
			s.logger.Error("retry: enqueue failed",
				"document_id", doc.ID, "err", err)
		}
	}

	return doc, nil
}

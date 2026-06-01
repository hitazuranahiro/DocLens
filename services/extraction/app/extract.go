package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/extraction/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// Service orchestrates one extract.document run.
//
// Flow:
//
//   1. Look up the document.
//   2. Transition status -> 'extracting'.
//   3. Download raw bytes from object storage.
//   4. Run the Extractor; on failure, mark the doc 'failed' with
//      the reason and return nil (job acked, won't retry forever).
//   5. Upload the produced Markdown to the artifacts bucket.
//   6. Best-effort: render and upload a page-1 thumbnail.
//   7. UpsertArtifact rows for everything we wrote.
//   8. Mark the doc 'ready' with computed metrics.
//
// Idempotency (Property 3): the artifacts upsert keys on
// (documentId, kind), so re-running overwrites in place. The
// Markdown S3 key is documentId-shaped, so re-uploading replaces
// the same object. Re-running over a 'ready' document is allowed
// and produces the same end-state.
type Service struct {
	library         libdomain.Repository
	store           storage.ObjectStore
	extractor       domain.Extractor
	thumbnailer     domain.Thumbnailer
	rawBucket       string
	artifactsBucket string
	logger          *slog.Logger
	enabledMimes    map[string]struct{}
}

// Options bundles the Service knobs.
type Options struct {
	// EnabledMimes guards the worker side too: a document whose
	// MIME isn't in the list is failed without invoking the engine.
	EnabledMimes []string
	// Logger receives every transition. Defaults to slog.Default().
	Logger *slog.Logger
	// Thumbnailer is optional. Pass noopthumbnailer.New() to skip.
	Thumbnailer domain.Thumbnailer
}

// NewService constructs a Service.
//
// rawBucket is where the worker reads documents from (matches the
// API's S3_BUCKET_RAW). artifactsBucket is where derived files land
// (S3_BUCKET_ARTIFACTS).
func NewService(
	library libdomain.Repository,
	store storage.ObjectStore,
	extractor domain.Extractor,
	rawBucket, artifactsBucket string,
	opts Options,
) (*Service, error) {
	if library == nil || store == nil || extractor == nil {
		return nil, errors.New("extraction: nil dependency")
	}
	if rawBucket == "" {
		return nil, errors.New("extraction: empty raw bucket")
	}
	if artifactsBucket == "" {
		return nil, errors.New("extraction: empty artifacts bucket")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	thumb := opts.Thumbnailer
	enabled := make(map[string]struct{})
	for _, m := range opts.EnabledMimes {
		enabled[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}
	if len(enabled) == 0 {
		enabled["application/pdf"] = struct{}{}
	}
	return &Service{
		library:         library,
		store:           store,
		extractor:       extractor,
		thumbnailer:     thumb,
		rawBucket:       rawBucket,
		artifactsBucket: artifactsBucket,
		logger:          logger,
		enabledMimes:    enabled,
	}, nil
}

// Extract runs the full pipeline for one document. The handler in
// apps/extraction-worker calls this method.
//
// Returns nil even when the document ends up 'failed' — extraction
// failures are domain outcomes, not job-level errors. The job-level
// error path is reserved for transient infrastructure failures
// (DB down, S3 5xx). Asynq retries those automatically.
func (s *Service) Extract(ctx context.Context, documentID uuid.UUID) error {
	doc, err := s.library.FindByIDUnscoped(ctx, documentID)
	if err != nil {
		if errors.Is(err, libdomain.ErrDocumentNotFound) {
			// Document was deleted between enqueue and pickup. No-op.
			s.logger.Info("extract: document gone; acking",
				"document_id", documentID)
			return nil
		}
		return fmt.Errorf("extract: lookup: %w", err)
	}
	if doc.Status == libdomain.StatusDeleted {
		s.logger.Info("extract: document deleted; acking",
			"document_id", documentID)
		return nil
	}

	mime := strings.ToLower(strings.TrimSpace(doc.MimeType))
	if _, ok := s.enabledMimes[mime]; !ok {
		return s.fail(ctx, doc.ID, fmt.Sprintf("mime not enabled in worker: %q", mime))
	}

	if err := s.library.MarkExtracting(ctx, doc.ID); err != nil {
		if errors.Is(err, libdomain.ErrInvalidTransition) {
			s.logger.Info("extract: invalid transition; acking",
				"document_id", documentID, "status", doc.Status)
			return nil
		}
		return fmt.Errorf("extract: mark extracting: %w", err)
	}

	rawBucket, rawKey, err := splitRawKey(doc.RawObjectKey, s.rawBucket)
	if err != nil {
		return s.fail(ctx, doc.ID, fmt.Sprintf("invalid raw object key: %v", err))
	}

	bytesIn, err := s.download(ctx, rawBucket, rawKey)
	if err != nil {
		// Object missing is a domain failure; transient errors aren't.
		if errors.Is(err, storage.ErrNotFound) {
			return s.fail(ctx, doc.ID, "raw object missing")
		}
		return fmt.Errorf("extract: download: %w", err)
	}

	result, err := s.extractor.Extract(ctx, bytes.NewReader(bytesIn), domain.MimeHint{
		MimeType: doc.MimeType,
		Filename: doc.SourceFilename,
	})
	if err != nil {
		// Extractor errors are split: timeout/crashed are domain
		// failures (no point retrying the same input), context
		// cancellations propagate (asynq will retry).
		if errors.Is(err, ctx.Err()) {
			return ctx.Err()
		}
		switch {
		case errors.Is(err, domain.ErrTimeout):
			return s.fail(ctx, doc.ID, "extraction timed out")
		case errors.Is(err, domain.ErrEngineCrashed),
			errors.Is(err, domain.ErrUnsupportedMime):
			return s.fail(ctx, doc.ID, fmt.Sprintf("extractor: %v", err))
		default:
			return s.fail(ctx, doc.ID, fmt.Sprintf("extractor: %v", err))
		}
	}
	if result == nil {
		return s.fail(ctx, doc.ID, "extractor returned nil result")
	}

	// Persist Markdown.
	mdKey := artifactKey(doc.ID, "extracted.md")
	mdBody := []byte(result.Markdown)
	if err := s.store.Put(ctx, s.artifactsBucket, mdKey, bytes.NewReader(mdBody), storage.PutOptions{
		ContentType:   "text/markdown; charset=utf-8",
		ContentLength: int64(len(mdBody)),
	}); err != nil {
		return fmt.Errorf("extract: put markdown: %w", err)
	}
	if err := s.library.UpsertArtifact(ctx, &libdomain.Artifact{
		DocumentID: doc.ID,
		Kind:       libdomain.ArtifactMarkdown,
		ObjectKey:  mdKey,
		ByteSize:   int64(len(mdBody)),
	}); err != nil {
		return fmt.Errorf("extract: upsert markdown artifact: %w", err)
	}

	// Best-effort thumbnail. We don't fail the document if this
	// step throws, only if Put or UpsertArtifact errors persistently.
	if s.thumbnailer != nil {
		if thumb, err := s.thumbnailer.Thumbnail(ctx, bytes.NewReader(bytesIn), domain.MimeHint{
			MimeType: doc.MimeType,
			Filename: doc.SourceFilename,
		}); err == nil && thumb != nil {
			thumbKey := artifactKey(doc.ID, "thumbnail.png")
			if putErr := s.store.Put(ctx, s.artifactsBucket, thumbKey, bytes.NewReader(thumb.Body), storage.PutOptions{
				ContentType:   thumb.ContentType,
				ContentLength: int64(len(thumb.Body)),
			}); putErr != nil {
				s.logger.Warn("extract: thumbnail put failed; continuing",
					"document_id", doc.ID, "err", putErr)
			} else {
				if upErr := s.library.UpsertArtifact(ctx, &libdomain.Artifact{
					DocumentID: doc.ID,
					Kind:       libdomain.ArtifactThumbnail,
					ObjectKey:  thumbKey,
					ByteSize:   int64(len(thumb.Body)),
				}); upErr != nil {
					s.logger.Warn("extract: thumbnail upsert failed; continuing",
						"document_id", doc.ID, "err", upErr)
				}
			}
		} else if err != nil && !errors.Is(err, domain.ErrUnsupportedThumbnail) {
			s.logger.Warn("extract: thumbnail render failed; continuing",
				"document_id", doc.ID, "err", err)
		}
	}

	// Compute metrics and flip to ready.
	metrics := libdomain.ReadyMetrics{
		PageCount:  result.Pages,
		WordCount:  WordCount(result.Markdown),
		Confidence: ConfidenceFor(result),
	}
	if err := s.library.MarkReady(ctx, doc.ID, metrics); err != nil {
		return fmt.Errorf("extract: mark ready: %w", err)
	}

	s.logger.Info("extract: completed",
		"document_id", doc.ID,
		"pages", metrics.PageCount,
		"words", metrics.WordCount,
		"confidence", metrics.Confidence,
	)
	return nil
}

// fail marks the document failed with the given reason and returns
// nil (extraction failure is acked, not retried by asynq).
func (s *Service) fail(ctx context.Context, id uuid.UUID, reason string) error {
	if err := s.library.MarkFailed(ctx, id, reason); err != nil {
		return fmt.Errorf("extract: mark failed (%s): %w", reason, err)
	}
	s.logger.Warn("extract: failed", "document_id", id, "reason", reason)
	return nil
}

// download streams the raw object into memory. We hold all bytes
// at once because (a) the 100 MB cap means worst-case RAM is
// bounded and (b) MarkItDown takes a path, so a temp-file-spool
// doesn't help us. Streaming straight to disk is a v0.2 concern.
func (s *Service) download(ctx context.Context, bucket, key string) ([]byte, error) {
	rc, err := s.store.Get(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// splitRawKey separates the bucket from the key in a doc's
// raw_object_key field. v0.1 stores keys as "raw/<owner>/<id>/<file>"
// (no bucket prefix), so we return the worker's configured raw bucket.
//
// The signature is forward-compatible: when we adopt full S3 URLs
// (s3://bucket/key) we add parsing here.
func splitRawKey(rawKey, defaultBucket string) (bucket, key string, err error) {
	if rawKey == "" {
		return "", "", errors.New("empty raw_object_key")
	}
	return defaultBucket, rawKey, nil
}

// artifactKey builds the artifact key for a (documentId, name) pair.
// Mirrors the prefix used in the M3 design diagram.
func artifactKey(documentID uuid.UUID, name string) string {
	return fmt.Sprintf("artifacts/%s/%s", documentID, name)
}

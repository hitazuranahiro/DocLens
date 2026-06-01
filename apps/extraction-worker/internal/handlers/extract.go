// Package handlers wires asynq task types to their Go handlers.
//
// M4 PR 1 ships only the registration plumbing and a logging
// placeholder for `extract.document`. The real handler — download,
// run extractor, upload artifacts, update DB — lands in PR 2 along
// with idempotency tests against testcontainers.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// TaskTypeExtractDocument is the canonical name asynq dispatches on.
// Both the API enqueuer and this handler reference the constant.
const TaskTypeExtractDocument = "extract.document"

// ExtractDocumentPayload is the JSON shape the API enqueues.
type ExtractDocumentPayload struct {
	DocumentID string `json:"documentId"`
	OwnerID    string `json:"ownerId"`
}

// ExtractHandler holds the dependencies the real handler will need.
// Today only the extractor and a logger are wired; PR 2 adds the
// document repo, artifact repo, and storage.
type ExtractHandler struct {
	logger    *slog.Logger
	extractor domain.Extractor
}

// NewExtractHandler returns the handler.
func NewExtractHandler(logger *slog.Logger, ex domain.Extractor) *ExtractHandler {
	return &ExtractHandler{logger: logger, extractor: ex}
}

// Handle implements asynq.Handler for the extract.document task.
//
// PR 1 behavior: log, no-op-ack. The plumbing is exercised end-to-end
// (asynq → mux → this method) so PR 2 just replaces the body.
func (h *ExtractHandler) Handle(_ context.Context, t *asynq.Task) error {
	var p ExtractDocumentPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		// Bad payload: don't retry forever.
		return fmt.Errorf("decode extract payload: %w: %w", err, asynq.SkipRetry)
	}

	h.logger.Info("extract.document received",
		"document_id", p.DocumentID,
		"owner_id", p.OwnerID,
		"task_id", asynqTaskID(t),
	)

	// PR 2 will replace this stub with the full pipeline:
	//   1. Look up the document, transition status to extracting.
	//   2. Download bytes from object storage.
	//   3. Invoke h.extractor.Extract(ctx, body, hint).
	//   4. Persist Markdown + thumbnail artifacts, set status=ready,
	//      compute confidence, emit NOTIFY.
	//
	// For now the placeholder must not error or asynq will retry the
	// stub forever.
	return nil
}

// Register binds task types to the asynq.ServeMux. Returning the mux
// keeps main.go small.
func Register(mux *asynq.ServeMux, h *ExtractHandler) {
	mux.HandleFunc(TaskTypeExtractDocument, h.Handle)
}

// asynqTaskID returns the asynq-assigned task ID if available.
// Asynq exposes it via the task's ResultWriter context, but on the
// receive path it's only on the task header which is internal — we
// best-effort look at the type-payload pair for now.
func asynqTaskID(t *asynq.Task) string {
	if rw := t.ResultWriter(); rw != nil {
		return rw.TaskID()
	}
	return ""
}

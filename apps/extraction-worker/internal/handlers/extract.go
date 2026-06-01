// Package handlers wires asynq task types to their Go handlers.
//
// extract.document is the only task today. The handler decodes the
// payload and delegates to extractionapp.Service.Extract; transient
// failures bubble back to asynq for retry, domain failures (the
// document ends up 'failed') are acked.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	extractionapp "github.com/tomeku/doclens/services/extraction/app"
	"github.com/tomeku/doclens/services/extraction/domain"
)

// TaskTypeExtractDocument is the canonical name asynq dispatches on.
// We re-export the constant from extraction/domain so callers stay
// out of the worker package.
const TaskTypeExtractDocument = domain.TaskTypeExtractDocument

// ExtractDocumentPayload is the JSON shape the API enqueues.
// Re-exported for the same reason as TaskTypeExtractDocument.
type ExtractDocumentPayload = domain.ExtractDocumentPayload

// ExtractHandler is the asynq handler for extract.document.
type ExtractHandler struct {
	logger  *slog.Logger
	service *extractionapp.Service
}

// NewExtractHandler returns the handler. service may be nil during
// development (e.g. before Postgres/S3 are reachable); when nil,
// the handler logs the payload and acks so the task isn't retried
// forever against a broken environment.
func NewExtractHandler(logger *slog.Logger, service *extractionapp.Service) *ExtractHandler {
	return &ExtractHandler{logger: logger, service: service}
}

// Handle implements asynq.Handler for extract.document.
func (h *ExtractHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var p ExtractDocumentPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		// Bad payload: don't retry forever.
		return fmt.Errorf("decode extract payload: %w: %w", err, asynq.SkipRetry)
	}

	id, err := uuid.Parse(p.DocumentID)
	if err != nil {
		return fmt.Errorf("invalid documentId %q: %w: %w", p.DocumentID, err, asynq.SkipRetry)
	}

	if h.service == nil {
		h.logger.Warn("extract.document: no service configured; dropping",
			"document_id", id, "owner_id", p.OwnerID,
			"task_id", asynqTaskID(t),
		)
		return nil
	}

	h.logger.Info("extract.document: starting",
		"document_id", id,
		"owner_id", p.OwnerID,
		"task_id", asynqTaskID(t),
	)
	if err := h.service.Extract(ctx, id); err != nil {
		// Transient infra failure: let asynq retry. The Service
		// converts domain failures (timeout, bad input, etc.) to
		// nil + status=failed, so anything that reaches us here
		// is worth a retry.
		if errors.Is(err, ctx.Err()) {
			return ctx.Err()
		}
		return fmt.Errorf("extract.document: %w", err)
	}
	return nil
}

// Register binds task types to the asynq.ServeMux.
func Register(mux *asynq.ServeMux, h *ExtractHandler) {
	mux.HandleFunc(TaskTypeExtractDocument, h.Handle)
}

// asynqTaskID returns the asynq-assigned task ID if available.
func asynqTaskID(t *asynq.Task) string {
	if rw := t.ResultWriter(); rw != nil {
		return rw.TaskID()
	}
	return ""
}

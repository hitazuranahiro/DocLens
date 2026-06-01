package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/tomeku/doclens/apps/extraction-worker/internal/handlers"
	"github.com/tomeku/doclens/services/extraction/adapters/passthrough"
	"github.com/tomeku/doclens/services/extraction/domain"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestExtractHandler_Handle_OkOnValidPayload(t *testing.T) {
	h := handlers.NewExtractHandler(quietLogger(), passthrough.New())
	body, _ := json.Marshal(handlers.ExtractDocumentPayload{
		DocumentID: "doc-1",
		OwnerID:    "user_42",
	})
	task := asynq.NewTask(handlers.TaskTypeExtractDocument, body)
	if err := h.Handle(context.Background(), task); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

func TestExtractHandler_Handle_BadPayloadIsTerminal(t *testing.T) {
	h := handlers.NewExtractHandler(quietLogger(), passthrough.New())
	task := asynq.NewTask(handlers.TaskTypeExtractDocument, []byte("not-json"))
	err := h.Handle(context.Background(), task)
	if err == nil {
		t.Fatal("expected error on bad payload")
	}
	// asynq.SkipRetry means the broker won't retry. Confirming the
	// signal is wrapped so the bad payload doesn't pile up forever.
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want wraps asynq.SkipRetry", err)
	}
	if !strings.Contains(err.Error(), "decode extract payload") {
		t.Fatalf("err message lost context: %v", err)
	}
}

// Compile-time guarantee that the handler keeps satisfying the
// asynq dispatch signature even if we evolve it later.
var _ asynq.HandlerFunc = (&handlers.ExtractHandler{}).Handle

// Compile-time guarantee that passthrough still matches the port. If
// someone breaks it the symbol below fails to resolve.
var _ domain.Extractor = passthrough.New()

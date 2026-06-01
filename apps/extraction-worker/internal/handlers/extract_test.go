package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/tomeku/doclens/apps/extraction-worker/internal/handlers"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestExtractHandler_Handle_NilService_AcksOk(t *testing.T) {
	// Worker boots in dev mode without Postgres / S3. The handler
	// must not retry forever; it logs and acks.
	h := handlers.NewExtractHandler(quietLogger(), nil)
	body, _ := json.Marshal(handlers.ExtractDocumentPayload{
		DocumentID: uuid.NewString(),
		OwnerID:    "user_42",
	})
	task := asynq.NewTask(handlers.TaskTypeExtractDocument, body)
	if err := h.Handle(context.Background(), task); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

func TestExtractHandler_Handle_BadPayloadIsTerminal(t *testing.T) {
	h := handlers.NewExtractHandler(quietLogger(), nil)
	task := asynq.NewTask(handlers.TaskTypeExtractDocument, []byte("not-json"))
	err := h.Handle(context.Background(), task)
	if err == nil {
		t.Fatal("expected error on bad payload")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want wraps asynq.SkipRetry", err)
	}
	if !strings.Contains(err.Error(), "decode extract payload") {
		t.Fatalf("err message lost context: %v", err)
	}
}

func TestExtractHandler_Handle_BadDocumentIDIsTerminal(t *testing.T) {
	h := handlers.NewExtractHandler(quietLogger(), nil)
	body, _ := json.Marshal(handlers.ExtractDocumentPayload{
		DocumentID: "not-a-uuid",
		OwnerID:    "user_42",
	})
	task := asynq.NewTask(handlers.TaskTypeExtractDocument, body)
	err := h.Handle(context.Background(), task)
	if err == nil {
		t.Fatal("expected error on bad documentId")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want wraps asynq.SkipRetry", err)
	}
}

// Compile-time guarantee that the handler still satisfies asynq.HandlerFunc.
var _ asynq.HandlerFunc = (&handlers.ExtractHandler{}).Handle

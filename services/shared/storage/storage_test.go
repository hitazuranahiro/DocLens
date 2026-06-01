package storage_test

import (
	"testing"
	"time"

	"github.com/tomeku/doclens/services/shared/storage"
)

func TestMaxPresignTTL(t *testing.T) {
	// Locks the value documented as Property 7 / Req 7.8 in the spec.
	// If you change this, update the OpenAPI description too.
	if storage.MaxPresignTTL != 5*time.Minute {
		t.Fatalf("MaxPresignTTL = %v, want 5m", storage.MaxPresignTTL)
	}
}

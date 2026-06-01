// Package domain models the Document aggregate and its invariants.
//
// Domain has no dependencies outside the standard library, google/uuid,
// and services/shared. Adapters depend on domain; the reverse is forbidden.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status is the lifecycle state of a document.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusExtracting Status = "extracting"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
	StatusDeleted    Status = "deleted"
)

// IsTerminal reports whether the status is a terminal end-state from the
// extraction worker's perspective.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusReady, StatusFailed, StatusDeleted:
		return true
	}
	return false
}

// Document is the aggregate root of the Library context.
//
// `OwnerID` is the Clerk userId carried in the JWT — v0.1 has no users
// table, the auth provider is the source of truth for identity.
type Document struct {
	ID             uuid.UUID
	OwnerID        string
	Title          string
	SourceFilename string
	SHA256         string
	ByteSize       int64
	MimeType       string
	Status         Status
	PageCount      *int
	WordCount      *int
	Confidence     *int // 0..100
	LastError      *string
	RawObjectKey   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Domain errors. The HTTP layer maps these to RFC 7807 problem documents.
var (
	ErrDocumentNotFound  = errors.New("library: document not found")
	ErrInvalidTransition = errors.New("library: invalid status transition")
	ErrDuplicateDocument = errors.New("library: document already exists")
)

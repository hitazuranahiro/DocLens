// Package domain models the Upload intent — the row the API records when
// a client says "I want to upload this file" but before the bytes have
// landed in object storage.
//
// The Upload aggregate carries everything we need to:
//   - validate the request (size, MIME)
//   - dedupe against Library by (ownerId, sha256)
//   - issue a presigned PUT URL keyed at a deterministic location
//   - on /finalize, verify the object exists and create the Document
//   - on sweep, find orphans older than 24h
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxUploadBytes is the hard ceiling for a single upload (Req 2.1).
const MaxUploadBytes int64 = 100 * 1024 * 1024 // 100 MiB

// SHA256HexLength is the canonical length of a hex SHA-256 digest.
const SHA256HexLength = 64

// UploadStatus is the lifecycle of a pending upload.
type UploadStatus string

const (
	UploadStatusPending   UploadStatus = "pending"
	UploadStatusFinalized UploadStatus = "finalized"
	UploadStatusExpired   UploadStatus = "expired"
)

// Upload is the row in `ingestion.uploads`.
type Upload struct {
	ID             uuid.UUID
	OwnerID        string
	DocumentID     *uuid.UUID
	ObjectKey      string
	Bucket         string
	SHA256         string
	MimeType       string
	ByteSize       int64
	SourceFilename string
	Title          string
	Status         UploadStatus
	ExpiresAt      time.Time
	CreatedAt      time.Time
	FinalizedAt    *time.Time
}

// Intent is the validated input to CreateUpload. Producing one performs
// every check that does not depend on database state.
type Intent struct {
	OwnerID        string
	Filename       string
	MimeType       string
	ByteSize       int64
	SHA256         string
	Title          string
}

// NewIntent validates the raw inputs and normalizes them.
//
// `enabledMimes` is the per-environment allow-list from
// EXTRACTION_ENABLED_FORMATS (default: just "application/pdf").
func NewIntent(
	ownerID, filename, mimeType string,
	byteSize int64,
	sha256, title string,
	enabledMimes map[string]struct{},
) (Intent, error) {
	if ownerID == "" {
		return Intent{}, ErrInvalidIntent
	}
	if filename == "" {
		return Intent{}, ErrInvalidIntent
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		return Intent{}, ErrInvalidIntent
	}
	if _, ok := enabledMimes[mimeType]; !ok {
		return Intent{}, ErrUnsupportedMime
	}
	if byteSize <= 0 {
		return Intent{}, ErrInvalidIntent
	}
	if byteSize > MaxUploadBytes {
		return Intent{}, ErrUploadTooLarge
	}
	sha256 = strings.ToLower(strings.TrimSpace(sha256))
	if !isHexSHA256(sha256) {
		return Intent{}, ErrInvalidIntent
	}
	resolvedTitle := strings.TrimSpace(title)
	if resolvedTitle == "" {
		resolvedTitle = stripExtension(filename)
	}

	return Intent{
		OwnerID:        ownerID,
		Filename:       filename,
		MimeType:       mimeType,
		ByteSize:       byteSize,
		SHA256:         sha256,
		Title:          resolvedTitle,
	}, nil
}

func isHexSHA256(s string) bool {
	if len(s) != SHA256HexLength {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

func stripExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		switch filename[i] {
		case '.':
			if i == 0 {
				return filename
			}
			return filename[:i]
		case '/':
			return filename
		}
	}
	return filename
}

// Domain errors.
var (
	ErrInvalidIntent   = errors.New("ingestion: invalid upload intent")
	ErrUnsupportedMime = errors.New("ingestion: mime type not enabled")
	ErrUploadTooLarge  = errors.New("ingestion: upload exceeds size limit")
	ErrUploadNotFound  = errors.New("ingestion: upload not found")
	ErrObjectMissing   = errors.New("ingestion: object not present in storage")
)

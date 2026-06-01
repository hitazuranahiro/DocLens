// Package storage defines the ObjectStore port that bounded contexts use
// for object storage, plus an S3-compatible adapter (MinIO, R2, AWS S3).
//
// Per ADR 0007 the API never proxies bytes; it issues short-lived
// presigned URLs and lets the browser talk directly to object storage.
// The port reflects that: PresignPut / PresignGet are first-class, and
// raw byte streaming is intentionally absent for now (workers can
// reach for the underlying client when they land in M4).
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ObjectInfo is the result of a HEAD request.
type ObjectInfo struct {
	// ByteSize is the Content-Length reported by the store.
	ByteSize int64
	// ETag is the entity tag (often the MD5 for single-part PUTs). Useful
	// for sanity checks but not authoritative.
	ETag string
	// LastModified is the server-side timestamp.
	LastModified time.Time
	// ContentType is the value the uploader supplied.
	ContentType string
}

// ObjectStore is the port that the API and workers depend on.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type ObjectStore interface {
	// PresignPut returns a URL the caller can upload to via HTTP PUT
	// within ttl. Implementations enforce a hard ceiling on ttl per
	// Property 7 (≤ 5 minutes).
	PresignPut(ctx context.Context, bucket, key string, opts PresignPutOptions) (PresignedURL, error)

	// PresignGet returns a URL the caller can read via HTTP GET within
	// ttl. Same TTL ceiling.
	PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (PresignedURL, error)

	// Head returns metadata for the object, or ErrNotFound.
	Head(ctx context.Context, bucket, key string) (ObjectInfo, error)

	// Get streams the object body. The caller MUST close the reader.
	// Workers use this for download-then-process; HTTP handlers use
	// presigned URLs instead so the API never proxies bytes (ADR 0007).
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// Put uploads an object. Used by workers to store derived
	// artifacts (extracted Markdown, thumbnails, etc.).
	Put(ctx context.Context, bucket, key string, body io.Reader, opts PutOptions) error

	// Delete removes the object. Idempotent: deleting a missing object
	// returns nil.
	Delete(ctx context.Context, bucket, key string) error
}

// PutOptions narrows what the caller is uploading.
type PutOptions struct {
	// ContentType is set on the stored object.
	ContentType string
	// ContentLength, when known, lets the adapter avoid buffering.
	// Pass 0 when you don't know the size; the adapter MAY buffer.
	ContentLength int64
}

// PresignPutOptions narrows what the client can PUT.
type PresignPutOptions struct {
	// TTL is how long the URL stays valid. Implementations cap this to
	// 5 minutes.
	TTL time.Duration
	// ContentType, when non-empty, is bound into the URL signature so
	// the client must PUT with the same Content-Type header.
	ContentType string
	// ContentLength, when non-zero, is bound similarly. Use this to
	// prevent the client from uploading a different size than the API
	// validated.
	ContentLength int64
}

// PresignedURL is what the API hands back to the browser.
type PresignedURL struct {
	URL       string
	ExpiresAt time.Time
}

// MaxPresignTTL is the upper bound enforced by every adapter, per
// Requirement 7.8 / Property 7.
const MaxPresignTTL = 5 * time.Minute

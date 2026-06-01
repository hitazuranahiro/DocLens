package domain

import (
	"time"

	"github.com/google/uuid"
)

// ArtifactKind enumerates the derived files we store per document.
// Mirrors the enum in library.artifacts.
type ArtifactKind string

const (
	ArtifactMarkdown  ArtifactKind = "markdown"
	ArtifactMetadata  ArtifactKind = "metadata"
	ArtifactThumbnail ArtifactKind = "thumbnail"
	ArtifactPageText  ArtifactKind = "page-text"
)

// Artifact is the row in library.artifacts.
type Artifact struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	Kind       ArtifactKind
	ObjectKey  string
	ByteSize   int64
	CreatedAt  time.Time
}

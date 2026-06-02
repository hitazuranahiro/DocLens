// Package handlers implements the API ServerInterface generated from
// openapi.yaml. Each method is small and delegates business work to use
// cases.
package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/tomeku/doclens/apps/api/internal/generated/api"
	"github.com/tomeku/doclens/apps/api/internal/pubsub"
	"github.com/tomeku/doclens/apps/api/internal/transport"

	ingapp "github.com/tomeku/doclens/services/ingestion/app"
	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	libapp "github.com/tomeku/doclens/services/library/app"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	searchapp "github.com/tomeku/doclens/services/search/app"
	"github.com/tomeku/doclens/services/shared/storage"
	"github.com/tomeku/doclens/services/shared/version"
)

// Server is the concrete implementation of gen.ServerInterface.
type Server struct {
	startedAt time.Time
	uploads   *ingapp.Service
	library   *libapp.Service
	hub       *pubsub.Hub
	search    *searchapp.Service
}

// Deps bundles every collaborator the Server needs.
//
// The struct lets us add new dependencies without touching every test
// that constructs a Server.
type Deps struct {
	Uploads *ingapp.Service
	Library *libapp.Service
	// Hub is the live-status fanout for SSE. Optional: when nil the
	// /v1/documents/stream endpoint returns 503.
	Hub *pubsub.Hub
	// Search is optional: when nil the /v1/search endpoint returns 503.
	Search *searchapp.Service
}

// New returns a Server ready to be wired into the chi router.
func New(deps Deps) *Server {
	return &Server{
		startedAt: time.Now(),
		uploads:   deps.Uploads,
		library:   deps.Library,
		hub:       deps.Hub,
		search:    deps.Search,
	}
}

// GetHealth implements GET /v1/health.
func (s *Server) GetHealth(w http.ResponseWriter, _ *http.Request) {
	uptime := int64(time.Since(s.startedAt).Seconds())
	writeJSON(w, http.StatusOK, gen.Health{
		Status:        gen.Ok,
		Version:       version.Version,
		Commit:        ptr(version.Commit),
		UptimeSeconds: uptime,
	})
}

// GetMe implements GET /v1/me. The auth middleware has already verified
// the bearer token; we only render the Identity.
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := transport.IdentityFrom(r.Context())
	if !ok {
		// Unreachable in production: the route is mounted behind AuthMiddleware.
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	out := gen.Identity{
		UserId: id.UserID,
		Email:  openapiEmail(id.Email),
	}
	if id.DisplayName != "" {
		dn := id.DisplayName
		out.DisplayName = &dn
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateUpload implements POST /v1/uploads. Validates the request,
// dedupes against the Library, and either returns an existing document
// (HTTP 200) or a fresh upload row + presigned URL (HTTP 201).
func (s *Server) CreateUpload(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Upload service unavailable", "the API is running without storage")
		return
	}
	id, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}

	var req gen.UploadRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	titleArg := ""
	if req.Title != nil {
		titleArg = *req.Title
	}

	res, err := s.uploads.CreateUpload(r.Context(),
		id.UserID,
		req.Filename,
		req.MimeType,
		req.Size,
		req.Sha256,
		titleArg,
	)
	if err != nil {
		writeUploadProblem(w, err)
		return
	}

	resp := gen.UploadResponse{
		DocumentId: res.DocumentID,
		Status:     gen.UploadResponseStatus(res.Status),
	}
	if res.UploadID != nil {
		uid := *res.UploadID
		resp.UploadId = &uid
	}
	if res.PresignedURL != nil {
		url := res.PresignedURL.URL
		exp := res.PresignedURL.ExpiresAt
		resp.PutUrl = &url
		resp.ExpiresAt = &exp
	}

	if res.Duplicate {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// FinalizeDocument implements POST /v1/documents/{id}/finalize. Verifies
// the object landed and creates the canonical Library row.
func (s *Server) FinalizeDocument(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.uploads == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Upload service unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}

	// `id` in this endpoint is the upload row ID. The two-phase flow
	// returns it as `uploadId` in the CreateUpload response so the
	// client knows what to pass here.
	res, err := s.uploads.FinalizeUpload(r.Context(), authID.UserID, id)
	if err != nil {
		writeUploadProblem(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, toGenDocument(res.Document))
}

// RetryDocument implements POST /v1/documents/{id}/retry. Re-enqueues
// extraction for a failed document.
func (s *Server) RetryDocument(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.uploads == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Upload service unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}

	doc, err := s.uploads.RetryDocument(r.Context(), authID.UserID, id)
	if err != nil {
		writeRetryProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toGenDocument(doc))
}

// --- helpers --------------------------------------------------------------

func writeUploadProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ingdomain.ErrUnsupportedMime):
		transport.WriteProblem(w, http.StatusUnsupportedMediaType,
			"Unsupported media type",
			"this MIME type is not enabled for this environment")
	case errors.Is(err, ingdomain.ErrUploadTooLarge):
		transport.WriteProblem(w, http.StatusRequestEntityTooLarge,
			"Payload too large",
			"upload exceeds the configured size limit")
	case errors.Is(err, ingdomain.ErrInvalidIntent):
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request",
			"upload request failed validation")
	case errors.Is(err, ingdomain.ErrUploadNotFound),
		errors.Is(err, libdomain.ErrDocumentNotFound):
		transport.WriteProblem(w, http.StatusNotFound,
			"Not found", "no such resource")
	case errors.Is(err, ingdomain.ErrObjectMissing):
		transport.WriteProblem(w, http.StatusConflict,
			"Object not present",
			"complete the PUT before calling finalize, or restart the upload")
	default:
		transport.WriteProblem(w, http.StatusInternalServerError,
			"Internal server error", "")
	}
}

// writeRetryProblem maps retry-specific errors to RFC 7807 documents.
func writeRetryProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, libdomain.ErrDocumentNotFound):
		transport.WriteProblem(w, http.StatusNotFound,
			"Not found", "no such document")
	case errors.Is(err, libdomain.ErrInvalidTransition):
		transport.WriteProblem(w, http.StatusConflict,
			"Not retryable",
			"document is not in a failed state")
	default:
		transport.WriteProblem(w, http.StatusInternalServerError,
			"Internal server error", "")
	}
}

func toGenDocument(d *libdomain.Document) gen.Document {
	out := gen.Document{
		Id:             d.ID,
		OwnerId:        d.OwnerID,
		Title:          d.Title,
		SourceFilename: d.SourceFilename,
		Sha256:         d.SHA256,
		ByteSize:       d.ByteSize,
		MimeType:       d.MimeType,
		Status:         gen.DocumentStatus(d.Status),
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
	if d.PageCount != nil {
		v := *d.PageCount
		out.PageCount = &v
	}
	if d.WordCount != nil {
		v := *d.WordCount
		out.WordCount = &v
	}
	if d.Confidence != nil {
		v := float32(*d.Confidence)
		out.Confidence = &v
	}
	if d.LastError != nil {
		v := *d.LastError
		out.LastError = &v
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := newJSONEncoder(w)
	_ = enc.Encode(body)
}

func ptr[T any](v T) *T { return &v }


// ListDocuments implements GET /v1/documents.
func (s *Server) ListDocuments(w http.ResponseWriter, r *http.Request, params gen.ListDocumentsParams) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	id, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}

	cursor := ""
	if params.Cursor != nil {
		cursor = *params.Cursor
	}
	page, err := s.library.List(r.Context(), id.UserID, cursor)
	if err != nil {
		writeLibraryProblem(w, err)
		return
	}

	out := gen.DocumentPage{
		Items: make([]gen.Document, 0, len(page.Items)),
	}
	for _, d := range page.Items {
		out.Items = append(out.Items, toGenDocument(d))
	}
	if page.NextCursor != "" {
		nc := page.NextCursor
		out.NextCursor = &nc
	}
	writeJSON(w, http.StatusOK, out)
}

// GetDocument implements GET /v1/documents/{id}.
func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	det, err := s.library.Get(r.Context(), authID.UserID, id)
	if err != nil {
		writeLibraryProblem(w, err)
		return
	}
	out := gen.DocumentDetail{
		Document:  toGenDocument(det.Document),
		Artifacts: make([]gen.Artifact, 0, len(det.Artifacts)),
	}
	for _, a := range det.Artifacts {
		out.Artifacts = append(out.Artifacts, gen.Artifact{
			Id:         a.ID,
			DocumentId: a.DocumentID,
			Kind:       gen.ArtifactKind(a.Kind),
			ByteSize:   a.ByteSize,
			CreatedAt:  a.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GetDocumentMarkdown implements GET /v1/documents/{id}/markdown.
func (s *Server) GetDocumentMarkdown(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	rc, size, err := s.library.MarkdownStream(r.Context(), authID.UserID, id)
	if err != nil {
		writeLibraryProblem(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	if size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// GetDocumentThumbnail implements GET /v1/documents/{id}/thumbnail.
func (s *Server) GetDocumentThumbnail(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	rc, size, err := s.library.ThumbnailStream(r.Context(), authID.UserID, id)
	if err != nil {
		writeLibraryProblem(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "image/png")
	if size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	// Cache aggressively — thumbnails are content-addressed by docID
	// and overwritten on retry.
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// GetDocumentRawURL implements GET /v1/documents/{id}/raw.
func (s *Server) GetDocumentRawURL(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	signed, err := s.library.RawPresignedURL(r.Context(), authID.UserID, id)
	if err != nil {
		writeLibraryProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.PresignedURL{
		Url:       signed.URL,
		ExpiresAt: signed.ExpiresAt,
	})
}

// writeLibraryProblem maps library use-case errors to RFC 7807 responses.
func writeLibraryProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, libdomain.ErrDocumentNotFound),
		errors.Is(err, libapp.ErrArtifactNotFound):
		transport.WriteProblem(w, http.StatusNotFound,
			"Not found", "no such resource")
	case errors.Is(err, libapp.ErrInvalidCursor):
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request", "invalid cursor")
	case errors.Is(err, libapp.ErrNotReady):
		transport.WriteProblem(w, http.StatusConflict,
			"Document not ready",
			"the document is still being extracted")
	case errors.Is(err, storage.ErrNotFound):
		// Object disappeared between the artifact row and the GET; rare
		// (deletion sweep) but treat as 404 rather than 500.
		transport.WriteProblem(w, http.StatusNotFound,
			"Not found", "artifact missing")
	default:
		transport.WriteProblem(w, http.StatusInternalServerError,
			"Internal server error", "")
	}
}


// DeleteDocument implements DELETE /v1/documents/{id}.
func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if s.library == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Library unavailable", "the API is running without storage")
		return
	}
	authID, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	if _, err := s.library.Delete(r.Context(), authID.UserID, id); err != nil {
		writeLibraryProblem(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

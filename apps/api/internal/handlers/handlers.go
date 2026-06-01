// Package handlers implements the API ServerInterface generated from
// openapi.yaml. Each method is small and delegates business work to use
// cases.
package handlers

import (
	"errors"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/tomeku/doclens/apps/api/internal/generated/api"
	"github.com/tomeku/doclens/apps/api/internal/transport"

	ingapp "github.com/tomeku/doclens/services/ingestion/app"
	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/version"
)

// Server is the concrete implementation of gen.ServerInterface.
type Server struct {
	startedAt time.Time
	uploads   *ingapp.Service
}

// Deps bundles every collaborator the Server needs.
//
// The struct lets us add new dependencies without touching every test
// that constructs a Server.
type Deps struct {
	Uploads *ingapp.Service
}

// New returns a Server ready to be wired into the chi router.
func New(deps Deps) *Server {
	return &Server{
		startedAt: time.Now(),
		uploads:   deps.Uploads,
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

// Search handler for /v1/search.
package handlers

import (
	"errors"
	"net/http"

	gen "github.com/tomeku/doclens/apps/api/internal/generated/api"
	"github.com/tomeku/doclens/apps/api/internal/transport"

	searchapp "github.com/tomeku/doclens/services/search/app"
	searchdomain "github.com/tomeku/doclens/services/search/domain"
)

// SearchDocuments implements GET /v1/search.
func (s *Server) SearchDocuments(w http.ResponseWriter, r *http.Request, params gen.SearchDocumentsParams) {
	if s.search == nil {
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Search unavailable", "the API is running without a search index")
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
	page, err := s.search.Search(r.Context(), id.UserID, params.Q, cursor)
	if err != nil {
		writeSearchProblem(w, err)
		return
	}

	out := gen.SearchPage{
		Items: make([]gen.SearchHit, 0, len(page.Hits)),
	}
	for _, h := range page.Hits {
		rank := float32(h.Rank)
		out.Items = append(out.Items, gen.SearchHit{
			DocumentId: h.DocumentID,
			Title:      h.Title,
			Snippet:    h.Snippet,
			Rank:       rank,
		})
	}
	if page.NextCursor != "" {
		nc := page.NextCursor
		out.NextCursor = &nc
	}
	writeJSON(w, http.StatusOK, out)
}

func writeSearchProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, searchdomain.ErrEmptyQuery):
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request", "query parameter q is required")
	case errors.Is(err, searchapp.ErrInvalidCursor):
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request", "invalid cursor")
	default:
		transport.WriteProblem(w, http.StatusInternalServerError,
			"Internal server error", "")
	}
}

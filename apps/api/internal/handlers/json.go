package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/oapi-codegen/runtime/types"

	"github.com/tomeku/doclens/apps/api/internal/transport"
)

// openapiEmail wraps a string in the openapi_types.Email helper produced
// by oapi-codegen for `format: email` fields.
func openapiEmail(s string) types.Email {
	return types.Email(s)
}

// newJSONEncoder centralizes our JSON encoding choices (no HTML escaping).
func newJSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

// decodeJSON parses the request body into out and writes a 400 problem
// document on failure. Returns true on success.
//
// Limits the body to 1 MiB to prevent runaway memory on malicious clients.
// 1 MiB is plenty for our largest known body shape (UploadRequest).
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	const maxBody = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			transport.WriteProblem(w, http.StatusRequestEntityTooLarge,
				"Payload too large", "request body exceeds the JSON size limit")
			return false
		}
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request", "could not decode request body")
		return false
	}
	if dec.More() {
		transport.WriteProblem(w, http.StatusBadRequest,
			"Bad request", "unexpected trailing JSON in request body")
		return false
	}
	return true
}

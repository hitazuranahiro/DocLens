package handlers

import (
	"encoding/json"
	"io"

	"github.com/oapi-codegen/runtime/types"
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

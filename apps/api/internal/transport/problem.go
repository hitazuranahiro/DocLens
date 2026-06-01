// Package transport contains HTTP-layer concerns shared across handlers:
// the problem-document writer, the auth middleware, and the request-id
// middleware.
package transport

import (
	"encoding/json"
	"net/http"
)

// Problem is the RFC 7807 response shape the API returns for errors.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// WriteProblem renders an RFC 7807 problem document.
//
// Detail is intended for human consumption; do not include sensitive
// values (raw tokens, internal exception messages).
func WriteProblem(w http.ResponseWriter, status int, title, detail string) {
	body := Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

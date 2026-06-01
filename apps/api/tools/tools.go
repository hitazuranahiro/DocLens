//go:build tools

// Package tools pins build-time tooling (oapi-codegen) so it travels with
// the module and is not stripped from go.mod by `go mod tidy`. This file
// is excluded from normal builds via the `tools` build tag.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)

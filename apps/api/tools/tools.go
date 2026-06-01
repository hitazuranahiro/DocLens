//go:build tools

// Package tools pins build-time tooling (oapi-codegen) so it travels with
// the module and is not stripped from go.mod by `go mod tidy`. This file
// is excluded from normal builds via the `tools` build tag.
//
// golang-migrate is also a build-time tool, but its CLI is `package main`
// and can't be imported here. We keep it in go.mod via `go.mod`'s require
// block (added by `go get`) and invoke it with `go run`.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)

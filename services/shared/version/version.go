// Package version exposes build-time version metadata.
//
// Values are populated via -ldflags during release builds. In dev they
// default to development sentinels.
package version

// Version is the semantic version of the build.
var Version = "0.0.0-dev"

// Commit is the git SHA the binary was built from.
var Commit = "unknown"

// BuildTime is the RFC3339 timestamp of the build.
var BuildTime = "unknown"

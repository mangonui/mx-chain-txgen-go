// Package version exposes build-time identity vars injected via ldflags.
//
// Defaults are deliberately conspicuous strings so a binary built without
// the Makefile (e.g. plain `go build`) is identifiable at runtime — a
// `/status` response with Version="dev" tells an operator the binary
// was not built through the canonical release path.
//
// Inject at build time:
//
//	go build -ldflags "-X github.com/mangonui/mx-chain-txgen-go/version.Version=v0.1.0 \
//	                   -X github.com/mangonui/mx-chain-txgen-go/version.Commit=576e93c1 \
//	                   -X github.com/mangonui/mx-chain-txgen-go/version.BuildDate=2026-05-11T12:00:00Z"
package version

// These are overwritten at link time via ldflags. They must remain `var`
// (not `const`) for the -X flag to take effect.
var (
	// Version is the human-readable release tag, e.g. "v0.1.0" or "dev"
	// for unreleased local builds.
	Version = "dev"

	// Commit is the short git SHA the binary was built from.
	Commit = "unknown"

	// BuildDate is ISO-8601 UTC, e.g. "2026-05-11T12:00:00Z".
	BuildDate = "unknown"
)

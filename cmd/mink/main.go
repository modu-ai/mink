// Package main provides the thin entry point for the mink CLI.
// It delegates all logic to internal/cli.
package main

import (
	"os"

	"github.com/modu-ai/mink/internal/cli"
)

// @MX:NOTE Version information is injected via ldflags at build time.
// Example: go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc123 -X main.builtAt=2026-04-28T10:00:00Z"
var (
	version = "dev"
	commit  = "none"
	builtAt = "unknown"
)

func main() {
	os.Exit(cli.Execute(version, commit, builtAt))
}

// Package adapter wires SlashCommandContext to the runtime: router registry,
// query loop controller, and subagent plan-mode awareness.
// SPEC: SPEC-GOOSE-CMDCTX-001
package adapter

import "errors"

// ErrLoopControllerUnavailable is returned by OnClear, OnCompactRequest, and
// OnModelChange when the LoopController was not injected at construction time.
// REQ-CMDCTX-015.
var ErrLoopControllerUnavailable = errors.New("adapter: LoopController is nil")

// Logger is the minimal logging interface the adapter needs for best-effort
// warning emission (e.g., os.Getwd failure). REQ-CMDCTX-018.
type Logger interface {
	Warn(msg string, fields ...any)
}

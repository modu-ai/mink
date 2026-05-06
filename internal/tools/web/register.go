package web

import (
	"fmt"
	"sync"

	"github.com/modu-ai/goose/internal/tools"
)

var (
	globalWebToolsMu sync.Mutex
	globalWebTools   []tools.Tool

	// snapshot is used by test helpers to save/restore globalWebTools state.
	snapshot []tools.Tool
)

// RegisterWebTool appends t to the package-level web tool list.
// It is called from each web tool file's init() function.
// Mirrors the tools.RegisterBuiltin pattern exactly.
func RegisterWebTool(t tools.Tool) {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	globalWebTools = append(globalWebTools, t)
}

// WithWeb returns a tools.Option that registers all globally-registered web
// tools into the given Registry. Panics on duplicate name, mirroring WithBuiltins.
//
// @MX:ANCHOR: [AUTO] Web tool registry entry point — mirrors WithBuiltins pattern
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-001 — fan_in >= 3 (main, tests, bootstrap)
func WithWeb() tools.Option {
	return func(r *tools.Registry) {
		globalWebToolsMu.Lock()
		toolsCopy := make([]tools.Tool, len(globalWebTools))
		copy(toolsCopy, globalWebTools)
		globalWebToolsMu.Unlock()

		for _, t := range toolsCopy {
			if err := r.Register(t, tools.SourceBuiltin); err != nil {
				panic(fmt.Sprintf("web tool registration failed for %q: %v", t.Name(), err))
			}
		}
	}
}

// ClearWebToolsForTest resets globalWebTools to empty, saving the current
// state for later restore. Must only be called in tests.
func ClearWebToolsForTest() {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	snapshot = make([]tools.Tool, len(globalWebTools))
	copy(snapshot, globalWebTools)
	globalWebTools = nil
}

// RestoreWebToolsForTest restores globalWebTools to the state saved by
// ClearWebToolsForTest. Must only be called in tests after ClearWebToolsForTest.
func RestoreWebToolsForTest() {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	globalWebTools = snapshot
	snapshot = nil
}

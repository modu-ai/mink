package web

import (
	"fmt"
	"sync"

	"github.com/modu-ai/mink/internal/tools"
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

// ClearWebToolsForTest is test-only: it resets globalWebTools to empty and
// saves the current state so RestoreWebToolsForTest can roll back. The
// production code path must never call this — it would silently drop every
// registered web tool.
func ClearWebToolsForTest() {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	snapshot = make([]tools.Tool, len(globalWebTools))
	copy(snapshot, globalWebTools)
	globalWebTools = nil
}

// RestoreWebToolsForTest is test-only: it restores globalWebTools to the
// state saved by ClearWebToolsForTest. Must only be called in tests after
// a paired ClearWebToolsForTest invocation; calling out of order leaks
// state across tests.
func RestoreWebToolsForTest() {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	globalWebTools = snapshot
	snapshot = nil
}

// RegisteredWebToolNamesForTest returns a snapshot of the registered web
// tool names in registration order. Tests use it to assert that init()
// callers have wired their tools into the global slice without depending on
// internal state.
func RegisteredWebToolNamesForTest() []string {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	names := make([]string, len(globalWebTools))
	for i, t := range globalWebTools {
		names[i] = t.Name()
	}
	return names
}

// RegisteredWebToolsForTest returns a snapshot of the registered web tools
// (full Tool interface, not just names). Sync-phase contract tests rely on it
// to iterate every tool's Schema and Name without depending on per-tool
// constructor signatures (some tools expose only NewXxxForTest).
func RegisteredWebToolsForTest() []tools.Tool {
	globalWebToolsMu.Lock()
	defer globalWebToolsMu.Unlock()
	out := make([]tools.Tool, len(globalWebTools))
	copy(out, globalWebTools)
	return out
}

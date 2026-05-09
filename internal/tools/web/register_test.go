package web_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modu-ai/goose/internal/tools"
	_ "github.com/modu-ai/goose/internal/tools/builtin/file"     // trigger init() for read/write/edit/grep/glob
	_ "github.com/modu-ai/goose/internal/tools/builtin/terminal" // trigger init() for bash
	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTool is a minimal tools.Tool implementation used for registration tests.
type stubTool struct {
	name   string
	schema json.RawMessage
}

func (s *stubTool) Name() string            { return s.name }
func (s *stubTool) Schema() json.RawMessage { return s.schema }
func (s *stubTool) Scope() tools.Scope      { return tools.ScopeShared }
func (s *stubTool) Call(_ context.Context, _ json.RawMessage) (tools.ToolResult, error) {
	return tools.ToolResult{}, nil
}

// minimalSchema is a valid JSON Schema for testing registration.
var minimalSchema = json.RawMessage(`{
  "type": "object",
  "properties": {},
  "additionalProperties": false
}`)

// TestRegistry_WithWeb_Empty verifies DC-01 (partial): when no web tools are
// registered via RegisterWebTool, WithWeb() on a fresh registry with only
// WithBuiltins() results in only the 6 built-in tools in ListNames.
func TestRegistry_WithWeb_Empty(t *testing.T) {
	// Use a snapshot of the current globalWebTools state so we don't pollute
	// other tests. We clear and restore via the package-level helper.
	web.ClearWebToolsForTest()
	defer web.RestoreWebToolsForTest()

	reg := tools.NewRegistry(tools.WithBuiltins(), web.WithWeb())
	names := reg.ListNames()

	// All names must not include any web tool (none registered in this test).
	for _, n := range names {
		assert.NotEqual(t, "http_fetch", n)
		assert.NotEqual(t, "web_search", n)
	}
}

// TestRegistry_WithWeb_DoubleDuplicate verifies edge-case: registering a tool
// with the same name twice via RegisterWebTool causes the second WithWeb()
// application to panic (mirrors builtin duplicate panic behavior).
func TestRegistry_WithWeb_DoubleDuplicate(t *testing.T) {
	web.ClearWebToolsForTest()
	defer web.RestoreWebToolsForTest()

	// Register same name twice.
	tool1 := &stubTool{name: "duplicate_tool", schema: minimalSchema}
	tool2 := &stubTool{name: "duplicate_tool", schema: minimalSchema}
	web.RegisterWebTool(tool1)
	web.RegisterWebTool(tool2)

	// Applying WithWeb() to a new registry with duplicate names must panic.
	assert.Panics(t, func() {
		_ = tools.NewRegistry(web.WithWeb())
	})
}

// TestRegistry_WithWeb_RegisterAndResolve verifies that a tool registered via
// RegisterWebTool is resolvable after applying WithWeb() to a Registry.
func TestRegistry_WithWeb_RegisterAndResolve(t *testing.T) {
	web.ClearWebToolsForTest()
	defer web.RestoreWebToolsForTest()

	tool := &stubTool{name: "test_web_tool", schema: minimalSchema}
	web.RegisterWebTool(tool)

	reg := tools.NewRegistry(web.WithWeb())
	resolved, ok := reg.Resolve("test_web_tool")
	require.True(t, ok)
	assert.Equal(t, "test_web_tool", resolved.Name())
}

// TestRegistry_WithWeb_ListNames verifies DC-01 / AC-WEB-001:
// production init() registrations of http_fetch + web_search + web_wikipedia +
// web_browse + web_rss + web_arxiv + web_maps + web_wayback produce a Registry
// whose ListNames returns 6 built-in + 8 web tool names = 14 after M4. This
// test does NOT call ClearWebToolsForTest — it relies on the actual init()
// registrations to verify wiring as the milestones land.
func TestRegistry_WithWeb_ListNames(t *testing.T) {
	reg := tools.NewRegistry(tools.WithBuiltins(), web.WithWeb())
	names := reg.ListNames()

	// M4 expectation: 6 built-in + 8 web (http_fetch, web_search, web_wikipedia,
	// web_browse, web_rss, web_arxiv, web_maps, web_wayback) = 14.
	require.Equal(t, 14, len(names), "expected 6 builtins + 8 web tools, got %v", names)
	assert.Contains(t, names, "http_fetch")
	assert.Contains(t, names, "web_search")
	assert.Contains(t, names, "web_wikipedia")
	assert.Contains(t, names, "web_browse")
	assert.Contains(t, names, "web_rss")
	assert.Contains(t, names, "web_arxiv")
	assert.Contains(t, names, "web_maps")
	assert.Contains(t, names, "web_wayback")

	// All eight web tools must resolve to non-nil Tool with ScopeShared.
	for _, n := range []string{
		"http_fetch", "web_search", "web_wikipedia", "web_browse",
		"web_rss", "web_arxiv", "web_maps", "web_wayback",
	} {
		tool, ok := reg.Resolve(n)
		require.True(t, ok, "tool %q must resolve", n)
		require.NotNil(t, tool)
		assert.Equal(t, tools.ScopeShared, tool.Scope())
	}
}

// TestRegistry_WithWeb_ListNamesSorted verifies that ListNames returns a sorted
// slice that includes web tools alongside built-ins.
func TestRegistry_WithWeb_ListNamesSorted(t *testing.T) {
	web.ClearWebToolsForTest()
	defer web.RestoreWebToolsForTest()

	tool := &stubTool{name: "zzz_last_tool", schema: minimalSchema}
	web.RegisterWebTool(tool)

	reg := tools.NewRegistry(tools.WithBuiltins(), web.WithWeb())
	names := reg.ListNames()

	// Names must be sorted alphabetically.
	for i := 1; i < len(names); i++ {
		assert.LessOrEqual(t, names[i-1], names[i], "names must be sorted")
	}
	// Our tool must appear.
	assert.Contains(t, names, "zzz_last_tool")
}

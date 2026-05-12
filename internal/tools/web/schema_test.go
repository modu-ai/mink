package web_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/modu-ai/mink/internal/tools/web"
	"github.com/modu-ai/mink/internal/tools/web/common"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllToolSchemasValid verifies AC-WEB-002 across the full Sprint 1 web
// tool catalog: every registered tool's Schema() must be a valid JSON
// document, declare type:object, and carry additionalProperties:false. The
// test enumerates the global web tool registry rather than instantiating
// individual constructors so it remains stable as constructors evolve.
func TestAllToolSchemasValid(t *testing.T) {
	registered := web.RegisteredWebToolsForTest()
	require.GreaterOrEqual(t, len(registered), 11, "expected at least 11 web tools registered (8 Sprint 1 + weather_current + weather_forecast + weather_air_quality)")

	expectedNames := []string{
		"http_fetch", "web_search", "web_wikipedia", "web_browse",
		"web_rss", "web_arxiv", "web_maps", "web_wayback",
		"weather_current", "weather_forecast", "weather_air_quality",
	}
	gotNames := make([]string, 0, len(registered))
	for _, tool := range registered {
		gotNames = append(gotNames, tool.Name())
	}
	sort.Strings(gotNames)
	sort.Strings(expectedNames)
	assert.Equal(t, expectedNames, gotNames, "all Sprint 1+weather web tools must be registered")

	for _, tool := range registered {
		t.Run(tool.Name(), func(t *testing.T) {
			schema := tool.Schema()
			require.NotNil(t, schema, "schema must not be nil for %q", tool.Name())
			require.Greater(t, len(schema), 0, "schema must not be empty for %q", tool.Name())

			var raw map[string]any
			require.NoError(t, json.Unmarshal(schema, &raw), "schema must be a valid JSON object")

			ap, ok := raw["additionalProperties"]
			require.True(t, ok, "%q schema must declare additionalProperties", tool.Name())
			apBool, isBool := ap.(bool)
			assert.True(t, isBool && !apBool, "%q additionalProperties must be false, got %v", tool.Name(), ap)

			typ, ok := raw["type"]
			require.True(t, ok, "%q schema must have type field", tool.Name())
			assert.Equal(t, "object", typ, "%q top-level type must be \"object\"", tool.Name())

			resourceID := "test://" + tool.Name() + "/schema"
			compiler := jsonschema.NewCompiler()
			require.NoError(t, compiler.AddResource(resourceID, raw), "%q schema must load as resource", tool.Name())
			_, err := compiler.Compile(resourceID)
			require.NoError(t, err, "%q schema must compile under draft 2020-12", tool.Name())
		})
	}
}

// TestM1ToolSchemasConstructed retains the original M1-era constructor-based
// schema check so we still exercise the public NewHTTPFetch / NewWebSearch
// constructors (covers regression against accidental constructor signature
// changes that the registry-iterating umbrella above would not catch).
func TestM1ToolSchemasConstructed(t *testing.T) {
	deps := &common.Deps{}

	toolCases := []struct {
		name string
		tool interface{ Schema() json.RawMessage }
	}{
		{"http_fetch", web.NewHTTPFetch(deps)},
		{"web_search", web.NewWebSearch(deps, "")},
	}

	for _, tc := range toolCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := tc.tool.Schema()
			require.NotNil(t, schema)
			require.Greater(t, len(schema), 0)

			var raw map[string]any
			require.NoError(t, json.Unmarshal(schema, &raw))
		})
	}
}

// TestStandardResponseShape_AllTools is a contract-aliased umbrella test that
// invokes the per-tool standard response shape tests.
//
// contract.md DC-11 verify command references this name; the per-tool tests
// (TestHTTPFetch_StandardResponseShape, TestSearch_StandardResponseShape) carry
// the actual assertions. This wrapper ensures `go test -run TestStandardResponseShape_AllTools`
// resolves and exercises both M1 tools.
func TestStandardResponseShape_AllTools(t *testing.T) {
	t.Run("http_fetch", TestHTTPFetch_StandardResponseShape)
	t.Run("web_search", TestSearch_StandardResponseShape)
}

// TestPermission_RegisterBeforeCheck is a contract-aliased wrapper for DC-14.
// The actual assertion lives in TestHTTPFetch_RegisterBeforeCheck (http_test.go);
// this wrapper resolves the contract verify command name.
func TestPermission_RegisterBeforeCheck(t *testing.T) {
	TestHTTPFetch_RegisterBeforeCheck(t)
}

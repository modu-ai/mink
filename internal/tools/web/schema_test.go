package web_test

import (
	"encoding/json"
	"testing"

	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllToolSchemasValid verifies AC-WEB-002: all M1 web tool schemas are
// draft 2020-12 meta-schema valid and carry additionalProperties: false.
//
// Round B: http_fetch. Round C: web_search added.
func TestAllToolSchemasValid(t *testing.T) {
	deps := &common.Deps{}

	toolCases := []struct {
		name string
		tool interface{ Schema() json.RawMessage }
	}{
		{"http_fetch", web.NewHTTPFetch(deps)},
		{"web_search", web.NewWebSearch(deps, "")},
	}

	for _, tc := range toolCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			schema := tc.tool.Schema()
			require.NotNil(t, schema)
			require.Greater(t, len(schema), 0)

			// Parse as object to inspect fields.
			var raw map[string]any
			require.NoError(t, json.Unmarshal(schema, &raw), "schema must be a valid JSON object")

			// additionalProperties must be explicitly false (bool).
			ap, ok := raw["additionalProperties"]
			require.True(t, ok, "schema must declare additionalProperties")
			apBool, isBool := ap.(bool)
			assert.True(t, isBool && !apBool, "additionalProperties must be false, got %v", ap)

			// top-level type must be "object".
			typ, ok := raw["type"]
			assert.True(t, ok, "schema must have type field")
			assert.Equal(t, "object", typ)

			// Compile with v6 compiler — catches meta-schema violations.
			// AddResource accepts a map[string]any (the parsed JSON object).
			resourceID := "test://" + tc.name + "/schema"
			compiler := jsonschema.NewCompiler()
			err := compiler.AddResource(resourceID, raw)
			require.NoError(t, err, "schema must load as resource: %v", err)

			_, err = compiler.Compile(resourceID)
			require.NoError(t, err, "schema must compile without v6 errors: %v", err)
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

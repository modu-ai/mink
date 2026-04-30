package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSentinelErrors_ProperlyWrapped verifies that all sentinel errors can be checked with errors.Is.
func TestSentinelErrors_ProperlyWrapped(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrBuiltinRequired", ErrBuiltinRequired},
		{"ErrOnlyOnePluginAllowed", ErrOnlyOnePluginAllowed},
		{"ErrNameCollision", ErrNameCollision},
		{"ErrToolNameCollision", ErrToolNameCollision},
		{"ErrInvalidProviderName", ErrInvalidProviderName},
		{"ErrUserMdReadOnly", ErrUserMdReadOnly},
		{"ErrProviderNotInit", ErrProviderNotInit},
		{"ErrUnknownPlugin", ErrUnknownPlugin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test direct error
			assert.True(t, errors.Is(tt.err, tt.err), "direct error should match itself")

			// Test wrapped error
			wrapped := fmt.Errorf("wrapped: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err), "wrapped error should be detectable")
		})
	}
}

// TestSessionContext_Fields verifies SessionContext construction and field access.
func TestSessionContext_Fields(t *testing.T) {
	ctx := SessionContext{
		HermesHome:    "/home/user/.goose",
		Platform:      "darwin",
		AgentContext:  map[string]string{"user_id": "123"},
		AgentIdentity: "teammate-1",
	}

	assert.Equal(t, "/home/user/.goose", ctx.HermesHome)
	assert.Equal(t, "darwin", ctx.Platform)
	assert.Equal(t, "123", ctx.AgentContext["user_id"])
	assert.Equal(t, "teammate-1", ctx.AgentIdentity)
}

// TestToolSchema_JSONMarshal verifies ToolSchema JSON serialization.
func TestToolSchema_JSONMarshal(t *testing.T) {
	schema := ToolSchema{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Owner:       "builtin",
	}

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var decoded ToolSchema
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test_tool", decoded.Name)
	assert.Equal(t, "A test tool", decoded.Description)
	assert.Equal(t, "builtin", decoded.Owner)
}

// TestRecallResult_Empty verifies zero-value behavior of RecallResult.
func TestRecallResult_Empty(t *testing.T) {
	var result RecallResult

	assert.Empty(t, result.Items)
	assert.Equal(t, 0, result.TotalTokens)
}

package tools_test

import (
	"context"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/tools/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPConnection_FetchToolManifest_VerifiesInterface verifies FetchToolManifest interface compliance.
func TestMCPConnection_FetchToolManifest_VerifiesInterface(t *testing.T) {
	t.Parallel()

	conn := newMockMCPConn("test-server", "test_tool")

	// Fetch manifest for existing tool
	manifest, err := conn.FetchToolManifest(context.Background(), "test_tool")
	require.NoError(t, err)
	assert.Equal(t, "test_tool", manifest.Name)
	assert.Equal(t, 1, conn.FetchCount("test_tool"))
}

// TestMCPConnection_FetchToolManifest_ContextTimeout verifies that fetch respects context cancellation.
func TestMCPConnection_FetchToolManifest_ContextTimeout(t *testing.T) {
	t.Parallel()

	conn := newMockMCPConn("test-server", "slow_tool")
	// Configure fetch to take longer than timeout
	conn.fetchFn = func(ctx context.Context, toolName string) (mcp.ToolManifest, error) {
		select {
		case <-ctx.Done():
			return mcp.ToolManifest{}, ctx.Err()
		case <-time.After(10 * time.Second):
			return mcp.ToolManifest{Name: toolName}, nil
		}
	}

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Fetch should respect context timeout
	_, err := conn.FetchToolManifest(ctx, "slow_tool")
	assert.Error(t, err, "context timeout should return error")
}

// TestMCPConnection_ServerID_ReturnsCorrectID verifies ServerID returns the correct identifier.
func TestMCPConnection_ServerID_ReturnsCorrectID(t *testing.T) {
	t.Parallel()

	conn := newMockMCPConn("my-server", "tool1")
	assert.Equal(t, "my-server", conn.ServerID())
}

// TestMCPConnection_ListTools_ReturnsAllTools verifies ListTools returns all registered tools.
func TestMCPConnection_ListTools_ReturnsAllTools(t *testing.T) {
	t.Parallel()

	conn := newMockMCPConn("test-server", "tool1", "tool2", "tool3")
	tools := conn.ListTools()

	assert.Len(t, tools, 3, "should have 3 tools")

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	assert.Contains(t, names, "tool1")
	assert.Contains(t, names, "tool2")
	assert.Contains(t, names, "tool3")
}

// TestMCPConnection_CallTool_ValidInput verifies successful tool execution.
func TestMCPConnection_CallTool_ValidInput(t *testing.T) {
	t.Parallel()

	conn := newMockMCPConn("test-server", "valid_tool")

	result, err := conn.CallTool(context.Background(), "valid_tool", map[string]any{"param": "value"})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, []byte("mcp_result"), result.Content)
}

// TestMCPToolManifest_HasRequiredFields verifies ToolManifest structure.
func TestMCPToolManifest_HasRequiredFields(t *testing.T) {
	t.Parallel()

	manifest := mcp.ToolManifest{
		Name:        "test_tool",
		Description: "Test tool description",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param1": map[string]any{"type": "string"},
			},
		},
	}

	assert.Equal(t, "test_tool", manifest.Name)
	assert.Equal(t, "Test tool description", manifest.Description)
	assert.NotNil(t, manifest.InputSchema)
	assert.IsType(t, map[string]any{}, manifest.InputSchema)
}

// TestMCPToolCallResult_ErrorHandling verifies error result structure.
func TestMCPToolCallResult_ErrorHandling(t *testing.T) {
	t.Parallel()

	result := mcp.ToolCallResult{
		Content: []byte(`{"error":"something went wrong"}`),
		IsError: true,
	}

	assert.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "error")
}

// TestMCPConnection_Integration_EndToEnd verifies full MCP tool workflow.
func TestMCPConnection_Integration_EndToEnd(t *testing.T) {
	t.Parallel()

	// Create mock connection
	conn := newMockMCPConn("prod-server", "calculator")

	// Test ListTools
	tools := conn.ListTools()
	assert.Len(t, tools, 1)
	assert.Equal(t, "calculator", tools[0].Name)

	// Test FetchToolManifest
	manifest, err := conn.FetchToolManifest(context.Background(), "calculator")
	require.NoError(t, err)
	assert.Equal(t, "calculator", manifest.Name)

	// Test CallTool
	result, err := conn.CallTool(context.Background(), "calculator", map[string]any{"value": 21.0})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify multiple calls work
	result2, err := conn.CallTool(context.Background(), "calculator", map[string]any{"value": 5.0})
	require.NoError(t, err)
	assert.False(t, result2.IsError)
}

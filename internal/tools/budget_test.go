package tools_test

import (
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/tools"
	"github.com/stretchr/testify/assert"
)

func TestApplyResultBudget_NoCap(t *testing.T) {
	result := tools.ToolResult{Content: []byte("hello world")}
	out, meta := tools.ApplyResultBudget(result, 0)
	assert.Equal(t, "hello world", string(out.Content))
	assert.False(t, meta.Truncated)
}

func TestApplyResultBudget_NegativeCap(t *testing.T) {
	result := tools.ToolResult{Content: []byte("hello world")}
	out, meta := tools.ApplyResultBudget(result, -1)
	assert.Equal(t, "hello world", string(out.Content))
	assert.False(t, meta.Truncated)
}

func TestApplyResultBudget_UnderCap(t *testing.T) {
	result := tools.ToolResult{Content: []byte("short")}
	out, meta := tools.ApplyResultBudget(result, 1000)
	assert.Equal(t, "short", string(out.Content))
	assert.False(t, meta.Truncated)
}

func TestApplyResultBudget_Truncated(t *testing.T) {
	longContent := strings.Repeat("a", 1000)
	result := tools.ToolResult{Content: []byte(longContent)}
	out, meta := tools.ApplyResultBudget(result, 100)
	assert.True(t, meta.Truncated)
	assert.Equal(t, int64(1000), meta.OriginalSize)
	assert.Less(t, meta.TruncatedSize, int64(1000))
	assert.Contains(t, string(out.Content), "truncated")
	// Metadata에 truncated 플래그
	assert.True(t, out.Metadata["truncated"].(bool))
}

func TestApplyResultBudget_PreservesIsError(t *testing.T) {
	result := tools.ToolResult{Content: []byte(strings.Repeat("x", 200)), IsError: true}
	out, _ := tools.ApplyResultBudget(result, 50)
	assert.True(t, out.IsError)
}

// Package tui provides glamour markdown rendering tests.
package tui

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/cli/tui/snapshots"
	"github.com/stretchr/testify/assert"
)

// TestRender_MarkdownCodeBlock_GlamourEscapes verifies that assistant messages
// containing code blocks are rendered through glamour (raw ``` markers are absent).
// AC-CLITUI-011
func TestRender_MarkdownCodeBlock_GlamourEscapes(t *testing.T) {
	snapshots.SetupASCIITermenv()

	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Add an assistant message with a code block.
	model.messages = append(model.messages, ChatMessage{
		Role:    "assistant",
		Content: "Here is some code:\n```go\nfmt.Println(\"hello\")\n```",
	})
	model.updateViewport()

	view := model.View()

	// Assert: raw ``` markers should NOT appear in the rendered output.
	assert.False(t, strings.Contains(view, "```"), "raw backtick fences should not appear in rendered output")

	// Assert: the code content itself should still appear (glamour preserves content).
	assert.True(t, strings.Contains(view, "hello") || strings.Contains(view, "Println"),
		"code content should still be visible after glamour rendering")
}

// TestRender_InlineCode_GlamourEscapes verifies inline code rendering.
func TestRender_InlineCode_GlamourEscapes(t *testing.T) {
	snapshots.SetupASCIITermenv()

	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// Add assistant message with inline code.
	model.messages = append(model.messages, ChatMessage{
		Role:    "assistant",
		Content: "Use `fmt.Println` to print output.",
	})
	model.updateViewport()

	view := model.View()

	// Assert: inline code content appears (glamour may strip backticks or style them).
	assert.True(t, strings.Contains(view, "fmt.Println"),
		"inline code content should be visible after rendering")
}

// TestRender_UserMessage_NoGlamour verifies user messages are NOT glamour-rendered.
func TestRender_UserMessage_NoGlamour(t *testing.T) {
	model := NewModel(nil, "test-session", true)
	model.width = 80
	model.height = 24
	model.viewport.Width = 80
	model.viewport.Height = 21

	// User message with backtick (should remain raw).
	model.messages = append(model.messages, ChatMessage{
		Role:    "user",
		Content: "Can you explain `interface{}`?",
	})
	model.updateViewport()

	view := model.View()
	// User messages pass through without glamour transformation.
	assert.True(t, strings.Contains(view, "interface{}"),
		"user message content should appear unchanged")
}

package message_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolUseSummary_Formatter는 FormatToolUseSummary가 올바른
// ToolUseSummaryMessage를 반환하는지 검증한다.
// plan.md T1.4 / REQ-QUERY-007
func TestToolUseSummary_Formatter(t *testing.T) {
	t.Parallel()

	content := map[string]any{
		"result": "ok",
		"count":  42,
	}
	msg := message.FormatToolUseSummary("tu-abc", content, false, 1024, 1024)

	require.NotNil(t, msg)
	assert.Equal(t, "tu-abc", msg.ToolUseID)
	assert.Equal(t, content, msg.Content)
	assert.False(t, msg.Truncated)
	assert.Equal(t, 1024, msg.BytesOriginal)
	assert.Equal(t, 1024, msg.BytesKept)
}

// TestToolUseSummary_Formatter_Truncated는 truncated=true 시
// BytesOriginal > BytesKept인 요약 메시지를 검증한다.
// plan.md T1.4 / REQ-QUERY-007 (1MB tool result 치환, AC-QUERY-009)
func TestToolUseSummary_Formatter_Truncated(t *testing.T) {
	t.Parallel()

	const originalSize = 2 * 1024 * 1024 // 2MB
	const keptSize = 512 * 1024          // 512KB

	msg := message.FormatToolUseSummary("tu-big", nil, true, originalSize, keptSize)

	assert.Equal(t, "tu-big", msg.ToolUseID)
	assert.True(t, msg.Truncated)
	assert.Equal(t, originalSize, msg.BytesOriginal)
	assert.Equal(t, keptSize, msg.BytesKept)
	assert.Greater(t, msg.BytesOriginal, msg.BytesKept, "원본 크기가 유지된 크기보다 커야 함")
}

// TestToolUseSummary_Formatter_ZeroBytes는 0 byte 케이스를 검증한다.
func TestToolUseSummary_Formatter_ZeroBytes(t *testing.T) {
	t.Parallel()

	msg := message.FormatToolUseSummary("tu-zero", map[string]any{}, false, 0, 0)
	assert.Equal(t, "tu-zero", msg.ToolUseID)
	assert.Equal(t, 0, msg.BytesOriginal)
	assert.Equal(t, 0, msg.BytesKept)
	assert.False(t, msg.Truncated)
}

// Package context_test — estimateBlockTokens 추가 커버리지 테스트.
package context_test

import (
	"testing"

	goosecontext "github.com/modu-ai/goose/internal/context"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
)

// TestTokenCountWithEstimation_BlockTypes는 다양한 block type의 token 추정을 검증한다.
func TestTokenCountWithEstimation_BlockTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msg     message.Message
		wantMin int64 // 최소 예상값
	}{
		{
			name: "tool_use block",
			msg: message.Message{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "tool_use", Text: "tool_name", ToolUseID: "tu_001"},
				},
			},
			wantMin: 10, // 12 + overhead
		},
		{
			name: "tool_result block",
			msg: message.Message{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "tool_result", ToolResultJSON: `{"result": "ok"}`, ToolUseID: "tu_001"},
				},
			},
			wantMin: 5,
		},
		{
			name: "thinking block",
			msg: message.Message{
				Role: "assistant",
				Content: []message.ContentBlock{
					{Type: "thinking", Thinking: "I am thinking about this problem carefully."},
				},
			},
			wantMin: 5,
		},
		{
			name: "redacted_thinking block",
			msg: message.Message{
				Role: "assistant",
				Content: []message.ContentBlock{
					{Type: "redacted_thinking"},
				},
			},
			wantMin: 8, // exactly 8 tokens
		},
		{
			name: "image block (unknown type)",
			msg: message.Message{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "image", Image: []byte("data")},
				},
			},
			wantMin: 4, // default 4 tokens
		},
		{
			name: "mixed blocks",
			msg: message.Message{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "text", Text: "Hello"},
					{Type: "tool_use", Text: "my_tool"},
					{Type: "redacted_thinking"},
				},
			},
			wantMin: 20,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := goosecontext.TokenCountWithEstimation([]message.Message{tc.msg})
			assert.GreaterOrEqual(t, got, tc.wantMin,
				"token count이 최소값보다 커야 함")
		})
	}
}

// TestTokenCountWithEstimation_EmptyMessages는 빈 입력 처리를 검증한다.
func TestTokenCountWithEstimation_EmptyMessages(t *testing.T) {
	t.Parallel()

	result := goosecontext.TokenCountWithEstimation(nil)
	assert.Equal(t, int64(0), result, "빈 messages는 0이어야 함")

	result2 := goosecontext.TokenCountWithEstimation([]message.Message{})
	assert.Equal(t, int64(0), result2, "빈 슬라이스는 0이어야 함")
}

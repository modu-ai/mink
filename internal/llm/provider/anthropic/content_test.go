package anthropic_test

import (
	"encoding/base64"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider/anthropic"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertMessages_TableDriven는 메시지 변환을 검증한다.
func TestConvertMessages_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      []message.Message
		wantSystem string
		wantLen    int
		wantErr    bool
		checkFn    func(t *testing.T, converted []anthropic.AnthropicMessage)
	}{
		{
			name:       "empty messages",
			input:      nil,
			wantSystem: "",
			wantLen:    0,
		},
		{
			name: "system message extracted",
			input: []message.Message{
				{Role: "system", Content: []message.ContentBlock{{Type: "text", Text: "You are helpful"}}},
				{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}},
			},
			wantSystem: "You are helpful",
			wantLen:    1,
		},
		{
			name: "user text message",
			input: []message.Message{
				{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello world"}}},
			},
			wantSystem: "",
			wantLen:    1,
			checkFn: func(t *testing.T, converted []anthropic.AnthropicMessage) {
				t.Helper()
				require.Len(t, converted, 1)
				assert.Equal(t, "user", converted[0].Role)
				require.Len(t, converted[0].Content, 1)
				assert.Equal(t, "text", converted[0].Content[0]["type"])
				assert.Equal(t, "hello world", converted[0].Content[0]["text"])
			},
		},
		{
			name: "assistant text message",
			input: []message.Message{
				{Role: "assistant", Content: []message.ContentBlock{{Type: "text", Text: "I can help!"}}},
			},
			wantLen: 1,
			checkFn: func(t *testing.T, converted []anthropic.AnthropicMessage) {
				t.Helper()
				require.Len(t, converted, 1)
				assert.Equal(t, "assistant", converted[0].Role)
			},
		},
		{
			name: "image block base64 conversion",
			input: []message.Message{
				{
					Role: "user",
					Content: []message.ContentBlock{
						{
							Type:           "image",
							Image:          []byte{0xFF, 0xD8, 0xFF}, // JPEG magic bytes
							ImageMediaType: "image/jpeg",
						},
					},
				},
			},
			wantLen: 1,
			checkFn: func(t *testing.T, converted []anthropic.AnthropicMessage) {
				t.Helper()
				require.Len(t, converted, 1)
				block := converted[0].Content[0]
				assert.Equal(t, "image", block["type"])
				src, ok := block["source"].(map[string]any)
				require.True(t, ok, "image block must have source")
				assert.Equal(t, "base64", src["type"])
				assert.Equal(t, "image/jpeg", src["media_type"])
				// base64 인코딩 검증
				expectedData := base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF})
				assert.Equal(t, expectedData, src["data"])
			},
		},
		{
			name: "tool_use block",
			input: []message.Message{
				{
					Role: "assistant",
					Content: []message.ContentBlock{
						{
							Type:      "tool_use",
							ToolUseID: "tu-123",
							Text:      "get_weather",
						},
					},
				},
			},
			wantLen: 1,
			checkFn: func(t *testing.T, converted []anthropic.AnthropicMessage) {
				t.Helper()
				block := converted[0].Content[0]
				assert.Equal(t, "tool_use", block["type"])
				assert.Equal(t, "tu-123", block["id"])
			},
		},
		{
			name: "tool_result block",
			input: []message.Message{
				{
					Role: "user",
					Content: []message.ContentBlock{
						{
							Type:           "tool_result",
							ToolUseID:      "tu-123",
							ToolResultJSON: `{"temperature": 25}`,
						},
					},
				},
			},
			wantLen: 1,
			checkFn: func(t *testing.T, converted []anthropic.AnthropicMessage) {
				t.Helper()
				block := converted[0].Content[0]
				assert.Equal(t, "tool_result", block["type"])
				assert.Equal(t, "tu-123", block["tool_use_id"])
			},
		},
		{
			name: "multiple system messages merged",
			input: []message.Message{
				{Role: "system", Content: []message.ContentBlock{{Type: "text", Text: "part1"}}},
				{Role: "system", Content: []message.ContentBlock{{Type: "text", Text: "part2"}}},
				{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}},
			},
			wantSystem: "part1\npart2",
			wantLen:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			system, converted, err := anthropic.ConvertMessages(tc.input)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSystem, system)
			assert.Len(t, converted, tc.wantLen)

			if tc.checkFn != nil {
				tc.checkFn(t, converted)
			}
		})
	}
}

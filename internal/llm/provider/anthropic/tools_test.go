package anthropic_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider/anthropic"
	"github.com/modu-ai/mink/internal/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertTools_TableDriven은 tool 변환을 검증한다 (AC-ADAPTER-002 일부).
func TestConvertTools_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     []tool.Definition
		wantLen   int
		wantFirst anthropic.AnthropicTool
	}{
		{
			name:    "empty tools",
			input:   nil,
			wantLen: 0,
		},
		{
			name: "simple function",
			input: []tool.Definition{
				{
					Name:        "get_weather",
					Description: "현재 날씨를 가져온다",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
			},
			wantLen: 1,
			wantFirst: anthropic.AnthropicTool{
				Name:        "get_weather",
				Description: "현재 날씨를 가져온다",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		{
			name: "tool with no parameters",
			input: []tool.Definition{
				{
					Name:        "list_items",
					Description: "아이템 목록을 반환한다",
					Parameters:  nil,
				},
			},
			wantLen: 1,
			wantFirst: anthropic.AnthropicTool{
				Name:        "list_items",
				Description: "아이템 목록을 반환한다",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		{
			name: "multiple tools",
			input: []tool.Definition{
				{Name: "tool_a", Description: "a"},
				{Name: "tool_b", Description: "b"},
				{Name: "tool_c", Description: "c"},
			},
			wantLen: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := anthropic.ConvertTools(tc.input)
			assert.Len(t, result, tc.wantLen)

			if tc.wantLen > 0 && tc.wantFirst.Name != "" {
				require.NotEmpty(t, result)
				assert.Equal(t, tc.wantFirst.Name, result[0].Name)
				assert.Equal(t, tc.wantFirst.Description, result[0].Description)
				// InputSchema type 확인
				if schemaType, ok := tc.wantFirst.InputSchema["type"]; ok {
					assert.Equal(t, schemaType, result[0].InputSchema["type"])
				}
			}
		})
	}
}

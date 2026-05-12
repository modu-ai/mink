package openai

import (
	"testing"

	"github.com/modu-ai/mink/internal/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertTools_Empty는 빈 tool 목록 변환을 테스트한다.
func TestConvertTools_Empty(t *testing.T) {
	t.Parallel()
	result := ConvertTools(nil)
	assert.Nil(t, result)
}

// TestConvertTools_Single는 단일 tool 변환을 테스트한다.
func TestConvertTools_Single(t *testing.T) {
	t.Parallel()
	defs := []tool.Definition{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
				"required": []string{"city"},
			},
		},
	}
	result := ConvertTools(defs)
	require.Len(t, result, 1)
	assert.Equal(t, "function", result[0].Type)
	assert.Equal(t, "get_weather", result[0].Function.Name)
	assert.Equal(t, "Get current weather", result[0].Function.Description)
	assert.Equal(t, defs[0].Parameters, result[0].Function.Parameters)
}

// TestConvertTools_Multiple는 여러 tool 변환을 테스트한다.
func TestConvertTools_Multiple(t *testing.T) {
	t.Parallel()
	defs := []tool.Definition{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
	}
	result := ConvertTools(defs)
	require.Len(t, result, 2)
	assert.Equal(t, "tool1", result[0].Function.Name)
	assert.Equal(t, "tool2", result[1].Function.Name)
}

// TestConvertToolChoice_Table은 tool_choice 변환 테이블 테스트이다.
func TestConvertToolChoice_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		choice   string
		expected any
	}{
		{"auto", "auto"},
		{"none", "none"},
		{"required", "required"},
		{"", "auto"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.choice, func(t *testing.T) {
			t.Parallel()
			result := ConvertToolChoice(tc.choice)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestConvertToolChoice_SpecificTool은 특정 tool을 강제 선택하는 경우를 테스트한다.
func TestConvertToolChoice_SpecificTool(t *testing.T) {
	t.Parallel()
	result := ConvertToolChoice("get_weather")
	m, ok := result.(map[string]any)
	require.True(t, ok, "specific tool choice should be a map")
	assert.Equal(t, "function", m["type"])
	fn, ok := m["function"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "get_weather", fn["name"])
}

package anthropic

import "github.com/modu-ai/mink/internal/tool"

// AnthropicTool은 Anthropic API의 tool 스키마이다.
type AnthropicTool struct {
	// Name은 tool 이름이다.
	Name string `json:"name"`
	// Description은 tool 설명이다.
	Description string `json:"description"`
	// InputSchema는 JSON Schema 형식의 입력 파라미터 정의이다.
	InputSchema map[string]any `json:"input_schema"`
}

// ConvertTools는 tool.Definition 목록을 Anthropic API 형식으로 변환한다.
// OpenAI function calling 형식 → Anthropic tool 형식 변환.
func ConvertTools(tools []tool.Definition) []AnthropicTool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]AnthropicTool, 0, len(tools))
	for _, t := range tools {
		at := AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: convertParameters(t.Parameters),
		}
		result = append(result, at)
	}
	return result
}

// convertParameters는 tool.Parameters를 Anthropic InputSchema로 변환한다.
// Parameters가 nil이면 기본 empty object schema를 반환한다.
func convertParameters(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{"type": "object"}
	}
	return params
}

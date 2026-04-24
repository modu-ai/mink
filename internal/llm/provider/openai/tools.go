package openai

import "github.com/modu-ai/goose/internal/tool"

// OpenAIToolDef는 OpenAI API의 tool 정의이다.
type OpenAIToolDef struct {
	// Type은 tool 타입이다 (항상 "function").
	Type string `json:"type"`
	// Function은 function tool 정의이다.
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction은 OpenAI function tool의 세부 정의이다.
type OpenAIFunction struct {
	// Name은 함수 이름이다.
	Name string `json:"name"`
	// Description은 함수 설명이다.
	Description string `json:"description,omitempty"`
	// Parameters는 JSON Schema 형식의 파라미터 정의이다.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// ConvertTools는 tool.Definition 목록을 OpenAI API 형식으로 변환한다.
// OpenAI function calling은 정의를 그대로 통과(passthrough)한다.
func ConvertTools(tools []tool.Definition) []OpenAIToolDef {
	if len(tools) == 0 {
		return nil
	}
	result := make([]OpenAIToolDef, 0, len(tools))
	for _, t := range tools {
		result = append(result, OpenAIToolDef{
			Type: "function",
			Function: OpenAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}

// ConvertToolChoice는 tool_choice 문자열을 OpenAI API 형식으로 변환한다.
//
//   - "auto" / "" → "auto"  (모델이 결정)
//   - "none"      → "none"  (tool 미사용 강제)
//   - "required"  → "required" (tool 사용 강제)
//   - 기타        → {"type":"function","function":{"name":...}} (특정 tool 강제)
func ConvertToolChoice(choice string) any {
	switch choice {
	case "", "auto":
		return "auto"
	case "none":
		return "none"
	case "required":
		return "required"
	default:
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": choice,
			},
		}
	}
}

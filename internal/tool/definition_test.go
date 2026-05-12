package tool_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/tool"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestDefinition_Fields는 Definition 구조체가 필요한 필드를 가지는지 검증한다.
func TestDefinition_Fields(t *testing.T) {
	t.Parallel()

	def := tool.Definition{
		Name:        "get_weather",
		Description: "현재 날씨를 가져온다",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "도시 이름",
				},
			},
			"required": []string{"location"},
		},
	}

	if def.Name != "get_weather" {
		t.Errorf("Name: got %q, want %q", def.Name, "get_weather")
	}
	if def.Description != "현재 날씨를 가져온다" {
		t.Errorf("Description: got %q, want %q", def.Description, "현재 날씨를 가져온다")
	}
	if def.Parameters == nil {
		t.Error("Parameters: nil이면 안 됨")
	}
	if _, ok := def.Parameters["type"]; !ok {
		t.Error("Parameters에 'type' 키 없음")
	}
}

// TestDefinition_Zero는 zero value가 유효한지 검증한다.
func TestDefinition_Zero(t *testing.T) {
	t.Parallel()

	var def tool.Definition
	if def.Name != "" {
		t.Errorf("zero Name: got %q, want empty", def.Name)
	}
	if def.Parameters != nil {
		t.Errorf("zero Parameters: got non-nil, want nil")
	}
}

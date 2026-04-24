// Package glm의 thinking.go: GLM 모델별 thinking mode 지원 여부 판별 및 파라미터 주입.
// SPEC-GOOSE-ADAPTER-002 M4
package glm

import (
	"fmt"

	"github.com/modu-ai/goose/internal/llm/provider"
)

// ThinkingCapableModels는 thinking:{enabled} 파라미터를 지원하는 GLM 모델 목록이다.
// glm-4.5-air는 경량 모델로 thinking 미지원이다.
// REQ-ADP2-007
var ThinkingCapableModels = map[string]bool{
	"glm-5":   true,
	"glm-4.7": true,
	"glm-4.6": true,
	"glm-4.5": true,
}

// BuildThinkingField는 CompletionRequest의 thinking 설정과 모델명을 받아
// ExtraRequestFields에 추가할 thinking 맵, 지원 여부(ok), 경고 이유를 반환한다.
//
// 반환값:
//   - field: ExtraRequestFields에 merge할 맵 (nil이면 주입 불필요)
//   - ok: false이면 REQ-ADP2-014 graceful degradation (WARN + 무시)
//   - reason: ok=false일 때 WARN 로그 메시지
func BuildThinkingField(cfg *provider.ThinkingConfig, model string) (field map[string]any, ok bool, reason string) {
	if cfg == nil || !cfg.Enabled {
		return nil, true, ""
	}

	if !ThinkingCapableModels[model] {
		return nil, false, fmt.Sprintf(
			"glm: thinking requested but model %q does not support it; proceeding without thinking param",
			model,
		)
	}

	return map[string]any{
		"thinking": map[string]any{
			"type": "enabled",
		},
	}, true, ""
}

package anthropic

import "github.com/modu-ai/mink/internal/llm/provider"

// AnthropicThinkingParam은 Anthropic API 요청의 thinking 파라미터이다.
// Adaptive Thinking 모델(Opus 4.7+)은 Effort를, 이전 모델은 BudgetTokens를 사용한다.
type AnthropicThinkingParam struct {
	// Type은 "enabled" 고정이다.
	Type string `json:"type"`
	// Effort는 Adaptive Thinking 모델의 노력 수준이다.
	// 값: "low" | "medium" | "high" | "xhigh" | "max"
	Effort string `json:"effort,omitempty"`
	// BudgetTokens는 non-adaptive 모델의 thinking 예산 토큰 수이다.
	BudgetTokens int `json:"budget_tokens,omitempty"`
}

// validEffortLevels는 유효한 Effort 레벨 집합이다.
var validEffortLevels = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
	"max":    true,
}

// BuildThinkingParam은 ThinkingConfig와 모델 이름을 기반으로 API 요청 파라미터를 구성한다.
//
// 결정 규칙 (AC-ADAPTER-012):
//   - cfg == nil 또는 cfg.Enabled == false → nil 반환
//   - model이 Adaptive Thinking 지원 모델이고 Effort가 유효한 레벨 → {Type:"enabled", Effort:"..."}
//   - 그 외 + BudgetTokens > 0 → {Type:"enabled", BudgetTokens:...}
//   - 그 외 → nil (budget_tokens 미설정, effort 미지원 조합은 무효)
func BuildThinkingParam(cfg *provider.ThinkingConfig, model string) *AnthropicThinkingParam {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Adaptive Thinking 모델 (Opus 4.7+): effort 파라미터 사용
	if IsAdaptiveThinkingModel(model) {
		if validEffortLevels[cfg.Effort] {
			return &AnthropicThinkingParam{
				Type:   "enabled",
				Effort: cfg.Effort,
			}
		}
		// Effort가 설정되지 않았거나 유효하지 않으면 nil
		return nil
	}

	// Non-adaptive 모델: budget_tokens 파라미터 사용
	if cfg.BudgetTokens > 0 {
		return &AnthropicThinkingParam{
			Type:         "enabled",
			BudgetTokens: cfg.BudgetTokens,
		}
	}

	// budget_tokens도 없는 경우 — 유효하지 않음
	return nil
}

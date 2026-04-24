// Package anthropic는 Anthropic Claude API 어댑터를 구현한다.
// SPEC-GOOSE-ADAPTER-001 M1
package anthropic

// modelAliases는 별칭 → 실제 모델 ID 매핑이다.
// REQ-ADAPTER-018: 모델 별칭 정규화
var modelAliases = map[string]string{
	"claude-3.5-sonnet": "claude-3-5-sonnet-20241022",
	"claude-3-5-sonnet": "claude-3-5-sonnet-20241022",
	"claude-opus-4":     "claude-opus-4-7",
	"claude-3.7-sonnet": "claude-3-7-sonnet-20250219",
	"claude-3-7-sonnet": "claude-3-7-sonnet-20250219",
	"claude-3-haiku":    "claude-3-haiku-20240307",
	"claude-3-opus":     "claude-3-opus-20240229",
	"claude-3-sonnet":   "claude-3-sonnet-20240229",
}

// modelMaxOutputTokens는 모델별 최대 출력 토큰 수이다.
var modelMaxOutputTokens = map[string]int{
	"claude-opus-4-7":            16000,
	"claude-opus-4-7-20260320":   16000,
	"claude-3-7-sonnet-20250219": 16000,
	"claude-3-5-sonnet-20241022": 8192,
	"claude-3-5-haiku-20241022":  8192,
	"claude-3-haiku-20240307":    4096,
	"claude-3-opus-20240229":     4096,
	"claude-3-sonnet-20240229":   4096,
}

// adaptiveThinkingModels는 Adaptive Thinking을 지원하는 모델 집합이다.
// Opus 4.7 style: effort 파라미터 사용 (budget_tokens 대신).
var adaptiveThinkingModels = map[string]bool{
	"claude-opus-4-7":          true,
	"claude-opus-4-7-20260320": true,
}

// NormalizeModel은 모델 별칭을 실제 모델 ID로 변환한다.
// 알 수 없는 모델은 그대로 반환한다.
func NormalizeModel(model string) string {
	if normalized, ok := modelAliases[model]; ok {
		return normalized
	}
	return model
}

// MaxOutputTokensFor는 모델의 최대 출력 토큰 수를 반환한다.
// 알 수 없는 모델은 기본값 4096을 반환한다.
func MaxOutputTokensFor(model string) int {
	if max, ok := modelMaxOutputTokens[model]; ok {
		return max
	}
	return 4096
}

// IsAdaptiveThinkingModel은 모델이 Adaptive Thinking을 지원하는지 반환한다.
func IsAdaptiveThinkingModel(model string) bool {
	return adaptiveThinkingModels[model]
}

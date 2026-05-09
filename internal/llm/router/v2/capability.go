// Package v2 — capability.go: 정적 15-provider × 4-capability 매트릭스.
//
// CapabilityMatrix 는 process lifetime 동안 read-only — init() 시점 1회
// 채워지고 이후 변경되지 않는다 (REQ-RV2-004). source-of-truth 는
// spec.md §6.1 표이며, provider 가 신모델로 capability 를 추가/제거하면
// 본 SPEC §13 manual update 정책에 따라 amendment SPEC 가 필요하다.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-004 / REQ-RV2-010
package v2

// CapabilityMatrix 는 provider id → 지원 capability set 매핑이다.
// 부재 provider 는 zero map 으로 취급되어 모든 Match 가 false.
type CapabilityMatrix map[string]map[Capability]bool

// Match 는 provider 가 required 의 모든 capability 를 지원하는지 반환한다.
//
// 동작:
//   - provider 가 매트릭스에 없으면 false (unknown provider 는 보수적 reject).
//   - required 가 빈 slice 면 true (no capability requirement = pass).
//   - required 중 하나라도 false 인 capability 가 있으면 false.
func (m CapabilityMatrix) Match(provider string, required []Capability) bool {
	caps, ok := m[provider]
	if !ok {
		return false
	}
	for _, r := range required {
		if !caps[r] {
			return false
		}
	}
	return true
}

// Providers 는 매트릭스에 등록된 provider id 의 정렬되지 않은 목록을
// 반환한다. 후보 풀 enumeration 에 사용된다 (P3 RouterV2 가 호출).
func (m CapabilityMatrix) Providers() []string {
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	return out
}

// defaultMatrix 는 spec.md §6.1 의 15×4 표이다. provider 의 신모델로
// capability 가 변하면 이 변수 + spec.md §6.1 양쪽 모두 amendment 필요.
//
// "model dependent" / "some" 항목 (openrouter, together, fireworks, ollama
// vision via llava, qwen vision via qwen3-vl) 은 보수적 true 로 표기 —
// 모델 단위 fine-grained 매칭은 OUT (spec.md §3.2).
var defaultMatrix = CapabilityMatrix{
	"anthropic": {
		CapPromptCaching:   true,
		CapFunctionCalling: true,
		CapVision:          true,
		CapRealtime:        false,
	},
	"openai": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true,
		CapRealtime:        true,
	},
	"google": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true,
		CapRealtime:        false,
	},
	"xai": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true,
		CapRealtime:        false,
	},
	"deepseek": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
	"ollama": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true, // llava / model-dependent — 보수적 true
		CapRealtime:        false,
	},
	"zai_glm": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
	"groq": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
	"openrouter": {
		CapPromptCaching:   false,
		CapFunctionCalling: true, // model-dependent
		CapVision:          true, // model-dependent
		CapRealtime:        false,
	},
	"together": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true, // some models
		CapRealtime:        false,
	},
	"fireworks": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true, // some models
		CapRealtime:        false,
	},
	"cerebras": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
	"mistral": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
	"qwen": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          true, // qwen3-vl
		CapRealtime:        false,
	},
	"kimi": {
		CapPromptCaching:   false,
		CapFunctionCalling: true,
		CapVision:          false,
		CapRealtime:        false,
	},
}

// DefaultMatrix 는 spec.md §6.1 의 정적 15×4 매트릭스 사본을 반환한다.
//
// process lifetime 동안 단 한 번 init 시점에 채워진 defaultMatrix 의
// shallow-copy 를 반환하여 caller 가 수정해도 다른 호출자에 영향이 없게
// 한다 (race-free).
func DefaultMatrix() CapabilityMatrix {
	out := make(CapabilityMatrix, len(defaultMatrix))
	for provider, caps := range defaultMatrix {
		copied := make(map[Capability]bool, len(caps))
		for k, v := range caps {
			copied[k] = v
		}
		out[provider] = copied
	}
	return out
}

// Package v2 implements policy-aware, capability-aware, ratelimit-aware
// routing on top of router/v1 (SPEC-GOOSE-ROUTER-001) using the decorator
// pattern. This file (policy.go) defines the user-facing policy schema:
// PolicyMode enum, RoutingPolicy struct, and the supporting Capability
// + ProviderRef value types.
//
// Defaults are deliberately chosen so that the zero value of RoutingPolicy
// represents "no opinion" — i.e. PreferQuality mode with empty chains.
// This lets the v2 decorator forward unchanged to v1 when no policy file
// is present (REQ-RV2-002 backward compatibility).
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-001 / REQ-RV2-002
package v2

// PolicyMode 는 RouterV2 의 의사결정 우선순위를 결정하는 사용자 정책 모드이다.
//
// PreferQuality 가 zero value 인 이유: routing-policy.yaml 부재 시
// RoutingPolicy{} 가 기본값으로 사용되며, 이 때 RouterV2 는 v1 Router 의
// 결정을 byte-identical 로 forward 해야 한다 (REQ-RV2-002).
type PolicyMode int

const (
	// PreferQuality 는 v1 Router 의 simple/complex 결정을 변경 없이 유지한다.
	// RoutingPolicy 의 zero value 이며, routing-policy.yaml 이 없을 때 적용된다.
	PreferQuality PolicyMode = iota

	// PreferLocal 은 Ollama 등 로컬 후보를 우선한다. 로컬 후보가 없으면
	// v1 결정을 그대로 유지한다 (silent fallback).
	PreferLocal

	// PreferCheap 은 정적 pricing 표 (spec.md §6.2) 의 per-million $
	// 평균값 오름차순으로 후보를 정렬하고 가장 저렴한 후보를 선택한다.
	PreferCheap

	// AlwaysSpecific 은 FallbackChain 의 0번째 ProviderRef 를 v1 결정
	// 무시하고 강제 선택한다 (REQ-RV2-008).
	AlwaysSpecific
)

// String 은 PolicyMode 의 정규 YAML 표기를 반환한다 (loader 와 round-trip 가능).
func (m PolicyMode) String() string {
	switch m {
	case PreferQuality:
		return "prefer_quality"
	case PreferLocal:
		return "prefer_local"
	case PreferCheap:
		return "prefer_cheap"
	case AlwaysSpecific:
		return "always_specific"
	default:
		return "unknown"
	}
}

// Capability 는 RouterV2 가 후보 필터링에 사용하는 provider 능력 이름이다.
// 정적 CapabilityMatrix (P2 에서 정의) 의 column 키와 1:1 대응한다.
type Capability string

const (
	// CapPromptCaching 은 prompt cache 지원 (현재 anthropic 단독).
	CapPromptCaching Capability = "prompt_caching"

	// CapFunctionCalling 은 tool/function calling 지원.
	CapFunctionCalling Capability = "function_calling"

	// CapVision 은 multimodal vision 입력 지원.
	CapVision Capability = "vision"

	// CapRealtime 은 realtime/voice 양방향 스트림 지원 (현재 openai 단독).
	CapRealtime Capability = "realtime"
)

// ProviderRef 는 fallback_chain 의 단일 항목이다. provider id (e.g. "anthropic")
// + model id (e.g. "claude-sonnet-4.6") 쌍으로 구성된다.
type ProviderRef struct {
	// Provider 는 ROUTER-001 ProviderRegistry 의 provider id 이다.
	Provider string `yaml:"provider"`

	// Model 은 provider 내부 model id 이다.
	Model string `yaml:"model"`
}

// DefaultRateLimitThreshold 는 RouterV2 가 RateLimitView.BucketUsage 의
// 4 bucket 중 하나라도 이 값 이상이면 후보에서 제외하는 기본 임계 (REQ-RV2-009).
const DefaultRateLimitThreshold = 0.80

// RoutingPolicy 는 ~/.goose/routing-policy.yaml 의 메모리 표현이다.
// zero value 는 "no opinion" 으로 v1 byte-identical 동작을 의미한다.
//
// SPEC §6.3 schema 와 1:1 매핑 — 추가 필드는 amendment SPEC 필요.
type RoutingPolicy struct {
	// Mode 는 routing 우선순위 (zero value: PreferQuality).
	Mode PolicyMode `yaml:"-"`

	// RateLimitThreshold 는 BucketUsage 임계값 (0.0~1.0, 기본 0.80).
	// loader 가 nil/빈 값일 때 DefaultRateLimitThreshold 로 채운다.
	RateLimitThreshold float64 `yaml:"rate_limit_threshold,omitempty"`

	// RequiredCapabilities 는 N 개 capability 모두를 지원하는 provider 만
	// 후보로 통과시킨다 (REQ-RV2-010).
	RequiredCapabilities []Capability `yaml:"required_capabilities,omitempty"`

	// ExcludedProviders 는 fallback chain 실행 중에도 silent skip 되는
	// provider id 목록이다 (REQ-RV2-012).
	ExcludedProviders []string `yaml:"excluded_providers,omitempty"`

	// FallbackChain 은 사용자 정의 후보 시도 순서이다.
	// AlwaysSpecific 모드에서 0번째가 강제 선택된다 (REQ-RV2-008).
	FallbackChain []ProviderRef `yaml:"fallback_chain,omitempty"`
}

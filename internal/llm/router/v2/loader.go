// Package v2 — loader.go: routing-policy.yaml 의 안전한 파싱.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-001 / REQ-RV2-002
package v2

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrUnknownPolicyMode 는 routing-policy.yaml 의 mode 필드가 4 enum
// (prefer_local / prefer_cheap / prefer_quality / always_specific)
// 중 어느 것에도 해당하지 않을 때 LoadPolicy 가 반환하는 sentinel 이다.
var ErrUnknownPolicyMode = errors.New("v2: unknown policy mode")

// ErrInvalidThreshold 는 rate_limit_threshold 가 [0.0, 1.0] 범위를
// 벗어났을 때 LoadPolicy 가 반환하는 sentinel 이다.
var ErrInvalidThreshold = errors.New("v2: rate_limit_threshold must be in [0.0, 1.0]")

// rawPolicy 는 YAML 디코딩 중간 표현이다. PolicyMode 는 enum 으로 직접
// 디코딩할 수 없어 string 으로 받은 뒤 매핑한다.
type rawPolicy struct {
	Mode                 string        `yaml:"mode"`
	RateLimitThreshold   *float64      `yaml:"rate_limit_threshold"`
	RequiredCapabilities []Capability  `yaml:"required_capabilities"`
	ExcludedProviders    []string      `yaml:"excluded_providers"`
	FallbackChain        []ProviderRef `yaml:"fallback_chain"`
}

// LoadPolicy 는 path 의 routing-policy.yaml 을 RoutingPolicy 로 로드한다.
//
// ctx 는 caller 의 cancellation propagation 용이다. os.ReadFile 자체는
// context 를 직접 지원하지 않지만, 진입 시점에 ctx.Err() 를 한 번 검사해
// 이미 취소된 컨텍스트로 호출되는 경우를 즉시 거절한다. 신규 public API
// 이므로 시그니처를 지금 잡아두는 것이 후속 호환성 비용을 절감한다.
//
// 동작 규칙:
//   - ctx 가 이미 취소됨 → ctx.Err() 를 wrap 해 반환.
//   - 파일 부재 (os.IsNotExist) → RoutingPolicy{Mode: PreferQuality, RateLimitThreshold: 0.80} + nil error.
//     이는 backward-compat fast path 의 핵심으로 v1 Router 와 byte-identical 동작을 보장한다.
//   - 파일은 있으나 mode 미지정 → PreferQuality 기본값.
//   - mode 가 4 enum 이 아닌 임의 문자열 → ErrUnknownPolicyMode.
//   - rate_limit_threshold 가 [0.0, 1.0] 밖 → ErrInvalidThreshold.
//   - rate_limit_threshold 미지정 → DefaultRateLimitThreshold (0.80).
//   - YAML 자체가 malformed → io/parse 에러를 그대로 wrap.
func LoadPolicy(ctx context.Context, path string) (RoutingPolicy, error) {
	if err := ctx.Err(); err != nil {
		return RoutingPolicy{}, fmt.Errorf("v2: load policy: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RoutingPolicy{
				Mode:               PreferQuality,
				RateLimitThreshold: DefaultRateLimitThreshold,
			}, nil
		}
		return RoutingPolicy{}, fmt.Errorf("v2: read policy %s: %w", path, err)
	}

	var raw rawPolicy
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return RoutingPolicy{}, fmt.Errorf("v2: parse policy %s: %w", path, err)
		}
	}

	mode, err := parsePolicyMode(raw.Mode)
	if err != nil {
		return RoutingPolicy{}, err
	}

	threshold := DefaultRateLimitThreshold
	if raw.RateLimitThreshold != nil {
		threshold = *raw.RateLimitThreshold
		if threshold < 0.0 || threshold > 1.0 {
			return RoutingPolicy{}, fmt.Errorf("%w: got %v", ErrInvalidThreshold, threshold)
		}
	}

	return RoutingPolicy{
		Mode:                 mode,
		RateLimitThreshold:   threshold,
		RequiredCapabilities: raw.RequiredCapabilities,
		ExcludedProviders:    raw.ExcludedProviders,
		FallbackChain:        raw.FallbackChain,
	}, nil
}

// parsePolicyMode 는 YAML 의 mode 문자열을 PolicyMode enum 으로 매핑한다.
// 빈 문자열은 PreferQuality (zero value) 로, 그 외 unrecognised 값은
// ErrUnknownPolicyMode 로 분류한다.
func parsePolicyMode(s string) (PolicyMode, error) {
	switch s {
	case "", "prefer_quality":
		return PreferQuality, nil
	case "prefer_local":
		return PreferLocal, nil
	case "prefer_cheap":
		return PreferCheap, nil
	case "always_specific":
		return AlwaysSpecific, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownPolicyMode, s)
	}
}

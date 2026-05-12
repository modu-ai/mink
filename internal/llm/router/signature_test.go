// Package router_test는 router 패키지의 외부 테스트를 포함한다.
package router_test

import (
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/llm/router"
)

// TestRouter_Signature_ArgsOrderIndependent는 Args 키 순서가 달라도
// 동일한 Signature를 반환하는지 검증한다 (canonical JSON 키 정렬).
func TestRouter_Signature_ArgsOrderIndependent(t *testing.T) {
	t.Parallel()

	// Args 포함한 RouteDefinition (두 가지 순서)
	cfg1 := router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
			Args: map[string]any{
				"max_tokens":  1000,
				"temperature": 0.7,
			},
		},
		ForceMode: router.ForceModeAuto,
	}
	cfg2 := router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
			Args: map[string]any{
				"temperature": 0.7,
				"max_tokens":  1000,
			},
		},
		ForceMode: router.ForceModeAuto,
	}

	r1, err := router.New(cfg1, router.DefaultRegistry(), zap.NewNop())
	if err != nil {
		t.Fatalf("router1 생성 실패: %v", err)
	}
	r2, err := router.New(cfg2, router.DefaultRegistry(), zap.NewNop())
	if err != nil {
		t.Fatalf("router2 생성 실패: %v", err)
	}

	req := router.RoutingRequest{
		Messages: []router.Message{{Role: "user", Content: "hello"}},
	}

	route1, err := r1.Route(nil, req) //nolint:staticcheck
	if err != nil {
		t.Fatalf("r1.Route() 에러: %v", err)
	}
	route2, err := r2.Route(nil, req) //nolint:staticcheck
	if err != nil {
		t.Fatalf("r2.Route() 에러: %v", err)
	}

	if route1.Signature != route2.Signature {
		t.Errorf("Args 순서만 다를 때 Signature 불일치: %q vs %q",
			route1.Signature, route2.Signature)
	}
}

// TestRouter_Signature_ArgsChangeCausesNewSignature는 Args 값이 변경되면
// 다른 Signature를 반환하는지 검증한다.
func TestRouter_Signature_ArgsChangeCausesNewSignature(t *testing.T) {
	t.Parallel()

	makeConfigWithTokens := func(maxTokens int) router.RoutingConfig {
		return router.RoutingConfig{
			Primary: router.RouteDefinition{
				Model:    "claude-opus",
				Provider: "anthropic",
				Mode:     "chat",
				Command:  "messages.create",
				Args:     map[string]any{"max_tokens": maxTokens},
			},
			ForceMode: router.ForceModeAuto,
		}
	}

	r1, _ := router.New(makeConfigWithTokens(1000), router.DefaultRegistry(), zap.NewNop())
	r2, _ := router.New(makeConfigWithTokens(2000), router.DefaultRegistry(), zap.NewNop())

	req := router.RoutingRequest{
		Messages: []router.Message{{Role: "user", Content: "hello"}},
	}

	route1, _ := r1.Route(nil, req) //nolint:staticcheck
	route2, _ := r2.Route(nil, req) //nolint:staticcheck

	if route1.Signature == route2.Signature {
		t.Error("Args 값이 다를 때 동일 Signature — 충돌")
	}
}

// TestRouter_Signature_NoTimestamp는 Signature에 타임스탬프 패턴이
// 없는지 검증한다 (REQ-ROUTER-014).
func TestRouter_Signature_NoTimestamp(t *testing.T) {
	t.Parallel()

	r, err := router.New(router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
		},
		ForceMode: router.ForceModeAuto,
	}, router.DefaultRegistry(), zap.NewNop())
	if err != nil {
		t.Fatalf("Router 생성 실패: %v", err)
	}

	req := router.RoutingRequest{
		Messages: []router.Message{{Role: "user", Content: "hello"}},
	}

	route, err := r.Route(nil, req) //nolint:staticcheck
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	sig := route.Signature
	// Signature는 파이프로 구분된 필드여야 하며 Unix timestamp처럼 긴 숫자만으로 된 필드가 없어야 함
	parts := strings.Split(sig, "|")
	for _, part := range parts {
		// 13자리 이상 순수 숫자 = 타임스탬프 의심
		if len(part) >= 13 && isAllDigits(part) {
			t.Errorf("Signature에 타임스탬프 의심 필드 발견: %q (전체: %q)", part, sig)
		}
	}
}

// isAllDigits는 문자열이 모두 숫자로 이루어져 있는지 확인한다.
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// TestRouter_Signature_EmptyArgs는 Args가 nil/비어 있을 때도
// Signature가 정상적으로 생성되는지 검증한다.
func TestRouter_Signature_EmptyArgs(t *testing.T) {
	t.Parallel()

	cfg := router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
			Args:     nil, // nil Args
		},
		ForceMode: router.ForceModeAuto,
	}

	r, err := router.New(cfg, router.DefaultRegistry(), zap.NewNop())
	if err != nil {
		t.Fatalf("Router 생성 실패: %v", err)
	}

	req := router.RoutingRequest{
		Messages: []router.Message{{Role: "user", Content: "hello"}},
	}

	route, err := r.Route(nil, req) //nolint:staticcheck
	if err != nil {
		t.Fatalf("Route() 에러: %v", err)
	}

	if route.Signature == "" {
		t.Error("빈 Args일 때 Signature가 비어 있음")
	}

	// nil Args와 빈 map Args의 Signature가 동일해야 함 (canonical "{}")
	cfgEmpty := router.RoutingConfig{
		Primary: router.RouteDefinition{
			Model:    "claude-opus",
			Provider: "anthropic",
			Mode:     "chat",
			Command:  "messages.create",
			Args:     map[string]any{}, // 빈 map
		},
		ForceMode: router.ForceModeAuto,
	}

	rEmpty, _ := router.New(cfgEmpty, router.DefaultRegistry(), zap.NewNop())
	routeEmpty, _ := rEmpty.Route(nil, req) //nolint:staticcheck

	if route.Signature != routeEmpty.Signature {
		t.Errorf("nil Args vs 빈 map Args Signature 불일치: %q vs %q",
			route.Signature, routeEmpty.Signature)
	}
}

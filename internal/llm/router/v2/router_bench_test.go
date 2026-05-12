// Package v2 — router_bench_test.go: NFR §11 latency benchmark.
//
// 목표: BenchmarkRouterV2_Route 의 평균 ns/op 가 5_000_000 (5ms) 미만.
// 본 SPEC 의 routing decision 자체는 in-memory 결정이므로 일반적으로
// μs 수준이지만, regression 발생 시 알림 역할.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
package v2

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/llm/router"
)

// BenchmarkRouterV2_Route_ZeroPolicy 는 zero policy 통과 경로의 핫패스 측정.
func BenchmarkRouterV2_Route_ZeroPolicy(b *testing.B) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)
	ctx := context.Background()
	req := router.RoutingRequest{}

	for b.Loop() {
		_, _ = r2.Route(ctx, req)
	}
}

// BenchmarkRouterV2_Route_PreferCheap 는 의사결정 트리 전체 경로 측정.
// capability + ratelimit + exclude + sort 모두 적용된 worst-case.
func BenchmarkRouterV2_Route_PreferCheap(b *testing.B) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	view := &fakeRateLimitView{usage: map[string][4]float64{
		"anthropic": {0.5, 0.5, 0.5, 0.5},
	}}
	policy := RoutingPolicy{
		Mode:                 PreferCheap,
		RequiredCapabilities: []Capability{CapFunctionCalling},
		ExcludedProviders:    []string{"weird"},
		RateLimitThreshold:   0.80,
		FallbackChain: []ProviderRef{
			{Provider: "anthropic", Model: "claude-opus-4-7"},
			{Provider: "openai", Model: "gpt-4o"},
			{Provider: "google", Model: "gemini-2.0-flash"},
			{Provider: "ollama", Model: "llama3-8b"},
			{Provider: "mistral", Model: "nemo"},
			{Provider: "groq", Model: "llama-3.3-70b"},
			{Provider: "cerebras", Model: "llama-3.3-70b"},
			{Provider: "deepseek", Model: "deepseek-chat"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), view, nil)
	ctx := context.Background()
	req := router.RoutingRequest{}

	for b.Loop() {
		_, _ = r2.Route(ctx, req)
	}
}

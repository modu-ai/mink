// Package v2 — integration_test.go: end-to-end 통합 검증.
//
// 5 fixture E2E + OpenRouter 제외 정책 회귀 + fallback chain 14 reason
// 분기 통합 검증. 본 파일은 P4 owner 이며 P1/P2/P3 의 산출물 (loader,
// matrix, ratelimit_filter, fallback, router, pricing, trace) 을 모두
// 실 인스턴스로 wiring 해 end-to-end 동작을 보장한다.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-001 ~ REQ-RV2-014
// AC: AC-RV2-004, AC-RV2-005, AC-RV2-011 (E2E focus)
package v2

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/evolve/errorclass"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeRealV1Router 는 통합 테스트용 실 v1 router.Router 를 만든다.
//
// primary=anthropic, cheap=groq 의 기본 설정으로 DefaultRegistry 와
// zaptest.NewLogger 를 wiring 한다. 본 함수가 실패하면 v1 v1.0.0 의
// 인터페이스가 변경된 것 — 본 SPEC 의 Risks 에 명시된 baseline 가정 위반.
func makeRealV1Router(t *testing.T) *router.Router {
	t.Helper()
	cfg := router.RoutingConfig{
		Primary: router.RouteDefinition{
			Provider: "anthropic",
			Model:    "claude-opus-4-7",
		},
		CheapRoute: &router.RouteDefinition{
			Provider: "groq",
			Model:    "llama-3.3-70b",
		},
	}
	registry := router.DefaultRegistry()
	logger := zaptest.NewLogger(t)
	r, err := router.New(cfg, registry, logger)
	require.NoError(t, err, "v1 router.New 가 실패하면 본 SPEC 가정이 깨진 것")
	return r
}

// loadFixture 는 testdata/ 의 YAML fixture 를 RoutingPolicy 로 로드한다.
func loadFixture(t *testing.T, name string) RoutingPolicy {
	t.Helper()
	path := filepath.Join("testdata", name)
	policy, err := LoadPolicy(context.Background(), path)
	require.NoError(t, err, "fixture %s 로드 실패", name)
	return policy
}

// makeUserReq 는 통합 테스트용 RoutingRequest 를 만든다. 단순한 user
// 메시지만 포함되며 v1 classifier 가 simple/complex 판정한다.
func makeUserReq(content string) router.RoutingRequest {
	return router.RoutingRequest{
		Messages: []router.Message{
			{Role: "user", Content: content},
		},
	}
}

// TestE2E_PreferLocal_OllamaSelected 는 prefer_local fixture 가 ollama 를
// 선택하는지 end-to-end 검증한다 (REQ-RV2-001 + capability matrix + pricing).
func TestE2E_PreferLocal_OllamaSelected(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_prefer_local.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	got, err := r2.Route(context.Background(), makeUserReq("Quick question"))
	require.NoError(t, err)
	assert.Equal(t, "ollama", got.Provider, "PreferLocal → ollama 우선")
	assert.Contains(t, got.RoutingReason, "v2:policy_prefer_local_ollama")
}

// TestE2E_PreferCheap_GroqFreeFirst 는 prefer_cheap fixture 가 chain 을
// pricing 표 오름차순으로 정렬해 무료 tier groq 를 선택하는지 검증한다
// (REQ-RV2-007).
func TestE2E_PreferCheap_GroqFreeFirst(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_prefer_cheap.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	got, err := r2.Route(context.Background(), makeUserReq("Hello"))
	require.NoError(t, err)
	// chain: anthropic:opus(45), openai:gpt-4o(6.25), groq:llama-3.3-70b(0.0)
	// PreferCheap 정렬 → groq 가 가장 저렴.
	assert.Equal(t, "groq", got.Provider, "PreferCheap → 무료 tier groq 우선")
	assert.Equal(t, "llama-3.3-70b", got.Model)
}

// TestE2E_PreferQuality_KeepsV1Anthropic 는 prefer_quality fixture 가 v1
// 의 결정 (anthropic primary) 을 우선하는지 검증한다.
func TestE2E_PreferQuality_KeepsV1Anthropic(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_prefer_quality.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	// 복잡한 메시지로 v1 이 primary (anthropic) 을 선택하도록 유도.
	got, err := r2.Route(context.Background(), makeUserReq(
		"Implement a complex distributed system with eventual consistency and CRDT "+
			"merge resolution. Provide detailed analysis of trade-offs."))
	require.NoError(t, err)
	assert.Equal(t, "anthropic", got.Provider, "PreferQuality + v1 anthropic 결정 유지")
}

// TestE2E_AlwaysSpecific_OverridesAll 는 always_specific fixture 가 v1 의
// 결정을 무시하고 chain[0] (mistral) 를 강제 선택하는지 검증한다 (AC-RV2-002).
func TestE2E_AlwaysSpecific_OverridesAll(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_always_specific.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	// v1 이 anthropic (primary) 또는 groq (cheap) 을 골라도 v2 가 무시.
	got, err := r2.Route(context.Background(), makeUserReq("Implement a complex algorithm"))
	require.NoError(t, err)
	assert.Equal(t, "mistral", got.Provider, "AlwaysSpecific → chain[0] mistral 강제")
	assert.Equal(t, "nemo", got.Model)
	assert.Contains(t, got.RoutingReason, "v2:policy_always_specific_mistral")
}

// TestE2E_WithExcluded_SkipsAnthropicChainUsesOpenAI 는 excluded_providers
// fixture 가 anthropic 을 chain 에서 silent skip 하고 openai 를 선택하는지
// 검증한다 (REQ-RV2-012, AC-RV2-008).
func TestE2E_WithExcluded_SkipsAnthropic(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_with_excluded.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	got, err := r2.Route(context.Background(), makeUserReq("Hello"))
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Provider, "anthropic excluded → chain[1] openai")
}

// TestE2E_OpenRouterInChain_NoSpecialTreatment 는 OpenRouter 가 chain 에
// 명시되어도 routing 결정에 우대 적용을 받지 않고 단순 provider 로 취급
// 되는지 검증한다 (AC-RV2-011 + spec.md §2.2 OpenRouter 의도적 제외 정책).
//
// 시나리오: PreferCheap + chain=[openrouter:gpt-oss-120b, openai:gpt-4o].
// 기대: openrouter 는 정적 pricing 표에 없음 → +Inf 로 정렬 → openai 가 먼저.
//
// 이 테스트가 실패하면 OpenRouter 가 우대를 받기 시작했다는 것 — spec.md
// §14 의 의도적 제외 정책 위반 회귀.
func TestE2E_OpenRouterInChain_NoSpecialTreatment(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := loadFixture(t, "policy_with_openrouter.yaml")
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)

	got, err := r2.Route(context.Background(), makeUserReq("Hello"))
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Provider, "OpenRouter 정적 pricing 부재 → SortByCost +Inf → 마지막")

	// 회귀 보호: pricing 표에 openrouter 가 추가되면 본 테스트가 깨지므로
	// SPEC amendment 필요 신호.
	_, hasOpenRouter := LookupPrice("openrouter:gpt-oss-120b")
	assert.False(t, hasOpenRouter, "openrouter 가 pricing 표에 추가되면 SPEC §14 amendment 필요")
}

// TestE2E_FallbackChain_RateLimitToContextOverflow 는 fallback chain 의
// 14 FailoverReason 분기가 통합 동작하는지 검증한다 (AC-RV2-007 + REQ-RV2-013).
//
// 시나리오:
//  1. 1차 anthropic 호출 → "rate limit exceeded" → ErrorClassifier → RateLimit (NEXT)
//  2. 2차 openai 호출 → "context length exceeded" → ContextOverflow (STOP)
//  3. 3차 google 은 호출되지 않음 (chain 즉시 중단).
//
// 본 테스트가 통과하면:
//   - RoutingReason 의 메시지 → reason 매핑이 정상 (errorclass.matchMessageRegex)
//   - FallbackExecutor 의 11+3 분기 정책이 wiring 됨
//   - LastAttempts 에 시도 기록이 보존됨 (REQ-RV2-011)
func TestE2E_FallbackChain_RateLimitToContextOverflow(t *testing.T) {
	cls := errorclass.New(errorclass.ClassifierOptions{})
	exec := NewFallbackExecutor(cls)
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "google", Model: "gemini-2.0-flash"},
	}

	calls := 0
	fn := func(_ context.Context, ref ProviderRef) (any, error) {
		calls++
		switch calls {
		case 1:
			return nil, errors.New("rate limit exceeded for " + ref.Provider)
		case 2:
			return nil, errors.New("context length exceeded for " + ref.Provider)
		default:
			t.Fatalf("3rd candidate should not be called (stop chain triggered)")
			return nil, nil
		}
	}

	_, err := exec.Execute(context.Background(), chain, fn)
	require.Error(t, err)

	var ferr *FallbackError
	require.True(t, errors.As(err, &ferr), "FallbackError 반환 필요")
	assert.True(t, ferr.Stopped, "ContextOverflow 는 STOP_CHAIN")
	assert.Equal(t, errorclass.ContextOverflow, ferr.LastReason)
	require.Len(t, ferr.Attempts, 2, "stopped after 2 attempts")
	assert.Equal(t, errorclass.RateLimit, ferr.Attempts[0].Reason, "1차: RateLimit")
	assert.Equal(t, "anthropic", ferr.Attempts[0].Provider)
	assert.Equal(t, errorclass.ContextOverflow, ferr.Attempts[1].Reason, "2차: ContextOverflow")
	assert.Equal(t, "openai", ferr.Attempts[1].Provider)
	assert.Equal(t, 2, calls, "google 은 호출되지 않아야 함")
}

// TestE2E_FixturesExist 는 5 fixture 파일이 모두 존재하고 LoadPolicy 가
// 모두 valid 한지 검증한다 (P4-T1 산출물 회귀 보호).
func TestE2E_FixturesExist(t *testing.T) {
	fixtures := []string{
		"policy_prefer_local.yaml",
		"policy_prefer_cheap.yaml",
		"policy_prefer_quality.yaml",
		"policy_always_specific.yaml",
		"policy_with_excluded.yaml",
		"policy_with_openrouter.yaml",
	}
	for _, f := range fixtures {
		t.Run(f, func(t *testing.T) {
			policy := loadFixture(t, f)
			assert.NotZero(t, policy.RateLimitThreshold, "default 0.80 적용")
		})
	}
}

// TestE2E_VisionFilter_E2E 는 capability matrix vision filter 가 E2E 에서
// 동작하는지 검증한다 (AC-RV2-004).
//
// vision 미지원 provider (groq, deepseek, cerebras, mistral, kimi, zai_glm)
// 가 chain 에 포함되어도 RequiredCapabilities=[vision] 으로 모두 제거되고
// vision 지원 provider (google) 만 남는지 확인.
func TestE2E_VisionFilter_E2E(t *testing.T) {
	v1 := makeRealV1Router(t)
	policy := RoutingPolicy{
		Mode:                 PreferCheap,
		RequiredCapabilities: []Capability{CapVision},
		FallbackChain: []ProviderRef{
			{Provider: "groq", Model: "llama-3.3-70b"},
			{Provider: "deepseek", Model: "deepseek-chat"},
			{Provider: "cerebras", Model: "llama-3.3-70b"},
			{Provider: "mistral", Model: "nemo"},
			{Provider: "kimi", Model: "k2.6"},
			{Provider: "google", Model: "gemini-2.0-flash"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), makeUserReq("Describe this image"))
	require.NoError(t, err)
	assert.Equal(t, "google", got.Provider, "vision 미지원 5 provider 제거 → google 단독")
}

// TestE2E_RateLimit80Pct_E2E 는 RateLimit 80% 임계 회피가 E2E 에서
// 동작하는지 검증한다 (AC-RV2-005).
func TestE2E_RateLimit80Pct_E2E(t *testing.T) {
	v1 := makeRealV1Router(t)
	view := &fakeRateLimitView{usage: map[string][4]float64{
		"anthropic": {0.85, 0.0, 0.0, 0.0}, // RPM 85% → 80% 초과
		"openai":    {0.50, 0.50, 0.50, 0.50},
	}}
	policy := RoutingPolicy{
		Mode:               PreferCheap,
		RateLimitThreshold: 0.80,
		FallbackChain: []ProviderRef{
			{Provider: "anthropic", Model: "claude-opus-4-7"},
			{Provider: "openai", Model: "gpt-4o"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), view, nil)
	got, err := r2.Route(context.Background(), makeUserReq("Hello"))
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Provider, "anthropic 가 RPM 85% → 80% 임계 초과로 제외")
}

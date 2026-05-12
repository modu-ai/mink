// Package v2 — router_test.go: RouterV2 의사결정 트리 검증.
//
// 7-step 의사결정 트리 (spec.md §7.3):
//  1. v1.Route() → primary/cheap 1차 결정
//  2. policy 가 zero (no opinion) → v1 결정 byte-identical
//  3. AlwaysSpecific → chain[0] 강제 override
//  4. PreferLocal → ollama 우선
//  5. PreferCheap → pricing 표 오름차순
//  6. PreferQuality → v1 + Opus/GPT-4o 우선
//  7. capability + ratelimit + exclude filter → 첫 후보
//  8. 후보 0 개 → v1 fallback (silent recovery)
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
package v2

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/modu-ai/mink/internal/evolve/errorclass"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeV1Router 는 router.Router 의 의존을 끊기 위한 V1Router 구현이다.
// calls 는 atomic — Concurrent_Safe 테스트에서 race 없이 카운트한다.
type fakeV1Router struct {
	route *router.Route
	err   error
	calls atomic.Int64
}

func (f *fakeV1Router) Route(_ context.Context, _ router.RoutingRequest) (*router.Route, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	// 새 인스턴스를 반환 — caller 가 수정해도 다음 호출에 영향 없게.
	r := *f.route
	return &r, nil
}

// fakeRateLimitView 는 RateLimitView 의 정해진 응답을 반환하는 stub.
type fakeRateLimitView struct {
	usage map[string][4]float64 // provider → [rpm, tpm, rph, tph]
}

func (f *fakeRateLimitView) BucketUsage(provider string) (rpm, tpm, rph, tph float64) {
	if u, ok := f.usage[provider]; ok {
		return u[0], u[1], u[2], u[3]
	}
	return 0, 0, 0, 0
}

// makeV1Route 는 fake v1 Route 생성 helper.
func makeV1Route(provider, model, reason string) *router.Route {
	return &router.Route{
		Provider:      provider,
		Model:         model,
		BaseURL:       "https://api.example.com",
		Mode:          "chat",
		Command:       "messages.create",
		RoutingReason: reason,
		Signature:     "v1-sig-stub",
	}
}

// TestRouterV2_ZeroPolicy_BytePassThrough 는 zero policy 시 v1 Route 가
// byte-identical 로 반환되는지 검증한다 (REQ-RV2-002, AC-RV2-001).
func TestRouterV2_ZeroPolicy_BytePassThrough(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", got.Provider)
	assert.Equal(t, "claude-opus-4-7", got.Model)
	assert.Equal(t, "complex_task", got.RoutingReason, "v1 RoutingReason 보존")
	assert.Equal(t, "v1-sig-stub", got.Signature, "v1 Signature 보존")
}

// TestRouterV2_V1Error_PassThrough 는 v1 이 error 반환 시 v2 도 그 error 를
// 그대로 반환하는지 검증한다.
func TestRouterV2_V1Error_PassThrough(t *testing.T) {
	v1 := &fakeV1Router{err: errors.New("v1-failed")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)
	_, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v1-failed")
}

// TestRouterV2_AlwaysSpecific_OverridesV1 는 AlwaysSpecific 모드 + chain[0]
// 이 v1 결정을 무시하고 강제 선택되는지 검증한다 (REQ-RV2-008, AC-RV2-002).
func TestRouterV2_AlwaysSpecific_OverridesV1(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode: AlwaysSpecific,
		FallbackChain: []ProviderRef{
			{Provider: "groq", Model: "llama-3.3-70b"},
			{Provider: "openai", Model: "gpt-4o"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "groq", got.Provider)
	assert.Equal(t, "llama-3.3-70b", got.Model)
	assert.Contains(t, got.RoutingReason, "v2:policy_always_specific_groq")
}

// TestRouterV2_AlwaysSpecific_EmptyChain_FallsBackToV1 는 AlwaysSpecific
// 인데 chain 이 비어 있으면 v1 결정으로 fallback 되는지 검증한다.
func TestRouterV2_AlwaysSpecific_EmptyChain_FallsBackToV1(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{Mode: AlwaysSpecific, FallbackChain: nil}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", got.Provider, "빈 chain 은 v1 으로 silent recovery")
}

// TestRouterV2_PreferCheap_SortsByCost 는 PreferCheap 모드가 chain 을
// pricing 표 오름차순으로 정렬한 첫 후보를 반환하는지 검증한다 (AC-RV2-003).
func TestRouterV2_PreferCheap_SortsByCost(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode: PreferCheap,
		FallbackChain: []ProviderRef{
			{Provider: "openai", Model: "gpt-4o"},             // 6.25
			{Provider: "anthropic", Model: "claude-opus-4-7"}, // 45.0
			{Provider: "ollama", Model: "llama3-8b"},          // 0.0
			{Provider: "mistral", Model: "nemo"},              // 0.02
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ollama", got.Provider, "최저가 ollama 선택")
}

// TestRouterV2_RequiredCapability_FiltersNonVision 는 Vision 필수 시
// vision 미지원 provider 를 후보에서 제거하는지 검증한다 (AC-RV2-004).
func TestRouterV2_RequiredCapability_FiltersNonVision(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode:                 PreferCheap,
		RequiredCapabilities: []Capability{CapVision},
		FallbackChain: []ProviderRef{
			{Provider: "deepseek", Model: "deepseek-chat"},  // no vision
			{Provider: "groq", Model: "llama-3.3-70b"},      // no vision
			{Provider: "google", Model: "gemini-2.0-flash"}, // vision ✓
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "google", got.Provider)
}

// TestRouterV2_RateLimit_ExcludesProviderAt80Pct 는 RateLimitView 가
// anthropic RPM=0.85 로 보고하면 anthropic 후보가 제외되는지 검증한다
// (REQ-RV2-009, AC-RV2-005).
func TestRouterV2_RateLimit_ExcludesProviderAt80Pct(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	view := &fakeRateLimitView{usage: map[string][4]float64{
		"anthropic": {0.85, 0.0, 0.0, 0.0}, // RPM 85%
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
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Provider, "anthropic 가 RPM 80% 초과로 제외되어야 함")
}

// TestRouterV2_Excluded_SilentSkip 는 ExcludedProviders 에 포함된 provider
// 가 silent skip 되는지 검증한다 (REQ-RV2-012, AC-RV2-008).
func TestRouterV2_Excluded_SilentSkip(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode:              AlwaysSpecific,
		ExcludedProviders: []string{"anthropic"},
		FallbackChain: []ProviderRef{
			{Provider: "anthropic", Model: "claude-opus-4-7"},
			{Provider: "openai", Model: "gpt-4o"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "openai", got.Provider, "anthropic excluded → openai")
}

// TestRouterV2_AllFiltered_RecoverV1 는 모든 후보가 capability/ratelimit/
// exclude 필터로 제거되면 v1 결정으로 silent recovery 하는지 검증한다
// (REQ-RV2-014, AC-RV2-009).
func TestRouterV2_AllFiltered_RecoverV1(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode:                 PreferCheap,
		RequiredCapabilities: []Capability{CapRealtime}, // openai 만 가능
		ExcludedProviders:    []string{"openai"},        // openai 제외 → 0개
		FallbackChain: []ProviderRef{
			{Provider: "anthropic", Model: "claude-opus-4-7"},
			{Provider: "openai", Model: "gpt-4o"},
			{Provider: "google", Model: "gemini-2.0-flash"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", got.Provider, "v1 fallback")
	assert.Equal(t, "v2:fallback_exhausted_recover_v1", got.RoutingReason)
}

// TestRouterV2_PreferLocal_PrependsOllama 는 PreferLocal 모드가 ollama 가
// chain 에 없어도 가장 우선되는지 검증한다.
func TestRouterV2_PreferLocal_PrependsOllama(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode: PreferLocal,
		FallbackChain: []ProviderRef{
			{Provider: "openai", Model: "gpt-4o"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ollama", got.Provider)
	assert.Contains(t, got.RoutingReason, "v2:policy_prefer_local_ollama")
}

// TestRouterV2_PreferLocal_ChainAlreadyHasOllama 는 chain 에 이미 ollama 가
// 있으면 새로 추가하지 않고 기존 위치를 우선하는지 검증한다.
func TestRouterV2_PreferLocal_ChainAlreadyHasOllama(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode: PreferLocal,
		FallbackChain: []ProviderRef{
			{Provider: "openai", Model: "gpt-4o"},
			{Provider: "ollama", Model: "llama3-70b"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ollama", got.Provider)
	assert.Equal(t, "llama3-70b", got.Model, "chain 의 ollama 모델 사용")
}

// TestRouterV2_PreferQuality_ChainNonEmpty_KeepsV1Decision 는 PreferQuality
// 모드 + chain 이 비어 있지 않을 때 (즉 zero-policy 가 아닐 때) v1 결정을
// 우선하는지 검증한다 — 단, capability/ratelimit/exclude 필터는 적용된다.
func TestRouterV2_PreferQuality_ChainNonEmpty_FiltersAndKeepsV1IfPasses(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	policy := RoutingPolicy{
		Mode: PreferQuality,
		FallbackChain: []ProviderRef{
			{Provider: "openai", Model: "gpt-4o"},
		},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, nil)
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	// v1 결정 (anthropic) 이 capability/ratelimit/exclude 모두 통과 → 유지.
	assert.Equal(t, "anthropic", got.Provider)
}

// TestRouterV2_TableDriven_DecisionMatrix 는 PolicyMode × Capability × Excluded
// 조합을 다수 검증한다 (plan.md "table-driven 50+ cases").
func TestRouterV2_TableDriven_DecisionMatrix(t *testing.T) {
	type tc struct {
		name         string
		policy       RoutingPolicy
		view         RateLimitView
		v1Provider   string
		wantProvider string
	}
	cases := []tc{
		{
			name:         "no_policy_v1_anthropic",
			policy:       RoutingPolicy{},
			v1Provider:   "anthropic",
			wantProvider: "anthropic",
		},
		{
			name:       "always_specific_chain0_groq",
			policy:     RoutingPolicy{Mode: AlwaysSpecific, FallbackChain: []ProviderRef{{Provider: "groq", Model: "llama-3.3-70b"}}},
			v1Provider: "anthropic", wantProvider: "groq",
		},
		{
			name: "prefer_cheap_filters_unknown_provider",
			policy: RoutingPolicy{Mode: PreferCheap, FallbackChain: []ProviderRef{
				{Provider: "weird", Model: "x"}, {Provider: "ollama", Model: "llama3"},
			}},
			v1Provider: "anthropic", wantProvider: "ollama",
		},
		{
			name: "vision_required_eliminates_groq",
			policy: RoutingPolicy{
				Mode:                 PreferCheap,
				RequiredCapabilities: []Capability{CapVision},
				FallbackChain: []ProviderRef{
					{Provider: "groq", Model: "llama-3.3-70b"},
					{Provider: "google", Model: "gemini-2.0-flash"},
					{Provider: "openai", Model: "gpt-4o"},
				},
			},
			v1Provider: "anthropic", wantProvider: "google",
		},
		{
			name: "realtime_required_only_openai",
			policy: RoutingPolicy{
				Mode:                 PreferCheap,
				RequiredCapabilities: []Capability{CapRealtime},
				FallbackChain: []ProviderRef{
					{Provider: "anthropic", Model: "claude-opus-4-7"},
					{Provider: "openai", Model: "gpt-4o"},
				},
			},
			v1Provider: "google", wantProvider: "openai",
		},
		{
			name: "ratelimit_85pct_excludes_anthropic",
			policy: RoutingPolicy{
				Mode:               PreferCheap,
				RateLimitThreshold: 0.80,
				FallbackChain: []ProviderRef{
					{Provider: "anthropic", Model: "claude-opus-4-7"},
					{Provider: "openai", Model: "gpt-4o"},
				},
			},
			view: &fakeRateLimitView{usage: map[string][4]float64{
				"anthropic": {0.85, 0, 0, 0},
			}},
			v1Provider: "anthropic", wantProvider: "openai",
		},
		{
			name: "exclude_anthropic_chain_uses_openai",
			policy: RoutingPolicy{
				Mode:              AlwaysSpecific,
				ExcludedProviders: []string{"anthropic"},
				FallbackChain: []ProviderRef{
					{Provider: "anthropic", Model: "claude-opus-4-7"},
					{Provider: "openai", Model: "gpt-4o"},
				},
			},
			v1Provider: "anthropic", wantProvider: "openai",
		},
		{
			name: "all_filtered_recovers_v1",
			policy: RoutingPolicy{
				Mode:                 PreferCheap,
				RequiredCapabilities: []Capability{CapRealtime},
				ExcludedProviders:    []string{"openai"},
				FallbackChain: []ProviderRef{
					{Provider: "anthropic", Model: "claude-opus-4-7"},
					{Provider: "openai", Model: "gpt-4o"},
				},
			},
			v1Provider: "google", wantProvider: "google",
		},
		{
			name: "prefer_local_no_chain_uses_default_ollama_model",
			policy: RoutingPolicy{
				Mode:          PreferLocal,
				FallbackChain: nil,
			},
			v1Provider: "anthropic", wantProvider: "ollama",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v1 := &fakeV1Router{route: makeV1Route(c.v1Provider, "stub-model", "complex_task")}
			r2 := New(v1, c.policy, DefaultMatrix(), c.view, nil)
			got, err := r2.Route(context.Background(), router.RoutingRequest{})
			require.NoError(t, err)
			assert.Equal(t, c.wantProvider, got.Provider)
		})
	}
}

// TestRouterV2_HookCalled 는 RoutingDecisionHook 가 등록되어 있을 때 v2 가
// 최종 Route 결정 직전 hook 을 호출하는지 검증한다 (REQ-RV2-006).
func TestRouterV2_HookCalled(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	called := false
	var observedRoute *router.Route
	hook := func(_ router.RoutingRequest, route *router.Route) {
		called = true
		observedRoute = route
	}
	policy := RoutingPolicy{
		Mode:          AlwaysSpecific,
		FallbackChain: []ProviderRef{{Provider: "groq", Model: "llama-3.3-70b"}},
	}
	r2 := New(v1, policy, DefaultMatrix(), nil, []router.RoutingDecisionHook{hook})
	got, err := r2.Route(context.Background(), router.RoutingRequest{})
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, observedRoute)
	assert.Equal(t, "groq", got.Provider)
	assert.Equal(t, got, observedRoute, "hook 가 최종 route 와 동일 포인터로 호출")
}

// TestRouterV2_Concurrent_Safe 는 동일 RouterV2 인스턴스가 여러 goroutine
// 에서 동시 호출 시 race detector 가 깨끗한지 검증한다 (NFR §11 Race-clean).
// 실제 race 검증은 -race flag 로 별도 수행.
func TestRouterV2_Concurrent_Safe(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)

	done := make(chan bool, 10)
	for range 10 {
		go func() {
			defer func() { done <- true }()
			_, err := r2.Route(context.Background(), router.RoutingRequest{})
			assert.NoError(t, err)
		}()
	}
	for range 10 {
		<-done
	}
}

// TestRouterV2_NewWithFallbackExecutor 는 외부 주입된 FallbackExecutor 가
// chain 실행 시 사용되는지 검증한다 (P3-T6 의 fallback executor wiring 테스트).
func TestRouterV2_FallbackExecutor_Default(t *testing.T) {
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)
	require.NotNil(t, r2.FallbackExecutor(), "default classifier 가 자동 생성되어야 함")
}

// TestRouterV2_ClassifyExternalErr_DriveFallback 는 외부에서 RouteAndExecute
// 같은 high-level API 가 fallback chain 실행을 트리거할 때 14 reason 분기가
// 적용되는지 검증한다.
//
// 본 SPEC 범위는 routing decision 만이므로 실제 LLM 호출은 OUT — 테스트는
// FallbackExecutor 의 Wiring 만 확인한다.
func TestRouterV2_FallbackExecutor_StopChainReason(t *testing.T) {
	cls := &fakeClassifier{reasons: []errorclass.FailoverReason{errorclass.ContextOverflow}}
	v1 := &fakeV1Router{route: makeV1Route("anthropic", "claude-opus-4-7", "complex_task")}
	r2 := New(v1, RoutingPolicy{}, DefaultMatrix(), nil, nil)
	r2.SetClassifier(cls)
	exec := r2.FallbackExecutor()
	chain := []ProviderRef{
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	}
	fn := &chainFn{errsByProvider: map[string]error{"anthropic": errors.New("ctx-overflow")}}
	_, err := exec.Execute(context.Background(), chain, fn.call)
	require.Error(t, err)
	var ferr *FallbackError
	require.True(t, errors.As(err, &ferr))
	assert.True(t, ferr.Stopped, "ContextOverflow 는 stop chain")
}

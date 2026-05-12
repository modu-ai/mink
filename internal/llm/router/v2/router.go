// Package v2 — router.go: RouterV2 decorator + 의사결정 트리.
//
// RouterV2 는 v1 router 위에 정책·capability·ratelimit·exclude 레이어를
// 얹는 decorator 이다. zero policy 시 v1 결정을 byte-identical 로 통과
// 시키고 (REQ-RV2-002), 정책이 활성화되면 7-step 의사결정 트리 (spec.md
// §7.3) 로 후보를 선정한다.
//
// 동시성: RouterV2.Route 는 stateless — 입력만으로 결정한다. 동일 인스턴스가
// 여러 goroutine 에서 안전하게 호출된다 (NFR §11 Race-clean).
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-001 ~ REQ-RV2-014
package v2

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/evolve/errorclass"
	"github.com/modu-ai/mink/internal/llm/router"
)

// V1Router 는 RouterV2 가 의존하는 v1 router.Router 의 최소 인터페이스이다.
// *router.Router 가 자동으로 만족하므로 별도 어댑터 없이 주입 가능하며,
// 테스트에서는 fake 로 대체할 수 있다.
type V1Router interface {
	Route(ctx context.Context, req router.RoutingRequest) (*router.Route, error)
}

// defaultLocalProvider 는 PreferLocal 모드에서 chain 에 ollama 가 없을
// 때 추가되는 fallback ProviderRef 이다. model id 는 가장 일반적인
// 8B 변종으로 — 사용자가 chain 에 명시 ollama 를 두면 그 model 이 우선.
var defaultLocalProvider = ProviderRef{Provider: "ollama", Model: "llama3"}

// RouterV2 는 정책·capability·ratelimit·exclude 인지 라우터이다.
//
// @MX:ANCHOR: [AUTO] v2 routing decision 단일 진입점 — QueryEngine + 후속
// SPEC 이 의존 (fan_in >= 3 예상)
// @MX:REASON: SPEC §7.3 7-step 의사결정 트리의 source-of-truth
type RouterV2 struct {
	base       V1Router
	policy     RoutingPolicy
	matrix     CapabilityMatrix
	view       RateLimitView
	hooks      []router.RoutingDecisionHook
	classifier errorclass.Classifier
	fallback   *FallbackExecutor
}

// New 는 V1Router (decorator base), RoutingPolicy, CapabilityMatrix,
// RateLimitView, hook list 를 받아 RouterV2 를 구성한다.
//
// 매개변수:
//   - base: v1 router.Router 또는 동일 시그니처 fake (필수)
//   - policy: 사용자 정의 라우팅 정책 (zero value = byte-identical pass-through)
//   - matrix: 정적 capability matrix (보통 DefaultMatrix())
//   - view: RATELIMIT-001 reader 어댑터 (nil 가능 — ratelimit 필터 비활성)
//   - hooks: 결정 직전 호출되는 관찰용 hook 목록 (nil 가능)
//
// FallbackExecutor 는 default classifier 와 함께 자동 생성된다. 이후
// SetClassifier 로 교체 가능 (테스트 / 배포별 정책).
func New(base V1Router, policy RoutingPolicy, matrix CapabilityMatrix, view RateLimitView, hooks []router.RoutingDecisionHook) *RouterV2 {
	cls := errorclass.New(errorclass.ClassifierOptions{})
	return &RouterV2{
		base:       base,
		policy:     policy,
		matrix:     matrix,
		view:       view,
		hooks:      hooks,
		classifier: cls,
		fallback:   NewFallbackExecutor(cls),
	}
}

// SetClassifier 는 RouterV2 의 ErrorClassifier 와 내부 FallbackExecutor 를
// 교체한다. 주로 테스트에서 reason 시나리오를 강제할 때 사용한다.
//
// 동시성: New 호출 후 첫 Route 호출 전에 호출하는 것을 권장. Route 호출
// 중 동시 SetClassifier 는 race condition 을 유발할 수 있으므로 caller 가
// 외부 동기화로 책임진다.
func (r *RouterV2) SetClassifier(cls errorclass.Classifier) {
	r.classifier = cls
	r.fallback = NewFallbackExecutor(cls)
	r.fallback.SetExcluded(r.policy.ExcludedProviders)
}

// FallbackExecutor 는 내부 FallbackExecutor 를 노출한다. caller 가 chain
// 실행 (RouteAndExecute 같은 high-level API 의 잠재적 hook) 시 사용한다.
// 본 SPEC 범위는 routing decision 까지이므로 실제 LLM 호출은 외부 API 책임.
func (r *RouterV2) FallbackExecutor() *FallbackExecutor {
	return r.fallback
}

// Route 는 RoutingRequest 를 받아 7-step 의사결정 트리를 통과한 Route 를
// 반환한다 (spec.md §7.3).
//
// 동작:
//  1. v1 router.Route() → primary/cheap 1차 결정 획득.
//  2. policy 가 zero (no opinion) → v1 결정 byte-identical 반환.
//  3. policy.Mode == AlwaysSpecific && chain[0] 있음 → chain[0] 강제.
//  4. policy.Mode == PreferLocal → ollama 우선 (없으면 chain 에 추가).
//  5. policy.Mode == PreferCheap → pricing 표 오름차순 정렬.
//  6. policy.Mode == PreferQuality → v1 결정 + chain 결합.
//  7. 후보 → capability filter → ratelimit filter → exclude filter.
//  8. 첫 후보를 v1 Route template 위에 substitute. 0 개면 v1 으로 silent recovery.
//  9. hook 호출 후 반환.
func (r *RouterV2) Route(ctx context.Context, req router.RoutingRequest) (*router.Route, error) {
	v1Route, err := r.base.Route(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("v2: base v1 Route: %w", err)
	}

	// REQ-RV2-002 backward-compat fast path — zero policy.
	if isZeroPolicy(r.policy) {
		r.callHooks(req, v1Route)
		return v1Route, nil
	}

	// 의사결정 트리 진입.
	candidates, traceReason := r.buildCandidates(v1Route)
	candidates = r.applyFilters(candidates)

	if len(candidates) == 0 {
		// REQ-RV2-014 silent recovery.
		v1Route.RoutingReason = TraceV2FallbackExhausted()
		r.callHooks(req, v1Route)
		return v1Route, nil
	}

	chosen := candidates[0]
	finalRoute := r.substitute(v1Route, chosen, traceReason)
	r.callHooks(req, finalRoute)
	return finalRoute, nil
}

// isZeroPolicy 는 RoutingPolicy 가 "no opinion" (zero value 동등) 인지 검사한다.
//
// zero policy 의 정의: PreferQuality 모드 + 모든 list 가 비어 있음.
// RateLimitThreshold 는 LoadPolicy 의 default 0.80 또는 0.0 인 zero value 도
// 허용 — view 가 nil 이면 어차피 ratelimit 필터가 비활성.
func isZeroPolicy(p RoutingPolicy) bool {
	return p.Mode == PreferQuality &&
		len(p.FallbackChain) == 0 &&
		len(p.RequiredCapabilities) == 0 &&
		len(p.ExcludedProviders) == 0
}

// buildCandidates 는 PolicyMode 별로 후보 list 와 trace reason 을 만든다.
// 후속 applyFilters 가 capability/ratelimit/exclude 를 적용한다.
func (r *RouterV2) buildCandidates(v1Route *router.Route) ([]ProviderRef, string) {
	switch r.policy.Mode {
	case AlwaysSpecific:
		if len(r.policy.FallbackChain) == 0 {
			return nil, "" // 빈 chain → applyFilters 0 개 → v1 silent recovery.
		}
		return append([]ProviderRef{}, r.policy.FallbackChain...), "always_specific"

	case PreferLocal:
		out := make([]ProviderRef, 0, len(r.policy.FallbackChain)+1)
		ollamaInChain := false
		var ollamaRef ProviderRef
		for _, ref := range r.policy.FallbackChain {
			if ref.Provider == "ollama" {
				ollamaInChain = true
				ollamaRef = ref
				continue // 일단 보류, 우선순위 head 에 배치.
			}
			out = append(out, ref)
		}
		if !ollamaInChain {
			ollamaRef = defaultLocalProvider
		}
		out = append([]ProviderRef{ollamaRef}, out...)
		return out, "prefer_local"

	case PreferCheap:
		// chain 이 비어 있으면 v1 결정을 후보로 (capability/ratelimit 통과 시 유지).
		if len(r.policy.FallbackChain) == 0 {
			return []ProviderRef{{Provider: v1Route.Provider, Model: v1Route.Model}}, "prefer_cheap"
		}
		return SortByCost(r.policy.FallbackChain), "prefer_cheap"

	case PreferQuality:
		// PreferQuality + non-empty chain → v1 결정을 head 에, chain 을 fallback.
		out := make([]ProviderRef, 0, len(r.policy.FallbackChain)+1)
		out = append(out, ProviderRef{Provider: v1Route.Provider, Model: v1Route.Model})
		for _, ref := range r.policy.FallbackChain {
			if ref.Provider == v1Route.Provider && ref.Model == v1Route.Model {
				continue // 중복 제거.
			}
			out = append(out, ref)
		}
		return out, "prefer_quality"
	}
	return nil, ""
}

// applyFilters 는 candidates 에 capability → exclude → ratelimit 필터를
// 순차 적용하고 남은 후보 slice 를 반환한다.
//
// 순서가 중요:
//  1. capability filter — provider 가 정적 매트릭스에서 capability 미지원 시 제거
//  2. exclude filter — RoutingPolicy.ExcludedProviders silent skip
//  3. ratelimit filter — 80% 임계 회피 (ratelimit view nil 이면 통과)
func (r *RouterV2) applyFilters(candidates []ProviderRef) []ProviderRef {
	if len(candidates) == 0 {
		return nil
	}

	// 1. Capability filter.
	if len(r.policy.RequiredCapabilities) > 0 {
		filtered := make([]ProviderRef, 0, len(candidates))
		for _, c := range candidates {
			if r.matrix.Match(c.Provider, r.policy.RequiredCapabilities) {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	// 2. Exclude filter.
	if len(r.policy.ExcludedProviders) > 0 {
		excluded := make(map[string]struct{}, len(r.policy.ExcludedProviders))
		for _, p := range r.policy.ExcludedProviders {
			excluded[p] = struct{}{}
		}
		filtered := make([]ProviderRef, 0, len(candidates))
		for _, c := range candidates {
			if _, skip := excluded[c.Provider]; skip {
				continue
			}
			filtered = append(filtered, c)
		}
		candidates = filtered
	}

	// 3. Ratelimit filter.
	threshold := r.policy.RateLimitThreshold
	if threshold == 0 {
		threshold = DefaultRateLimitThreshold
	}
	candidates = FilterByRateLimit(candidates, r.view, threshold)

	return candidates
}

// substitute 는 v1Route 를 template 으로 새 *router.Route 를 생성한다.
// chosen 의 provider/model 로 substitution 하고 RoutingReason 을 v2 prefix
// 로 설정한다.
//
// Signature 는 v1.Route 가 unexported helper 로 계산하므로 v2 가 그대로
// 재현할 수 없다. v2 substitution 시 Signature 는 v2-specific format 으로
// 새로 계산한다 ("v2|provider|model" 단순 fingerprint). v2 에서 변경된
// Route 의 signature 가 v1 과 다른 것은 의도된 동작 — caller 가 이를
// 구분할 수 있다.
func (r *RouterV2) substitute(v1Route *router.Route, chosen ProviderRef, traceKey string) *router.Route {
	out := *v1Route
	out.Provider = chosen.Provider
	out.Model = chosen.Model
	out.Signature = fmt.Sprintf("v2|%s|%s", chosen.Provider, chosen.Model)
	out.RoutingReason = r.buildReason(traceKey, chosen.Provider)
	return &out
}

// buildReason 은 trace key 별로 Trace*Builder 를 호출한다. 형식 source-of-
// truth 는 trace.go 이며 본 함수는 dispatch 만.
func (r *RouterV2) buildReason(traceKey, provider string) string {
	switch traceKey {
	case "always_specific":
		return TraceV2Policy(AlwaysSpecific, provider)
	case "prefer_local":
		return TraceV2Policy(PreferLocal, provider)
	case "prefer_cheap":
		return TraceV2Policy(PreferCheap, provider)
	case "prefer_quality":
		return TraceV2Policy(PreferQuality, provider)
	default:
		return "v2:unknown_trace"
	}
}

// callHooks 는 등록된 hook 들을 순서대로 호출한다 (REQ-RV2-006).
// hook 가 panic 해도 다음 hook 는 호출되어야 하므로 defer recover 보호한다.
func (r *RouterV2) callHooks(req router.RoutingRequest, route *router.Route) {
	for _, h := range r.hooks {
		func() {
			defer func() { _ = recover() }()
			h(req, route)
		}()
	}
}

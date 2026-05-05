# SPEC-GOOSE-LLM-ROUTING-V2-001 — Implementation Plan

> **Phase 진행 순서**: P1 (policy schema + loader) → P2 (capability matrix + ratelimit reader 어댑터) → P3 (fallback chain + RouterV2 decorator + pricing) → P4 (integration tests). 이유: P1 의 RoutingPolicy 가 P2/P3 모든 결정의 입력이므로 첫 phase 에 깔아야 함. P2 의 CapabilityMatrix + RateLimitView 가 P3 의 RouterV2 의사결정 트리 의존성. P4 는 모든 layer 가 안정된 후 end-to-end 회귀 보호.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 4-phase decomposition (P1 policy/loader, P2 capability/ratelimit, P3 fallback/decorator/pricing, P4 integration), TDD per phase, RED test 정의, plan_complete signal | manager-spec |

---

## 0. Approach Summary

**Greenfield decorator pattern**. 기존 `internal/llm/router/` (v1) 를 **수정 없이** 위에 `internal/llm/router/v2/` 를 신규 패키지로 얹는다. v1 Router 는 v2 의 `base router.Router` 필드로 주입되어 정책 미지정 시 byte-identical 동작 보장.

본 SPEC 은 **TDD per phase, RED → GREEN → REFACTOR** 사이클을 따른다 (`.moai/config/sections/quality.yaml` `development_mode: tdd` 기본 가정).

phase 별 file ownership:
- **P1** owner: `internal/llm/router/v2/policy.go`, `loader.go`. 외부 변경: 없음.
- **P2** owner: `internal/llm/router/v2/capability.go`, `ratelimit_filter.go`. 외부 read: RATELIMIT-001 reader 인터페이스 (signature 어댑터 작성).
- **P3** owner: `internal/llm/router/v2/fallback.go`, `router.go`, `pricing.go`, `trace.go`. 외부 read: ERROR-CLASS-001 `ErrorClassifier`, ROUTER-001 `Router` 인터페이스.
- **P4** owner: `internal/llm/router/v2/integration_test.go`, `testdata/policy_*.yaml` fixtures. mock provider via httptest.

phase 간 worktree 분리는 권장하지 않음 — 모든 phase 가 동일 패키지 (`internal/llm/router/v2/`) 에 집중되어 sequential merge 가 자연스러움.

---

## 1. Phase Decomposition (4 phases, 17 tasks 총)

### Phase 1 — RoutingPolicy schema + YAML loader (REQ-RV2-001, -002)

**Goal**: 사용자 정책 입력을 안전하게 파싱하고, 파일 부재/스키마 위반/알 수 없는 mode 를 명시적으로 처리.

**Files (NEW)**:
- `internal/llm/router/v2/policy.go` — `RoutingPolicy`, `PolicyMode`, `Capability`, `ProviderRef` struct + enum
- `internal/llm/router/v2/loader.go` — `LoadPolicy(path string) (RoutingPolicy, error)`

**Tasks**:
1. **P1-T1 (RED)** — `policy_test.go` 에서 `RoutingPolicy{}` zero value = `PreferQuality` mode 확인 (default 가정).
2. **P1-T2 (GREEN)** — `policy.go` 에서 `PolicyMode` iota, zero value 가 `PreferQuality` 가 되도록 enum 순서 결정.
3. **P1-T3 (RED)** — `loader_test.go` 에서 파일 부재 시 `RoutingPolicy{Mode: PreferQuality}` + nil error 반환 검증.
4. **P1-T4 (GREEN)** — `loader.go` 에서 `os.IsNotExist(err)` graceful 처리.
5. **P1-T5 (RED)** — 알 수 없는 mode (`mode: prefer_unicorn`) 시 명시적 `ErrUnknownPolicyMode` 반환 검증.
6. **P1-T6 (GREEN)** — `loader.go` 에서 string → enum 매핑 + 알 수 없는 mode 명시적 에러.
7. **P1-T7 (RED)** — `rate_limit_threshold` 범위 위반 (음수, > 1.0) 시 `ErrInvalidThreshold` 검증.
8. **P1-T8 (GREEN)** — schema validator 추가.
9. **P1-T9 (REFACTOR)** — error sentinel 정리, godoc 작성.

**RED tests (Initial failing)**:
- `TestPolicyModeZeroValueIsPreferQuality`
- `TestLoadPolicy_FileNotFound_ReturnsDefault`
- `TestLoadPolicy_UnknownMode_ReturnsError`
- `TestLoadPolicy_ThresholdOutOfRange_ReturnsError`
- `TestLoadPolicy_ValidYAML_ParsesAllFields`

**Coverage gate**: ≥ 90% for `policy.go` + `loader.go`

**Drift signal**: 본 phase 내에서 `internal/llm/router/v2/` 외부 파일 수정 0 건. 위반 시 P1 미완.

### Phase 2 — CapabilityMatrix + RateLimit reader 어댑터 (REQ-RV2-003, -004, -009, -010)

**Goal**: 정적 15×4 matrix 와 RATELIMIT-001 reader 추상화로 후보 필터링 인프라 완성.

**Files (NEW)**:
- `internal/llm/router/v2/capability.go` — `CapabilityMatrix` + `Match()` + 정적 init
- `internal/llm/router/v2/ratelimit_filter.go` — `RateLimitView` 인터페이스 + `FilterByRateLimit()`

**Files (READ-ONLY)**:
- `internal/llm/router/registry.go` (v1) — `ProviderRegistry.AdapterReady` 참조
- RATELIMIT-001 의 reader 인터페이스 (실 구현체 import)

**Tasks**:
10. **P2-T1 (RED)** — `capability_test.go` 의 table-driven 60 cases (15 provider × 4 capability) 작성, 정적 matrix 가 spec.md §6.1 표와 일치 검증.
11. **P2-T2 (GREEN)** — `capability.go` 정적 init + `Match([]Capability, provider)` 구현.
12. **P2-T3 (RED)** — `ratelimit_filter_test.go` mock RateLimitView 로 단일 bucket 80% threshold 검증.
13. **P2-T4 (GREEN)** — `ratelimit_filter.go` 의 `FilterByRateLimit(candidates, view, threshold)` 구현 — 4 bucket 중 하나라도 임계 이상이면 후보 제거.
14. **P2-T5 (RED)** — RATELIMIT-001 reader 와 본 SPEC `RateLimitView` 시그니처 어댑터 — fake reader 통과 검증.
15. **P2-T6 (REFACTOR)** — godoc 보강, capability matrix 의 source-of-truth comment 작성 (provider 신모델 추가 시 amendment 필요 명시).

**RED tests**:
- `TestCapabilityMatrix_StaticConsistency_15x4`
- `TestCapabilityMatrix_Match_AllRequired`
- `TestCapabilityMatrix_Match_OneMissing_Rejects`
- `TestFilterByRateLimit_RPMBucketAt80Percent_ExcludesProvider`
- `TestFilterByRateLimit_AllBucketsBelowThreshold_KeepsProvider`
- `TestFilterByRateLimit_OverrideThresholdTo50Percent_AppliesCorrectly`

**Coverage gate**: ≥ 90% for `capability.go` + `ratelimit_filter.go`

**Drift signal**: RATELIMIT-001 reader 시그니처가 v2 가정과 다른 경우 — 어댑터 layer 작성으로 흡수, RATELIMIT-001 본체 변경 0.

### Phase 3 — Fallback chain + RouterV2 decorator + Pricing (REQ-RV2-005~008, -011~014)

**Goal**: 14 FailoverReason 분기 + 정책 의사결정 트리 + decorator 패턴으로 v1 위 얹기 + 정적 pricing 표.

**Files (NEW)**:
- `internal/llm/router/v2/fallback.go` — `FallbackExecutor` + 14 reason switch
- `internal/llm/router/v2/router.go` — `RouterV2` decorator + `Route()` 의사결정 트리
- `internal/llm/router/v2/pricing.go` — 정적 per-million $ 표 + `SortByCost([]ProviderRef)`
- `internal/llm/router/v2/trace.go` — `RoutingReason` v2 prefix 빌더 (`v2:policy_*`, `v2:rate_limit_*`, `v2:fallback_*`)

**Files (READ-ONLY)**:
- ERROR-CLASS-001 `ErrorClassifier` + 14 `FailoverReason` enum
- ROUTER-001 `Router` 인터페이스, `Route` struct, `RoutingRequest`

**Tasks**:
16. **P3-T1 (RED)** — `fallback_test.go` 14 case (각 FailoverReason 1 cases) 로 분기 검증:
    - 11 reason → 다음 후보 시도
    - 3 reason (ContentFilter, ContextWindowExceeded, MalformedResponse) → chain 즉시 중단
17. **P3-T2 (GREEN)** — `fallback.go` switch-case 구현, `MultiError` wrapping.
18. **P3-T3 (RED)** — `pricing_test.go` 의 정적 표 검증 + `SortByCost()` 오름차순 검증.
19. **P3-T4 (GREEN)** — `pricing.go` 정적 init (`map[string]Price{Input, Output}`) + `SortByCost()`.
20. **P3-T5 (RED)** — `router_test.go` table-driven 50+ cases 로 의사결정 트리 검증:
    - PolicyMode (4) × Capability (4) × RateLimit (3 단계) × FailoverReason (5 cases) 조합
21. **P3-T6 (GREEN)** — `router.go` 의 `RouterV2.Route()` 의사결정 트리 구현 (spec.md §7.3 7-step).
22. **P3-T7 (RED)** — `trace_test.go` 로 RoutingReason v2 prefix 형식 검증 (spec.md §6.4 표).
23. **P3-T8 (GREEN)** — `trace.go` builder 함수 구현.
24. **P3-T9 (REFACTOR)** — race detector pass 확인 (`go test -race ./internal/llm/router/v2/...`), benchmark 추가 (`BenchmarkRouterV2_Route` < 5ms p99).

**RED tests (high-priority)**:
- `TestFallback_RateLimit_TriesNextCandidate`
- `TestFallback_ContentFilter_StopsImmediately`
- `TestFallback_AllCandidatesFail_ReturnsMultiError`
- `TestRouterV2_PolicyAlwaysSpecific_OverridesV1Decision`
- `TestRouterV2_PolicyPreferCheap_SortsByCostAscending`
- `TestRouterV2_RequiredCapabilityVision_FiltersNonVisionProviders`
- `TestRouterV2_RateLimit80Percent_ExcludesProvider`
- `TestRouterV2_AllCandidatesZero_FallsBackToV1Decision`
- `TestRouterV2_ExcludedProviderInChain_SilentSkip`
- `TestTraceBuilder_AllPrefixesMatchSpec`

**Coverage gate**: ≥ 90% for `fallback.go` + `router.go` + `pricing.go` + `trace.go`

**Drift signal**: ERROR-CLASS-001 14 FailoverReason 중 누락 case 발생 시 P3 미완 — switch-case `default` 가 panic 또는 silent ignore 가 되어선 안 됨, missing case 는 `Unknown` reason 으로 보수적 처리.

### Phase 4 — Integration tests + OpenRouter 제외 검증 (AC-RV2-011)

**Goal**: 실제 mock provider 와의 end-to-end 통합 검증, OpenRouter 의도적 제외 정책 회귀 보호.

**Files (NEW)**:
- `internal/llm/router/v2/integration_test.go` — end-to-end via httptest mock providers
- `internal/llm/router/v2/testdata/policy_*.yaml` — 5 fixture (prefer_local, prefer_cheap, prefer_quality, always_specific, with_excluded)

**Files (READ-ONLY)**:
- ADAPTER-001/002 의 fake/mock client 추상화

**Tasks**:
25. **P4-T1 (RED)** — `integration_test.go` 의 5 fixture 시나리오:
    - `TestE2E_PreferLocal_OllamaSelected`
    - `TestE2E_PreferCheap_GroqFreeFirst`
    - `TestE2E_PreferQuality_OpusOrGPT4o`
    - `TestE2E_AlwaysSpecific_OverridesAll`
    - `TestE2E_OpenRouterInChain_NoSpecialTreatment`
26. **P4-T2 (RED)** — `TestE2E_FallbackChain_RateLimitToContentFilter` — 1차 anthropic ratelimit → openai 시도 → openai content_filter → 즉시 중단 검증.
27. **P4-T3 (GREEN)** — fixture YAML + mock httptest server 작성, P3 의 `RouterV2.Route()` 가 실제 호출 사이클 통과.
28. **P4-T4 (REFACTOR)** — progress.md 의 fallback trace 기록 형식 검증 (`v2:fallback_chain_step_<n>_<reason>` 패턴).

**RED tests (integration)**:
- `TestE2E_PreferLocal_OllamaSelected`
- `TestE2E_PreferCheap_GroqFreeFirst`
- `TestE2E_AlwaysSpecific_OverridesAll`
- `TestE2E_OpenRouterInChain_NoSpecialTreatment`
- `TestE2E_FallbackChain_RateLimitToContentFilter`

**Coverage gate**: integration_test 추가로 패키지 전체 ≥ 92%

**Drift signal**: P4 에서 P1~P3 의 단위 테스트 외 실제 mock provider 와의 통합에서 발견된 missing case → 해당 phase 로 backport (drift 30% 임계 검사 적용).

---

## 2. Milestones (priority-based, no time estimates)

| Milestone | Deliverable | Priority |
|-----------|-----------|----------|
| **M1: Policy Layer** | P1 완료, RoutingPolicy YAML 로딩 + 기본값 backward-compat | High |
| **M2: Filter Layer** | P2 완료, CapabilityMatrix 정적 init + RateLimit reader 어댑터 | High |
| **M3: Decorator Layer** | P3 완료, RouterV2.Route() 의사결정 트리 + 14 FailoverReason 분기 | High |
| **M4: Integration** | P4 완료, 5 fixture E2E + OpenRouter 제외 회귀 보호 | Medium |

각 milestone 종료 시:
- `go test -race -cover ./internal/llm/router/v2/...` 통과
- progress.md 에 milestone 완료 + AC 진척률 기록
- gofmt 사전 검증 (lesson_gofmt_pre_push 메모리 적용)

## 3. 기술적 접근 (Technical Approach)

### 3.1 Decorator 패턴

v1 Router 를 v2 의 `base` 필드로 주입. v2 가 정책 미지정 (zero-value `RoutingPolicy{}`) 시 v1 결정을 그대로 통과시켜 byte-identical BC 보장:

```go
func (r *RouterV2) Route(ctx, req) (Route, error) {
    v1Route, err := r.base.Route(ctx, req)
    if err != nil { return Route{}, err }

    if r.policy.Mode == PreferQuality && len(r.policy.FallbackChain) == 0 &&
       len(r.policy.RequiredCapabilities) == 0 && len(r.policy.ExcludedProviders) == 0 {
        return v1Route, nil  // backward-compat fast path
    }
    // ... v2 의사결정 트리 (spec.md §7.3)
}
```

### 3.2 Capability Matrix 정적 init

`map[string][]Capability` 단일 전역 변수, init() 시점 1회 채움. process lifetime 동안 read-only — race 안전. spec.md §6.1 표 = source of truth.

### 3.3 14 FailoverReason 분기 (switch-case 완전성)

`go vet` + `exhaustive` linter 로 14 enum 모두 case 작성 강제. `default:` 는 `Unknown` 으로 매핑 (보수적 fallback, 다음 후보 시도).

### 3.4 정적 Pricing 표 (정확성보다 일관성)

per-million $ 의 input + output 평균값 단일 `float64` 로 비교. 6 개월 manual update 정책 (spec.md §13). PreferCheap 정렬은 `sort.SliceStable` 로 동일 가격 시 fallback chain 입력 순서 보존.

### 3.5 Trace 형식 회귀 보호

spec.md §6.4 의 7 가지 RoutingReason 형식을 enum 화하여 `trace.go` 빌더 함수가 string concat 으로 생성. `trace_test.go` 가 형식 일치 검증.

## 4. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|-----|----------|-------|----------|
| RATELIMIT-001 reader 시그니처가 본 SPEC 가정과 다름 | Medium | Medium | 어댑터 layer (`ratelimit_filter.go`) 로 흡수, RATELIMIT-001 본체 변경 0. P2 시작 시 reader 인터페이스 확인 (assumption 검증) |
| ERROR-CLASS-001 14 FailoverReason 가 추후 확장 | Low | Medium | switch-case `default → Unknown` 보수적 매핑. 신규 reason 추가 시 amendment SPEC |
| 정적 pricing 표가 6 개월 미만에 부정확 | Medium | Low | spec.md §13 maintenance 정책. PreferCheap 결정의 잘못된 선택은 fallback chain 으로 회복 |
| OpenRouter 정책이 6 개월 내 시장 변화로 무효 | Low | Low | spec.md §2.2 OpenRouter 결정 기록. 재평가 시 별도 SPEC |
| v1 Router 가 본 SPEC 진행 중 변경 | Very Low | High | ROUTER-001 status=completed 확인됨. v1 변경 시 Re-planning Gate 트리거 |
| Coverage gate 90% 미달 | Low | Medium | TDD per phase 로 RED-first 작성. table-driven test 50+ cases |
| `go test -race` flake | Low | Low | RATELIMIT-001 reader 추상화 → `RateLimitView` 인터페이스로 mock 화 |

## 5. Drift Guard

각 phase 종료 시:
1. `git diff --stat` 으로 수정 파일 list 생성.
2. plan.md 의 phase owner files vs 실제 modified files 비교.
3. drift > 30% 시 Re-planning Gate 트리거.

본 SPEC 의 file ownership 명확:
- P1: `policy.go`, `loader.go` (2 신규)
- P2: `capability.go`, `ratelimit_filter.go` (2 신규)
- P3: `fallback.go`, `router.go`, `pricing.go`, `trace.go` (4 신규)
- P4: `integration_test.go`, `testdata/policy_*.yaml` (1 신규 + fixture)

phase 간 다른 phase 의 owner file 수정은 drift 로 간주.

## 6. Quality Gates

본 SPEC 진행 중 매 phase 종료 시:

1. **TRUST 5 자체 점검**:
   - **Tested**: ≥ 90% coverage per file
   - **Readable**: golangci-lint pass
   - **Unified**: gofmt -l 결과 비어 있음 (lesson_gofmt_pre_push 적용)
   - **Secured**: routing-policy.yaml 권한 0600 강제 (spec.md §12)
   - **Trackable**: progress.md milestone 기록 + commit message conventional + 한국어 본문 + SPEC trailer

2. **LSP gate**:
   - run phase: 0 error, 0 type error, 0 lint error
   - sync phase: 0 error, ≤ 10 warning, clean LSP

3. **Pre-push 자체 검증** (lesson_gofmt_pre_push):
   - `gofmt -l ./...` 빈 출력
   - `go vet ./...` clean
   - `go test -race -cover ./internal/llm/router/v2/...` pass

## 7. Plan Audit Targets

본 plan 은 plan-auditor 1라운드 audit 진입 시 다음 항목 검증:

- EARS REQ 14 개 (REQ-RV2-001~014) 가 spec.md §4 에 모두 작성되어 있는가?
- 각 REQ 가 acceptance.md 의 AC 와 1:N 매핑되어 있는가?
- Phase 분해가 file ownership 충돌 없이 sequential 로 정렬되어 있는가?
- OpenRouter 의도적 제외 결정이 spec.md §2.2 + §3.2 + §14 에 일관되게 명시되어 있는가?
- Risks & Mitigations 가 SPEC 의존성 그래프의 모든 외부 SPEC 을 cover 하는가?
- Coverage gate 90% 가 phase 별 적용되는가?
- 14 FailoverReason 분기가 11+3 (다음 시도 vs 즉시 중단) 으로 명확히 분류되어 있는가?
- Drift Guard 가 phase 별 file ownership 으로 정의되어 있는가?

## 8. plan_complete signal

```
<moai>plan_complete</moai>
SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
Status: draft → audit-ready (자동 전환 목표)
Phases: 4 (P1 policy, P2 capability+ratelimit, P3 fallback+decorator, P4 integration)
EARS REQs: 14 (Ubiquitous 4, Event-Driven 4, State-Driven 3, Unwanted 3)
ACs: 11
Files (NEW): 9 in internal/llm/router/v2/
Files (MODIFIED): 0
Coverage gate: ≥ 90% per phase, ≥ 92% integration
Audit: 1 round → PASS 시 audit-ready 자동 전환, CONDITIONAL_GO/FAIL 시 사용자 보고 후 종료
```

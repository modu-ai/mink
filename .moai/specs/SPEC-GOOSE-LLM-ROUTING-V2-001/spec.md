---
id: SPEC-GOOSE-LLM-ROUTING-V2-001
version: "0.1.0"
status: audit-ready
created_at: 2026-05-05
updated_at: 2026-05-05
author: manager-spec
priority: P1
issue_number: null
phase: 4
size: 중(M)
lifecycle: spec-anchored
labels: [routing, llm, policy, fallback-chain, capability-aware, rate-limit-aware, phase-4]
---

# SPEC-GOOSE-LLM-ROUTING-V2-001 — 15 Direct Adapter Cost/Latency/Capability-aware Routing + Manual Fallback Chain

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — SPEC-GOOSE-ROUTER-001 v1.0.0 (completed) 위에 v2 라우팅 레이어를 얹는다. 사용자 정책(prefer_local/cheap/quality/specific) + provider capability 매트릭스 + RATELIMIT-001 awareness + ERROR-CLASS-001 14 FailoverReason 통합 분기 + manual fallback chain. **OpenRouter 의도적 제외** (Sprint 1 정책 결정, 6개월 후 재평가). 4 phase decomposition: P1 policy schema → P2 capability matrix → P3 fallback chain → P4 integration tests. | manager-spec |

---

## 1. 개요 (Overview)

ROUTER-001 의 단순성 판정 heuristic (chars/words/keyword 6 기준 → primary vs cheap 양자택일) 위에, **사용자 정책 + 작업 특성 + provider rate limit 상태**를 종합해 15 개 direct adapter 중 **최적 단일 모델**을 선택하고, 실패 시 **사용자가 미리 선언한 fallback chain** 으로 순차 재시도하는 v2 라우팅 레이어를 정의한다.

본 SPEC 이 통과한 시점에서:

- 사용자가 `~/.goose/routing-policy.yaml` 에 4 가지 정책 (`prefer_local` / `prefer_cheap` / `prefer_quality` / `always_specific`) 중 하나를 선언하고 fallback chain (`["anthropic", "openai", "google"]` 등) 을 지정하면, `internal/llm/router/v2/` 가 ROUTER-001 의 결정 위에 정책·capability·rate limit 을 순차 평가해 최종 provider/model 을 반환한다.
- Capability detection matrix (prompt_caching, function_calling, vision, realtime) 를 참조해 SPEC-002 9 개 + SPEC-001 6 개 = **15 개 direct adapter** 중 요청된 capability 를 지원하는 후보만 남긴다.
- SPEC-GOOSE-RATELIMIT-001 의 4-bucket tracker (RPM/TPM/RPH/TPH) 가 80% 임계 도달을 보고하면 v2 router 가 동일 capability 를 충족하는 alternate provider 로 자동 전환한다.
- SPEC-GOOSE-ERROR-CLASS-001 의 14 `FailoverReason` 분류값 (rate_limit, auth, network, server_5xx, capability_unsupported 등) 에 따라 fallback chain 의 다음 후보로 분기한다.
- **OpenRouter 는 의도적으로 미도입**: Sprint 1 의 정책 결정으로 6 개월 후 재평가. SPEC §3.2 OUT SCOPE 에 명시.

본 SPEC 은 **라우팅 결정 정책 + capability matrix + fallback chain 실행 규칙**만 규정한다. 실제 LLM HTTP 호출, credential 취득, rate limit header parsing 은 ADAPTER-001/002, CREDPOOL-001, RATELIMIT-001 책임.

## 2. 배경 (Background)

### 2.1 ROUTER-001 v1 의 한계

ROUTER-001 v1.0.0 은 conservative single-axis heuristic (단순 vs 복잡) 으로 primary↔cheap 양자택일만 수행한다. 다음 갭이 누적:

1. **Provider 다양성 미활용**: SPEC-002 머지로 Z.ai/Groq/Together/Fireworks/Cerebras/Mistral/Qwen/Kimi 등 9 개 provider 가 GREEN 이지만 v1 router 는 primary/cheap 두 모델만 인지.
2. **Capability 미인지**: prompt caching (Anthropic 전용), function calling (OpenAI/Google), vision, realtime 등 provider 별 비대칭 capability 가 SPEC-001/002 에 명세됨에도 v1 router 는 capability 거부 시 fail-fast 만 함.
3. **Rate limit 무인지**: RATELIMIT-001 4-bucket tracker 는 GREEN 이지만 v1 router 는 정적 heuristic 만으로 rate limit 80% 임계 자동 회피 불가.
4. **사용자 정책 부재**: "로컬 우선 (Ollama) / 비용 우선 (Groq free / Mistral Nemo $0.02/M) / 품질 우선 (Claude Opus / GPT-4o) / 특정 provider 강제" 4 가지 의도를 표현할 정책 스키마 없음.
5. **수동 fallback chain 부재**: ROUTER-001 §3.2 OUT 에 "Fallback model chain 실행" 이 명시 OUT — QueryEngine (QUERY-001) 의 `FallbackModels` 와 연계는 수동 list 만 받고 분기 조건 없음.

### 2.2 OpenRouter 의도적 제외 결정 (Sprint 1)

OpenRouter 는 300+ 모델 gateway 로 v2 routing 의 자연스러운 통합 후보지만, 다음 사유로 **본 SPEC 에서 의도적으로 제외**:

| 사유 | 상세 |
|-----|------|
| **이중 라우팅 충돌** | v2 router 가 결정한 후 OpenRouter 내부 routing 이 또 결정 → debugging 곤란, traceability 손실 |
| **비용 모델 불투명** | OpenRouter 마진이 provider 별 가변, prefer_cheap 정책의 비용 비교 정확도 저하 |
| **rate limit 신호 손실** | OpenRouter 가 upstream rate limit header 를 일관 forward 하지 않음 — RATELIMIT-001 4-bucket tracker 정확도 저하 |
| **capability 비대칭 손실** | OpenRouter 모델 list 가 prompt_caching/realtime 같은 비대칭 capability 를 일관 노출하지 않음 |
| **재평가 시점** | Sprint 1 종료 + 6 개월 후 (2026-11 경) OpenRouter v2 router 통합 별도 SPEC 으로 재검토 |

**대안**: 사용자가 OpenRouter 를 원하면 SPEC-002 의 OpenRouter adapter 를 직접 fallback chain 에 명시 (`["anthropic", "openrouter:gpt-oss-120b", "google"]`) — v2 router 는 OpenRouter 를 단일 provider 로 취급할 뿐 routing 결정에 우대/할인 적용 안 함.

### 2.3 상속 자산

- **SPEC-GOOSE-ROUTER-001 v1.0.0 (completed)**: `Router` 인터페이스, `Route` 결과 struct, `ProviderRegistry` (15 provider 메타), simple/complex heuristic. v2 는 v1 위에 decorator 패턴으로 얹는다 (v1 미수정).
- **SPEC-GOOSE-RATELIMIT-001**: 4-bucket tracker (RPM/TPM/RPH/TPH). v2 가 read-only 로 참조.
- **SPEC-GOOSE-ERROR-CLASS-001**: 14 `FailoverReason` 열거값 (rate_limit, auth, network, server_5xx, capability_unsupported, model_not_found, content_filter, context_window_exceeded, billing, region_restricted, deprecated_model, malformed_response, timeout, unknown). v2 의 fallback chain 분기 조건.
- **SPEC-GOOSE-LLM-001**: `Provider` 인터페이스 + `ProviderCapabilities` struct (Vision/FunctionCalling/Streaming/VisionModels/MaxContextWindow). v2 는 capability matrix 의 source of truth 로 활용.

### 2.4 범위 경계 (한 줄)

- **IN**: `internal/llm/router/v2/` 신규 패키지, `~/.goose/routing-policy.yaml` 스키마, capability matrix struct (prompt_caching/function_calling/vision/realtime), fallback chain 실행 (manual chain + 14 FailoverReason 분기), rate-limit-aware 후보 필터링 (80% 임계 회피).
- **OUT**: OpenRouter adapter v2 router 통합 (Sprint 1 정책 결정, 6 개월 후 재평가), auto-failover (수동 chain 만), token-counting cost prediction (정적 per-million pricing 표만), multi-region routing (region 은 provider option 으로만), 학습 기반 routing (Phase 4 INSIGHTS-001 후속).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE — 6 areas

#### Area 1 — RoutingPolicy schema (REQ-RV2-001, -002)

1. **신규 파일** `internal/llm/router/v2/policy.go`:
   - `RoutingPolicy` struct: `{Mode PolicyMode, FallbackChain []ProviderRef, RequiredCapabilities []Capability, ExcludedProviders []string}`.
   - `PolicyMode` enum: `PreferLocal` / `PreferCheap` / `PreferQuality` / `AlwaysSpecific`.
   - `ProviderRef` struct: `{Provider string, Model string}` — fallback chain 의 단위.
2. **신규 파일** `internal/llm/router/v2/loader.go`:
   - `~/.goose/routing-policy.yaml` 읽어 `RoutingPolicy` 반환.
   - 파일 부재 → `RoutingPolicy{Mode: PreferQuality, FallbackChain: nil}` 기본값 (v1 router 와 호환).
   - Schema validation: 알 수 없는 PolicyMode → 명시적 에러 (silent fallback 금지).

#### Area 2 — CapabilityMatrix (REQ-RV2-003, -004)

1. **신규 파일** `internal/llm/router/v2/capability.go`:
   - `Capability` enum: `PromptCaching` / `FunctionCalling` / `Vision` / `Realtime`.
   - `CapabilityMatrix` struct: `map[string][]Capability` (provider → 지원 capability 리스트).
   - 정적 초기화: 15 provider × 4 capability 매트릭스 (Section 6.1 표).
2. **`Match(required []Capability, provider string) bool`**: 필수 capability 모두 충족 시 true.
3. ROUTER-001 의 `ProviderRegistry.AdapterReady=true` 인 provider 만 후보로 인정.

#### Area 3 — Rate-Limit-Aware Filtering (REQ-RV2-005, -006)

1. **신규 파일** `internal/llm/router/v2/ratelimit_filter.go`:
   - `RateLimitView` 인터페이스 (RATELIMIT-001 의 reader 추상화):
     ```go
     type RateLimitView interface {
         BucketUsage(provider string) (rpm, tpm, rph, tph float64) // 0.0-1.0
     }
     ```
   - `FilterByRateLimit(candidates []ProviderRef, view RateLimitView, threshold float64) []ProviderRef`:
     - 4 bucket 중 **하나라도** `>= threshold` (기본 0.80) 면 후보에서 제외.
     - 모든 후보가 80% 이상이면 빈 list 반환 (호출자가 fallback chain 진입 결정).

#### Area 4 — Fallback Chain Execution (REQ-RV2-007, -008, -009)

1. **신규 파일** `internal/llm/router/v2/fallback.go`:
   - `FallbackExecutor` struct + `Execute(ctx, chain, fn)` 함수형 패턴.
   - 입력: `chain []ProviderRef` + `fn func(ctx, provider, model) (resp, err)`.
   - 첫 후보 호출 → err 분류 (`ErrorClassifier` from ERROR-CLASS-001) → 14 `FailoverReason` 매핑 → 분기:
     - `RateLimit` / `Server5xx` / `Network` / `Timeout` → 다음 후보 시도
     - `Auth` / `Billing` / `RegionRestricted` → 다음 후보 시도 + 사용자 알림 로그
     - `CapabilityUnsupported` / `ModelNotFound` / `DeprecatedModel` → 다음 후보 시도 (capability matrix 동적 업데이트는 별도 SPEC)
     - `ContentFilter` / `ContextWindowExceeded` / `MalformedResponse` → **chain 중단, 즉시 에러 반환** (다음 후보로 같은 문제 재발 가능성 높음)
     - `Unknown` → 다음 후보 시도 (보수적 fallback)
2. Chain 끝까지 실패 시 마지막 에러 + chain 모든 시도 기록 반환 (`MultiError` wrapping).

#### Area 5 — RouterV2 Decorator (REQ-RV2-010, -011)

1. **신규 파일** `internal/llm/router/v2/router.go`:
   - `RouterV2` struct: `{base router.Router, policy RoutingPolicy, matrix CapabilityMatrix, rateLimitView RateLimitView}`.
   - `Route(ctx, req) (Route, error)` 메서드:
     - Step 1: v1 `Router.Route()` 호출 → primary/cheap 1차 결정 획득.
     - Step 2: PolicyMode 적용:
       - `AlwaysSpecific` → fallback chain 0번째 강제 (v1 결정 무시).
       - `PreferLocal` → Ollama 후보 추가, 없으면 v1 결정 유지.
       - `PreferCheap` → 정적 pricing 표 (Section 6.2) 의 per-million $ 오름차순 후보 추가.
       - `PreferQuality` → v1 결정 + Anthropic Opus / GPT-4o 우선 후보 추가.
     - Step 3: `RequiredCapabilities` 로 `CapabilityMatrix.Match()` 필터.
     - Step 4: `FilterByRateLimit()` 로 80% 임계 회피.
     - Step 5: `ExcludedProviders` 제거.
     - Step 6: 첫 후보를 `Route` 로 반환. 후보 0 개면 v1 결정으로 fallback (silent recovery).
2. 함수 호출 사이트는 v1 `Router` 와 동일 시그니처 — drop-in replacement 가능.

#### Area 6 — Decision Trace + Hook (REQ-RV2-012)

1. **`Route.RoutingReason`** 확장: v1 의 `"simple"`/`"complex"` 외에 v2 사유 추가:
   - `"v2:policy_prefer_cheap_groq_free"` — Groq 무료 tier 선택
   - `"v2:rate_limit_avoid_anthropic"` — Anthropic 80% 임계로 OpenAI 전환
   - `"v2:capability_vision_required_only_google"` — vision 필수 → Google 단독
   - `"v2:fallback_chain_step_2"` — 1차 실패 후 chain 2번째
2. `RoutingDecisionHook` (v1 정의) 가 v2 decision trace 도 받도록 확장.

### 3.2 OUT OF SCOPE

- **OpenRouter adapter v2 router 통합** — Sprint 1 정책 결정으로 의도적 제외 (Section 2.2). 사용자가 fallback chain 에 OpenRouter 를 직접 명시하는 것은 허용 (단순 provider 취급).
- **Auto-failover** — v2 는 사용자 선언 chain 만 실행. 자동 chain 생성/추론은 후속 SPEC.
- **Cost prediction with token counting** — tiktoken 등으로 prompt 토큰 카운트 + 비용 예측은 후속. 본 SPEC 은 정적 per-million $ pricing 표 (Section 6.2) 만.
- **Multi-region routing** — Qwen `intl/cn/sg/hk`, Kimi `intl/cn` 의 region 자동 선택은 후속. Region 은 provider option 으로만.
- **학습 기반 routing** — 사용자 패턴 (어느 작업에 어느 모델 선택) 기반 자동 정책 추천은 INSIGHTS-001 후속.
- **Capability matrix 동적 업데이트** — provider deprecation/신모델 발표 자동 감지는 후속. 본 SPEC 은 정적 매트릭스 (Section 6.1).
- **Streaming 결정** — v2 는 Provider/Model 만 결정. Streaming 여부는 ADAPTER-001/002.
- **Tool schema 변환** — ADAPTER-001/002.

### 3.3 SPEC 의존성 그래프

```
SPEC-GOOSE-LLM-001 (Provider 인터페이스, ProviderCapabilities)
    └─ SPEC-GOOSE-ROUTER-001 (Smart Routing v1, ProviderRegistry, completed)
        └─ SPEC-GOOSE-LLM-ROUTING-V2-001 (본 SPEC, 정책·capability·ratelimit decorator)
SPEC-GOOSE-RATELIMIT-001 (4-bucket tracker) ─┘  (read-only 참조)
SPEC-GOOSE-ERROR-CLASS-001 (14 FailoverReason) ─┘  (분기 분류)
SPEC-GOOSE-ADAPTER-001 (6 native adapter) ─┐
SPEC-GOOSE-ADAPTER-002 (9 OpenAI-compat adapter) ─┴ (15 candidate pool)
```

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 전반 항상 적용)

**REQ-RV2-001** — RouterV2 는 사용자 `~/.goose/routing-policy.yaml` 의 `mode` 필드를 항상 우선 평가한다. v1 Router 의 simple/complex 결정은 PolicyMode 가 `PreferQuality` 일 때만 변경 없이 유지된다.

**REQ-RV2-002** — RouterV2 는 정책 파일 부재 시 `RoutingPolicy{Mode: PreferQuality, FallbackChain: nil, RequiredCapabilities: nil, ExcludedProviders: nil}` 을 기본값으로 사용하며, v1 Router 와 byte-identical 동작을 보장한다 (backward compatibility).

**REQ-RV2-003** — RouterV2 는 ROUTER-001 `ProviderRegistry.AdapterReady=true` 인 provider 만 routing 후보로 간주한다. `AdapterReady=false` provider 는 fallback chain 에 명시되어도 silent skip 한다 (사용자 알림 로그 1회).

**REQ-RV2-004** — RouterV2 는 정적 `CapabilityMatrix` (Section 6.1) 를 init 시점 1회 로드하고 process lifetime 동안 불변으로 유지한다. 동적 capability 갱신은 OUT.

### 4.2 Event-Driven (이벤트 기반)

**REQ-RV2-005** — When provider X 가 `ErrorClassifier` 에 의해 `FailoverReason.RateLimit` / `Server5xx` / `Network` / `Timeout` / `Auth` / `Billing` / `RegionRestricted` / `CapabilityUnsupported` / `ModelNotFound` / `DeprecatedModel` / `Unknown` 중 하나로 분류되면, RouterV2 는 fallback chain 의 다음 후보로 분기한다.

**REQ-RV2-006** — When `RoutingDecisionHook` 가 등록되어 있으면, RouterV2 는 최종 Route 결정 직전 hook 을 호출하고 v2 decision trace (정책 적용 단계, 필터링 결과, 선택 사유) 를 인자로 전달한다.

**REQ-RV2-007** — When 사용자 정책 `Mode=PreferCheap` 이면, RouterV2 는 정적 pricing 표 (Section 6.2) per-million $ 오름차순으로 후보를 정렬하고 capability/ratelimit 필터 통과한 가장 저렴한 후보를 반환한다.

**REQ-RV2-008** — When 사용자 정책 `Mode=AlwaysSpecific` 이고 `FallbackChain` 이 비어 있지 않으면, RouterV2 는 chain 의 0번째 ProviderRef 를 v1 Router 결정 무시하고 강제 선택한다.

### 4.3 State-Driven (상태 기반)

**REQ-RV2-009** — While `RateLimitView.BucketUsage(provider)` 의 4 bucket (RPM/TPM/RPH/TPH) 중 하나라도 0.80 (기본값) 이상이면, RouterV2 는 해당 provider 를 routing 후보 pool 에서 제외한다. Threshold 는 `routing-policy.yaml` `rate_limit_threshold` 로 override 가능 (0.0~1.0 범위).

**REQ-RV2-010** — While `RoutingPolicy.RequiredCapabilities` 에 N 개 capability 가 명시된 동안, RouterV2 는 `CapabilityMatrix` 가 N 개 모두를 지원한다고 보증하는 provider 만 후보로 통과시킨다.

**REQ-RV2-011** — While fallback chain 실행 중인 동안, RouterV2 는 매 시도 직후 `FailoverReason` + 시도 순번 + provider/model + duration 을 progress.md 에 append-only 기록한다 (debugging trace).

### 4.4 Unwanted Behavior (금지)

**REQ-RV2-012** — If `RoutingPolicy.ExcludedProviders` 에 명시된 provider 가 fallback chain 에 포함되어 있으면, RouterV2 는 해당 provider 를 chain 실행 중에도 silent skip 하고 다음 후보로 즉시 진행한다. exclude 와 chain 동시 명시 충돌은 명시적 에러 (init 시점) 가 아닌 chain 실행 시점 silent skip 이다.

**REQ-RV2-013** — If fallback chain 실행 중 `FailoverReason.ContentFilter` / `ContextWindowExceeded` / `MalformedResponse` 발생 시, RouterV2 는 chain 즉시 중단하고 해당 에러를 그대로 반환한다 (다음 후보로 같은 문제 재발 가능성 높음).

**REQ-RV2-014** — If 모든 fallback chain 후보가 실패하거나 capability/ratelimit 필터로 0 개가 남으면, RouterV2 는 v1 Router 의 결정을 silent recovery 로 반환한다 (사용자 작업 불가능 상태 회피). 단, RouterV2 는 progress.md 에 `"v2:fallback_exhausted_recover_v1"` 사유를 기록한다.

---

## 5. 가정 (Assumptions)

| 가정 | 영향 | 검증 방법 |
|-----|-----|---------|
| ROUTER-001 v1.0.0 의 `Router` 인터페이스 + `ProviderRegistry` 가 v2 SPEC 진행 중 변경 없이 안정 유지 | 변경 시 v2 decorator 패턴 깨짐 | ROUTER-001 status=completed 확인됨, version 1.0.0 frozen |
| RATELIMIT-001 의 reader 추상화가 `BucketUsage(provider) (rpm,tpm,rph,tph float64)` 시그니처로 노출 가능 | 시그니처 다르면 v2 가 RATELIMIT-001 어댑터 추가 작업 필요 | RATELIMIT-001 spec.md 확인 (M0~M3 단계에서 reader 인터페이스 정의 예정), 본 SPEC plan P2 에서 어댑터 작성 |
| ERROR-CLASS-001 14 FailoverReason 열거값이 본 SPEC 과 1:1 매핑 가능 | 매핑 실패 시 v2 의 분기 분류 missing case | ERROR-CLASS-001 spec.md §6 14 enum 확인됨 |
| 정적 pricing 표 (Section 6.2) 가 SPEC 작성 시점 (2026-05-05) 의 provider 공식 가격 기준이며, 6 개월 단위 manual update 정책 | 가격 변동 시 PreferCheap 결정 부정확 | 본 SPEC §13 maintenance 섹션 명시 |
| 사용자가 `~/.goose/routing-policy.yaml` 작성 시 YAML schema validation 을 따라줌 | schema 위반 시 init 실패 + 명시적 에러 반환 | loader.go 의 schema validator 강제 |
| OpenRouter 의도적 제외 결정이 6 개월간 유효 (2026-11 까지 재평가 안 함) | 시장 변화로 OpenRouter 통합 요구 폭증 시 별도 SPEC 필요 | §2.2 OpenRouter 결정 기록 + 6 개월 후 재평가 |

## 6. 명세 (Specifications)

### 6.1 CapabilityMatrix — 15 provider × 4 capability

| Provider | PromptCaching | FunctionCalling | Vision | Realtime |
|---------|:-------------:|:---------------:|:------:|:--------:|
| anthropic | ✅ | ✅ | ✅ | ❌ |
| openai | ❌ | ✅ | ✅ | ✅ |
| google | ❌ | ✅ | ✅ | ❌ |
| xai | ❌ | ✅ | ✅ | ❌ |
| deepseek | ❌ | ✅ | ❌ | ❌ |
| ollama | ❌ | ✅ (model dependent) | ✅ (llava) | ❌ |
| zai_glm | ❌ | ✅ | ❌ | ❌ |
| groq | ❌ | ✅ | ❌ | ❌ |
| openrouter | ❌ (gateway) | ✅ (model dependent) | ✅ (model dependent) | ❌ |
| together | ❌ | ✅ | ✅ (some) | ❌ |
| fireworks | ❌ | ✅ | ✅ (some) | ❌ |
| cerebras | ❌ | ✅ | ❌ | ❌ |
| mistral | ❌ | ✅ | ❌ | ❌ |
| qwen | ❌ | ✅ | ✅ (qwen3-vl) | ❌ |
| kimi | ❌ | ✅ | ❌ | ❌ |

**Note**: Capability detection 은 정적 매트릭스. provider 가 신모델로 capability 추가 시 본 SPEC §13 manual update 정책에 따라 SPEC amendment 필요.

### 6.2 정적 Pricing 표 (per million tokens, $ USD, input avg)

> **유효 시점**: 2026-05-05. 6 개월 단위 manual update.

| Provider:Model | Input ($/M) | Output ($/M) | Notes |
|---------|:-----------:|:------------:|-------|
| ollama:* | 0.00 | 0.00 | local, hardware cost only |
| groq:llama-3.3-70b | 0.00 | 0.00 | free tier, 30 RPM |
| mistral:nemo | 0.02 | 0.02 | cheapest paid |
| together:llama-3.3-70b-turbo | 0.88 | 0.88 | |
| fireworks:llama-3.1-405b | 3.00 | 3.00 | |
| deepseek:deepseek-chat | 0.27 | 1.10 | |
| zai_glm:glm-4.6 | 0.50 | 1.50 | |
| qwen:qwen3-max | 0.80 | 2.40 | |
| kimi:k2.6 | 0.60 | 1.80 | 1T MoE |
| google:gemini-2.0-flash | 0.075 | 0.30 | |
| openai:gpt-4o | 2.50 | 10.00 | |
| openai:o1-preview | 15.00 | 60.00 | |
| anthropic:claude-sonnet-4.6 | 3.00 | 15.00 | |
| anthropic:claude-opus-4-7 | 15.00 | 75.00 | premium |
| xai:grok-3 | 5.00 | 15.00 | |
| cerebras:llama-3.3-70b | 0.85 | 1.20 | 1000+ TPS |

PreferCheap 정렬 시: input + output 평균값 오름차순.

### 6.3 RoutingPolicy YAML schema

```yaml
# ~/.goose/routing-policy.yaml
mode: prefer_cheap            # prefer_local | prefer_cheap | prefer_quality | always_specific
rate_limit_threshold: 0.80    # 0.0-1.0, default 0.80
required_capabilities:        # optional, default []
  - function_calling
  - vision
excluded_providers:           # optional, default []
  - anthropic
fallback_chain:               # optional, default []
  - provider: groq
    model: llama-3.3-70b
  - provider: mistral
    model: nemo
  - provider: openai
    model: gpt-4o
```

### 6.4 Route 확장 — RoutingReason v2 prefix

v1 `Route.RoutingReason` 형식 (`"simple"` / `"complex"`) 위에 v2 prefix 추가:

| Format | Trigger |
|-------|---------|
| `v1:simple` | v2 정책 비활성 + v1 simple 판정 |
| `v1:complex` | v2 정책 비활성 + v1 complex 판정 |
| `v2:policy_<mode>_<provider>` | v2 정책 적용으로 v1 결정 변경 |
| `v2:capability_<cap>_required_<count>_candidates` | required capabilities 필터 적용 |
| `v2:rate_limit_avoid_<provider>_<bucket>_<usage>` | 80% 임계 회피 |
| `v2:fallback_chain_step_<n>_<reason>` | chain n번째 시도, reason 매핑 |
| `v2:fallback_exhausted_recover_v1` | chain 모두 실패, v1 결정 silent recovery |

## 7. 구현 가이드 (Implementation Guide)

### 7.1 패키지 구조

```
internal/llm/router/v2/
├── policy.go           # RoutingPolicy, PolicyMode, ProviderRef
├── loader.go           # ~/.goose/routing-policy.yaml 로더
├── capability.go       # Capability enum, CapabilityMatrix
├── ratelimit_filter.go # RateLimitView 인터페이스 + FilterByRateLimit
├── fallback.go         # FallbackExecutor + 14 FailoverReason 분기
├── router.go           # RouterV2 decorator (v1 위 얹기)
├── pricing.go          # 정적 pricing 표 + PreferCheap 정렬
├── trace.go            # RoutingReason v2 prefix 빌더
└── *_test.go           # table-driven 50+ test cases
```

### 7.2 v1 Router decorator 패턴

```go
// 사용자 코드 변경 없이 v2 활성화
v1 := router.New(...)                  // 기존 v1
policy, _ := v2.LoadPolicy("~/.goose/routing-policy.yaml")
matrix := v2.NewCapabilityMatrix()     // 정적 init
rlView := ratelimit.NewReader(...)     // RATELIMIT-001 reader
v2Router := v2.New(v1, policy, matrix, rlView)

route, err := v2Router.Route(ctx, req) // v1 시그니처 동일
```

### 7.3 의사결정 트리 (Section 6.4 RoutingReason 매핑)

```
1. v1.Route(req) → primary/cheap 결정
2. policy.Mode == PreferQuality && chain 없음 → v1 결정 그대로 (v1:simple|complex)
3. policy.Mode == AlwaysSpecific && chain[0] 있음 → chain[0] 강제 (v2:policy_always_specific_<provider>)
4. policy.Mode == PreferLocal → Ollama 후보 추가 → capability filter → ratelimit filter → exclude filter → 첫 후보
5. policy.Mode == PreferCheap → pricing 표 오름차순 정렬 → 동일 filter chain → 첫 후보
6. policy.Mode == PreferQuality → v1 결정 + Opus/GPT-4o 우선 → 동일 filter chain → 첫 후보
7. 모든 후보 0 개 → v1 결정 fallback (v2:fallback_exhausted_recover_v1)
```

### 7.4 Test 전략

- **Table-driven test** 50+ cases: PolicyMode × Capability × RateLimit × FailoverReason 조합
- **Mocking**: `RateLimitView` 인터페이스 mock, `ErrorClassifier` mock, fake v1 Router
- **Race detection**: `go test -race` 통과 필수 (RATELIMIT-001 reader 동시 호출)
- **Coverage gate**: 90%+ for v2 패키지 (정책 분기 누락 방지)

## 8. Phase 분해 (4 phases)

상세는 plan.md 참조. 요약:

| Phase | 산출물 | RED test 우선 | 기간 (priority 기준) |
|-------|-------|------|---------|
| **P1** Policy schema | policy.go, loader.go | YAML schema 위반 ErrParse | High |
| **P2** Capability matrix + RateLimit reader 어댑터 | capability.go, ratelimit_filter.go | 정적 매트릭스 일관성, RPM/TPM/RPH/TPH 80% threshold | High |
| **P3** Fallback chain + Decorator | fallback.go, router.go, pricing.go, trace.go | 14 FailoverReason 분기, 50+ table-driven | High |
| **P4** Integration tests | end-to-end with mock providers | OpenRouter 제외 검증, AlwaysSpecific override | Medium |

---

## 9. 인터페이스 (Interfaces)

### 9.1 신규 공개 API (internal package, but stable across SPECs)

```go
package v2

// PolicyMode 4가지 정책
type PolicyMode int
const (
    PreferQuality PolicyMode = iota  // default
    PreferLocal
    PreferCheap
    AlwaysSpecific
)

// Capability 4가지
type Capability int
const (
    PromptCaching Capability = iota
    FunctionCalling
    Vision
    Realtime
)

// ProviderRef 단일 후보
type ProviderRef struct {
    Provider string
    Model    string
}

// RoutingPolicy 사용자 선언
type RoutingPolicy struct {
    Mode                  PolicyMode
    RateLimitThreshold    float64
    RequiredCapabilities  []Capability
    ExcludedProviders     []string
    FallbackChain         []ProviderRef
}

// RateLimitView RATELIMIT-001 reader 추상화
type RateLimitView interface {
    BucketUsage(provider string) (rpm, tpm, rph, tph float64)
}

// CapabilityMatrix 정적 15×4 매트릭스
type CapabilityMatrix struct { /* unexported */ }
func NewCapabilityMatrix() *CapabilityMatrix
func (m *CapabilityMatrix) Match(required []Capability, provider string) bool

// RouterV2 decorator
type RouterV2 struct { /* unexported */ }
func New(base router.Router, policy RoutingPolicy, matrix *CapabilityMatrix, view RateLimitView) *RouterV2
func (r *RouterV2) Route(ctx context.Context, req router.RoutingRequest) (router.Route, error)

// LoadPolicy YAML 로더
func LoadPolicy(path string) (RoutingPolicy, error)
```

### 9.2 기존 API 변경 없음

- `internal/llm/router/router.go` (v1) 미변경
- `internal/llm/router/registry.go` (v1) 미변경
- `Route` struct 의 `RoutingReason` 필드는 string — v2 prefix 가 추가되지만 타입 변경 없음 (BC 보존)

---

## 10. 인수 기준 요약 (AC Summary)

상세는 acceptance.md 참조. 본 SPEC 의 11 AC:

| AC ID | REQ | One-liner | Phase |
|-------|-----|-----------|-------|
| AC-RV2-001 | REQ-001, -002 | 정책 파일 부재 시 v1 결정과 byte-identical | P1 |
| AC-RV2-002 | REQ-001, -008 | mode=always_specific + chain[0]=groq 시 v1 결정 무시하고 groq 강제 | P1, P3 |
| AC-RV2-003 | REQ-007 | mode=prefer_cheap + 모든 capability 동일 시 ollama → groq → mistral:nemo 순 선택 | P3 |
| AC-RV2-004 | REQ-010 | required_capabilities=[vision] 시 deepseek/groq/cerebras/mistral/kimi 후보 제거 | P2 |
| AC-RV2-005 | REQ-009 | RateLimitView mock 의 anthropic RPM=0.85 시 anthropic 후보 제거 | P2 |
| AC-RV2-006 | REQ-005 | 1차 anthropic → FailoverReason.RateLimit → chain 2번째 openai 자동 시도 | P3 |
| AC-RV2-007 | REQ-013 | 1차 anthropic → FailoverReason.ContentFilter → chain 즉시 중단, 에러 반환 | P3 |
| AC-RV2-008 | REQ-012 | excluded_providers=[anthropic] + chain=[anthropic, openai] 시 anthropic silent skip → openai 직접 | P3 |
| AC-RV2-009 | REQ-014 | 모든 후보 capability/ratelimit 필터로 0개 → v1 결정 silent recovery + RoutingReason="v2:fallback_exhausted_recover_v1" | P3 |
| AC-RV2-010 | REQ-011 | fallback chain 실행 매 시도 progress.md 에 reason+provider+model+duration append-only 기록 | P3 |
| AC-RV2-011 | OpenRouter 제외 정책 | chain=[openrouter:gpt-oss] 명시 시 단순 provider 로 취급, v2 routing 결정에 우대 적용 안 함 | P4 |

---

## 11. 비기능 요구사항 (Non-Functional)

| 항목 | 목표 | 측정 |
|-----|-----|------|
| Routing latency p99 | < 5ms (v2 decorator overhead) | benchmark `BenchmarkRouterV2_Route` |
| Memory overhead | < 100KB (CapabilityMatrix + Pricing 표) | `go test -bench -benchmem` |
| Race-clean | `go test -race` 통과 | CI gate |
| Coverage | ≥ 90% v2 패키지 | `go test -cover` |
| BC 보존 | v1 Router 사용자 코드 변경 0 | v1 테스트 100% 통과 |

## 12. 보안 고려사항 (Security)

- **routing-policy.yaml 권한**: `~/.goose/routing-policy.yaml` 은 `0600` 권한 강제. 다른 사용자 읽기 불가 (provider 선호도 = 잠재 사용 패턴 노출).
- **fallback chain 의 model name**: 임의 model name 주입 (`"../../../etc/passwd"`) 방어 — `ProviderRegistry.AdapterReady` 검증으로 차단.
- **PII 로깅 방지**: progress.md 의 fallback trace 는 provider/model/reason/duration 만 기록, 사용자 message 본문 절대 미기록.

## 13. Maintenance 정책

- **Pricing 표 (Section 6.2) update**: 6 개월 주기 manual update (다음 review: 2026-11). Provider 공식 가격 페이지 link 본 SPEC §15 references.
- **CapabilityMatrix update**: provider 가 신모델로 capability 추가 시 본 SPEC amendment 필요 (예: anthropic prompt caching → openai 도입 시).
- **OpenRouter 재평가**: 2026-11 별도 SPEC 으로 v2 router 통합 검토.
- **FailoverReason 추가**: ERROR-CLASS-001 의 14 개 enum 확장 시 본 SPEC fallback.go 분기 추가 + amendment.

## 14. Exclusions (What NOT to Build)

본 SPEC 에서 **명시적으로 만들지 않는** 것 (스코프 보호):

1. **OpenRouter v2 router 우대/할인 적용** — Section 2.2 의 5 가지 사유로 의도적 제외. fallback chain 단순 명시는 허용.
2. **Auto-failover chain 자동 생성** — 사용자가 명시한 chain 만 실행. AI 추천/학습 기반 chain 제안 OUT.
3. **Token counting cost prediction** — tiktoken/anthropic-tokenizer 등으로 prompt 토큰 카운트 후 정확한 USD 비용 예측 OUT. 정적 per-million 표만 사용.
4. **Multi-region routing** — Qwen `intl/cn/sg/hk`, Kimi `intl/cn` region 자동 선택 OUT. provider option 으로만.
5. **Capability matrix 동적 갱신** — provider deprecation 자동 감지 OUT.
6. **Streaming 결정** — Provider/Model 만 결정. Streaming 은 ADAPTER-001/002.
7. **Embeddings/audio routing** — text completion 만 본 SPEC 범위.
8. **Pricing 자동 갱신** — provider 공식 페이지 scrape 자동화 OUT. 6 개월 manual update.

## 15. References

- SPEC-GOOSE-ROUTER-001 v1.0.0 (completed) — `Router` 인터페이스, `ProviderRegistry`, simple/complex heuristic
- SPEC-GOOSE-RATELIMIT-001 — 4-bucket tracker (RPM/TPM/RPH/TPH)
- SPEC-GOOSE-ERROR-CLASS-001 — 14 `FailoverReason` enum
- SPEC-GOOSE-LLM-001 — `Provider` 인터페이스, `ProviderCapabilities`
- SPEC-GOOSE-ADAPTER-001 — 6 native adapters (Anthropic, OpenAI, xAI, DeepSeek, Google, Ollama)
- SPEC-GOOSE-ADAPTER-002 — 9 OpenAI-compat adapters (Z.ai, Groq, OpenRouter, Together, Fireworks, Cerebras, Mistral, Qwen, Kimi)
- `.moai/project/tech.md` §9 — LLM Provider 생태계 15 provider adapter-ready
- `.moai/config/sections/quality.yaml` — `development_mode: tdd` (본 SPEC RED-first 작성 가정)

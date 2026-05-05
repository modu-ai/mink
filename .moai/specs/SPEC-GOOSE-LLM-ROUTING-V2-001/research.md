# SPEC-GOOSE-LLM-ROUTING-V2-001 — Research Notes (간단)

> **목적**: v2 router decorator 의 의사결정 트리를 안전하게 설계하기 위한 의존 SPEC 의 인터페이스 검토. spec.md / plan.md 의 가정을 검증.

## HISTORY

| 버전 | 날짜 | 변경 | 담당 |
|-----|------|------|------|
| 0.1.0 | 2026-05-05 | 초안 — 의존 SPEC 4 종 (ROUTER-001, RATELIMIT-001, ERROR-CLASS-001, LLM-001) 인터페이스 검토 + decorator pattern 적용 가능성 검증 + OpenRouter 제외 의사결정 근거 정리 | manager-spec |

---

## 1. 의존 SPEC 검토

### 1.1 SPEC-GOOSE-ROUTER-001 v1.0.0 (completed)

- **Status**: completed (2026-04-27)
- **Version**: 1.0.0 frozen — v2 SPEC 진행 중 변경 없음 가정 안전
- **인터페이스**:
  - `Router.Route(ctx, RoutingRequest) (Route, error)` 단일 메서드 — decorator 패턴 적용 자연스러움
  - `Route` struct: `{Model, Provider, BaseURL, RoutingReason, Signature}` — `RoutingReason` 이 string 타입이라 v2 prefix 추가가 BC 안전
  - `ProviderRegistry` 의 `AdapterReady` 필드가 v2 의 후보 검증 source-of-truth 로 사용 가능
- **Co-ownership**: SPEC-002 가 ProviderRegistry 의 9 추가 provider AdapterReady 를 false→true 로 전환하는 패턴이 SPEC §3.3 에 명시됨. 본 SPEC 도 v2 의 capability matrix 를 ProviderRegistry 와 sync 유지.

### 1.2 SPEC-GOOSE-RATELIMIT-001

- **Status**: 본 SPEC 작성 시점 implementation 진행 중
- **인터페이스 가정**: 4-bucket tracker 의 `BucketUsage(provider) (rpm, tpm, rph, tph float64)` (0.0~1.0)
- **위험**: 실 reader 시그니처가 다를 경우 — 본 SPEC plan P2 가 어댑터 layer (`ratelimit_filter.go` 의 `RateLimitView` 인터페이스) 로 흡수
- **검증 방법**: P2 시작 시 RATELIMIT-001 의 reader 인터페이스 확인 (assumption 검증). 시그니처 다르면 어댑터 함수로 변환

### 1.3 SPEC-GOOSE-ERROR-CLASS-001

- **Status**: 본 SPEC 작성 시점 spec.md 완성, implementation 진행 중
- **인터페이스 가정**: 14 `FailoverReason` enum + `ErrorClassifier.Classify(error) FailoverReason` 단일 메서드
- **14 enum**: rate_limit, auth, network, server_5xx, capability_unsupported, model_not_found, content_filter, context_window_exceeded, billing, region_restricted, deprecated_model, malformed_response, timeout, unknown
- **분기 분류**:
  - 다음 후보 시도 (11): rate_limit, server_5xx, network, timeout, auth, billing, region_restricted, capability_unsupported, model_not_found, deprecated_model, unknown
  - 즉시 중단 (3): content_filter, context_window_exceeded, malformed_response
- **검증**: ERROR-CLASS-001 spec.md §6 14 enum 확인됨

### 1.4 SPEC-GOOSE-LLM-001

- **Status**: amendment-v0.2 적용된 상태 (active)
- **인터페이스**: `Provider.Capabilities() ProviderCapabilities` — `ProviderCapabilities{Vision, FunctionCalling, Streaming, VisionModels, MaxContextWindow}`
- **본 SPEC 활용**: ProviderCapabilities 의 boolean 필드를 본 SPEC 의 정적 CapabilityMatrix 의 source-of-truth 와 일관 유지. 단, prompt_caching/realtime 은 LLM-001 ProviderCapabilities 에 미존재 → 본 SPEC 에서 정적 매트릭스로 보충
- **Drift 회피**: 본 SPEC §6.1 매트릭스가 LLM-001 ProviderCapabilities 와 모순되지 않도록 P2 capability_test.go 에서 cross-check

---

## 2. Decorator Pattern 적용 가능성

### 2.1 v1 Router 의 idempotency

ROUTER-001 v1 의 `Router` 는 stateless (spec.md §3.1.9: "순수 함수에 가까운 상태 없는 객체, Thread-safe by design"). 따라서 v2 가 v1 을 base 로 주입해 호출해도 race condition 위험 없음.

### 2.2 BC 보장 fast path

v2 의 `RoutingPolicy{}` zero value 가 `PreferQuality` 모드 + 빈 chain + 빈 capability + 빈 exclude 일 때, v2 의 `Route()` 는 v1 의 결정을 변경 없이 반환하는 fast path 진입. AC-RV2-001 이 이를 검증.

```go
// router.go
if r.policy.Mode == PreferQuality && len(r.policy.FallbackChain) == 0 &&
   len(r.policy.RequiredCapabilities) == 0 && len(r.policy.ExcludedProviders) == 0 {
    return r.base.Route(ctx, req)  // v1 byte-identical
}
```

### 2.3 RoutingReason 확장의 BC 안정성

`Route.RoutingReason` 은 string 타입. v1 이 `"simple"` / `"complex"` 만 사용. v2 가 `"v1:simple"` / `"v2:policy_*"` prefix 추가. 사용자 코드가 RoutingReason 을 prefix-aware 하게 처리해야 하는 위험.

**완화책**:
- v1 의 기존 string 형식 그대로 유지하는 fast path (정책 비활성 시) — AC-RV2-001
- v2 prefix 사용은 명시적 v2 활성 시에만
- 사용자 가이드 (사용자 코드가 RoutingReason 을 switch-case 한다면 prefix-aware 로 업그레이드 필요)

---

## 3. OpenRouter 제외 의사결정 근거

### 3.1 시장 조사 (2026-04 기준)

OpenRouter 가 300+ 모델 gateway 로서 v2 routing 의 자연스러운 통합 후보로 보이는 이유:
- 단일 API key 로 다수 provider 접근
- "/v1/models" endpoint 로 동적 모델 list 획득 가능
- Provider routing 자체 logic 내장 (priority, fallback)

### 3.2 제외 사유 5 가지

| # | 사유 | 영향 |
|---|------|------|
| 1 | **이중 라우팅 충돌** | v2 결정 후 OpenRouter 내부 routing → debugging 어려움, traceability 손실 |
| 2 | **비용 모델 불투명** | OpenRouter 마진이 provider 별 가변 → prefer_cheap 정책의 비용 비교 정확도 저하 |
| 3 | **Rate limit 신호 손실** | OpenRouter 가 upstream rate limit header 일관 forward 안 함 → RATELIMIT-001 4-bucket tracker 정확도 저하 |
| 4 | **Capability 비대칭 손실** | OpenRouter 모델 list 가 prompt_caching/realtime 등 비대칭 capability 일관 노출 안 함 → CapabilityMatrix 정확도 저하 |
| 5 | **시장 변동성** | OpenRouter 의 가격/모델/policy 가 비교적 자주 변경 → 본 SPEC 의 정적 매트릭스 + 6 개월 update 정책과 충돌 |

### 3.3 제외 정책의 회귀 보호

- **AC-RV2-011** 이 "chain=[openrouter:gpt-oss] 명시 시 단순 provider 취급, prefer_cheap 가격 정렬 우대 안 함" 검증
- `pricing.go` 정적 표에 OpenRouter 의도적 부재 (회귀 test 가 검증)
- spec.md §2.2, §3.2, §14 에 일관 명시

### 3.4 6 개월 후 재평가 trigger

다음 조건 중 하나 충족 시 OpenRouter 통합 별도 SPEC 재검토:
- OpenRouter 가 upstream rate limit header 일관 forward 시작
- 사용자 요청 (issue tracker) 이 N 건 이상 누적
- 시장 점유율 (다른 gateway 등장) 변화로 재평가 필요

재평가 시점: 2026-11 (Sprint 1 종료 + 6 개월).

---

## 4. 정적 Pricing 표의 정확성 vs 일관성 trade-off

### 4.1 정확성 한계

per-million $ 의 input + output 평균값으로 단순 비교는 다음 부정확 요인:
- **Discount tier**: Together AI, Fireworks 의 batch/cached 50% 할인 미반영
- **Region 변동**: Qwen `intl/cn/sg/hk` 가격 차이 미반영
- **모델별 fine-grained**: Mistral Nemo $0.02/M 외 다른 Mistral 모델 가격 차이 누락
- **Token weighting**: input vs output 호출 비율 차이 (chat 은 보통 input >> output) 미반영

### 4.2 정확성보다 일관성 선택 이유

- **결정 단순성**: prefer_cheap 결정의 단순성이 사용자 신뢰 확보
- **회귀 가능성**: 정확한 가격 추론은 token counting + tiktoken 의존 → SPEC OUT (Section 3.2)
- **Fallback 안전망**: 가격 정렬 부정확하더라도 fallback chain 으로 회복 가능

### 4.3 6 개월 manual update 정책

- spec.md §13 maintenance 명시
- 다음 review: 2026-11
- Provider 공식 페이지 link 본 SPEC §15 references
- Update 시 amendment SPEC 또는 patch version bump

---

## 5. Capability 매트릭스의 정확성 검증 계획

### 5.1 검증 방법

- spec.md §6.1 매트릭스 vs 각 provider 공식 문서 (anthropic.com/docs, openai.com/docs 등) cross-check
- LLM-001 의 ProviderCapabilities boolean 필드와 일관 유지
- P2 capability_test.go 의 60 case (15 × 4) table-driven 으로 회귀 보호

### 5.2 미해결 모호성

- **Ollama function_calling**: 모델별 가변 (llama3.1+ 지원, mistral-nemo 미지원). 본 SPEC 은 "model dependent" 표시 + true 로 가정 (보수적 통과)
- **OpenRouter capability**: gateway 특성상 모델별 가변. 정적 매트릭스에 표시는 하되 routing 결정에서 OpenRouter 우대 안 함 (Section 3 참조)
- **Together/Fireworks vision**: "some" 표시 — 모델별 명세 후속 SPEC 에서 세분화

---

## 6. 구현 Risks 종합

| Risk | 검증 시점 | 완화 |
|-----|---------|-----|
| RATELIMIT-001 reader 시그니처 mismatch | P2 시작 | 어댑터 layer 흡수 |
| ERROR-CLASS-001 14 enum 확장 | 본 SPEC 진행 중 모니터링 | switch-case default → Unknown 보수적 매핑 |
| 정적 pricing 표 부정확 | 6 개월 단위 review | spec.md §13 maintenance 정책 |
| 정책 YAML schema 위반 silent fallback | P1 loader_test | 명시적 ErrUnknownPolicyMode 강제 |
| `~/.goose/routing-policy.yaml` 권한 누출 | P1 loader.go | 0600 강제 + os.Stat 검증 |

---

## 7. 결론

본 SPEC 의 v2 라우팅 레이어는 v1 ROUTER-001 위에 decorator pattern 으로 안전하게 얹을 수 있다. 의존 SPEC 4 종 모두 본 SPEC 가정과 호환 가능 (단, RATELIMIT-001 reader 시그니처는 P2 시점 검증 필요).

OpenRouter 의도적 제외 결정은 5 가지 사유로 정당화되며, 6 개월 후 재평가 trigger 가 명시되어 있다. AC-RV2-011 이 회귀 보호.

정적 pricing 표 + capability matrix 는 정확성보다 일관성을 우선했고, 6 개월 manual update 정책으로 부정확 요인을 회복한다.

본 SPEC 은 plan-auditor 1라운드 audit 진입 가능 상태.

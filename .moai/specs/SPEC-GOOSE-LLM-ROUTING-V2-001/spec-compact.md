# SPEC-GOOSE-LLM-ROUTING-V2-001 (compact)

> ROUTER-001 v1 위에 정책·capability·ratelimit·fallback chain decorator 를 얹는 v2 라우팅 레이어. 15 direct adapter cost/latency/capability-aware routing + manual fallback chain. **OpenRouter 의도적 제외** (Sprint 1, 6 개월 후 재평가). ~30% token saving 의 implementation reference snapshot.

**Meta**: id=SPEC-GOOSE-LLM-ROUTING-V2-001 | version=0.1.0 | status=audit-ready | priority=P1 | phase=4 | size=중(M) | lifecycle=spec-anchored
**Depends**: SPEC-GOOSE-ROUTER-001 (completed), SPEC-GOOSE-RATELIMIT-001, SPEC-GOOSE-ERROR-CLASS-001, SPEC-GOOSE-LLM-001, SPEC-GOOSE-ADAPTER-001, SPEC-GOOSE-ADAPTER-002

---

## 핵심 결정 사항

1. **Decorator 패턴**: v1 Router 미수정, v2 가 base 로 주입. 정책 미지정 = byte-identical BC.
2. **OpenRouter 의도적 제외**: 5 가지 사유 (이중 라우팅, 비용 불투명, ratelimit 손실, capability 비대칭, debugging 곤란). chain 단순 명시는 허용.
3. **15 provider × 4 capability 정적 매트릭스**: prompt_caching/function_calling/vision/realtime. 6 개월 manual update.
4. **14 FailoverReason 분기**: 11 → 다음 후보, 3 (ContentFilter/ContextWindowExceeded/MalformedResponse) → chain 즉시 중단.
5. **4 PolicyMode**: PreferQuality (default), PreferLocal, PreferCheap, AlwaysSpecific.
6. **80% threshold rate-limit-aware**: RPM/TPM/RPH/TPH 4 bucket 중 하나라도 ≥ 0.80 시 후보 제거.
7. **Silent recovery**: 후보 0 개면 v1 결정 그대로 반환 + `v2:fallback_exhausted_recover_v1` trace.

---

## REQ (14)

| ID | Type | Statement (1-line) |
|----|------|---------------------|
| REQ-RV2-001 | Ubiquitous | RouterV2 는 사용자 정책 mode 를 항상 우선 평가; PreferQuality 일 때만 v1 simple/complex 결정 유지 |
| REQ-RV2-002 | Ubiquitous | 정책 파일 부재 시 RoutingPolicy{Mode: PreferQuality, ...nil} 기본값으로 v1 와 byte-identical 동작 |
| REQ-RV2-003 | Ubiquitous | ProviderRegistry.AdapterReady=true 인 provider 만 후보로 인정; AdapterReady=false 는 chain 명시되어도 silent skip + 1회 알림 |
| REQ-RV2-004 | Ubiquitous | 정적 CapabilityMatrix 를 init 1회 로드, process lifetime 불변; 동적 갱신 OUT |
| REQ-RV2-005 | Event-Driven | When ErrorClassifier 가 11 reason (RateLimit/Server5xx/Network/Timeout/Auth/Billing/RegionRestricted/CapabilityUnsupported/ModelNotFound/DeprecatedModel/Unknown) 분류 시 chain 다음 후보 |
| REQ-RV2-006 | Event-Driven | When RoutingDecisionHook 등록 시 최종 Route 결정 직전 호출 + v2 decision trace 전달 |
| REQ-RV2-007 | Event-Driven | When Mode=PreferCheap 시 정적 pricing 표 input+output 평균 오름차순 정렬 + filter 통과한 가장 저렴한 후보 |
| REQ-RV2-008 | Event-Driven | When Mode=AlwaysSpecific + chain 비어있지 않으면 chain[0] 강제 (v1 결정 무시) |
| REQ-RV2-009 | State-Driven | While RateLimitView.BucketUsage 의 4 bucket 중 하나라도 ≥ threshold (default 0.80) 면 후보 제외 |
| REQ-RV2-010 | State-Driven | While RequiredCapabilities N 개 명시 동안 CapabilityMatrix 가 N 개 모두 지원하는 provider 만 통과 |
| REQ-RV2-011 | State-Driven | While fallback chain 실행 중 매 시도 직후 (FailoverReason, 순번, provider, model, duration) progress.md append-only |
| REQ-RV2-012 | Unwanted | If ExcludedProviders 가 chain 에 포함되면 chain 실행 중 silent skip (init 명시 에러 아님) |
| REQ-RV2-013 | Unwanted | If 3 reason (ContentFilter/ContextWindowExceeded/MalformedResponse) 발생 시 chain 즉시 중단 + 에러 그대로 반환 |
| REQ-RV2-014 | Unwanted | If 모든 후보 0 개면 v1 결정 silent recovery + RoutingReason="v2:fallback_exhausted_recover_v1" |

---

## AC (11)

| AC ID | REQ | One-liner | Phase |
|-------|-----|-----------|-------|
| AC-RV2-001 | 001, 002 | 정책 파일 부재 시 v1 byte-identical, RoutingReason="v1:simple" 보존 | P1 |
| AC-RV2-002 | 001, 008 | always_specific + chain[0]=groq 시 v1 anthropic 결정 무시하고 groq 강제 | P1, P3 |
| AC-RV2-003 | 007 | prefer_cheap + 동일 capability 시 ollama (free) → groq (free) → mistral:nemo ($0.02/M) 순 | P3 |
| AC-RV2-004 | 010 | required_capabilities=[vision] 시 deepseek/groq/cerebras/mistral/kimi/zai_glm 6개 제외 | P2 |
| AC-RV2-005 | 009 | RateLimitView mock anthropic RPM=0.85 시 anthropic 제외 + RoutingReason="v2:rate_limit_avoid_anthropic_rpm_0.85" | P2 |
| AC-RV2-006 | 005 | 1차 anthropic→429 (RateLimit) → chain 2차 openai 자동 시도 + trace 2 lines append | P3 |
| AC-RV2-007 | 013 | 1차 anthropic→ContentFilter → openai 호출 0회 + chain 즉시 중단 + 에러 그대로 반환 | P3 |
| AC-RV2-008 | 012 | excluded=[anthropic] + chain=[anthropic, openai] 시 anthropic silent skip → openai 직접 | P3 |
| AC-RV2-009 | 014 | required=[prompt_caching, realtime] (둘 다 충족 0개) → v1 결정 silent recovery + "v2:fallback_exhausted_recover_v1" | P3 |
| AC-RV2-010 | 011 | fallback trace `v2:fallback_chain_step_<n>_<reason> (provider, model, duration)` append-only | P3 |
| AC-RV2-011 | OpenRouter 정책 | chain=[openrouter:gpt-oss] 명시 시 단순 provider 취급, prefer_cheap 가격 정렬 우대 X, pricing.go 표 부재 | P4 |

---

## CapabilityMatrix (15 × 4)

```
              prompt_cache  func_call  vision  realtime
anthropic     ✅            ✅         ✅      ❌
openai        ❌            ✅         ✅      ✅
google        ❌            ✅         ✅      ❌
xai           ❌            ✅         ✅      ❌
deepseek      ❌            ✅         ❌      ❌
ollama        ❌            ✅(model)  ✅(llava) ❌
zai_glm       ❌            ✅         ❌      ❌
groq          ❌            ✅         ❌      ❌
openrouter    ❌(gateway)   ✅(model)  ✅(model) ❌
together      ❌            ✅         ✅(some) ❌
fireworks     ❌            ✅         ✅(some) ❌
cerebras      ❌            ✅         ❌      ❌
mistral       ❌            ✅         ❌      ❌
qwen          ❌            ✅         ✅(qwen3-vl) ❌
kimi          ❌            ✅         ❌      ❌
```

---

## Pricing 표 (per million $, 2026-05-05 유효, 6m manual update)

```
ollama:*                        0.00 / 0.00   (local)
groq:llama-3.3-70b              0.00 / 0.00   (free tier 30 RPM)
google:gemini-2.0-flash         0.075 / 0.30
mistral:nemo                    0.02 / 0.02   (cheapest paid)
deepseek:deepseek-chat          0.27 / 1.10
zai_glm:glm-4.6                 0.50 / 1.50
kimi:k2.6                       0.60 / 1.80
qwen:qwen3-max                  0.80 / 2.40
cerebras:llama-3.3-70b          0.85 / 1.20
together:llama-3.3-70b-turbo    0.88 / 0.88
anthropic:claude-sonnet-4.6     3.00 / 15.00
fireworks:llama-3.1-405b        3.00 / 3.00
openai:gpt-4o                   2.50 / 10.00
xai:grok-3                      5.00 / 15.00
anthropic:claude-opus-4-7       15.00 / 75.00
openai:o1-preview               15.00 / 60.00

* OpenRouter 의도적 제외 (Section 2.2)
```

---

## Phase 분해 (4)

| Phase | 산출물 | Owner files |
|-------|------|------------|
| P1 | Policy schema + YAML loader | `policy.go`, `loader.go` |
| P2 | Capability matrix + RateLimit reader | `capability.go`, `ratelimit_filter.go` |
| P3 | Fallback chain + Decorator + Pricing | `fallback.go`, `router.go`, `pricing.go`, `trace.go` |
| P4 | Integration tests + OpenRouter 검증 | `integration_test.go`, `testdata/policy_*.yaml` |

**Files (NEW)**: 9 in `internal/llm/router/v2/`
**Files (MODIFIED)**: 0 (decorator 패턴, v1 미변경)

---

## RoutingReason v2 prefix 형식

| Format | Trigger |
|-------|---------|
| `v1:simple` / `v1:complex` | v2 정책 비활성, v1 결정 그대로 |
| `v2:policy_<mode>_<provider>` | v2 정책 적용으로 v1 결정 변경 |
| `v2:capability_<cap>_required_<count>_candidates` | required capabilities 필터 적용 |
| `v2:rate_limit_avoid_<provider>_<bucket>_<usage>` | 80% 임계 회피 |
| `v2:fallback_chain_step_<n>_<reason>` | chain n번째 시도, reason 매핑 |
| `v2:fallback_exhausted_recover_v1` | chain 모두 실패, v1 silent recovery |

---

## Quality Gates

- Coverage ≥ 90% per file (P1, P2, P3), ≥ 92% integration (P4)
- `go test -race ./internal/llm/router/v2/...` pass
- `gofmt -l ./internal/llm/router/v2/...` 빈 출력
- `golangci-lint run ./internal/llm/router/v2/...` 0 issue
- `BenchmarkRouterV2_Route` < 5ms p99
- v1 Router 사용자 코드 변경 0 (BC 보장)

---

## Out of Scope (8)

1. OpenRouter v2 router 우대/할인 (Sprint 1 의도적 제외, 6m 후 재평가)
2. Auto-failover chain 자동 생성
3. Token counting cost prediction (정적 표만)
4. Multi-region routing (provider option 으로만)
5. 학습 기반 routing (INSIGHTS-001 후속)
6. CapabilityMatrix 동적 갱신 (정적 매트릭스만)
7. Streaming 결정 (ADAPTER-001/002)
8. Embeddings/audio routing (text completion 만)

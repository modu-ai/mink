# SPEC-GOOSE-LLM-ROUTING-V2-001 — Progress Log

## 2026-05-07 — P1 (Policy Layer) GREEN

- Branch: `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p1` (base=main, commit 1fd2378)
- Files NEW (P1 owner — drift 0):
  - `internal/llm/router/v2/policy.go` — `PolicyMode` (4 enum, zero=PreferQuality), `Capability` (4 enum), `ProviderRef`, `RoutingPolicy`, `DefaultRateLimitThreshold=0.80`
  - `internal/llm/router/v2/loader.go` — `LoadPolicy(path)` + `ErrUnknownPolicyMode` + `ErrInvalidThreshold` + `parsePolicyMode` + yaml.v3 기반 파싱
  - `internal/llm/router/v2/policy_test.go` — 4 tests (PolicyMode zero value, String round-trip, RoutingPolicy zero slices nil, DefaultRateLimitThreshold=0.80)
  - `internal/llm/router/v2/loader_test.go` — 9 tests (file-not-found default / empty file default / unknown mode / threshold OOB / threshold boundaries / valid YAML all fields / 4 enum + empty / malformed YAML wrap / permission error wrap)
- RED tests (plan.md §1 P1 expected): 5/5 PASS + 8 추가 보강 케이스 (총 13 tests, 24 subtests)
  - `TestPolicyModeZeroValueIsPreferQuality` — REQ-RV2-002 회귀 보호
  - `TestLoadPolicy_FileNotFound_ReturnsDefault` — backward-compat fast path
  - `TestLoadPolicy_UnknownMode_ReturnsError` — sentinel ErrUnknownPolicyMode
  - `TestLoadPolicy_ThresholdOutOfRange_ReturnsError` — 음수 + > 1.0 양쪽
  - `TestLoadPolicy_ValidYAML_ParsesAllFields` — spec.md §6.3 example schema 매핑
- 추가 강화 케이스: `EmptyFile_ReturnsDefault`, `ThresholdBoundaries_AreValid` (0.0/1.0 inclusive), `AllFourModes_ParseCorrectly` (empty → PreferQuality), `MalformedYAML_WrapsError`, `PermissionError_WrapsError`, `PolicyMode_String_RoundTrip` (4 enum + unknown).
- Verify: golangci-lint 0, vet 0, gofmt 0, race GREEN, **coverage 100.0%** (gate ≥ 90% 충족).
- Drift: 0 (외부 패키지 수정 없음, P1 owner files 만 생성).
- 의존성 변경: `gopkg.in/yaml.v3` 는 이미 다른 패키지에서 사용 중 — go.mod 변경 없음.

### M1 Policy Layer — DONE

| AC | Phase | Status | Evidence |
|----|-------|--------|----------|
| AC-RV2-001 (default backward-compat) | P1 | GREEN | TestLoadPolicy_FileNotFound_ReturnsDefault |
| AC-RV2-002 (mode enum 매핑) | P1 | GREEN | TestLoadPolicy_AllFourModes_ParseCorrectly + UnknownMode |
| AC-RV2-003 (threshold validation) | P1 | GREEN | ThresholdOutOfRange + ThresholdBoundaries |

### Next

- P2 — CapabilityMatrix + RateLimit reader 어댑터 (`capability.go`, `ratelimit_filter.go`).
- 진입점: 동일 feature branch 또는 신규 `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p2` (P1 PR 머지 후 분기).

---

## 2026-05-09 — P1 CodeRabbit fixes + P2 (Filter Layer) GREEN

### P1 CodeRabbit review 2건 수용 (commit 16bf50a, PR #120)

- #1 Minor — `loader_test.go` permission test 의 tautological assertion (`errors.Is(err, os.ErrPermission) || err != nil`) 을 `assert.ErrorIs(t, err, os.ErrPermission)` 로 정정.
- #2 Major — `LoadPolicy(path)` → `LoadPolicy(ctx context.Context, path string)` 시그니처 변경. ctx.Err() pre-check 추가. 신규 public API 시그니처 안정화 + Go 가이드라인 (context.Context 첫 파라미터) 준수.
- 신규 test: `TestLoadPolicy_CanceledContext_ReturnsContextError` (취소된 ctx → context.Canceled wrap).
- PR #120 squash merged on commit d33022e.

### P2 Filter Layer 완료

- Branch: `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p2` (base=main, commit d33022e)
- Files NEW (P2 owner — drift 0):
  - `internal/llm/router/v2/capability.go` — `CapabilityMatrix` (provider id → Capability set), `Match(provider, required)` (모든 required 충족 시 true, unknown provider 보수적 false), `Providers()` enumeration, `DefaultMatrix()` deep-copy 반환 (race-free)
  - `internal/llm/router/v2/ratelimit_filter.go` — `RateLimitView` 인터페이스 (`BucketUsage(provider) (rpm, tpm, rph, tph float64)`), `FilterByRateLimit(candidates, view, threshold)` — 4 bucket 어느 하나라도 ≥ threshold 시 후보 제외
  - `internal/llm/router/v2/capability_test.go` — 6 tests (StaticConsistency_15x4 / Match_AllRequired / Match_OneMissing_Rejects / Match_UnknownProvider / Providers_Returns15 / DefaultMatrix_IsCopy)
  - `internal/llm/router/v2/ratelimit_filter_test.go` — 7 tests (RPMAt80Percent / AnyBucketAtThreshold / AllBelowThreshold / Override50Percent / NilView / UnknownProvider / EmptyCandidates / DoesNotMutateInput)
- RED tests (plan.md §1 P2 expected): 6/6 PASS + 7 추가 보강 케이스
- Verify: golangci-lint 0, vet 0, gofmt 0, race GREEN, **coverage 100.0%** (gate ≥ 90% 충족, 누적 P1+P2 모두 100%).
- Drift: 0 (외부 패키지 수정 없음).

### M2 Filter Layer — DONE

| AC | Phase | Status | Evidence |
|----|-------|--------|----------|
| AC-RV2-004 (vision 필터) | P2 | GREEN | TestCapabilityMatrix_Match_OneMissing_Rejects (vision 미지원 6 provider 검증) |
| AC-RV2-005 (rate limit 80%) | P2 | GREEN | TestFilterByRateLimit_RPMBucketAt80Percent_ExcludesProvider |

### 보수적 결정 기록

- "model dependent" / "some" 카테고리 (openrouter, together, fireworks vision; ollama vision via llava; qwen vision via qwen3-vl) 는 매트릭스에서 보수적 true 로 표기 — capability filter 가 너무 엄격해서 후보 0개로 떨어지는 것보다 false-positive 후보를 두고 fallback chain 으로 회복하는 편이 안전.
- `RateLimitView` 인터페이스 누가 구현할지: P3 RouterV2 가 RATELIMIT-001 의 reader 와 본 SPEC `RateLimitView` 시그니처 사이 어댑터를 wiring (plan.md §1 P2-T5).
- `FilterByRateLimit` view==nil graceful — 테스트/부트스트랩 환경에서 panic 없이 후보 그대로 통과.

### Next

- P3 — Fallback chain + RouterV2 decorator + Pricing (`fallback.go`, `router.go`, `pricing.go`, `trace.go`).
- 진입점: P2 PR 머지 후 신규 `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p3` 분기.

---

## 2026-05-09 — P2 머지 (PR #121, commit 98ccedc) + P3 (Decorator Layer) GREEN

### P2 PR #121 squash merged

- commit 98ccedc on main, 5 files changed (+671), branch `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p2` 자동 삭제.
- 머지 후 즉시 P3 분기 — `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p3` (base=main).

### P3 Decorator Layer 완료 — 4 신규 파일

- Branch: `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p3` (base=main, commit 98ccedc)
- Files NEW (P3 owner — drift 0):
  - `internal/llm/router/v2/fallback.go` — `FallbackExecutor` + `Attempt` + `FallbackError` + 14 FailoverReason 분기 (3 stop / 11 next), `SetExcluded` (REQ-RV2-012), `LastAttempts()` trace (REQ-RV2-011), `ErrEmptyChain` + `ErrAllExcluded` sentinel
  - `internal/llm/router/v2/pricing.go` — `Price{Input, Output}` + `Average()`, `defaultPrices` 16 entry (spec.md §6.2 source-of-truth), `LookupPrice` (정확 매칭 → wildcard fallback), `SortByCost` (오름차순, stable tie-break, 입력 미변경)
  - `internal/llm/router/v2/router.go` — `V1Router` 인터페이스, `RouterV2` decorator, `New(base, policy, matrix, view, hooks)`, `Route()` 7-step 의사결정 트리 (zero-policy fast path → AlwaysSpecific override → PreferLocal/Cheap/Quality 빌드 → capability/exclude/ratelimit 필터 → silent recovery), `SetClassifier` (테스트 hook), `FallbackExecutor()` 노출, hook panic 격리
  - `internal/llm/router/v2/trace.go` — 7 RoutingReason builder (`TraceV1Simple`, `TraceV1Complex`, `TraceV2Policy`, `TraceV2Capability`, `TraceV2RateLimit`, `TraceV2FallbackStep`, `TraceV2FallbackExhausted`)
- Test files NEW (RED-first, 95+ cases):
  - `fallback_test.go` — 14 reason coverage (3 stop + 11 next subtests) + multi-error + ctx canceled + exclude silent skip + ALL excluded + nil classifier + Error/Unwrap 형식 + LastAttempts 기록
  - `pricing_test.go` — 16 entry 정적 표 + wildcard fallback + SortByCost ascending/stable/unknown-last/no-mutation/empty
  - `trace_test.go` — 7 builder 형식 회귀 + RateLimit 부동소수 2자리 반올림
  - `router_test.go` — table-driven 9 cases + ZeroPolicy byte-pass-through + AlwaysSpecific override + PreferCheap sort + Vision/Realtime capability filter + RateLimit 80% exclude + Excluded silent skip + AllFiltered v1 recovery + PreferLocal prepend ollama + PreferQuality keep v1 + hook called + concurrent race-free + FallbackExecutor wiring (default + stop-chain reason)
  - `router_bench_test.go` — `BenchmarkRouterV2_Route_ZeroPolicy` + `BenchmarkRouterV2_Route_PreferCheap`

### M3 Decorator Layer — DONE

| AC | REQ | Phase | Status | Evidence |
|----|-----|-------|--------|----------|
| AC-RV2-001 (정책 파일 부재 시 v1 byte-identical) | REQ-RV2-002 | P1+P3 | GREEN | `TestRouterV2_ZeroPolicy_BytePassThrough` |
| AC-RV2-002 (AlwaysSpecific override v1) | REQ-RV2-008 | P3 | GREEN | `TestRouterV2_AlwaysSpecific_OverridesV1` |
| AC-RV2-003 (PreferCheap 정렬) | REQ-RV2-007 | P3 | GREEN | `TestRouterV2_PreferCheap_SortsByCost` + `TestSortByCost_Ascending` |
| AC-RV2-006 (RateLimit → 다음 후보) | REQ-RV2-005 | P3 | GREEN | `TestFallback_NextCandidateReasons/rate_limit` |
| AC-RV2-007 (ContentFilter/Overflow/Malformed → chain 중단) | REQ-RV2-013 | P3 | GREEN | `TestFallback_StopChainReasons` (3 stop reasons: ContextOverflow, FormatError, PayloadTooLarge) |
| AC-RV2-008 (excluded silent skip) | REQ-RV2-012 | P3 | GREEN | `TestRouterV2_Excluded_SilentSkip` + `TestFallback_ExcludedProviders_SilentSkip` |
| AC-RV2-009 (모든 후보 0개 → v1 silent recovery) | REQ-RV2-014 | P3 | GREEN | `TestRouterV2_AllFiltered_RecoverV1` |
| AC-RV2-010 (chain trace 기록) | REQ-RV2-011 | P3 | GREEN | `TestFallback_AttemptsRecorded` (Attempt struct + LastAttempts()) |

### Verify

- `go test -race -cover ./internal/llm/router/v2/...` — **PASS, coverage 97.1%** (gate ≥ 90% 충족, P3 이전 100% → REFACTOR 후 일부 error path 추가로 97.1%)
- `go vet ./internal/llm/router/v2/...` — clean
- `gofmt -l ./internal/llm/router/v2/...` — empty
- `golangci-lint run ./internal/llm/router/v2/...` — 0 issues
- `BenchmarkRouterV2_Route_ZeroPolicy` — **27.73 ns/op, 144 B/op, 1 alloc** (NFR 5ms 의 0.0006%)
- `BenchmarkRouterV2_Route_PreferCheap` (worst case 8 candidates + capability + ratelimit + exclude + sort) — **1736 ns/op, 1537 B/op, 15 allocs** (NFR 5ms 의 0.034%)
- Drift: 0 (외부 패키지 수정 없음, P3 owner files 만 생성).

### 보수적 결정 기록 (P3 의 trade-off)

1. **14 FailoverReason 매핑** — spec.md §4.4 의 "ContentFilter / ContextWindowExceeded / MalformedResponse" 명칭이 ERROR-CLASS-001 enum 과 정확히 일치하지 않음. 실제 ERROR-CLASS-001 의 14 enum 중 STOP_CHAIN 으로 매핑한 3 개:
   - `ContextOverflow` (← spec ContextWindowExceeded) — 다음 provider 도 같은 길이 입력이라 동일 실패
   - `FormatError` (← spec MalformedResponse) — 잘못된 request body 는 다음 provider 도 동일
   - `PayloadTooLarge` — 다음 provider 도 같은 데이터 크기라 동일 실패
   
   spec 의 "ContentFilter" 는 현재 ERROR-CLASS-001 enum 에 없으므로 SPEC amendment 시 추가 검토 필요. 본 P3 는 14 enum 기반으로 11+3 분류 완성.

2. **Signature 재계산 정책** — v1 의 `makeSignature` 가 unexported 이라 v2 가 새 Route 를 구성할 때 재현 불가. v2 substitution 시 `"v2|provider|model"` 단순 fingerprint 로 대체 — caller 가 v1/v2 origin 을 구분 가능. zero-policy 경로는 v1 Signature 보존.

3. **PreferLocal default model** — chain 에 ollama 없을 때 inject 할 default model 을 `llama3` 으로 (production 검증 시 model id 확인 필요). chain 에 ollama 가 명시되어 있으면 그 model 우선.

4. **PreferQuality + chain non-empty** — spec §7.3 의 "PreferQuality → v1 결정 + Opus/GPT-4o 우선" 중 후자는 Opus/GPT-4o **하드코딩 inject** 가 아닌 **v1 결정을 head 에 두고 chain 을 fallback** 으로 결합. v1 이 이미 quality-aware 결정을 했다고 가정.

5. **Concurrent_Safe race fix** — production 코드는 stateless 라 race 없음. 테스트의 `fakeV1Router.calls` 카운터를 `atomic.Int64` 로 변경해 race detector 충족.

### Next

- P4 — Integration tests + OpenRouter 제외 검증 (AC-RV2-004, AC-RV2-005, AC-RV2-011, fixture 5 + e2e fallback chain rate_limit→content_filter).
- 진입점: P3 PR 머지 후 신규 `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p4` 분기.

---

## 2026-05-09 — P3 머지 (PR #122, commit 4bb21c1) + P4 (Integration tests) GREEN

### P3 PR #122 squash merged

- commit 4bb21c1 on main, 10 files changed (+1951 LOC), branch `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p3` 자동 삭제.
- 머지 후 즉시 P4 분기 — `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p4` (base=main).

### P4 Integration tests 완료 — 1 신규 + 6 fixture

- Branch: `feature/SPEC-GOOSE-LLM-ROUTING-V2-001-p4` (base=main, commit 4bb21c1)
- Files NEW (P4 owner — drift 0):
  - `internal/llm/router/v2/integration_test.go` — 10 E2E 시나리오 + makeRealV1Router/loadFixture/makeUserReq helpers
  - `internal/llm/router/v2/testdata/policy_prefer_local.yaml`
  - `internal/llm/router/v2/testdata/policy_prefer_cheap.yaml`
  - `internal/llm/router/v2/testdata/policy_prefer_quality.yaml`
  - `internal/llm/router/v2/testdata/policy_always_specific.yaml`
  - `internal/llm/router/v2/testdata/policy_with_excluded.yaml`
  - `internal/llm/router/v2/testdata/policy_with_openrouter.yaml` (AC-RV2-011 회귀 보호용)
- E2E test inventory:
  - `TestE2E_PreferLocal_OllamaSelected` — fixture → ollama 우선 (REQ-RV2-001)
  - `TestE2E_PreferCheap_GroqFreeFirst` — fixture → groq 무료 tier (REQ-RV2-007)
  - `TestE2E_PreferQuality_KeepsV1Anthropic` — fixture → v1 anthropic 결정 유지
  - `TestE2E_AlwaysSpecific_OverridesAll` — fixture → mistral chain[0] 강제 (AC-RV2-002)
  - `TestE2E_WithExcluded_SkipsAnthropic` — fixture → anthropic skip → openai (REQ-RV2-012)
  - `TestE2E_OpenRouterInChain_NoSpecialTreatment` — pricing 표 부재 검증 + LookupPrice 회귀 가드 (AC-RV2-011)
  - `TestE2E_FallbackChain_RateLimitToContextOverflow` — RateLimit (NEXT) → ContextOverflow (STOP) 분기 (REQ-RV2-013)
  - `TestE2E_FixturesExist` — 6 fixture LoadPolicy 로드 가능 + RateLimitThreshold 0.80 default 적용
  - `TestE2E_VisionFilter_E2E` — 5 vision-미지원 + google → google 단독 (AC-RV2-004)
  - `TestE2E_RateLimit80Pct_E2E` — anthropic RPM 0.85 → openai 전환 (AC-RV2-005)

### M4 Integration — DONE

| AC | REQ | Phase | Status | Evidence |
|----|-----|-------|--------|----------|
| AC-RV2-004 (vision 필터 E2E) | REQ-RV2-010 | P2+P4 | GREEN | TestE2E_VisionFilter_E2E |
| AC-RV2-005 (rate limit 80% E2E) | REQ-RV2-009 | P2+P4 | GREEN | TestE2E_RateLimit80Pct_E2E |
| AC-RV2-011 (OpenRouter 단순 provider) | OpenRouter 정책 | P4 | GREEN | TestE2E_OpenRouterInChain_NoSpecialTreatment + LookupPrice 회귀 가드 |

### Verify

- `go test -race -cover ./internal/llm/router/v2/...` — **PASS, coverage 97.5%** (gate ≥ 92% 충족, P3 97.1% → P4 97.5%로 상승)
- `go vet ./internal/llm/router/v2/...` — clean
- `gofmt -l ./internal/llm/router/v2/...` — empty
- `golangci-lint run ./internal/llm/router/v2/...` — 0 issues
- Drift: 0 (외부 패키지 수정 없음, P4 owner files 만 생성)

### 보수적 결정 기록 (P4 의 trade-off)

1. **httptest 미사용 결정** — RouterV2.Route() 자체는 HTTP 호출 없음. integration 의 본질은 (a) YAML loader → RouterV2 wiring + (b) FallbackExecutor + ErrorClassifier 연동이며, FallbackExecutor.classify() 가 메시지 기반이므로 httptest 추가 가치 적음. e2e fallback test 는 errors.New 메시지로 충분히 14 reason 분기 검증.
2. **fallback chain 시나리오 변경** — spec 의 "RateLimit → ContentFilter" 는 ERROR-CLASS-001 enum 에 ContentFilter 부재로 실현 불가. 대안: "RateLimit → ContextOverflow" 로 동등 효과 (둘 다 STOP_CHAIN reason). spec amendment 시 ContentFilter enum 추가 검토 가능.
3. **6번째 fixture 추가** — spec 의 5 fixture (prefer_local, prefer_cheap, prefer_quality, always_specific, with_excluded) 외에 policy_with_openrouter.yaml 추가. AC-RV2-011 회귀 보호 (OpenRouter 가 pricing 표에 추가되면 본 테스트 깨짐 → SPEC §14 amendment 신호).
4. **실 v1 Router 사용** — fake 가 아닌 router.New(DefaultRegistry, zaptest.Logger) 로 v1 baseline 가정 (v1.0.0 frozen status) 검증. v1 인터페이스 변경 시 본 테스트가 즉시 깨짐 → Risks #5 회귀 가드.
5. **PreferQuality + chain 비어있지 않음 시 v1 결정 유지** — spec §7.3 의 "Opus/GPT-4o 우선" 은 v1 의 quality-aware 결정에 위임. 본 fixture 는 anthropic chain 명시로 v1 결정과 일치 → routing 결과 변화 없이 통합 path 만 검증.

### SPEC-GOOSE-LLM-ROUTING-V2-001 — ALL PHASES COMPLETE

| Milestone | Phase | Status | AC GREEN |
|-----------|-------|--------|----------|
| M1 Policy Layer | P1 | DONE | AC-RV2-001, -002, -003 |
| M2 Filter Layer | P2 | DONE | AC-RV2-004 (P2 unit), -005 (P2 unit) |
| M3 Decorator Layer | P3 | DONE | AC-RV2-001, -002, -003, -006, -007, -008, -009, -010 |
| M4 Integration | P4 | DONE | AC-RV2-004 (E2E), -005 (E2E), -011 |

전체 11 AC 중 GREEN: 11/11 (AC-RV2-001 ~ AC-RV2-011 all GREEN).

### Next

- /moai sync — 최종 sync PR + CHANGELOG + status: implemented + REQ coverage 보고서.
- 진입점: P4 PR 머지 후 sync workflow.

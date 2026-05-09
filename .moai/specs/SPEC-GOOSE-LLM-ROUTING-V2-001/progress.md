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

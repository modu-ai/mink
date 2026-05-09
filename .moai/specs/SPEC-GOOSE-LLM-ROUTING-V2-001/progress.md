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

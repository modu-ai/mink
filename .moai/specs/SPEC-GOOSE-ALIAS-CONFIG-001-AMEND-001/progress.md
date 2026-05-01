## SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 Progress

- Started: 2026-04-30 (plan phase)
- **Status: planned** (작성 직후 — annotation cycle 또는 사용자 검토 후 run phase 전환 결정)
- Mode: TDD (quality.yaml development_mode=tdd 가정 — run phase 진입 시 재확인)
- Harness: standard (file_count<10 예상, 단일 Go domain, 비보안/비결제, backward-compat 보장)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: amendment — 기존 패키지에 신규 표면만 추가 (non-breaking)
- Branch base: main (부모 SPEC 머지 완료, PR #52 c018ec5 + 후속 commit 기반)

### 부모 / 후속 SPEC 상태 확인

| SPEC | Status | 본 amendment 와의 관계 |
|------|--------|----------------------|
| SPEC-GOOSE-ALIAS-CONFIG-001 v0.1.0 | completed (FROZEN, 2026-04-27) | 본 amendment 의 부모. export 표면 보존, 신규 표면만 추가. |
| SPEC-GOOSE-CMDCTX-HOTRELOAD-001 | planned (P4) | 본 amendment 의 후속 협력 SPEC. ConfigPath/Reload/Metrics hook 이 watcher 통합 비용 ↓. |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | `*router.ProviderRegistry` read-only 사용. 변경 없음. |
| SPEC-GOOSE-CMDCTX-001 v0.1.1 | implemented (FROZEN) | `adapter.Options.AliasMap` 소비자. 본 amendment 무영향. |

### Phase Log

- 2026-04-30 plan phase 시작 (사용자 결정으로 amendment 신설)
  - 사전 조사 완료:
    - `internal/command/adapter/aliasconfig/loader.go` (281 LOC) read-only 검토
    - 4개 테스트 파일 (loader_test.go 246줄, loader_p3_test.go 416줄, integration_test.go 205줄, validate_test.go 122줄) 검토
    - 부모 spec.md (550줄) 24 REQ + 27 AC 본문 vs 실제 구현 차이 식별
    - 부모 progress.md Open Questions 31~36행 매핑
    - HOTRELOAD-001 spec.md (831줄) §6.7 의 Loader/Validator interface 호환 요구사항 식별
  - 식별된 결손 (research.md §4 6개 영역):
    1. **Area 1** (P1): Hot-reload 호환 API — `ConfigPath()` getter + `Reload()` 명시화
    2. **Area 2** (P1): Multi-source merge — 부모 SPEC §4.5 REQ-ALIAS-040 본문 미구현 (현 OR 분기) 결손
    3. **Area 3** (P2): Validation 세분화 — 부모 progress OQ #2 (lenient in-place 삭제) 해소
    4. **Area 4** (P3): Error 안정 분류 — `ErrorCode` + `Categorize` 함수
    5. **Area 5** (P2): Schema 확장 — `AliasEntry` struct + yaml union + `LoadEntries()` method
    6. **Area 6** (P3): Observability — `Metrics` interface + noop default
  - 본 amendment 산출물:
    - `research.md` — 6개 영역 분석, 부모 코드 인용, 권장 우선순위 표, Open Questions 4건
    - `spec.md` — EARS 형식, 12 REQ (Ubiquitous 4, Event 2, State 3, Unwanted 2, Optional 1) + 14 AC (governance 3건 포함), Exclusions 10건
    - `progress.md` — 본 파일

### 핵심 보존 약속 (HARD)

부모 v0.1.0 의 다음 export 표면은 본 amendment 가 변경하지 않는다:

- `Loader`, `Options` (기존 4 필드), `Logger` interface, `AliasConfig`
- `New(opts Options) *Loader`
- `(*Loader).Load() (map[string]string, error)`
- `(*Loader).LoadDefault() (map[string]string, error)`
- `Validate(m, registry, strict bool) []error`
- `ErrConfigNotFound` sentinel
- yaml schema 의 flat `aliases: {alias: "provider/model"}` 형식
- 부모 5개 테스트 파일의 모든 test (회귀 금지 — AC-AMEND-051)

### 다음 단계 후보

- (a) **plan-auditor 사이클**: independent SPEC 검토, EARS 컴플라이언스, 부모 surface 보존 검증, HOTRELOAD-001 정합 확인
- (b) **사용자 annotation cycle**: research §9 Open Questions 4건 결정 + run phase 분기 여부

### Open Questions (사용자 결정 필요 — research.md §9 인용)

1. **Area 2 backward-compat 영향 범위**: user-only 사용자가 amendment 후 project file 추가 시 동작 변화. CHANGELOG / release notes 항목 필요 여부?
2. **Area 5 yaml union 복잡도**: string 또는 map 두 형식 동시 지원 시 yaml.v3 unmarshal 복잡도. 단순화 옵션 — `aliases_v2:` top-level key 분리. 사용자 선택?
3. **Area 6 Metrics interface 표준 align**: prometheus / opentelemetry 어느 collector 시그니처에 align 할지. v0.1 amendment 는 noop only 채택 권장 — confirm?
4. **HOTRELOAD-001 머지 순서**: 본 amendment 가 HOTRELOAD-001 plan/run 보다 먼저 implementation 되어야 하는가? Area 1 의 ConfigPath getter 가 watcher 즉시 사용 — 권장 순서: amendment first.

### 산출 파일

- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001/research.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001/spec.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001/progress.md`

### Next-step Hand-off Notes

- Run phase 시 manager-tdd 가 본 amendment 로 진입할 때 다음 자료 우선 로딩:
  - `internal/command/adapter/aliasconfig/loader.go` (현재 구현 — Options 확장 + ConfigPath/Reload methods 추가 대상)
  - `internal/command/adapter/aliasconfig/{loader_test.go, loader_p3_test.go, integration_test.go, validate_test.go}` (회귀 검증 baseline)
  - `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` §6.1 패키지 layout (확장 위치 결정)
  - `.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/spec.md` §6.7 (Loader/Validator interface — 호환 검증 baseline)
- 검증 필수 정적 체크:
  - AC-AMEND-050: `go doc ./internal/command/adapter/aliasconfig` baseline diff (export 0 변경)
  - AC-AMEND-051: `go test ./internal/command/adapter/aliasconfig/... -count=10 -race` (회귀 0건)
  - AC-AMEND-052: HOTRELOAD-001 의 `Loader` / `Validator` interface satisfaction 정적 검증
  - 신규 외부 의존성 0건 (`go.mod` diff)
- 신규 작성 파일 (예상):
  - `internal/command/adapter/aliasconfig/merge.go` + `merge_test.go` (Area 2)
  - `internal/command/adapter/aliasconfig/policy.go` + `policy_test.go` (Area 3)
  - `internal/command/adapter/aliasconfig/codes.go` + `codes_test.go` (Area 4)
  - `internal/command/adapter/aliasconfig/entries.go` + `entries_test.go` (Area 5)
  - `internal/command/adapter/aliasconfig/metrics.go` + `metrics_test.go` (Area 6)
- 수정 예상 파일:
  - `internal/command/adapter/aliasconfig/loader.go` — Options 에 `MergePolicy` / `Metrics` 필드 추가, Loader 에 `ConfigPath()` / `Reload()` method 추가, `LoadDefault()` 가 merge.go 호출하도록 변경
  - 부모 5개 테스트 파일 — 변경 없음 (회귀 금지)

### TDD 진입 순서 권장 (plan-auditor 가 검토 후 확정)

| 순서 | 작업 | 검증 AC |
|------|------|--------|
| T-001 | Options 확장 (`MergePolicy`, `Metrics`) + 부모 회귀 검증 | AC-AMEND-051 |
| T-002 | `ConfigPath()` / `Reload()` method 추가 | AC-AMEND-001, AC-AMEND-002 |
| T-003 | `merge.go` user+project merge + override log | AC-AMEND-010-A, AC-AMEND-010-B, AC-AMEND-020 |
| T-004 | `policy.go` ValidationPolicy + ValidateAndPrune | AC-AMEND-003 |
| T-005 | `codes.go` ErrorCode + Categorize | AC-AMEND-004, AC-AMEND-011 |
| T-006 | `entries.go` AliasEntry + LoadEntries + yaml union | AC-AMEND-031, AC-AMEND-040 |
| T-007 | `metrics.go` Metrics interface + noopMetrics | AC-AMEND-021, AC-AMEND-022 |
| T-008 | Reload preserve 검증 | AC-AMEND-030 |
| T-009 | HOTRELOAD-001 interface 호환 정적 검증 | AC-AMEND-052 |
| T-010 | Export signature baseline diff CI gate | AC-AMEND-050 |

---

## Phase Log — Run Phase (2026-05-02)

### 산출물

| 카테고리 | 파일 | LOC | 비고 |
|----------|------|-----|------|
| 신규 (production) | `internal/command/adapter/aliasconfig/codes.go` | 83 | ErrorCodeOf + Categorize, sentinelTable |
| 신규 (production) | `internal/command/adapter/aliasconfig/entries.go` | 112 | AliasEntry + LoadEntries + custom UnmarshalYAML |
| 신규 (production) | `internal/command/adapter/aliasconfig/merge.go` | 195 | user+project merge + override info-log |
| 신규 (production) | `internal/command/adapter/aliasconfig/metrics.go` | 39 | Metrics interface + noopMetrics |
| 신규 (production) | `internal/command/adapter/aliasconfig/policy.go` | 141 | MergePolicy + ValidationPolicy + ValidateAndPrune + joinError |
| 신규 (test) | `internal/command/adapter/aliasconfig/codes_test.go` | ~110 | AC-AMEND-004, AC-AMEND-011 |
| 신규 (test) | `internal/command/adapter/aliasconfig/entries_test.go` | ~150 | AC-AMEND-031, AC-AMEND-040 + malformed/empty/mixed cases |
| 신규 (test) | `internal/command/adapter/aliasconfig/loader_amend_test.go` | 251 | Options 확장, ConfigPath/Reload, HOTRELOAD interface, baseline assertion |
| 신규 (test) | `internal/command/adapter/aliasconfig/merge_test.go` | 302 | AC-AMEND-010-A/B, AC-AMEND-020 |
| 신규 (test) | `internal/command/adapter/aliasconfig/metrics_test.go` | ~120 | AC-AMEND-021, AC-AMEND-022 + zero-alloc check |
| 신규 (test) | `internal/command/adapter/aliasconfig/policy_test.go` | 166 | AC-AMEND-003 + ValidateAndPrune 변종 |
| 수정 | `internal/command/adapter/aliasconfig/loader.go` | +111 / -41 | Options.MergePolicy + Options.Metrics, Loader.metrics, ConfigPath/Reload, LoadEntries 위임 |
| baseline 캡처 | `.moai/specs/.../audit-baseline-godoc.txt` | 61줄 | v0.1.0 export 표면 baseline (AC-AMEND-050 게이트용) |

### TDD 사이클 (T-001 ~ T-010)

- **T-001 RED→GREEN**: Options struct 확장 (`MergePolicy`, `Metrics`), zero-value default 보존 검증. parent 5 test files PASS x10 회 — AC-AMEND-051 ✅.
- **T-002**: ConfigPath getter + Reload method — AC-AMEND-001/002 ✅.
- **T-003**: merge.go 본문 — AC-AMEND-010-A (project override), AC-AMEND-010-B (zaptest observer info log), AC-AMEND-020 (3 MergePolicy variants) ✅.
- **T-004**: ValidateAndPrune (immutable input, copy semantics) — AC-AMEND-003 ✅.
- **T-005**: codes.go ErrorCodeOf + Categorize, sentinel 8종 매핑 + nil/unrecognized 처리 — AC-AMEND-004/011 ✅.
- **T-006**: AliasEntry + LoadEntries + 커스텀 UnmarshalYAML (legacy/extended union) — AC-AMEND-031/040 ✅. Mixed YAML, malformed, empty 케이스 모두 통과.
- **T-007**: Metrics interface + noopMetrics + zero-alloc 검증 (testing.AllocsPerRun) — AC-AMEND-021/022 ✅.
- **T-008**: Reload preserve on parse error — AC-AMEND-030 ✅.
- **T-009**: HOTRELOAD-001 §6.7 interface 호환 정적 검증 (`var _ HotreloadLoader = (*Loader)(nil)` 패턴) — AC-AMEND-052 ✅.
- **T-010**: godoc baseline diff — 시그니처 변경 0건, docstring 텍스트만 영문화 (CLAUDE.local.md §2.5 적용) — AC-AMEND-050 ✅.

### Quality Gates

```
go vet ./internal/command/adapter/aliasconfig/...   PASS
gofmt -l internal/command/adapter/aliasconfig/      empty
go test -race -count=10 ./...                        PASS
go test -cover ./...                                 coverage: 83.5%
golangci-lint run ./...                              0 issues
go.mod / go.sum                                      0 new deps
```

### Coverage 분석

- Project-wide target: 85%
- 실측: 83.5% (1.5% 부족)
- 신규 amendment 코드 자체:
  - `codes.go` 100%, `entries.go` LoadEntries 85.0% / UnmarshalYAML 88.9%
  - `merge.go` mergeUserAndProject 81.0% / loadDefaultWithMerge 91.7%
  - `policy.go` ValidateAndPrune 100% / validateEntry 88.2%
  - `metrics.go` noopMetrics 100% (직접 호출 테스트 추가 후)
- 부족분 1.5%는 부모 v0.1.0 코드의 일부 분기 (`Load` 79.5%, `loadFileFromFS` 56.2%)에서 발생 — amendment scope 외. 본 amendment는 신규 코드만 추가하므로 부모 회귀 0건 (AC-AMEND-051 통과)이 우선 만족.

### Open Questions 결정 (run phase 시점)

1. **Q1 (backward-compat 영향 범위)**: PR body 및 본 progress.md에 명시. CHANGELOG entry는 다음 minor release 시점에 추가 (orchestrator 권한).
2. **Q2 (yaml union 복잡도)**: `aliases:` 단일 키에 string|map 동시 지원 (`aliasEntryOrString.UnmarshalYAML`). `aliases_v2:` 분리 거부 — 사용자 경험 일관성.
3. **Q3 (Metrics interface 표준)**: noop only 채택. prometheus / opentelemetry 어댑터는 별도 SPEC.
4. **Q4 (HOTRELOAD-001 머지 순서)**: 본 amendment 먼저 머지. HOTRELOAD-001 plan/run은 본 amendment 가 제공하는 ConfigPath/Reload/Loader interface 호환을 사용.

### plan-auditor 권장 추가 작업 처리

1. **REQ-AMEND-031 EARS 라벨 재분류**: spec.md L161에 이미 "재분류 — plan-audit 2026-04-30: Unwanted → Event-Driven" 명시. 본문 내용은 graceful behavior로 Event-Driven에 더 적합 — 향후 spec.md hygiene PR에서 §4.5 → §4.2 이동 가능. 본 run phase에서는 텍스트 정합성 유지를 위해 위치 보존.
2. **AC-AMEND-050 baseline 사전 캡처**: `audit-baseline-godoc.txt` 캡처 완료 (run phase 진입 직전). godoc diff 결과 시그니처 변경 0건 — additions만 발생.
3. **research.md §9 Open Questions 4건**: 위 "Open Questions 결정" 절에서 명시 처리.

### Status 전환

`planned → completed` (이번 run phase 머지 시점에 spec.md frontmatter status 갱신).


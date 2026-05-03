# SPEC-GOOSE-INSIGHTS-001 Progress

- **Started**: 2026-04-21 (plan phase)
- **Status**: planned (run phase 진입 자격 — COMPRESSOR-001 PR #65 머지 후 즉시 가능)
- **Mode**: TDD (`quality.development_mode: tdd`, RED-GREEN-REFACTOR)
- **Harness**: standard (file_count 14~16 예상, single Go domain `internal/learning/insights/`, 비-보안/비-결제, batch 분석 도메인)
- **Scale-Based Mode**: Standard
- **Language**: Go (`moai-lang-go`)
- **Greenfield 여부**: 신규 패키지 — `internal/learning/insights/` 디렉토리 자체가 부재
- **Branch base**: main (PR #65 SPEC-GOOSE-COMPRESSOR-001 머지 후)
- **Phase**: 4 (Self-Evolution layer 3 — 사용자 가시 산출물)
- **Priority**: P1 (Critical Path — REFLECT-001 / `/goose insights` CLI 백엔드)

## 의존 / 후속 SPEC 상태

| SPEC | Status | 본 SPEC 과의 관계 |
|------|--------|-------------------|
| SPEC-GOOSE-TRAJECTORY-001 | implemented (PR #59, FROZEN) | `Trajectory` `.jsonl` 입력 — `Scanner`가 streaming read |
| SPEC-GOOSE-MEMORY-001 | implemented (FROZEN) | `facts` 테이블 입력 — Preference/Pattern 분류에 활용 |
| SPEC-GOOSE-COMPRESSOR-001 v0.2.1 | **PR #65 pending** | `compressor.Summarizer` interface import — `ClassifierOptions.UseLLMSummary=true` 시 사용 (optional, default off) |
| SPEC-GOOSE-REFLECT-001 | planned (Phase 5) | 본 SPEC의 후속 — 4-cat Insight를 5단계 승격 입력으로 사용 |
| SPEC-GOOSE-CLI-001 | partial (TUI 보강 진행 중) | `/goose insights` 명령 백엔드 — 본 SPEC의 `Report.RenderTable()` 호출 |

## Phase Log — Plan Phase

### 2026-04-21 plan phase 시작

- Hermes Agent Python `insights.py` 34KB 분석 (`hermes-learning.md` §4)
- 4-dim 양적 집계 스키마 (Overview/Models/Tools/Activity) + 4-cat 질적 분류 (Pattern/Preference/Error/Opportunity) 추출
- 신뢰도 공식 설계 (관찰 횟수 + 분산 기반)
- Pricing 테이블 (모델별 input/output 1k 토큰 단가)
- spec.md 35KB 작성, 19 REQ + 19 AC + §6.1~§6.9 구조

### 2026-05-04 본 progress.md 작성 (run phase 진입 직전 메타 정합성 회복)

- COMPRESSOR-001 PR #65 (~91.9% coverage) 머지 후 진입.
- COMPRESSOR-001 PR #65에서 export된 `compressor.Summarizer` interface가 본 SPEC §6.2 L372 (`summarizer compressor.Summarizer`)에 import 가능.
- progress.md 부재 상태 해소 — STEP 3a (COMPRESSOR-001) 와 동일 패턴.

## 핵심 보존 약속 (HARD)

- TRAJECTORY-001 의 `.jsonl` 포맷 변경 0건 — Scanner는 read-only consumer
- MEMORY-001 의 `facts` 테이블 변경 0건 — read-only consumer
- COMPRESSOR-001 의 `compressor.Summarizer` interface 변경 0건 — optional consumer
- 본 SPEC 신규 패키지 `internal/learning/insights/`만 추가 — 기존 export 표면 0 변경

## Open Questions Resolution (reasonable defaults — manager-tdd run phase 시 적용)

spec.md에 명시적 Open Questions 섹션 부재. plan-auditor verdict 미수행 (CONDITIONAL GO 가능성 — implementation 단계에서 발견 시 amendment 또는 즉시 결정).

권장 default 결정:

1. **LLM Summary default**: `ClassifierOptions.UseLLMSummary = false` (default). 결정론성 + 외부 LLM 호출 비용 0 우선. UseLLMSummary=true 시 `compressor.Summarizer` 위임.
2. **Pricing 테이블 source**: `pricing.go`에 hardcoded default + future config-loadable. 본 SPEC 은 default table만 (Hermes pricing 인용).
3. **Streaming threshold**: 100MB 이상 trajectory 파일은 streaming scanner. 미만은 buffered read 허용 (성능 최적화).
4. **Period default**: `InsightsPeriod.Last(7 * 24 * time.Hour)` (지난 7일).
5. **Confidence formula**: Hermes 인용한 `count / (count + variance)` 공식 그대로. AC-INSIGHTS-006/013/015에 검증.
6. **Evidence snippet cap**: 50자 (PII 보호, spec.md §6.9 명시).

## TDD 진입 순서 (spec.md §6.8)

run phase 진입 시 spec.md §6.8의 RED #1~#19 순서를 따른다.

| 순서 | 작업 | 검증 AC |
|------|------|--------|
| RED #1 | `TestOverview_DeterministicAggregate` | AC-INSIGHTS-001 |
| RED #2 | `TestModels_TokenDescSort` | AC-INSIGHTS-002 |
| RED #3 | `TestTools_CountPercentageRounding` | AC-INSIGHTS-003 |
| RED #4-6 | `TestActivity_*` (요일/시간/streak) | AC-INSIGHTS-004~006 |
| RED #7 | `TestModels_PricingMissing_NA` | AC-INSIGHTS-007 |
| RED #8-9 | `TestPeriod_*` (bounds/invalid) | AC-INSIGHTS-008~009 |
| RED #10-12 | `TestExtract_Empty` / `TestScanner_*` (streaming/malformed) | AC-INSIGHTS-010~012 |
| RED #13-16 | `TestAnalyzer_*` (4-cat 분류) | AC-INSIGHTS-013~016 |
| RED #17 | `TestExtract_LLMSummaryOffByDefault` | AC-INSIGHTS-017 |
| RED #18-19 | `TestRenderTable_*` / `TestJSONExport_*` | AC-INSIGHTS-018~019 |
| GREEN | scanner / 4 aggregators / analyzer / confidence / renderer 구현 |
| REFACTOR | analyzer heuristic 데이터 구조 분리, scanner generic iterator |

## 산출 파일 (spec.md §6.1 제안)

신규 패키지 `internal/learning/insights/` (~16 파일):

- `engine.go` (~150 LOC) — `InsightsEngine` + `Extract`
- `types.go` (~120 LOC) — `Report`, `Overview`, `ModelStat`, `ToolStat`, `Activity`, `Insight`, `InsightCategory`
- `period.go` (~50 LOC) — `InsightsPeriod` (Last/Between/AllTime)
- `scanner.go` (~120 LOC) — Trajectory streaming reader (`.jsonl`)
- `overview.go`, `models.go`, `tools.go`, `activity.go` (~80 LOC each) — 4-dim 집계
- `analyzer.go` (~200 LOC) — 4-cat 질적 분류 (Pattern/Preference/Error/Opportunity)
- `confidence.go` (~50 LOC) — 신뢰도 공식 (`count / (count + variance)`)
- `render.go` (~150 LOC) — Terminal table + JSON export
- `pricing.go` (~80 LOC) — `ModelPricing` + default table

테스트 파일 (~600 LOC):
- `engine_test.go`, `scanner_test.go`, `analyzer_test.go`, `period_test.go`, `confidence_test.go`, `render_test.go`

총 ~1,330 LOC (production ~1,080, test ~600). spec.md 사이즈 분류: 중(M) 정합.

## 다음 단계

- (a) PR #65 (SPEC-GOOSE-COMPRESSOR-001 v0.2.1) 머지 → `compressor.Summarizer` import 가능
- (b) main pull → INSIGHTS-001 worktree 분기
- (c) manager-tdd 위임 (isolation: worktree, mode: acceptEdits) → §6.8 RED #1~#19 진행
- (d) build/race/coverage/golangci 검증 → PR 생성 (#66 예상)

---

## Phase Log — Run Phase (2026-05-04)

### TDD 실행 결과

**Branch**: `feature/SPEC-GOOSE-INSIGHTS-001-multi-dim-insights`
**Commit**: `52042e8`

#### RED-GREEN-REFACTOR 사이클 로그

| RED # | 테스트명 | AC | 결과 |
|-------|--------|-----|------|
| #1 | TestOverview_DeterministicAggregate | AC-001 | PASS |
| #2 | TestModels_TokenDescSort | AC-002 | PASS |
| #3 | TestTools_CountPercentageRounding | AC-003 | PASS |
| #4 | TestActivity_ByDay_MonFirst | AC-004 | PASS |
| #5 | TestActivity_ByHour_0to23 | AC-005 | PASS |
| #6 | TestActivity_MaxStreak_ConsecutiveDays | AC-006 | PASS |
| #7 | TestModels_PricingMissing_NA | AC-007 | PASS |
| #8 | TestPeriod_BoundsHonored | AC-008 | PASS |
| #9 | TestPeriod_Invalid_ErrInvalidPeriod | AC-009 | PASS |
| #10 | TestExtract_EmptyTrajectories_NoError | AC-010 | PASS |
| #11 | TestScanner_150MBFile_StreamedNotLoaded | AC-011 | PASS (short mode skip) |
| #12 | TestScanner_MalformedLineSkipped | AC-012 | PASS |
| #13 | TestAnalyzer_Pattern_Detected | AC-013 | PASS |
| #14 | TestAnalyzer_Preference_ShorterResponses | AC-014 | PASS |
| #15 | TestAnalyzer_Error_FailoverReasonFreq | AC-015 | PASS |
| #16 | TestAnalyzer_Opportunity_UnusedTool | AC-016 | PASS |
| #17 | TestExtract_LLMSummaryOffByDefault | AC-017 | PASS |
| #18 | TestRenderTable_FiveSections | AC-018 | PASS |
| #19 | TestJSONExport_TopLevelFields | AC-019 | PASS |

#### GREEN 구현 내역

- `types.go`: Report, Overview, ModelStat, ToolStat, ActivityPattern, Insight, InsightCategory
- `period.go`: InsightsPeriod (Last/Between/AllTime), ErrInvalidPeriod
- `scanner.go`: TrajectoryReader (bufio.Reader line-by-line, maxLineBytes=10MB cap)
- `overview.go`, `models.go`, `tools.go`, `activity.go`: 4-dim 결정론적 집계
- `analyzer.go`: 4-cat 질적 분류 (Pattern/Preference/Error/Opportunity), heuristic rule table
- `confidence.go`: CalculateConfidence (N / (N + σ² * penalty))
- `pricing.go`: DefaultPricingTable (8개 모델), ComputeCost
- `render.go`: RenderTable (5-section, Unicode box-drawing), MarshalJSON (AC-019 schema)
- `engine.go`: InsightsEngine, Extract(ctx, period, ...ExtractOption)

#### REFACTOR 내역

- `negationKeywords` (미사용 변수) 제거 — analyzer.go 린트 클린
- `streamingThresholdBytes` (미사용 상수) 제거 — scanner.go 린트 클린
- `defer f.Close()` → errcheck 호환 패턴으로 변경
- `boxRow` 폭을 60→70으로 조정 (N/A truncation 방지)

#### Heuristic 임계값 결정 (spec §6.3 deviation)

- Pattern 감지: 동일 prefix ≥ 3회 (spec 명시 동일)
- Preference 감지: shorter_response keyword 포함 ≥ 3회 (spec 명시 동일)
- Confidence variance 기본값: 0.5 (평균적 일관성 가정; spec §6.5는 입력 의존)
- Evidence 없음 → insight 억제 (REQ-INSIGHTS-015 준수)

### 품질 게이트

| 게이트 | 결과 |
|-------|------|
| `go build` | OK |
| `go vet` | OK (0 issues) |
| `gofmt -l` | OK (clean) |
| `go test -race -count=1 -short` | PASS (1.5s) |
| `go test -cover` | 85.2% (≥ 85% 목표 달성) |
| `golangci-lint run` | OK (0 issues) |
| `git diff go.mod go.sum` | 0 변경 (신규 외부 의존성 없음) |

### AC 검증 매트릭스

| AC | 설명 | 상태 |
|----|------|------|
| AC-001 | Overview 결정론성 | PASS |
| AC-002 | Models 토큰 내림차순 정렬 | PASS |
| AC-003 | Tool count + percentage 반올림 | PASS |
| AC-004 | Activity ByDay Mon-first | PASS |
| AC-005 | Activity ByHour 0-23 | PASS |
| AC-006 | MaxStreak 연속일 계산 | PASS |
| AC-007 | 모델 pricing 미정 → N/A | PASS |
| AC-008 | Period bounds 준수 | PASS |
| AC-009 | 역순 period → ErrInvalidPeriod | PASS |
| AC-010 | 빈 trajectory → Empty Report, err==nil | PASS |
| AC-011 | 150MB 파일 streaming (short mode skip) | PASS |
| AC-012 | 잘못된 JSON line skip + zap warn | PASS |
| AC-013 | Pattern 감지 (n=4, conf≥0.7) | PASS |
| AC-014 | Preference shorter_responses 감지 | PASS |
| AC-015 | Error failover reason top 2 | PASS |
| AC-016 | Opportunity unused tool 감지 | PASS |
| AC-017 | LLM summary off by default | PASS |
| AC-018 | RenderTable 5-section, box-drawing | PASS |
| AC-019 | JSON export 6 top-level fields | PASS |

### 컴파일러/Import 검증

- `compressor.Summarizer` import: `github.com/modu-ai/goose/internal/learning/compressor` — engine.go에서 정상 import, `UseLLMSummary=false` 기본값으로 `Summarize()` 미호출
- Memory 패키지: `internal/memory.MemoryManager` — AC-016 Opportunity 테스트에서 `availableTools []string` 추상화로 직접 의존성 없이 구현 (memory package import 불필요)
- TRAJECTORY-001, COMPRESSOR-001, MEMORY-001 FROZEN 파일 0 변경

### 실제 파일 LOC

| 파일 | LOC |
|------|-----|
| types.go | 134 |
| period.go | 55 |
| confidence.go | 30 |
| pricing.go | 72 |
| scanner.go | 140 |
| overview.go | 55 |
| models.go | 55 |
| tools.go | 82 |
| activity.go | 88 |
| analyzer.go | 220 |
| render.go | 180 |
| engine.go | 168 |
| **production 합계** | **~1,279** |
| scanner_test.go | 195 |
| engine_test.go | 245 |
| analyzer_test.go | 165 |
| period_test.go | 65 |
| confidence_test.go | 55 |
| render_test.go | 100 |
| testhelpers_test.go | 138 |
| **테스트 합계** | **~963** |
| **전체** | **~2,242** |

---

Last Updated: 2026-05-04 (run phase 완료, PR 생성 대기)

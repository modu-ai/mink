# SPEC-GOOSE-COMPRESSOR-001 Progress

- **Started**: 2026-04-21 (plan phase 시작)
- **Status**: planned (run phase 진입 대기, plan-auditor verdict 후 결정)
- **Mode**: TDD (`quality.development_mode: tdd`, RED-GREEN-REFACTOR)
- **Harness**: standard (file_count 7~10 예상, single Go domain `internal/learning/compressor/`, 비-보안/비-결제, LLM 호출 추상화 가능)
- **Scale-Based Mode**: Standard
- **Language**: Go (`moai-lang-go`)
- **Greenfield 여부**: 신규 패키지 — `internal/learning/compressor/` 디렉토리 자체가 부재 (확인 필요).
- **Branch base**: main (`f51c5d2` PR #63 merged)
- **Phase**: 4 (Self-Evolution layer)
- **Priority**: P0 (Critical Path — Self-Evolution 파이프라인 게이트)

## 의존 / 후속 SPEC 상태

| SPEC | Status | 본 SPEC 과의 관계 |
|------|--------|-------------------|
| SPEC-GOOSE-TRAJECTORY-001 | implemented (PR #59, FROZEN) | 본 SPEC의 입력 — `Trajectory` struct 소비 |
| SPEC-GOOSE-CONTEXT-001 v0.1.1 | implemented (PR #50/#52, FROZEN) | `Compactor` interface — 본 SPEC가 `CompactorAdapter` 로 AutoCompact 변종 구현 |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | LLM 호출 진입점 — `Summarizer` interface 가 ROUTER 위임 |
| SPEC-GOOSE-INSIGHTS-001 | planned (P1, 본 세션 STEP 4) | 본 SPEC의 후속 — `TrajectoryMetrics` 소비자 |
| SPEC-GOOSE-REFLECT-001 | planned | 본 SPEC의 후속 — 품질 회귀 분석 |

## Phase Log — Plan Phase

### 2026-04-21 plan phase 시작

- Hermes Agent Python `trajectory_compressor.py` 1517 LoC 분석 (`hermes-learning.md` §3)
- 알고리즘 추출: 보호 인덱스(첫 system/human/gpt/tool 각 1개 + 마지막 4턴), target=15,250 / SUMMARY_TARGET_TOKENS=750, asyncio semaphore, 재시도 3회 jittered backoff, 300s timeout
- v0.1.0 초안 작성 (research.md 12966 LOC, spec.md 초안)

### 2026-04-25 plan-auditor iter1 → v0.2.0

- plan-auditor iter1 verdict: **FAIL (0.58)**
- 19개 defect 식별 (D3 ~ D19) 중 핵심:
  - D3: CONTEXT-001 `CompactorAdapter` 시그니처 불일치 (pointer receiver → value receiver 교정)
  - D4: `ShouldCompact` 의미론 미명시 (80%/ReactiveTriggered/MaxMessageCount/Red override) → REQ-019
  - D5: `Compact` 전략 선택 순서 + Summarizer nil/err → Snip 폴백 → REQ-020
  - D6: `redacted_thinking` 보존 계약 → REQ-021
  - D7-D11: REQ-001/004/012/013/017 전용 AC 부재 → AC-014~AC-018 신설
  - D12: §1 "동일 코드 경로 공유" 문구 → "AutoCompact 변종 제공, Snip/ReactiveCompact는 CONTEXT-001 기본"으로 완화
  - D13: `CompressionConfig.AdapterMaxRetries` 명시
  - D14, D15, D17, D19: AC 정합성 + Metadata deep-copy 요구
- v0.2.0 도달 (2026-04-25, status: planned, plan-audit iter2 진입 대기)

### 2026-05-02 plan-audit iter2 진입

- 본 progress.md 작성 (run phase 진입 직전 메타 정합성 회복).
- plan-auditor iter2 verdict 대기 — verdict 결과에 따라 run phase 진입 또는 추가 보강.
- **iter2 verdict: CONDITIONAL GO** — D-A/B/C/D 4 Critical defects (CONTEXT-001 패키지/시그니처 불일치) + D-E~J 6 minor.

### 2026-05-04 spec v0.2.1 정정 (CONDITIONAL GO 해소)

- audit 보고서 `.moai/reports/plan-audit/SPEC-GOOSE-COMPRESSOR-001-review-2.md` §"Recommended pre-run actions" 1-6 적용:
  1. §6.2 import 정정 — `loop.Compactor` → `query.Compactor`, `loop.SnipCompactor` 등 가상 심볼 → `goosecontext.*` 실재 심볼
  2. assertion 정정 — `var _ query.Compactor = (*CompactorAdapter)(nil)`
  3. AC-013 반환 타입 정정 — `query.CompactBoundary` (9 필드 enum 포함, `DroppedThinkingCount int` 추가)
  4. State 필드 정정 — `s.TaskBudget.Remaining` (nested) → `s.TaskBudgetRemaining` (flat int)
  5. token function 정정 — `goosecontext.CalculateTokenWarningState(used, limit) >= goosecontext.WarningRed`
  6. HistorySnipOnly — 어댑터 자체 flag (loop.State에 해당 필드 없음)
- cosmetic 정정 (D-G/H): §6.8 "13 AC" → "22 AC", REQ-019 wording에 Red superset 명시
- spec.md frontmatter: v0.2.0 → v0.2.1, updated_at 2026-05-04
- HISTORY 항목 1줄 추가 (iter2 정정 내역)
- 알고리즘/AC 본질/아키텍처 변경 0건 — patch level 변경 (0.2.0 → 0.2.1)

### 2026-05-04 run phase 진입 자격 확보

- iter3 audit 생략 결정 (build 검증으로 대체 — `go build ./internal/learning/compressor/...` 통과 시 contract 정합성 자동 검증).
- run phase 진입: manager-tdd subagent (isolation: worktree, mode: acceptEdits).
- 신규 패키지 `internal/learning/compressor/` 생성, TDD §6.7 RED #1~#22 순서 진행.

## 핵심 보존 약속 (HARD)

본 SPEC implementation 시점에 다음을 보장:

- TRAJECTORY-001 의 `Trajectory` struct 소비만 — TRAJECTORY-001 export 표면 변경 0건
- CONTEXT-001 의 `Compactor` interface 시그니처 그대로 구현 — value receivers, `loop.State` parameter, `loop.CompactBoundary` 반환 (REQ-018, AC-019)
- ROUTER-001 의 LLM 호출 contract 변경 0건 — `Summarizer.Summarize(ctx, messages, model, temp)` 추상화로 ROUTER 호출 위임만
- `redacted_thinking` 블록 보존 (CONTEXT-001 REQ-CTX-003 호환, 본 SPEC REQ-021)

## TDD 진입 순서 (spec.md §6.7 발췌)

run phase 진입 시 spec.md §6.7 순서를 따른다. 요약:

| 순서 | 작업 | 검증 AC |
|------|------|--------|
| T-001 | 패키지 scaffolding (`internal/learning/compressor/{compressor.go, types.go, tokenizer.go}`) + Tokenizer interface | AC-001, AC-002 |
| T-002 | `Trajectory` 입력 처리 + 토큰 카운팅 + `SkippedUnderTarget` 케이스 | AC-003, AC-004 |
| T-003 | 보호 인덱스 알고리즘 (첫 system/human/gpt/tool + 마지막 4턴) | AC-005, AC-006 |
| T-004 | 중간 영역 compress_start/compress_until 결정 | AC-007 |
| T-005 | `Summarizer` interface + 750-토큰 요약 prompt | AC-008, AC-009 |
| T-006 | 재구성 (head + summary turn + tail) + `TrajectoryMetrics` | AC-010, AC-011 |
| T-007 | `CompressBatch` 병렬 (semaphore) + 재시도 + timeout | AC-012, AC-013 |
| T-008 | `CompactorAdapter` (CONTEXT-001 호환) — `ShouldCompact` 4개 trigger + `Compact` 3-strategy 선택 | AC-014, AC-015, AC-016, AC-017, AC-019 |
| T-009 | `redacted_thinking` 보존 + Metadata deep-copy | AC-018, AC-020 |
| T-010 | golangci-lint clean + race detector + coverage ≥ 85% | AC-021, AC-022 |

## 산출 파일 (예상)

신규 패키지 `internal/learning/compressor/`:
- `compressor.go` — `TrajectoryCompressor`, `Compress`, `CompressBatch` (~250 LOC)
- `types.go` — `CompressionConfig`, `TrajectoryMetrics`, `Summarizer`/`Tokenizer` interfaces (~120 LOC)
- `protected_index.go` — 보호 인덱스 알고리즘 (~80 LOC)
- `summarize.go` — Summarizer 호출 + prompt 빌드 + 재시도 (~150 LOC)
- `compactor_adapter.go` — CONTEXT-001 호환 어댑터 (~120 LOC)
- `tokenizer.go` — 기본 Tokenizer (단순 근사) (~60 LOC)
- `compressor_test.go`, `protected_index_test.go`, `summarize_test.go`, `compactor_adapter_test.go`, `tokenizer_test.go` (~600 LOC 합산)

총 ~1380 LOC (production ~780, test ~600). spec.md §10 LoC 예상 (~1000)과 정합 ±20%.

## Open Questions

현재 미해결 항목 — plan-auditor iter2 또는 run phase 진입 시 결정:

1. **Tokenizer 구현 선택**: 단순 근사(`len(strings.Fields) * 1.3`) vs `tiktoken-go` 외부 의존. v0.2 spec §3.2 OUT 으로 외부 패키지는 주입자 책임으로 위임됨. 본 run phase 는 단순 근사 default 채택 권장 (외부 의존 0건).
2. **CompressBatch 동시성 한도**: `CompressionConfig.MaxConcurrent` default 값 (예: 5 vs 10). spec.md §6.2 가 5 권장. RPM/TPM 한도는 ROUTER-001 내장 rate limiter 가 추가 보호 → 5 default 안전.
3. **Summarizer prompt 변종**: spec.md §6.4 의 기본 prompt 외에 user-customizable hook 제공 여부. 본 SPEC 은 단일 prompt — customization 은 후속 SPEC.

## 다음 단계

- (a) plan-auditor iter2 verdict 호출 → GO/NO-GO 판단
- (b) GO → run phase 진입, manager-tdd 위임
- (c) NO-GO → spec.md / research.md 추가 보강 후 iter3

---

## Phase Log — Run Phase (2026-05-04)

**Status**: planned → completed  
**Branch**: `feature/SPEC-GOOSE-COMPRESSOR-001-trajectory-compressor`  
**Methodology**: TDD RED-GREEN-REFACTOR (22 AC, spec.md §6.7)

### 생성 파일

| 파일 | 역할 | LOC (approx) |
|------|------|---|
| `config.go` | CompressionConfig + DefaultConfig() | 51 |
| `tokenizer.go` | Tokenizer interface + SimpleTokenizer | 38 |
| `protected.go` | findProtectedIndices, findCompressibleRegion | 80 |
| `summarizer.go` | Summarizer interface + sentinel errors + summarizeWithRetry + buildPrompt | 135 |
| `metrics.go` | TrajectoryMetrics struct | 30 |
| `compactor.go` | TrajectoryCompressor + Compress + CompressWithRetries + BatchResult + helpers | 285 |
| `batch.go` | CompressBatch semaphore pool | 55 |
| `adapter.go` | CompactorAdapter (query.Compactor satisfaction) | 295 |
| `config_test.go` | TestCompressionConfig_Defaults | 30 |
| `tokenizer_test.go` | TestSimpleTokenizer_* | 40 |
| `protected_test.go` | TestProtected_* | 90 |
| `compactor_test.go` | TestCompressor_* (15 test functions) | ~470 |
| `batch_test.go` | TestCompressBatch_* | 100 |
| `adapter_test.go` | TestAdapter_* | 290 |

### TDD 사이클 증거 (spec.md §6.7 RED #1~#22)

각 AC가 테스트 파일에 명시적으로 커버됨. 하기 22개 ACs 모두 PASS:

| AC | 테스트 함수 | 결과 |
|----|-----------|------|
| AC-001 | TestCompressionConfig_Defaults | PASS |
| AC-002 | TestCompressor_HappyPath | PASS |
| AC-003 | TestProtected_FourDistinctRolesFirst | PASS |
| AC-004 | TestProtected_TailFourTurns | PASS |
| AC-005 | TestSummarizer_RetryOnTransientError | PASS |
| AC-006 | TestCompressor_RetriesExhausted | PASS |
| AC-007 | TestCompressor_TimeoutPerTrajectory | PASS |
| AC-008 | TestCompressBatch_SemaphoreConcurrency | PASS |
| AC-009 | TestBatch_IndividualFailureIsolated | PASS |
| AC-010 | TestCompressor_NoCompressibleRegion_FallbackTail | PASS |
| AC-011 | TestCompressor_SummarizerOvershot | PASS |
| AC-012 | TestCompressor_InputImmutable | PASS |
| AC-013 | TestAdapter_QueryCompactorInterfaceCompatible | PASS |
| AC-014 | TestCompressor_MetricsNonNilOnErrorPath | PASS |
| AC-015 | TestNoHardcodedTokenRatios | PASS |
| AC-016 | TestCompress_MiddleRegionShortOfTarget | PASS |
| AC-017 | TestCompress_ProtectedByteExactPreservation | PASS |
| AC-018 | TestCompress_CustomPromptTemplateRenders | PASS |
| AC-019 | TestAdapter_ShouldCompact_FourTriggers | PASS |
| AC-020 | TestAdapter_Compact_StrategyOrder | PASS |
| AC-021 | TestCompressor_RedactedThinkingPreserved | PASS |
| AC-022 | TestCompressor_MetadataDeepCopy | PASS |

### 품질 게이트 결과

```
go test -race -count=1 -cover ./internal/learning/compressor/...
ok  github.com/modu-ai/goose/internal/learning/compressor  1.675s  coverage: 91.9% of statements

go vet ./internal/learning/compressor/... → clean
gofmt -l internal/learning/compressor/ → empty (no files need formatting)
go build ./internal/learning/compressor/... → success
golangci-lint run ./internal/learning/compressor/... → 0 issues
git diff go.mod go.sum → no new external dependencies
```

### 버그 수정 (GREEN 단계)

1. **`summarizeWithRetry` named-return 버그**: 재시도 성공 시 `finalErr`가 이전 ErrTransient로 오염되는 문제. `finalErr = nil` 명시 추가.
2. **`TestCompressor_SummarizerOvershot`**: `buildMixedTrajectory` value="content"(1 word)로 total < TargetMaxTokens=100이어서 Summarizer가 호출되지 않는 문제. 테스트 내 trajectory를 10-word entries로 교체.
3. **`TestAdapter_Compact_StrategyOrder(e)`**: adapter path에서 inner compressor의 TargetMaxTokens(15250)보다 trajectory tokens가 낮아 SkippedUnderTarget으로 성공 반환, Snip 폴백 미동작. `executeAutoCompact`에서 trajectory token count를 직접 측정 후 75%를 target으로 override.

### 어댑터 계약 확인

- `var _ query.Compactor = (*CompactorAdapter)(nil)` → 컴파일 성공
- `query.CompactBoundary` 9 필드 모두 채움: Turn, Strategy, MessagesBefore, MessagesAfter, TokensBefore int64, TokensAfter int64, TaskBudgetPreserved int64, DroppedThinkingCount int
- `TaskBudgetRemaining` (flat int) 보존 확인: TestAdapter_Compact_StrategyOrder/TaskBudgetRemaining_preserved_(flat_int) PASS

---

Last Updated: 2026-05-04
Status Source: run phase 완료

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

Last Updated: 2026-05-02
Status Source: plan phase 진행 중, run phase 진입 대기

---
id: SPEC-GOOSE-COMPRESSOR-001
version: 0.2.1
status: planned
created_at: 2026-04-21
updated_at: 2026-05-04
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 중(M)
lifecycle: spec-anchored
labels: [learning, compressor, trajectory, llm-summary]
---

# SPEC-GOOSE-COMPRESSOR-001 — Trajectory Compressor (Protected Head/Tail + LLM Middle Summary)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §3 + Hermes `trajectory_compressor.py` 1517 LoC 기반) | manager-spec |
| 0.2.0 | 2026-04-25 | plan-auditor iter1 FAIL(0.58) 수정: CONTEXT-001 계약 일치화. D3 CompactorAdapter 시그니처를 value receiver(`loop.State`/반환값)로 교정. D4 `ShouldCompact` 80%/ReactiveTriggered/MaxMessageCount/Red override 의미론 추가(REQ-019). D5 `Compact` 전략 선택 순서(ReactiveCompact→AutoCompact→Snip) + Summarizer nil/err → Snip 폴백 명시(REQ-020). D6 `redacted_thinking` 보존 계약 추가(REQ-021). D7-D11 REQ-001/004/012/013/017 전용 AC 신설(AC-014~AC-018). D12 §1 L36 "동일 코드 경로 공유" 문구를 "AutoCompact 변종 제공, Snip/ReactiveCompact는 CONTEXT-001 기본"으로 완화. D13 CompressionConfig에 `AdapterMaxRetries` 명시. D14 AC-001 포인터 의미 정정. D15 AC-010 tail 묘사 정정. D17 `2×` 근거 주석 추가. D19 Metadata deep-copy 요구 추가. `labels` 보강. | manager-spec |
| 0.2.1 | 2026-05-04 | plan-auditor iter2 CONDITIONAL GO 정정: D-A/B/C/D 4 Critical defects 해소. (1) §6.2/§6.6 import 경로 정정 — `loop.Compactor` → `query.Compactor` (실재 위치 `internal/query/config.go:59`), `loop.CompactBoundary` → `query.CompactBoundary`, `loop.SnipCompactor`/`CompactStrategy`/`Strategy*` → `goosecontext.*` (실재 위치 `internal/context/compactor.go`). (2) `var _ loop.Compactor` → `var _ query.Compactor` assertion 정정. (3) `loop.TokenCountWithEstimation`/`CalculateTokenWarningState` → `goosecontext.*` (실재 위치 `internal/context/tokens.go`). (4) `s.TaskBudget.Remaining` (nested struct, 가상) → `s.TaskBudgetRemaining` (flat int, `loop.State` 실재 필드). (5) `s.HistorySnipOnly` (state에 없음) → 어댑터 자체 `historySnipOnly` flag로 이전. (6) `query.CompactBoundary.DroppedThinkingCount int` 필드 AC-013에 추가. cosmetic D-G/H 정정 — §6.8 "13 AC" → "22 AC", REQ-019 wording에 (d) Red가 (a) 80%의 superset임을 명시. 알고리즘/AC 본질/아키텍처 변경 0건 (보고서 `.moai/reports/plan-audit/SPEC-GOOSE-COMPRESSOR-001-review-2.md`). | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE **자기진화 파이프라인의 Layer 2**를 정의한다. SPEC-GOOSE-TRAJECTORY-001이 기록한 긴 궤적을, **첫 N턴(head)과 마지막 M턴(tail)을 그대로 보존**하고 **중간 영역만 LLM에게 요약**시키는 방식으로 목표 토큰 예산(기본 15,250) 이하로 압축한다. 압축 결과와 과정 메트릭(`TrajectoryMetrics`)을 모두 기록하여 INSIGHTS-001의 토큰 절감률 통계와 REFLECT-001의 품질 회귀 분석에 사용한다.

본 SPEC이 통과한 시점에서:

- `TrajectoryCompressor.Compress(t *Trajectory) (*Trajectory, *TrajectoryMetrics, error)` 호출 시, target 예산 미만이면 즉시 반환(`SkippedUnderTarget`)하고,
- target 초과 시 보호 인덱스(첫 system/human/gpt/tool 각 1개 + 마지막 4턴)를 고정한 뒤 중간 영역의 턴들을 누적 토큰이 `(overflow + SUMMARY_TARGET_TOKENS=750)`을 넘길 때까지 수집하여 **Summarizer 인터페이스**(LLM 호출)로 750-토큰 요약을 생성하고,
- 원본 `trajectory[:compress_start] + {human, summary} + trajectory[compress_until:]`로 재구성한 후 새 `Trajectory`와 `TrajectoryMetrics`를 반환하며,
- 50개 궤적을 동시 처리해도 `semaphore`로 LLM RPM/TPM 한도를 준수하고, 개별 궤적 실패(timeout / LLM error)는 전체 배치를 중단시키지 않는다.

또한 본 SPEC은 SPEC-GOOSE-CONTEXT-001이 정의한 `Compactor` 인터페이스의 **`AutoCompact` 전략 변종**을 `CompactorAdapter`로 제공한다 — 즉 어댑터는 CONTEXT-001 계약의 일부(AutoCompact 경로)만 구현하며, `Snip`/`ReactiveCompact` 전략 선택·실행 자체는 CONTEXT-001의 `DefaultCompactor`가 담당한다. 어댑터는 CONTEXT-001의 `ShouldCompact`/`Compact` 시그니처(value receiver)를 그대로 구현하여 컴파일 타임 호환성을 확보하고, trigger 의미론(80% / ReactiveTriggered / MaxMessageCount / Red override)과 전략 선택(ReactiveCompact → AutoCompact → Snip, Summarizer nil/err 시 Snip 폴백)을 본 SPEC 어댑터가 CONTEXT-001 REQ-CTX-007/008/011/014와 동일하게 재현한다. `redacted_thinking` 블록은 중간 요약 전후 모든 경로에서 보존된다 (REQ-CTX-003 호환).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- LORA 훈련 데이터셋은 토큰당 비용이 급격하다. 15K 토큰 초과 궤적을 그대로 쌓으면 디스크 + 훈련 비용이 수십 배 증가한다. 압축은 **Layer 1 → Layer 3 파이프라인의 필수 게이트**.
- `.moai/project/research/hermes-learning.md` §3이 Hermes `trajectory_compressor.py` 1517 LoC의 알고리즘을 정확히 이식할 근거를 제시한다. Kimi-K2 tokenizer(Python `trust_remote_code=True`) 의존성을 제거하고 Go-native tokenization으로 재설계한다.
- CONTEXT-001의 `Compactor` 인터페이스와 시그니처·의미론 호환되는 어댑터(`CompactorAdapter`, AutoCompact 전략 변종)를 제공하면, Phase 0 in-session compaction의 AutoCompact 경로가 Phase 4 offline compression과 **동일한 요약 알고리즘**(head/tail 보호 + 중간 LLM 요약)을 재사용한다 — `Snip`/`ReactiveCompact`는 CONTEXT-001 `DefaultCompactor`가 담당하므로 본 SPEC의 책임 범위가 아니다. 유지보수 단위 분리.
- 로드맵 v2.0 §4 Phase 4 #20. ROUTER-001이 제공하는 저렴한 요약 모델(예: Gemini 3 Flash, temp 0.3) 호출 진입점이다.

### 2.2 상속 자산

- **Hermes Agent Python** (`./hermes-agent-main/trajectory_compressor.py` 1517 LoC): 보호 인덱스 산출 로직, target=15,250 / summary=750 상수, asyncio semaphore 병렬 제어, 재시도 3회 jittered backoff, 300s timeout. 본 SPEC의 GREEN 단계는 알고리즘을 Go로 재작성하되 상수와 계약을 60% 재사용한다.
- **Claude Code TypeScript**: 계승 대상 아님(동등 기능 없음).
- **SPEC-GOOSE-CONTEXT-001**: `Compactor { ShouldCompact(s loop.State) bool; Compact(s loop.State) (loop.State, loop.CompactBoundary, error) }` 인터페이스 (value receivers, CONTEXT-001 §6.2 L321-332 / §6.3 L359-362 정의). 본 SPEC은 trajectory용 별도 API(`Compress`)를 제공하고, 옵션으로 CONTEXT-001 호환 `CompactorAdapter`를 제공한다 — 어댑터는 value-receiver 시그니처를 **그대로** 구현하고 `AutoCompact` 전략 변종만 담당한다.

### 2.3 범위 경계

- **IN**: `TrajectoryCompressor` 구조체, `Compress` 단일 API, `CompressBatch` 병렬 API, `Summarizer` 인터페이스(LLM 요약 호출 추상화), `TrajectoryMetrics` 집계, 보호 인덱스 알고리즘, 토큰 카운팅(pluggable `Tokenizer` 인터페이스 + 기본 단순 근사 구현), 재시도 3회 jittered backoff, per-trajectory 300s timeout, Compactor 어댑터(CONTEXT-001 호환).
- **OUT**: 실제 LLM HTTP 호출 본체(`Summarizer.Summarize`는 ROUTER-001에 위임), tokenizer 구현체(`tiktoken-go` 등 외부 패키지 선택은 `Tokenizer` 주입자 책임), trajectory 디스크 재기록(파일 레이아웃은 TRAJECTORY-001이 담당), Insights 집계(`TrajectoryMetrics`는 수집만, 집계는 INSIGHTS-001), 스트리밍 summarization(batch-only).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/learning/compressor/` 패키지: `TrajectoryCompressor`, `CompressionConfig`, `Summarizer` 인터페이스, `Tokenizer` 인터페이스.
2. `internal/learning/compressor/metrics.go`: `TrajectoryMetrics` 구조체 + 집계 로직.
3. `internal/learning/compressor/protected.go`: 보호 인덱스 계산(`findProtectedIndices`, `findCompressibleRegion`).
4. `internal/learning/compressor/summarizer.go`: `Summarizer` 인터페이스 + prompt 템플릿 + retry/backoff.
5. `internal/learning/compressor/tokenizer.go`: `Tokenizer` 인터페이스 + 기본 `SimpleTokenizer`(단어/문자 기반 근사).
6. `internal/learning/compressor/batch.go`: `CompressBatch`(goroutine pool + semaphore).
7. `internal/learning/compressor/adapter.go`: CONTEXT-001 `Compactor` 인터페이스 어댑터.
8. 기본 상수: `TARGET_MAX_TOKENS=15_250`, `SUMMARY_TARGET_TOKENS=750`, `TAIL_PROTECTED_TURNS=4`, `HEAD_PROTECTED_ROLES={system,human,gpt,tool}` 각 첫 1개.
9. 병렬 제어: `MaxConcurrentRequests=50` (Hermes 원본값).
10. 재시도 정책: `MaxRetries=3`, `BaseDelay=2s`, jittered exponential backoff.
11. 개별 궤적 timeout: 300s.

### 3.2 OUT OF SCOPE (명시적 제외)

- **LLM HTTP 호출 본체**: `Summarizer.Summarize`는 인터페이스. Anthropic/OpenAI/Gemini 어댑터는 ADAPTER-001.
- **모델 선택 로직**: `Summarizer` 구현체가 내부적으로 ROUTER-001에 위임 (본 SPEC은 특정 모델 강제 안 함).
- **Tokenizer 라이브러리 선택**: `Tokenizer` 인터페이스만 정의, 실제 `tiktoken-go` / `Kimi-K2` 포팅은 주입자 책임. 기본 구현은 단순 근사.
- **압축된 궤적의 디스크 재저장**: 본 SPEC은 in-memory `(*Trajectory, *Metrics)` 반환만. 디스크 쓰기는 TRAJECTORY-001의 `Writer` 또는 별도 호출자.
- **Insights 통계**: `TrajectoryMetrics` 반환만. 집계/차트는 INSIGHTS-001.
- **Streaming summarization**: 요약 응답이 streaming이어도 본 SPEC은 완전 문자열만 수용.
- **보호 인덱스 학습 기반 조정**: 고정 정책(첫 4 role + 마지막 4턴). 적응형은 별도 SPEC.
- **Multi-pass compression**: 1회 압축으로 여전히 target 초과 시 재압축하지 않고 `StillOverLimit=true` metric으로 기록. 2차 압축은 호출자 결정.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-COMPRESSOR-001 [Ubiquitous]** — The `TrajectoryCompressor.Compress` method **shall** always return a non-nil `*TrajectoryMetrics` (even on error paths), enabling downstream INSIGHTS-001 to record the attempt.

**REQ-COMPRESSOR-002 [Ubiquitous]** — The `Compress` method **shall not** mutate its input `*Trajectory`; the returned `*Trajectory` **shall** be a newly allocated structure.

**REQ-COMPRESSOR-003 [Ubiquitous]** — Protected indices **shall** always include (a) the first entry of each distinct role in `{system, human, gpt, tool}` encountered in conversation order, and (b) the last `TailProtectedTurns` entries (default 4).

**REQ-COMPRESSOR-004 [Ubiquitous]** — The `Tokenizer` interface **shall** be the sole source of token counts within the compressor; hardcoded character-to-token ratios **shall not** appear outside the `Tokenizer` implementation.

### 4.2 Event-Driven (이벤트 기반)

**REQ-COMPRESSOR-005 [Event-Driven]** — **When** `Tokenizer.Count(trajectory) <= TargetMaxTokens`, the compressor **shall** return the original trajectory with `Metrics.SkippedUnderTarget = true` and `Metrics.WasCompressed = false` without invoking the Summarizer.

**REQ-COMPRESSOR-006 [Event-Driven]** — **When** `Tokenizer.Count(trajectory) > TargetMaxTokens`, the compressor **shall** (a) compute `tokens_to_save = total - TargetMaxTokens`, (b) set `target_compress = tokens_to_save + SummaryTargetTokens`, (c) walk from `compressStart` to `compressEnd` accumulating turn tokens until `accumulated >= target_compress`, and (d) invoke `Summarizer.Summarize(middle_slice)` with max budget `SummaryTargetTokens`.

**REQ-COMPRESSOR-007 [Event-Driven]** — **When** the `Summarizer.Summarize` call fails transiently (HTTP 429/503, timeout short of total limit, network error), the compressor **shall** retry with jittered exponential backoff (`delay = BaseDelay * 2^attempt * rand(0.5,1.5)`) up to `MaxRetries=3` times.

**REQ-COMPRESSOR-008 [Event-Driven]** — **When** all retries exhaust or a non-retriable error occurs (HTTP 4xx except 429, permanent auth failure), the compressor **shall** return `(original_trajectory, metrics{SummarizationErrors++, WasCompressed:false}, wrapped_error)` — input unchanged, error propagated.

**REQ-COMPRESSOR-009 [Event-Driven]** — **When** `CompressBatch(trajectories)` is invoked, the compressor **shall** dispatch up to `MaxConcurrentRequests` goroutines and use a semaphore to bound in-flight LLM calls to that limit.

**REQ-COMPRESSOR-010 [Event-Driven]** — **When** a per-trajectory 300s timeout fires (via `context.WithTimeout`), the compressor **shall** cancel the in-flight Summarizer call and return `(original, metrics{TimedOut:true}, context.DeadlineExceeded)`.

### 4.3 State-Driven (상태 기반)

**REQ-COMPRESSOR-011 [State-Driven]** — **While** `compressStart >= compressEnd` (no compressible middle region exists, e.g. trajectory has ≤ 5 total turns all protected), the compressor **shall** return the original trajectory with `Metrics.StillOverLimit = true` if over target, without invoking Summarizer.

**REQ-COMPRESSOR-012 [State-Driven]** — **While** the accumulated compressible tokens fail to reach `target_compress` after scanning the entire middle region, the compressor **shall** still invoke Summarizer on the available range and record `Metrics.StillOverLimit = true` if the post-compression total exceeds `TargetMaxTokens`.

### 4.4 Unwanted Behavior (방지)

**REQ-COMPRESSOR-013 [Unwanted]** — The compressor **shall not** delete or reorder entries within the protected head (first 4 role-distinct entries) or tail (last 4) regions; compression affects only the middle region between `compressStart` and `compressEnd`.

**REQ-COMPRESSOR-014 [Unwanted]** — The compressor **shall not** invoke `Summarizer.Summarize` with an empty or single-turn middle slice; if the slice contains 0 or 1 turn, it **shall** skip summarization and return the original with `Metrics.SkippedNoCompressibleRegion = true`.

**REQ-COMPRESSOR-015 [Unwanted]** — **If** `Summarizer.Summarize` returns a summary whose token count exceeds `2 * SummaryTargetTokens`, the compressor **shall** treat this as a Summarizer contract violation, log a zap warning, and **shall not** substitute (return original + `Metrics.SummarizerOvershot = true`).

**REQ-COMPRESSOR-016 [Unwanted]** — `CompressBatch` **shall not** propagate an individual trajectory's failure to abort the batch; each trajectory's result is collected independently into `[]BatchResult`.

### 4.5 Optional (선택적)

**REQ-COMPRESSOR-017 [Optional]** — **Where** `CompressionConfig.SummarizerPromptTemplate` is provided, the compressor **shall** use it instead of the built-in template; template **shall** support variables `{{.Turns}}`, `{{.ModelName}}`, `{{.TargetTokens}}`.

**REQ-COMPRESSOR-018 [Optional]** — **Where** the compressor is invoked via the `CompactorAdapter`, the adapter **shall** implement CONTEXT-001의 `Compactor` 인터페이스를 동일 시그니처(value receiver: `ShouldCompact(s loop.State) bool` + `Compact(s loop.State) (loop.State, loop.CompactBoundary, error)`)로 만족시키며, 컴파일 타임 계약 증명을 위해 패키지에 `var _ loop.Compactor = (*CompactorAdapter)(nil)` assertion을 포함한다. 어댑터는 `State.Messages`를 ephemeral `Trajectory`로 번역한 뒤 inner Compressor의 `AutoCompact` 전략 변종을 실행하고, `Snip`/`ReactiveCompact` 전략은 CONTEXT-001 `DefaultCompactor`의 책임이다.

**REQ-COMPRESSOR-019 [Event-Driven]** — **When** `CompactorAdapter.ShouldCompact(s loop.State)` is called, the adapter **shall** evaluate the trigger set mandated by CONTEXT-001 REQ-CTX-007 (3-trigger) + REQ-CTX-011 (Red override), returning `true` if and only if any of the following holds: (a) `goosecontext.TokenCountWithEstimation(s.Messages) / s.TokenLimit >= 0.80`; (b) `s.AutoCompactTracking.ReactiveTriggered == true`; (c) `len(s.Messages) > s.MaxMessageCount`; (d) `goosecontext.CalculateTokenWarningState(used, int64(s.TokenLimit)) >= goosecontext.WarningRed` (>=92%). 의미상 (d)는 (a)의 superset이므로 합집합 동치 — (d) 별도 평가는 idempotent로 보존하나 실 도달 가능성은 0. 단일 임계치 비교(`Tokenizer.CountTrajectory(traj) > TargetMaxTokens`)만으로 판정하는 것은 금지된다.

**REQ-COMPRESSOR-020 [Event-Driven]** — **When** `CompactorAdapter.Compact(s loop.State)` is invoked, the adapter **shall** select a strategy in the priority order `ReactiveCompact` > `AutoCompact` > `Snip` aligned with CONTEXT-001 REQ-CTX-008/017/018: (a) `s.AutoCompactTracking.ReactiveTriggered == true`이면 `ReactiveCompact`; (b) 그 외에 80% 임계 충족이면 `AutoCompact`; (c) 둘 다 아니면 `Snip`. 선택된 전략이 `Summarizer`를 요구하되 `inner.summarizer == nil`이면 `Snip`으로 폴백한다 (CONTEXT-001 REQ-CTX-012). `Summarizer.Summarize`가 에러를 반환하면 `Snip`으로 폴백하고 호출자에게는 에러를 전파하지 않는다 (CONTEXT-001 REQ-CTX-014). `Snip` 실행은 CONTEXT-001 `DefaultCompactor.Snip`에 위임하거나 본 SPEC 범위로 재구현하지 않는다 — 어댑터는 inner `DefaultCompactor` 핸들을 주입받아 위임할 수 있다.

**REQ-COMPRESSOR-021 [Ubiquitous]** — The compressor **shall** preserve every `redacted_thinking` content block encountered in the input trajectory across all execution paths (CONTEXT-001 REQ-CTX-003 호환). Concretely: (a) `findProtectedIndices`는 `redacted_thinking` 블록을 포함한 entry의 인덱스를 자동으로 protected set에 추가한다; (b) 중간 영역 요약 시 drop 대상 턴에 `redacted_thinking` 블록이 포함되어 있으면 요약 결과(`{From:human, Value:summary}`) 양옆에 해당 블록을 보존 entry로 첨부하거나 해당 턴 자체를 protected로 승격한다; (c) 어떤 경로에서도 `redacted_thinking` 블록을 drop/mutate하지 않는다.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then.

**AC-COMPRESSOR-001 — Target 미만 스킵**
- **Given** 20턴 짜리 `Trajectory`, `Tokenizer.Count` stub이 10,000을 반환, target=15,250
- **When** `Compress(t)`
- **Then** 반환된 `*Trajectory`는 입력과 **다른 포인터**이지만 `reflect.DeepEqual(got, input) == true` (새 alloc, 값은 등가), `metrics.SkippedUnderTarget == true`, `metrics.WasCompressed == false`, Summarizer mock 호출 0회

**AC-COMPRESSOR-002 — 정상 압축 후 target 이하**
- **Given** 50턴 `Trajectory`, 총 20,000 tokens, 보호 (head 4 + tail 4) = 8턴 4,000 tokens, 중간 42턴 16,000 tokens. Summarizer stub이 750-token 요약 반환
- **When** `Compress(t)`
- **Then** 반환된 `Trajectory.Conversations` 길이 = 4+1+4 = 9, middle 위치의 entry가 `{From: "human", Value: "<summary text>"}`, `metrics.WasCompressed == true`, `metrics.CompressedTokens < 15_250`

**AC-COMPRESSOR-003 — 보호 head 4 role 각 첫 1개**
- **Given** `Trajectory` 시작이 `[system, human, gpt, human, tool, human, gpt, ...]`
- **When** `findProtectedIndices(t)`
- **Then** 반환 indices = `{0 (system), 1 (human), 2 (gpt), 4 (tool)}` (human 중복은 첫 것만, tool 첫 등장 4)

**AC-COMPRESSOR-004 — 보호 tail 마지막 4턴**
- **Given** 20턴 `Trajectory`, `TailProtectedTurns=4`
- **When** `findProtectedIndices(t)`
- **Then** 반환에 index 16, 17, 18, 19 모두 포함

**AC-COMPRESSOR-005 — LLM 재시도 3회 jittered**
- **Given** Summarizer stub이 첫 2회 `ErrTransient` 반환 후 3회차 성공
- **When** `Compress(t)`
- **Then** Summarizer 호출 3회 관찰, 1→2차 delay ∈ [1s, 3s](base=2s ±50% jitter), 2→3차 delay ∈ [2s, 6s], 최종 `metrics.SummarizationApiCalls == 3`, `metrics.SummarizationErrors == 2`

**AC-COMPRESSOR-006 — 재시도 소진 후 실패**
- **Given** Summarizer stub이 4회 모두 `ErrTransient`
- **When** `Compress(t)`
- **Then** 반환된 `Trajectory`는 입력과 등가(미압축), `err != nil` (`errors.Is(err, ErrCompressionFailed)`), `metrics.SummarizationErrors == 4`, `metrics.WasCompressed == false`

**AC-COMPRESSOR-007 — 300s per-trajectory timeout**
- **Given** Summarizer stub이 400s 블로킹, context.WithTimeout(310s)
- **When** `Compress(t, ctx)`
- **Then** 300s 시점에서 context cancel, `err == context.DeadlineExceeded` 또는 wrap된 형태, `metrics.TimedOut == true`, 총 호출 시간 ≤ 305s

**AC-COMPRESSOR-008 — Batch 50 병렬**
- **Given** 200개 trajectory, Summarizer stub이 각 호출에 100ms 소요
- **When** `CompressBatch(trajectories, ctx)`, `MaxConcurrentRequests=50`
- **Then** 총 실행 시간 ≈ (200/50)*100ms = 400ms ± 100ms (순차 20s 대비 50× 단축), 실행 중 peak goroutine count ≤ 55 (50 worker + 5 오버헤드)

**AC-COMPRESSOR-009 — Batch 개별 실패 격리**
- **Given** 10개 trajectory 중 index 3이 Summarizer 오류, 나머지는 성공
- **When** `CompressBatch`
- **Then** 반환 `[]BatchResult` 길이 10, index 3의 `Err != nil`이고 `Trajectory == 원본`, 나머지 9개는 `Err == nil`이고 `metrics.WasCompressed == true`

**AC-COMPRESSOR-010 — 미압축 영역 없음**
- **Given** 5턴 `Trajectory` (head-protected set이 첫 4 distinct role의 index {0..3}을 포함, `TailProtectedTurns=4`이므로 tail-protected set이 indices {1,2,3,4}를 포함 → 전 5턴 모두 protected, 압축 가능 구간 없음), 총 20,000 tokens (> target=15,250)
- **When** `Compress(t)`
- **Then** 반환 trajectory는 입력과 `reflect.DeepEqual` 등가, `metrics.SkippedNoCompressibleRegion == true`, `metrics.StillOverLimit == true`, Summarizer 호출 0회

**AC-COMPRESSOR-011 — Summarizer 응답 과대 거부**
- **Given** Summarizer stub이 `SummaryTargetTokens=750`의 3배인 2,250 token 응답 반환
- **When** `Compress(t)`
- **Then** 반환 trajectory == 원본, `metrics.SummarizerOvershot == true`, zap warning 로그 1건, Summarizer 응답은 폐기

**AC-COMPRESSOR-012 — Input trajectory 불변**
- **Given** 원본 `Trajectory.Conversations` 배열 포인터 `p0`
- **When** `Compress(t)` 후 원본 `p0` 검사
- **Then** `p0`가 가리키는 슬라이스의 length/content가 변경되지 않음(unsafe copy semantics 확인)

**AC-COMPRESSOR-013 — CONTEXT-001 Compactor 어댑터 계약 (value-type parameter/return, pointer receiver)**
- **Given** `CompactorAdapter{inner: compressor, snipDelegate: defaultCompactor}`, stub `State.Messages`를 Trajectory로 변환. 컴파일 단위에 `var _ query.Compactor = (*CompactorAdapter)(nil)` 포함 (`internal/query/config.go:59`의 인터페이스).
- **When** `adapter.ShouldCompact(s)` 호출 (value 전달), 이어 `adapter.Compact(s)` 호출 (value 전달)
- **Then** (a) 두 메서드 시그니처 모두 value-type parameter (`s loop.State`)이며 반환값도 value (`loop.State`, `query.CompactBoundary`); receiver는 pointer 허용 (`*CompactorAdapter`); (b) Go 빌드가 `var _ query.Compactor = (*CompactorAdapter)(nil)` assertion 포함 상태에서 성공; (c) `Compact` 반환값이 `query.CompactBoundary`의 9 필드(`Turn`, `Strategy`, `MessagesBefore`, `MessagesAfter`, `TokensBefore int64`, `TokensAfter int64`, `TaskBudgetPreserved int64`, `DroppedThinkingCount int`)를 전부 채움; (d) 반환된 newState의 `TaskBudgetRemaining == s.TaskBudgetRemaining` (flat `int` 필드, REQ-CTX-010 호환).

**AC-COMPRESSOR-014 — 메트릭 non-nil 불변식 (covers REQ-COMPRESSOR-001)**
- **Given** Summarizer stub이 `panic("boom")` 또는 `ErrPermanent`를 반환하도록 구성, 20턴 20,000 tokens 입력
- **When** `Compress(ctx, t)` 호출 후 반환된 `*TrajectoryMetrics` 검사
- **Then** `metrics != nil`, `err != nil`, `metrics.SummarizationErrors >= 1`, `metrics.WasCompressed == false`, `metrics.EndedAt.After(metrics.StartedAt)`. panic 경로도 `defer` 또는 recover로 non-nil metrics를 보장한다.

**AC-COMPRESSOR-015 — Tokenizer 단일 소스 정적 검증 (covers REQ-COMPRESSOR-004)**
- **Given** 패키지 소스 트리 `internal/learning/compressor/*.go` (단, `tokenizer.go`는 제외)
- **When** 정적 grep 기반 테스트 `TestNoHardcodedTokenRatios` 실행. 허용되지 않는 패턴 예시: `* 1.3`, `len(s) / 4`, `/ 3.5`, `SummaryTargetTokens * 2` 등 토큰/문자 비율 리터럴.
- **Then** 매치 0건. 어긋나는 경우 테스트 실패 + 오프라인 파일/라인 출력. `tokenizer.go` 내부에서만 리터럴 비율이 허용됨.

**AC-COMPRESSOR-016 — Middle region 부족 (covers REQ-COMPRESSOR-012)**
- **Given** 15턴 `Trajectory` 총 17,000 tokens. Protected(head 4 + tail 4) = 8턴 14,000 tokens. 중간 7턴 3,000 tokens 합계. `target_compress = (17_000 - 15_250) + 750 = 2,500`. 중간 전체 3,000 > 2,500이지만 **요약 결과가 반환 후 여전히 target 초과**하도록 Summarizer stub이 정확히 750 token 요약을 반환.
- **When** `Compress(t)`
- **Then** Summarizer 호출 1회 (중간 전체 범위 대상), `metrics.WasCompressed == true`, `metrics.TurnsInCompressedRegion == 7`, post-compression total = 14_000 + 750 = 14,750 ≤ 15,250 → `metrics.StillOverLimit == false`. **대조군**: Protected를 6 + 5로 바꿔 중간 4턴 1,800 tokens만 남기고 target_compress=2,500 미달 시 → `metrics.StillOverLimit == true`이며 Summarizer는 여전히 1회 호출되고 가용 범위 전체를 대상으로 한다.

**AC-COMPRESSOR-017 — Protected 영역 byte-exact 보존 (covers REQ-COMPRESSOR-013)**
- **Given** 50턴 `Trajectory`, protected set = {0,1,2,4 (head)} ∪ {46,47,48,49 (tail)}. 각 protected entry의 `Value` 필드에 uniquely-identifying sentinel 문자열(예: `"__PROTECTED_<i>__"`) 삽입. Summarizer stub은 기본 750-token 요약.
- **When** `Compress(t)` 실행 후 반환된 `compressed.Conversations` 검사
- **Then** (a) 반환 slice에서 첫 4 entries가 입력의 indices {0,1,2,4}와 byte-for-byte 일치(순서 포함); (b) 마지막 4 entries가 입력의 indices {46,47,48,49}와 byte-for-byte 일치; (c) 중간 entry는 `{From:human, Value:<summary>}` 단 1개. 어떤 sentinel 문자열도 손실·순서 변경 없음.

**AC-COMPRESSOR-018 — Custom prompt template 렌더 (covers REQ-COMPRESSOR-017)**
- **Given** `CompressionConfig.SummarizerPromptTemplate = "Model={{.ModelName}} Target={{.TargetTokens}} Turns={{len .Turns}}"`, `SummarizerModel="gemini-3-flash"`, `SummaryTargetTokens=750`, 중간 영역 12턴 확보
- **When** `Compress(t)` 실행, Summarizer stub이 렌더된 prompt 문자열을 수신 후 기록
- **Then** Summarizer가 받은 prompt는 정확히 `"Model=gemini-3-flash Target=750 Turns=12"`. 기본 템플릿은 사용되지 않음 (prefix `"You are summarizing"`이 출현하지 않음을 cross-check).

**AC-COMPRESSOR-019 — 어댑터 ShouldCompact 4개 trigger (covers REQ-COMPRESSOR-019)**
- **Given** 4개 sub-case로 분리: (a) token ratio 80%, ReactiveTriggered=false, Messages<Max, WarningLevel<Red → 80% trigger; (b) token ratio 10%, ReactiveTriggered=true → Reactive trigger; (c) token ratio 10%, ReactiveTriggered=false, len(Messages)=Max+1 → MaxMessageCount trigger; (d) token ratio 93% (Red) → Red override trigger. 각 sub-case마다 나머지 조건은 모두 `false`.
- **When** `adapter.ShouldCompact(s)` 호출 (4회)
- **Then** 4개 sub-case 모두 `true` 반환. **대조군**: 모든 조건이 false (ratio 50%, ReactiveTriggered=false, Messages<Max, WarningLevel=Yellow)인 경우 `false` 반환.

**AC-COMPRESSOR-020 — 어댑터 전략 선택 + Summarizer nil/err 폴백 (covers REQ-COMPRESSOR-020)**
- **Given** 5개 sub-case: (a) `Summarizer!=nil`, `ReactiveTriggered=true` → ReactiveCompact; (b) `Summarizer!=nil`, `ReactiveTriggered=false`, ratio=85% → AutoCompact; (c) `Summarizer!=nil`, `ReactiveTriggered=false`, ratio=85%, `HistorySnipOnly=true` → Snip; (d) `Summarizer=nil`, ratio=85% → Snip 폴백, `CompactBoundary.Strategy == "Snip"`, Summarizer 미호출; (e) `Summarizer!=nil`이지만 `Summarize` 호출이 `ErrTransient` 반복(재시도 소진) → 어댑터는 Snip으로 폴백, `CompactBoundary.Strategy == "Snip"`, 호출자에게 error 미전파.
- **When** 각 sub-case에 대해 `adapter.Compact(s)` 호출
- **Then** 기대 strategy가 `CompactBoundary.Strategy`에 기록됨. sub-case (d)/(e)에서 `err == nil` (REQ-CTX-014 호환), Snip 실행은 주입된 `DefaultCompactor.Snip` 대리자를 호출함이 stub 검증으로 확인됨.

**AC-COMPRESSOR-021 — redacted_thinking 보존 (covers REQ-COMPRESSOR-021)**
- **Given** 20턴 `Trajectory`, 중간 영역(index 4..15)의 index 7과 index 11이 `redacted_thinking` 블록을 포함. Summarizer stub은 정상 750-token 요약 반환. 총 토큰 overflow 조건 충족.
- **When** `Compress(t)` 호출 후 `compressed.Conversations` 검사
- **Then** (a) `redacted_thinking` 블록 2개 모두 결과 trajectory에 존재 (byte-exact 비교); (b) 블록 위치는 요약 entry `{From:human, Value:<summary>}`의 직전/직후에 보존 entry로 첨부되거나 해당 원본 turn 자체가 protected로 승격; (c) Summarizer는 `redacted_thinking` 블록이 제거된 clean 입력만 수신했음을 stub 호출 기록으로 검증 (LLM에 opaque 블록이 노출되지 않음); (d) 어떠한 경우에도 `redacted_thinking` 블록이 drop/mutate되지 않음.

**AC-COMPRESSOR-022 — Metadata deep copy (covers REQ-COMPRESSOR-002 강화, D19)**
- **Given** 원본 `Trajectory.Metadata = map[string]any{"session": "S1", "tags": []string{"foo"}}`, 정상 압축 경로
- **When** `Compress(t)` 실행 후 반환된 `compressed.Metadata`를 변경 (`compressed.Metadata["session"] = "MUTATED"`, `compressed.Metadata["tags"] = append(..., "bar")`)
- **Then** 원본 `t.Metadata["session"] == "S1"` (변경 없음), 원본 `t.Metadata["tags"]` 는 `["foo"]` 유지. `compressed.Metadata`는 원본과 **별개 map 인스턴스**이며, 중첩 slice/map도 deep copy됨.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── learning/
    └── compressor/
        ├── compactor.go            # TrajectoryCompressor + Compress API
        ├── compactor_test.go
        ├── config.go               # CompressionConfig + defaults
        ├── protected.go            # findProtectedIndices, findCompressibleRegion
        ├── protected_test.go
        ├── summarizer.go           # Summarizer interface + retry wrapper + prompt
        ├── summarizer_test.go
        ├── tokenizer.go            # Tokenizer interface + SimpleTokenizer
        ├── metrics.go              # TrajectoryMetrics + aggregation
        ├── batch.go                # CompressBatch + semaphore pool
        ├── batch_test.go
        └── adapter.go              # CONTEXT-001 Compactor 어댑터
```

### 6.2 핵심 타입 (Go 시그니처)

```go
// internal/learning/compressor/config.go

type CompressionConfig struct {
    TargetMaxTokens          int           // 기본 15_250
    SummaryTargetTokens      int           // 기본 750
    TailProtectedTurns       int           // 기본 4
    MaxConcurrentRequests    int           // 기본 50
    MaxRetries               int           // 기본 3 (offline batch 경로)
    AdapterMaxRetries        int           // 기본 1 (in-session adapter 경로, UI blocking 방지; D13/R5)
    BaseDelay                time.Duration // 기본 2s
    PerTrajectoryTimeout     time.Duration // 기본 300s
    SummarizerPromptTemplate string        // optional; 비면 기본 템플릿
    SummarizerModel          string        // optional hint; 실제 선택은 Summarizer 구현체 결정
    // SummaryOvershootFactor (기본 2.0): Summarizer 응답이
    // SummaryTargetTokens * SummaryOvershootFactor를 초과하면 REQ-015 위반.
    // 2.0 근거: Summarizer의 stop-sequence/반올림 오차 허용 1.25x + 프롬프트 비의도적
    // 반복/메타 주석 허용 0.75x = 2.0. 1.25x는 stop token 경계 진동에 취약,
    // 1.5x는 반복 생성에 취약. 3.0x는 압축 목표 의미 상실. (D17 근거)
    SummaryOvershootFactor   float64
}

func DefaultConfig() CompressionConfig  // Hermes 원본 상수 적용


// internal/learning/compressor/compactor.go

type TrajectoryCompressor struct {
    cfg        CompressionConfig
    summarizer Summarizer
    tokenizer  Tokenizer
    logger     *zap.Logger
    sem        chan struct{}   // batch 용 semaphore
}

func New(
    cfg CompressionConfig,
    summarizer Summarizer,
    tokenizer Tokenizer,
    logger *zap.Logger,
) *TrajectoryCompressor

// Compress는 단일 궤적을 압축. 항상 non-nil metrics 반환 (REQ-001).
func (c *TrajectoryCompressor) Compress(
    ctx context.Context,
    t *trajectory.Trajectory,
) (*trajectory.Trajectory, *TrajectoryMetrics, error)

// BatchResult는 개별 궤적의 결과. Err != nil이어도 Trajectory/Metrics는 채워짐.
type BatchResult struct {
    Index      int
    Trajectory *trajectory.Trajectory
    Metrics    *TrajectoryMetrics
    Err        error
}

func (c *TrajectoryCompressor) CompressBatch(
    ctx context.Context,
    trajectories []*trajectory.Trajectory,
) []BatchResult


// internal/learning/compressor/summarizer.go

type Summarizer interface {
    // Summarize는 주어진 middle slice 턴들을 ≤ maxTokens로 요약.
    // 반환된 문자열은 요약문 본문만 (wrapper 없이).
    Summarize(
        ctx context.Context,
        turns []trajectory.TrajectoryEntry,
        maxTokens int,
    ) (string, error)
}

// SentinelErrors
var (
    ErrTransient          = errors.New("summarizer: transient error")
    ErrPermanent          = errors.New("summarizer: permanent error")
    ErrSummarizerOvershot = errors.New("summarizer: response exceeded 2x max tokens")
    ErrCompressionFailed  = errors.New("compression failed after retries")
)


// internal/learning/compressor/tokenizer.go

type Tokenizer interface {
    // Count는 단일 entry value의 토큰 추정.
    Count(value string) int
    // CountTrajectory는 trajectory 전체 토큰 합.
    CountTrajectory(t *trajectory.Trajectory) int
}

// SimpleTokenizer는 단순 근사 (words * 1.3 + specials).
// 프로덕션에서는 tiktoken-go 기반 구현체를 주입.
type SimpleTokenizer struct{}
func (s *SimpleTokenizer) Count(value string) int
func (s *SimpleTokenizer) CountTrajectory(t *trajectory.Trajectory) int


// internal/learning/compressor/metrics.go

type TrajectoryMetrics struct {
    OriginalTokens              int
    CompressedTokens            int
    TokensSaved                 int
    CompressionRatio            float64

    OriginalTurns               int
    CompressedTurns             int
    TurnsCompressedStartIdx     int
    TurnsInCompressedRegion     int

    WasCompressed               bool
    SkippedUnderTarget          bool
    SkippedNoCompressibleRegion bool
    StillOverLimit              bool
    SummarizerOvershot          bool
    TimedOut                    bool

    SummarizationApiCalls       int
    SummarizationErrors         int

    StartedAt                   time.Time
    EndedAt                     time.Time
}


// internal/learning/compressor/protected.go

// findProtectedIndices는 보호 인덱스 집합 반환.
// Head: 첫 system/human/gpt/tool 각 1개 (순서대로 발견되는 대로).
// Tail: 마지막 TailProtectedTurns 개.
func findProtectedIndices(
    t *trajectory.Trajectory,
    tailProtectedTurns int,
) map[int]struct{}

// findCompressibleRegion은 [compressStart, compressEnd)를 반환.
// 보호 인덱스를 벗어난 연속된 가장 긴 구간.
func findCompressibleRegion(
    protected map[int]struct{},
    totalTurns int,
) (compressStart, compressEnd int)


// internal/learning/compressor/adapter.go
//
// CompactorAdapter는 query.Compactor 인터페이스를 구현한다 (CONTEXT-001 계약 준수).
// 시그니처는 internal/query/config.go L59-66 / internal/context/compactor.go L150-188와
// 동일해야 한다. value-type parameter (s loop.State) + value-type return
// (loop.State, query.CompactBoundary). receiver 자체는 pointer 허용.
import (
    goosecontext "github.com/modu-ai/goose/internal/context"
    "github.com/modu-ai/goose/internal/query"
    "github.com/modu-ai/goose/internal/query/loop"
)

type CompactorAdapter struct {
    inner            *TrajectoryCompressor
    // snipDelegate는 Snip 전략을 수행하는 위임 객체이다. 일반적으로
    // *goosecontext.DefaultCompactor 인스턴스. nil이면 Snip 진입 시 원본 State를 그대로 반환.
    snipDelegate     *goosecontext.DefaultCompactor
    historySnipOnly  bool                                                  // GOOSE_HISTORY_SNIP=1 feature gate (REQ-CTX-016 호환)
    messageToEntry   func(m message.Message) trajectory.TrajectoryEntry    // 주입
    entryToMessage   func(e trajectory.TrajectoryEntry) message.Message    // 역변환
}

// [HARD] 컴파일 타임 계약 증명 — query.Compactor interface satisfaction.
// 위 import의 query 패키지 (`internal/query/config.go:59`)에 정의된 인터페이스.
var _ query.Compactor = (*CompactorAdapter)(nil)

// value-type parameter / 반환; receiver는 pointer 허용 (CONTEXT-001 DefaultCompactor와 동일 패턴).
func (a *CompactorAdapter) ShouldCompact(s loop.State) bool
func (a *CompactorAdapter) Compact(s loop.State) (loop.State, query.CompactBoundary, error)
```

`loop.State`는 `internal/query/loop/state.go` 소유. `query.Compactor` / `query.CompactBoundary`는 `internal/query/config.go` 소유 (CONTEXT-001 §6.2/§6.3에 명시된 계약 인터페이스). `goosecontext.DefaultCompactor` / `goosecontext.Strategy*` / `goosecontext.TokenCountWithEstimation` / `goosecontext.CalculateTokenWarningState` / `goosecontext.WarningRed`는 모두 `internal/context` (alias `goosecontext`) 소유. 본 SPEC 어댑터는 `query`/`loop`/`goosecontext` 3개 패키지를 import하여 계약에 맞춘다.

### 6.3 알고리즘 의사코드 (hermes-learning.md §3 기반)

```
Compress(t):
    metrics = new TrajectoryMetrics{StartedAt: now()}
    
    turnTokens = [tokenizer.Count(e.Value) for e in t.Conversations]
    total = sum(turnTokens)
    metrics.OriginalTokens = total
    metrics.OriginalTurns  = len(t.Conversations)
    
    if total <= cfg.TargetMaxTokens:
        metrics.SkippedUnderTarget = true
        metrics.CompressedTokens   = total
        return t, metrics, nil
    
    protected = findProtectedIndices(t, cfg.TailProtectedTurns)
    // REQ-021: redacted_thinking 블록을 포함한 인덱스는 protected에 자동 추가
    for i, entry := range t.Conversations:
        if entry.HasRedactedThinkingBlock():
            protected[i] = struct{}{}
    compressStart, compressEnd = findCompressibleRegion(protected, len(t.Conversations))
    
    if compressStart >= compressEnd:
        metrics.SkippedNoCompressibleRegion = true
        metrics.StillOverLimit = true
        return t, metrics, nil
    
    tokensToSave  = total - cfg.TargetMaxTokens
    targetCompress = tokensToSave + cfg.SummaryTargetTokens
    
    accumulated  = 0
    compressUntil = compressStart
    for i := compressStart; i < compressEnd; i++:
        accumulated += turnTokens[i]
        compressUntil = i + 1
        if accumulated >= targetCompress:
            break
    
    middle = t.Conversations[compressStart:compressUntil]
    if len(middle) <= 1:
        metrics.SkippedNoCompressibleRegion = true
        return t, metrics, nil
    
    summary, err = summarizeWithRetry(ctx, middle)
    metrics.SummarizationApiCalls = attempts
    metrics.SummarizationErrors   = errors
    if err != nil:
        return t, metrics, wrap(err)
    
    summaryTokens = tokenizer.Count(summary)
    // D17 근거: SummaryOvershootFactor(기본 2.0) 참조 — §6.2 주석 참고
    if summaryTokens > int(float64(cfg.SummaryTargetTokens) * cfg.SummaryOvershootFactor):
        metrics.SummarizerOvershot = true
        return t, metrics, nil
    
    // REQ-002/D19: Metadata deep copy로 입력 불변성 보장 (map 공유 금지).
    compressed = new Trajectory{
        Conversations: concat(
            t.Conversations[:compressStart],
            [{From: human, Value: summary}],
            t.Conversations[compressUntil:],
        ),
        Timestamp: t.Timestamp, Model: t.Model, Completed: t.Completed,
        SessionID: t.SessionID,
        Metadata:  deepCopyMetadata(t.Metadata),   // map/slice/중첩 struct 복사
    }
    
    metrics.WasCompressed         = true
    metrics.CompressedTurns       = len(compressed.Conversations)
    metrics.CompressedTokens      = tokenizer.CountTrajectory(compressed)
    metrics.TokensSaved           = metrics.OriginalTokens - metrics.CompressedTokens
    metrics.CompressionRatio      = float64(metrics.CompressedTokens) / float64(metrics.OriginalTokens)
    metrics.TurnsCompressedStartIdx = compressStart
    metrics.TurnsInCompressedRegion = compressUntil - compressStart
    metrics.StillOverLimit        = (metrics.CompressedTokens > cfg.TargetMaxTokens)
    metrics.EndedAt               = now()
    return compressed, metrics, nil
```

### 6.4 Summarizer Prompt 템플릿 (기본)

```
You are summarizing a middle section of an AI agent's tool-augmented conversation.
The summary will replace the middle turns in a trajectory. The head and tail are preserved.

CONSTRAINTS:
- Maximum {{.TargetTokens}} tokens.
- Preserve: tool names invoked, error outcomes, key decisions, file paths mentioned.
- Drop: verbose tool output bodies, boilerplate assistant explanations.
- Format: 3-7 bullet points, each starting with a verb.

TURNS TO SUMMARIZE:
{{range .Turns}}
[{{.From}}] {{.Value}}
{{end}}

SUMMARY:
```

### 6.5 재시도 백오프 공식

```
delay(attempt) = BaseDelay * 2^attempt * rand.Float64(0.5, 1.5)
// attempt: 0, 1, 2  (MaxRetries=3 이므로 최대 3회 호출)
// BaseDelay=2s 기본:
//   attempt 0: [1s, 3s]
//   attempt 1: [2s, 6s]
//   attempt 2: [4s, 12s]
// 총 worst-case wait: 21s (실행 시간 별도)
```

Jitter 근거: thundering herd 방지 (50개 동시 호출이 동일 시점에 재시도 몰리는 걸 방지).

### 6.6 CONTEXT-001 어댑터 세부 (REQ-018/019/020/021 구현 근거)

어댑터는 **value-type parameter / 반환** (receiver는 pointer) 시그니처를 유지하고, CONTEXT-001의 `ShouldCompact`/`Compact` 의미론을 그대로 재현한다. 전략 선택 순서는 `ReactiveCompact → AutoCompact → Snip` (CONTEXT-001 REQ-CTX-008/017/018)이며, `Summarizer == nil` 또는 `Summarize` 에러 시 `Snip` 폴백 (REQ-CTX-012/014). 패키지 alias: `goosecontext = "github.com/modu-ai/goose/internal/context"`, `query = "github.com/modu-ai/goose/internal/query"`.

```go
// REQ-019: 4개 trigger 의미론 (CONTEXT-001 REQ-CTX-007 3-trigger + REQ-CTX-011 Red override).
// 의미상 Red(>=92%)는 80% trigger의 superset이므로 (a) ratio>=0.80과 (d) Red는 합집합 동치이며
// (d)의 별도 평가는 idempotent (Stage-D defect plan-audit iter2 정정).
func (a *CompactorAdapter) ShouldCompact(s loop.State) bool {
    used := goosecontext.TokenCountWithEstimation(s.Messages)
    ratio := float64(used) / float64(s.TokenLimit)
    if ratio >= 0.80 {
        return true
    }
    if s.AutoCompactTracking.ReactiveTriggered {
        return true
    }
    if len(s.Messages) > s.MaxMessageCount {
        return true
    }
    // Red override (>=92%): REQ-CTX-011 (80% trigger의 superset이므로 도달 가능성 0이지만 명시 보존)
    if goosecontext.CalculateTokenWarningState(used, int64(s.TokenLimit)) >= goosecontext.WarningRed {
        return true
    }
    return false
}

// REQ-020: 전략 선택 + Summarizer nil/err 폴백
// REQ-021: redacted_thinking 보존
func (a *CompactorAdapter) Compact(s loop.State) (loop.State, query.CompactBoundary, error) {
    strategy := a.selectStrategy(s)

    // (b) Summarizer == nil 폴백 (CONTEXT-001 REQ-CTX-012)
    if (strategy == goosecontext.StrategyAutoCompact || strategy == goosecontext.StrategyReactiveCompact) &&
       a.inner.summarizer == nil {
        strategy = goosecontext.StrategySnip
    }

    switch strategy {
    case goosecontext.StrategySnip:
        // Snip은 CONTEXT-001 DefaultCompactor 책임 — 어댑터는 inner DefaultCompactor에 위임
        if a.snipDelegate != nil {
            return a.snipDelegate.Compact(s)
        }
        // delegate 미주입 시 no-op (원본 State 반환)
        return s, query.CompactBoundary{
            Turn: s.TurnCount, Strategy: goosecontext.StrategySnip,
            MessagesBefore: len(s.Messages), MessagesAfter: len(s.Messages),
            TaskBudgetPreserved: int64(s.TaskBudgetRemaining),
        }, nil

    case goosecontext.StrategyAutoCompact, goosecontext.StrategyReactiveCompact:
        traj := a.messagesToTrajectoryPreservingRedacted(s.Messages)  // REQ-021
        // adapterConfig는 inner.cfg를 AdapterMaxRetries로 override (D13/R5)
        compressed, metrics, err := a.inner.CompressWithRetries(ctx, traj, a.inner.cfg.AdapterMaxRetries)
        if err != nil {
            // CONTEXT-001 REQ-CTX-014: Summarizer 에러 시 Snip 폴백 (에러 미전파)
            a.inner.logger.Warn("summarizer failed; falling back to Snip",
                zap.Error(err), zap.String("strategy", string(strategy)))
            if a.snipDelegate != nil {
                return a.snipDelegate.Compact(s)
            }
            return s, query.CompactBoundary{Turn: s.TurnCount, Strategy: goosecontext.StrategySnip}, nil
        }
        newState := cloneStateDeep(s)
        newState.Messages = a.trajectoryToMessagesPreservingRedacted(compressed)  // REQ-021
        // TaskBudgetRemaining 보존 (CONTEXT-001 REQ-CTX-010) — flat int 필드 (loop.State.TaskBudgetRemaining)
        newState.TaskBudgetRemaining = s.TaskBudgetRemaining
        boundary := query.CompactBoundary{
            Turn:                 s.TurnCount,
            Strategy:             string(strategy),
            MessagesBefore:       len(s.Messages),
            MessagesAfter:        len(newState.Messages),
            TokensBefore:         metrics.OriginalTokens,
            TokensAfter:          metrics.CompressedTokens,
            TaskBudgetPreserved:  int64(s.TaskBudgetRemaining),
            DroppedThinkingCount: metrics.DroppedThinkingCount, // REQ-021 보존 카운트
        }
        return newState, boundary, nil
    }
    return s, query.CompactBoundary{}, fmt.Errorf("unknown strategy: %v", strategy)
}

// selectStrategy: CONTEXT-001 REQ-CTX-008/017/018 우선순위.
// HistorySnipOnly는 어댑터 자체 flag (loop.State에 해당 필드 없음 — REQ-CTX-016 호환).
func (a *CompactorAdapter) selectStrategy(s loop.State) string {
    if a.historySnipOnly {
        return goosecontext.StrategySnip                              // REQ-CTX-016
    }
    if s.AutoCompactTracking.ReactiveTriggered {
        return goosecontext.StrategyReactiveCompact                   // REQ-CTX-017
    }
    used := goosecontext.TokenCountWithEstimation(s.Messages)
    ratio := float64(used) / float64(s.TokenLimit)
    if ratio >= 0.80 {
        return goosecontext.StrategyAutoCompact                       // REQ-CTX-018
    }
    return goosecontext.StrategySnip
}
```

`messagesToTrajectoryPreservingRedacted` / `trajectoryToMessagesPreservingRedacted` 쌍은 **REQ-021의 핵심 구현**으로, `redacted_thinking` content block을 부가 payload로 운반하여 요약 전후 어느 경로에서도 drop/mutate되지 않도록 보장한다. Summarizer에 전달되는 `middle_slice`에서는 `redacted_thinking` 블록을 먼저 제거(opaque 블록이 LLM에 노출되지 않도록)한 뒤, 요약 결과 entry 양옆에 보존 entry로 재부착한다.

### 6.7 TDD 진입 순서

1. **RED #1**: `TestProtected_FourDistinctRolesFirst` — AC-COMPRESSOR-003.
2. **RED #2**: `TestProtected_TailFourTurns` — AC-COMPRESSOR-004.
3. **RED #3**: `TestCompress_UnderTargetSkips` — AC-COMPRESSOR-001.
4. **RED #4**: `TestCompress_InputImmutable` — AC-COMPRESSOR-012.
5. **RED #5**: `TestCompress_HappyPath` — AC-COMPRESSOR-002.
6. **RED #6**: `TestCompress_RetriesThreeThenSucceeds` — AC-COMPRESSOR-005.
7. **RED #7**: `TestCompress_RetriesExhausted` — AC-COMPRESSOR-006.
8. **RED #8**: `TestCompress_PerTrajectoryTimeout` — AC-COMPRESSOR-007.
9. **RED #9**: `TestCompress_NoCompressibleRegion` — AC-COMPRESSOR-010.
10. **RED #10**: `TestCompress_SummarizerOvershot` — AC-COMPRESSOR-011.
11. **RED #11**: `TestBatch_50Parallelism` — AC-COMPRESSOR-008.
12. **RED #12**: `TestBatch_IndividualFailureIsolated` — AC-COMPRESSOR-009.
13. **RED #13**: `TestAdapter_CompactorInterfaceCompatible` — AC-COMPRESSOR-013 (value-receiver + `var _ loop.Compactor = ...`).
14. **RED #14**: `TestCompress_MetricsNonNilOnErrorPath` — AC-COMPRESSOR-014 (REQ-001).
15. **RED #15**: `TestNoHardcodedTokenRatios` — AC-COMPRESSOR-015 (REQ-004, 정적 grep).
16. **RED #16**: `TestCompress_MiddleRegionShortOfTarget` — AC-COMPRESSOR-016 (REQ-012).
17. **RED #17**: `TestCompress_ProtectedByteExactPreservation` — AC-COMPRESSOR-017 (REQ-013).
18. **RED #18**: `TestCompress_CustomPromptTemplateRenders` — AC-COMPRESSOR-018 (REQ-017).
19. **RED #19**: `TestAdapter_ShouldCompact_FourTriggers` — AC-COMPRESSOR-019 (REQ-019).
20. **RED #20**: `TestAdapter_StrategySelection_And_NilSummarizerFallback` — AC-COMPRESSOR-020 (REQ-020).
21. **RED #21**: `TestCompress_RedactedThinkingPreserved` — AC-COMPRESSOR-021 (REQ-021).
22. **RED #22**: `TestCompress_MetadataDeepCopy` — AC-COMPRESSOR-022 (REQ-002 강화/D19).
23. **GREEN**: 알고리즘 본체 + Summarizer retry wrapper + Semaphore batch + adapter strategy selector + redacted-thinking preserver + metadata deep copy.
24. **REFACTOR**: `protected.go`로 보호 계산 추출, `retry.go`로 backoff 추출, `redacted.go`로 redacted_thinking preserver 추출, `strategy.go`로 어댑터 전략 선택 추출.

### 6.8 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 22 AC 전부 integration test, `-race` 통과, Summarizer stub + Tokenizer stub으로 격리 |
| **R**eadable | 알고리즘 본체(§6.3)가 pseudocode와 1:1, 매직 넘버는 config 상수로 |
| **U**nified | `golangci-lint` 통과, metrics 필드 명명이 Hermes `TrajectoryMetrics`와 snake_case ↔ PascalCase 1:1 |
| **S**ecured | per-trajectory timeout 300s 강제, batch 실패 격리(한 궤적이 전체 abort 안 함), Summarizer overshoot 방어 |
| **T**rackable | 모든 경로에서 `TrajectoryMetrics` 반환 보장, zap 로그에 `session_id` / `original_tokens` / `compression_ratio` |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-TRAJECTORY-001 | `Trajectory`, `TrajectoryEntry`, `Role` 타입 소비 |
| 선행 SPEC | SPEC-GOOSE-ROUTER-001 | `Summarizer` 구현체가 내부에서 저렴한 모델 선택 시 사용 |
| 선행 SPEC | SPEC-GOOSE-ADAPTER-001 | `Summarizer.Summarize`의 실제 LLM 호출 구현 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap 로거, context 루트 |
| 후속 SPEC | SPEC-GOOSE-INSIGHTS-001 | `TrajectoryMetrics` 집계 소비 |
| 후속 SPEC | SPEC-GOOSE-CONTEXT-001 | `CompactorAdapter`가 CONTEXT-001 인터페이스 만족 |
| 외부 | Go 1.22+ | generics(`semaphore`), `context.WithTimeout` |
| 외부 | `go.uber.org/zap` v1.27+ | CORE-001 계승 |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |
| 외부(선택) | `github.com/tiktoken-go/tokenizer` | 정밀 Tokenizer 구현체(주입자 선택) |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | SimpleTokenizer 근사가 실제 LLM 토큰과 20%+ 오차 | 고 | 중 | `Tokenizer` 인터페이스로 분리, 프로덕션은 `tiktoken-go` 주입. `SimpleTokenizer`는 개발/테스트용 |
| R2 | Summarizer가 압축 목표(`SummaryTargetTokens=750`)를 무시하고 장황한 요약 반환 | 중 | 중 | REQ-015/AC-011로 2배 초과 시 폐기. 프롬프트 템플릿에 "≤ 750 tokens" 명시 |
| R3 | 보호 인덱스 규칙이 한 도메인에 편향됨(예: 툴 많은 세션은 head 4 role 만족 못함) | 중 | 낮 | `findProtectedIndices`가 없는 role은 스킵(nil entry 안 만듦). Edge case 테스트 추가 |
| R4 | Batch 50 동시성이 provider RPM 한도 위반 | 중 | 중 | `MaxConcurrentRequests` 설정 가능. Hermes 원본값 50은 Gemini 3 Flash 전용. 타 모델은 RATELIMIT-001과 교차 조정 |
| R5 | Retry jittered backoff가 CONTEXT-001의 in-session compaction에서 너무 길음(21s worst case가 UI blocking) | 중 | 중 | `CompressionConfig.AdapterMaxRetries`(기본 1) 필드로 어댑터 경로만 별도 상한 지정(§6.2 / D13). offline batch 경로는 `MaxRetries=3` 유지. 어댑터가 `inner.CompressWithRetries(ctx, traj, AdapterMaxRetries)`를 호출하여 override를 강제한다 |
| R6 | Summarizer가 PII를 요약에 노출(원본에는 redact됐으나 LLM 응답은 아님) | 중 | 고 | TRAJECTORY-001에서 redact한 궤적만 입력 허용. 프롬프트에 "do not reveal masked tokens" 명시. 향후 2차 redact pass 고려 |
| R7 | StillOverLimit 궤적이 누적되어 LoRA 훈련 시 batch skew | 낮 | 중 | INSIGHTS-001가 비율 모니터링, 임계치 초과 시 TargetMaxTokens 재조정 권고 |
| R8 | 어댑터 경로의 `messagesToTrajectory` 변환 비용이 매 turn 반복 | 중 | 낮 | CONTEXT-001에서 ShouldCompact는 cheap tokenizer만 호출, 변환은 Compact 시에만 |
| R9 | 압축된 trajectory의 meta(Partial/FailureReason) 보존 실수 | 낮 | 중 | `compressed.Metadata = t.Metadata` 강제, 테스트로 검증 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-learning.md` §3 Trajectory Compressor 알고리즘(의사코드 원본), §2 TrajectoryMetrics 스키마
- `.moai/project/learning-engine.md` §6.2 Catastrophic Forgetting 방지(요약 품질 기준)
- `.moai/specs/ROADMAP.md` §4 Phase 4 #20, §11 오픈 이슈 #8 (Tokenizer 선택)
- `.moai/specs/SPEC-GOOSE-TRAJECTORY-001/spec.md` — 입력 공급자
- `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md` — Compactor 인터페이스 정의자

### 9.2 외부 참조

- **Hermes `trajectory_compressor.py`** (1517 LoC): 알고리즘 원본
- **Google Gemini 3 Flash** (temp=0.3 recommended): Hermes가 선택한 summary 모델 근거
- **tiktoken-go**: https://github.com/tiktoken-go/tokenizer — OpenAI 호환 Go 토크나이저
- **Exponential backoff jitter**: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/

### 9.3 부속 문서

- `./research.md` — Hermes 1517 LoC → Go 800 LoC 이식 매핑, Tokenizer 결정 근거, Prompt 템플릿 설계
- `../SPEC-GOOSE-TRAJECTORY-001/spec.md` — 선행
- `../SPEC-GOOSE-INSIGHTS-001/spec.md` — 후속 메트릭 소비자

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **실제 LLM HTTP 호출을 구현하지 않는다**. `Summarizer` 인터페이스만. ADAPTER-001 구현.
- 본 SPEC은 **Tokenizer 본격 구현체를 포함하지 않는다**. `SimpleTokenizer` 근사만 제공, `tiktoken-go` 등은 주입자 책임.
- 본 SPEC은 **Insights 집계 / 통계 / 차트를 포함하지 않는다**. `TrajectoryMetrics` 수집만. INSIGHTS-001.
- 본 SPEC은 **디스크 재저장을 수행하지 않는다**. `Compress`는 in-memory 반환. 호출자가 TRAJECTORY-001 `Writer` 호출.
- 본 SPEC은 **Multi-pass 재압축을 하지 않는다**. 1회 실행. `StillOverLimit` 플래그로 호출자에게 위임.
- 본 SPEC은 **Streaming summarization을 지원하지 않는다**. `Summarizer.Summarize`는 완전 문자열만.
- 본 SPEC은 **보호 인덱스의 학습 기반 동적 조정을 포함하지 않는다**. 고정 정책(4 role + 4 tail).
- 본 SPEC은 **Summarizer 모델 선택 자체를 구현하지 않는다**. ROUTER-001 위임.
- 본 SPEC은 **압축 결과의 품질 회귀 검증을 포함하지 않는다**. REFLECT-001에서 평가.
- 본 SPEC은 **LoRA 훈련 데이터셋 포맷 변환을 포함하지 않는다**. LORA-001.
- 본 SPEC은 **CONTEXT-001의 `Snip` 전략 구체 구현(protected window + redacted_thinking 마커 부착)을 포함하지 않는다**. `CompactorAdapter.Compact`의 Snip 폴백 경로는 CONTEXT-001 `DefaultCompactor.Snip`에 주입 위임한다.
- 본 SPEC은 **CONTEXT-001의 `ReactiveCompact` 분기 자체를 독립 구현하지 않는다**. 어댑터는 `ReactiveTriggered` 신호를 전달받아 전략 선택에 반영할 뿐, 예측/트리거 설정(`ReactiveTriggered = true`)은 SPEC-GOOSE-QUERY-001 책임이다.
- 본 SPEC은 **`loop.State`/`loop.CompactBoundary` 타입을 정의하지 않는다**. 정의는 SPEC-GOOSE-QUERY-001 `internal/query/loop/` 소유이며, 본 SPEC은 consumer/producer.

---

**End of SPEC-GOOSE-COMPRESSOR-001**

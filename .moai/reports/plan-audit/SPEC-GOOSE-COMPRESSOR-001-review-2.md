## SPEC-GOOSE-COMPRESSOR-001 v0.2.0 Audit Report (iter2)

Auditor: plan-auditor
Date: 2026-05-02
Iteration: 2 / 3
입력 자료: spec.md (786 lines), research.md (346 lines), progress.md (107 lines)

> Reasoning context ignored per M1 Context Isolation. Verdict derived solely from spec.md / research.md / progress.md plus code/SPEC cross-references in `internal/query/`, `internal/query/loop/`, `internal/context/`, and `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md`.

### Verification Matrix

| 축 | 평가 | 근거 |
|---|---|---|
| D1 EARS | PASS | spec.md L97-148 — 21 REQ 모두 EARS 패턴 라벨 보유. Ubiquitous(REQ-001~004, 021), Event-Driven(005~010, 019, 020), State-Driven(011, 012), Unwanted(013~016), Optional(017, 018) 분류 적절. 단, REQ-019/REQ-020이 "Optional" 섹션(L138)에 있으나 라벨은 `[Event-Driven]`(L143, L145) — 섹션 헤더와 라벨 라벨링 불일치(minor). |
| D2 REQ↔AC mapping | PASS-WITH-NOTE | REQ 21개 / AC 22개 (AC-001..AC-022). REQ→AC 명시 매핑: 001→AC-014, 002→AC-001/012/022, 003→AC-003, 004→AC-004/015, 005→AC-001, 006→AC-002, 007→AC-005, 008→AC-006, 009→AC-008, 010→AC-007, 011→AC-010, 012→AC-016, 013→AC-017, 014→AC-010, 015→AC-011, 016→AC-009, 017→AC-018, 018→AC-013, 019→AC-019, 020→AC-020, 021→AC-021. 모든 REQ가 ≥1 AC 보유. 다만 §6.8 TRUST 표(L700)가 "13 AC"로 옛 카운트 사용 — 본문과 불일치(minor). |
| D3 CONTEXT-001 contract conformance | **FAIL (Critical)** | 본 SPEC §6.2 L449/L452-453, §6.6 L578/L598 모두 `loop.Compactor` / `loop.CompactBoundary` 를 import한다고 선언. 그러나 **실제 인터페이스는 `query` 패키지 소유** — `internal/query/config.go:59-64` 에 `query.Compactor`, L68-86에 `query.CompactBoundary` 정의. CONTEXT-001 §6.3 L367/L373 도 명시적으로 `context.DefaultCompactor` 가 `query.Compactor` 를 구현한다고 적시(`var _ query.Compactor = ...`). 또한 `internal/context/compactor.go:67` 의 실제 컴파일타임 assertion 도 `var _ query.Compactor = (*DefaultCompactor)(nil)`. 본 SPEC L449 의 `var _ loop.Compactor = (*CompactorAdapter)(nil)` 는 **빌드 실패 지시문**이다. |
| D4 Exclusions specificity | PASS | spec.md L766-782 — 12개 OUT 항목 모두 named SPEC(ADAPTER-001, INSIGHTS-001, ROUTER-001, REFLECT-001, LORA-001, CONTEXT-001, QUERY-001, TRAJECTORY-001) 또는 구체적 책임 경계 참조. §3.2 L82-89 도 동일하게 구체적. |
| D5 Bias absence | PASS | iter1→iter2 delta 가 HISTORY L23 에 D3-D19 로 나열되고 본문 변경에 1:1 매핑 가능: D13 → §6.2 L300 `AdapterMaxRetries`, D17 → §6.2 L305-310 + §6.3 L509-510 `SummaryOvershootFactor` 근거 주석, D19 → AC-022(L260-263) Metadata deep copy, D14 → AC-001(L156-158)에 reflect.DeepEqual 명시, REQ-019/020/021 → §4.5(L141-148). Hermes 1517 LoC 인용 L22, L46, §6.3 의사코드 L460-536 등 외부 evidence 풍부. |
| D6 Backward compat | PASS | 신규 패키지 `internal/learning/compressor/` (research.md L17 "전부 부재"). CONTEXT-001 surface 변경 0건 (progress.md L55-58, L57). TRAJECTORY-001 / ROUTER-001 surface 변경 0건. Adapter 는 consumer 측에서만 작성. |
| D7 Successor SPEC compat | PASS | `TrajectoryMetrics` (§6.2 L392-415) 23 필드 — INSIGHTS-001 의 token 절감률·비율·turn 통계 소비에 필요한 필드(OriginalTokens, CompressedTokens, TokensSaved, CompressionRatio, WasCompressed, SkippedUnderTarget, TimedOut, SummarizationApiCalls, StartedAt/EndedAt) 모두 포함. REFLECT-001 품질 회귀를 위한 SummarizerOvershot, StillOverLimit, SkippedNoCompressibleRegion 도 노출. |
| D8 TRUST 5 mapping | PASS-WITH-NOTE | spec.md §6.8 L698-704 5 차원 모두 매핑. T(85%+), R(pseudocode 1:1), U(golangci-lint), S(timeout/격리/overshoot 방어), T(non-nil metrics + zap). 다만 "13 AC" 표기 오류(D2 참조). |

### Defects

- **D-A (Critical)** — spec.md L449, L452-453, L456, L578, L598, L602, L604, L608, L613, L620, L621, L631, L637, L654-664, AC-013 L218: 어댑터가 의존하는 `loop.Compactor`, `loop.CompactBoundary`, `loop.SnipCompactor`, `loop.CompactStrategy`, `loop.StrategyAutoCompact|ReactiveCompact|Snip`, `loop.TokenCountWithEstimation`, `loop.CalculateTokenWarningState`, `loop.WarningRed` 심볼이 **`internal/query/loop/` 패키지에 존재하지 않는다**. 실제 위치: `internal/query/config.go` 가 `query.Compactor` / `query.CompactBoundary` 소유; `internal/context/compactor.go:14-21` 이 `StrategyAutoCompact/ReactiveCompact/Snip` 소유; `internal/context/tokens.go:32, 81, 22` 가 `TokenCountWithEstimation`, `CalculateTokenWarningState`, `WarningRed` 소유. `loop.SnipCompactor` / `loop.CompactStrategy` 는 **레포 어디에도 정의되지 않은 가상 타입**. 권고: §6.2/§6.6 import 경로를 `query.Compactor`, `query.CompactBoundary` 로 정정하고, AutoCompact 위임을 위해 `goosecontext.DefaultCompactor` 를 inner delegate로 주입(또는 별도 `SnipDelegate interface { Snip(loop.State) (loop.State, query.CompactBoundary, error) }` 본 SPEC 신규 정의 + DefaultCompactor 어댑팅). REQ-018/AC-013/REQ-020/AC-020 모두 영향.
- **D-B (Critical)** — spec.md L636 `newState.TaskBudget.Remaining = s.TaskBudget.Remaining`, AC-013 L218 `TaskBudget.Remaining`. 실제 `loop.State` 필드는 `TaskBudgetRemaining int`(state.go L29, single-level). `s.TaskBudget` 중첩 struct 는 존재하지 않으며 또한 `int` ↔ `int64` 타입 불일치(`query.CompactBoundary.TaskBudgetPreserved` 는 int64, config.go L83). 권고: `newState.TaskBudgetRemaining = s.TaskBudgetRemaining` 으로 정정, AC-013 (d)도 동일.
- **D-C (Critical)** — spec.md L590 `loop.CalculateTokenWarningState(s).Level == loop.WarningRed`. 실제 시그니처 `CalculateTokenWarningState(used, limit int64) WarningLevel`(tokens.go L81, `internal/context` 패키지) — State 가 아니라 두 int64 인자, 그리고 반환은 `WarningLevel` 자체이지 `.Level` 필드 보유 struct 아님. REQ-019 사용 예시(L143)도 동일 오류 — 의사코드 수정 필요.
- **D-D (Major)** — spec.md L653 `if s.HistorySnipOnly`. `loop.State` 는 `HistorySnipOnly` 필드가 없다(state.go L17-41). 실제 위치는 `goosecontext.DefaultCompactor.HistorySnipOnly`(compactor.go L60). 어댑터는 inner DefaultCompactor 인스턴스를 통해 접근하거나 CONTEXT-001 에 State-side flag 추가 amendment 가 필요 — 후자는 본 SPEC 범위 초과로 권고하지 않음. 권고: `if a.snipDelegate.HistorySnipOnly()` 또는 inner config 채널로 우회.
- **D-E (Major)** — REQ-019 L143 (d) 가 "Red override"를 ShouldCompact 의 4번째 trigger로 명시하나 CONTEXT-001 REQ-CTX-007(L117) 은 3-trigger(80%/Reactive/MaxMessageCount)만 정의하고 Red 는 REQ-CTX-011 별도 state-driven override. 실제 코드(compactor.go L139-146) 는 Red 가 80% 의 strict superset 이므로 합집합과 동치 — 의미론적으로 안전. 다만 DRY 위반: spec.md REQ-019 의 "(a) ratio>=0.80 ... (d) Red" 는 `(a)` 가 `(d)` 를 함의하므로 (d) 는 redundant. 권고: REQ-019 wording 을 "REQ-CTX-007 의 3 trigger + REQ-CTX-011 의 Red override(80% 조건의 superset이므로 별도 평가는 idempotent)" 로 명시.
- **D-F (Major)** — spec.md §6.6 L598 `func (a *CompactorAdapter) Compact(s loop.State) (loop.State, loop.CompactBoundary, error)` 시그니처는 spec.md L218 AC-013 (a) "value receiver" 와 모순. Go 의 `func (a *CompactorAdapter) ...` 는 **pointer receiver**. CONTEXT-001 §6.2 L339-340 도 실제로는 `*DefaultCompactor` pointer receiver로 구현(compactor.go L118, L153). HISTORY L23 의 "value receiver(`loop.State`/반환값)로 교정" 표현이 receiver 와 parameter 를 혼동. 권고: HISTORY 와 AC-013 (a) 를 "value-type parameter(`s loop.State`) 및 value-type 반환(`loop.State, query.CompactBoundary`); receiver 는 pointer 허용" 으로 정정.
- **D-G (Minor)** — §6.8 L700 "13 AC 전부 integration test" — 실제 22 AC. 카운트 정정 필요.
- **D-H (Minor)** — §4.5 L138 헤더 "Optional (선택적)" 아래에 REQ-019/REQ-020 이 `[Event-Driven]` 라벨로 들어가 섹션-라벨 일관성 깨짐. 별도 §4.2.1 또는 라벨 변경 필요.
- **D-I (Minor)** — progress.md L67-75 의 "T-001..T-010" tasks 가 spec.md §6.7 L671-694 의 22 RED 단계와 매핑 불명확(T-008 이 AC-014~019 로 5개 합쳐짐). RED #1..#22 와 1:1 정렬 권고.
- **D-J (Minor)** — research.md L249 `var _ context.Compactor = (*CompactorAdapter)(nil)` — `context` 가 표준 라이브러리 충돌. 실제 코드는 `goosecontext` alias(compactor_test.go 에서 일관 사용). research.md 도 alias 명시 권고.

### Chain-of-Verification Pass

2nd self-check 결과:
- REQ 번호 sequencing: 001..021 연속, 중복 없음 (확인).
- AC 번호 sequencing: 001..022 연속, 중복 없음 (확인).
- AC→REQ trace 22개 모두 spot-check 가 아닌 line-by-line 확인 (D2 결과 PASS-WITH-NOTE).
- Exclusions specificity: §3.2 + §10 두 곳 모두 SPEC ID 또는 구체 책임 — 누락 없음.
- Adapter 시그니처 byte-exact 검증: `internal/query/config.go:59-64` 와 `spec.md L452-453` 비교 결과 패키지명 불일치 (D-A) 로 부적합 — 첫 패스 의 "PASS-WITH-NOTE" 후보를 **FAIL** 로 강등. 이는 단순 lint 가 아니라 빌드타임 컴파일 실패를 야기하는 P0 결함이며, RED #13 가 시작 자체 불가.
- Loop package symbol existence: `SnipCompactor`, `CompactStrategy`, `StrategyAutoCompact/Reactive/Snip` 가 `loop` 패키지에 존재하지 않음 — repo-wide grep 으로 0 hit 확인. 첫 패스에서 "value receiver" 만 보고 통과시킬 뻔했음을 회복.
- TaskBudget 필드 검증: state.go L29 `TaskBudgetRemaining int` (single-level) 확인 — spec L636/AC-013(d) 의 `s.TaskBudget.Remaining` 은 nil dereference 또는 컴파일 실패 (D-B).

### Regression Check (vs iter1)

iter1 D3-D19 19 defects 의 v0.2.0 해소 상태:

| iter1 defect | 해소 여부 |
|---|---|
| D3 (Compactor signature) | **UNRESOLVED** — receiver 는 pointer 로 정정됐으나 import 패키지(`loop` vs `query`) 가 여전히 잘못됨. D-A로 재제기. |
| D4 (ShouldCompact 4 trigger) | RESOLVED 의도, 하지만 D-E 잔여 redundancy. |
| D5 (Compact 전략 선택 + nil/err 폴백) | RESOLVED (REQ-020, AC-020). |
| D6 (redacted_thinking) | RESOLVED (REQ-021, AC-021). |
| D7-D11 (REQ-001/004/012/013/017 전용 AC) | RESOLVED (AC-014~018). |
| D12 (§1 wording) | RESOLVED (L36 / L38 변경). |
| D13 (AdapterMaxRetries) | RESOLVED (§6.2 L300, R5 L733). |
| D14 (AC-001 포인터 의미) | RESOLVED (L158 reflect.DeepEqual). |
| D15 (AC-010 tail) | RESOLVED. |
| D17 (2× 근거) | RESOLVED (§6.2 L305-310). |
| D19 (Metadata deep-copy) | RESOLVED (AC-022, §6.3 L514, L523). |

D-A/B/C/D 는 D3 의 부분 해소 후 재발 — iter1 가 receiver 의 syntax-level 만 검토하고 패키지 경로 / nested field / 외부 함수 시그니처를 검증하지 못한 결과. Stagnation(3-iter unchanged) 은 아니나 **D3 가 부분 해결**된 점을 명시.

### Verdict

**CONDITIONAL GO**

Rationale:
- 8 axes 중 D3(CONTEXT-001 계약 일치) 가 FAIL — adapter 가 import 하는 패키지가 실재하지 않거나 가상 심볼을 가정 (D-A/B/C/D). 이대로 RED #13 (TestAdapter_CompactorInterfaceCompatible) 는 시작 즉시 빌드 실패한다.
- 다른 7 axis 는 모두 PASS / PASS-WITH-NOTE. 알고리즘 본체(§6.3), 보호 인덱스(§3.1), batch/timeout/retry, redacted_thinking 보존, metadata deep copy, TRUST 매핑은 D-G/H/I/J(minor) 외 견고.
- D-A 는 spec 의 일부 패키지 alias / 함수 호출 정정만으로 해소되며 알고리즘·AC·전체 아키텍처 변경 불요 — fix scope 가 좁고 잘 격리됨. NO-GO 까지 갈 정도의 구조적 결함은 아님.
- D-A/B/C/D 가 해소되지 않은 채 run phase 진입 시, REQ-018/REQ-019/REQ-020 의 어댑터 경로(spec 의 약 25%) 가 작동 불가하며 **TDD RED 단계 자체 시작 불가**. 따라서 P0 Critical Path 임을 감안할 때 GO 부적격.

Recommended pre-run actions (CONDITIONAL GO 해제 조건):

1. spec.md §6.2 L437-454: `loop.Compactor` → `query.Compactor`; `loop.CompactBoundary` → `query.CompactBoundary`; `loop.SnipCompactor` 는 본 SPEC 신규 정의(예: `type SnipDelegate interface { Snip(loop.State) (loop.State, query.CompactBoundary, error) }`) 또는 inner `*goosecontext.DefaultCompactor` 직접 보유로 변경. import 절: `"github.com/modu-ai/goose/internal/query"`, `"github.com/modu-ai/goose/internal/query/loop"`, `"github.com/modu-ai/goose/internal/context"` (alias 권장 `goosecontext`).
2. spec.md L449: `var _ loop.Compactor = (*CompactorAdapter)(nil)` → `var _ query.Compactor = (*CompactorAdapter)(nil)`.
3. spec.md L452-453, L598, AC-013(L218): 반환 타입 `loop.CompactBoundary` → `query.CompactBoundary`. AC-013 (c) 필드 enumeration 은 그대로 유효 (Turn/Strategy/MessagesBefore/MessagesAfter/TokensBefore/TokensAfter/TaskBudgetPreserved 모두 query.CompactBoundary 보유, +DroppedThinkingCount 필드 추가 발견 — AC-013 (c) 에 명시 권고).
4. spec.md L636, AC-013(d) L218: `s.TaskBudget.Remaining` → `s.TaskBudgetRemaining` (int 단층). int↔int64 캐스팅 (`int64(s.TaskBudgetRemaining)`) 명시.
5. spec.md L590, REQ-019 L143 (d): `loop.CalculateTokenWarningState(s).Level == loop.WarningRed` → `goosecontext.CalculateTokenWarningState(goosecontext.TokenCountWithEstimation(s.Messages), tokenLimit) >= goosecontext.WarningRed` (또는 어댑터가 inner DefaultCompactor 의 메서드 위임). REQ-019 wording 도 "Red override 는 80% trigger 의 의미상 superset" 임을 주석.
6. spec.md L653 `if s.HistorySnipOnly`: 이 필드는 `loop.State` 에 없음. 어댑터가 보유하는 `goosecontext.DefaultCompactor` 인스턴스의 `HistorySnipOnly` 필드를 참조하거나, 어댑터 자체에 별도 flag 추가. AC-020 sub-case (c) 도 동일.
7. spec.md L23 HISTORY 의 "value receiver" 와 AC-013(a): "value-type parameter / 반환; receiver 는 pointer" 로 wording 명확화.
8. spec.md §6.8 L700 "13 AC" → "22 AC" 정정.
9. spec.md §4.5 L138: REQ-019/020 을 §4.2 Event-Driven 으로 이동 또는 §4.5 의 sub-section 신설(예: "Optional 어댑터 경로 — Event-Driven sub-rules").
10. progress.md L62-75: T-008/T-009 를 spec §6.7 RED #13~RED #22 와 1:1 정렬.

위 1-6 항이 핵심(D-A/B/C/D 해소). 7-10 은 cosmetic. 1-6 적용 후 iter3 재감사 권고. 적용된 수정은 build-test 가능 — `go build ./internal/learning/compressor/...` 가 통과하면 contract 정합성이 검증됨.

---

End of report.

# SPEC Review Report: SPEC-GOOSE-CONTEXT-001

Iteration: 2/3
Verdict: FAIL
Overall Score: 0.78

> Reasoning context ignored per M1 Context Isolation. Evidence sources: `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md` (v0.1.1) only; prior audit `.moai/reports/plan-audit/mass-20260425/CONTEXT-001-audit.md` referenced solely for regression mapping.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency** — REQ-CTX-001..018 순차, 중복 없음, 3자리 zero-padding 일관 (spec.md:L98-140). AC-CTX-001..015 순차 (spec.md:L146-219). 시퀀스 gap 없음.
- **[PASS] MP-2 EARS format compliance** — 18개 REQ 모두 라벨 부착, 형식 부합. 특히 신규 REQ-CTX-017/018은 `When ... AND ..., the compactor shall select ...` Event-Driven 패턴 정확 준수 (L120, L122). REQ-CTX-013은 [Ubiquitous] 라벨로 수정되어 "shall always return" 불변식 형식 부합 (L132).
- **[PASS] MP-3 YAML frontmatter validity** — id, version(0.1.1), status, created_at(이름 정정), updated_at, author, priority, issue_number, phase, size, lifecycle, labels 모두 present + 정확한 타입 (spec.md:L1-14). `labels: [area/runtime, type/feature, priority/p0-critical]` 배열 형식 PASS.
- **[N/A] MP-4 Section 22 language neutrality** — 본 SPEC은 단일 언어(Go) context/compaction 패키지 명세. 다중 언어 tooling 열거 요건 해당 없음. AUTO-PASS.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band (minor ambiguity in one or two requirements) | Strategy 우선순위 문자열이 §3.1 #9 L75("AutoCompact > ReactiveCompact > Snip")와 REQ-CTX-008 L114("ReactiveCompact > AutoCompact > Snip") 사이에 정반대로 기술됨. 구현자 혼선 확실. |
| Completeness | 0.92 | 1.0 band 근접 (1.0에서 약간 감점) | 모든 필수 섹션 present. Frontmatter 12필드 완비. Exclusions 11개 구체적 위임 항목 명시 (L494-505). HISTORY 테이블에 0.1.0→0.1.1 변경 사유 5개 항목으로 상세 기록 (L22-23). |
| Testability | 0.95 | 1.0 band 근접 | AC-CTX-001..015 모두 Given/When/Then 3요소 구조. 측정 가능 경계값 명시(AC-CTX-011 79_999/80_000/80_001 L198, AC-CTX-012 92_500 L202, AC-CTX-007 1234 정수동치 L178). 모호어 부재. AC-CTX-003 fixture 경로 미명시는 minor. |
| Traceability | 0.78 | 0.75 band (one REQ uncovered, one indirect) | REQ-CTX-017(신규) 전용 §5 AC 부재 (아래 D10 참조). REQ-CTX-018은 AC-CTX-005(token 90_000/100_000 + Strategy=="AutoCompact", L166-169)로 암묵 커버되나 명시적 AC 없음. 16개 REQ 중 iter1 지적 5건 고아는 모두 AC-CTX-011~015 신설로 해소(L196-219). |

## Defects Found

**D10. spec.md:L75 vs spec.md:L114 — Strategy 우선순위 모순 (NEW)** — Severity: **major**
- §3.1 #9 (L75): "Strategy 선택 (`AutoCompact` > `ReactiveCompact` > `Snip` 우선순위)" — Auto 우선.
- REQ-CTX-008 (L114): "evaluating trigger conditions in the priority order `ReactiveCompact` > `AutoCompact` > `Snip`" — Reactive 우선.
- REQ-CTX-017 (L120) "select ReactiveCompact in preference to AutoCompact" 및 REQ-CTX-018 (L122) "ReactiveTriggered == false" 조건 → REQ 계열은 Reactive>Auto>Snip을 채택. §3.1 #9 문자열이 0.1.1 갱신 시 stale.
- 영향: 구현자가 §3.1 요약을 근거로 AutoCompact 우선 분기를 작성하면 REQ-CTX-017 위반. 계약 수준 모순.

**D11. REQ-CTX-017 대응 §5 AC 부재 (NEW ORPHAN)** — Severity: **major**
- REQ-CTX-017 (L120): "When `Compactor.Compact(state)` is invoked AND `ReactiveTriggered == true`, the compactor shall select the `ReactiveCompact` strategy"
- AC-CTX-001..015 (L146-219) 중 어느 AC도 "state.AutoCompactTracking.ReactiveTriggered = true" 선조건에서 `CompactBoundary.Strategy == "ReactiveCompact"`를 검증하지 않음.
- §6.6 RED #17 (L420): `TestCompactor_ReactiveTriggered_SelectsReactive` 테스트 계획만 나열. iter1에서 지적한 "§5에 미등록된 test plan" 동일 패턴이 신규 REQ에서 재발.
- Traceability MP 위반 (AC-5: Each REQ-XXX has at least one corresponding AC).

**D12. spec.md:L112 vs spec.md:L325 — `MaxMessageCount` 명명 충돌 (UNRESOLVED from iter1 D8)** — Severity: **minor**
- REQ-CTX-007 (L112): `state.MaxMessageCount` — `loop.State` 필드.
- §6.2 `DefaultCompactor` struct (L325): `MaxMessageCount int // default 500` — compactor 필드.
- 동일 이름의 두 필드가 서로 다른 owner(state vs compactor). 디폴트 값만 compactor에 있고 REQ-CTX-007이 비교 대상은 state.MaxMessageCount. 어느 쪽이 source-of-truth인지 불명.

**D13. AC-CTX-003 providerGroundTruth fixture 경로 미명시 (UNRESOLVED from iter1 D7)** — Severity: **minor**
- spec.md:L159: "ground truth는 테스트 fixture에 cp949/utf-8 영/한 혼합 기준값" — fixture 저장 경로·파일명·획득 방법이 미명시. 테스트 reproducibility 약화.

**D14. REQ-CTX-013 배치 구조 불일치 (PARTIALLY RESOLVED from iter1 D4)** — Severity: **minor**
- REQ-CTX-013 라벨은 [Unwanted]→[Ubiquitous]로 정정(L132)되어 EARS 패턴 합치.
- 그러나 여전히 §4.4 "Unwanted Behavior (방지)" 하위에 배치. 섹션 제목과 라벨이 불일치. §4.1 Ubiquitous로 이동하는 편이 구조적 일관성 향상.

**D15. REQ-CTX-009 `<moai-snip-marker>` 포맷 불변식 미명문화 (UNRESOLVED from iter1 D9)** — Severity: **minor**
- REQ-CTX-009 (L116)는 "`<moai-snip-marker>`를 insert + redacted_thinking 블록 attach" 를 선언하나, auxiliary content의 structural schema(key 이름, array 순서, role 값) 불변식이 REQ/AC에 명문화되지 않음. R2(Risk L457)에서 "Anthropic API 포맷 부합" 요건이 식별되나 REQ 수준 계약으로 끌어올리지 않음.

## Chain-of-Verification Pass

2차 재검 수행 결과:

- **REQ 18개 전수 재점검**: 001~016 + 신규 017, 018 모두 존재. 라벨 부착·형식 합치 재확인.
- **AC 15개 전수 재점검**: 001~010 + 신규 011~015. 각 AC의 Given/When/Then 3요소 구조 재확인.
- **REQ↔AC 역방향 매핑 재검**: REQ-CTX-017의 §5 AC 부재 확정. REQ-CTX-018은 AC-CTX-005 암묵 커버(token 90_000/100_000 비율 + Strategy=="AutoCompact")만 존재하고 명시적 AC는 없음. 1차 통독에서 식별한 D11을 2차 재독에서 확정.
- **§3.1 전체 14개 항목 스캔**: #9 (L75)의 우선순위 문자열이 REQ-CTX-008 L114와 정반대로 stale. 2차 통독에서 확정 → D10 확정.
- **타입 소속 패키지 재검 (iter1 D2)**: L72, L78, L229-246, L331-332, L335-348, L357-365 모두 `loop.State`/`loop.CompactBoundary` 일관. 회귀 없음.
- **Exclusions 구체성 재검 (11개 항목 L494-505)**: 모두 위임 대상(COMPRESSOR-001/ADAPTER-001/CONFIG-001/MEMORY-001/REFLECT-001/QUERY-001) 또는 "opaque 보존만" 같은 구체적 무처리 선언 포함. placeholder 없음. PASS.
- **새로 발견된 결함**: D10(§3.1 vs REQ-008 우선순위), D11(REQ-017 고아). 두 건 모두 1차 통독에서 감지, 2차 재검으로 확정.

## Regression Check (Iteration 2)

Iteration 1 defects from `CONTEXT-001-audit.md`:

| # | Iter1 결함 | 상태 | Evidence |
|---|----------|-----|----------|
| D1 | 고아 REQ 5건 (CTX-007/011/013/015/016) | **RESOLVED** | AC-CTX-011~015 신설 (spec.md:L196-219). 각 AC가 주석에 `(covers REQ-CTX-NNN)` 명시. |
| D2 | CompactBoundary/State 타입 소속 불일치 (query.State vs loop.State, context vs loop) | **RESOLVED** | §3.1 #9/#12(L72,L78), §6.1 레이아웃(L229-246), §6.2 서명(L331-332) + loop 패키지 정의(L335-348), §6.3 Contract(L357-365) 모두 `loop.State`/`loop.CompactBoundary`로 통일. Exclusions에도 재천명(L504). |
| D3 | AutoCompact/ReactiveCompact trigger 분리 REQ 부재 | **PARTIALLY RESOLVED** | REQ-CTX-017(L120), REQ-CTX-018(L122) 신설. 그러나 §3.1 #9 L75가 stale 우선순위를 유지 → D10(신규). |
| D4 | REQ-CTX-013 `[Unwanted]` 라벨 부적절 | **PARTIALLY RESOLVED** | 라벨 `[Ubiquitous]`로 수정(L132). 섹션 배치는 여전히 §4.4 Unwanted → D14(신규 minor). |
| D5 | Frontmatter `created` vs `created_at` 불일치 / `labels` 부재 | **RESOLVED** | `created_at: 2026-04-21`(L5), `labels: [area/runtime, type/feature, priority/p0-critical]`(L13). |
| D6 | R5 integration test 포인트 미명시 | **RESOLVED** | R5 mitigation 확장(L460): `TestQueryEngine_CompactTurn_TaskBudgetAccounting` integration test 포인트 명시 + 책임 경계(AC-CTX-007 본 SPEC, AC-QUERY-011 QUERY-001) 명문화. |
| D7 | AC-CTX-003 providerGroundTruth fixture 경로 미명시 | **UNRESOLVED** | L159 여전히 fixture 경로 부재 → D13(carried-over minor). |
| D8 | REQ-CTX-007 `state.MaxMessageCount` vs §6.2 `DefaultCompactor.MaxMessageCount` 이름 혼선 | **UNRESOLVED** | L112, L325 동일 이름 유지, owner 구분 해설 없음 → D12(carried-over minor). |
| D9 | `<moai-snip-marker>` 포맷 불변식 REQ 미명문화 | **UNRESOLVED** | REQ-CTX-009(L116) auxiliary content schema 계약 부재 → D15(carried-over minor). |

**Resolved 비율**: 6/9 = 67% (D1, D2, D5, D6 완전 해소; D3, D4 부분 해소로 새 결함 생성). **Unresolved minor 3건**(D7/D8/D9 → D13/D12/D15 carry-over). **Stagnation 감지 없음**(각 결함에 변화 흐름 관찰됨). **신규 결함 2건**(D10 major, D11 major).

## Recommendation

**FAIL.** 재작업 지침 (manager-spec):

1. **§3.1 #9 우선순위 문자열 정정** (spec.md:L75): `(AutoCompact > ReactiveCompact > Snip 우선순위)` → `(ReactiveCompact > AutoCompact > Snip 우선순위; 상세 분기 조건은 REQ-CTX-017/018 참조)`. REQ-CTX-008 L114, REQ-CTX-017 L120과 일관화.

2. **REQ-CTX-017 전용 AC 추가** (§5, 권장 위치 AC-CTX-015 다음):
   ```
   AC-CTX-016 — ReactiveTriggered 강제 ReactiveCompact 선택 (covers REQ-CTX-017)
   - Given DefaultCompactor{Summarizer: stubSummarizer, HistorySnipOnly: false},
           state.AutoCompactTracking.ReactiveTriggered = true,
           token usage 40_000/100_000 (= 40%, <80% 임계 미만),
           len(state.Messages) < state.MaxMessageCount
   - When Compactor.Compact(state)
   - Then CompactBoundary.Strategy == "ReactiveCompact", Summarizer 1회 호출됨.
           대조군: 같은 state에 ReactiveTriggered = false 로 바꿔 호출 시 Strategy 가
           "AutoCompact"가 아닌 "Snip"(40% < 80% 임계 미충족)이 되어 ReactiveTriggered 가
           AutoCompact 대비 ReactiveCompact를 강제하는 유일 조건임을 검증.
   ```

3. **REQ-CTX-018 명시적 AC 보강**: AC-CTX-005(L166-169)는 AutoCompact strategy 선택을 이미 검증하나 REQ-CTX-018 특유의 "ReactiveTriggered == false" 조건을 명시하지 않음. AC-CTX-005의 Given 절에 `state.AutoCompactTracking.ReactiveTriggered = false` 선조건을 추가하고 주석에 `(also covers REQ-CTX-018)`를 명시.

4. **REQ-CTX-013 섹션 재배치** (minor): §4.4 "Unwanted Behavior" 하에서 §4.1 "Ubiquitous" 로 이동. 섹션 heading과 라벨 일관화. 내용 변경 불필요.

5. **D12/D13/D15 minor 정리** (batch):
   - D12 (L325 DefaultCompactor.MaxMessageCount): 주석 추가 `// default applied when state.MaxMessageCount == 0; otherwise state value wins`.
   - D13 (AC-CTX-003 fixture): fixture 경로를 `internal/context/testdata/tokens/ko_en_mixed.json` 등 구체적으로 명시.
   - D15 (snip marker format): REQ-CTX-009에 `<moai-snip-marker>`의 auxiliary content schema(array 순서, role 값, block type 필드)를 명시하거나, "포맷 계약은 SPEC-GOOSE-ADAPTER-001 AC-ADAPTER-NNN이 정의" 를 명시적으로 위임.

재작업 후 iteration 3 감사 재요청 요망. 현재 iter2 만료 시 iter3에서도 D10/D11 미해소 시 stagnation 판정으로 상향.

---

**End of Report**

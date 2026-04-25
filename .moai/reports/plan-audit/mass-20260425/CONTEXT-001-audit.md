# SPEC-GOOSE-CONTEXT-001 감사 리포트

감사 대상: `.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md` (+ `research.md`)
감사 일자: 2026-04-25
감사자: plan-auditor (독립 감사, M1 Context Isolation 준수)
Iteration: 1/3

> 감사자 선언: 오케스트레이터/저자 reasoning context는 무시하고 spec.md + research.md 두 파일만을 evidence source로 사용했습니다 (M1). 다른 SPEC-GOOSE-* 문서 내용은 본 SPEC이 인용한 범위 내에서만 교차 참조했습니다.

---

## 요약

본 SPEC은 Claude Code의 Context 계층(SystemContext/UserContext/Compactor)을 Go로 이식하기 위한 인터페이스 계약을 명료하게 정의하고 있으며, EARS 5종 라벨과 Given/When/Then AC 구조는 대체로 잘 준수되었다. 그러나 **REQ 4건이 직접 대응하는 AC를 보유하지 않고**(REQ-CTX-011/013/015/016 → 고아 REQ) **AutoCompact와 ReactiveCompact의 trigger 구분이 REQ 수준에서 명시되지 않았으며**, spec §6.3 서명(CompactBoundary 무네임스페이스, §3.1은 `query.State`, §6.2/§6.3/research는 `loop.State`)에 **타입 이름/소속 패키지 불일치**가 존재한다. Traceability 고아가 MP 수준 must-pass이므로 **FAIL**.

---

## 축별 결과

### 1. EARS 준수

**결과: PASS (minor 1건)**

- 16개 REQ가 모두 `[Ubiquitous]/[Event-Driven]/[State-Driven]/[Unwanted]/[Optional]` 라벨을 부착하고 있으며 5종 모두 최소 1건씩 존재 (Ubiquitous 4, Event-Driven 6, State-Driven 2, Unwanted 3, Optional 1).
- shall/when/while/if/where 표지어 정확 사용 확인:
  - `REQ-CTX-005`: "When ... is called ... shall invoke" (spec.md:L106)
  - `REQ-CTX-011`: "While WarningLevel ... is Red ... shall return true" (spec.md:L120)
  - `REQ-CTX-014`: "If Summarizer.Summarize returns an error, then ..." (spec.md:L128)
  - `REQ-CTX-016`: "Where environment variable ... is set, ... shall be preferred" (spec.md:L134)
- 이슈(MINOR): `REQ-CTX-013 [Unwanted]` (spec.md:L126)은 Unwanted 라벨을 갖지만 "If ... then" 트리거 없이 "shall not return" 단일 불변식으로 서술되어 EARS Unwanted 패턴(조건부 불원 동작)의 형식에 정확히 부합하지는 않음. Ubiquitous 라벨이 더 적절할 수 있으나 의미 왜곡은 없음.

### 2. REQ↔AC 매핑 완전성

**결과: FAIL (고아 REQ 4건)**

REQ→AC 매핑표 (spec.md L92-189 기준):

| REQ | 대응 AC | 상태 |
|-----|--------|------|
| REQ-CTX-001 (SystemContext 멱등) | AC-CTX-001 | ✓ |
| REQ-CTX-002 (UserContext currentDate) | AC-CTX-002 | ✓ |
| REQ-CTX-003 (redacted_thinking 보존) | AC-CTX-006 | ✓ |
| REQ-CTX-004 (TokenCount 결정성) | AC-CTX-003 (부분) | △ 결정성 자체 검증 AC 부재 |
| REQ-CTX-005 (git 명령 호출) | AC-CTX-001 | ✓ |
| REQ-CTX-006 (CLAUDE.md walk) | AC-CTX-002 | ✓ |
| REQ-CTX-007 (ShouldCompact 80% 임계) | **없음** | ✗ 고아 |
| REQ-CTX-008 (Compact 전략 선택) | AC-CTX-005 | ✓ |
| REQ-CTX-009 (Snip protected window) | AC-CTX-006 | ✓ |
| REQ-CTX-010 (task_budget 불변) | AC-CTX-007 | ✓ |
| REQ-CTX-011 (Red 강제 트리거) | **없음** | ✗ 고아 |
| REQ-CTX-012 (Summarizer 등록 조건) | AC-CTX-008 (부분) | △ |
| REQ-CTX-013 (빈 Messages 금지) | **없음** | ✗ 고아 |
| REQ-CTX-014 (Summarizer 에러 fallback) | AC-CTX-009 | ✓ |
| REQ-CTX-015 (git 부재 graceful) | **없음** | ✗ 고아 |
| REQ-CTX-016 (HISTORY_SNIP env) | **없음** | ✗ 고아 |

Research.md §8.1–8.2은 TestShouldCompact_Over80Percent/RedLevel_Overrides/MaxMessageCount, TestSnip_MaintainsMinimumMessages, TestGetSystemContext_NoGit_ReturnsGraceful, TestCompact_HistorySnipOnlyFeatureGate 테스트 계획을 나열하지만 **SPEC의 §5 Acceptance Criteria 목록에는 대응 AC가 등록되어 있지 않다**. §5와 research.md 사이에 AC 정의 누락.

역방향(AC→REQ):

- AC-CTX-001..010 모두 최소 1개의 REQ에 추적 가능 → 고아 AC는 없음.

**결론**: 16개 REQ 중 4개(25%)가 대응 AC 부재 → 테스트 설계 시 누락 위험. MAJOR 결함.

### 3. AC 테스트 가능성

**결과: PASS**

- AC-CTX-001..010 (spec.md:L140-188) 모두 **Given/When/Then 3요소** 구조를 엄격히 따르며 **observable Then** 을 갖는다.
- 측정 가능한 기준 명시:
  - AC-CTX-001: "git 실행 횟수는 1회", "pointer-equal" (spec.md:L143)
  - AC-CTX-002: "±1초" 시각 오차 허용 (spec.md:L148)
  - AC-CTX-003: "providerGroundTruth ± 5%" (spec.md:L153)
  - AC-CTX-004: 4개 경계값(59999/60001/80001/92001)으로 enum 분기 검증 (spec.md:L158)
  - AC-CTX-006: protected head/tail 개수와 redacted_thinking 2개 보존 수치 명시 (spec.md:L166-168)
  - AC-CTX-007: "Remaining == 1234" 정수 동치 검증 (spec.md:L173)
- 모호 어휘("적절", "합리적", "충분히") 부재.

### 4. 스코프 일관성 (IN/OUT/Exclusions)

**결과: PASS**

- §3.1 IN SCOPE 14항목과 §3.2 OUT OF SCOPE, 말미 Exclusions 섹션(spec.md:L446-457)은 서로 모순 없음.
- "LLM 요약 실제 호출"은 §2.3, §3.2, Exclusions 세 곳에서 일관되게 COMPRESSOR-001 소관으로 유지.
- File watcher, 정확한 tokenizer, cross-session cache, redacted_thinking 해석 등 OUT 항목들이 Exclusions에서 구체적으로 재천명(단순 placeholder 아님).

### 5. 의존성 & 위험

**결과: PASS (minor 1건)**

- §7 Dependencies 표(spec.md:L393-402): CORE-001(선행), QUERY-001(선행), COMPRESSOR-001(후속), ADAPTER-001(후속), CONFIG-001(후속) 각각 제공/소비 인터페이스 명시. 양방향 화살표 방향 정확.
- Risks §8 (spec.md:L406-416) R1–R7은 각각 완화책을 구체 REQ ID 혹은 향후 SPEC으로 연결(예: R1→REQ-CTX-011, R4→REQ-CTX-015). 추적 가능.
- MINOR: R5(Summarizer task_budget 논리)의 완화책이 REQ-CTX-010 "compaction 자체 예산 불변" 선언으로 귀결되나, **Summarizer 호출 turn 이 QUERY-001에서 어떻게 계상되는지**는 본 SPEC이 보장할 수 없고 QUERY-001 해석에 의존 — cross-SPEC 책임 경계는 명시되었으나 integration test 포인트는 명시되지 않음.

### 6. MoAI 제약 (frontmatter / EARS / Exclusions)

**결과: PASS (minor 2건)**

- Frontmatter(spec.md:L1-13): id, version, status, created, updated, author, priority, issue_number, phase, size, lifecycle 11 필드 존재.
- EARS 5종 모두 사용(§4.1–§4.5).
- Exclusions 섹션 존재 및 8개 구체 항목 열거(spec.md:L446-457).
- MINOR #1: 표준 plan-auditor 스키마는 `created_at` 키를 요구하는데 본 SPEC은 `created`/`updated` 사용. 프로젝트 컨벤션일 가능성이 높으나 일관성 검증 필요.
- MINOR #2: `labels` 필드 부재. priority=P0는 존재하나 type/area 축의 label 이 frontmatter에 포함되지 않음.

### 7. 문서 자체 일관성

**결과: FAIL (MAJOR 2건)**

**결함 A — State 타입 소속 패키지 불일치**:

- §3.1 #9 (spec.md:L72): `ShouldCompact(s query.State) bool` — `query.State`
- §6.2 (spec.md:L296-297): `func (c *DefaultCompactor) ShouldCompact(s loop.State) bool` — `loop.State`
- §6.3 (spec.md:L319): `ShouldCompact(s loop.State) bool` — `loop.State`
- research.md:L326: `ShouldCompact(s loop.State) bool` — `loop.State`
- research.md:L344: "의존 방향은 `context → query/loop ← query`로 단방향" → `loop.State`가 `internal/query/loop/` 소유

3 vs 1 다수결로 `loop.State`가 정답으로 보이나, §3.1 #9의 `query.State`가 오탈자인지 의도적 축약인지 불명. 구현자에게 혼선 유발.

**결함 B — CompactBoundary 소속 패키지 불일치**:

- §6.2 (spec.md:L302-311): `internal/context/boundary.go` 에서 `type CompactBoundary struct` 정의 → context 패키지 소유
- §6.3 (spec.md:L322): `Compact(s loop.State) (loop.State, CompactBoundary, error)` — 네임스페이스 없음
- research.md:L328: `Compact(s loop.State) (loop.State, loop.CompactBoundary, error)` — **`loop.CompactBoundary`**

spec 본체는 CompactBoundary를 context 패키지에 두지만 research는 loop 패키지로 이동시킴. Circular import 방지 맥락(research:L344)에서 loop 쪽에 정의해야 계약이 성립하므로 **research가 맞고 spec §6.2가 틀렸을** 가능성이 높다. 이는 QUERY-001과의 인터페이스 계약에 직접 영향.

**결함 C — AutoCompact vs ReactiveCompact 구분 REQ 부재** (MAJOR):

- §3.1 #9, §3.1 #11에서 두 전략의 존재는 언급하나, **REQ 수준에서 "언제 AutoCompact vs ReactiveCompact 를 선택하는가"의 trigger 조건이 분리 명세되지 않음**.
- REQ-CTX-008은 "순서 AutoCompact → ReactiveCompact → Snip"만 규정.
- REQ-CTX-007은 `state.AutoCompactTracking.ReactiveTriggered` 플래그를 참조하지만 이 플래그가 언제 true가 되는지(ReactiveCompact 트리거 조건 자체)는 본 SPEC이 정의하지 않음.
- research.md도 ReactiveCompact 테스트(`TestReactiveCompact_TriggeredByFlag`)만 언급할 뿐 flag 세팅 주체와 조건 미정의.
- **특별 집중 사항인 "3개 알고리즘이 분리된 REQ로 정의됐는가"에 대해 Snip(REQ-CTX-009)만 분리 명세, Auto/Reactive는 혼합** → 명세 gap.

**REQ/AC 번호 연속성**:
- REQ-CTX-001 ~ REQ-CTX-016 연속, 3자리 zero-padding 일관.
- AC-CTX-001 ~ AC-CTX-010 연속, 3자리 zero-padding 일관.

---

## Must-Pass 결함

- **MP (Traceability)**: REQ-CTX-007 / REQ-CTX-011 / REQ-CTX-013 / REQ-CTX-015 / REQ-CTX-016 — 총 **5건** 의 REQ 가 대응 AC 부재. research.md §8은 테스트 계획만 나열할 뿐, SPEC §5 AC 목록에 정식 등록되지 않음. 구현자가 테스트 설계할 때 §5를 따르면 이들 REQ의 검증이 체계적으로 누락됨. **MP-FAIL**.
- **MP (Cross-SPEC 인터페이스 일관성)**: CompactBoundary/State 타입의 소속 패키지가 spec 본체와 research, spec §3.1/§6.2/§6.3 사이에 불일치. QUERY-001과의 계약 서명이 확정되지 않음. **MP-FAIL**.
- **MP (알고리즘 specificity)**: AutoCompact와 ReactiveCompact의 분리 trigger REQ 부재. 특별집중 검증 요구 사항 미충족. **MP-FAIL**.

## Minor Observations

1. `REQ-CTX-013 [Unwanted]` 형식(spec.md:L126)은 "If ... then" 트리거 없이 순수 불변식이므로 Ubiquitous 분류가 더 적합.
2. Frontmatter가 `created_at` 대신 `created`를 사용하고 `labels` 필드 부재(spec.md:L1-13). 프로젝트 규약일 가능성.
3. R5(Summarizer 예산 소비) integration test 포인트가 명시되지 않음 — QUERY-001과의 경계 계약 검증 지점 불명(spec.md:L414).
4. AC-CTX-003의 "providerGroundTruth fixture"가 어디에 어떻게 저장되는지 명시 안됨(spec.md:L152-153) — 테스트 reproducibility 이슈.
5. REQ-CTX-007의 `state.MaxMessageCount` 필드는 §6.2의 `DefaultCompactor.MaxMessageCount`와 이름이 동일하나 owner(state vs compactor)가 다름. 구현자 혼선 가능(spec.md:L110 vs L291).
6. `<moai-snip-marker>`(spec.md:L114)의 auxiliary content format은 R2(Risk)에서 "고" 영향으로 식별되었지만 REQ에 포맷 불변식이 직접 명문화되지 않음 — ADAPTER-001 integration에 의존.

---

## 결함 집계

- Critical: 0
- Major: 3 (고아 REQ 5건 집합 / State·CompactBoundary 타입 불일치 / Auto·Reactive 구분 REQ 부재)
- Minor: 6

---

## Verdict

**FAIL**

사유: (1) 16개 REQ 중 5개가 대응 AC 부재로 **traceability must-pass 위반**, (2) QUERY-001과의 인터페이스 계약에 직결되는 `State`/`CompactBoundary` 타입 소속 패키지가 spec 내부 및 research와 불일치, (3) 3개 compaction 알고리즘 중 Snip만 분리된 REQ로 명세되고 AutoCompact/ReactiveCompact의 분기 조건은 REQ 수준에 누락. EARS 형식 준수와 AC 테스트가능성은 우수하나 위 세 MP 결함이 구현 단계에서 스펙 재작성을 강요한다.

## 권고 (manager-spec 재작업 지시)

1. **신규 AC 추가** (§5):
   - `AC-CTX-011` — ShouldCompact 80% 임계 경계 테스트 (REQ-CTX-007 커버)
   - `AC-CTX-012` — WarningRed 상태에서 ShouldCompact 강제 true (REQ-CTX-011 커버)
   - `AC-CTX-013` — Compact 결과 Messages ≥ ProtectedTail+1 불변식 (REQ-CTX-013 커버)
   - `AC-CTX-014` — git 부재 시 `GitStatus == "(no git)"` graceful path (REQ-CTX-015 커버)
   - `AC-CTX-015` — `GOOSE_HISTORY_SNIP=1` 환경에서 Summarizer 등록돼도 Snip 선택 (REQ-CTX-016 커버)
2. **타입 소속 패키지 확정** (§6.2/§6.3):
   - `State` 는 `loop.State`(research.md §9 근거)로 통일. spec §3.1 #9의 `query.State`를 `loop.State`로 수정.
   - `CompactBoundary` 는 `internal/query/loop/` 소유로 이동 시키고 §6.2 레이아웃에서 `boundary.go` 항목 삭제 또는 "`loop.CompactBoundary` re-export wrapper only"로 축소. §6.3 서명 `(loop.State, loop.CompactBoundary, error)` 명시.
3. **신규 REQ 추가** (§4.2):
   - `REQ-CTX-008a [Event-Driven]` — **When** `state.AutoCompactTracking.ReactiveTriggered == true` **and** token usage < 80%, `Compact` **shall** select `ReactiveCompact` in preference to `AutoCompact` (혹은 반대 — 의도한 우선순위 정의 필요).
   - `REQ-CTX-008b [Event-Driven]` — AutoCompact strategy 자체의 내부 계약 (Summarizer 호출 횟수, target token reduction 비율 등).
4. **REQ-CTX-013 라벨 정정**: `[Unwanted]` → `[Ubiquitous]` (조건부 트리거 없음).
5. **R5 완화 검증 지점 추가** (§8): QUERY-001 integration test 중 Compact 전후 `TaskBudget.Remaining` 회계 assertion을 명시.
6. **Frontmatter 보강**: `labels: [area/runtime, type/feature, priority/p0-critical]` 추가 권장 (CLAUDE.local.md §1.5 label 체계 반영).

재작업 후 iteration 2 감사 재요청 요망.

---

## Chain-of-Verification Pass

2차 재검 결과(감사자 자가검증):

- **REQ 16개 전수 검토**: 1차에서 4건만 고아로 식별했으나 재검 시 REQ-CTX-007도 "80% 임계 자체를 직접 측정하는 AC"가 없음을 확인 → 고아 5건으로 수정.
- **AC 10개 전수 검토**: AC-CTX-008은 REQ-CTX-012(Summarizer 등록 조건)을 "역방향"으로만 검증(미등록 → Snip) → 등록된 정상 경로 검증 AC는 AC-CTX-005 하나뿐. 부분 커버 유지.
- **CompactBoundary 패키지 재검**: spec §6.2 boundary.go 정의와 research §9 `loop.CompactBoundary` 명시 재확인 → 문서 내부 충돌 확정.
- **Exclusions 구체성**: 8개 항목 모두 "XXX-001 소관" 또는 "raw concat" 같은 구체적 위임/무처리 명시 → PASS 유지.
- **AutoCompact·ReactiveCompact 분리 REQ**: §4.2 재독 후에도 독립 REQ 없음 확인.
- **새로 발견된 결함 없음 외에 위 5건 고아 판정이 1차(4건)보다 더 엄격함** → defect list 갱신 반영.

## Regression Check

N/A — Iteration 1.

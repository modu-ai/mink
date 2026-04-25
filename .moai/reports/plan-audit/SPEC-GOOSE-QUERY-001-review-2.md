# SPEC-GOOSE-QUERY-001 감사 리포트 — iteration 2

> Reasoning context ignored per M1 Context Isolation. 본 감사는 spec.md / plan.md / acceptance.md / spec-compact.md 최종 텍스트만을 근거로 수행되었으며, review-1.md 는 결함 목록 조회 용도로만 사용되었다.

## 요약

iteration 1 의 9개 Must-Pass 결함(D1~D9) 은 **모두 해결**되었다. AC-QUERY-013~016 가 신설되었고, REQ-010 단계 순서는 yield→close 로 교정, REQ-015/016 라벨은 `[Ubiquitous]` 로 재분류, `SDKMsgPermissionCheck` 타입이 §6.2 enum 에 추가, §6.1 파일 트리에 `state.go` 가 포함, L417 공백 오타가 제거되었다. turnCount 증분 모델(§6.3) 은 경로 A/B 로 재정의되어 AC-001/AC-002 의 산술이 성립한다. Minor observation M2/M3/M5 세 항목도 모두 수용되었다.

그러나 **수정 과정에서 파생 문서 동기화가 완전하지 않아 새 결함 3건(D10'~D12') 이 도입**되었다. 모두 plan.md/frontmatter 의 숫자 정합성 문제로 구현자에게 잘못된 상한을 지시할 수 있어 Must-Pass 관점의 scope 경계를 넘는다.

- D1~D9: **9/9 해결**
- 신규 Must-Pass/Major 결함: **3건**
- Minor observation (미해결 + 신규): **3건 (M4 미해결, 파생 문서 updated 일자, AC-003 permission_check{deny} 관찰 누락)**

## Regression Check — D1~D9 각각 해결 상태

### D1 (REQ-013 AC 고아) → 해결됨
- spec.md L212-215 + acceptance.md L230-250 에 **AC-QUERY-013** 신설.
- 테스트 시그니처 `func TestQueryLoop_AskPermission_SuspendResume(t *testing.T)` (build tag `integration`) 명시 (acceptance.md L242).
- Payload recorder 로 suspend 기간 "LLM call 건수 1건 고정" 확증(acceptance.md L236), StubExecutor fail-on-early-call guard(L246) 포함.
- Edge case: resolve_deny / cancel_while_pending / multiple_asks_fifo 3개 서브테스트 명시.

### D2 (REQ-016 AC 고아) → 해결됨
- spec.md L217-220 + acceptance.md L254-277 에 **AC-QUERY-014** 신설.
- N=1000 반복 + p99 ≤ 10ms 기준, `t0 := time.Now(); SubmitMessage; t1 := time.Now()` 측정 패턴 정량화(acceptance.md L258-265).
- 성능 회귀 실패 시 CI 재시도 금지 명시 — 허용 가능.

### D3 (REQ-018/REQ-020 AC 고아) → 해결됨
- **AC-QUERY-015** (PostSamplingHooks FIFO): spec.md L222-225 + acceptance.md L281-300. h1/h2 append 관찰, 순서 민감성 서브테스트(`fifo_h1_then_h2` vs `fifo_h2_then_h1`).
- **AC-QUERY-016** (TeammateIdentity): spec.md L227-230 + acceptance.md L304-320. system header + meta 2경로 payload recorder 검증, nil_identity 네거티브 서브테스트.

### D4 (REQ-015/REQ-016 EARS 라벨) → 해결됨
- spec.md L132 `REQ-QUERY-015 [Ubiquitous]`, L134 `REQ-QUERY-016 [Ubiquitous]`.
- §4.4 섹션 제목도 "Unwanted Behavior / Ubiquitous Prohibition" 으로 업데이트, 라벨 교정 사유 주기(L128) 포함.
- REQ 번호 안정성 유지 확인(001..020 연속, 중복 0).

### D5 (turnCount 증분 모델) → 해결됨
- spec.md §6.3 L405-420 에 경로 A (pure assistant terminal, `after_assistant_terminal` inline) 와 경로 B (tool roundtrip, `after_tool_results`) 명시.
- AC-001 (tool 없음) = 1 (경로 A 1회), AC-002 (tool 1 roundtrip + 후속 stop) = 2 (경로 B 1회 + 경로 A 1회) 산술 성립.
- 요약 문장(L420): "State 재할당 경로(continue site) 는 3곳으로 불변이나, turnCount 증분은 iteration 완료 시점에 발생" — REQ-QUERY-003 의 "State 재할당 3곳" 약속을 깨지 않음.

### D6 (permission_check SDKMessage) → 해결됨
- spec.md §6.2 L361 `SDKMsgPermissionCheck SDKMessageType = "permission_check"` enum 추가.
- REQ-QUERY-006 (L108) 에서 Allow → `permission_check{behavior:"allow"}` yield, Deny → `permission_check{behavior:"deny", reason}` yield 명시.
- AC-QUERY-002 관찰 기대(`permission_check{allow}`) 와 정합.
- (부분 관찰): AC-QUERY-003 (acceptance.md L56-67) 에는 `permission_check{deny}` SDKMessage 관찰 기대가 여전히 없고 `ToolResult` 합성만 검증 — REQ-006 의 Deny yield 요구가 AC 에서 관찰되지 않음(Minor gap, Major 미해당).

### D7 (REQ-010 순서) → 해결됨
- spec.md L116-117: "(a) stop consuming LLM chunks, (b) release any pending tool permissions, (c) yield a `Terminal{...error:"aborted"}` `SDKMessage`, (d) close the output channel".
- yield → close 순서 강제(주기 L116), 닫힌 채널 send 패닉 회피.
- AC-QUERY-008 (spec.md L187-190) "채널이 close 되고, 마지막 `SDKMessage` 가 `Terminal{...aborted}`" 와 정합 — range drain 시 close 전 마지막 send 가 Terminal 이 되는 순서.

### D8 (L417 공백 오타) → 해결됨
- spec.md L451 `TestQueryLoop_MaxOutputTokens_Retries3ThenFails` 공백 제거.
- 동일 식별자가 acceptance.md L95, plan.md T4.3 과 일치.

### D9 (§6.1 파일 트리 `state.go`) → 해결됨
- spec.md L247 `│       ├── state.go              # State, Continue, Terminal 타입` 행 추가.
- §6.2 코드 주석 "// internal/query/loop/state.go" 및 plan.md §5.1 L191 과 정합.

**Regression summary: 9/9 해결.**

---

## 신규 결함 (iteration 2 수정 과정에서 도입)

### D10' — spec.md frontmatter `version` vs HISTORY 엔트리 불일치 (Major, 문서 자체 일관성)

- **파일:행**: spec.md L3 `version: 0.1.0`, HISTORY L22 `| 0.1.1 | 2026-04-25 | ... | manager-spec |`.
- **문제**: iteration 2 수정 사항을 HISTORY 에 `0.1.1` 엔트리로 기록했으나 frontmatter `version` 필드는 여전히 `0.1.0`. 독자는 "현재 SPEC 버전" 을 frontmatter 로 판정하므로 HISTORY 와 불일치한다. MoAI SPEC 규약 상 frontmatter 는 현재 버전을 반영해야 한다. spec.md 의 자기 일관성 FAIL.
- **영향**: 파생 산출물(SPEC 인덱스, SPEC 변경 추적 보고서, PR 자동화) 이 frontmatter 를 기준으로 삼을 경우 iteration 2 수정 사실이 누락된다.
- **수정 제안**: spec.md frontmatter `version: 0.1.1` + `updated: 2026-04-25` 로 교정. 파생 문서(plan.md L3 `version: 0.1.0` + L6 `updated: 2026-04-24`, acceptance.md L3 `version: 0.1.0` + L6 `updated: 2026-04-24`) 도 iteration 2 반영이 필요하다면 동기 업데이트.

### D11' — plan.md 에서 `SDKMessage` enum/payload 개수 "9개" 고정 (Major, 구현 misdirection)

- **파일:행**: plan.md L34 (T0.1) "`SDKMessage{Type,Payload}`, `StreamEvent` 타입 선언만 ... Type enum **9개** (spec.md §6.2).", plan.md L46 (T1.2) "`SDKMessage` payload 구조체 **9종** + exhaustive type-switch helper".
- **현재 상태**: spec.md §6.2 L353-365 에 `SDKMsgPermissionCheck` (D6 수정 결과) 를 포함하여 **10개** 상수 선언. plan.md 는 9개로 고정된 과거 수치.
- **문제**: 구현자가 plan.md 를 근거로 T0.1 에서 enum 9개, T1.2 에서 payload 9종만 생성하면 `SDKMsgPermissionCheck` 및 그 payload 구조체가 누락된다. AC-QUERY-002 (`permission_check{allow}`) 의 관찰이 테스트에서 실패한다. 또한 `TestSDKMessage_TypeSwitchExhaustive` 가 "9종 exhaustive" 로 작성되면 10번째 타입을 표현할 길이 닫힌다.
- **영향**: AC-002/AC-003 연쇄 RED 실패, REQ-QUERY-006 구현 누락.
- **수정 제안**: plan.md L34 "Type enum **10개**", L46 "`SDKMessage` payload 구조체 **10종**" 으로 수정. 혹은 "spec.md §6.2 에 정의된 모든 타입" 으로 숫자 하드코딩 제거.

### D12' — plan.md §7 TRUST 5 Tested 체크리스트 "Integration test 12개" (Major, 구현 misdirection)

- **파일:행**: plan.md L265 "Integration test **12개(AC-QUERY-001~012)** 전부 GREEN (build tag `integration`)".
- **현재 상태**: AC 가 16개로 확장되었고 plan.md §8 Quality Gate L308, §11 Definition of Done L354 는 모두 "AC-QUERY-001 ~ 016" 로 업데이트됨. 그러나 §7 TRUST 5 Tested 체크리스트만 구 수치 12 로 남아있다.
- **문제**: 구현자가 §7 체크리스트를 종료 기준으로 해석하면 AC-013~016 integration test 를 누락해도 TRUST 5 Tested 를 PASS 로 착각한다. plan.md 내부 자기 불일치 (§7 vs §8/§11).
- **영향**: AC-QUERY-013/014/015/016 GREEN 여부가 완료 기준에서 실수로 제외될 위험. D1~D3 수정 효과가 품질 게이트 레벨에서 약화됨.
- **수정 제안**: plan.md L265 "Integration test **16개(AC-QUERY-001~016)** 전부 GREEN" 으로 교정.

---

## Minor Observation 수용 확인 (M2/M3/M5)

### M2 (REQ-008 `after_compact` reset 조항) → 수용됨
- spec.md L112 REQ-008 후반부 "Additionally, at the `after_compact` continue site (REQ-QUERY-009), the `queryLoop` **shall** reset `State.maxOutputTokensRecoveryCount` to 0 because the post-compaction context is materially different and deserves a fresh retry budget." 삽입.
- spec-compact.md L34 에도 축약판 반영 ("At the `after_compact` continue site (REQ-QUERY-009), the `queryLoop` **shall** reset `maxOutputTokensRecoveryCount` to 0").
- §6.3 continue site 표 L415 `after_compact` 행의 새 State 구성에 "`maxOutputTokensRecoveryCount` → 0 (REQ-QUERY-008 reset 조항)" 명시.
- (관찰): REQ-008 은 여전히 `[Event-Driven]` 라벨. "Additionally" 로 도입된 두 번째 문장은 Ubiquitous 불변식에 가까우나 같은 REQ 내부 추가 조항이므로 허용 가능. 정량적으로 R5 가 AC-QUERY-011 edge case (`TestQueryLoop_RetryCounter_ResetsAfterCompact`, acceptance.md L208, plan.md T4.3/T4.4) 로 잠금되어 실용적 문제 없음.

### M3 (spec-compact REQ-014 원문 "with the error details" 복원) → 수용됨
- spec-compact.md L48 "yield a final `SDKMessage` of type `error` **with the error details**" — 누락되었던 어절 복원. spec.md L130 원문과 일치.

### M5 (spec-compact frontmatter 보강) → 수용됨
- spec-compact.md L1-10 frontmatter 에 `id`, `version`, `status`, `created`, `updated: 2026-04-25`, `author`, `priority: P0`, `issue_number: 5` 전부 포함. spec.md frontmatter 와 필수 필드 집합 일치(updated 는 iteration 2 반영).

---

## 축별 재검증 결과

### 1. EARS 준수

- REQ-006 [Event-Driven]: "When ... the `queryLoop` shall invoke CanUseTool ... and dispatch based on ..." — 정형. ✓
- REQ-010 [Event-Driven]: "When ctx.Done() fires, the queryLoop shall (a)(b)(c)(d) within 500ms." — yield → close 순서로 재배치된 문장, 정형. ✓
- REQ-015/016 [Ubiquitous]: `shall not` 시작, 트리거 없음 — Ubiquitous prohibition 정형. 단 §4.4 섹션 제목 "Unwanted Behavior / Ubiquitous Prohibition" 아래에 Ubiquitous 라벨이 함께 존재하는 것은 구조적으로 혼합이다. 주기(L128)가 사유를 설명하지만, §4.1 로의 이동이 더 깔끔했을 것. Minor observation (아래 참조).
- REQ-008: 두 번째 문장("Additionally, ... shall reset ...") 은 `at the after_compact continue site` 를 트리거처럼 읽을 수 있어 Event-Driven/State-Driven 성격 혼합. 그러나 원문 REQ 확장이므로 허용. ✓

**축 결과**: **PASS** (Minor structural 관찰 1건, §4.4 섹션 vs [Ubiquitous] 라벨 혼합).

### 2. REQ ↔ AC 매핑 완전성

| REQ | AC 커버 | 비고 |
|-----|--------|------|
| REQ-001 | AC-004 | ✓ |
| REQ-002 | AC-001, AC-014 (streaming 측) | ✓ |
| REQ-003 | AC-001/002/005 (continue site 간접) | ✓ |
| REQ-004 | AC-004 | ✓ |
| REQ-005 | AC-001 | ✓ |
| REQ-006 | AC-002, AC-003, AC-013 | ✓ |
| REQ-007 | AC-009 | ✓ |
| REQ-008 | AC-005 + AC-011 edge case (counter reset) | ✓ |
| REQ-009 | AC-011 | ✓ |
| REQ-010 | AC-008, AC-013 edge (`cancel_while_pending`) | ✓ |
| REQ-011 | AC-001/006/007 | ✓ |
| REQ-012 | AC-010 | ✓ |
| REQ-013 | **AC-013** | ✓ NEW |
| REQ-014 | AC-003 경계 + plan T6.2 | weak but acceptable |
| REQ-015 | AC-008, AC-013, race detector gate (plan §8) | ✓ |
| REQ-016 | **AC-014** | ✓ NEW |
| REQ-017 | AC-010 edge case | ✓ |
| REQ-018 | **AC-015** | ✓ NEW |
| REQ-019 | AC-012 | ✓ |
| REQ-020 | **AC-016** | ✓ NEW |

모든 REQ(20) 가 AC(16) 또는 품질 게이트로 커버. 역방향: 16개 AC 모두 유효 REQ 매핑.

**축 결과**: **PASS**.

### 3. AC 테스트 가능성

- AC-013~016 신설분: Given/When/Then 이 구체적 Go 코드로 변환 가능 수준. payload recorder, timing 측정 헬퍼, FIFO 체인 관찰 패턴까지 지정.
- AC-014 10ms p99 기준 N=1000 은 성능 회귀 감지로 수용 가능하나, CI 재시도 금지 정책이 flaky 리스크를 수반한다. 의도적 정책(acceptance.md L277).
- AC-006 (budget) vs REQ-011: acceptance.md L110 "Remaining - 60 < 0 감지" 와 REQ-011 "remaining <= 0" 불일치. review-1 M4 로 지적, iteration 2 에서 미수정 → 여전한 자기 모순. 단 독립적 Minor.
- AC-003 에서 REQ-006 Deny 분기의 `permission_check{deny}` SDKMessage yield 가 관찰 대상에 포함되지 않음. REQ 측 yield 요구만 있고 AC 측 관찰은 `ToolResult` 합성만 검증. Minor gap.

**축 결과**: **PASS with 2 Minor observations** (AC-006 budget gate, AC-003 permission_check{deny} 관찰 누락).

### 4. 스코프 일관성

- IN SCOPE / OUT OF SCOPE / Exclusions 변경 없음. iteration 2 수정은 기존 스코프 내부 정합성 보수.
- §6.1 파일 트리 `state.go` 추가로 §6.2 코드 주석 / plan.md §5.1 와 정합.
- spec-compact.md "Files to Modify" 트리(L148-159) 에도 `state.go` 포함. ✓
- `internal/query/testsupport/stubs.go` 는 spec.md §6.1 에는 여전히 부재이나 plan.md §5.1 + spec-compact.md 보조 파일 주석에 명시됨. 테스트 헬퍼이므로 정당한 확장. ✓ (review-1 판정 유지)

**축 결과**: **PASS**.

### 5. 의존성 & 위험

- §7 의존성 표, §8 R1-R7 변경 없음. R5(max_output_tokens counter reset) 는 REQ-008 에 정식 편입(M2 수용). plan.md R5 항목 L166 "`after_compact` continue site 에서 counter → 0" 과 정합.
- R7 (10ms SubmitMessage) 가 AC-014 로 AC 승격되어 검증 경로가 명확해짐.

**축 결과**: **PASS**.

### 6. MoAI 제약 준수

- spec.md Exclusions 10개 항목 유지.
- EARS 5종 모두 사용 (Ubiquitous 4개 + Event-Driven 6개 + State-Driven 3개 + Unwanted 2개 + Optional 3개 + Ubiquitous prohibition 2개). REQ 수 20개 연속.
- spec.md frontmatter 필수 필드(id/version/status/created/updated/author/priority/issue_number) 모두 존재. (D10' version 값 일관성 별도 기록)
- spec-compact.md frontmatter 도 필수 필드 보강 완료(M5).

**축 결과**: **PASS** (D10' version 값 일관성은 별도 Must-Pass/Major 결함으로 기록).

### 7. 문서 자체 일관성

- spec.md 내부: §6.1 트리 / §6.2 코드 주석 / §6.3 continue site 표 정합. REQ-008 reset 조항 ↔ §6.3 `after_compact` 행 ↔ R5 완화 전략 정합.
- spec.md ↔ acceptance.md: AC-001~016 전 항목 Given/When/Then 정합. turnCount 증분 모델 §6.3 이 AC-001/AC-002 설명과 일치.
- spec.md ↔ plan.md: RED #1~#16 번호 매핑 정합. 그러나 **plan.md enum 9개 기록(D11')** 및 **§7 Integration test 12개 기록(D12')** 은 spec.md §6.2 / §5 (AC 16개) 와 불일치. 새 결함으로 분리.
- spec.md ↔ spec-compact.md: REQ 20개, AC 16개, Files 트리(`state.go` 포함), Exclusions 10개 전부 동기화. ✓
- HISTORY 0.1.1 엔트리 vs frontmatter 0.1.0: **D10' 로 기록**.

**축 결과**: **FAIL** (D10' + D11' + D12' 는 문서 자체 일관성 측).

---

## Must-Pass 결함

**3건** (모두 iteration 2 수정 과정에서 신규 도입).

- **D10'** — spec.md frontmatter `version: 0.1.0` vs HISTORY `0.1.1` 엔트리 불일치 (Major, 문서 자체 일관성).
- **D11'** — plan.md T0.1/T1.2 의 `SDKMessage` enum/payload "9개" 수치가 spec.md §6.2 10개와 불일치 (Major, 구현 misdirection).
- **D12'** — plan.md §7 TRUST 5 Tested 체크리스트 "Integration test 12개(AC-QUERY-001~012)" 가 AC 16개 확장과 불일치 (Major, 품질 게이트 misdirection).

---

## Minor Observations

- **M4 (미해결, review-1 이월)** — AC-QUERY-006 의 budget gate 표현("2턴차 시작 시점에서 `Remaining - 60 < 0` 감지", acceptance.md L110) 은 REQ-QUERY-011 의 `remaining <= 0` 과 다른 predictive 알고리즘. iteration 2 에서 미수정. 두 문서가 같은 알고리즘을 지칭한다는 확신이 여전히 약하다. 수정 제안(review-1 M4 재인용): AC-006 의 Given 을 "이미 소진된 예산 상태에서 추가 turn" 으로 재구성하거나, REQ-011 의 gate 를 "현재값 + 다음 turn 예상 소비 ≤ 0" 으로 명시.
- **AC-QUERY-003 에서 `permission_check{deny}` SDKMessage 관찰 누락** — REQ-QUERY-006 은 Deny 분기에서 `permission_check{deny, reason}` yield 를 요구하지만 AC-003 Then 은 `ToolResult` 합성만 검증. D6 수정으로 enum/REQ 는 정합하나 AC 관찰 그물이 일부 성김. 제안: AC-003 Then 에 "채널 순서에 `permission_check{tool_use_id, behavior:"deny", reason:"destructive"}` 가 포함됨" 한 줄 추가.
- **파생 문서 updated 필드 이월** — acceptance.md L6 `updated: 2026-04-24`, plan.md L6 `updated: 2026-04-24` 는 iteration 2 수정(2026-04-25) 반영 안 됨. spec-compact.md 만 L6 `updated: 2026-04-25` 로 갱신. 동기화 필요.
- **§4.4 섹션 제목 vs [Ubiquitous] 라벨 혼합** — "Unwanted Behavior / Ubiquitous Prohibition" 섹션 아래 REQ-015/016 [Ubiquitous] 라벨이 포함된 구조는 주기로 설명되나 독자 혼동 여지. REQ 번호 안정성 유지 목적은 달성했으므로 수용 가능하나, 향후 version bump 시 §4.1 로 이동 권장.

---

## Chain-of-Verification Pass

2차 패스에서 다음을 재확인:

1. REQ-QUERY-001~020 번호 연속성 재전수: 001..020 gap 0, 중복 0. ✓
2. AC-QUERY-001~016 번호 연속성 재전수: 001..016 gap 0, 중복 0. ✓
3. `SDKMsgPermissionCheck` 를 §6.2 enum, REQ-006 본문, AC-002 관찰, spec-compact L71 모두에서 재조회. 네 곳 전부 존재 및 정합. ✓
4. REQ-010 단계 (a)~(d) 를 spec.md / spec-compact.md 양쪽에서 re-read. yield → close 순서가 두 문서 모두에서 유지됨. ✓
5. turnCount 증분: AC-001 (1), AC-002 (2), AC-004 (누적), AC-007 (MaxTurns=2 → 2턴 실행) 를 §6.3 경로 A/B 정의로 모두 재산술. 모순 없음. ✓
6. plan.md 에서 AC 개수 언급 그렙: "12개" 3회 발견 — L265 (§7 Tested), L354 Definition of Done 은 "001~016" 으로 수정됨, L308 Quality Gate 는 "001~016" 으로 수정됨. §7 L265 만 미수정 → D12' 확증. 추가로 §7 의 "패키지당 파일 LoC 평균 ≤ 200" 등 품질 기준은 AC 수 확장과 무관하므로 영향 없음.
7. plan.md 에서 "9개" / "9종" 그렙: L34, L46 두 곳 발견 → D11' 확증.
8. frontmatter `version` 필드 재확인: spec.md L3 `0.1.0`, HISTORY L22 `0.1.1`. plan.md L3, acceptance.md L3, spec-compact.md L3 모두 `0.1.0`. D10' 확증.
9. §4.4 섹션 제목("Unwanted Behavior / Ubiquitous Prohibition") 아래 [Ubiquitous] 라벨 혼합 — 의도된 구조로 판단(L128 주기), Minor 수준 유지.
10. acceptance.md "성능/품질 게이트" 표(L324-337) 가 AC-014 를 포함하도록 업데이트됨. Integration 16개 명시됨. ✓

추가 발견 없음. 위 3개 Must-Pass/Major 결함이 iteration 2 의 완전한 신규 결함 집합.

---

## 종합 판정

Verdict: FAIL

# SPEC-GOOSE-QUERY-001 감사 리포트 — iteration 1

> Reasoning context ignored per M1 Context Isolation. 본 감사는 spec.md / plan.md / acceptance.md / spec-compact.md / research.md 5개 파일의 최종 텍스트만을 근거로 수행되었다.

## 요약

SPEC은 EARS 5종을 모두 사용하고, 파일 레이아웃·RED 번호·MX 태그 계획까지 매우 구체적으로 기술되어 있다. 그러나 **(1) REQ ↔ AC 매핑에 최소 3개의 고아 REQ(REQ-013/018/020) 존재**, **(2) `turnCount` 증분 모델이 문서 내에서 자기모순(§6.3 vs AC-001/AC-002)**, **(3) AC-002가 요구하는 `permission_check{allow}` SDKMessage가 타입 enum(§6.2)과 REQ-006 어디에도 정의되지 않음**, **(4) REQ-015/016이 [Unwanted]로 라벨링되었지만 "If...then" 구조 부재(라벨 오분류)**, **(5) spec.md L417 테스트 함수명 오타(`Retries3Then Fails` — Go 식별자에 공백 불가)** 등 must-pass 축에서 다수 결함이 발견되었다. 범위·의존성·Exclusions은 정합하지만 테스트 가능성과 자기 일관성이 자체 결함으로 실패.

---

## 검사 축별 결과

### 1. EARS 준수

**전반 PASS with exceptions**:

- REQ-001~004 (Ubiquitous): 명확, `shall` 정상 normative. ✓
- REQ-005~010 (Event-Driven): "When X, Y shall Z" 구조 모두 충족. ✓
- REQ-011 (State-Driven): **복합 문장**. "While X, queryLoop shall continue" 뒤에 "reaching == maxTurns shall transition", "remaining <= 0 shall transition" 두 전이를 한 REQ에 묶음. 사실상 REQ-011a/b/c 3건을 1건으로 합성. 테스트 가능성은 남지만 "하나의 REQ = 하나의 shall" 원칙 위반 소지.
- REQ-012, REQ-013 (State-Driven): ✓ 정형.
- **REQ-014 (Unwanted)**: "If panic or 4xx, then loop shall (a)(b)(c)(d)" — EARS Unwanted 패턴 충족. ✓
- **REQ-015 (Unwanted, 라벨 오분류)**: spec.md L129 "The QueryEngine **shall not** write to `State.messages` from goroutines other than..." — "If [undesired condition]" 트리거 없음. 이는 **Ubiquitous prohibition**이지 Unwanted가 아님. 라벨 오분류.
- **REQ-016 (Unwanted, 라벨 오분류)**: spec.md L131 "The QueryEngine.SubmitMessage **shall not** block the caller for longer than 10ms..." — 동일하게 "If" 트리거 없음. Ubiquitous 성능 불변식이지 Unwanted가 아님.
- REQ-017 (Unwanted): "If tool name not present, then loop shall synthesize..." ✓ 정형.
- REQ-018~020 (Optional): "Where X, Y shall Z" 구조 충족. ✓

### 2. REQ ↔ AC 매핑

**FAIL — 고아 REQ 존재**.

spec.md §5는 AC-QUERY-001~012 (12개)만 정의한다. acceptance.md의 명시 매핑 + spec.md AC 본문을 대조하면:

| REQ | AC 커버 | 비고 |
|-----|--------|-----|
| REQ-001 | AC-004 | ✓ |
| REQ-002 | AC-001 | ✓ |
| REQ-003 | AC-002, AC-005, AC-011 | ✓ |
| REQ-004 | AC-004 | ✓ |
| REQ-005 | AC-001 | ✓ |
| REQ-006 | AC-002, AC-003 | ✓ |
| REQ-007 | AC-009 | ✓ |
| REQ-008 | AC-005, AC-011(보조) | ✓ |
| REQ-009 | AC-011 | ✓ |
| REQ-010 | AC-008 | ✓ |
| REQ-011 | AC-001, AC-002, AC-006, AC-007 | ✓ |
| REQ-012 | AC-010 | ✓ |
| **REQ-013** | **없음** | plan.md T8.2 `TestQueryEngine_ResolvePermission_ResumesLoop` 단위 테스트만 존재. AC-QUERY-xxx 레벨 미커버. |
| REQ-014 | AC-003 (경계 주석만) | 약한 매핑. 부정적 경로는 plan T6.2에만. |
| REQ-015 | AC-008 | 약하게 경유. |
| **REQ-016** | **없음** | acceptance.md "성능/품질 게이트" 표에서만 언급. 별도 AC 없음(unit test만). |
| REQ-017 | AC-010 (edge case) | ✓ 보조 커버. |
| **REQ-018** | **없음** | plan.md T8.1 unit test만. |
| REQ-019 | AC-012 | ✓ |
| **REQ-020** | **없음** | plan.md T8.3 unit test만. |

**결론**: REQ-013, REQ-016, REQ-018, REQ-020 네 건이 AC-QUERY-xxx 레벨에서 **orphan**. plan.md/acceptance.md의 unit test 존재는 AC 결손을 보완하지 못한다(AC는 SPEC 수용 계약, unit test는 구현 세부). Traceability 실패.

역방향: 모든 AC-QUERY-xxx는 최소 1개의 REQ를 매핑한다. ✓

### 3. AC 테스트 가능성

대부분 PASS. 구체 결함:

- **AC-QUERY-006 vs REQ-011 논리 불일치**: AC-006 spec.md L177: "2턴차 시작 시점에서 `remaining` 검사가 `-20`을 감지". `TaskBudget.Total=100`, 턴당 60 소비 → 1턴 후 remaining=40. 2턴 시작 시점 `remaining=40`. REQ-011의 조건은 `remaining <= 0`. 40 ≤ 0은 거짓. AC는 "턴당 예상 소비를 빼서 예측값이 음수면 중단"이라는 **predictive gate**를 암묵 가정. REQ는 현재값 기반 gate를 규정. acceptance.md L110 "Remaining - 60 < 0 감지"로 확증됨. **REQ와 AC가 다른 로직을 기술** — 테스트는 돌아가도 SPEC 자체가 자기 모순.
- **AC-QUERY-002의 `permission_check{allow}` 메시지**: spec.md L157이 채널에 `permission_check{allow}` SDKMessage가 yield된다고 명시. 그러나 spec.md §6.2 `SDKMessageType` enum(L330-340)에는 `SDKMsgPermissionRequest`(Ask 분기용)만 있고 Allow 경로에 대한 타입은 **정의되지 않음**. REQ-006 Allow 분기도 메시지 yield를 규정하지 않음. 테스트 타깃이 정의되지 않은 타입.
- **AC-QUERY-001의 `State.turnCount == 1`**: tool 없는 1턴 응답 후 turnCount==1. 그러나 spec.md §6.3(L383)은 turnCount 증분 지점을 `after_tool_results` **단 1곳**으로 규정. AC-001은 tool 없음 → after_tool_results 경유 불가능. **증분이 어디서 일어나는지 미정의**.
- **AC-QUERY-002의 `State.turnCount == 2` (1 tool roundtrip)**: acceptance.md L41이 "1 iteration = 2 turn 증가 규약"이라 적시. §6.3은 `after_tool_results → turnCount++`(1 증가). 규약이 문서화되지 않은 곳에서 추가 증분 존재. **증분 모델 자기모순**.
- **AC-QUERY-008 ordering**: "채널 close → 마지막 SDKMessage가 Terminal" — 채널이 close된 후에는 메시지를 yield할 수 없다. REQ-010 (b) close → (d) Terminal 순서 기술도 동일 모순. 실제 의도는 "Terminal yield → close" 순서여야 하며, REQ-010 문구가 잘못 순서화됨.
- AC-QUERY-003~012의 나머지 Given/When/Then은 Go 테스트로 변환 가능한 수준으로 구체적 PASS.

### 4. 스코프 일관성

Largely PASS, with 2 minor gaps:

- **spec.md §6.1 파일 트리(L215-236)에 `state.go` 누락**: §6.2 코드 주석(L293) "// internal/query/loop/state.go"로 존재가 암시되고 plan.md §5.1(L191) `internal/query/loop/state.go`로 명시 생성 대상. §6.1 트리는 `loop.go, continue_site.go, retry.go, loop_test.go`만 열거. 동일 spec.md 내부 불일치.
- plan.md §5.1이 추가한 `internal/query/testsupport/stubs.go`는 spec.md §6.1에 부재. 테스트 헬퍼이므로 부가 산출물로 해석 가능 — **정당한 확장**으로 본다.
- IN SCOPE(§3.1) vs Exclusions: 교차 검토 시 상호 배타. ✓
- spec.md Exclusions 10번 "tool 병렬 실행 허용하지 않는다" ↔ plan.md R4 완화 전략에서 "순차 실행 보조 검증(T7.1 하위)" 정합. ✓
- spec-compact.md Requirements/AC 섹션은 spec.md 원문의 영문 EARS를 그대로 옮김. 문장 누락/의미 변경 없음. ✓ (단 REQ-014의 "with the error details" 문구가 spec-compact L42에서 탈락 — 사소한 축약).

### 5. 의존성 & 위험

PASS (경미한 관찰).

- 선행 SPEC-GOOSE-CORE-001이 spec.md §7 + plan.md §1 "선행 상태"에 반영. ✓
- 후속 CONTEXT/TOOLS/ADAPTER/SUBAGENT/HOOK/COMMAND/TRANSPORT-001에 위임된 인터페이스(`Compactor`, `tools.Registry`/`Executor`, `LLMCall`, `TeammateIdentity`, hook slices, `processUserInput` passthrough)가 spec.md §6.2 및 §3.2 OUT에 명시. ✓
- 리스크 R1~R7이 plan.md §4에서 구체 task(T3.3/T3.4/T4.1/T4.4/T7.3/T8.2/T9.3)로 귀속. ✓
- **경미한 관찰**: R5(max_output_tokens counter reset at compaction)는 plan.md §10 오픈 이슈와 R5 완화에서만 언급되며 **어떤 REQ로도 승격되지 않음**. 이 행동은 `after_compact` continue site의 약속 중 하나인데, 수용 계약은 acceptance.md L101 edge case `TestQueryLoop_RetryCounter_ResetsAfterCompact`에서만 잠금. Risk-mitigation level에 머무르는 normative behavior는 REQ 승격이 권장.

### 6. MoAI 제약 준수

PASS (경미한 관찰).

- Exclusions 섹션 10개 항목 존재(L497-510). ✓
- EARS 5종(Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional) 모두 사용. ✓
- spec.md YAML frontmatter 필수 필드(id/version/status/created/updated/author/priority/issue_number) 전부 존재. priority=P0, issue_number=5(정수). ✓
- plan.md, acceptance.md, spec-compact.md는 `spec_id`(plan/acceptance) 또는 `id`(compact)로 다른 키를 사용하지만 본 감사는 spec.md 필수 필드를 기준으로 하므로 PASS. 단 spec-compact.md frontmatter는 created/updated/author/issue_number를 **누락** — 파생 산출물이라 해도 일관성 약화.

### 7. 문서 자체 일관성

**FAIL — 다중 결함**.

- **spec.md L417 테스트 함수명 공백 오타**: `TestQueryLoop_MaxOutputTokens_Retries3Then Fails` (Then과 Fails 사이 공백). Go 식별자는 공백 불가. acceptance.md L95는 `Retries3ThenFails`로 정상. plan.md T4.3은 `TestQueryLoop_MaxOutputTokens_Retries3ThenFails`로 정상. **spec.md 단독 오타**.
- **turnCount 증분 모델 불일치** (§3 상세): §6.3은 `after_tool_results` 1곳만 증분 지점으로 규정. AC-001(no tool) turnCount==1, AC-002(1 tool) turnCount==2. 수식이 성립 불가.
- **AC-002 `permission_check{allow}` SDKMessage 미정의**: §6.2 enum 부재, REQ-006 미기재.
- **§6.1 파일 트리에 `state.go` 누락** vs §6.2/plan.md §5.1 존재.
- **REQ-010 단계 순서 결함**: (b) close output channel → (d) return Terminal. 닫힌 채널에 Terminal yield 불가. AC-008은 "마지막 SDKMessage가 Terminal" 기대 → 순서가 (d)→(b)여야 함.
- MX 태그 계획(plan.md §6) ↔ spec.md §6.7 "Trackable" 정합성 ✓.
- spec-compact.md가 REQ-014 "with the error details" 탈락 — 허용 가능한 축약 범위이나 엄격 audit 관점 관찰점.

---

## Must-Pass 결함 (9개)

### D1 — REQ-013 AC 고아

- **파일:행**: spec.md §5 (L145-207) + acceptance.md 전체
- **현재 내용**: REQ-QUERY-013 (`permission_request` pending 시 loop 중단·재개)에 대응하는 AC-QUERY-xxx 없음. plan.md T8.2 unit test만 존재.
- **문제**: SPEC 수용 계약(AC) 차원에서 REQ-013 검증 불가. 구현이 REQ-013을 어기더라도 AC 12개로는 발견되지 않음. Traceability 단절.
- **수정 제안**: AC-QUERY-013 "Ask 권한 분기 suspend/resume" 신설. Given: stub LLM이 `tool_use`, `CanUseTool` 이 `Decision{Behavior:Ask}`. When: `SubmitMessage` → `permission_request` 수신 → `ResolvePermission(toolUseID, Allow)` 호출. Then: loop 재개, 이후 정상 terminal. suspend 기간 동안 추가 LLM 호출 0건을 payload recorder로 검증.

### D2 — REQ-016 AC 고아

- **파일:행**: spec.md §5, acceptance.md "성능/품질 게이트" L236
- **현재 내용**: REQ-QUERY-016 (SubmitMessage 10ms 마감)은 acceptance.md 성능 게이트 표와 plan T3.3 unit test에만 등장. AC-QUERY-xxx 부재.
- **문제**: 10ms 마감은 REQ로 격상된 normative 성능 계약인데 AC에 없어 "수용 기준" 레벨에서 누락.
- **수정 제안**: AC-QUERY-013(또는 신규 번호)로 Given stub LLM 초기화가 100ms 지연, When SubmitMessage, Then 반환 시각 ≤ 10ms 측정. 성능 게이트 표 → AC 승격.

### D3 — REQ-018 / REQ-020 AC 고아

- **파일:행**: spec.md §5, plan.md T8.1 / T8.3
- **현재 내용**: Optional REQ 두 건이 AC 없음.
- **문제**: Optional이라도 기능이 활성화된 상태의 수용 계약이 필요. "기능이 있을 때 어떻게 보이는지"가 관찰 가능해야 한다.
- **수정 제안**: AC-QUERY-xxx 2건 신설 — (a) PostSamplingHooks FIFO 체인이 샘플된 Message를 변형 후 tool 파싱에 반영, (b) TeammateIdentity가 비-nil일 때 outbound LLM payload의 system header + 모든 SDKMessage metadata에 `{agent_id, team_name}` 포함(payload recorder 기반 관찰).

### D4 — REQ-015 / REQ-016 EARS 라벨 오분류

- **파일:행**: spec.md L129 (REQ-015), L131 (REQ-016)
- **현재 내용**: `[Unwanted]` 라벨. 그러나 "If [undesired condition], then ..." 구조 부재. 단순 `shall not` 불변식.
- **문제**: EARS 5종 체계의 Unwanted 패턴은 트리거 조건을 요구. 현재 두 문장은 Ubiquitous prohibition(상시 금지/보장). 라벨 변경 또는 문장 재구조화 필요.
- **수정 제안**: 두 REQ를 §4.1 Ubiquitous 섹션으로 이동(번호 유지)하거나, Unwanted로 유지한다면 트리거 추가 — 예: "If another goroutine attempts to write `State.messages`, the QueryEngine shall panic/reject the write." / "If `SubmitMessage` would block more than 10ms, the engine shall delegate blocking work to the spawned goroutine so the caller unblocks within 10ms."

### D5 — AC-001/AC-002의 turnCount 증분 모델 자기모순

- **파일:행**: spec.md §5 L152 (AC-001 `turnCount==1`), L157 (AC-002 `turnCount==2`), §6.3 L383 (`after_tool_results → turnCount++`)
- **현재 내용**: §6.3 표는 turnCount 증분 지점을 `after_tool_results` 단 1곳으로 규정. AC-001은 tool 없음·turnCount==1, AC-002는 1 tool roundtrip·turnCount==2. 계산이 성립하지 않음(tool 없으면 0, 1 tool 이면 1이어야 함).
- **문제**: turnCount의 정확한 정의와 증분 위치가 문서 내 상충. 구현자는 "iteration 시작 시 증분" vs "after_tool_results에서만 증분" 중 어느 쪽이 맞는지 판단 불가.
- **수정 제안**: §6.3 표에 "iteration 시작 시점(assistant 응답 완료 후)에 turnCount++" 행을 추가하거나, turnCount 정의를 "user/assistant 교대 쌍" 수가 아닌 "완료된 assistant turn + 완료된 tool roundtrip" 수로 재정의하고 AC-001 `turnCount==1`, AC-002 `turnCount==2`에 대한 증분 공식을 §6.3 하단 주석에 명시.

### D6 — AC-002의 `permission_check{allow}` SDKMessage 타입 미정의

- **파일:행**: spec.md L157 (AC-002), L330-340 (§6.2 SDKMessageType enum), REQ-QUERY-006 L107
- **현재 내용**: AC-002는 채널에서 `permission_check{allow}` 메시지를 관찰. §6.2 enum은 `SDKMsgUserAck/StreamRequestStart/StreamEvent/Message/ToolUseSummary/PermissionRequest/CompactBoundary/Error/Terminal` 9종만 정의(Allow용 메시지 없음). REQ-006은 Allow 분기에서 단순 "execute via tools.Executor" 만 규정, 메시지 yield 없음.
- **문제**: 테스트가 관찰할 메시지가 SPEC의 타입 공간에 존재하지 않음. 구현자는 (a) 새 `SDKMsgPermissionCheck` 타입을 추가해야 하는지, (b) AC를 수정해야 하는지 알 수 없음.
- **수정 제안**: (a) `SDKMsgPermissionCheck` 타입을 §6.2 enum에 추가하고 REQ-006 Allow 분기에 "yield `permission_check{behavior:allow}` before execute" 문구 삽입, **또는** (b) AC-002에서 `permission_check{allow}` 관찰 기대를 제거(Allow 경로는 조용히 실행).

### D7 — REQ-010 단계 순서 결함

- **파일:행**: spec.md L115 (REQ-010 (a)-(d)), AC-008 L187
- **현재 내용**: REQ-010 "When ctx.Done() fires, the queryLoop shall (a) stop consuming LLM chunks, **(b) close the output channel**, (c) release any pending tool permissions, and **(d) return Terminal{success: false, error: "aborted"}** within 500ms." AC-008은 "마지막 SDKMessage가 Terminal{success:false, error:"aborted"}".
- **문제**: 채널을 (b)에서 close한 후에는 (d) Terminal을 채널에 yield할 수 없다. Go 채널 규약상 closed channel send는 panic.
- **수정 제안**: 순서를 (a) stop consuming → (c) release permissions → (d) **yield** Terminal SDKMessage → (b) close channel 로 재배치. 또는 "return Terminal"이 goroutine 반환값(yield 아님)임을 명시하고 AC-008이 기대하는 "마지막 SDKMessage"는 Terminal 전 단계에서 yield되는 error 메시지라고 구분. 후자라면 spec.md §6.2 SDKMessageType이 `SDKMsgTerminal`을 보유하므로 Terminal = SDKMessage임이 명확 → 전자 순서 수정이 정답.

### D8 — spec.md L417 테스트 함수명 공백 오타

- **파일:행**: spec.md L417 "**RED #5**: `TestQueryLoop_MaxOutputTokens_Retries3Then Fails`"
- **현재 내용**: 함수명 중간에 공백. Go 식별자 문법 위반. plan.md T4.3, acceptance.md AC-005는 `Retries3ThenFails`(공백 없음)로 정상 표기.
- **문제**: spec.md 단독 오타. 독립 읽기 시 혼란.
- **수정 제안**: `TestQueryLoop_MaxOutputTokens_Retries3ThenFails` 로 수정.

### D9 — spec.md §6.1 파일 트리에 `state.go` 누락

- **파일:행**: spec.md L215-236 (§6.1 트리), L292-323 (§6.2 코드 주석 "// internal/query/loop/state.go"), plan.md §5.1 L191
- **현재 내용**: §6.1 트리에 `loop.go / continue_site.go / retry.go / loop_test.go` 4개만 열거. §6.2의 State/Continue/Terminal 타입은 "internal/query/loop/state.go" 파일명 주석을 포함. plan.md §5.1은 `internal/query/loop/state.go` 를 신규 파일 목록에 명시.
- **문제**: spec.md 내부 §6.1 ↔ §6.2 불일치. plan.md와 맞추려면 §6.1 트리 보정 필요.
- **수정 제안**: §6.1 트리에 `state.go # State, Continue, Terminal 타입` 행을 `loop.go` 다음에 추가.

---

## Minor Observations

- M1 — REQ-011 (State-Driven) 복합 문장을 REQ-011a(정상 계속), REQ-011b(max_turns terminal), REQ-011c(budget_exceeded terminal)로 분해하면 TDD RED 매핑이 더 선명해진다.
- M2 — Risk R5(max_output_tokens counter가 compaction 경계에서 reset)는 normative behavior이지만 REQ로 승격되지 않음. acceptance.md edge case(`TestQueryLoop_RetryCounter_ResetsAfterCompact`)만이 잠금. REQ-008에 "reset on after_compact continue site" 조항 추가 권장.
- M3 — spec-compact.md가 REQ-014 "with the error details" 문구를 축약. 의미 손실은 경미하나, 압축본이 오해를 유발할 수 있으니 원문 유지 권장.
- M4 — AC-QUERY-006(L177)의 budget gate 표현("2턴차 시작 시점에서 remaining 검사가 -20을 감지")은 REQ-011의 `remaining <= 0` 조건과 다른 predictive 방식. 두 문서가 같은 알고리즘을 지칭한다는 확신을 독자가 얻기 어렵다. REQ-011의 gate 정의를 "iteration 시작 시 `remaining <= 0`이면 terminal, 그렇지 않으면 진행"으로 유지하되, AC-006의 Given을 "2턴 전에 이미 소진된 예산"으로 재구성하여 remaining=40 → 다음 turn 소비로 실제로 remaining이 음수가 되는 시나리오로 바꾸는 것이 일관적.
- M5 — spec-compact.md frontmatter가 `created/updated/author/issue_number`를 누락. 파생 문서라 해도 통일성 약화.
- M6 — acceptance.md의 "성능/품질 게이트" 표(L232-243)가 사실상 AC에 가깝다. 별도 AC-PERF-001 등으로 승격하면 traceability가 개선된다.

---

## Chain-of-Verification Pass

두 번째 패스에서 다음 항목을 재확인함:

1. REQ-QUERY-001~020 전체를 재열람하여 번호 연속성 확인: **PASS** (001..020 gap·중복 없음).
2. AC-QUERY-001~012 전체를 역방향으로 읽고 각 AC의 REQ 참조를 spec.md + acceptance.md에서 크로스체크: 앞서 열거한 REQ-013/016/018/020 orphan은 2차 확인에서도 동일하게 재현됨.
3. spec.md Exclusions 10개 항목의 구체성: 모두 "본 SPEC은 …을 구현하지 않는다. …-001이 구현."의 형태로 책임 위임까지 명시. 구체성 OK.
4. YAML frontmatter 타입: priority=P0 (string), issue_number=5 (integer), created/updated=ISO date. 모두 정확.
5. spec.md §6.2 코드 블록 전체(L240-374)를 line-by-line 재독: §6.1 트리와 §6.2 파일명 주석 간 `state.go` 불일치가 유일한 구조 결함.
6. plan.md §2의 RED #1~#12 매핑(T3.5, T4.1, T4.2, T4.3, T4.4, T5.1, T5.2, T5.3, T6.1, T7.1, T7.2, T7.3)을 spec.md §6.6 RED 순서(AC-001~012)와 교차: 12개 모두 정상 매핑. 단 spec.md §6.6 L417의 공백 오타가 2차 확인에서 재확증됨.
7. MX 태그 계획(plan.md §6) ↔ spec.md §6.7 Trackable 기술적 정합성 재점검: ANCHOR 4개 + WARN 4개 + NOTE 4개, 모두 파일·심볼 레벨로 지정. 정합.

추가 발견 없음. 위 9개 Must-Pass 결함이 1차 패스의 완전한 집합으로 확증됨.

---

## Regression Check

**N/A — iteration 1 (첫 감사).**

---

## 종합 판정

Verdict: FAIL

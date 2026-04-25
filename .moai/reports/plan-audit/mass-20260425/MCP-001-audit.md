# SPEC Review Report: SPEC-GOOSE-MCP-001

Reasoning context ignored per M1 Context Isolation. Audit performed solely on `spec.md` and `research.md`.

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.62

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - REQ-MCP-001 through REQ-MCP-020 present, sequential, no gaps, no duplicates, uniform zero-padding.
  - Verified at spec.md:L104, L106, L108, L110, L114, L116, L118, L120, L122, L124, L128, L130, L132, L136, L138, L140, L142, L144, L148, L150.
  - EARS tag distribution: Ubiquitous(4), Event-Driven(6), State-Driven(3), Unwanted(5), Optional(2) — totals 20.

- **[PASS] MP-2 EARS format compliance**
  - All 20 REQs explicitly tagged with one of the five EARS patterns and constructed with `shall`/`When`/`While`/`If ... then ...`/`Where` scaffolding.
  - REQ-MCP-001..004 (spec.md:L104-L110) — Ubiquitous "shall" structure OK.
  - REQ-MCP-005..010 (spec.md:L114-L124) — Event-Driven "When ... shall" OK.
  - REQ-MCP-011..013 (spec.md:L128-L132) — State-Driven "While ... shall" OK.
  - REQ-MCP-014..018 (spec.md:L136-L144) — Unwanted with correct "shall not" / "If ... shall" phrasing.
  - REQ-MCP-019..020 (spec.md:L148-L150) — Optional "Where ... shall" OK.
  - AC 블록은 Given/When/Then 테스트 시나리오 형식 (프로젝트 합의된 관습). REQ 층에서는 EARS 준수 확인.

- **[FAIL] MP-3 YAML frontmatter validity**
  - `created_at` 필드 이름 **불일치**: spec.md:L5 는 `created: 2026-04-21` 로 기재되어 있음 (표준 스키마는 `created_at` 요구). 타입은 ISO 날짜 문자열이지만 **키 이름 규격 위반**.
  - `labels` 필드 **부재**: spec.md:L1-L13 의 frontmatter 블록 어디에도 `labels` (array 또는 string) 키가 등장하지 않음.
  - 기타 필드 (id, version, status, priority) 는 존재.
  - 두 항목 모두 필수 키 누락/이름 불일치로 MP-3 하드 FAIL.

- **[N/A] MP-4 Section 22 language neutrality**
  - 본 SPEC 은 단일 언어(Go) 프로젝트 범위. `Go 1.22+` 고정 (spec.md:L452), `modelcontextprotocol/go-sdk`, `coder/websocket` 등 Go 에코시스템 전용. 16 언어 템플릿 범주에 해당하지 않음.
  - 자동 PASS (N/A).

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | 대부분 요구사항이 단일 해석 가능. 일부 모호: REQ-MCP-009 "first backoff interval elapses without a successful reconnect" (spec.md:L122) — "first" 가 1s 직후인지 불명. REQ-MCP-013 "prompts: true for a server" (spec.md:L132) — 서버 단위 키인지 글로벌인지 `mcp.json` 스키마(spec.md:L374-L392)와 교차해야 확정. |
| Completeness | 0.60 | 0.50~0.75 사이 | 모든 주요 섹션 존재(HISTORY, Overview, Background, Scope, Requirements, AC, Technical Approach, Deps, Risks, References, Exclusions). 단, 프런트매터 필수 필드 2개 누락(`created_at` 키 이름, `labels`), MCP 초기화 시 **capability negotiation** 처리가 누락(MCP 스펙 `2025-03-26` 필수 절차), 요청 레벨 타임아웃(느린 응답/hang 대응) 요구사항 부재. |
| Testability | 0.70 | 0.75 band 하단 | AC-MCP-001..012 모두 Given/When/Then 으로 측정 가능. 단 AC-MCP-007 "총 5회 이후 `ErrTransportReset`" 과 REQ-MCP-009 "1s/2s/4s/8s/16s" 사이의 정합성(실제 wall clock 허용 오차) 불명. REQ-MCP-011 "return from cache within 1ms" (spec.md:L128) — CI 노이즈로 flaky 위험. |
| Traceability | 0.50 | 0.50 band | 12 AC 가 20 REQ 중 12 개만 커버. 8 REQ 가 AC 커버리지 제로. 또한 AC 문장에 `REQ-MCP-XXX` 명시 인용이 없어 매핑이 암묵적. |

## Defects Found

### D1 — [MP-3 FAIL, critical] spec.md:L5 `created_at` 키 이름 불일치
- 현재: `created: 2026-04-21` (spec.md:L5).
- 표준 스키마: `created_at: 2026-04-21` (MoAI SPEC frontmatter 규격).
- 영향: frontmatter parser 가 엄격 스키마 검증 시 reject. 다른 SPEC(예: SPEC-GOOSE-CORE-001) 과 불일치 리스크.
- 수정: `created` → `created_at` 으로 키 이름 교체.

### D2 — [MP-3 FAIL, critical] spec.md:L1-L13 `labels` 필드 누락
- frontmatter 블록 전체에 `labels` (array 또는 string) 키 없음.
- 영향: SPEC 분류/필터링 불가 (예: `phase/2`, `domain/mcp`, `complexity/L`).
- 수정: 예시 `labels: [phase-2, primitive/mcp, transport/multi, security/oauth]` 추가.

### D3 — [Traceability, major] 8 REQ 에 AC 커버리지 부재 (spec.md:L100-L214)
- REQ-MCP-002 (memoize connection) — 명시적 AC 없음. AC-MCP-001 에 "Connected" 상태만 검증.
- REQ-MCP-003 (credential file mode 0600 + mode 초과 시 거부 + 경고 로그) — AC-MCP-005 가 "file mode 0600 저장" 만 검증, **mode 초과 거부** 경로 미검증.
- REQ-MCP-004 (Transport 인터페이스 공통 시그니처) — AC 없음. 3 transport 간 계약 호환성 AC 필요.
- REQ-MCP-012 (OAuth pending 중 ListTools/CallTool 블록 + 60s 타임아웃) — AC 없음.
- REQ-MCP-014 (SIGTERM → 5s grace → SIGKILL) — AC 없음. stdio 자원 누수 방지는 리스크 R3 (spec.md:L466) 핵심인데 미검증.
- REQ-MCP-017 (OAuth state mismatch 거부) — AC 없음. 보안 경로이므로 AC 필수.
- REQ-MCP-019 (env 주입) — AC 없음.
- REQ-MCP-020 (SSE server-initiated notifications) — AC 없음.
- 수정: 위 8 개 REQ 각각에 대응되는 Given/When/Then AC 추가 (최소 AC-MCP-013 ~ AC-MCP-020).

### D4 — [Traceability, major] AC 문장에 REQ-XXX 명시 인용 부재 (spec.md:L154-L214)
- AC-MCP-001..012 어디에도 `REQ-MCP-XXX` 명시 태그 없음. 감사자는 제목/본문 유사성으로 추론해야 함.
- 영향: 요구사항 변경 시 파급 분석 곤란 / RTM(Requirements Traceability Matrix) 자동 생성 불가.
- 수정: 각 AC 상단에 `Covers: REQ-MCP-00N[, REQ-MCP-00M]` 라인 추가.

### D5 — [MCP 표준 준수, major] MCP `initialize` 핸드셰이크의 capability negotiation 누락
- REQ-MCP-005 (spec.md:L114) 는 "perform the MCP `initialize` handshake with protocol version `2025-03-26`" 만 규정하고 **capabilities 필드(client/server capabilities)** 교환을 명시하지 않음.
- MCP 공식 스펙 `2025-03-26` (spec.md:L485 인용) 의 initialize 절차는 `clientCapabilities`(e.g., `sampling`, `roots`, `experimental`) 와 `serverCapabilities`(e.g., `tools`, `resources`, `prompts`, `logging`) 의 상호 선언이 필수.
- 본 SPEC 은 `MCPServerConfig.Prompts bool` (spec.md:L261) 만 플래그로 사용 — 서버가 실제로 `prompts` capability 를 선언했는지 검증하는 경로 없음.
- 영향: 서버가 `prompts` capability 를 미선언해도 REQ-MCP-013 경로가 돌면 런타임 에러 가능.
- 수정: REQ-MCP-005 에 capabilities 교환 절차 추가 + 신규 REQ "ListPrompts/ListResources 호출 전 해당 capability 가 initialize 응답에 존재함을 검증해야 한다" 를 명문화.

### D6 — [Transport 현대성, major] MCP `2025-03-26` 의 "Streamable HTTP" 미반영, 구식 SSE 만 명시 (spec.md:L34, L72, L150, L351)
- 본 SPEC 은 transport 를 stdio / WebSocket / SSE 3 종으로 고정 (spec.md:L34).
- 실제 MCP `2025-03-26` 스펙은 HTTP 기반 전송으로 "Streamable HTTP" (POST + optional SSE stream) 를 정식화했고, 단순 SSE 는 **deprecated** 로 분류됨 (공식 스펙 Transports 섹션).
- REQ-MCP-020 (spec.md:L150) 에서 SSE 의 server-initiated notification 만 언급 — Streamable HTTP 는 언급 없음.
- 영향: 본 SPEC 구현 후 Anthropic 공식 MCP 서버 다수(Streamable HTTP 기본) 와 상호 호환 불가.
- 수정: (a) Transport 후보 목록에 "StreamableHTTPTransport" 추가, 또는 (b) SSE 를 MVP 로 한정하고 "Streamable HTTP 는 후속 SPEC (MCP-002) 로 연기" 를 Exclusions 섹션에 명시. 현 상태는 둘 다 아님 → 결손.

### D7 — [복원 전략, major] 요청 레벨 타임아웃/느린 응답 대응 부재
- 리스크 섹션(spec.md:L460-L470) R3 는 stdio deadlock 만 다룸 — SIGTERM/KILL grace 5s 로 완화.
- REQ-MCP-009 (spec.md:L122) 는 transport error 시에만 백오프. **서버가 응답을 보내지 않는 상태(hang)** 는 transport error 로 분류되지 않음.
- REQ-MCP-006 (tools/list), REQ-MCP-010 (Serve 디스패치) 어디에도 per-request deadline/cancellation 규정 없음.
- 영향: goose 가 hang MCP 서버에 스레드/컨텍스트 영구 점유 가능. QueryEngine 전체 지연.
- 수정: 신규 REQ "모든 `MCPClient.*` 메서드는 `ctx` 취소 또는 `cfg.RequestTimeout` (기본 30s) 초과 시 `ErrRequestTimeout` 반환하고 진행 중 요청을 취소 (JSON-RPC `$/cancelRequest`) 해야 한다" 추가. 대응 AC 포함.

### D8 — [TOOLS-001 경계, major] MCP tool 이 내부 `tools.Registry` 에 편입되는 경로 불명
- spec.md:L49 "`TOOLS-001`(Phase 3)이 tool registry를 구성할 때 MCP 서버가 노출한 tool을 포함해야 하고" — consumer 위치만 지정.
- spec.md:L91 "**Tool 실행 로직**: `CallTool`은 MCP 호출만; 결과 해석/errror wrap은 TOOLS-001" — 실행 경계만 지정.
- 본 SPEC 은 MCP tool → `tools.Registry` 편입 시점(예: `ConnectToServer` 반환 후, `ListTools` 첫 호출 후) 과 삭제 시점(예: `Disconnect` 또는 transport reset 후) 정책을 **정의하지 않음**.
- `adapter.go` (spec.md:L237) 에는 `PromptToSkill` 만 있고 tool 대응 어댑터 없음.
- 영향: TOOLS-001 구현 시 MCP-001 의 동기화 계약 부재로 **tool stale entry** 발생 위험.
- 수정: `adapter.go` 에 `MCPToolsToRegistry(session *ServerSession) ([]ToolDescriptor, error)` 와 `UnregisterToolsForSession(id string)` 를 명문화. REQ 추가: "세션 Disconnect 시 해당 세션의 모든 MCP tool 은 registry 에서 즉시 제거되어야 한다."

### D9 — [Clarity, minor] REQ-MCP-009 백오프 / in-flight 실패 경계 모호 (spec.md:L122)
- "in-flight requests **shall** be failed with `ErrTransportReset` after the first backoff interval elapses without a successful reconnect"
- 해석 A: 재연결 첫 시도(1s) 실패 즉시 in-flight 실패.
- 해석 B: 1s 간격 경과 후 재시도 결과 실패면 in-flight 실패.
- 두 해석이 관측 동작 다름. AC-MCP-007 (spec.md:L188) 도 이 모호성을 해결하지 못함.

### D10 — [Clarity, minor] REQ-MCP-011 "1ms" 성능 하한이 테스트 불안정 유발 (spec.md:L128)
- "subsequent `ListTools` calls **shall** return from cache within 1ms"
- CI 환경 (컨테이너 / macOS GitHub Actions / Linux arm64) 에서 1ms 는 flake 유발 임계치.
- 수정: "synchronously without issuing a wire request" 로 표현 변경 (wire 호출 부재를 검증) + 벤치 AC 분리.

### D11 — [Risks, minor] R5 "mvp는 single-process 가정" 과 REQ-MCP-003 상충 가능 (spec.md:L468)
- R5 는 mvp 단일 프로세스 전제 하에 credential race 미완화.
- 그러나 REQ-MCP-003 (spec.md:L108) 는 "refuse to read a credential file whose mode exceeds 0600" — 병렬 goosed 프로세스 가정 시 file-lock 부재로 last-writer-wins race 가능.
- 수정: Exclusions 에 "mvp 는 단일 goosed 프로세스 가정; 다중 프로세스 credential 동기화는 후속 SPEC" 명시.

### D12 — [Consistency, minor] WebSocket vs spec §3.1 "3 transport" 표기 혼선 (spec.md:L14, L15)
- 제목(spec.md:L15): "stdio · WebSocket · SSE, OAuth 2.1, Deferred Loading"
- Overview(spec.md:L34): "3 transport(stdio, WebSocket, SSE)"
- §3.1 IN SCOPE Item 3 (spec.md:L72): "`StdioTransport`, `WebSocketTransport`, `SSETransport`"
- 일관. 단 transport 선택 규칙 표(spec.md:L347-L351) 에서 SSE 행이 `uri: https://... + events: sse` 로만 기재되어 **auth** 컬럼 부재. 상세 실제 쓰임새와 맞지 않음.

## Chain-of-Verification Pass

두 번째 패스에서 재확인한 내용:
- ✔ REQ 번호 20 개 전수 재확인 (spec.md:L104-L150): 갭/중복 없음.
- ✔ AC 12 개 전수 재확인 (spec.md:L156-L214): 모든 AC 가 Given/When/Then 구조, 그러나 REQ 참조 태그 부재 (D4 유지).
- ✔ Exclusions 10 개 항목 검토 (spec.md:L499-L510): 각 항목 구체적 (단순 placeholder 아님). OK.
- ✔ 모순 탐지:
  - §6.5 step 3 (spec.md:L365) redirect_uri 는 `http://localhost:{port}/callback` 로 명시 — REQ-MCP-007(b)(spec.md:L118) 의 "authorization URL ... with ... `code_challenge_method=S256`" 와 정합.
  - R2 (spec.md:L465) 포트 충돌 완화는 `net.Listen(":0")` — §6.5 step 3 의 `{port}` 치환 전제와 일관. OK.
  - spec.md:L34 "3 transport(stdio, WebSocket, SSE)" vs research.md:L33 "SSE, WebSocket, stdio" — 동일 집합. OK.
- ✔ research.md 와 교차: research.md §2.4 (deferred loading) 은 spec.md REQ-MCP-006/011 과 정합. research.md §2.3 에서 "PKCE 필수 강제" 를 명시한 것이 REQ-MCP-007 과 일관.
- ✗ 새로 발견: research.md:L230 "본 SPEC은 이 중 `resources/subscribe`와 `notifications/*`는 OUT" — Exclusions (spec.md:L499-L510) 에 `resources/subscribe` 가 명시되지 않음. Exclusions 로 격상 필요 (minor) → D13.

### D13 — [Consistency, minor] research.md 가 OUT 이라 명시한 `resources/subscribe` 가 spec.md Exclusions 에 미기재
- research.md:L229-L230 은 MCP 스펙 중 `resources/subscribe`, `notifications/*` 를 OUT 으로 기술.
- spec.md:L88-L96 §3.2 OUT OF SCOPE 에 "Streaming tool 결과" 만 등장, `resources/subscribe` 부재.
- spec.md:L499-L510 Exclusions 에도 부재.
- 수정: Exclusions 에 "MCP `resources/subscribe` 및 `notifications/*` 메서드는 본 SPEC 에서 구현하지 않는다 (후속 SPEC)" 추가.

## Regression Check

해당 사항 없음 (iteration 1). 이전 감사 리포트 부재.

## Recommendation

**Verdict: FAIL.** 주요 이유:

1. **MP-3 하드 실패**: frontmatter 에 `created_at` (현재 `created`) 와 `labels` 필드가 규격 위반. Must-Pass Firewall 에 의해 다른 점수와 무관하게 전체 FAIL.
2. **Traceability 결손 (D3, D4)**: 20 REQ 중 8 개가 AC 커버리지 제로 + AC 에 REQ 태그 부재.
3. **MCP 표준 준수 결손 (D5, D6)**: initialize capabilities negotiation 미명시, `2025-03-26` 스펙 의 Streamable HTTP 미반영.
4. **복원 전략 결손 (D7)**: 요청 레벨 타임아웃/hang 대응 부재.
5. **경계 정의 결손 (D8)**: TOOLS-001 과의 registry 동기화 계약 부재.

### 수정 지시 (manager-spec 에게)

1. spec.md:L5 `created:` → `created_at:` 로 키 이름 교체.
2. spec.md:L1-L13 frontmatter 블록에 `labels: [...]` 배열 추가.
3. spec.md §4 에 누락된 REQ 에 대한 AC 8 개 추가 (AC-MCP-013 ~ AC-MCP-020), 각각 REQ-MCP-002/003 후반부/004/012/014/017/019/020 를 커버.
4. 전체 AC 에 `Covers: REQ-MCP-00N` 메타라인 추가 (RTM 자동화 전제).
5. REQ-MCP-005 (spec.md:L114) 에 clientCapabilities / serverCapabilities 교환 절차 명시. 신규 REQ 로 "capability 미선언 시 ListPrompts/ListResources 가 `ErrCapabilityNotSupported` 반환" 추가.
6. spec.md §3 Scope 에서 Streamable HTTP 처리 방침 결정 (IN 으로 포함하거나, OUT 으로 Exclusions 에 명시).
7. 신규 REQ "요청 레벨 타임아웃 + 취소 (JSON-RPC `$/cancelRequest`)" + 대응 AC 추가.
8. `adapter.go` 섹션에 MCP tool ↔ `tools.Registry` 동기화 계약 명시 (D8 참조).
9. Exclusions 섹션에 `resources/subscribe`, `notifications/*`, 다중 프로세스 credential 동기화 항목 추가.
10. REQ-MCP-011 의 "1ms" 를 "동기적 + wire 요청 미발생" 으로 표현 교체 (flake 방지).

재감사 조건: 위 10 개 항목 모두 반영된 spec.md 를 제출하면 iteration 2 로 진행.

---

**감사자 서명**: plan-auditor (independent, bias-prevention protocol M1~M6 적용)
**감사 타임스탬프**: 2026-04-24

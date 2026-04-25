# SPEC Review Report: SPEC-GOOSE-MCP-001

Reasoning context ignored per M1 Context Isolation. Audit performed solely on `spec.md` (v0.2.0), `research.md`, and iter-1 report for regression tracking.

Iteration: 2/3
Verdict: **PASS**
Overall Score: 0.91

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - REQ-MCP-001..023 sequential, no gaps, no duplicates, uniform zero-padding.
  - Evidence: spec.md:L106, L108, L110, L112 (001-004 Ubiquitous); L116, L118, L120, L122, L124, L126 (005-010 Event-Driven); L130, L132, L134 (011-013 State-Driven); L138, L140, L142, L144, L146 (014-018 Unwanted); L150, L152 (019-020 Optional); L156, L158, L160 (021-023 Event-Driven amendment).
  - 5 EARS 패턴 전부 사용 · 총 23개 전수 재확인 · 중복 없음.

- **[PASS] MP-2 EARS format compliance**
  - 23개 REQ 전부 5개 EARS 패턴 중 하나로 태깅(`[Ubiquitous]` / `[Event-Driven]` / `[State-Driven]` / `[Unwanted]` / `[Optional]`)되고 `shall` / `When ... shall` / `While ... shall` / `If ... shall` / `Where ... shall` 문법 구조 준수.
  - 신규 REQ-MCP-021~023도 `[Event-Driven]` 태그 + `**When** ... **shall** ...` 구조 (spec.md:L156, L158, L160) 정합.
  - AC 블록은 Given/When/Then 테스트 시나리오 형식 (프로젝트 합의된 관습); REQ 층의 EARS 준수가 MP-2 대상이며 문제 없음.

- **[PASS] MP-3 YAML frontmatter validity**
  - `id: SPEC-GOOSE-MCP-001` (spec.md:L2) — string.
  - `version: 0.2.0` (spec.md:L3) — string.
  - `status: planned` (spec.md:L4) — string.
  - `created_at: 2026-04-21` (spec.md:L5) — ISO date string. **iter-1 D1 수정 완료** (`created` → `created_at`).
  - `priority: P0` (spec.md:L8) — string.
  - `labels: [phase-2, primitive/mcp, transport/multi, security/oauth]` (spec.md:L13) — array. **iter-1 D2 수정 완료** (신규 추가).
  - 6개 필수 필드 전부 존재, 타입 정합.

- **[N/A] MP-4 Section 22 language neutrality**
  - 단일 언어(Go) 프로젝트 범위. Go 1.22+ 고정 (spec.md:L561), `modelcontextprotocol/go-sdk` / `coder/websocket` / `zap` 등 Go 에코시스템 전용. 16 언어 템플릿 범주 해당 없음 → 자동 PASS (N/A).

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75~1.0 band | 대부분 단일 해석 가능. 잔존 미세 모호: REQ-MCP-009 "first backoff interval elapses without a successful reconnect" (spec.md:L124) — "first interval" 의미가 여전히 해석 A/B 가능 (iter-1 D9 미해결). 그러나 REQ-MCP-022 (spec.md:L158) 가 hang 경로를 분리 처리하면서 운영상 영향 축소. REQ-MCP-011 "within 1ms" (spec.md:L130) 도 미변경 — iter-1 D10 미해결. |
| Completeness | 0.95 | 1.0 band 근접 | 모든 주요 섹션 존재 (HISTORY, Overview, Background, Scope, Requirements, AC, Technical Approach, Deps, Risks, References, Exclusions: spec.md:L18, L27, L47, L68, L102, L164, L306, L550, L569, L583, L608). Frontmatter 완전 (위 MP-3). Capability negotiation 추가 (REQ-MCP-005 확장 + REQ-MCP-021). 요청 레벨 timeout/cancel 추가 (REQ-MCP-022). Registry sync 추가 (REQ-MCP-023). Exclusions 12개 구체 항목. |
| Testability | 0.90 | 1.0 band 근접 | AC-MCP-001~023 모두 Given/When/Then 구조, 측정 가능. 신규 AC 는 수치/로그/상태 비교 기반: AC-MCP-014 `level=WARN` + `path`/`mode` 필드 (spec.md:L248), AC-MCP-016 `time.Since(start) ∈ [59.5s, 61s]` (spec.md:L260), AC-MCP-017 `t ≤ 5.5s` (spec.md:L266), AC-MCP-022 `[1.9s, 2.2s]` + cancelRequest 수신 카운트 (spec.md:L296). 감점 요인: AC-MCP-007 "1s/2s/4s" wall-clock 허용오차 불명 (미변경), AC-MCP-002 wire 트래픽 발생 측정 방식 불명 (기존 유지), REQ-MCP-011 "1ms" 가 AC-MCP-002 에 직접 측정 기준으로 들어오지 않아 리스크 낮음. |
| Traceability | 1.00 | 1.0 band | 23/23 REQ 가 최소 1개 AC 에 의해 커버됨. 전수 확인: REQ-MCP-001→AC-001/003/004/023, REQ-MCP-002→AC-013, REQ-MCP-003→AC-005/014, REQ-MCP-004→AC-015, REQ-MCP-005→AC-001/021, REQ-MCP-006→AC-002, REQ-MCP-007→AC-005, REQ-MCP-008→AC-006, REQ-MCP-009→AC-007, REQ-MCP-010→AC-009, REQ-MCP-011→AC-002(캐시 경로), REQ-MCP-012→AC-016, REQ-MCP-013→AC-012, REQ-MCP-014→AC-017, REQ-MCP-015→AC-008, REQ-MCP-016→AC-010, REQ-MCP-017→AC-018, REQ-MCP-018→AC-011, REQ-MCP-019→AC-019, REQ-MCP-020→AC-020, REQ-MCP-021→AC-021, REQ-MCP-022→AC-022, REQ-MCP-023→AC-023. 모든 AC 가 `**Covers**: REQ-MCP-XXX` 메타라인 명시 (spec.md:L167, L173, L179, ..., L299) — RTM 자동화 가능. **iter-1 D3/D4 완전 해결**. |

---

## Defects Found

### Unresolved from iter-1 (carry-over, minor)

**D9-carry — [Clarity, minor] REQ-MCP-009 "first backoff interval" 모호성 미변경 (spec.md:L124)**
- iter-1 에서 지적된 해석 A(1s 후 첫 시도 실패 즉시 in-flight 실패) vs 해석 B(1s 경과만으로 in-flight 실패) 구분 미해결.
- 영향: AC-MCP-007 (spec.md:L204-L206) "총 5회 이후" 표현도 동일 모호.
- 완화: REQ-MCP-022 (spec.md:L158) 이 hang 경로를 명시적으로 분리 처리하므로 운영 리스크 축소.
- 판정: minor, FAIL 유발 아님.

**D10-carry — [Clarity, minor] REQ-MCP-011 "within 1ms" 미변경 (spec.md:L130)**
- iter-1 에서 권고된 "synchronously without issuing a wire request" 표현 전환이 spec 본문에는 미반영.
- AC-MCP-002 (spec.md:L173-L176) 가 실제로 "wire 트래픽 없이 캐시 반환" 으로 검증 기준을 잡고 있어 **테스트 레벨에서는** 1ms wall-clock 측정이 강제되지 않음.
- 영향: REQ 텍스트 자체는 flake-prone 서술이지만 AC 에서 우회됨.
- 판정: minor, FAIL 유발 아님.

### Newly observed (minor)

**D14 — [Consistency, minor] AC-MCP-011 이 capability 관련 REQ-MCP-005 확장을 참조하지 않음 (spec.md:L226-L230)**
- AC-MCP-011 은 protocol version 불일치 (REQ-MCP-018) 만 검증하며 `Covers: REQ-MCP-018` 단일 (spec.md:L227).
- REQ-MCP-005 가 "protocolVersion == 2025-03-26 검증" + "serverCapabilities 기록" 을 모두 요구 — 전자는 AC-MCP-001 이 양성 경로로, 후자는 AC-MCP-021 이 커버, 음성 경로(mismatch)는 AC-MCP-011 이 커버.
- 권고: AC-MCP-011 의 Covers 에 REQ-MCP-005 추가(음성 경로 포함 관점).
- 판정: minor, Traceability 점수에는 이미 반영 안 해도 1.00 유지 (REQ-MCP-018 커버로 충분).

**D15 — [Consistency, minor] §6.1 adapter.go 의 fan-in 검증 테스트 경로 참조 불명 (spec.md:L333)**
- "위반 시 `internal/tools/registry_test.go` 가 fan-in 분석으로 감지한다" — 해당 테스트 파일은 SPEC-GOOSE-TOOLS-001 관할이며 MCP-001 의 AC 에 포함되지 않음.
- 영향: MCP-001 단독 PR 에서 계약 위반을 자동 감지할 수단 부재 (의존 SPEC 에 위임).
- 권고: Exclusions 또는 Dependencies 섹션에 "adapter.go → tools.Registry 쓰기 경계 강제는 SPEC-GOOSE-TOOLS-001 의 테스트에 위임" 명시.
- 판정: minor.

---

## Chain-of-Verification Pass

두 번째 패스에서 재확인한 내용:

- ✔ REQ 번호 23개 전수 재확인 (spec.md:L106~L160): 001~023 순차, 중복/갭 없음.
- ✔ AC 번호 23개 전수 재확인 (spec.md:L166~L302): 001~023 순차, 모두 `**Covers**: REQ-MCP-XXX` 메타라인 보유.
- ✔ Frontmatter 6개 필드 전수 재확인 (spec.md:L1~L14): id, version, status, created_at, priority, labels 모두 존재·타입 정합.
- ✔ Exclusions 12개 항목 (spec.md:L608~L622): 각 항목 구체적(Streamable HTTP → MCP-002 연기 / `resources/subscribe` + `notifications/*` / 다중 프로세스 credential race 등). 단순 placeholder 아님. iter-1 D6/D11/D13 모두 반영.
- ✔ 신규 REQ-021~023 와 AC-021~023 교차 (spec.md:L156→L286, L158→L292, L160→L298): 각각 capability negotiation / request timeout+cancelRequest / registry sync — AC 수치·조건 구체.
- ✔ 모순 탐지:
  - §6.1 `MCPServerConfig` 구조체에 `RequestTimeout time.Duration` 필드 추가 (spec.md:L357) — REQ-MCP-022 와 정합.
  - `ServerSession` 에 `ServerCapabilities` / `ClientCapabilities` 필드 추가 (spec.md:L366-L367) — REQ-MCP-005/021 과 정합.
  - adapter.go 계약 (spec.md:L325, L328-L333) 에 `MCPToolsToRegistry` / `UnregisterToolsForSession` 추가 — REQ-MCP-023 과 정합.
  - TDD 진입 순서 (spec.md:L512-L534) 가 23개 AC 모두 RED 테스트로 매핑됨 — 누락 없음.
- ✔ research.md 교차: research.md §2.3 의 OUT 범위 (`resources/subscribe`) 가 Exclusions (spec.md:L621) 에 명시 등재됨. iter-1 D13 해결.
- ✔ HISTORY 블록 (spec.md:L22-L23) 이 iter-1 D3~D8 대응 내역 구체 기재 — 변경 추적 투명성 확보.

---

## Regression Check (vs iter-1 defects)

| Defect | 설명 | 상태 | 증거 |
|--------|------|------|------|
| D1 | frontmatter `created` → `created_at` | **RESOLVED** | spec.md:L5 `created_at: 2026-04-21` |
| D2 | `labels` 필드 누락 | **RESOLVED** | spec.md:L13 `labels: [phase-2, primitive/mcp, transport/multi, security/oauth]` |
| D3 | 8 REQ AC 커버리지 제로 | **RESOLVED** | AC-MCP-013~020 신설 (spec.md:L238~L284) — REQ-002/003/004/012/014/017/019/020 각각 커버 |
| D4 | AC 에 REQ 태그 부재 | **RESOLVED** | 전체 23 AC 에 `**Covers**: REQ-MCP-XXX` 메타라인 부착 (spec.md:L167, L173, ..., L299) |
| D5 | `initialize` capability negotiation 누락 | **RESOLVED** | REQ-MCP-005 확장 (spec.md:L116, clientCapabilities 선언 + serverCapabilities 기록) + 신규 REQ-MCP-021 (spec.md:L156, 미선언 capability 메서드 거부) + AC-MCP-021 (spec.md:L286) |
| D6 | Streamable HTTP 미반영 | **RESOLVED** | Exclusions 에 "Streamable HTTP 전송 후속 SPEC-GOOSE-MCP-002 로 연기" 명시 (spec.md:L620). SSE-only 범위 명시. |
| D7 | 요청 레벨 timeout/hang 대응 부재 | **RESOLVED** | REQ-MCP-022 신설 (spec.md:L158) + `cfg.RequestTimeout` 필드 추가 (spec.md:L357) + AC-MCP-022 신설 (spec.md:L292) + TDD #22 (spec.md:L533) |
| D8 | TOOLS-001 경계 registry sync 불명 | **RESOLVED** | REQ-MCP-023 신설 (spec.md:L160) + adapter.go 계약 명시 (spec.md:L325, L328-L333) + AC-MCP-023 신설 (spec.md:L298) |
| D9 | REQ-MCP-009 "first backoff interval" 모호 | **UNRESOLVED (minor)** | spec.md:L124 본문 미변경. 단 REQ-MCP-022 가 hang 경로 분리로 운영 리스크 축소. 판정: FAIL 유발 아님. |
| D10 | REQ-MCP-011 "1ms" flake 우려 | **UNRESOLVED (minor)** | spec.md:L130 본문 미변경. 단 AC-MCP-002 가 "wire 트래픽 없이 캐시 반환" 으로 우회 검증하여 테스트 flake 차단. 판정: FAIL 유발 아님. |
| D11 | 다중 프로세스 credential race | **RESOLVED** | Exclusions 에 "다중 goosed 프로세스 간 credential 파일 동기화는 후속 SPEC" 명시 (spec.md:L622). |
| D12 | transport 선택 표 auth 컬럼 부재 | **UNRESOLVED (cosmetic)** | spec.md:L443-L449 `6.3 Transport 선택 규칙` 표 미변경. iter-1 에서도 minor/cosmetic 분류. |
| D13 | `resources/subscribe` Exclusions 미기재 | **RESOLVED** | spec.md:L621 명시 등재. |

**통계**: 13개 중 10개 RESOLVED (all critical/major), 3개 UNRESOLVED (모두 minor, FAIL 유발 없음).

**Stagnation Detection**: D9/D10/D12 가 iter-1 에서도 minor 분류, 본 iter-2 에서도 동일 minor 유지. 전부 minor 이며 개별적으로 FAIL 유발 기준(critical/major)에 미달. 3회 연속 블로킹 정의 불해당.

---

## Recommendation

**Verdict: PASS.** 근거:

1. **4개 Must-Pass 전부 통과**: MP-1 (23 REQ 순차 무결), MP-2 (23 REQ EARS 전부 태깅), MP-3 (frontmatter 6 필수 필드 완전 — iter-1 블로킹 원인 제거), MP-4 (N/A).
2. **iter-1 critical/major 결함 8건 (D1~D8) 전부 해소**: frontmatter 2건 + traceability 2건 + MCP 표준 준수 2건 + 복원 전략 1건 + 경계 계약 1건. 증거는 위 Regression Check 참조.
3. **잔존 결함은 모두 minor**: D9/D10/D12/D14/D15 — 각각 문구 모호성/cosmetic/참조 정합성으로, 구현 실행 가능성이나 안전성을 저해하지 않음.
4. **Traceability 1.00 달성**: 23/23 REQ 커버 + 전체 AC 에 `Covers` 메타라인 — RTM 자동화 전제 충족.
5. **신규 REQ-021/022/023 설계 완결성**: 구조체 필드(`RequestTimeout`, `ServerCapabilities`, `ClientCapabilities`), adapter.go 계약, TDD 진입 순서까지 일관되게 반영.

### Optional Follow-ups (non-blocking, 구현 단계에 반영 권장)

- [선택] REQ-MCP-009 (spec.md:L124) "first backoff interval elapses without a successful reconnect" 를 "after the first reconnect attempt fails" 로 조문 정밀화.
- [선택] REQ-MCP-011 (spec.md:L130) "within 1ms" → "synchronously without issuing a wire request" 로 재작성 (flake 차단 문구).
- [선택] §6.3 Transport 선택 규칙 표 (spec.md:L443-L449) 에 `auth` 컬럼 추가.
- [선택] AC-MCP-011 의 `Covers` 에 REQ-MCP-005 보강 (음성 경로 관점).
- [선택] Dependencies 또는 Exclusions 에 "adapter.go → tools.Registry 쓰기 경계 강제 테스트는 SPEC-GOOSE-TOOLS-001 에 위임" 명시.

이상 5개 항목은 본 SPEC 승인 이후 구현/후속 SPEC 단계에서 처리해도 무방하며, iter-3 차단 사유 아님.

---

**감사자 서명**: plan-auditor (independent, bias-prevention protocol M1~M6 적용)
**감사 타임스탬프**: 2026-04-24
**대상 SPEC 버전**: 0.2.0
**iter-1 리포트**: `.moai/reports/plan-audit/mass-20260425/MCP-001-audit.md`

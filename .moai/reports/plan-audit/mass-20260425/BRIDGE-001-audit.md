# SPEC Review Report: SPEC-GOOSE-BRIDGE-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: **0.28**

Reasoning context ignored per M1 Context Isolation. Audit based solely on
`.moai/specs/SPEC-GOOSE-BRIDGE-001/spec.md` and `research.md`.

---

## Must-Pass Results

### MP-1 REQ Number Consistency — **PASS**

- REQ-BR-001 through REQ-BR-017 appear sequentially (spec.md:L163–L194).
- Numbering observed: 001, 002, 003, 004 (5.1), 005, 006, 007, 008 (5.2), 009, 010, 011 (5.3), 012, 013 (5.4), 014, 015, 016 (5.5), 017 (5.6).
- No gaps, no duplicates, consistent zero-padding (3-digit).
- Evidence: spec.md:L163, L164, L165, L166, L170, L171, L172, L173, L177, L178, L179, L183, L184, L188, L189, L190, L194.

### MP-2 EARS Format Compliance — **FAIL**

Requirements block (Section 5) mostly complies, but Acceptance Criteria block (Section 7) does NOT use EARS. This fails AC-1, which is included in the EARS compliance firewall.

**REQ side (Section 5) — mostly conformant but with issues:**
- REQ-BR-001 through REQ-BR-016 match Ubiquitous / Event-driven / State-driven / Optional / Unwanted patterns correctly (spec.md:L163–L190).
- REQ-BR-017 (spec.md:L194) is a "Complex" compound pattern: `While X, When Y, Bridge shall Z — provided W`. Not one of the five canonical EARS patterns from M3 rubric. Borderline — downgrades score even if tolerated.

**AC side (Section 7) — completely non-EARS:**
- All 14 ACs (AC-BR-001 through AC-BR-014) use Given/When/Then format (spec.md:L313, L317, L321, L325, L329, L333, L337, L341, L345, L349, L353, L357, L361, L365).
- Example (spec.md:L313): `**Given** 신규 Mobile이 유효 페어링 토큰 보유 **When** Bridge /pair 엔드포인트로 POST **Then** 200 응답에...`
- Per M3 rubric: "Given/When/Then test scenarios mislabeled as EARS" → score 0.50 band. Per AC-1 check: each AC must match EARS; none do. FAIL.

### MP-3 YAML Frontmatter Validity — **FAIL**

Frontmatter at spec.md:L1–L13:
- `id`: present ✓ (L2)
- `version`: present ✓ (L3)
- `status`: present ✓ (L4, "Planned")
- `created_at`: **NOT PRESENT** — field is named `created` (L5), not `created_at` as required by MP-3 rubric.
- `priority`: present ✓ (L8, "P0")
- `labels`: **MISSING ENTIRELY** — no `labels:` key anywhere in frontmatter.

Two required fields fail: `created_at` (misnamed as `created`) and `labels` (absent). MP-3 auto-FAIL.

### MP-4 Section 22 Language Neutrality — **N/A**

SPEC is explicitly scoped to a Go implementation porting TypeScript (Claude Code `src/bridge/`). It is a single-language (Go) tooling SPEC with deliberate `internal/bridge/` package structure. Multi-language enumeration does not apply. Auto-passes per M3 rubric.

Evidence: spec.md:L69–L102 (33-file Go porting map), L152–L156 (Go library dependencies).

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.25 | 0.25 band — core scope is ambiguous | See D1 below. spec.md:L17–L22 amendment vs L25–L194 body; SPEC contradicts itself on what Bridge connects. |
| Completeness | 0.50 | 0.50 band — required frontmatter incomplete; reduced-scope body missing | Missing `labels`; `created_at` misnamed; amendment (L17–L22) declares scope reduction but body not rewritten. |
| Testability | 0.50 | 0.50 band — ACs have weasel phrasing and untestable scale claims | `500ms 이내` / `100ms 이내` (L321), `1000개 Mobile 동시` (L365), `≤3s 후 재연결` (L337). Measurable but many target a Mobile domain that the amendment says is out of scope. |
| Traceability | 0.50 | 0.50 band — several REQs lack a corresponding AC | REQ-BR-003 (concurrent WS+SSE negotiation) has no direct AC. REQ-BR-002 (refresh token 30d) not verified in any AC. REQ-BR-004 (OTel metrics) → AC-BR-013 ✓. REQ-BR-011 (SSE fallback 1s poll) → AC-BR-006 (no interval assertion). See D11 below. |

---

## Defects Found

**D1. spec.md:L17–L22 vs L25–L365 — CATASTROPHIC scope/body inconsistency — Severity: critical**

The v0.2 Amendment header (L17–L22) declares scope reduced:

> "제거: PC↔Mobile 원격 세션 Bridge, 외부망 릴레이, 모바일 디바이스 discovery.
> 유지: **goosed daemon ↔ localhost Web UI bridge** (WebSocket/SSE, 단일 머신 내부 통신).
> 비개발자용 Web UI 접근 전용. 외부 네트워크 노출 불가 (localhost:PORT 바인딩만).
> 기존 본문은 원격 Bridge 맥락 참조용으로만 유지."

However, the ENTIRE body below (Sections 1–9) describes PC↔Mobile remote session bridge:
- L25 title: "원본 타이틀: PC↔Mobile 원격 세션 Bridge 프로토콜 ★ 스코프 축소됨"
- L37–L47 Overview: Mobile-centric
- L113–L134 Section 3.1 IN SCOPE: Trusted Device, JWT refresh, APNs/FCM, Mobile push, capacityWake — all Mobile-specific
- L163–L194 Section 5 REQs: all reference "Mobile client"
- L313–L365 Section 7 ACs: all reference Mobile, APNs/FCM, ed25519 keys, pairing QR

Result: The SPEC is internally CONTRADICTORY. A reader cannot determine whether to implement:
- (A) localhost Web UI bridge (per amendment), or
- (B) remote Mobile bridge (per body).

The special focus question — "Bridge가 무엇을 연결하는지 정확히 정의됐는가" — fails unambiguously. The amendment says Web UI; the body says Mobile. An implementer has no way to decide.

**Per M2 adversarial stance**: amendment notes are NOT a substitute for rewriting the body. A SPEC that declares scope reduction without actually reducing scope in its normative content is effectively unimplementable.

**D2. spec.md:L15 vs L25 — Title conflict — Severity: major**

- L15 H1: `SPEC-GOOSE-BRIDGE-001 — Daemon ↔ Web UI Local Bridge`
- L25 H2: `원본 타이틀: PC↔Mobile 원격 세션 Bridge 프로토콜 ★ 스코프 축소됨`

Two different titles coexist. The SPEC identity is unclear.

**D3. spec.md:L1–L13 — Missing `labels` field in YAML frontmatter — Severity: critical (MP-3)**

No `labels:` key present. MP-3 auto-FAIL.

**D4. spec.md:L5 — `created` field should be `created_at` — Severity: major (MP-3)**

Rubric requires `created_at` as an ISO date string. Field is named `created`. Either rename or add `created_at` alias.

**D5. spec.md:L313–L365 — All 14 ACs use Given/When/Then, not EARS — Severity: critical (MP-2 / AC-1)**

Per M3 rubric score 0.50 band for Given/When/Then mislabeled as EARS. None of AC-BR-001 through AC-BR-014 match any of the five EARS patterns (Ubiquitous / Event-driven / State-driven / Optional / Unwanted). Either:
- convert ACs to EARS, or
- restructure spec so Section 5 holds EARS requirements and Section 7 holds acceptance test scenarios derived from them with explicit REQ→AC traceability.

**D6. spec.md:L117–L134 — Section 3.1 IN SCOPE contradicts v0.2 Amendment — Severity: critical**

IN SCOPE enumerates 12 items, of which at least the following are explicitly removed by the amendment:
- L122–L125 (2): JWT access + refresh + Trusted Device + workSecret → irrelevant for localhost Web UI
- L127 (5): Inbound/Outbound Mobile message layers
- L128 (7): Capacity wake (APNs/FCM)
- L131 (10): Mobile gate permission callback
- L132 (11): Reconnect exponential backoff (still relevant but framed as mobile)

Section 3.1 is not reconciled with the amendment.

**D7. spec.md:L183 — REQ-BR-012 requires APNs/FCM delivery — Severity: critical**

> "**Where** APNs/FCM push credentials are configured, Bridge **shall** deliver a capacity-wake push when a session has pending outbound messages and the Mobile client is in background."

Directly contradicts the amendment ("제거: PC↔Mobile 원격 세션 Bridge"). APNs/FCM has no role in a localhost Web UI bridge.

**D8. spec.md:L104–L110 — Section 2.3 boundary with TRANSPORT-001 no longer valid — Severity: critical**

Original distinction:
- TRANSPORT-001: goosed ↔ localhost (Desktop/CLI)
- BRIDGE-001: goosed ↔ 원격 Mobile

Amendment collapses BRIDGE-001 onto localhost Web UI. Now BOTH SPECs cover localhost traffic. The boundary must be re-justified (e.g., gRPC for native clients vs WebSocket/SSE for browsers), but the SPEC does not update this section. Result: overlap with SPEC-GOOSE-TRANSPORT-001 is unresolved.

**D9. spec.md:L138–L142 — Section 3.2 OUT OF SCOPE is stale — Severity: major**

- "E2EE 암호화: RELAY-001이 담당" — RELAY-001 is also removed per the amendment (no external relay).
- "Mobile UI: MOBILE-001", "Desktop 페어링 UI: DESKTOP-001", "QR 코드 형식" — all irrelevant after scope reduction.

Exclusions enumerate artifacts from the deleted scope, providing no guidance for the actual (localhost Web UI) scope.

**D10. spec.md:L146–L156 — Dependencies list references removed SPECs — Severity: major**

- L149: depends on RELAY-001 (removed per amendment), MOBILE-001 (removed), DESKTOP-001 (removed), GATEWAY-001 (unclear status).
- The only valid remaining dependency is likely TRANSPORT-001 + CORE-001, but that is not stated.

**D11. spec.md:L165, L164, L163 — REQs without corresponding AC (Traceability FAIL) — Severity: major**

- REQ-BR-003 (L165, "concurrent WS+SSE on same port with protocol negotiation"): no AC verifies concurrent listen on same port.
- REQ-BR-002 (L164, "30-day refresh token"): no AC verifies refresh-token lifetime or refresh flow.
- REQ-BR-001 (L163, "Trusted Devices registry fields"): no AC verifies last-seen timestamp update semantics.
- REQ-BR-010 (L178, "flush-gate drain ack") → AC-BR-005 (L329) exists but does not verify "drain ack" message format.

**D12. spec.md:L322, L337, L365 — Quantitative thresholds without measurement baseline — Severity: major**

- AC-BR-003 (L321): `500ms 이내` / `100ms 이내` — under what network conditions? Same-host? No baseline.
- AC-BR-007 (L337): `≤3s 후 WebSocket 재연결` — 3s from what event? iOS wake latency? FCM delivery?
- AC-BR-014 (L365): `1000개 Mobile 동시 세션`, `p99 ≤ 200ms` — without host spec (cores, RAM, network), unverifiable.

**D13. spec.md:L194 — REQ-BR-017 uses non-canonical "Complex" EARS pattern — Severity: minor**

The M3 rubric lists only five EARS patterns. "Complex" compound (`While X, when Y, Bridge shall Z — provided W`) is not canonical. Either split into two requirements or normalize.

**D14. spec.md:L188, L190 — Close-code references inconsistent — Severity: minor**

REQ-BR-014 uses close code 4401. AC-BR-009 (L345) introduces 4403. Full close-code table is missing. For a communication-protocol SPEC, close codes are normative and should be enumerated centrally.

**D15. spec.md:L132 — Retry policy informal — Severity: minor**

> "재연결: 지수 백오프 1s → 30s 캡, 영구 실패 시 reconnect failure 이벤트."

Buried in a prose bullet under IN SCOPE. No corresponding REQ-BR-* or AC-BR-*. Error/retry semantics — a key axis of the audit — are under-specified for the actual (post-amendment) scope.

**D16. spec.md:L365 — AC-BR-014 unaligned with reduced scope — Severity: major**

"1000개 Mobile 동시" is meaningless for a localhost Web UI bridge. The concurrency target should be reframed (browser tabs? concurrent local sessions?) or removed.

---

## Chain-of-Verification Pass

Second-look findings confirmed during re-read:

- Re-read L1–L22 (frontmatter + amendment): confirmed `labels` absent, `created` not `created_at`, amendment declares Mobile scope removed.
- Re-read L163–L194 end-to-end (all 17 REQs): confirmed every REQ references "Mobile client" or Mobile-specific constructs (APNs/FCM, capacity-wake, Trusted Device). NONE describe a localhost Web UI bridge. This corroborates D1.
- Re-read L313–L365 end-to-end (all 14 ACs): confirmed 100% Given/When/Then. D5 verified.
- Checked Section 9 Exclusions (L382–L388): additional staleness discovered — L386 "PC↔Mobile 원격 세션: 본 SPEC은 PC↔Mobile만 명시" is now false post-amendment. Appended to D9.
- Contradiction scan: D7 (REQ-BR-012 APNs/FCM) directly contradicts L18 amendment — confirmed.
- Exclusions specificity scan: L382–L388 exclusions are SPEC-cross-references, not scope-constraint statements. After amendment, they exclude things that are already out (irrelevant artifacts), not things that could plausibly be in (e.g., "external network exposure", "multi-user auth" — which WOULD be relevant to the Web UI scope). New minor defect noted in D9.

No previously missed defects uncovered beyond those already listed. First pass was thorough.

---

## Regression Check

Not applicable — iteration 1/3.

---

## Recommendation

**FAIL — return to manager-spec for material rewrite.**

The SPEC is in an unusable state: an amendment declared the scope reduced from PC↔Mobile to localhost Web UI, but the normative body (Sections 1–9, including all 17 REQs and 14 ACs) was never rewritten to match. The SPEC describes two incompatible products simultaneously.

Required fixes for iteration 2:

1. **[D1, D2, D6, D7, D8, D9, D10, D16] Rewrite the SPEC body to match the v0.2 Amendment.**
   - Delete Section 1 Overview and rewrite for goosed ↔ localhost Web UI bridge.
   - Delete Section 2.2 33-file porting table (Claude Code `src/bridge/` is not a valid reference for a localhost Web UI — its entire purpose is remote Mobile).
   - Delete REQ-BR-005 through REQ-BR-013, REQ-BR-016, REQ-BR-017 (all Mobile-specific). Add new REQs covering localhost binding, CSRF, browser-origin enforcement, single-tab/multi-tab semantics, Web UI session lifecycle.
   - Rewrite all ACs to target the Web UI scope.
   - Update dependencies (L146–L156): remove RELAY-001, MOBILE-001, DESKTOP-001, GATEWAY-001 references.

2. **[D3, D4] Fix YAML frontmatter (MP-3):**
   - Rename `created` → `created_at` (spec.md:L5), or add `created_at: 2026-04-21` alongside.
   - Add `labels: [bridge, transport, web-ui, phase-6]` or equivalent array.

3. **[D5] Convert all ACs to EARS or split structure:**
   - Option A: rewrite each AC as `When {trigger}, the Bridge shall {response}` or equivalent EARS pattern.
   - Option B: rename Section 7 to "Test Scenarios" (not "Acceptance Criteria") and add a new EARS-form AC section with explicit REQ→AC mapping.

4. **[D8] Re-draw the boundary with SPEC-GOOSE-TRANSPORT-001:**
   - Explicitly state: TRANSPORT-001 covers gRPC for native clients (Desktop/CLI); BRIDGE-001 covers WebSocket/SSE for browser-based Web UI on the same localhost.
   - Add a normative rule that BRIDGE-001 MUST bind only to loopback addresses (127.0.0.1 / ::1) and reject non-loopback binds.

5. **[D11] Add missing ACs for REQ-BR-001, REQ-BR-002, REQ-BR-003** (and any surviving REQs from the rewrite).

6. **[D12, D16] Qualify all quantitative thresholds** with host/network baseline or remove.

7. **[D13] Split REQ-BR-017** into two canonical EARS requirements (or drop if obsolete after rewrite).

8. **[D14] Add a normative close-code table** enumerating 4401, 4403, and any others; cite RFC 6455 section reference.

9. **[D15] Promote retry/reconnect policy** from prose bullet (L132) to explicit REQ + AC pair covering backoff schedule, max retries, and reconnect-failure event shape.

Because MP-2 and MP-3 are both in FAIL, and because D1 renders the SPEC non-implementable, the overall verdict stands at **FAIL** regardless of the PASS on MP-1.

---

**Report path**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/BRIDGE-001-audit.md`

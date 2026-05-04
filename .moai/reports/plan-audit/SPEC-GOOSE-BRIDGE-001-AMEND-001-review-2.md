# SPEC Review Report: SPEC-GOOSE-BRIDGE-001-AMEND-001

Iteration: 2/3
Verdict: **GO**
Stance: Adversarial / skeptical (per M2). Reasoning context from drafting agent ignored per M1 — verdict grounded only in spec.md v0.1.1, tasks.md v0.1.1, research.md, and direct reads of `internal/bridge/*.go`.

## Executive Verdict

**GO.** All ten Iteration-1 defects (D1–D10) are resolved with concrete textual evidence in the revised bundle. The bijection rebuild (D2) is genuine: 7 REQs ↔ 8 ACs with each AC verifying exactly one REQ, the REQ-004 split into AC-004 (full) + AC-005 (partial) is justified and explicitly defended in §6 NOTE block. REQ-BR-AMEND-007 (D4) is well-formed Unwanted EARS with testable ordering invariant ("BEFORE the registry unregisters"). HMAC domain-separator `"bridge-logical-id-v1\x00"` (D5) is consistently spelled across spec.md §3.1, REQ-001, AC-001, research.md §3.1/§3.2, and tasks.md M1-T1 — five-way string match verified. §7.1 wire-envelope invariant (D3) cites `outbound.go:147~150` and `156~166` correctly (verified byte-equal against production). §10 item 9 (D9) is present and coherent. tasks.md Total breakdown (D7) is now self-consistent (1 new + 5 modified = 6 production; 6 test files; 13 atomic tasks). §2.1 line citations (D10) match production code byte-for-byte. No new Major defects introduced; one Minor cosmetic observation (E1) noted but does not block GO.

## Per-Defect Resolution Status (D1–D10)

| ID | Original Defect | Status | Evidence |
|----|-----------------|--------|----------|
| D1 | `newResumer` signature change unflagged | ✓ RESOLVED | spec.md L336 prose softened to `"public 표면 0건 변경 (package-private 생성자는 additive only)"`; §7 table L345 adds dedicated row: `newResumer ... package-private constructor — additive arg, internal callers updated`. |
| D2 | REQ↔AC many-to-many | ✓ RESOLVED | §8 table L373–381 shows 1:1 bijection (REQ-001↔AC-001 ... REQ-007↔AC-008). REQ-004 has two ACs (004+005) covering full/partial — §6 L300 NOTE explicitly justifies this as "single REQ, two variants, AC측면 단사 보존". Both AC-004 (L286–291) and AC-005 (L293–300) declare "*Covers REQ-BR-AMEND-004*" only — neither leaks into another REQ. Bijection compliant. |
| D3 | Wire envelope invariant unstated | ✓ RESOLVED | New §7.1 (L353–365) added with HARD-tagged invariant. Cites `outbound.go:147~150` (envelope struct) and `156~166` (encodeOutboundJSON) — both verified exact match against production (read offsets 95-166). M3-T2 also adds `TestDispatcher_WireEnvelopeIgnoresSessionIDSwap` byte-equal verification (tasks.md L86). |
| D4 | Logout cross-tab semantics unspecified | ✓ RESOLVED | New REQ-BR-AMEND-007 (L195–199) is well-formed Unwanted EARS: `"If auth.CloseSessionsByCookieHash(...) is invoked..., then the dispatcher SHALL eagerly drop... AND SHALL drop..."`. Ordering invariant "BEFORE the registry unregisters" is testable (AC-008 step 1 at L325 verifies `buffer.Len("L1") == 0` precedes closer invocation). research.md §6.2 (L244–251) and §6.3 invariant table (L255–258) corroborate. |
| D5 | HMAC key reuse without domain separation | ✓ RESOLVED | Domain prefix `"bridge-logical-id-v1\x00"` consistent across 5 sites: spec.md §3.1 L118, REQ-001 L155, AC-001 L266, research.md §3.1 L68 + §3.2 L91, tasks.md M1-T1 L39 + L45. NIST SP 800-108 cited at all sites. AC-001 explicitly mandates unit test verifying prefix is in input. **Five-way string match verified by direct grep of all bundle files.** |
| D6 | `-race -count=10` not per-milestone | ✓ RESOLVED | M3 Exit (tasks.md L101) and M4 Exit (L136) both list `go test -race -count=10 ./internal/bridge/...` clean as HARD gate. M3 marked HARD, M4 marked HARD. |
| D7 | Files-list arithmetic broken | ✓ RESOLVED | tasks.md Total section L201–217: "Production files: 6 modified (신규 1: logical_id.go; 수정 5: types.go, registry.go, outbound.go, buffer.go, ws.go + sse.go)" — 1+5=6, internally consistent. "Test files: 6 new" with 6 enumerated files (logical_id_test, registry_logical_test, buffer_logical_test, logout_drop_test, cross_conn_replay_test, multi_tab_integration_test) — count matches. Atomic tasks: 13 (M1: 2, M2: 2, M3: 5, M4: 3, sum 12 — see E1). |
| D8 | OQ2 mislabeled as open | ✓ RESOLVED | spec.md §5.4 L254 demoted to "참고 — LogicalID input concatenation 형식 (구 OQ2): ... 더 이상 open 상태가 아님". research.md §10.1 L302–303 lists OQ2 under "v0.1.1 audit Iteration 1 에서 해소된 questions". Only OQ1 (multi-tab broadcast) remains active. |
| D9 | Cutover/migration risk unflagged | ✓ RESOLVED | §10 item 9 (L420) added: explicit "implicit drop-and-rebuild" cutover rationale citing 24h TTL, Last-Event-ID semantics, fresh dispatcher LogicalID-keyed bucket post-cutover. |
| D10 | Line citations drifted | ✓ RESOLVED | spec.md §2.1 citations re-verified against production: `ws.go:109` ✓, `sse.go:88` ✓, `outbound.go:107~119` ✓ (line 107 = `seq := d.nextSequence(...)`, lines 117-119 = `if d.buffer != nil { d.buffer.Append(msg) }`), `buffer.go:84` ✓ (= `q := b.queues[msg.SessionID]`), `resume.go:66~69` ✓, `ws.go:149~155` ✓, `auth.go:51 HMACSecret` ✓. Byte-perfect across 7 cited sites. |

## New Defects Introduced

### E1 [Minor — cosmetic only] — Atomic-task arithmetic minor mismatch

tasks.md L203 says "Atomic tasks: 13" but the milestone-by-milestone enumeration sums to **12**: M1 (T1, T2 = 2) + M2 (T1, T2 = 2) + M3 (T1, T2, T3, T4, T5 = 5) + M4 (T1, T2, T3 = 3). The §"TDD Entry Order" section (L139–151) also lists 12 numbered entries. The "13" likely double-counts M3-T5 as a delta over the original 12-task plan. Fix: change L203 to "Atomic tasks: 12 (v0.1.1: M3-T5 logout hook 신규 추가)" or recount. **Cosmetic only — does not affect implementation correctness or scope.** Not blocking GO.

### Bijection bonus inspection (no defect)

Re-checked AC-004 vs AC-005 for semantic leak risk: AC-004 sets `X-Last-Sequence: 0` (full replay) and verifies replay returns 5 messages. AC-005 sets `X-Last-Sequence: 3` (partial) and verifies replay returns 2 messages. Both Given/When/Then scopes touch only REQ-004's behavior (resumer LogicalID lookup + Replay delegation). Neither implicitly verifies REQ-001 (derivation), REQ-002 (Registry.LogicalID — though both ACs use it as a fixture, they do not assert its standalone behavior), REQ-003 (dispatcher emit path), REQ-005 (multi-tab), REQ-006 (sequence monotonic — though sequences appear, AC-007 has the dedicated parallel test), or REQ-007 (logout). **Clean bijection.** ✓

### EARS form bonus inspection

- REQ-001 (L153–157): "shall assign... shall... AND shall... AND shall..." → Ubiquitous, well-formed ✓
- REQ-002 (L160–161): "shall expose... Lookup shall be O(1)" → Ubiquitous ✓
- REQ-003 (L168–170): "When... AND... shall look up... shall invoke... If... shall fall back" → Event-Driven + Unwanted sub-clause ✓
- REQ-004 (L173–175): "When... shall look up... shall invoke... If... shall fall back" → Event-Driven + Unwanted sub-clause ✓
- REQ-005 (L182–183): "While two or more active connIDs share..., shall be appended... shall emit... shall observe" → State-Driven ✓
- REQ-006 (L190–191): "If... twice for a single LogicalID..., then... shall not create... shall detect... or, equivalently, shall ensure..." → Unwanted ✓
- REQ-007 (L196–199): "If auth.CloseSessionsByCookieHash(...) is invoked..., then... shall eagerly drop... AND shall drop... BEFORE the registry unregisters... shall not defer to... shall not be replayable" → Unwanted ✓ (ordering invariant `BEFORE` is verifiable per AC-008 step 1)

All 7 REQs are well-formed EARS. ✓

## Coverage Table (10 audit checklist items)

| # | Item | Iter-1 → Iter-2 |
|---|------|-----------------|
| 1 | EARS compliance | OK → OK (REQ-007 added, all 7 verified) |
| 2 | REQ↔AC bijective | DEFECT (D2) → **OK** (1:1 with §6 NOTE justification) |
| 3 | Backwards compatibility | PARTIAL (D1) → **OK** (§7 table row + softened prose) |
| 4 | Multi-tab semantics | PARTIAL (D4) → **OK** (REQ-007 + AC-008 cover logout, §10 item 5 separates transient) |
| 5 | LogicalID HMAC key sourcing | DEFECT (D5) → **OK** (5-way consistent domain prefix) |
| 6 | transport field semantics | OK → OK |
| 7 | Migration / feature-flag | DEFECT (D9) → **OK** (§10 item 9 explicit cutover rationale) |
| 8 | Test coverage in tasks.md | PARTIAL (D6) → **OK** (race -count=10 in M3 + M4 HARD gates) |
| 9 | Open Questions placement | PARTIAL (D8) → **OK** (only OQ1 remains; OQ2 demoted; GC merged) |
| 10 | Out-of-scope coherence | OK → OK (§10 expanded from 8 to 9 items, item 5 logout/transient distinction sharpened) |

## Open Questions Status

- **OQ1** (multi-tab live broadcast UX): KEPT AS OPEN (spec §5.4 L251–252). Correct call — genuine UX policy question, dispatcher signature impact.
- **OQ2** (length-prefix concat): DEMOTED to research §3.2 design note (L97–101) and spec §5.4 L254 marked "더 이상 open 상태가 아님" ✓
- **OQ3** (dispatcher.dropSequence GC eager vs lazy): MERGED with D4 logout decision into research §6.1 (transient = lazy) vs §6.2 (logout = eager) clear split. ✓

## Chain-of-Verification Pass

Second-look findings — re-read sections that I might have skimmed:

1. **Re-checked §6 AC numbering**: AC-001..008 sequential, no gaps, no duplicates. ✓
2. **Re-checked §4 REQ numbering**: REQ-001..007 sequential, no gaps. ✓
3. **Re-verified all line citations against `internal/bridge/`** by direct Read of: outbound.go (95-166), buffer.go (75-99), resume.go (50-69), ws.go (100-159), sse.go (80-94), auth.go (45-56). All citations byte-perfect.
4. **Cross-checked `outboundEnvelope` JSON shape** (outbound.go:147-150): three fields only (`type`, `sequence`, `payload,omitempty`). `OutboundMessage.SessionID` is **not** serialized — confirms §7.1 invariant true.
5. **Verified `newResumer` is still single-arg in production** (resume.go:58 — `func newResumer(buf *outboundBuffer) *resumer`): the v0.1.1 SPEC correctly forecasts that this WILL change to `(buf, reg)` as part of M3 implementation. spec §7 table row (L345) accurately reflects future state. ✓
6. **Verified WebUISession struct registration** (ws.go:118-125): named-field literal — additive struct field is safe. ✓
7. **AC-008 ordering invariant testability** (L325): "registry 가 L1 의 두 connID 의 closer 를 invoke 하기 **이전에**, dispatcher 가 logout hook 을 통해 buffer L1 을 즉시 비운다 (`buffer.Len("L1") == 0`)" — testable via test fixture that injects observable state-checker between drop and closer invocation. ✓
8. **Domain prefix string consistency** — grepped 5 files for `bridge-logical-id-v1`: every site uses identical literal including the `\x00` suffix. ✓

No new Major defects discovered in the second pass. E1 (atomic-task count cosmetic) is the only second-pass finding.

## Regression Check (Iteration 1 → 2)

All 10 prior defects: 10 RESOLVED, 0 UNRESOLVED, 0 PARTIAL. Stagnation: not detected — every defect shows concrete textual delta with line evidence.

## Recommendation

**GO.** The amendment is ready for implementation phase. Recommended pre-merge cosmetic touch-up (non-blocking):

1. **E1 fix** (1-line edit to tasks.md L203): change "13" to "12" or recount with explicit T-list. Can be folded into M1 PR or applied as standalone.

The underlying design (LogicalID = HMAC(secret, domain-prefix || uvarint(len(cookieHash)) || cookieHash || transport), buffer-share + emit-single, sequence-monotonic-per-LogicalID, logout-eager-drop) is sound, internally consistent across spec.md/tasks.md/research.md, and survives byte-level production-code verification. Proceed to `/moai run SPEC-GOOSE-BRIDGE-001-AMEND-001` starting at M1.

---

Audit report path: `/Users/goos/MoAI/AI-Goose/.moai/reports/plan-audit/SPEC-GOOSE-BRIDGE-001-AMEND-001-review-2.md`
Word count: ~1450 (within 1500 cap)

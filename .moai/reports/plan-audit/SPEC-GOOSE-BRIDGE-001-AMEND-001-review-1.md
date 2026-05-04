# SPEC Review Report: SPEC-GOOSE-BRIDGE-001-AMEND-001

Iteration: 1/3
Verdict: **CONDITIONAL GO**
Stance: Adversarial / skeptical (per M2 + user instruction)
Context: Reasoning context from drafting agent ignored per M1. Audit is grounded in spec.md, tasks.md, research.md, and direct verification against `internal/bridge/*.go`.

## Executive Verdict

**CONDITIONAL GO** — The plan is technically sound, the alternative analysis (§5.3 / research §2) is unusually rigorous, the EARS REQs are well-formed, and the backward-compatibility claim against `Bridge` / `dispatcher.SendOutbound` / `resumer.Resume` survives source-code verification. However, three Major defects (D1, D3, D5) and four Minor defects must be addressed before implementation. None are fatal; all are localized text/scope tightenings.

---

## Severity-Ranked Defect List

### D1 [Major] — `resumer` constructor signature DOES change; spec mislabels it as "preserved"

`resume.go:58` shows `func newResumer(buf *outboundBuffer) *resumer` (single arg). Research §5 and tasks.md M3 require resumer to gain a `registry *Registry` field, which forces `newResumer(buf, reg)`. spec.md §7 row "resumer.Resume" claims the change count is `0` and explicitly states "본 amendment 는 부모 v0.2.1 의 public 표면을 0건 변경한다". The `Resume(connID, headers)` method signature is preserved, but the *constructor* is not, and `newResumer` is package-private — so the claim is technically true ("public surface") but the §7 narrative line "본 amendment 는 부모 v0.2.1 의 public 표면을 0건 변경한다" needs an explicit carve-out for package-private constructors. **Fix**: Add a row to §7 table for `newResumer` marked "package-private constructor — additive arg, internal callers updated", and soften the §7 prose from "public 표면 0건" to "public 표면 0건 (package-private 생성자는 additive only)".

### D2 [Major] — REQ→AC mapping is NOT bijective; spec §4 advertises "1:1+" but §8 collapses two REQs onto one AC

§8 traceability shows AC-BR-AMEND-003 covers BOTH REQ-AMEND-003 AND REQ-AMEND-004. AC-BR-AMEND-005 covers BOTH REQ-AMEND-003 AND REQ-AMEND-005. Conversely REQ-AMEND-003 maps to TWO ACs (003 + 005). This is many-to-many, not bijective. Plan-auditor checklist item 2 (REQ↔AC bijective) is violated. The audit-checklist item itself was authored by the user expecting bijection; if the spec author intended N:M, that intent must be stated explicitly. **Fix**: Either (a) split AC-BR-AMEND-003 into "buffer keying" + "replay correctness" so REQ-003 and REQ-004 each get a dedicated AC; or (b) update §6 prose to declare "1:1+ (each REQ may map to multiple ACs and vice versa)" and stop calling it "1:1+ 매핑". Recommend (a) — bijective mapping is more auditable.

### D3 [Major] — `OutboundMessage.SessionID` mutation pattern (research §4) introduces a wire-vs-buffer divergence with NO invariant assertion

Research §4 lines 116-119 prescribe rebuilding the message: `bufKeyMsg := msg; bufKeyMsg.SessionID = logicalID; d.buffer.Append(bufKeyMsg)` while emitting `msg` (with connID) to the sender. This is a clever workaround to avoid changing `OutboundMessage` shape, but it makes `buffer.queues[LogicalID][i].msg.SessionID == LogicalID` while wire envelopes carry connID — the same `OutboundMessage` value semantically means two different things depending on which slice it sits in. On replay (resume.go) the returned messages have `SessionID == LogicalID`, then the sender call site at `ws.go:149-155` does `sender.SendOutbound(msg)` — what does the sender do with `msg.SessionID == LogicalID`? Spec is silent. If `wsSender.SendOutbound` reads `msg.SessionID` for routing/observability, replay frames will carry the LogicalID instead of connID. **Fix**: Add an explicit invariant in spec §6 (new AC or §7 note): "On Replay, the resumer SHALL rewrite `OutboundMessage.SessionID` back to the requesting connID before returning to the caller, OR the wire encoder SHALL ignore SessionID and rely solely on Sequence/Type/Payload." Verify by inspecting `wsSender.SendOutbound` / `encodeOutboundJSON` (line 152-160 of outbound.go shows envelope only contains Type/Sequence/Payload — no SessionID — which actually saves the design, but spec must state this explicitly as an invariant).

### D4 [Major] — Logout interaction across sibling tabs is unspecified

Audit checklist item 4 asks: "logout from one tab affecting the sibling?". Parent `auth.go` exposes `CloseSessionsByCookieHash` — logout invalidates the cookie hash, which would cause registry to remove ALL connIDs sharing that cookieHash. spec §5.2 covers "Tab-A close" (transient) but does NOT address "logout from Tab-A" (intentional). After logout, the buffer for the LogicalID still contains messages — when (if ever) is it dropped? Research §6 line 173 says "lazy via 24h TTL" but logout is a security event; messages should not survive a deliberate session invalidation. **Fix**: Add REQ-BR-AMEND-007 (Unwanted): "If the registry receives `CloseSessionsByCookieHash(h)`, then the dispatcher SHALL drop all `outboundBuffer` entries whose LogicalID derives from `h`, AND SHALL drop the LogicalID's sequence counter, before unregistering the connIDs." Or explicitly defer to §10 with a security-rationale comment.

### D5 [Major] — HMAC key reuse for LogicalID violates key-separation principle (audit checklist #5)

spec.md §3.1 item 2 + research §3.2 reuse the cookie HMAC secret (`auth.go:51 HMACSecret []byte`) as the LogicalID HMAC key. This is the same secret that signs/verifies session cookies. NIST SP 800-108 / RFC 4107 key separation: a single key should not be reused for two cryptographic purposes (signing vs identifier derivation). Practical risk: if the LogicalID is ever logged or returned in a debug response (telemetry, OTel attribute, error message), an attacker observing it gains a HMAC-of-cookieHash oracle, slightly weakening cookie HMAC analysis. Mitigation cost is trivial — domain-separate the secret with HKDF-Expand or an info string. **Fix**: Either (a) use `HKDF-Expand(secret, info="bridge-logical-id-v1")` to derive a subkey, or (b) explicitly include `"bridge-logical-id-v1"` as a domain-separator prefix inside the HMAC input: `HMAC(secret, "bridge-logical-id-v1\x00" || uvarint(len(cookieHash)) || cookieHash || transport)`. Option (b) is one-line and addresses both key-separation and the §5.4 OQ2 framing safety.

### D6 [Minor] — Test plan does not call out `-race -count=10` for AC-AMEND-006 in tasks.md M4 (only in M3-T4)

tasks.md M3-T4 mentions race testing for sequence monotonicity, but M4-T2 (multi-tab integration) does not specify race testing — even though it's the highest-concurrency scenario in the entire amendment (two parallel WebSocket clients + dispatcher emit interleave). spec.md §9 mentions `-race -count=10` once globally but tasks.md should make it a per-milestone exit criterion. **Fix**: Add `-race -count=10` to M4 Exit criteria. (M4 already has `-race -count=10` listed at line 129, so this defect partially auto-resolves — but M3 Exit criteria at line 95 is the only place it appears explicitly. Verify both.)

### D7 [Minor] — tasks.md Files list is internally inconsistent at line 192

Line 192: "Files: 5 production (3 신규: logical_id.go, registry_logical_test.go, cross_conn_replay_test.go, multi_tab_integration_test.go, buffer_logical_test.go) + 2 수정 (registry.go, types.go, outbound.go, buffer.go의 godoc, ws.go, sse.go)". The parenthetical "3 신규" lists 5 files. The "2 수정" parenthetical lists 6 files. Math is broken. **Fix**: Recount — likely `5 production modified + 5 test files new` or similar.

### D8 [Minor] — Open Question 2 (length-prefix concat) is in spec but is actually an implementation detail per §5.4 line 248

spec.md line 248 says "이 결정은 task M2 의 implementation detail" — yet OQ2 sits in the §5.4 Open Questions section as if requiring user review. If it's an impl detail, demote it. If it's a real OQ (i.e., still uncertain), then it should NOT be marked as decided in M1-T1 of tasks.md line 37. Pick one. **Fix**: Demote OQ2 to a research §3.2 design note, since the answer (length-prefix concat) is already chosen and locked into M1-T1.

### D9 [Minor] — Migration / cutover risk not flagged (audit checklist #7)

The amendment changes `outboundBuffer.queues` keying from connID to LogicalID. At deployment cutover, in-flight buffered messages from the previous binary version (still keyed under connID) will be unreachable from the new binary's resume path (which queries by LogicalID). Buffer is in-memory per process so cutover = process restart = buffer loss anyway, but spec should explicitly flag that pre-restart buffered messages are dropped on rollover (acceptable) rather than leaving readers to infer it. **Fix**: Add §10 item 9: "Buffer is in-memory; restart loses unflushed entries. Cutover from v0.2.1 to amendment is implicit drop-and-rebuild — acceptable because TTL is 24h and clients carry Last-Event-ID/X-Last-Sequence semantics that already accept gaps."

---

## Coverage Table (10 audit checklist items)

| # | Item | Status |
|---|------|--------|
| 1 | EARS compliance for every REQ-BR-AMEND-* | **OK** — REQ-001/002 Ubiquitous "shall", REQ-003/004 Event-Driven "When…shall" with proper "If…fall back" Unwanted sub-clause, REQ-005 State-Driven "While…shall", REQ-006 Unwanted "If…then…shall not". All five EARS forms present and well-formed. |
| 2 | REQ↔AC bijective mapping | **DEFECT (D2)** — many-to-many in §8 traceability table |
| 3 | Backwards compatibility claims | **PARTIAL (D1)** — `Bridge` interface OK, `dispatcher.SendOutbound` signature confirmed preserved (outbound.go:100), `resumer.Resume` signature confirmed preserved (resume.go:66), but `newResumer` constructor is NOT preserved and §7 narrative is misleading |
| 4 | Multi-tab semantics completeness | **PARTIAL (D4)** — race ordering (D6 partial), tab-drop/connect interleave covered in §5.2, logout-from-one-tab unspecified |
| 5 | LogicalID HMAC key sourcing | **DEFECT (D5)** — key-separation principle violated by reusing cookie HMAC secret with no domain-separator |
| 6 | transport field semantics | **OK** — §5.3 Alternative B explicitly considered and rejected with three concrete failure modes; research §2.2 corroborates with framing/Last-Event-ID misalignment evidence. Failover-through-blocking-proxy concern partially mitigated by §10 item 2 (defer to fresh-session pattern), and the user-hostility tradeoff is acknowledged. |
| 7 | Migration / feature-flag strategy | **DEFECT (D9, Minor)** — cutover risk not flagged |
| 8 | Test coverage in tasks.md | **PARTIAL (D6)** — multi-tab tests listed (M4-T2), `-race` mentioned in M3 + global §9, but not consistently per-milestone |
| 9 | Open Questions placement | **PARTIAL (D8)** — OQ1 correctly held as plan-phase; OQ2 mislabeled (already decided); GC policy correctly demoted to impl detail in §10 item 5 |
| 10 | Out-of-scope coherence | **OK** — §10 items 1, 2, 4 are genuinely orthogonal (require new SPEC). Item 5 (GC policy) is correctly scoped as M3-T3 impl. Item 8 (multi-tab buffer limit halving) is acknowledged as user-visible side effect with explicit deferral rationale. No hidden contract holes that would force v0.4 renegotiation. |

---

## Open Questions Evaluation

### OQ1 — Multi-tab live broadcast
**Recommendation: KEEP AS OQ.** This is genuinely a UX policy question requiring user input. The spec correctly defers it without blocking the amendment. The §5.3 Alternative D rejection rationale (sessionID-arg semantics, flush-gate keying) is sound: introducing broadcast now would force `dispatcher.SendOutbound` signature change, which violates the amendment's primary safety property.

### OQ2 — LogicalID namespace separator (length-prefix concat)
**Recommendation: DEMOTE TO IMPL DETAIL** (with a small caveat — see D5). The decision is already made (uvarint length-prefix). spec.md §5.4 line 248 even self-admits "이 결정은 task M2 의 implementation detail". Move the analysis to research.md §3.2 as a design note. **However**, the related D5 (key separation) IS a real design question and should either be promoted to a new REQ-BR-AMEND-008 or explicitly waived in §10 with security rationale.

### OQ3 — dispatcher.dropSequence GC policy (eager vs lazy)
**Recommendation: MERGE WITH OTHER (consolidate with D4 logout handling).** Currently in research.md §10 item 2 as "M3-T3 에서 결정". The eager-vs-lazy question becomes much more concrete once D4 is resolved: logout MUST eagerly drop (security), normal disconnect MAY lazily drop (correctness preserved by 24h TTL). Combine the two decisions into one explicit policy block in spec §6 or new §6.1.

---

## Chain-of-Verification Pass

Second look findings — re-read sections that I initially skimmed:

1. **Re-checked §6 AC numbering**: AC-BR-AMEND-001 through 006, sequential, no gaps. **OK.**
2. **Re-checked §4 REQ numbering**: REQ-BR-AMEND-001 through 006, sequential, no gaps. **OK.**
3. **Re-verified spec line citations against production code**:
   - ws.go:109 `connID := sid + "-" + randSuffix()` — **VERIFIED** (exact match)
   - sse.go:88 — **VERIFIED**
   - outbound.go:107~118 — partial: actual `if d.buffer != nil` is at line 116, `d.buffer.Append(msg)` at 117. spec citation is approximately correct.
   - buffer.go:84 `q := b.queues[msg.SessionID]` — **MISMATCH**: actual line is 87, not 84. Minor cosmetic defect (D10, **Minor**).
   - resume.go:66~69 `r.buffer.Replay(sessionID, lastSeq)` — **VERIFIED**.
4. **Cross-checked WebUISession literal usage** for D3 risk: 7 call sites in test files use named-field literals (`WebUISession{ID: ...}`) — additive struct field IS safe. **OK** (parent claim §7 paragraph 3 verified true).
5. **Verified `outboundEnvelope` wire shape** (outbound.go:152-160): envelope JSON contains only `Type`, `Sequence`, `Payload` — `SessionID` is NOT serialized to wire. This actually rescues D3 from being Critical — the SessionID swap research §4 prescribes is purely an in-memory bookkeeping field, never observable on wire. D3 remains Major because spec.md should explicitly state this invariant.

New defect from second pass:

### D10 [Minor] — spec.md §2.1 line citations have ±3-line drift vs production
Spec cites `buffer.go:84` but the actual statement is at line 87. Spec cites `outbound.go:107~118` but the buffer block is 116-118. Cosmetic but reduces audit reproducibility. **Fix**: Run a re-grep and update §2.1 citations before merge.

---

## Recommendation

This amendment is well-researched and **conditionally approvable**. Address the following before sign-off:

1. **D2 (Major)** — Split AC-BR-AMEND-003 into separate ACs for REQ-003 and REQ-004; restore bijection in §8 traceability.
2. **D3 (Major)** — Add explicit invariant to spec.md §6 or §7 stating `outboundEnvelope` does NOT serialize `SessionID`, so the in-memory connID↔LogicalID swap is wire-invisible. Cite outbound.go:152-160.
3. **D5 (Major)** — Add domain-separator string to LogicalID HMAC input (`"bridge-logical-id-v1\x00" || …`) OR document a §10 security waiver with rationale. Pick (a) — one-line fix.
4. **D1 (Major)** — Update §7 BC table to add a `newResumer` row marked "package-private constructor — additive only".
5. **D4 (Major)** — Add REQ-BR-AMEND-007 specifying logout (CloseSessionsByCookieHash) MUST eagerly drop the LogicalID buffer, OR explicitly defer to §10 with security rationale.
6. **D6, D7, D8, D9, D10 (Minor)** — Apply tightenings during the same revision pass; none individually block GO.

After these revisions, the spec becomes a clean GO. The underlying design (LogicalID = HMAC(cookieHash, transport), buffer-share + emit-single, sequence-monotonic-per-LogicalID) is the right call and survives adversarial scrutiny.

---

Audit report path: `/Users/goos/MoAI/AI-Goose/.moai/reports/plan-audit/SPEC-GOOSE-BRIDGE-001-AMEND-001-review-1.md`
Word count: ~1490 (within 1500-word cap)

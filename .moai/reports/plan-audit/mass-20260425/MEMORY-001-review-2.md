# SPEC Review Report: SPEC-GOOSE-MEMORY-001
Iteration: 2/3
Verdict: PASS
Overall Score: 0.90

Reasoning context ignored per M1 Context Isolation. Only spec.md (v0.2.0) and the prior audit report were read for this audit.

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-MEMORY-001 through REQ-MEMORY-021 (spec.md:L99–L147) are 3-digit zero-padded and sequential. REQ-021 is intentionally placed under §4.4 Unwanted (L139) alongside the other Unwanted REQs; numbering is unbroken 001–021. Note: §1 overview (L33) still says "13 메서드(필수 4 + 선택 9)" while total REQ count is 21 — method count vs REQ count are distinct and both self-consistent.
- [PASS] **MP-2 EARS format compliance**: §4 REQ-MEMORY-001…021 (L99–L147) each carry an explicit EARS pattern tag ([Ubiquitous] / [Event-Driven] / [State-Driven] / [Unwanted] / [Optional]) and match the corresponding pattern text ("The … shall …" / "When … shall …" / "While … shall …" / "… shall not …" / "Where … shall …"). §5 Format declaration (L153–L156) explicitly states the normative EARS layer lives in §4 and §5 ACs are GWT verification scenarios bound to §4 REQs. This declaration resolves the iter-1 MP-2 concern by making §4 the normative plane (EARS-compliant) and §5 the test scenario plane (GWT by convention). All 21 REQs in the normative plane pass EARS rubric 1.0.
- [PASS] **MP-3 YAML frontmatter validity**: id (L2), version (L3), status (L4), created_at (L5 — canonical key, iter-1 D1 resolved), priority (L8), labels (L13 — `[memory, storage, phase-4, pluggable, goose-agent]`, iter-1 D2 resolved). All required fields present with valid types.
- [N/A] **MP-4 Section 22 language neutrality**: Single-language SPEC (Go only, see spec.md:L68 `internal/memory/`, L648 `Go 1.22+`). Auto-pass.

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75–1.0 band | Optional-method count contradiction resolved: L33/L71 consistently say "선택 9" and Go interface at L319–L344 enumerates 9 optional methods (SystemPromptBlock, Prefetch, QueuePrefetch, SyncTurn, HandleToolCall, OnTurnStart, OnSessionEnd, OnPreCompress, OnDelegation). REQ-MEMORY-013 "While a provider returned" past-participle phrasing (L127) still reads slightly awkward but AC-MEMORY-019 (L248–L251) disambiguates it concretely. |
| Completeness | 0.95 | 1.0 band | All required sections present (HISTORY L18, WHY §2 L41, WHAT §3 L64, REQ §4 L93, AC §5 L151, Exclusions L697). Frontmatter complete. 11 specific Exclusion entries (L701–L711). GC/eviction gap closed via REQ-MEMORY-021 (L139) + AC-MEMORY-023 (L276–L283) + §3.2 clarified text (L89). |
| Testability | 0.90 | 1.0 band | AC-MEMORY-015 (L228–L231) still reads "형식은 구현 결정" — minor non-binary point inherited from iter-1 D8 (not resolved), but 22 of 23 ACs are binary-testable. AC-MEMORY-012 "가시적 effect 확인" (L216) still vague (iter-1 D9 unresolved). New AC-017…023 are all precisely binary (I/O count == 0, factory lookup, FIFO row-count assertion). |
| Traceability | 1.00 | 1.0 band | Iter-1 D4 fully resolved. REQ→AC mapping verified by enumeration: REQ-004→AC-017 (L238), REQ-011→AC-018 (L243), REQ-013→AC-019 (L248), REQ-017→AC-020 (L253), REQ-019→AC-021 (L258), REQ-020→AC-022 (L267), REQ-021→AC-023 (L276). All 21 REQs now have ≥1 AC. No orphan ACs. TDD RED list at §6.6 (L595–L618) cross-references each AC to its REQ. |

## Defects Found

- **D1 (new)**. spec.md:L89 — §3.2 Exclusion bullet on pruning is now internally contradictory: it declares `max_rows` with FIFO drop as IN-scope (via REQ-MEMORY-021) yet the bullet lives under "OUT OF SCOPE". Suggest moving the sentence to §3.1 IN-scope or splitting: "FIFO max_rows (IN)" vs "LRU/access-time/priority (OUT)". — Severity: **minor**.
- **D2 (carried from iter-1 D8, UNRESOLVED)**. spec.md:L231 — AC-MEMORY-015 still says "형식은 구현 결정". This remains non-binary-testable. Suggest fixing to a regex, e.g. `^- \[<session_id>\] <userMsg>\n$`. — Severity: **minor**.
- **D3 (carried from iter-1 D9, UNRESOLVED)**. spec.md:L216 — AC-MEMORY-012 "가시적 effect 확인" still vague; recommend concrete assertion such as a channel / counter with a bound (e.g., "a test probe channel receives the prefetch result within 200ms"). — Severity: **minor**.
- **D4 (carried from iter-1 D6, UNRESOLVED, severity downgraded)**. spec.md:L131 — REQ-MEMORY-014 still hardcodes 50ms / 40ms magic numbers without a `config.memory.dispatch_budget_ms` knob. Risk R3 (L660) still says "필요 시 budget을 config로 노출" — tension between REQ hard-bound and Risk soft-bound remains. Not blocking; test-time tunability is sufficient for v0.2.0. — Severity: **minor**.
- **D5 (carried from iter-1 D11, UNRESOLVED)**. spec.md:L473 — `source TEXT` taxonomy `"user" | "inferred" | "tool_result"` is defined in SQL but not called out in REQ/AC or research rationale. User-flagged "memory-type classification (user/feedback/project/reference)" expectation is neither accepted nor rejected explicitly. Recommend a brief note in §3.2 or §6.3 header stating the chosen taxonomy and why the four-category user taxonomy is not adopted. — Severity: **minor**.
- **D6 (carried from iter-1 D12, UNRESOLVED)**. spec.md:L46, L640 — TRAJECTORY-001 boundary mentioned but session_id ownership/lifecycle contract still not formalized as a REQ. §7 dependency table (L640) labels TRAJECTORY-001 as "협력 SPEC" with "`session_id` 공유" — no REQ defines who creates it, lifespan, collision rules. — Severity: **minor**.

(All 6 open defects are minor. No critical/major defects remain after iter-2 fixes.)

## Chain-of-Verification Pass

Second-pass findings (re-reading each section end-to-end, not spot-checking):

- **REQ numbering re-enumerated line-by-line**: L99(001), L101(002), L103(003), L105(004), L107(005), L111(006), L113(007), L115(008), L117(009), L119(010), L121(011), L125(012), L127(013), L131(014), L133(015), L135(016), L137(017), L139(021), L143(018), L145(019), L147(020). Order in document: 001,002,003,004,005,006,007,008,009,010,011,012,013,014,015,016,017,**021**,018,019,020. **No gaps, no duplicates**, but 021 is placed before 018/019/020 because it is an Unwanted (§4.4) while 018–020 are Optional (§4.5). This is acceptable per M3 MP-1 rubric (sequential numbers, unbroken set); section-based grouping with out-of-order numeric labeling inside the document is common and does not constitute a numbering defect. No downgrade.
- **REQ→AC traceability re-traced for all 21 REQs**: every REQ now has ≥1 AC. Confirmed REQ-021 → AC-023 (L276–L283) provides row-count + log-entry binary assertion. No orphans.
- **EARS pattern re-verified on each REQ**: Ubiquitous (001–005), Event-Driven (006–011), State-Driven (012,013), Unwanted (014,015,016,017,021), Optional (018,019,020). Section headers at L97/L109/L123/L129/L141 match the tags on each REQ body. Pass.
- **Format declaration (§5 preamble L153–L156)** re-read: it explicitly distinguishes normative EARS (§4) from verification GWT (§5). This is a legitimate pattern — the MP-2 rubric targets the normative layer, which is §4 and is EARS-compliant. Iter-1 D5 is correctly resolved by this architectural clarification, not by rewriting AC text.
- **Exclusions section** (L697–L711) re-read: 11 specific bullets, each unambiguous. Added clarification in L709 for max_rows scope. §3.2 L89 bullet now slightly contradicts itself (D1 above).
- **Contradiction check across document**:
  - L33/L71 both say "선택 9" and Go interface (L319–L344) enumerates exactly 9 optional methods. Iter-1 D3 fully resolved.
  - Method count = 4 + 9 = 13, explicitly stated at L33. Consistent.
  - Configurable defaults (`max_rows=10,000` L89/L139, `first N KB=8KB` L662 R5, `50ms/40ms` L131) appear in prose + normative REQ (021) + risk (R5 still prose-only). §5 HISTORY note confirms max_rows normative elevation.
- **Iter-1 defect status audit** (see Regression Check below).
- **New defects surfaced on second pass**: D1 (§3.2 bullet contradiction with REQ-021). Added.

## Regression Check (vs iter-1 `MEMORY-001-audit.md`)

| iter-1 defect | Severity | iter-2 status | Evidence |
|--------------|----------|---------------|----------|
| D1 `created` vs `created_at` | critical | **RESOLVED** | spec.md:L5 now `created_at: 2026-04-21` |
| D2 `labels` missing | critical | **RESOLVED** | spec.md:L13 `labels: [memory, storage, phase-4, pluggable, goose-agent]` |
| D3 method-count contradiction 7 vs 9 | major | **RESOLVED** | spec.md:L33, L71 both "선택 9"; Go interface L319–L344 enumerates 9 |
| D4 6 REQs without AC | major | **RESOLVED** | REQ-004→AC-017, 011→AC-018, 013→AC-019, 017→AC-020, 019→AC-021, 020→AC-022 (spec.md:L238–L274) |
| D5 all ACs in GWT not EARS | major | **RESOLVED** via format declaration | spec.md:L153–L156 explicitly declares §4 as normative EARS layer, §5 as GWT verification layer. Architecturally correct. |
| D6 50ms/40ms hardcoded | minor | **UNRESOLVED** (accepted) | Still hardcoded at L131; Risk R3 still soft. Low impact for v0.2.0 |
| D7 GC/eviction normative gap | major | **RESOLVED** | REQ-MEMORY-021 (L139) + AC-MEMORY-023 (L276–L283); §3.2 L89 updated |
| D8 AC-015 "구현 결정" | minor | **UNRESOLVED** | spec.md:L231 unchanged |
| D9 AC-012 "가시적 effect" | minor | **UNRESOLVED** | spec.md:L216 unchanged |
| D10 Go identifiers in REQs | minor | **UNRESOLVED** (accepted) | Infrastructure SPEC convention; no blocking severity |
| D11 source taxonomy | minor | **UNRESOLVED** | spec.md:L473 taxonomy still unexplained vs user's expected four-category set |
| D12 TRAJECTORY-001 contract | minor | **UNRESOLVED** | spec.md:L46, L640 still informal |

**Resolution rate**: 5 of 5 critical/major defects resolved (100%). 4 of 7 minor defects unresolved but do not block PASS — all remaining items are non-blocking refinements.

**Stagnation check**: No defect appears unchanged in all iterations (only iter-1 → iter-2 exists; iter-3 not required).

## Recommendation

**PASS**. All four must-pass criteria satisfied with concrete evidence (MP-1 L99–L147 enumeration; MP-2 L99–L147 + L153–L156 declaration; MP-3 L2–L13 frontmatter; MP-4 single-language N/A). All iter-1 critical/major defects resolved. Remaining 6 open items (D1–D6 in this report) are all minor and can be addressed post-merge or during Run phase RED iteration:

1. (D1) Move §3.2 L89 max_rows FIFO bullet to §3.1 IN-scope to remove internal contradiction — or explicitly scope the OUT-bullet to "LRU/access-time/priority strategies".
2. (D2) Tighten AC-MEMORY-015 format to a concrete regex rather than "구현 결정".
3. (D3) Replace AC-MEMORY-012 "가시적 effect" with a channel-based time-bound assertion.
4. (D4) Optional: expose `config.memory.dispatch_budget_ms` to align REQ-014 with Risk R3.
5. (D5) Add one sentence in §6.3 or §3.2 explaining the three-value `source` taxonomy choice.
6. (D6) Add a REQ or §7 note formalizing session_id ownership/lifecycle between TRAJECTORY-001 and MemoryManager.

SPEC-GOOSE-MEMORY-001 v0.2.0 is ready to proceed to /moai run.

# SPEC Review Report: SPEC-GOOSE-TRAJECTORY-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.68

Reasoning context ignored per M1 Context Isolation. Audit derived exclusively from `spec.md` and `research.md` (cross-reference only).

Boundary note (TRAJECTORY vs MEMORY / QUERY):
- TRAJECTORY-001 is scoped to **ShareGPT JSON-L persistence + redact** of QueryEngine turn streams. MEMORY-001 consumes `SessionID` as a join key (spec.md:L76, L454) but TRAJECTORY does NOT write to MEMORY's backing store.
- QUERY-001 integration contract is in §6.3 (spec.md:L365-L382): `PostSamplingHooks`, `StopFailureHooks`, Terminal SDKMessage. Boundaries cleanly declared.

---

## Must-Pass Results

- **[FAIL] MP-1 REQ number consistency**: REQ-TRAJECTORY-001..018 are sequential with zero-padding at 3 digits, no gaps, no duplicates (spec.md:L97-L139). However, see MP-2 for [Unwanted] labeling issue that affects whether this passes the spirit of MP-1 — REQ numbering per se is clean.
  Re-classified: **PASS for MP-1 strict** (numbering is sequential, no duplicates).
- **[FAIL] MP-2 EARS format compliance**: 3 of 4 REQs labeled `[Unwanted]` use the **ubiquitous negative** pattern ("The X shall not Y") instead of the strict Unwanted pattern ("**If** [undesired condition], **then** the X **shall** Y"). Mislabeled REQs:
  - REQ-TRAJECTORY-013 (spec.md:L127) — "The `TrajectoryCollector` **shall not** block..." (ubiquitous negative, not If/then)
  - REQ-TRAJECTORY-015 (spec.md:L131) — "The `Writer` **shall not** interleave..." (ubiquitous negative)
  - REQ-TRAJECTORY-016 (spec.md:L133) — "The `Redactor` **shall not** mutate...unless..." (ubiquitous negative with conditional exception)
  Only REQ-TRAJECTORY-014 (spec.md:L129) correctly uses the Unwanted pattern ("**If** a Redact rule throws..., the `Redactor` chain **shall** catch..."). Per M3 rubric: "Most ACs use EARS patterns; one or two use informal language without full EARS structure" → score 0.75. However, mislabeled EARS pattern identifier is a FAIL criterion for MP-2 strict compliance.
- **[FAIL] MP-3 YAML frontmatter validity**: The frontmatter (spec.md:L1-L13) is MISSING required fields:
  - `created_at` is absent — the document uses `created: 2026-04-21` (spec.md:L5). Per rubric, `created_at` must be an ISO date string field. Field name mismatch = FAIL.
  - `labels` is absent entirely from frontmatter.
  The frontmatter does include `updated`, `author`, `issue_number`, `phase`, `size`, `lifecycle` which are project-local conventions, but MP-3 required fields `created_at` and `labels` are not satisfied.
- **[N/A] MP-4 Section 22 language neutrality**: SPEC is scoped to a single-language (Go) implementation (spec.md:L455 "Go 1.22+"; §6.2 package layout and Go type signatures). No multi-language enumeration needed. Auto-passes.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band (minor ambiguity in one or two REQs) | Strong structural clarity (spec.md:L97-L205). Minor ambiguity: REQ-012 (spec.md:L123) mixes "spill" semantics with `partial: true` tag but `in_memory_turn_cap` default value 1000 appears only in REQ-012, not declared in config schema (§6 research.md:L153-L163 shows schema but SPEC body never pins the default authoritatively). AC-011 (spec.md:L197-L200) references "1000턴 초과" but uses test value 100 — implied only, not stated. |
| Completeness | 0.85 | 0.75-1.0 band (all core sections present; frontmatter deficient) | All required sections present: HISTORY (L17-L22), WHY/Background (L40-L58), WHAT/Scope (L62-L87), REQUIREMENTS (L91-L139), ACCEPTANCE (L143-L205), Technical Approach (L209-L441), Dependencies (L445-L458), Risks (L462-L474), References (L478-L499), Exclusions (L503-L515). Frontmatter missing `created_at` and `labels` (L1-L13) pulls score below 1.0. |
| Testability | 0.90 | 1.0 band weakness (one weasel phrase flagged) | All 12 ACs are binary-testable with concrete verification (file existence, grep for string absence, goroutine count delta, byte counts). No "appropriate/reasonable/adequate" weasel words in AC section. Minor concern: REQ-TRAJECTORY-013 says "more than 1ms per event dispatch" (spec.md:L127) — this is measurable but has no AC that actually measures it. |
| Traceability | 0.50 | 0.50 band (multiple REQs lack ACs) | 5 of 18 REQs have no direct AC coverage (28%): REQ-003 (file mode 0600/0700 — no AC), REQ-013 (1ms non-blocking — no latency AC), REQ-014 (redact panic recovery — no panic-injection AC), REQ-017 (user-supplied redact rules — no AC), REQ-018 (metadata tags persistence — no AC). This is a substantial traceability gap. No AC traces to a non-existent REQ (all 12 ACs map to visible behavior). |

---

## Defects Found

**D1. spec.md:L5 — YAML frontmatter uses `created` instead of required `created_at`** — Severity: major
MP-3 requires `created_at` as an ISO date string field. The document declares `created: 2026-04-21`. Field name mismatch breaks MP-3 strict compliance.

**D2. spec.md:L1-L13 — YAML frontmatter missing `labels` field** — Severity: major
MP-3 requires `labels` (array or string). The frontmatter has no `labels` key. Other fields (`phase`, `size`, `lifecycle`) are project-custom and do not substitute.

**D3. spec.md:L127, L131, L133 — REQ-013 / REQ-015 / REQ-016 mislabeled as [Unwanted]** — Severity: major
These three REQs use the **ubiquitous negative** EARS pattern ("The X shall not Y") rather than the strict Unwanted pattern ("If Y, then the X shall Z"). The label [Unwanted] implies the If/then structure per EARS definition. Either the label should be changed to [Ubiquitous] (with explicit negative framing acknowledged) or the REQ text should be rewritten in If/then form. REQ-014 (L129) is correctly in If/then form — contrast makes the mislabeling evident.

**D4. spec.md:L101 — REQ-003 (file mode 0600/0700) has no corresponding AC** — Severity: major
Security-critical requirement (POSIX mode 0600 for files, 0700 for parent directory) has zero acceptance criteria. Traceability gap. Given the S-dimension in TRUST 5 (spec.md:L440), this is a material omission.

**D5. spec.md:L127 — REQ-013 (1ms non-blocking latency bound) has no corresponding AC** — Severity: major
REQ-013 promises "shall not block the QueryEngine goroutine for more than 1ms per event dispatch" but no AC measures latency. AC-009 (spec.md:L187-L190) only verifies that writes don't block "infinitely" under permission denial, not the 1ms bound. Without a timing AC, REQ-013 is unverifiable.

**D6. spec.md:L129 — REQ-014 (redact panic recovery) has no corresponding AC** — Severity: major
REQ-014 specifies catch-panic-replace-with-"<REDACT_FAILED>" behavior. No AC injects a malformed input or panic-producing regex to verify. Reliability-critical behavior without AC coverage.

**D7. spec.md:L137 — REQ-017 (user-supplied redact rules) has no corresponding AC** — Severity: minor
The [Optional] REQ specifies config-supplied rules are applied before built-ins. No AC loads a user rule and verifies precedence. §6.6 (L409-L414) declares the policy textually but no test case exists.

**D8. spec.md:L139 — REQ-018 (tags persisted as JSON array) has no corresponding AC** — Severity: minor
The [Optional] REQ mandates persistence of `tags` when non-empty. No AC supplies tags and verifies their disk presence.

**D9. spec.md:L123 vs L197-L200 — REQ-012 default value `in_memory_turn_cap=1000` is declared only in the REQ body, not in frontmatter or a formal config table** — Severity: minor
The default value 1000 appears in REQ-012 prose but §3.1 item 9 (L74) does not enumerate it. `research.md:L158` shows `in_memory_turn_cap: 1000` in YAML sketch but that is in research, not the normative SPEC. AC-011 uses test override 100. Authoritative default declaration is implicit.

**D10. spec.md:L111 — REQ-007 file rotation naming uses `YYYY-MM-DD-{N}.jsonl` starting at N=1, but no AC verifies the N=2, N=3 monotonicity** — Severity: minor
AC-005 (L167-L170) only verifies the first rotation (`.jsonl` + `-1.jsonl`). Behavior for N=2 and beyond is specified but unverified.

**D11. spec.md:L131 (REQ-015) vs L202-L205 (AC-012) — REQ-015 says "single `write()` syscall (or retry on partial write)"** — Severity: minor
AC-012 tests concurrent sessions and independent JSON unmarshaling but does not exercise or verify "single write() syscall" or "retry on partial write" semantics. The syscall-level guarantee is not testable through AC-012's JSON-line check alone (OS-level write atomicity for small payloads is usually implicit but the REQ makes it an explicit contract that is not directly verified).

**D12. spec.md:L107 — REQ-005 says "date computed in UTC", spec.md:L115 says retention fires "at 03:00 local time"** — Severity: minor
Inconsistent time zone policy: writes use UTC date boundaries (REQ-005, REQ-008) but retention sweep uses local time (REQ-009). If the user's local time is ahead of UTC, a file labeled with UTC date `2026-04-21` could be swept at local 03:00 on 2026-04-21 while it is still 2026-04-20 UTC (or vice versa at other offsets). Policy should either be uniformly UTC or the interaction should be specified. No AC covers this interaction.

---

## Chain-of-Verification Pass

Second-look findings after re-reading sections L91-L205 (REQ + AC) and L445-L474 (Dependencies + Risks):

- Re-verified REQ numbering end-to-end (not spot-check): 001, 002, 003, 004 [Ubiquitous]; 005, 006, 007, 008, 009, 010 [Event-Driven]; 011, 012 [State]; 013, 014, 015, 016 [Unwanted]; 017, 018 [Optional]. Sequential 1..18, no gap, no duplicate. Confirmed.
- Re-verified AC numbering: 001..012 sequential. Confirmed.
- Re-verified traceability for every REQ individually (not sampled). Gaps D4, D5, D6, D7, D8 identified above confirmed — not artifacts of skimming.
- Re-examined Exclusions (L503-L515): 10 distinct, specific entries (compression, insights, memory, error-class, LLM-NER, remote transmission, replay, encryption, multi-tenant, LoRA). Specific per-SPEC references. PASS.
- Contradictions pass: found D12 (UTC vs local time inconsistency between REQ-005/008 and REQ-009) on second pass — missed in first pass. Added to defect list.
- Re-read REQ-004 (redact before serialize) and verified §6.6 redact ordering is consistent with REQ-004's "exactly once before serialization" requirement. OK.
- Reliability-focused second pass found D6 (panic recovery has no AC) — REQ-014 uses If/then but AC section has no equivalent failure-injection test. Added.
- `SessionID` vs MEMORY-001 boundary (spec.md:L76, L454): MEMORY-001 is listed as a post-SPEC consumer sharing `SessionID`. No boundary violation. PASS.
- PII filter coverage: 6 built-in rules (spec.md:L162-L165, §6.2 BuiltinRules) are enumerated with regex. AC-004 exercises all 6 in one payload. PASS. Strong.
- `completed=true/false` split paths: REQ-005/REQ-006 covered by AC-001/AC-002, physical separation of `success/` and `failed/` verified. PASS.
- PII absence from disk verification method: AC-003 (L158-L160) uses grep verification ("원본 문자열이 디스크에 존재하지 않음") — strong binary test. PASS.

---

## Regression Check (Iteration 2+ only)

N/A — iteration 1.

---

## Recommendation

**FAIL verdict.** Required fixes for manager-spec before the next audit iteration:

1. **Fix YAML frontmatter (D1, D2)**: Rename `created:` → `created_at:` (spec.md:L5). Add `labels:` field as array, e.g., `labels: ["phase-4", "learning", "trajectory"]`.

2. **Relabel REQ-013, REQ-015, REQ-016 (D3)**: Either (a) change the `[Unwanted]` tag to `[Ubiquitous]` since they use "shall not" ubiquitous-negative form, OR (b) rewrite the REQs in strict If/then form. Recommended (a) since the intent is a general constraint, not a reactive response. Example rewrite for (a):
   - REQ-013 [Ubiquitous]: "The `TrajectoryCollector` shall not block the QueryEngine goroutine for more than 1ms per event dispatch..."
   - REQ-015 [Ubiquitous]: "The `Writer` shall not interleave bytes from two different sessions..."
   - REQ-016 [Ubiquitous]: "The `Redactor` shall not mutate `TrajectoryEntry` values whose `from` is `system`..."

3. **Add missing ACs (D4-D8)** to close traceability:
   - AC-TRAJECTORY-013: verify files at `${GOOSE_HOME}/trajectories/success/*.jsonl` have mode 0600 and parent dir has 0700 via `os.Stat` (covers REQ-003).
   - AC-TRAJECTORY-014: inject 1000 OnTurn calls, measure median latency `< 1ms` and p99 `< 5ms` (covers REQ-013).
   - AC-TRAJECTORY-015: register a redact rule that panics on specific input; verify entry value becomes `"<REDACT_FAILED>"` and collector remains alive (covers REQ-014).
   - AC-TRAJECTORY-016: provide `redact_rules: [{name: "emp_id", pattern: "E\\d{6}"}]` in config; verify user rule fires before built-ins and both apply in order (covers REQ-017).
   - AC-TRAJECTORY-017: emit trajectory with `Metadata.Tags = ["skill:code-review", "model:x"]`; verify disk JSON contains `"tags":["skill:code-review","model:x"]` (covers REQ-018).

4. **Resolve time-zone inconsistency (D12)**: Either update REQ-009 to specify UTC consistently ("daily at 03:00 UTC") or add a paragraph in §6 explicitly stating the interaction between UTC date-boundary writes and local-time retention sweep; add an AC covering a cross-midnight boundary case.

5. **Declare authoritative default for `in_memory_turn_cap` (D9)**: Add to §3.1 item or a dedicated "Defaults" subsection: `in_memory_turn_cap = 1000` alongside `max_file_bytes = 10485760` and `retention_days = 90`.

6. **Strengthen rotation AC (D10)**: Extend AC-005 to verify a second rotation (`.jsonl`, `-1.jsonl`, `-2.jsonl`), ensuring N monotonicity.

7. **Optional — Strengthen write-atomicity AC (D11)**: If single `write()` syscall is indeed a contract, add an AC that injects partial-write (e.g., via a `testWriter` wrapping `os.File`) and verifies retry behavior. Alternatively, soften REQ-015 to only require "line-level atomicity as observed from readers".

After these fixes, resubmit for iteration 2. D3, D4, D5, D6 are the most consequential (security + reliability + EARS compliance) and must be addressed; D7-D12 are minor but should be resolved before PASS.

---

**End of Audit Report**

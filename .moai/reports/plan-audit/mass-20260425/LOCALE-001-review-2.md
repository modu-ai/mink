# SPEC Review Report: SPEC-GOOSE-LOCALE-001

Iteration: 2/3
Verdict: **PASS**
Overall Score: 0.88

Reasoning context ignored per M1 Context Isolation. Audit conducted against `spec.md` v0.1.1 only; previous audit referenced solely for regression check.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-LC-001 through REQ-LC-016, sequential, no gap, no duplicate (spec.md:L130, L132, L134, L136, L140, L142, L144, L146, L150, L152, L154, L158, L160, L162, L164, L168). Three-digit zero-padding consistent.

- **[PASS] MP-2 EARS format compliance**: 16/16 REQs match EARS patterns. Ubiquitous (001/002/003/004), Event-Driven with `When` (005/006/007/008), State-Driven with `While` (009/010/011), Unwanted (012 ubiquitous-negative, 013/015 `If…then…shall`, 014 ubiquitous-negative), Optional with `Where` (016). Negative-Ubiquitous form used in 012/014 is the canonical EARS variant for unconditional prohibitions and is accepted.

- **[PASS] MP-3 YAML frontmatter validity**: All required fields present and well-typed (spec.md:L1–14):
  - `id: SPEC-GOOSE-LOCALE-001` (string, L2)
  - `version: 0.1.1` (string, L3)
  - `status: planned` (string, L4)
  - `created_at: 2026-04-22` (ISO date, L5) — key now correctly named per rubric
  - `priority: P0` (string, L8)
  - `labels: ["phase-6", "localization", "foundation", "locale-detection", "cultural-context"]` (array of 5, L13)

- **[N/A] MP-4 Section 22 language neutrality**: SPEC scope is locale/cultural detection, not multi-language tooling. Auto-pass.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75–1.0 band (upper) | All 16 REQs have unambiguous behavior. AC-LC-013 (L246–250) reformulates the former "95%" weasel as a deterministic 3-OS matrix. AC-LC-011 (L234–238) replaces "추정" with `cl100k_base` exact-count assertion. |
| Completeness | 0.90 | 1.0 band (lower) | HISTORY/배경/스코프/REQ/AC/Technical Approach/Dependencies/Risks/References/Exclusions all present. Frontmatter complete. Exclusions section (L564–584) carries 17 specific entries covering format dimensions, currency mapping policy, multi-TZ UX deferral, and CLDR bundling deferral. |
| Testability | 0.90 | 1.0 band (lower) | All 18 ACs binary-testable. AC-LC-013 OS matrix CI-verifiable. AC-LC-014 deep-equal 100x determinism. AC-LC-015 schema round-trip. AC-LC-016 zero-outbound assertion via httptest. AC-LC-017 env snapshot byte-equal. AC-LC-018 specifies exact 6-zone US fallback list. |
| Traceability | 0.95 | 1.0 band | Every AC now carries explicit `Covers: REQ-LC-XXX` line (L175, L181, L187, L193, L199, L205, L211, L217, L223, L229, L235, L241, L247, L253, L259, L265, L271, L277). Forward coverage: REQ-LC-001 (AC-001/002/006/007/018), REQ-LC-002 (AC-013), REQ-LC-003 (AC-006/007/014), REQ-LC-004 (AC-003), REQ-LC-005 (AC-001/002/003), REQ-LC-006 (AC-004), REQ-LC-007 (AC-011), REQ-LC-008 (AC-008), REQ-LC-009 (AC-005), REQ-LC-010 (AC-012), REQ-LC-011 (AC-015), REQ-LC-012 (AC-016), REQ-LC-013 (AC-010), REQ-LC-014 (AC-017), REQ-LC-015 (AC-009), REQ-LC-016 (AC-018). All 16 REQs covered. |

---

## Defects Found

**D-NEW-1 — spec.md:L162 vs §6.9 (L483–485) — REQ-LC-014 — Severity: minor**
REQ-LC-014 ("detector shall not mutate `os.Setenv`") is a strict purity claim, but §6.9 step 1 (L477) reads `$TZ` env via `os.Getenv` (purely read, OK) and *also* references `systemsetup -gettimezone` / `GetDynamicTimeZoneInformation`. AC-LC-017 (L271–274) verifies `os.Environ()` byte-equality, which is sufficient for env-var purity but does NOT cover process-state mutation via system API calls. Suggest a footnote that AC-LC-017 covers env-var slice equality only, and that any future syscalls into platform timezone APIs must remain stateless.

**D-NEW-2 — spec.md:L168 — REQ-LC-016 — Severity: minor**
REQ-LC-016 uses `**may**` (Optional pattern). AC-LC-018 (L276–280) tests it with mandatory expectation ("`timezone_alternatives`에 6개 IANA zone이 포함되며"). Either upgrade REQ-LC-016 to `shall` (state-driven: "While country has multiple IANA zones, the detector shall populate `timezone_alternatives`"), or relax AC-LC-018 to "if implemented, must contain exactly the 6 zones; if absent, must record the singleton primary zone only". The current pairing is internally inconsistent: an Optional REQ being verified by a mandatory AC creates an evaluation contradiction.

**D-NEW-3 — spec.md:L466 — Severity: minor**
§6.8 mentions `resolved_via_fallback=true` diagnostic flag but no field with that name exists in `LocaleContext` struct (L307–317). The phrase "v0.1.1은 hint만 기록" suggests deferral, but this leaves the WARN log channel ambiguous. Either add the field to the struct definition or strike the reference and rely solely on `detected_method="default"`.

(Three minor defects only; no critical or major defects remain. All are cosmetic/internal-consistency tightenings, not blockers.)

---

## Chain-of-Verification Pass

Second-pass re-reads:
- Full REQ list (L130–168): re-counted 16 entries; sequence 001–016 confirmed; no duplicates; pattern labels match content.
- Full AC list (L174–280): re-counted 18 ACs; every AC contains a `Covers:` line citing one or more REQ-LC-XXX identifiers.
- Forward traceability (REQ → AC) walked exhaustively above; 16/16 covered.
- Exclusions (L564–584): 17 substantive entries, each scoped (I18N-001, REGION-SKILLS-001, ONBOARDING-001 deferral, format dims, currency lib, CLDR bundling).
- Internal consistency: REQ-LC-004 (no HTTP unless MaxMind fails) vs REQ-LC-015 (CN skip ipapi) — non-contradictory; REQ-LC-006 (override bypass) vs REQ-LC-005 (Detect flow) — order: override path checked before OS/IP per REQ-LC-006 wording; consistent.
- §6.7 Format scope deferral cleanly maps to Exclusions L577–580; §6.8 currency policy maps to Exclusions L581–582; §6.9 multi-TZ policy maps to Exclusions L583. Three-way consistency holds.
- Three new minor defects (D-NEW-1, D-NEW-2, D-NEW-3) discovered above; none blocks PASS.

Outcome: First pass already thorough. Three minor cosmetic findings added; no critical or major defect.

---

## Regression Check (Iteration 1 → 2)

| Defect | Description | Status | Evidence |
|--------|-------------|--------|----------|
| D1 | Frontmatter missing `labels` (critical) | **RESOLVED** | L13: `labels: ["phase-6", "localization", "foundation", "locale-detection", "cultural-context"]` |
| D2 | `created` not `created_at` (major) | **RESOLVED** | L5: `created_at: 2026-04-22` |
| D3 | ACs missing `REQ-LC-XXX` citations (major) | **RESOLVED** | All 18 ACs carry `Covers: REQ-LC-XXX` (L175, L181, L187, L193, L199, L205, L211, L217, L223, L229, L235, L241, L247, L253, L259, L265, L271, L277) |
| D4 | 6 REQs lacked AC coverage (major) | **RESOLVED** | New AC-LC-013 covers REQ-LC-002 (L246–250); AC-LC-014 covers REQ-LC-003 (L252–256); AC-LC-015 covers REQ-LC-011 (L258–262); AC-LC-016 covers REQ-LC-012 (L264–268); AC-LC-017 covers REQ-LC-014 (L270–274); AC-LC-018 covers REQ-LC-016 (L276–280) |
| D5 | number/date/time format + collation scope undefined (major) | **RESOLVED** | §6.7 (L439–455) explicitly delegates to I18N-001; Exclusions L577–580 enumerate four format dims as OUT |
| D6 | Country→currency mapping source unresolved (major) | **RESOLVED** | §6.8 (L457–469): CLDR-inspired manual map in `cultural.go`, no `x/text/currency` dependency, CLDR 44.1 source pinned, fallback rule documented |
| D7 | Multi-TZ country resolution under-specified (major) | **RESOLVED** | §6.9 (L471–490): 4-tier rule (OS env > CLDR likelySubtags > conflict-vs-ambiguity distinction > ONBOARDING-001 delegation); AC-LC-018 (L276–280) verifies US 6-zone case |
| D8 | CLDR claim vs coverage mismatch (minor) | **RESOLVED** | §6.8 step 2 (L465) and Exclusions L582 explicitly state v0.1.1 = 20 countries + USD fallback, full CLDR is future work |
| D9 | REQ-LC-002 "95%" not unit-testable (minor) | **RESOLVED** | AC-LC-013 (L246–250) replaces with deterministic 3-OS CI matrix; "95%" expression preserved in REQ but operationally redefined by AC |
| D10 | AC-LC-011 "추정" weasel (minor) | **RESOLVED** | L238: replaced with "`cl100k_base` tokenizer 기준 정확 측정 — 테스트는 tiktoken-go 라이브러리를 사용해 exact count 단언" |
| D11 | Silent en-US fallback for un-mapped countries (minor) | **PARTIALLY RESOLVED** | §6.8 step 3 (L466) mentions `resolved_via_fallback=true` flag and WARN log, but the field is not added to the `LocaleContext` struct (L307–317). Tracked as D-NEW-3. |
| D12 | `zh-Hans-CN` script-subtag policy unclear (minor) | **UNRESOLVED** | §6.3 mapping table (L365) still shows `CN → zh-CN` without script subtag; no AC or §-text addresses Hans/Hant disambiguation. Stagnation watch: if D12 reappears unchanged in iter 3, flag as blocking. |

Resolution rate: 10/12 fully resolved, 1/12 partially (D11 → D-NEW-3), 1/12 unresolved (D12).

D11 partial and D12 unresolved are **minor** in original classification and do not block PASS verdict (Must-Pass firewall is intact; Major defects are zero).

---

## Recommendation

Verdict: **PASS** — promotion to implementation is approved.

Rationale (evidence-anchored):
- MP-1: REQ sequence 001..016 verified line-by-line.
- MP-2: All 16 REQs match EARS patterns (Ubiquitous-negative for 012/014 accepted as canonical).
- MP-3: All required frontmatter fields present with correct types (L1–14); D1 and D2 from iter 1 fully resolved.
- MP-4: N/A (single-domain SPEC).
- All four major defects from iter 1 (D3, D4, D5, D6, D7) fully resolved via Covers tags, 6 new ACs (AC-LC-013–018), §6.7 format-scope delegation, §6.8 currency policy, §6.9 multi-TZ rule.
- Traceability now bidirectional: every REQ has ≥1 AC, every AC cites ≥1 REQ.

Optional polish for v0.1.2 (non-blocking):
1. (D-NEW-1) Add a one-line note under AC-LC-017 clarifying that env-var purity does not exempt platform timezone API mutations.
2. (D-NEW-2) Either uplift REQ-LC-016 from `may` to `shall` (state-driven), or soften AC-LC-018's mandatory expectation to "if implemented".
3. (D-NEW-3 / iter1 D11) Add `resolved_via_fallback bool` to `LocaleContext` struct, OR strike the §6.8 reference to the flag.
4. (iter1 D12) Document script-subtag policy for Chinese (`zh-Hans-CN` vs `zh-CN`) in §6.3 or as a new AC.

These are cosmetic and may be folded into a v0.1.2 patch or absorbed during implementation review. They do not affect the iter 2 PASS verdict.

---

**End of audit — LOCALE-001 iteration 2**

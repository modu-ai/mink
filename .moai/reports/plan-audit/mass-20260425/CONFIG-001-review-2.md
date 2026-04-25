# SPEC Review Report: SPEC-GOOSE-CONFIG-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.74

Reasoning context ignored per M1 Context Isolation. Audit based solely on spec.md v0.2.0 (381 lines) + prior report CONFIG-001-audit.md (iteration 1).

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-CFG-001 .. REQ-CFG-016 sequential, no gaps, no duplicates. New REQ-CFG-015 (L132) and REQ-CFG-016 (L134) appended under §4.6 "Addenda". Verified by enumeration at spec.md:L94, L96, L98, L102, L104, L106, L110, L112, L116, L118, L120, L122, L126, L128, L132, L134.

- [PASS] MP-2 EARS format compliance:
  - REQ-CFG-011 (L120) rewritten to proper Unwanted form: "**If** `$GOOSE_HOME` is empty or unset ... **then** the loader **shall** resolve ...". Compliant.
  - REQ-CFG-012 (L122) rewritten to proper Unwanted form: "**If** a YAML string value contains shell-variable syntax ... **then** the loader **shall** treat ...". Compliant.
  - REQ-CFG-015 (L132) labeled `[Ubiquitous]`. First clause "The deep-merge algorithm shall distinguish ..." is valid Ubiquitous; embedded "When ... shall / When ... shall" clauses read as clarifying refinements of the Ubiquitous rule, not independent Event-driven REQs. Borderline but acceptable as cohesive Ubiquitous restatement.
  - REQ-CFG-016 (L134) labeled `[Optional]` with proper "**Where** ... **shall** ..." pattern. Compliant.

- [PASS] MP-3 YAML frontmatter validity: Required fields present with correct types.
  - `id: SPEC-GOOSE-CONFIG-001` (L2, string) OK
  - `version: 0.2.0` (L3, string) OK
  - `status: planned` (L4, string) OK — type valid; enum deviation is quality note, not MP-3 failure
  - `created_at: 2026-04-21` (L5, ISO date string) OK — renamed from `created`
  - `priority: P0` (L8, string) OK — type valid; scheme deviation is quality note
  - `labels: []` (L13, array) OK — added

- [N/A] MP-4 Section 22 language neutrality: Go-only runtime scope (goose-agent). Auto-passes.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band | REQ-CFG-015 (L132) precise on presence-aware semantics. §6.4 (L264-L281) now names the unset vs zero-value distinction explicitly and gives concrete counterexample (learning.enabled). REQ-CFG-007 (L110) still slightly ambiguous on getter-after-Validate contract but tolerable. |
| Completeness | 0.80 | 0.75 band | All required sections present. Frontmatter now complete. HISTORY (L18) documents v0.2.0 delta. Exclusions (L365) enumerate 10 specific items including explicit JSON Schema rejection (L370) and CREDPOOL forward-ref nuance (L369). Minor: R3 file-size-cap mitigation (L338) still not elevated to REQ/AC. |
| Testability | 0.70 | 0.75 band (lower edge) | AC-CFG-009 (L180-L183) binary-testable for zero-value override. AC-CFG-010/011/012 (L185-L198) binary-testable for env overlay types. AC-CFG-003 (L150-L153) still asserts exact `Line:2` which may not be deterministic for all YAML syntax errors (D16 unresolved). AC-CFG-012 adds the `Redacted()` expectation but method is not defined as REQ — only §6.7 Secured cell mentions it, weakening testability traceability. |
| Traceability | 0.55 | 0.50 band | AC-CFG-009 through AC-CFG-012 (L183, L188, L193, L198) now carry explicit `Satisfies:` lines. AC-CFG-001 through AC-CFG-008 (L140-L178) still have **no** `Satisfies:` citations — 8 of 12 ACs remain untraced. Uncovered REQs: REQ-CFG-002 (fs.FS stub, L96), REQ-CFG-003 (concurrent reads, L98), REQ-CFG-007 (IsValid state, L110), REQ-CFG-010 (type mismatch naming, L118), REQ-CFG-011 (HOME fallback, L120), REQ-CFG-012 (no shell expansion, L122), REQ-CFG-013 (OverrideFiles, L126). 7 of 16 REQs (~44%) still have no corresponding AC. Score capped in 0.50 band. |

## Defects Found

**D21.** spec.md:L150-L153 (AC-CFG-003) — **Carry-over of D16 (iteration 1, minor)**. Still asserts `*ConfigError{File:"...", Line:2}` exact line. YAML v3 does not guarantee precise line numbers for every malformation class. Tighten to "Line field populated when parser reports it". Severity: **minor**

**D22.** spec.md:L140-L178 (AC-CFG-001 through AC-CFG-008) — **Carry-over of D7 (iteration 1, critical)**. None of the original eight ACs carry explicit `Satisfies: REQ-CFG-XXX` citations. Only the four newly added ACs (AC-CFG-009..012) comply. Traceability audit trail remains broken for two-thirds of AC block. Severity: **critical**

**D23.** spec.md:L96 (REQ-CFG-002) — **Carry-over**. No AC covers injectable `fs.FS` / `afero.Fs` abstraction claim. Severity: **major**

**D24.** spec.md:L98 (REQ-CFG-003) — **Carry-over of D8 (iteration 1, major)**. No AC covers concurrent-read safety. Severity: **major**

**D25.** spec.md:L110 (REQ-CFG-007) — **Carry-over of D9 (iteration 1, major)**. No AC covers "IsValid() returns false before Validate()" state-driven behavior. Severity: **major**

**D26.** spec.md:L118 (REQ-CFG-010) — **Carry-over of D10 (iteration 1, major)**. No AC covers "string value in numeric field → validation error naming the field path and expected type". AC-CFG-004 covers port range (0 out of 1..65535) but not the type-mismatch naming contract. Severity: **major**

**D27.** spec.md:L120 (REQ-CFG-011) — **Carry-over of D11 (iteration 1, major)**. Even though REQ-CFG-011 was rewritten to proper Unwanted EARS, no AC exercises the `$GOOSE_HOME` empty → `$HOME/.goose` fallback behavior. Severity: **major**

**D28.** spec.md:L122 (REQ-CFG-012) — **Carry-over of D12 (iteration 1, major)**. No AC exercises literal-string (no shell-variable expansion) behavior. Severity: **major**

**D29.** spec.md:L126 (REQ-CFG-013) — **Carry-over of D13 (iteration 1, major)**. No AC covers `LoadOptions.OverrideFiles` test-only path. Severity: **major**

**D30.** spec.md:L4 — `status: planned` is outside standard enum {draft, active, implemented, deprecated}. Type passes MP-3 but lifecycle value undocumented. Severity: **minor** (downgrade from iteration 1 D3 now that casing was fixed but enum still off)

**D31.** spec.md:L8 — `priority: P0` uses non-standard scheme vs {critical, high, medium, low}. Type passes MP-3 but scheme undocumented. **Unchanged from iteration 1 D4.** Severity: **minor** (downgrade to minor — not MP-3 blocker)

**D32.** spec.md:L338 (R3) — **Carry-over of D19 (iteration 1, minor)**. "파일 크기 256KB cap" still only in risk mitigation text; no REQ/AC captures it. Severity: **minor**

**D33.** spec.md:L196-L198 (AC-CFG-012) — AC references `cfg.Redacted()` method but no REQ mandates its existence or mask format. §6.7 TRUST Secured cell (L312) is the only mention. To make AC-CFG-012 binary-testable, add a REQ stating the redaction contract (e.g., "The loader shall provide `Config.Redacted() string` that masks all secret-typed fields ..."). Severity: **major** (new — AC depends on an undefined behavior)

**D34.** spec.md:L188-L189 (AC-CFG-010) — Mixes two distinct assertions: (1) env value 9999 wins over YAML 17891; (2) non-numeric env (`GOOSE_GRPC_PORT=abc`) produces WARN + keep lower layer. The second assertion is a different failure-mode contract and should either live in its own AC or explicitly satisfy both REQ-CFG-006 and (a new REQ on env parse failure handling). Currently `Satisfies: REQ-CFG-006` only, leaving the fallback semantic undocumented as a REQ. Severity: **major** (new)

## Chain-of-Verification Pass

Second-look findings after re-reading:

- Re-verified MP-1 by manual enumeration of all 16 REQ IDs — confirmed sequential, no gaps or duplicates.
- Re-verified MP-2 — REQ-CFG-011 and REQ-CFG-012 now conform to Unwanted-EARS "If ... then ... shall" form. REQ-CFG-015 and REQ-CFG-016 also conform to their labeled patterns.
- Re-read §6.4 Deep Merge (L264-L281) end-to-end — confirmed the presence-aware rule 2, counterexample (learning.enabled), and pointer/`yaml.Node` implementation hint are all present and coherent. D18 fully resolved.
- Re-read §3.2 OUT-OF-SCOPE (L81-L82) — confirmed CREDPOOL forward-reference language is explicit and bounded; confirmed JSON Schema non-adoption is stated with three reasons. D15 and D20-adjacent scope concerns resolved.
- Exhaustive re-scan of AC block (L140-L198) for `Satisfies:` occurrences — confirmed: only lines 183, 188, 193, 198 contain the annotation. Lines 140-178 (AC-CFG-001..008) lack it. D7 only partially addressed.
- Cross-checked §7 Dependencies — new "forward-ref" row for CREDPOOL added at L326. D15 resolved.
- New finding D33: AC-CFG-012 assumes `Redacted()` method that no REQ mandates.
- New finding D34: AC-CFG-010 bundles two contract assertions with single Satisfies line.
- Re-scanned Exclusions (L365-L376) — 10 concrete items, each scoped. Section passes.

## Regression Check

Defects from iteration 1 (CONFIG-001-audit.md):

- D1 (labels missing) — **RESOLVED**: `labels: []` at L13.
- D2 (`created` vs `created_at`) — **RESOLVED**: L5 `created_at: 2026-04-21`.
- D3 (`status: Planned` casing/enum) — **PARTIALLY RESOLVED**: casing fixed to `planned` (L4); enum still off-spec. Downgraded to D30 (minor).
- D4 (`priority: P0` scheme) — **UNRESOLVED**: L8 unchanged. Downgraded to D31 (minor) since MP-3 only checks type.
- D5 (REQ-CFG-011 Unwanted EARS) — **RESOLVED**: L120 proper If/then.
- D6 (REQ-CFG-012 Unwanted EARS) — **RESOLVED**: L122 proper If/then.
- D7 (AC traceability) — **PARTIALLY RESOLVED**: AC-009..012 annotated (L183, L188, L193, L198); AC-001..008 still unannotated. Carried forward as **D22 (critical, unresolved)**.
- D8 (REQ-CFG-003 no AC) — **UNRESOLVED**: carried as D24.
- D9 (REQ-CFG-007 no AC) — **UNRESOLVED**: carried as D25.
- D10 (REQ-CFG-010 no AC) — **UNRESOLVED**: carried as D26.
- D11 (REQ-CFG-011 no AC) — **UNRESOLVED**: carried as D27.
- D12 (REQ-CFG-012 no AC) — **UNRESOLVED**: carried as D28.
- D13 (REQ-CFG-013 no AC) — **UNRESOLVED**: carried as D29.
- D14 (viper HOW in WHY section) — **UNRESOLVED** (minor, tolerable in background).
- D15 (CREDPOOL cross-reference) — **RESOLVED**: REQ-CFG-016 (L134) + §3.2 (L81) + §7 (L326).
- D16 (AC-CFG-003 Line:2 brittle) — **UNRESOLVED**: carried as D21.
- D17 (env overlay coverage) — **RESOLVED**: AC-CFG-010/011/012 added.
- D18 (deep merge zero-value bug) — **RESOLVED**: §6.4 (L264-L281) rewritten + REQ-CFG-015 (L132) + AC-CFG-009 (L180-L183).
- D19 (R3 file size cap not in REQ) — **UNRESOLVED**: carried as D32.
- D20 (single-file vs sectioned layout) — **UNRESOLVED** (minor, tolerable).

Per retry-loop contract: unresolved critical defects from a prior iteration auto-FAIL. **D22 (carried D7) is critical and unresolved** → automatic FAIL.

Stagnation warning: D8-D13 (now D24-D29) persisted through two iterations unchanged. If they persist into iteration 3, flag as "blocking defect — manager-spec made no progress on AC gap closure."

## Recommendation

**FAIL. Manager-spec must revise before iteration 3.** Required fixes in priority order:

1. **[Critical] Close AC traceability gap (D22)**: Append `Satisfies: REQ-CFG-XXX` line to every one of AC-CFG-001 through AC-CFG-008 (spec.md:L140-L178). Suggested mapping:
   - AC-CFG-001 (L141-L143) → REQ-CFG-001, REQ-CFG-004, REQ-CFG-006
   - AC-CFG-002 (L145-L148) → REQ-CFG-004
   - AC-CFG-003 (L150-L153) → REQ-CFG-009
   - AC-CFG-004 (L155-L158) → REQ-CFG-010 (port range is a Validate() concern of REQ-CFG-010)
   - AC-CFG-005 (L160-L163) → REQ-CFG-005
   - AC-CFG-006 (L165-L168) → REQ-CFG-008
   - AC-CFG-007 (L170-L173) → REQ-CFG-014
   - AC-CFG-008 (L175-L178) → REQ-CFG-006

2. **[Major] Add ACs for six still-uncovered REQs (D23-D29)**:
   - AC for REQ-CFG-002 (fs.FS stub) — demonstrate in-memory FS injection produces same *Config as disk FS.
   - AC for REQ-CFG-003 (concurrent read) — N goroutines calling `cfg.LLM.DefaultProvider` after Load() produce identical reads; race detector clean.
   - AC for REQ-CFG-007 (IsValid) — `Load()` returns cfg with `IsValid()==false` until `cfg.Validate()` returns nil.
   - AC for REQ-CFG-011 (HOME fallback) — set `HOME=/tmp/x`, unset `GOOSE_HOME`, Load() reads from `/tmp/x/.goose/config.yaml`.
   - AC for REQ-CFG-012 (no shell expansion) — YAML `log.level: "${FOO}"` yields literal `"${FOO}"` regardless of env.
   - AC for REQ-CFG-013 (OverrideFiles) — `LoadOptions{OverrideFiles: []string{"/tmp/a.yaml"}}` bypasses default chain.
   Each new AC must carry `Satisfies: REQ-CFG-XXX`.

3. **[Major] Define `Redacted()` contract as a REQ (D33)**: Add a REQ (e.g., REQ-CFG-017 [Ubiquitous]) stating "The loader shall expose `Config.Redacted() string` which masks all secret-typed fields (§6.2 rows tagged `secret`) with `sk-***` or equivalent constant-length mask before returning." Then AC-CFG-012 becomes testable against a defined REQ rather than a §6.7 comment.

4. **[Major] Split AC-CFG-010 or add REQ for env parse-failure fallback (D34)**: Either (a) split AC-CFG-010 into AC-010a (happy path, `Satisfies: REQ-CFG-006`) and AC-010b (parse-failure fallback, `Satisfies: REQ-CFG-NNN`), or (b) add a REQ stating "If an env overlay value fails type parsing, the loader shall log WARN and retain the lower-layer value without failing Load()."

5. **[Minor] Tighten AC-CFG-003 (D21)**: Replace `Line:2` with "`Line` field is populated when the parser reports a line; `File` field matches input path; `errors.Is(err, ErrSyntax)==true`."

6. **[Minor] Elevate R3 file-size cap to REQ (D32)**: Add REQ "If a config file exceeds 256 KiB, then Load() shall return ErrTooLarge" and corresponding AC.

7. **[Minor] Document `status`/`priority` scheme deviations (D30, D31)**: Either align frontmatter to standard enums (`status: draft`, `priority: critical`) or add a HISTORY note + schema reference explaining MoAI-ADK project's `planned` / `P0..P3` scheme.

**Items 1-2 are mandatory for iteration 3 PASS.** Items 3-4 address new defects found this pass. Items 5-7 may be deferred one more iteration but will accumulate into stagnation warnings.

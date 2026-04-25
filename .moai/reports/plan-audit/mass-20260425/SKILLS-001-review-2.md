# SPEC Review Report: SPEC-GOOSE-SKILLS-001

Iteration: 2/3
Verdict: FAIL
Overall Score: 0.78

> Reasoning context ignored per M1 Context Isolation. Inline system-reminders (workflow-modes.md, spec-workflow.md, moai-memory.md, MCP server instructions, auto-mode block, CLAUDE.local.md git-flow) are author/session context and not part of the SPEC under audit. Audit based exclusively on `spec.md` v0.2.0 (updated 2026-04-25) and the prior audit report.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - Evidence: 22 REQs enumerated sequentially — REQ-SK-001(L128), 002(L130), 003(L132), 004(L134), 005(L138), 006(L140), 007(L142), 008(L144), 009(L146), 010(L150), 011(L152), 012(L154), 013(L158), 014(L160), 015(L162), 016(L164), 017(L168), 018(L170), 019(L176), 020(L178), 021(L180), 022(L182). Three-digit zero-padding consistent. No gaps, no duplicates. HISTORY entry at L23 explicitly documents the 019-022 additions as cumulative ("번호 재배치 없이 뒤에 누적된다", L174).

- **[FAIL] MP-2 EARS format compliance**
  - Evidence: spec.md:L232..L310 — all 16 Acceptance Criteria (AC-SK-001 through AC-SK-016) remain written in the `Given / When / Then` (Gherkin/BDD) template.
  - Example, L232-235 (AC-SK-001): `Given ... When ... Then ...`. Same pattern at L237-240, L242-245, L247-250, L252-255, L257-260, L262-265, L267-270, L272-275, L277-280, L282-285, L287-290, L292-295, L297-300, L302-305, L307-310. 16/16 ACs are Gherkin.
  - The author has added §5.1 Format Declaration (L192-201) arguing that "§5 AC is not EARS but EARS's verification scenario" and provides §5.2 Traceability Matrix (L203-228). This is a policy-change request directed at the audit rubric, not a format fix.
  - Per this auditor's M3 rubric (Score 0.50 band): "Given/When/Then test scenarios mislabeled as EARS = FAIL." Per M5 MP-2 firewall: "Every acceptance criterion must match one of the five EARS patterns." The §5.1 declaration does NOT relax MP-2; it reasserts a design choice that MP-2 explicitly prohibits. The criterion stands.
  - Note: §4 REQs (L128-186) are correctly EARS-formatted (Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional with `shall` wording). The defect remains isolated to §5 ACs. MP-2 gates on ACs, not REQs, so the isolation does not rescue the verdict.

- **[PASS] MP-3 YAML frontmatter validity**
  - Evidence: spec.md:L1..L14.
    - `id: SPEC-GOOSE-SKILLS-001` (L2) — string, correct pattern.
    - `version: 0.2.0` (L3) — string.
    - `status: planned` (L4) — string.
    - `created_at: 2026-04-21` (L5) — ISO date string. Field-name corrected from prior `created` (D1 resolved).
    - `priority: P0` (L8) — string.
    - `labels: [skills, progressive-disclosure, yaml-frontmatter, trigger-matching, security, go]` (L13) — array, 6 entries. Previously absent (D2 resolved).
  - All six required frontmatter fields present with correct types.

- **[N/A] MP-4 Section 22 language neutrality**
  - Evidence: The SPEC is scoped to a single implementation language (Go). L319-329 names `internal/skill/` package, L449-453 pins `gopkg.in/yaml.v3`, `github.com/denormal/go-gitignore`, `go.uber.org/zap`. The Skill *content* format (SKILL.md/YAML) is language-agnostic at the ecosystem level, but the SPEC targets Go library primitives exclusively. Auto-pass per M5 MP-4 N/A clause.

Must-Pass Result: 3 PASS (MP-1, MP-3), 1 N/A (MP-4), 1 FAIL (MP-2). **Overall = FAIL** (single must-pass failure is not compensable).

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75-1.0 transition band | REQs remain clearly worded in EARS. §2.4.1 (L67-74) resolves agentskills.io compatibility ambiguity. §2.4.2 (L76-87) + REQ-SK-020 (L178) explicitly declare L0-L3 effort and 3-Level Progressive Disclosure as **orthogonal axes** with a dimensional table (L80-84), eliminating the naming collision flagged in D14. REQ-SK-021 (L180) fixes 4-trigger scope, closing D15. Minor residual: REQs still embed Go identifiers (L128 `ParseSkillFile`/`ErrUnsafeFrontmatterProperty`, L134 `LoadSkillsDir`/`ErrDuplicateSkillID`, etc.) — acknowledged as API surface contract per §6.1-6.2 but not fully paraphrased to WHAT/WHY form (D11 unresolved). |
| Completeness | 0.90 | 0.75-1.0 band | All 6 required frontmatter fields present (L1-14). §2.4 new Standard Alignment section (L63-87) addresses agentskills.io + CLAUDE.md §13 reconciliation (D12/D14 resolved). 22 REQs cover schema (REQ-SK-019), orthogonality (020), scope (021), shell-injection threat model (022). Exclusions at L550-562 remain 10 specific entries. Gap: D16 (registry memory bounds / unload API / concurrent LoadSkillsDir behavior) not addressed in any REQ. |
| Testability | 0.85 | 0.75-1.0 transition band | All 16 ACs (including new AC-SK-011..016) are binary-testable with explicit assertions: AC-SK-012 enumerates 6 observable facts including `/tmp/pwned` non-creation, subprocess counter, `os.Stat` call counter (L287-290); AC-SK-013 requires `os.Getenv` call counter delta = 0 and regex assertion against literal secret leak (L292-295); AC-SK-014 requires `-race` detector pass and heap snapshot/reflect check (L297-300). No weasel words detected. Residual gap from D19: REQ-SK-009 + AC-SK-007 (L146, L262-265) still cover only the `true` branch of `disable-model-invocation`; default/false case and unknown actor strings are not tested. |
| Traceability | 1.0 | 1.0 band | §5.2 Traceability Matrix (L203-228) maps every REQ-SK-001..022 to one or more ACs. Every AC header at L232..L307 carries inline `(covers REQ-SK-XXX)` citation. Cross-checked: REQ-SK-002 (hooks ordering) → AC-SK-011 (L282-285); REQ-SK-013 (shell no-exec) → AC-SK-012 (L287-290); REQ-SK-014 (unknown var / no os.Getenv) → AC-SK-013 (L292-295); REQ-SK-016 (atomic Replace) → AC-SK-014 (L297-300); REQ-SK-017 → AC-SK-015; REQ-SK-018 → AC-SK-016. No orphan REQs, no orphan ACs. D4-D9 + D10 fully resolved. |

Weighted category average: ~0.90. Must-pass failure (MP-2) forces overall FAIL per M5 firewall; reported overall score 0.78 reflects the strong category recovery discounted by the binary must-pass hit.

---

## Defects Found

**D3 (iteration 1, carried forward).** spec.md:L232-310 — All 16 Acceptance Criteria (AC-SK-001..AC-SK-016) use Given/When/Then (Gherkin/BDD) format, not EARS. §5.1 Format Declaration (L192-201) is a policy argument to the auditor, not a format fix; it does not change the observable fact that no AC starts with `When`/`While`/`Where`/`If`/`The system shall`. Severity: **critical** (MP-2 blocker).

**D11 (iteration 1, carried forward, severity-reduced).** spec.md:L128, L134, L138, L140, L142, L144, L146, L150, L154, L160, L164 — REQ sentences continue to embed Go identifiers (`ParseSkillFile`, `LoadSkillsDir`, `ErrUnsafeFrontmatterProperty`, `FileChangedConsumer`, etc.). The author has not paraphrased to behavior form nor explicitly framed §4 as "API surface contract". However §6.1-6.2 provides the API surface explicitly, which partially mitigates the implementation-lock concern. Severity: **minor**.

**D16 (iteration 1, carried forward).** spec.md (entire) — No REQ bounds (a) maximum registry memory footprint, (b) body-read cache eviction policy, (c) concurrent `LoadSkillsDir` invocation behavior (re-entrancy, second-caller semantics), (d) teardown/unload API. The new REQ-SK-016 (atomic Replace) addresses in-place mutation but not these four sub-concerns. Severity: **minor**.

**D19 (iteration 1, carried forward).** spec.md:L146 + L262-265 — REQ-SK-009 specifies only the `disable-model-invocation: true` branch (`CanInvoke` returns `true` only for user/hook actors). Neither the REQ nor AC-SK-007 covers: (a) default/absent case (should model be allowed by default?), (b) `disable-model-invocation: false` explicit case, (c) unknown actor strings (e.g., `actor = "plugin"`). The actor matrix is incomplete. Severity: **minor**.

**D20 (new, iteration 2).** spec.md:L192-201 (§5.1) — The Format Declaration argues that "if the audit rule requires AC to be EARS, it is a tooling interpretation problem that §5.2 traceability resolves." This is a direct policy-contest with the audit rubric rather than a format fix. This is a noted but non-blocking procedural concern: the SPEC author is free to hold this position, but an audit report cannot accept it without the audit standard itself being revised upstream. This is flagged so downstream reviewers understand the unresolved tension. Severity: **minor** (procedural).

No other defects added beyond those listed above. D1, D2, D4-D10, D12-D15, D17, D18 all fully resolved (see Regression Check).

---

## Chain-of-Verification Pass

Second-look re-read executed on:
- L1-14 frontmatter (confirmed `created_at`, `labels`, all 6 required fields present)
- L128-186 full REQ enumeration (re-counted 22 REQs end-to-end, verified sequencing 001-022, confirmed no gaps and no duplicates; verified §4.6 L172-186 is an additive block not a renumbering)
- L232-310 all 16 ACs (each read, confirmed Gherkin pattern in 16/16; confirmed every AC header carries `(covers REQ-SK-XXX)` inline citation)
- L203-228 §5.2 Traceability Matrix (each row cross-checked against AC header; REQ-SK-001..022 all mapped at least once; AC-SK-001..016 all referenced)
- L67-87 §2.4 Standard Alignment (confirmed agentskills.io + CLAUDE.md §13 §2.4.2 both addressed)
- L550-562 Exclusions (10 specific entries retained; new entry at L559 `$ARG` substitution reinforces scope boundary)

Second-pass new findings:
- **D20 (procedural, new)**: §5.1 Format Declaration (L192-201) explicitly argues that "audit rule interpretation" is the blocker, not format. This is a procedural concern worth flagging: the SPEC has adopted a stance that deliberately contradicts the MP-2 audit criterion. Added to defect list at severity minor/procedural.
- Verified no regressions: every defect resolution in iteration 1's recommendation (D1, D2, D4-D10, D12-D15, D17, D18) landed correctly. HISTORY entry (L23) enumerates 10 explicit changes, each of which I cross-checked against the file.
- No defects from first pass were downgraded other than D11 (severity major → minor, because §6.1-6.2 API surface now explicitly backs the identifier-in-REQ usage).
- Confirmed REQ-SK-022 (shell threat model, L182-186) fully delegates enforcement to consumers without claiming the loader enforces a whitelist — this is self-consistent with §3.2 scope boundary.

---

## Regression Check

Defects from iteration 1:

| # | Description | Status | Evidence |
|---|---|---|---|
| D1 | `created` vs `created_at` field name | **RESOLVED** | L5 `created_at: 2026-04-21` |
| D2 | `labels:` field absent | **RESOLVED** | L13 `labels: [skills, progressive-disclosure, yaml-frontmatter, trigger-matching, security, go]` |
| D3 | 10 ACs in Gherkin, not EARS | **UNRESOLVED** | L232-310, 16 ACs remain Gherkin; §5.1 declaration is argument not fix |
| D4 | REQ-SK-002 (hooks ordering) has no AC | **RESOLVED** | AC-SK-011 at L282-285 |
| D5 | REQ-SK-013 (shell no-exec) has no AC | **RESOLVED** | AC-SK-012 at L287-290 with 6 assertions |
| D6 | REQ-SK-014 os.Getenv non-invocation untested | **RESOLVED** | AC-SK-013 at L292-295 with counter delta = 0 + regex |
| D7 | REQ-SK-016 (atomic Replace) has no AC | **RESOLVED** | AC-SK-014 at L297-300 with `-race` + 100 reader goroutines |
| D8 | REQ-SK-017 (PreferredModel) has no AC | **RESOLVED** | AC-SK-015 at L302-305 |
| D9 | REQ-SK-018 (ArgumentHint) has no AC | **RESOLVED** | AC-SK-016 at L307-310 |
| D10 | AC headers lack inline REQ citation | **RESOLVED** | Every AC header at L232..L307 carries `_(covers REQ-SK-XXX)_` |
| D11 | Go identifiers in REQ sentences | **UNRESOLVED** (minor, severity-reduced) | §6.1-6.2 API surface mitigates |
| D12 | agentskills.io standard alignment unstated | **RESOLVED** | §2.4.1 L67-74 declares "부분 호환 (compatible subset, with extensions)" + enumerates 호환/확장/차별/비호환 portions |
| D13 | YAML schema not promoted to REQ | **RESOLVED** | REQ-SK-019 at L176 enumerates exactly 15 keys + semver-minor policy for changes |
| D14 | L0-L3 vs CLAUDE.md §13 3-Level collision | **RESOLVED** | §2.4.2 L76-87 orthogonality table + REQ-SK-020 L178 fixes relationship as normative |
| D15 | keyword/agent/phase/language trigger scope | **RESOLVED** | §3.2 L120 explicit OUT + REQ-SK-021 L180 codifies exclusion + rejection path via REQ-SK-001 |
| D16 | Memory / unload / concurrent-load bounds | **UNRESOLVED** (minor) | No REQ added |
| D17 | Shell-injection threat model unspecified | **RESOLVED** | REQ-SK-022 at L182-186 covers (a) executable literal no-resolve, (b) deny-write as hint flag delegated, (c) hook command raw metacharacter preservation, (d) remote skill same-policy |
| D18 | REQ-SK-004 vs AC-SK-009 error-semantics contradiction | **RESOLVED** | REQ-SK-004 rewritten at L134 to match error-slice + partial-success semantics; AC-SK-009 at L272-275 aligned |
| D19 | REQ-SK-009 actor matrix incomplete | **UNRESOLVED** (minor) | No change to REQ or AC |

Stagnation check: No defect has appeared in both iterations with zero progress. D11/D16/D19 are carried forward unchanged, but the SPEC made substantive progress on 15 of 19 defects including all critical/major items from iteration 1 except D3. The sole blocker is D3, which the author has chosen to contest rather than fix.

---

## Recommendation

**Verdict: FAIL** (single must-pass failure: MP-2 / D3). Iteration 3 can achieve PASS by addressing D3 alone; D11/D16/D19/D20 are non-blocking.

**Required for iteration 3 PASS:**

1. **Rewrite all 16 ACs in EARS form (D3 / MP-2 blocker).** The §5.1 Format Declaration argument cannot be accepted by this audit regardless of its merit. Preserve the Gherkin scenarios as supporting test-plan prose inside each AC body, but the AC **header sentence** must match one of the five EARS patterns. Example transformation for AC-SK-001:
   - **Current (L232-235, Gherkin):** `Given /tmp/skills/hello/SKILL.md ... When LoadSkillsDir(...) Then 레지스트리에 1개 skill ...`
   - **Target (Event-Driven EARS):** `When LoadSkillsDir is invoked on a directory containing a valid SKILL.md with name=hello and description="say hi", the registry shall contain exactly one entry with ID=hello, Effort=L1, IsInline=true, IsConditional=false, and EstimateSkillFrontmatterTokens shall return a value derived from len(name)+len(description)+len(when-to-use). The Gherkin scenario below documents the test harness.` (then optionally keep Given/When/Then as sub-block for test implementation guidance.)
   - Apply the analogous rewrite to AC-SK-002..016. Retain `(covers REQ-SK-XXX)` citation.
   - If the author believes Gherkin has intrinsic value, preserve it as a *nested* verification block under each EARS-formatted AC — this satisfies MP-2 without losing the test scenario expressiveness the author values.

**Optional for iteration 3 (non-blocking, quality lift):**

2. **D11 — API surface framing.** Add a single sentence at the top of §4 clarifying that §4 REQs reference Go identifiers as the authoritative API-surface contract defined in §6.1-6.2. This removes the implementation-leak ambiguity without rewriting REQ wording.

3. **D16 — Memory / lifecycle REQs.** Add one or more REQs covering: registry memory bounds or absence thereof; concurrent `LoadSkillsDir` behavior (mutex-serialized vs reentrant); teardown/unload API or explicit statement that none is provided in v0.2.0. Alternative: add to Exclusions with explicit deferral to PLUGIN-001.

4. **D19 — Actor matrix completeness.** Extend REQ-SK-009 to cover the `false/absent` and `unknown-actor` branches; extend AC-SK-007 to test both branches.

5. **D20 — Format Declaration revision.** If iteration 3 addresses D3, §5.1 should be rewritten to describe *why both EARS headers and Gherkin scenarios coexist* (rather than arguing one replaces the other). This turns a procedural defect into a feature.

If iteration 3 FAILS because D3 is again contested rather than fixed, escalation to user is required with the explicit question: "Accept the SPEC despite MP-2 non-compliance (policy override), or require the manager-spec to rewrite AC headers in EARS form?"

---

**End of report**

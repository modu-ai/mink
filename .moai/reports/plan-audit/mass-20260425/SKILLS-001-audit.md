# SPEC Review Report: SPEC-GOOSE-SKILLS-001

Iteration: 1/3
Verdict: FAIL
Overall Score: 0.58

> Reasoning context ignored per M1 Context Isolation. The three inline system-reminders (workflow-modes.md, spec-workflow.md, moai-memory.md) and MCP server instructions are author-context noise and not part of the SPEC under audit. Audit is based exclusively on `spec.md` and `research.md`.

---

## Must-Pass Results

- **[PASS]** **MP-1 REQ number consistency**
  - Evidence: spec.md:L99..L141 enumerates REQ-SK-001 through REQ-SK-018 in sequence. Verified individually: 001(L99), 002(L101), 003(L103), 004(L105), 005(L109), 006(L111), 007(L113), 008(L115), 009(L117), 010(L121), 011(L123), 012(L125), 013(L129), 014(L131), 015(L133), 016(L135), 017(L139), 018(L141). No gaps, no duplicates, consistent 3-digit zero-padding.

- **[FAIL]** **MP-2 EARS format compliance**
  - Evidence: spec.md:L145..L196 (§5 수용 기준). Every AC uses the Given/When/Then (Gherkin/BDD) template, NOT one of the five EARS patterns.
  - Example, spec.md:L148-150 — `AC-SK-001` uses `Given ... When ... Then ...` structure.
  - Same pattern across AC-SK-001 through AC-SK-010 (10/10 ACs).
  - Per M3 rubric: "Given/When/Then test scenarios mislabeled as EARS = FAIL".
  - Note: §4 REQUIREMENTS (L95..L141) DOES use EARS correctly (Ubiquitous / Event-Driven / State-Driven / Unwanted / Optional labels, `shall` wording). The defect is isolated to §5 ACs. However MP-2 explicitly gates on the ACs.

- **[FAIL]** **MP-3 YAML frontmatter validity**
  - Evidence: spec.md:L1..L13.
  - L5 reads `created: 2026-04-21`. Required field per rubric is `created_at` (ISO date). Field-name mismatch = missing required field.
  - `labels:` field is entirely absent (not anywhere in L1..L13). Required field missing.
  - Two required fields failing = FAIL.
  - Passing fields: `id` (L2), `version` (L3), `status` (L4), `priority` (L8).

- **[N/A]** **MP-4 Section 22 language neutrality**
  - N/A: this SPEC is scoped to a single implementation language (Go — `internal/skill/` package, `gopkg.in/yaml.v3`, `github.com/denormal/go-gitignore`). No multi-language tool enumeration is required. The Skill *content* is language-agnostic, but the SPEC targets Go library primitives exclusively. Auto-pass per rubric.

Must-Pass Result: 2 PASS (MP-1, MP-4 N/A), 2 FAIL (MP-2, MP-3). **Overall = FAIL**.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band (minor ambiguity) | REQs are clearly worded in EARS; however, REQ text embeds Go identifiers (spec.md:L99 `ParseSkillFile`/`ErrUnsafeFrontmatterProperty`, L105 `LoadSkillsDir`/`ErrDuplicateSkillID`, L113 `FileChangedConsumer`) which confuse behavior vs. API surface. "open issues" in research.md §8 (L263-270) explicitly admit 5 unresolved semantic ambiguities (`model: inherit` meaning, tokenizer accuracy, Remote auth). |
| Completeness | 0.50 | 0.50 band (multiple gaps) | Two required frontmatter fields missing (`created_at`, `labels`). Special-focus gaps: agentskills.io standard compatibility NOT stated anywhere in spec.md or research.md. YAML frontmatter schema keys are enumerated only inside a Go code block at spec.md:L280-286 (`SAFE_SKILL_PROPERTIES`), not fixed as a REQ sentence. |
| Testability | 0.75 | 0.75 band | ACs are binary-testable (spec.md:L148-196). No weasel words detected. But several REQs lack any AC (see Traceability). |
| Traceability | 0.50 | 0.50 band (multiple gaps) | ACs do not inline-cite REQ-SK-XXX numbers (spec.md:L147-196). Mapping exists only in research.md:L243-254, which is a secondary document. Multiple REQs are uncovered: REQ-SK-002, REQ-SK-013, REQ-SK-014 (partial), REQ-SK-016, REQ-SK-017, REQ-SK-018. See D4-D9. |

---

## Defects Found

**D1.** spec.md:L5 — YAML frontmatter uses `created` instead of the required `created_at` field. Severity: **critical** (MP-3 contributor).

**D2.** spec.md:L1-13 — YAML frontmatter is missing the required `labels` field entirely. Severity: **critical** (MP-3 contributor).

**D3.** spec.md:L147-196 — All 10 Acceptance Criteria (AC-SK-001..010) are written in Given/When/Then (Gherkin/BDD) template, not EARS. MP-2 rubric explicitly rejects Given/When/Then test scenarios mislabeled as ACs. Severity: **critical** (MP-2 contributor).

**D4.** spec.md:L101 — REQ-SK-002 (hook handler ordering preservation) has NO corresponding AC in §5. The research.md AC-matrix table (L243-254) omits REQ-SK-002 entirely. Severity: **major**.

**D5.** spec.md:L129 — REQ-SK-013 (shell directive MUST NOT execute at parse time) has NO corresponding AC. This is a security-critical requirement left untested. Severity: **major** (security-adjacent).

**D6.** spec.md:L131 — REQ-SK-014 (unknown `${VAR}` must not leak `os.Getenv` and must not fail the load) is only partially covered. AC-SK-006 (L173-175) verifies literal retention of unknown variables, but NO AC verifies the explicit `os.Getenv` non-invocation / secret non-exposure behavior. Severity: **major** (security-critical requirement under-tested).

**D7.** spec.md:L135 — REQ-SK-016 (registry MUST NOT mutate in-place, must go through atomic `Replace`) has NO AC. Research.md:L238 mentions `TestRegistry_Replace_AtomicSwap` as unit test but no integration AC in spec.md §5. Severity: **major**.

**D8.** spec.md:L139 — REQ-SK-017 (`model: opus[1m]` recording as `PreferredModel`) has NO AC. Severity: **minor**.

**D9.** spec.md:L141 — REQ-SK-018 (`argument-hint:` exposure via `ArgumentHint`) has NO AC. Severity: **minor**.

**D10.** spec.md:L147-196 — No AC inline-cites its source REQ-SK-XXX number. Traceability depends on external document (research.md). AC-to-REQ mapping should be embedded in each AC header line (e.g., "AC-SK-003 — effort 매핑 (covers REQ-SK-010)"). Severity: **minor**.

**D11.** spec.md:L99, L105, L109, L111, L113, L115, L117, L121, L125, L135 — Several REQ sentences contain Go identifiers (`ParseSkillFile`, `LoadSkillsDir`, `EstimateSkillFrontmatterTokens`, `ResolveBody(session)`, `FileChangedConsumer`, `IsForked(fm)`, `CanInvoke(skill, actor)`, `EffortLevel`, `LoadRemoteSkill`, `Replace(id, newDef)`). These are implementation HOW, not behavior WHAT. For a Go library SPEC this is common but should be paraphrased to behavior form or explicitly framed as "API surface contract" to avoid implementation drift lock-in. Severity: **minor**.

**D12.** spec.md (entire) + research.md (entire) — NO mention of the `agentskills.io` standard anywhere. The user asked explicitly: "Claude Code Agent Skills 표준 (agentskills.io)와의 호환성 여부 명시". The SPEC cites only `claude-primitives.md §2` and the TypeScript source map as upstream references. Compatibility with the public `agentskills.io` spec is neither affirmed nor denied. Severity: **major** (completeness — user-requested scope item).

**D13.** spec.md:L280-286 — The YAML frontmatter schema (15 allowed keys in `SAFE_SKILL_PROPERTIES`) is fixed only inside a Go source-code block in §6.2 Technical Approach. It is NOT fixed as a REQ sentence. REQ-SK-001 (L99) only references "the allowlist" without enumerating keys. If §6.2 is interpreted as non-normative (technical approach = HOW), then the schema is under-specified as a behavioral contract. Severity: **major**.

**D14.** spec.md (entire) — Progressive Disclosure is defined as 4-level (L0/L1/L2/L3) across REQ-SK-010 and §2.2 research.md. The user's audit focus asked about "3-level" progressive disclosure. Either the user's prompt referenced a different system (CLAUDE.md Section 13 mentions 3-Level Progressive Disclosure: Level 1/2/3 = Metadata/Body/Bundled), OR the SPEC diverges from that existing MoAI 3-level system without noting the difference. The SPEC does not reconcile L0~L3 (effort tiers) with the pre-existing 3-level progressive disclosure (metadata/body/bundled). Severity: **major** (naming/system collision risk).

**D15.** spec.md §4 (entire) — Trigger matching algorithm is defined as 4 triggers (inline/fork/conditional/remote) based on Claude Code TS upstream. The user's audit focus enumerated "keywords, agents, phases, languages" as the expected trigger dimensions — these are MoAI-ADK SKILL.md frontmatter conventions (e.g., `triggers: keywords: [...]` in existing `.claude/skills/` files). The SPEC does NOT address keyword/agent-name/phase/language-based activation AT ALL. If the target product is MoAI-style multi-dimensional trigger matching, the SPEC is incomplete. If the target is strictly Claude Code 4-trigger porting, this should be stated explicitly as a scope decision. No such statement exists. Severity: **major** (scope ambiguity).

**D16.** spec.md (entire) — Skill loading timing and memory management are addressed implicitly ("eager walk + lazy body read" at spec.md:L52, research.md:L120-121) but no REQ bounds (a) maximum registry memory, (b) body-read cache eviction policy, (c) concurrent LoadSkillsDir invocation behavior, (d) teardown/unload API. The user's audit focus explicitly asked about this. Severity: **minor**.

**D17.** spec.md:L129-131 — Shell command injection boundary is under-specified. REQ-SK-013 prohibits parse-time execution; §6.4 table (L314-322) prohibits `os.Getenv`; §3.2 (L86) excludes `!command` expansion. But NO REQ enumerates the shell-injection threat model: (a) Is `shell.executable` path validated against a whitelist? (b) Does `shell.deny-write` enforce at the consumer or at SKILL parse? (c) What happens if a hook command contains shell metacharacters? Research.md does not address these. Severity: **major** (security boundary under-specified).

**D18.** spec.md:L105 — REQ-SK-004 says "duplicate IDs **shall** cause `LoadSkillsDir` to return `ErrDuplicateSkillID`". AC-SK-009 (L188-190) says "두 번째 파일은 `ErrDuplicateSkillID`로 error slice에 포함, 첫 번째만 레지스트리 진입" — the AC says the error goes to an error slice (non-fatal), but REQ-SK-004 says the loader "shall return" the error (ambiguous between return-value-error and error-slice-element). Similar ambiguity in REQ-SK-005 clause (e). The REQ and AC disagree on failure semantics: fatal return vs. collected slice. Severity: **major** (REQ/AC contradiction).

---

## Chain-of-Verification Pass

Re-read pass executed on:
- L1-13 frontmatter (confirmed `created` vs `created_at`, `labels` absent)
- L95-141 full REQ enumeration (re-counted 18 REQs, verified sequencing 001-018 end-to-end, not spot-checked)
- L145-196 all 10 ACs (each read, confirmed Given/When/Then pattern; confirmed none cite REQ-SK-XXX inline)
- L431-443 Exclusions (10 specific entries — good specificity, no weasel items)
- L218-286 Technical Approach code block (confirmed SAFE_SKILL_PROPERTIES enumerates 15 keys in §6.2, not in REQ body)

Second-pass new findings:
- **D18 is a second-pass finding**: REQ-SK-004 "shall return" vs AC-SK-009 "error slice에 포함" contradiction was missed in first pass because REQ read as API signature and AC as behavior narrative. On re-read of REQ-SK-005 clause (e) (L109 "return the populated registry plus an error slice — no partial failure aborts the full load"), the semantics clarify: partial-success with error-slice is the intent. But REQ-SK-004 does NOT restate this, leaving ambiguity. Added as D18.
- **D19 (new on re-read)**: spec.md:L117 REQ-SK-009 says `CanInvoke` returns `true` only for `actor = "user"` or `actor = "hook"` when `disable-model-invocation: true`. But REQ-SK-009 does NOT specify default behavior when `disable-model-invocation: false` (implicit or absent). AC-SK-007 (L179-180) tests only the `true` case. Missing AC for the default/false case + missing REQ wording for non-model/non-user/non-hook actors (e.g., unknown actor names). Severity: **minor** — added below.

**D19.** spec.md:L117 + L179-180 — REQ-SK-009 and AC-SK-007 do not specify `CanInvoke` behavior when `disable-model-invocation` is false/absent (should return `true` for model), nor for unknown `actor` strings. Severity: **minor**.

No defects from first pass were downgraded on re-read. All D1-D17 confirmed.

---

## Regression Check

Iteration 1 — no prior iterations to regress against. N/A.

---

## Recommendation

**Verdict: FAIL.** manager-spec must address the following before re-submission:

1. **Fix YAML frontmatter (MP-3 blocker)**:
   - Rename `created: 2026-04-21` → `created_at: 2026-04-21` at spec.md:L5.
   - Add `labels: [skills, progressive-disclosure, yaml-frontmatter, trigger-matching, security]` (or equivalent) to the frontmatter block.

2. **Rewrite all 10 ACs in EARS format (MP-2 blocker)**:
   - Convert AC-SK-001..010 from Given/When/Then to EARS (Event-Driven/State-Driven/Unwanted pattern).
   - Example for AC-SK-001: "When `LoadSkillsDir("/tmp/skills")` is invoked on a directory containing `/tmp/skills/hello/SKILL.md` with valid frontmatter `name: hello, description: "say hi"`, the registry shall contain exactly one skill with `ID=hello`, `Effort=L1`, `IsInline=true`, `IsConditional=false`, and `EstimateSkillFrontmatterTokens()` shall return a value derived from `len("hello")+len("say hi")`."
   - Each AC must inline-cite its source REQ-SK-XXX (addresses D10).

3. **Add missing ACs** for REQ-SK-002 (D4), REQ-SK-013 (D5), REQ-SK-014 security sub-clause (D6), REQ-SK-016 (D7), REQ-SK-017 (D8), REQ-SK-018 (D9), REQ-SK-009 false/default case (D19).

4. **Resolve REQ/AC contradiction** (D18): clarify REQ-SK-004 to match AC-SK-009's "error slice, partial success" semantics. Restate as "duplicate IDs **shall** cause `LoadSkillsDir` to include `ErrDuplicateSkillID` in the returned error slice for the offending file; first-registered entry is preserved; load does not abort."

5. **Add explicit scope statements** for the three items in the special-focus list the audit flagged:
   - D12: Add a statement in §2 or §9 on `agentskills.io` standard compatibility status (compatible / superset / divergent, and how).
   - D14: Reconcile L0-L3 effort tiers with MoAI's pre-existing 3-level progressive disclosure (CLAUDE.md §13). Add a note in §2 or a dedicated REQ.
   - D15: State explicitly whether keyword/agent/phase/language-based trigger matching is IN or OUT of scope. If OUT, defer to a future SPEC with ID. If IN, add REQs.

6. **Promote the YAML schema to a REQ (D13)**: add a new REQ-SK-XXX that enumerates the 15 `SAFE_SKILL_PROPERTIES` keys as the authoritative schema, not a code-block-only definition. Alternative: mark §6.2 as normative.

7. **Tighten security boundary (D17)**: add REQs that enumerate shell-injection threat model (executable path validation, metacharacter handling, `shell.deny-write` enforcement point).

8. **Address remaining minor defects** (D11 implementation-leak language, D16 memory management, D19 actor matrix completeness) as lower-priority cleanup.

Re-submit as iteration 2 after addressing the four critical items (D1, D2, D3, and at minimum D4-D7 + D18). Must-pass fixes alone (D1, D2, D3) are insufficient; traceability (D4-D9, D18) must also reach PASS to clear overall FAIL in the next iteration.

---

**End of report**

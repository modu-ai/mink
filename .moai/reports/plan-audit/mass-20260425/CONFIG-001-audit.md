# SPEC Review Report: SPEC-GOOSE-CONFIG-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Audit based on spec.md (340 lines) and research.md was not required for this verdict because spec.md alone contains blocking defects.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-CFG-001 through REQ-CFG-014 are sequential, zero-padded (3 digits), no gaps, no duplicates. Verified by enumeration at spec.md:L92, L94, L96, L100, L102, L104, L108, L110, L114, L116, L118, L120, L124, L126.

- [FAIL] MP-2 EARS format compliance:
  - REQ-CFG-011 (spec.md:L118) is labeled `[Unwanted]` but uses Ubiquitous-negative form ("The loader **shall not** read from $HOME when..."). Unwanted EARS pattern requires "If [undesired condition], then the [system] shall [response]". The conditional "when $GOOSE_HOME is empty" is embedded as subordinate clause, not as EARS "If" trigger. The second sentence "it shall instead fall back..." is a compound positive requirement, not Unwanted.
  - REQ-CFG-012 (spec.md:L120) is labeled `[Unwanted]` but reads "The loader **shall not** expand shell variables..." — this is a plain prohibition, not an Unwanted-EARS conditional. No "If X, then Y shall Z" structure.
  - Both misclassifications = FAIL per MP-2.

- [FAIL] MP-3 YAML frontmatter validity:
  - `status: Planned` (spec.md:L4) — not in the standard enum {draft, active, implemented, deprecated}. "Planned" is not a documented lifecycle value.
  - Field named `created` (spec.md:L5) not `created_at` — MP-3 explicitly requires `created_at (ISO date string)`. Missing required field name.
  - `priority: P0` (spec.md:L8) — not in standard enum {critical, high, medium, low}. Uses alternate scheme (P0/P1/P2) without documented mapping.
  - `labels` field is **entirely absent** from frontmatter. MP-3 lists labels as required (array or string). Missing = FAIL.
  - Extra non-standard fields present (`issue_number`, `phase`, `size`, `lifecycle`, `updated`, `author`) — not inherently wrong, but no `labels` and no `created_at` means required fields are missing.

- [N/A] MP-4 Section 22 language neutrality: N/A — SPEC is scoped to Go (goose-agent runtime). No multi-language tooling enumeration required. Auto-passes per M5.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band | REQs are generally precise. Minor ambiguity: REQ-CFG-007 (L108) couples Validate() and IsValid() without defining ordering of public getter semantics. REQ-CFG-008 (L110) says "preserved in Config.Unknown" but REQ-CFG-014 (L126) says strict mode "shall instead cause load failure" — interaction with nested unknown keys (only top-level per L110) is unclear. |
| Completeness | 0.60 | 0.50 band | Required sections present (HISTORY L17, Overview L25, Background L37, Scope L63, Requirements L88, Acceptance Criteria L130, Technical Approach L174, Dependencies L279, Risks L293, References L305, Exclusions L326). However: frontmatter `labels` missing (L1-L13), no explicit `created_at` field, no CREDPOOL cross-reference despite "secret" type in env table L200-L201 (see Defect D4). |
| Testability | 0.75 | 0.75 band | ACs are mostly binary-testable (L132-L170) with concrete Given/When/Then. Weasel phrases minimal. AC-CFG-006 (L157-L160) says "WARN 로그 1건" — testable if log sink is injected. However, AC-CFG-003 (L142-L145) asserts `ErrConfigError{File, Line:2}` but YAML v3 does not always deliver exact line numbers for all syntax errors (e.g., unclosed brackets); this may be untestable as written. |
| Traceability | 0.30 | 0.25 band | **Critical defect**. Acceptance Criteria AC-CFG-001 through AC-CFG-008 (L132-L170) do NOT cite any REQ-CFG-XXX IDs. No explicit mapping exists. Reverse-tracing by content: REQ-CFG-003 (concurrent reads, L96), REQ-CFG-007 (IsValid state, L108), REQ-CFG-010 (type mismatch naming, L116), REQ-CFG-011 ($GOOSE_HOME fallback, L118), REQ-CFG-012 (no shell expansion, L120), REQ-CFG-013 (OverrideFiles option, L124) have no corresponding AC. 6 of 14 REQs (~43%) are uncovered. Score capped at 0.30. |

## Defects Found

**D1.** spec.md:L1-L13 — YAML frontmatter missing `labels` field (MP-3 required). — Severity: **critical**

**D2.** spec.md:L5 — Field name `created` does not match MP-3 required field `created_at`. Rename required, or document deviation. — Severity: **critical**

**D3.** spec.md:L4 — `status: Planned` uses a value not in standard enum {draft, active, implemented, deprecated}. Either change to `draft` or document the non-standard lifecycle. — Severity: **major**

**D4.** spec.md:L8 — `priority: P0` uses non-standard scheme. Standard enum is {critical, high, medium, low}. — Severity: **major**

**D5.** spec.md:L118 (REQ-CFG-011) — Labeled `[Unwanted]` but structurally is Ubiquitous-negative with embedded conditional. Does not match Unwanted EARS pattern "If X, then Y shall Z". — Severity: **critical** (MP-2)

**D6.** spec.md:L120 (REQ-CFG-012) — Labeled `[Unwanted]` but is a plain prohibition without "If-then" structure. — Severity: **critical** (MP-2)

**D7.** spec.md:L132-L170 (AC section) — Zero explicit REQ-CFG-XXX citations in any AC. Given/When/Then describes behavior but breaks traceability audit trail. At minimum each AC must end with "Satisfies: REQ-CFG-XXX". — Severity: **critical** (Traceability)

**D8.** spec.md:L96 (REQ-CFG-003) — No AC covers concurrent-read safety claim. Either remove REQ or add AC with concurrent reader test. — Severity: **major**

**D9.** spec.md:L108 (REQ-CFG-007) — No AC covers the "IsValid() returns false before Validate()" state-driven behavior. — Severity: **major**

**D10.** spec.md:L116 (REQ-CFG-010) — No AC covers the type-mismatch-with-named-field-path error. AC-CFG-004 (L147-L150) covers port range (REQ-CFG-004 wording) but not "string where int expected". — Severity: **major**

**D11.** spec.md:L118 (REQ-CFG-011) — No AC covers `$GOOSE_HOME` empty → `$HOME/.goose` fallback. — Severity: **major**

**D12.** spec.md:L120 (REQ-CFG-012) — No AC covers literal-string (no shell expansion) behavior. — Severity: **major**

**D13.** spec.md:L124 (REQ-CFG-013) — No AC covers `LoadOptions.OverrideFiles` test-only path. — Severity: **major**

**D14.** spec.md:L47-L54 — Section 2.2 "viper 사용 여부" reads as implementation justification (HOW), not requirement. It belongs in research.md. In WHY section this is acceptable, but the decision "viper 미사용" becomes a constraint that is not captured as a REQ or negative requirement. — Severity: **minor**

**D15.** spec.md:L192-L203 — Env mapping table conflates public (log/transport/ui) and secret fields (OPENAI_API_KEY, ANTHROPIC_API_KEY) without cross-reference to a credential-pool SPEC. User requested specifically: "secret 참조 (CREDPOOL 연동)". SPEC §3.2 OUT OF SCOPE (L79) lists "Secret/credential 저장소 통합" but does not reference CREDPOOL / keychain bridging. No REQ addresses "if a credential-pool SPEC exists, secret-typed env vars shall defer to it". — Severity: **major**

**D16.** spec.md:L142-L145 (AC-CFG-003) — Asserts `*ConfigError{File:"...", Line:2}` exact line number. YAML v3 syntax errors do not always yield exact column/line for every malformation class. This AC may be untestable for some syntax error types. Tighten to "Line field is populated and matches document line if parser provides it". — Severity: **minor**

**D17.** spec.md:L167-L170 (AC-CFG-008) — Covers `GOOSE_LOG_LEVEL` and `GOOSE_LEARNING_ENABLED` but does not cover `GOOSE_GRPC_PORT` int overlay, `OLLAMA_HOST` URL overlay, or `OPENAI_API_KEY` secret overlay. Env mapping table (L192-L203) has 10 rows but only 2 are tested. — Severity: **major**

**D18.** spec.md:L242 (Deep Merge Rule 2) — "overlay가 zero-value가 아니면 override" creates a contradiction: if user explicitly sets `learning.enabled: false` (Go bool zero-value) over default `true`, the merge algorithm as stated will IGNORE the user's explicit false. This is a silent bug in the algorithm spec. Need to distinguish "unset" from "set to zero-value". — Severity: **critical**

**D19.** spec.md:L299 (Risk R3) — Mitigation "파일 크기 256KB cap" is not mirrored into a REQ or AC. If it is a real constraint, it needs REQ-CFG-015 and AC-CFG-009. — Severity: **minor**

**D20.** spec.md:L44 — References `.moai/config/sections/*.yaml` pattern, but SPEC adopts **single-file** `~/.goose/config.yaml` (L33) without documenting why the sectioned layout was rejected. Inconsistency with cited precedent. — Severity: **minor**

## Chain-of-Verification Pass

Second-look findings after re-reading:

- Re-verified MP-1: enumerated all 14 REQ numbers manually — confirmed sequential L92→L126, no gaps.
- Re-verified AC section (L132-L170): confirmed all 8 ACs lack explicit REQ citations. Not just sampled — exhaustive check.
- Re-checked Exclusions (L326-L336): 9 specific items, each concrete and scoped. This section passes.
- New finding (D18): Deep merge Rule 2 zero-value semantic collision. Missed on first pass because the algorithm was not mapped against example env table values (learning.enabled bool is a concrete case).
- New finding (D20): single-file vs sectioned config inconsistency with `.moai/config/sections/*.yaml` precedent cited in WHY.
- Re-examined REQ-CFG-004 (L100) vs AC-CFG-004 (L147-L150): REQ-004 is event-driven "when $GOOSE_HOME/config.yaml exists, parse and merge", but AC-004 covers port-range validation — these do not correspond. AC-CFG-004's REQ counterpart is actually REQ-CFG-010 (L116, type/range). Confirms D7 (no explicit mapping) leads to content-drift errors.
- Confirmed no contradictions between REQs that I missed.

Chain-of-Verification adds defects D18 and D20 to final list.

## Regression Check

N/A — Iteration 1.

## Recommendation

**FAIL. Manager-spec must revise before iteration 2.** Required fixes in priority order:

1. **Frontmatter (MP-3)**: Add `labels:` field (array). Rename `created` → `created_at`. Change `status: Planned` → `status: draft`. Either change `priority: P0` → `priority: critical`, or document MoAI-ADK's P0/P1 mapping in a frontmatter schema reference.

2. **EARS compliance (MP-2)**: Rewrite REQ-CFG-011 (L118) and REQ-CFG-012 (L120) in Unwanted-EARS form: "If [trigger], then the loader shall [response]". Example for REQ-CFG-011: "If `$GOOSE_HOME` is empty, then the loader shall use `$HOME/.goose` as the resolved user config directory and shall not dereference `$HOME` otherwise." Or reclassify them as `[Ubiquitous]` with negative form.

3. **Traceability (critical)**: Append `Satisfies: REQ-CFG-XXX[, REQ-CFG-YYY]` line to each AC (L132-L170). Add missing ACs for REQ-CFG-003 (concurrent read), REQ-CFG-007 (IsValid state), REQ-CFG-010 (type naming), REQ-CFG-011 (HOME fallback), REQ-CFG-012 (no shell expansion), REQ-CFG-013 (OverrideFiles).

4. **Deep merge bug (D18)**: Rewrite §6.4 Rule 2 to distinguish "field is unset in overlay YAML" from "field is explicitly set to zero-value". Use `yaml.Node` presence detection or sentinel pointer-wrapping. Add AC verifying explicit `learning.enabled: false` overrides default `true`.

5. **Secret/CREDPOOL cross-reference (D15)**: Add a dependency row (§7) pointing to the credential-pool SPEC if one exists, or add an explicit REQ-CFG-015 stating "Where a future credential-pool SPEC is adopted, secret-typed fields (`llm.providers.*.api_key`) shall be sourced from that SPEC's resolver in preference to env/YAML." Current OUT-OF-SCOPE entry L79 is too terse.

6. **Env overlay coverage (D17)**: Expand AC-CFG-008 or add AC-CFG-009/010 to cover int, URL, and secret env overlays. Currently only enum + bool are tested.

7. **AC-CFG-003 tightening (D16)**: Replace "Line:2" with "Line field populated when parser reports it; File field matches input path".

8. **Minor**: Document R3 (file size cap) as REQ-CFG-NNN. Address single-file vs sectioned layout inconsistency (D20) either by changing the layout or by documenting the rejection in §2.

**Do not PASS this SPEC until items 1-5 are resolved.** Items 6-8 may be deferred to iteration 2 if items 1-5 land cleanly.

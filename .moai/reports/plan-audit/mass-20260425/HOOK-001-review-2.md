# SPEC Review Report: SPEC-GOOSE-HOOK-001

Iteration: 2/3
Verdict: **FAIL**
Overall Score: 0.65

> Reasoning context ignored per M1 Context Isolation. Only `spec.md` (v0.2.0) and the iter-1 report (`HOOK-001-audit.md`) were read for this audit. System reminders about MoAI workflow rules noted but did not influence verdicts.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency** — spec.md:L113–L165. 22 REQs present (REQ-HK-001 through REQ-HK-022). No gaps, no duplicates, consistent 3-digit padding. **Caveat**: document ordering is non-monotonic — §4.4 Unwanted block is 014→015→016→017→018→**021→022** (L147–L159), then §4.5 Optional block is **019→020** (L163, L165). All numbers 001–022 are present exactly once, so MP-1 passes, but the out-of-order placement is a minor readability defect (see D14).

- [FAIL] **MP-2 EARS format compliance** — spec.md:L171–L222. **All 10 ACs remain in Given/When/Then (Gherkin) format**. v0.2.0 adds a "Format declaration" preamble at L171 arguing EARS compliance should be evaluated only against §4 REQs and that the AC section is "intentionally authored in Given/When/Then test-scenario format". This rationalization does **not** override the MP-2 firewall. Per M5: "Every acceptance criterion must match one of the five EARS patterns listed in M3. Informal language, Given/When/Then test scenarios mislabeled as EARS, or mixed informal/formal within a single criterion = FAIL." Adding a disclaimer that redirects the auditor to §4 does not make the ACs themselves EARS-compliant. The REQ blocks are EARS-compliant (PASS on REQ text), but the AC section fails MP-2. Evidence: L173–L177 (AC-HK-001 Given/When/Then/And), L179–L182 (AC-HK-002), ..., L219–L222 (AC-HK-010). Remediation required: either (a) rewrite each AC as a single EARS sentence, or (b) keep Given/When/Then as a secondary "test scenario" block AND add a parallel EARS-format assertion line per AC.

- [PASS] **MP-3 YAML frontmatter validity** — spec.md:L1–L14. Required fields now present:
  - `id: SPEC-GOOSE-HOOK-001` (L2)
  - `version: 0.2.0` (L3)
  - `status: planned` (L4)
  - `created_at: 2026-04-21` (L5) — **field-name fix vs iter-1**
  - `priority: P0` (L8)
  - `labels: [hook, dispatcher, permission, phase-2, goose-agent]` (L13) — **added vs iter-1**
  All six required fields present with correct types. MP-3 now passes.

- [N/A] **MP-4 Section 22 language neutrality** — Auto-passes. Single-language Go SPEC scoped to `internal/hook/` package (§6.1 L228–L239, §6.8 L456–L464 dependency table with stdlib + zap + testify). No 16-language enumeration expected.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.65 | 0.50–0.75 band | Most REQs unambiguous. Implementation-detail leaks persist (see D5-residual) and new REQ-HK-021 adds substantial implementation coupling (syscall.Setrlimit, `RLIMIT_AS=1GiB`, `O_CLOEXEC` at L157). Ambiguity in REQ-HK-021(b): "session workspace root (resolved from `HookInput.SessionID`)" — resolution function is unspecified. |
| Completeness | 0.80 | 0.75 band | HISTORY (L18–L23), Overview (§1 L27–L39), Background (§2 L43–L59), Scope (§3 L63–L105), REQUIREMENTS (§4 L109–L165), ACCEPTANCE CRITERIA (§5 L169–L222), Technical Approach (§6 L226–L489), Dependencies (§7 L493–L506), Risks (§8 L510–L520), References (§9 L524–L545), Exclusions (L549–L560) all present and substantive. Frontmatter complete. Deduction: Registry API in §6.2 omits the `SkillsFileChangedConsumer` registration method referenced by REQ-HK-008 (D11 unresolved). |
| Testability | 0.70 | 0.50–0.75 band | ACs are binary-testable as written (each Given/When/Then maps to a unique test case in §6.9 L467–L477). But 12 of 22 REQs still lack AC coverage; testability of those uncovered REQs relies only on research.md unit tests, which are not spec-level gates. No weasel words detected. |
| Traceability | 0.45 | 0.25–0.50 band | **Regressed**. Iter-1 had 10/20 REQs uncovered (~50%); v0.2.0 added 2 new REQs (021, 022) without corresponding ACs, so coverage is now **10/22 covered, 12/22 uncovered (~55% uncovered)**. Covered: REQ-HK-001 (AC-001), REQ-HK-003 (AC-010), REQ-HK-005 (AC-002), REQ-HK-006 (AC-003/004), REQ-HK-007 (AC-005), REQ-HK-008 (AC-006), REQ-HK-009 (AC-008/009), REQ-HK-011 (AC-002), REQ-HK-012 (AC-007), REQ-HK-017 (AC-009). Uncovered: REQ-HK-002, 004, 010, 013, 014, 015, 016, 018, 019, 020, **021 (new, critical safety)**, **022 (new, critical safety)**. The two newest REQs address the most impactful safety concerns from iter-1 (D8 subprocess isolation, D9 payload cap) but carry no AC — ironic given they were added in response to iter-1 defects. |

---

## Defects Found

### Unresolved from iter-1

- **D3 (critical, UNRESOLVED)** — spec.md:L169–L222 — All 10 ACs remain Given/When/Then. v0.2.0 HISTORY (L23) claims "D4 (major) §5 AC 섹션 'Given/When/Then format' 명시적 선언" resolves D4, but this actually targets D3 (MP-2 firewall) and the response is an **explicit rationalization** that M5 instructs the auditor to reject ("HARD RULES: NEVER rationalize acceptance of a problem you identified ... A PASS verdict without concrete evidence is malpractice"). Adding a disclaimer does not convert Gherkin scenarios into EARS statements. **Severity: critical**

- **D4 (major, REGRESSED)** — spec.md:L113–L165 vs L173–L222 — REQ→AC coverage has **worsened**, not improved. Iter-1: 10/20 REQs uncovered. v0.2.0: **12/22 uncovered** (the two new REQs HK-021 and HK-022 introduced to address iter-1 D8 and D9 have no corresponding AC). REQ-HK-021 specifies four concrete isolation guarantees (env scrub, CWD pin, rlimits, FD hygiene) — each is a discrete, testable contract and each is unverified. REQ-HK-022 specifies a 4 MiB cap and goroutine stdin write — also unverified by any AC. **Severity: major**

- **D5 (major, WORSENED)** — spec.md:L119, L125, L129, L131, L135, L143, L149, L163 — original implementation-detail leaks from iter-1 are unchanged. v0.2.0 adds substantially more: REQ-HK-021 (L157) embeds the literal regex pattern `(?i)(token|secret|password|apikey|api_key)`, exact env var names (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOSE_AUTH_*`), exact rlimit constants with magic numbers (`RLIMIT_AS=1GiB`, `RLIMIT_NOFILE=128`, `RLIMIT_CPU=cfg.Timeout + 5s`), specific syscalls (`syscall.Setrlimit`, `syscall.CloseOnExec`, `O_CLOEXEC`), and specific error type `ErrHookPayloadTooLarge` (L159). These belong in §6 Technical Approach, not §4 Requirements. A requirement should state the observable contract; the implementation names should be changeable without amending a REQ. **Severity: major**

- **D7 (major, UNRESOLVED)** — spec.md:L147–L155 — REQ-HK-014 through REQ-HK-018 remain "shall not" Ubiquitous-negative form rather than proper EARS Unwanted (`If [undesired condition], then the [system] shall [response]`). Only REQ-HK-021 (L157) uses the correct Unwanted pattern starting with "If". REQ-HK-022 (L159) reverts to "shall not be written" Ubiquitous-negative. The EARS labels on these REQs are inconsistent with their actual form. **Severity: major**

- **D11 (minor, UNRESOLVED)** — spec.md:L131 (REQ-HK-008) vs L320–L322 (§6.2 Registry API) — REQ-HK-008 mandates calling "the externally-registered `SkillsFileChangedConsumer`", but the Registry API still exposes only `Register`, `ClearThenRegister`, `Handlers`. No `SetSkillsFileChangedConsumer(fn)` or equivalent API is defined. AC-HK-006 (L199–L202) says "stub `SkillsFileChangedConsumer`가 등록됨" without specifying via what method. Backchannel registration path is undefined. **Severity: minor**

- **D12 (minor, UNRESOLVED)** — spec.md:L133 (REQ-HK-009) — `permCtx.Role` still not enumerated. §6.2 type section (L242–L359) defines `PermissionBehavior`, `PermissionResult`, `DecisionReason` but no `Role` enum or `permCtx` struct. Decision flow at L133 mentions `coordinator/swarmWorker/interactive/non-TTY` branches but role value set is unspecified. **Severity: minor**

- **D13 (minor, UNRESOLVED)** — §3.1 L83 lists `DispatchPermissionRequest` as a dispatcher but §5 has no AC targeting its default/abstain path independent of AC-HK-008. **Severity: minor**

### New defects introduced in v0.2.0

- **D14 (minor, NEW)** — spec.md:L147–L165 — REQ ordering is non-monotonic. §4.4 Unwanted contains 014–018, then **021–022** (added in v0.2.0 patch). §4.5 Optional then contains 019–020. In document order: 018 → 021 → 022 → 019 → 020. Option A: renumber 019→021, 020→022, 021→019, 022→020 so numerical order matches document order. Option B: relocate REQ-HK-021/022 to a new §4.6 appendix. Currently satisfies MP-1 (no gaps/duplicates) but damages readability. **Severity: minor**

- **D15 (minor, NEW)** — spec.md:L157 (REQ-HK-021) — Ambiguity in "session workspace root (resolved from `HookInput.SessionID`)". The resolution function is not specified in this SPEC and `HookInput` (L276–L286) has no `WorkspaceRoot` field. Is `SessionID → workspace path` a responsibility of CORE-001? SESSION-001? Undefined. Implementation cannot proceed without this contract. **Severity: minor**

- **D16 (minor, NEW)** — spec.md:L159 (REQ-HK-022) — "4 MiB (4 × 1024 × 1024 bytes)" boundary case ambiguity. Is the condition `>= 4 MiB` or `> 4 MiB`? §6.5 L415 says `len(inputJSON) > 4*1024*1024`, so exactly 4 MiB would pass. REQ text says "exceeds 4 MiB" which matches strict `>`, but this should be explicit in the REQ itself. **Severity: minor**

- **D17 (minor, NEW)** — spec.md:L157 (REQ-HK-021 c) — `RLIMIT_CPU=cfg.Timeout + 5s` creates a hardcoded relationship between wall-clock timeout and CPU-time rlimit. If `cfg.Timeout` is zero or negative (config error), `RLIMIT_CPU` becomes meaningless or negative. No REQ specifies validation of `cfg.Timeout`. Edge-case behavior undefined. **Severity: minor**

---

## Chain-of-Verification Pass

Second-pass re-read of the v0.2.0 diff surface against iter-1 defects:

- **REQ count** — re-counted: L113–L165 contains exactly 22 REQs (001–022 each appearing once). Confirmed. D14 ordering issue identified in second pass.
- **AC count** — re-counted: L173–L222 contains exactly 10 ACs (001–010). Format declaration L171 does not count as an AC. No new ACs added for new REQs HK-021/022 → D4 regression confirmed.
- **Frontmatter** — re-scanned L1–L14: six required fields present. `created` field from iter-1 was renamed to `created_at` (L5). `labels` array present (L13). MP-3 clean.
- **§6.2 enum** — re-counted L248–L273: exactly 24 `Ev*` constants. AC-HK-001 L176 enumerated 24 names match the enum set-equal. D1 resolved.
- **New REQ-HK-021/022** — re-read L157–L159: new text carries significant implementation coupling (D5 worsened) and uses inconsistent EARS labeling ("Unwanted" but 022 is written Ubiquitous-negative — D7 unresolved). Both REQs lack ACs — D4 regressed.
- **§6.5 stdin write** — re-read L410–L428: now specifies goroutine write (L421) with context cancellation. D9 resolved.
- **§3.1 L84 DispatchPermissionDenied** — confirmed added. §6.2 L328 declaration confirmed. REQ-HK-009 L133 c/d clauses confirmed. D10 resolved.
- **Cross-contradiction hunt** — checked HISTORY L23 claims vs. defect numbering: HISTORY says "D4 (major) §5 AC 섹션 'Given/When/Then format' 명시적 선언" — but iter-1 D4 was "10 REQs uncovered", and the format declaration actually targets iter-1 D3 (MP-2 EARS). **HISTORY defect-number mapping is incorrect** — added to D18 below.

- **D18 (minor, NEW)** — spec.md:L23 — HISTORY entry for v0.2.0 misattributes defect numbers. It says "D4 (major) §5 AC 섹션 'Given/When/Then format' 명시적 선언" but D4 in iter-1 was the 10 uncovered REQs traceability issue, not the Gherkin/EARS format question (which was iter-1 D3). A reader reconciling the change log against the iter-1 report will be misled. **Severity: minor**

Second-pass added 5 new defects (D14, D15, D16, D17, D18). First-pass categorization of regression was thorough; second pass focused on new-surface defects and HISTORY accuracy.

---

## Regression Check (Iteration 2)

Mapping each iter-1 defect to iter-2 status:

| Iter-1 Defect | Severity | Status | Evidence |
|---------------|----------|--------|----------|
| D1 (AC-HK-001 24/27 mismatch) | critical | **RESOLVED** | L176–L177: 24 names enumerated, set-equal to §6.2 enum, Elicitation/ElicitationResult/InstructionsLoaded explicitly excluded. |
| D2 (frontmatter missing created_at/labels) | critical | **RESOLVED** | L5 `created_at`, L13 `labels` both present with correct types. |
| D3 (all ACs Gherkin, MP-2 FAIL) | critical | **UNRESOLVED** | L171 format declaration rationalizes non-compliance; ACs unchanged. MP-2 firewall not satisfied. |
| D4 (10/20 REQs uncovered) | major | **REGRESSED** | 12/22 uncovered (~55%). Two new REQs added without ACs. |
| D5 (implementation details in REQs) | major | **WORSENED** | All iter-1 leaks present; REQ-HK-021 adds syscall/rlimit/env-var specifics; REQ-HK-022 hardcodes 4 MiB and ErrHookPayloadTooLarge name. |
| D6 (exit code 2 compatibility) | major | **RESOLVED** | L125 REQ-HK-006(e) treats exit 2 as blocking; L127 Rationale block cites research.md. |
| D7 (Unwanted mislabeling) | major | **PARTIALLY RESOLVED** | REQ-HK-021 uses proper "If ... then shall" form. REQ-HK-014~018 and REQ-HK-022 still Ubiquitous-negative "shall not" form. 6/7 Unwanted REQs remain mislabeled. |
| D8 (subprocess isolation) | major | **RESOLVED** | REQ-HK-021 L157 adds four-part isolation contract. |
| D9 (payload cap + goroutine) | minor | **RESOLVED** | REQ-HK-022 L159 + §6.5 L421 goroutine write. |
| D10 (DispatchPermissionDenied) | critical* | **RESOLVED** | §3.1 L84, REQ-HK-009 c/d, §6.2 L328 formalize dispatcher. (*iter-1 marked minor; HISTORY L23 elevates to critical — inconsistent classification but the fix itself is complete.) |
| D11 (SkillsFileChangedConsumer registration API) | minor | **UNRESOLVED** | §6.2 Registry API unchanged; no Set/Register method for external consumer. |
| D12 (permCtx.Role enum) | minor | **UNRESOLVED** | Role enum still undefined. |
| D13 (no AC for DispatchPermissionRequest) | minor | **UNRESOLVED** | No targeted AC added. |

Summary: 6 resolved, 1 partial, 4 unresolved, 2 regressed.

**Stagnation signal**: D3 (MP-2 EARS firewall) is unresolved via explicit rationalization, not via implementation. If iter-3 arrives with the same rationalization unchanged, this becomes a blocking defect per orchestrator policy ("manager-spec made no progress"). Remediation must produce actual EARS-form ACs, not additional disclaimers.

---

## Recommendation

This SPEC still fails one must-pass criterion (MP-2) and has regressed on Traceability (D4) and Clarity (D5). It must be revised before Plan phase signoff.

Required fixes for iteration 3 (priority order):

1. **[BLOCKING] Remove the Format Declaration at L171 and rewrite all 10 ACs in EARS form** [D3, MP-2]. Example for AC-HK-002: "When `DispatchPreToolUse(ctx, input)` is invoked with two registered handlers where the first returns `HookJSONOutput{Continue: false, PermissionDecision: {Approve: false, Reason: \"unsafe\"}}`, the dispatcher shall return `PreToolUseResult{Blocked: true, PermissionDecision: ...}` and shall not invoke the second handler." If the author insists on preserving Given/When/Then scenarios for test-fixture binding, add them as a secondary test-scenario block under each EARS AC, not as replacement.

2. **Add ACs for REQ-HK-021 and REQ-HK-022** [D4 regression]. New AC drafts:
   - AC-HK-011 (REQ-HK-021 env scrub): When `DispatchPreToolUse` executes an `InlineCommandHandler` and `ANTHROPIC_API_KEY=xyz` is set in the parent environment, the child subprocess's environment shall not contain `ANTHROPIC_API_KEY`.
   - AC-HK-012 (REQ-HK-021 CWD): When `DispatchPreToolUse` invokes an `InlineCommandHandler` with `HookInput.SessionID = "S1"` resolving to workspace root `/ws/S1`, the child subprocess shall run with `cmd.Dir == /ws/S1` regardless of parent CWD.
   - AC-HK-013 (REQ-HK-022): When `len(json.Marshal(HookInput)) > 4*1024*1024` the dispatcher shall return `ErrHookPayloadTooLarge` before `cmd.Start()` and shall log a WARN entry.
   Also add ACs for currently-uncovered REQs HK-002, 004, 010, 013, 014, 015, 016, 018, 019, 020, OR explicitly state per-REQ that verification is delegated to unit tests (with justification).

3. **Move implementation names out of REQ text into §6** [D5]:
   - REQ-HK-004 L119: replace "structured zap log entry" → "a structured log entry"; move zap-specific format to §6.
   - REQ-HK-013 L143: replace `ErrRegistryLocked` → "a registry-locked error"; define the error type name in §6.2.
   - REQ-HK-015 L149: replace `ErrInvalidHookInput` → "an invalid-input error"; name in §6.2.
   - REQ-HK-019 L163: replace `GOOSE_HOOK_TRACE=1` → "a trace-enable environment variable"; name the variable in §6.
   - REQ-HK-021 L157: replace regex/env-var list and rlimit constants with behavior clauses; move the concrete deny-list and rlimit values to a §6.11 "Isolation Defaults" subsection.
   - REQ-HK-022 L159: replace `ErrHookPayloadTooLarge` with "a payload-too-large error"; name in §6.

4. **Fix Unwanted EARS labeling** [D7 partial]: rewrite REQ-HK-014, 015, 016, 017, 018, and 022 in proper `If [undesired condition], then the [system] shall [response]` form. Example REQ-HK-014: "If a hook handler attempts to mutate the input object, the dispatcher shall deliver downstream handlers a fresh deep copy of `HookInput` so the mutation is not observable."

5. **Define `SkillsFileChangedConsumer` registration API** in §6.2 Registry type [D11]: add `SetSkillsFileChangedConsumer(fn SkillsFileChangedConsumer)` method signature and lifecycle rules (replace-only, nil to unregister, thread-safety).

6. **Enumerate `permCtx.Role` values** in §6.2 [D12]: either define an enum `Role = coordinator | swarmWorker | interactive | non_tty` here, or explicitly state that SUBAGENT-001 defines this enum and this SPEC accepts any non-empty string with fallback to `interactive` on unknown.

7. **Renumber or relocate REQ-HK-021/022** [D14]: preferred Option A — renumber so §4.5 Optional becomes REQ-HK-019/020 and §4.4 Unwanted extension becomes REQ-HK-021/022 in document-numerical order (current Optional REQs shift down). Update all §6.5/§6.10/§8 cross-references accordingly.

8. **Resolve ambiguities** [D15, D16, D17]:
   - REQ-HK-021(b): specify that `workspaceRoot(sessionID string) string` is provided by CORE-001 or define a default resolution here.
   - REQ-HK-022: change "exceeds" to explicit `> 4 MiB` or `>= 4 MiB`, consistent with §6.5 L415.
   - REQ-HK-021(c): require `cfg.Timeout > 0`; document behavior when `cfg.Timeout <= 0` (reject config or apply default).

9. **Correct HISTORY defect-number attribution** [D18]: the v0.2.0 entry at L23 should list which iter-1 defect each change addresses; current mapping of "D4 (major) §5 AC 'Given/When/Then format'" is incorrect — iter-1 D4 was traceability, iter-1 D3 was the EARS question. Separately, D10 is marked critical in the HISTORY line but was minor in iter-1.

---

**File**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/HOOK-001-review-2.md`
**Iteration**: 2/3
**Verdict**: FAIL
**Defects**: 13 (1 critical, 4 major, 8 minor) — 1 regression, 2 worsened, 5 new
**Stagnation alert**: D3 unresolved via rationalization — iter-3 must produce real EARS ACs or this becomes a blocking defect.

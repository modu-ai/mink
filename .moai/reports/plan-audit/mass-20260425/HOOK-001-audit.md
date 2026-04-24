# SPEC Review Report: SPEC-GOOSE-HOOK-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.62

> Reasoning context ignored per M1 Context Isolation. Only `spec.md` and `research.md` were read. System reminders about MoAI workflow/memory rules were noted but did not influence audit verdicts.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency** — REQ-HK-001 ~ REQ-HK-020 (spec.md:L110-L156). 20 sequential requirements, no gaps, no duplicates, consistent 3-digit zero padding. Grouping by EARS subtype: Ubiquitous (001–004), Event-Driven (005–010), State-Driven (011–013), Unwanted (014–018), Optional (019–020).

- [FAIL] **MP-2 EARS format compliance** — REQ blocks are EARS-compliant (spec.md:L110–L156 consistently use "shall", "When...shall...", "While...shall...", "Where...shall..."), but **all 10 acceptance criteria (AC-HK-001 through AC-HK-010, spec.md:L162–L210) are written in Given/When/Then test-scenario format**, not EARS patterns. Per M3 rubric "Score 0.50 — ... Given/When/Then test scenarios mislabeled as EARS" and M5 definition "Every acceptance criterion must match one of the five EARS patterns", this is a hard failure. Example: AC-HK-002 uses "Given 2개 핸들러 등록 ... When DispatchPreToolUse ... Then PreToolUseResult" — this is a Gherkin test scenario, not Ubiquitous/Event-driven/State-driven/Optional/Unwanted EARS.

- [FAIL] **MP-3 YAML frontmatter validity** — spec.md:L2-L13 YAML block is missing two M5-required fields:
  - Required `created_at` is absent; only `created: 2026-04-21` is present (spec.md:L5). Field-name mismatch — MP-3 requires exact `created_at`.
  - Required `labels` is absent entirely from the frontmatter.
  Extra fields present (`updated`, `author`, `issue_number`, `phase`, `size`, `lifecycle`) do not compensate for missing required fields per MP-3 rules.

- [N/A] **MP-4 Section 22 language neutrality** — N/A: this SPEC is explicitly scoped to a single-language Go package (`internal/hook/`, spec.md:L29, L216-L228, §6.8 stdlib+zap dependency table). No 16-language enumeration expected; no hardcoded per-language LSP tool naming. Auto-passes.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.50–0.75 band | Most REQs unambiguous. Implementation-detail leaks reduce score: spec.md:L116 (zap by name), L122 (cfg.Timeout field name, 30s magic), L124 (zap warn), L126 (SkillsFileChangedConsumer interface name baked into requirement), L130 (`spawn a goroutine`), L138 (`ErrRegistryLocked` error type), L144 (`ErrInvalidHookInput`), L154 (`GOOSE_HOOK_TRACE=1` specific env name). A SPEC should describe behavior/contract; these leak HOW. |
| Completeness | 0.75 | 0.75 band | HISTORY (L19-L21), Overview (§1), Background (§2), Scope (§3 IN/OUT), REQUIREMENTS (§4), ACCEPTANCE CRITERIA (§5), Technical Approach (§6), Dependencies (§7), Risks (§8), References (§9), Exclusions (L540-L551) all present. Frontmatter missing required fields (see MP-3) is the sole deduction. |
| Testability | 0.70 | 0.50–0.75 band | ACs are concrete Given/When/Then scenarios — each is binary-testable against the described fixture (AC-HK-002/003/004/005/007/009 especially clear). However 10 of 20 REQs have no corresponding AC (see Traceability below), so half the normative surface lacks a measurable acceptance gate. AC-HK-006 uses "stub consumer"; AC-HK-009 depends on "stdout이 파이프" which is measurable. No weasel words detected. |
| Traceability | 0.50 | 0.50 band | REQ→AC coverage is ~50%. Covered: REQ-HK-001 (AC-HK-001), REQ-HK-003 (AC-HK-010), REQ-HK-005 (AC-HK-002), REQ-HK-006 (AC-HK-003/004), REQ-HK-007 (AC-HK-005), REQ-HK-008 (AC-HK-006), REQ-HK-009 (AC-HK-007/008/009), REQ-HK-011 (partial via AC-HK-002), REQ-HK-012 (AC-HK-007), REQ-HK-017 (AC-HK-009). **Uncovered**: REQ-HK-002 (FIFO ordering), REQ-HK-004 (structured logging contract), REQ-HK-010 (async handler goroutine + deadline), REQ-HK-013 (PluginHookLoader.IsLoading lock), REQ-HK-014 (deep-copy input isolation), REQ-HK-015 (schema validation → ErrInvalidHookInput), REQ-HK-016 (no-sudo privilege boundary), REQ-HK-018 (ClearThenRegister no-fire), REQ-HK-019 (GOOSE_HOOK_TRACE), REQ-HK-020 (regex matcher). research.md §7.1 lists unit tests for most of these, but research.md tests are not a substitute for spec-level ACs. |

---

## Defects Found

- **D1 (critical)** — spec.md:L165 — **AC-HK-001 is internally self-contradictory**. The test claims exactly 24 event strings must be returned, but the parenthetical enumeration lists **27 names** including three (`Elicitation`, `ElicitationResult`, `InstructionsLoaded`) that the very same SPEC declares OUT OF SCOPE at §3.2 (spec.md:L99) and §8 R5 (L509) and Exclusions (L547) and research.md §2.1 (L62, L80). The parenthetical also says "28개에서 본 SPEC은 24 핵심만" — the "28" count is wrong (27 listed) and the in-parenthesis list contains scope-excluded events. A test written literally against this AC would either require excluded events (contradicting §3.2) or fail to compile against the enum in §6.2 (which correctly has 24 and does NOT contain Elicitation/ElicitationResult/InstructionsLoaded). — **Severity: critical**

- **D2 (critical)** — spec.md:L2-L13 — **YAML frontmatter is missing M5-required fields**: `created_at` (only `created` exists at L5 — field-name mismatch) and `labels` (absent entirely). This is an MP-3 hard failure. — **Severity: critical**

- **D3 (critical)** — spec.md:L162-L210 — **All 10 acceptance criteria use Given/When/Then (Gherkin) structure rather than one of the five EARS patterns**. Per M5 MP-2, every AC must match an EARS pattern. Remediation options: (a) rewrite ACs as EARS statements, or (b) add a parallel set of EARS-formatted criteria alongside the Given/When/Then scenarios. The REQ blocks themselves are EARS-compliant; only the AC section fails MP-2. — **Severity: critical**

- **D4 (major)** — spec.md:L110-L156 vs L162-L210 — **50% of REQs have no corresponding AC**: REQ-HK-002, 004, 010, 013, 014, 015, 016, 018, 019, 020 are uncovered. This is the largest traceability gap: critical safety REQs such as REQ-HK-014 (deep-copy isolation), REQ-HK-015 (invalid input rejection), REQ-HK-016 (no-sudo), and REQ-HK-018 (no-fire-during-swap) are not guarded by any AC in this SPEC. — **Severity: major**

- **D5 (major)** — spec.md:L116, L122, L124, L126, L130, L138, L144, L154 — **Implementation details embedded in normative REQ text**. Specific library (`zap`), specific field names (`cfg.Timeout`), specific error identifiers (`ErrRegistryLocked`, `ErrInvalidHookInput`), specific env var (`GOOSE_HOOK_TRACE`), specific interface names (`SkillsFileChangedConsumer`), and a specific Go primitive (`spawn a goroutine`) belong in §6 Technical Approach, not in §4 Requirements. A requirement should specify the observable contract; the implementation names should be changeable without amending a REQ. — **Severity: major**

- **D6 (major)** — **Claude Code hook exit code 2 convention is not addressed**. Standard Claude Code hooks use exit code 0 (pass-through), 2 (block with stderr shown to user), and non-zero (error). This SPEC instead uses stdout JSON `{"continue": false}` as the sole blocking channel (spec.md:L122, §6.5 L408-L418). Legitimate design choice, but:
  - REQ-HK-006 (L122) says "non-zero exit code ... handler_error" — meaning every non-zero exit, including what Claude Code treats as "block", becomes a handler_error. This means **existing MoAI-ADK Claude Code hooks in `.claude/hooks/moai/*.py` that return exit 2 will be mis-classified as errors** when run via `InlineCommandHandler`, even though research.md L239 calls these "소비 대상 (consumption target)".
  - No mapping or compatibility shim is specified. This is a latent integration defect. — **Severity: major**

- **D7 (major)** — spec.md:L140-L150 (REQ-HK-014 ~ REQ-HK-018) — **Subtype mislabeling**. All five "Unwanted" REQs are written as Ubiquitous "shall not" statements, not as the EARS Unwanted pattern ("If [undesired condition], then the [system] shall [response]"). Example: REQ-HK-014 should be "If a hook handler attempts to mutate the input object, the dispatcher shall deliver downstream handlers a fresh deep copy." The current form mixes negation and Ubiquitous structure. Does not violate MP-1/MP-3/MP-4 but weakens EARS labeling integrity for REQs too. — **Severity: major**

- **D8 (major)** — §3.1 (L88-L89), §6.5 (L391-L419), REQ-HK-006 (L122), REQ-HK-016 (L146) — **Hook script execution isolation is under-specified**. Coverage:
  - Timeout: covered (30s default, REQ-HK-006, AC-HK-004).
  - stdout/stderr: covered (§6.5 captures stderr buffer).
  - Sandbox / process isolation: **NOT specified**. No cgroups, no seccomp, no chroot, no resource limits (RLIMIT_*), no FD-close-on-exec guidance. A hook subprocess inherits the parent process's full environment, CWD, open file descriptors, network access, and user identity.
  - Environment scrubbing: Not specified. `HookInput` JSON is piped to stdin, but environment variables are inherited verbatim — including potentially sensitive variables (API keys, tokens).
  - Working directory: Not specified. Subprocess inherits parent CWD.
  - The only explicit isolation rule is REQ-HK-016 "no sudo / no capability bits" — necessary but far from sufficient.
  Research §8 open issue 4 acknowledges "WASM 기반 샌드박스 고려 (별도 SPEC)" as future work, but the current SPEC does not even specify minimum env-var/CWD/rlimit hygiene. — **Severity: major**

- **D9 (minor)** — spec.md:L396-L419 (§6.5) — **HookInput JSON piped to stdin is not size-bounded or sanitized in the REQ layer**. Risk R2 (L506) mentions "payload cap 4MB" and "ErrHookPayloadTooLarge" but this is only in the risk table — no corresponding REQ enforces it. A malformed or oversized `Output` field in `HookInput` (e.g., for PostToolUse with a multi-MB tool result) could cause pipe deadlock if the subprocess doesn't read fast enough, despite the research's claim of "별도 goroutine 쓰기" (§6.5 shows a synchronous `stdin.Write(inputJSON)` without size check — inconsistent with research.md §4.2 L180 which says "별도 goroutine에서 JSON 쓰기"). Implementation inconsistency between research and spec §6.5. — **Severity: minor**

- **D10 (minor)** — spec.md:L128 — REQ-HK-009 describes the useCanUseTool decision flow but omits the explicit PermissionRequest→PermissionDenied event emission sequence. When a handler returns `behavior: deny`, does that also dispatch the `PermissionDenied` event (enum member on L249)? The SPEC lists `PermissionDenied` as an enum constant but never specifies who dispatches it or when. No dispatcher function (`DispatchPermissionDenied`) is mentioned in §3.1 (L81 lists only PermissionRequest). Dangling event. — **Severity: minor**

- **D11 (minor)** — spec.md:L126 — REQ-HK-008 says "after the internal handlers complete, call the externally-registered SkillsFileChangedConsumer". This introduces **a second registration channel** (not the standard `HookRegistry.Register`) specifically for SKILLS-001's consumer. The contract for how `SkillsFileChangedConsumer` is registered, deregistered, or replaced is not specified in §6.2 types nor in the Registry API (L308-L310). Backchannel registration path is undefined. — **Severity: minor**

- **D12 (minor)** — spec.md:L128 (REQ-HK-009) — `permCtx.Role` values are not enumerated. The REQ mentions routing by `coordinator / swarmWorker / interactive / non-TTY` but §6.2 type section does not define the `Role` enum or `permCtx` struct. A later SPEC (SUBAGENT-001) presumably populates `coordinator` role, but the contract for role values is unspecified here. — **Severity: minor**

- **D13 (minor)** — §3.1 L81 — `DispatchPermissionRequest` is listed, but §5 ACs never test it directly; coverage appears only indirectly through AC-HK-008. No AC guards the dispatcher's behavior when a hook-less call is made (the default/abstain path beyond REQ-HK-017). — **Severity: minor**

---

## Chain-of-Verification Pass

Second-pass re-read of the following sections to catch first-pass oversights:

- **REQ enumeration (L110–L156)** — re-counted: exactly 20 REQs, sequential. First-pass PASS confirmed.
- **§6.2 Go enum (L237–L260)** — re-counted: exactly 24 `Ev*` constants. Confirmed 24 ≠ the 27 listed in AC-HK-001 → D1 confirmed critical.
- **AC enumeration (L162–L210)** — re-counted: exactly 10 ACs (001–010), all Given/When/Then. D3 confirmed.
- **Exclusions (L540–L551)** — 10 clearly scoped exclusions, all specific. Not a defect.
- **Cross-contradiction hunt** — Checked for internal contradictions:
  - L99 (OUT) vs L165 (AC-HK-001 list including Elicitation/etc.) → **contradiction confirmed (D1)**.
  - L122 (non-zero exit = handler_error) vs the existing MoAI-ADK hook ecosystem's use of exit 2 for block → **contradiction identified → D6 added in second pass**.
  - Research.md §4.2 L180 (goroutine write to stdin) vs spec §6.5 L407 (synchronous write) → **contradiction identified → D9 added in second pass**.
  - L126 `SkillsFileChangedConsumer` external registration vs §6.2 Registry API (no corresponding method) → **gap identified → D11 added in second pass**.
- **Event dispatch completeness** — re-scanned enum: `EvPermissionDenied` (L249) is declared but has no Dispatch function in §3.1 listing (L81) — **D10 identified in second pass**.
- **Risk table R2/R3/R4/R7** — R2's 4MB cap is not promoted to a REQ → D9 reinforced.

Second-pass added 4 defects (D6, D9, D10, D11). First pass was not thorough enough; the extended re-read was necessary.

---

## Recommendation

This SPEC fails two must-pass criteria (MP-2, MP-3) and has a critical internal contradiction (D1). It must be revised before manager-spec signs off on Plan phase.

Required fixes for iteration 2:

1. **Fix frontmatter** (spec.md:L2–L13): add `created_at: 2026-04-21` (keep or rename `created`), add `labels: [hook, dispatcher, permission, phase-2]` (or project convention). [D2, MP-3]

2. **Rewrite all 10 ACs in EARS format** (spec.md:L162–L210). Example rewrite for AC-HK-002: "When `DispatchPreToolUse` is invoked with two registered handlers where the first returns `{Continue: false, PermissionDecision: {Approve: false, Reason: \"unsafe\"}}`, the dispatcher shall return `PreToolUseResult{Blocked: true, PermissionDecision: ...}` and shall not invoke the second handler." Alternatively keep Given/When/Then as a secondary "test scenario" block but add EARS-form assertion lines. [D3, MP-2]

3. **Fix AC-HK-001** (spec.md:L162–L165): remove `Elicitation`, `ElicitationResult`, `InstructionsLoaded` from the enumerated list; correct "28개" → "24개" (since those three are explicitly out of scope); verify the enumerated 24 names match §6.2 enum exactly. [D1]

4. **Add 10 missing ACs** for REQ-HK-002, 004, 010, 013, 014, 015, 016, 018, 019, 020, OR explicitly mark them as "verified by unit tests in research.md §7.1 only, no integration AC required" with justification per REQ. [D4]

5. **Move implementation details out of REQs** (L116, L122, L124, L126, L130, L138, L144, L154): replace library/type/env-var names with behavior descriptions. Example: L116 "zap log entry" → "a structured log entry at INFO/ERROR level". [D5]

6. **Add exit code compatibility contract** (spec.md near REQ-HK-006): specify whether exit code 2 is treated as (a) blocking (Claude Code convention), (b) handler_error (current spec), or (c) configurable. Given the SPEC claims to consume existing `.claude/hooks/moai/*.py` scripts (research.md L236), option (a) or (c) is recommended. [D6]

7. **Relabel REQ-HK-014 ~ 018 as Ubiquitous** or rewrite in proper EARS Unwanted form (`If ..., then the [system] shall ...`). Current "shall not" form is Ubiquitous-negative, not Unwanted. [D7]

8. **Add hook subprocess isolation REQ** (new REQ-HK-021 or expand REQ-HK-016): specify minimum guarantees for env var scrubbing (allowlist sensitive env filter), CWD (pinned to workspace root), rlimits (RLIMIT_AS, RLIMIT_NOFILE, RLIMIT_CPU), and FD hygiene (close-on-exec for parent's FDs). [D8]

9. **Promote R2 4MB payload cap to a REQ** and resolve the sync-vs-goroutine stdin write inconsistency between research.md §4.2 and spec.md §6.5. [D9]

10. **Define `DispatchPermissionDenied`** or explicitly state that `EvPermissionDenied` is a downstream-only event emitted as a side effect of `DispatchPermissionRequest` when decision = Deny. [D10]

11. **Define `SkillsFileChangedConsumer` registration API** in §6.2 Registry type (e.g., `SetSkillsFileChangedConsumer(fn SkillsFCConsumer)`). [D11]

12. **Enumerate `permCtx.Role` values** in §6.2 or state that SUBAGENT-001 defines the role value set and this SPEC accepts any non-empty string. [D12]

---

**File**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/HOOK-001-audit.md`
**Iteration**: 1/3
**Verdict**: FAIL
**Defects**: 13 (3 critical, 6 major, 4 minor)

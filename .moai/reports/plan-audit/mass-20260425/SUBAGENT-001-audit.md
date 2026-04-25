# SPEC Review Report: SPEC-GOOSE-SUBAGENT-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.74

Reasoning context ignored per M1 Context Isolation. Audit is based solely on `spec.md` and `research.md`.

---

## Must-Pass Results

### MP-1 REQ Number Consistency — **PASS**

REQ IDs use prefix `REQ-SA-` and run sequentially from 001 to 020 with consistent 3-digit zero-padding:
- `REQ-SA-001` through `REQ-SA-004` (Ubiquitous, spec.md:L109-115)
- `REQ-SA-005` through `REQ-SA-010` (Event-Driven, spec.md:L119-129)
- `REQ-SA-011` through `REQ-SA-013` (State-Driven, spec.md:L133-137)
- `REQ-SA-014` through `REQ-SA-018` (Unwanted, spec.md:L141-149)
- `REQ-SA-019` through `REQ-SA-020` (Optional, spec.md:L153-155)

Verified by sequential read: no gaps, no duplicates, consistent padding.

### MP-2 EARS Format Compliance — **PASS (with reservation)**

All 20 REQs are explicitly tagged with EARS pattern in brackets and follow the structural pattern:
- Ubiquitous: "Every X shall Y" / "The Y shall Z" (e.g. REQ-SA-001 spec.md:L109: "Every spawned `Subagent` **shall** have a unique `AgentID`...")
- Event-Driven: "When X, the spawner shall Y" (e.g. REQ-SA-005 spec.md:L119: "**When** `RunAgent(ctx, def, input)` is invoked...")
- State-Driven: "While X, spawned sub-agents shall Y" (e.g. REQ-SA-011 spec.md:L133: "**While** a parent `QueryEngine` is in `CoordinatorMode == true`...")
- Unwanted: "X shall not Y" (e.g. REQ-SA-014 spec.md:L141: "The spawner **shall not** allow cyclic agent spawning...")
- Optional: "Where X, Y shall Z" (e.g. REQ-SA-019 spec.md:L153: "**Where** `def.Model == "inherit"`...")

Reservation: REQ-SA-014 is technically labeled [Unwanted] (spec.md:L141) but its trailing clause uses "shall return `ErrSpawnDepthExceeded`" — this is a normative consequence rather than an "If undesired... then" pattern. Acceptable because the main structural rule ("shall not allow cyclic agent spawning") is the Unwanted core; the second clause is remediation. Still counts as PASS.

### MP-3 YAML Frontmatter Validity — **FAIL**

spec.md:L1-13 frontmatter:
```
id: SPEC-GOOSE-SUBAGENT-001    ✔ present
version: 0.1.0                   ✔ present
status: Planned                  ✔ present
created: 2026-04-21             ✘ field name mismatch — rubric requires `created_at`
updated: 2026-04-21             (not required)
author: manager-spec             (not required)
priority: P0                     ✔ present (non-standard value; rubric lists critical/high/medium/low — P0 is domain-specific but acceptable convention)
issue_number: null               (not required)
phase: 2                         (not required)
size: 대(L)                     (not required)
lifecycle: spec-anchored         (not required)
```

Missing required field: `labels` (array or string) — **absent entirely** from frontmatter.
Field name mismatch: `created` instead of the canonical `created_at` (ISO date string present, but the key name differs from the rubric's required schema).

Per M5 MP-3: "Any missing required field = FAIL." Defect confirmed: `labels` missing.

Severity: major (blocks traceability/filtering tooling that depends on label index).

### MP-4 Language Neutrality — **N/A**

This SPEC targets `internal/subagent/` Go package and is explicitly single-language scoped (Go 1.22+, spec.md:L486). It is not template-bound content spanning multiple language servers. Auto-pass per M3.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band | Most REQs unambiguous; a few interpretive issues — see D5, D6 |
| Completeness | 0.70 | 0.50→0.75 band | All major sections present but frontmatter `labels` missing (MP-3); Exclusions section present and specific (spec.md:L534-546) |
| Testability | 0.85 | 0.75→1.0 band | 12 AC each bound to AC-SA-NNN with Given/When/Then structure; most binary-testable. Weasel concerns below. |
| Traceability | 0.75 | 0.75 band | AC → REQ mapping incomplete — see D1 |

---

## Defects Found

**D1. [spec.md:L161-219] Traceability gap: AC do not cite specific REQ-SA-XXX IDs — Severity: major**

Each AC block has a title and Given/When/Then steps but **none of the 12 AC explicitly cross-reference the REQ-SA-XXX they implement**. Inspection of AC-SA-001 (spec.md:L161-164) shows it tests REQ-SA-005 behavior implicitly (fork isolation spawn) but the text does not say "Verifies: REQ-SA-005". Traceability must be bidirectional. Reverse-mapping exists only in research.md:L393-406 (Integration Test table), not in spec.md itself.

Impact: REQ-SA-004 (allowlist validation), REQ-SA-015 (worktree cleanup on crash), REQ-SA-018 (invalid agent name), REQ-SA-019 (model inherit), REQ-SA-020 (MCPServers init) — **have NO corresponding AC in spec.md**. Only research.md test list references them indirectly.

REQ coverage audit:
| REQ | AC in spec.md? |
|---|---|
| REQ-SA-001 | implied in AC-SA-001 (AgentID format) ✔ |
| REQ-SA-002 | AC-SA-005 ✔ |
| REQ-SA-003 | AC-SA-004 ✔ |
| REQ-SA-004 | **no AC** ✘ |
| REQ-SA-005 | AC-SA-001 ✔ |
| REQ-SA-006 | AC-SA-002 ✔ |
| REQ-SA-007 | AC-SA-003 ✔ |
| REQ-SA-008 | AC-SA-005 partial ✔ |
| REQ-SA-009 | AC-SA-006 ✔ |
| REQ-SA-010 | AC-SA-007 ✔ |
| REQ-SA-011 | AC-SA-008 ✔ |
| REQ-SA-012 | **no AC** ✘ (concurrency race behaviour untested in AC list) |
| REQ-SA-013 | AC-SA-010 ✔ |
| REQ-SA-014 | AC-SA-009 ✔ |
| REQ-SA-015 | **no AC** ✘ (SessionEnd orphan cleanup untested in AC list) |
| REQ-SA-016 | AC-SA-011 ✔ |
| REQ-SA-017 | AC-SA-012 ✔ |
| REQ-SA-018 | **no AC** ✘ |
| REQ-SA-019 | **no AC** ✘ |
| REQ-SA-020 | **no AC** ✘ |

6 of 20 REQs (30%) lack direct AC coverage in spec.md. This is a Traceability FAIL condition for REQ-coverage completeness.

**D2. [spec.md:L1-13] Frontmatter missing `labels` field — Severity: major (MP-3)**

As noted in MP-3 section above. The frontmatter omits `labels` entirely. Add `labels: [subagent, runtime, isolation, phase-2]` or similar.

**D3. [spec.md:L5] Frontmatter `created` should be `created_at` — Severity: minor**

Rubric requires field name `created_at`. Current spec uses `created`. Value format (ISO date `2026-04-21`) is correct; only the key name deviates.

**D4. [spec.md:L109] REQ-SA-001 has implicit but unstated uniqueness guarantee scope — Severity: minor**

"Collisions shall not occur within a single parent session's lifetime" — but AgentID format `{agentName}@{sessionId}-{spawnIndex}` does not define how `spawnIndex` is computed atomically in the presence of concurrent `RunAgent` calls from the same parent. No REQ specifies atomic counter semantics. Under parallel spawn (explicitly allowed by overview spec.md:L27: "순차·병렬로 spawn"), race on `spawnIndex` could produce duplicate IDs. Add a REQ or refine REQ-SA-001 to specify monotonic atomic allocation.

**D5. [spec.md:L123] REQ-SA-007 uses weasel-like threshold — Severity: minor**

"a 5-second inactivity period" — the 5s value is hardcoded in a normative REQ without configuration handle. Contradicts research.md:L428 which calls it "기본 5s" (default 5s) and suggests learning-based adaptation in Phase 4. REQ-SA-007 should reference a named configuration constant (e.g., `BackgroundIdleThreshold`) to make the test binary-evaluable against that constant rather than a magic number.

**D6. [spec.md:L133-134] REQ-SA-011 has ambiguous interaction with REQ-SA-011 override clause — Severity: minor**

"explicit override via `def.CoordinatorMode = true` is allowed but logs a WARN" — the WARN behaviour is normative but no AC tests that the WARN is emitted (AC-SA-008 spec.md:L197-199 does assert the WARN log, so this is actually covered). Re-verification: AC-SA-008 does cover this — demote to non-defect on second pass.

(Removed from final defect list after M6 verification.)

**D7. [spec.md:L135-137] REQ-SA-012 concurrency scope unclear — Severity: major**

"concurrent reads **shall** see either the prior state or the new state — never a partial line" — but the REQ is labeled [State-Driven] scoped to "While an agent's `memdir.jsonl` is being written". The REQ does not address cross-process / cross-agent concurrent writes when two sub-agents share the same scope directory (e.g., project scope shared by peers). Research.md:L499 acknowledges this gap ("여러 sub-agent가 동일 scope를 공유 시 file lock...추가 검토") but the REQ itself does not commit to file-lock semantics. This is a silent assumption; concurrent writes from peer sub-agents could corrupt memdir.jsonl beyond the POSIX O_APPEND atomic-below-PIPE_BUF guarantee (which only holds for single-process or well-behaved POSIX; NFS, macOS APFS behavior differs).

**D8. [spec.md:L145-147] REQ-SA-016 background write deny is sound BUT the "allow rule" source is undefined — Severity: minor**

"unless an explicit allow rule is present" — no definition of where allow rules live (settings.json? per-agent YAML? runtime injection?). AC-SA-011 (spec.md:L211-214) reuses the same term without specifying the rule source. Binary testability is compromised: a tester cannot construct a positive case without knowing how to inject an allow rule.

**D9. [spec.md:L143] REQ-SA-015 has event trigger without mapped hook implementation contract — Severity: minor**

"A `SessionEnd` hook handler (HOOK-001) **shall** invoke `git worktree prune` + `os.RemoveAll`" — but HOOK-001 is declared as a precondition dependency (spec.md:L480). This SPEC does not define its own SessionEnd handler; it assumes HOOK-001 provides the dispatcher. If HOOK-001 lacks a SessionEnd event (not verified from this SPEC alone), REQ-SA-015 is unimplementable. Consider making this a conditional ("Given HOOK-001 emits SessionEnd, the spawner shall register a handler that...") or adding a dependency verification AC.

**D10. [spec.md:L125] REQ-SA-008 ordering ambiguity — Severity: minor**

"the spawner **shall** (a) write the final transcript... (b) invoke `DispatchSubagentStop`... (c) close the output channel, (d) mark the `Subagent.State`..." — ordering is labeled a/b/c/d implying sequence, but if (c) close channel happens before (d) state mark, a consumer reading the channel close signal may observe `Subagent.State == Running` transiently. Either strengthen the REQ to require (d) before (c) or explicitly permit the race and document consumer contract.

**D11. [spec.md:L109] REQ-SA-001 AgentID format lacks sanitization requirement — Severity: minor**

`{agentName}@{sessionId}-{spawnIndex}` — if `agentName` contains `@` or `-` characters (REQ-SA-018 restricts to `[a-zA-Z0-9-_]` but allows `-`), parsing AgentID back to components is ambiguous when agentName contains `-`. Combined with sessionId also potentially containing delimiters, round-trip parsing is unreliable. Not immediately blocking but worth tightening.

**D12. [spec.md:L119] REQ-SA-005 does not specify failure path — Severity: minor**

REQ-SA-005 lists 5 steps (a-e) for the success path of fork spawn, returning "Subagent + output channel + nil error". No REQ specifies the contract when any of the 5 steps fails (e.g., QueryEngine construction error, hook dispatch failure). No AC tests failure modes. Ubiquitous/Unwanted REQs should cover error cases.

**D13. [spec.md:L86] Scope item #9 `ResumeAgent(agentId)` signature vs §6.2 signature mismatch — Severity: major**

- §3.1 IN SCOPE #9 (spec.md:L86): `ResumeAgent(agentId)` — one argument.
- §6.2 Go types (spec.md:L342-346): `func ResumeAgent(parentCtx context.Context, agentID string, opts ...RunOption) (*Subagent, <-chan message.SDKMessage, error)` — three arguments.
- REQ-SA-009 (spec.md:L127): describes semantics but does not commit to signature.
- AC-SA-006 (spec.md:L186-189) tests `ResumeAgent("researcher@sess-old-2")` — single string argument form.

Three-way inconsistency between scope summary, type section, and AC. The actual implementation signature is ambiguous.

**D14. [spec.md:L181] AC-SA-004 uses non-binary union claim — Severity: minor**

"다른 고유 키는 union" — "union" is unambiguous semantically, but the AC does not enumerate what "union" means for conflicting types (e.g., when scope A has `key: "str"` and scope B has `key: 42`). REQ-SA-003 resolves by "nearest wins" which handles key collisions but the AC phrasing "union" could be read as merge-of-values. Align AC language with REQ-SA-003 resolution rule.

**D15. [spec.md:L38] Overview asserts `buildMemoryPrompt()` allows model to "query·update" memory, but no REQ covers the update path — Severity: major**

Overview (spec.md:L38): "모델이 memory를 쿼리·업데이트 가능" — but no REQ-SA-NNN defines the memory-update tool or the contract by which the model triggers an append. REQ-SA-012 (spec.md:L135) covers atomic writes, but not who initiates writes. §6.2 `MemdirManager.Append` is exposed (spec.md:L356) but no REQ specifies that the tool set or system prompt enables the model to call it. Either remove the claim from overview or add a REQ defining the update mechanism.

**D16. [spec.md:L97-98] Exclusion "Agent 간 SendMessage / mailbox 메시징을 구현하지 않는다" contradicts task framing — Severity: informational (not a defect)**

This is a clear exclusion (spec.md:L96, L539) and aligns with the special-focus item "teammate mailbox / inbox 메시지 protocol" being explicitly out of scope. The SPEC properly defers this to TEAM-001 or CLI-001. Not a defect — note for the caller: mailbox protocol is intentionally absent and should not be graded as a gap here.

---

## Chain-of-Verification Pass

Second-look findings:

1. **Re-read all 20 REQs end-to-end** (spec.md:L109-155): REQ numbering verified sequential 001-020. No skipped numbers. Confirmed.

2. **Re-read all 12 ACs in detail** (spec.md:L161-219): AC numbering sequential AC-SA-001 through AC-SA-012. Given/When/Then structure consistent. Confirmed.

3. **Built REQ→AC coverage matrix** (see D1): 6 REQs lack direct AC coverage — REQ-SA-004, REQ-SA-012, REQ-SA-015, REQ-SA-018, REQ-SA-019, REQ-SA-020. This is a traceability gap that I did not immediately notice on first skim. D1 promoted to major severity.

4. **Checked Exclusions section** (spec.md:L534-546): 11 specific exclusion entries, each with a clear reason and often a cross-reference (QUERY-001, TEAM-001, etc.). Exclusions are specific, not vague. Completeness partial credit granted.

5. **Looked for contradictions across requirements**: Found D13 (ResumeAgent signature three-way mismatch) and D15 (overview claims update capability not backed by any REQ). Both promoted.

6. **Re-read research.md cross-reference**: Research document is thorough, covers TDD strategy, fallback policies, and dependencies. Research tests (research.md:L354-406) do reference missing-AC REQs (e.g., REQ-SA-004, REQ-SA-012, REQ-SA-017, REQ-SA-018), suggesting the test plan is more complete than the AC list. This is not a substitute for spec-level AC coverage: the SPEC is the contract; research.md is derivation.

7. **Special-focus audit** (from caller's prompt, evaluated independently per M1):
   - Subagent fork parent QueryEngine context isolation: REQ-SA-005 covers context propagation via `context.WithValue`. Adequate.
   - TeammateIdentity injection (QUERY-001 REQ-020): REQ-SA-005(b) injects `TeammateIdentity{AgentId, AgentName, TeamName, ParentSessionId}`. Adequate.
   - Coordinator mode tool visibility (QUERY-001 REQ-019): REQ-SA-011 covers `CoordinatorMode` inheritance; no REQ covers the **tool visibility switch** semantics explicitly in this SPEC. Scope item §3.1 #12 mentions "CoordinatorMode 플래그 반영" — flag propagation only. Consumer is responsible for behavior. Not a defect if consumer contract in QUERY-001 is sufficient.
   - Mailbox / inbox protocol: explicitly excluded (spec.md:L97-98, L539). Out of scope, not a defect.
   - `isolation: worktree` Claude Code v2.1.49+ support: REQ-SA-006 defines worktree spawn mechanics. Adequate.
   - Plan-mode approval wait: `PlanModeRequired` field exists in TeammateIdentity (spec.md:L285) but **no REQ specifies approval-wait semantics**. This is a gap — adding D17.
   - TeammateIdle / shutdown_request protocol: REQ-SA-007 covers `DispatchTeammateIdle` after 5s inactivity. `shutdown_request` message type is NOT mentioned in this SPEC — it is the Claude Code Agent Teams protocol. Overview (spec.md:L62) excludes "Team 전체 오케스트레이션". Shutdown_request is arguably part of mailbox protocol (excluded). Adequate.
   - Goroutine leak prevention on subagent termination: R2 in §8 (spec.md:L498) commits to `goleak` CI verification, and REQ-SA-008 covers transcript write + channel close + state marking on Terminal. However, **no REQ explicitly guarantees all spawned goroutines observe parent `ctx.Done()` and terminate** — this is asserted in research.md §4.5 but not codified as a REQ. Adding D18.

**D17. [spec.md:L281-286 `PlanModeRequired`] Plan-mode approval wait semantics undefined — Severity: major**

The Go type `TeammateIdentity` includes `PlanModeRequired bool` (spec.md:L285) but no REQ-SA-NNN defines:
- When `PlanModeRequired == true` is set
- What blocking/wait behavior the spawner implements
- How approval is solicited and received
- What the consumer contract looks like when the flag is true

This field exists in the type definition with no REQ backing. Either remove from the type, move to a dedicated REQ, or defer to a follow-up SPEC and remove the field here.

**D18. [Missing REQ] No explicit REQ requires all subagent goroutines to respect parent ctx cancellation — Severity: major**

Risk R2 (spec.md:L498) states goroutines auto-terminate on parent cancel, and research.md §4.5 commits to the discipline. However, this critical safety property is **not formalized as an Unwanted REQ** (e.g., "The spawner shall not leave goroutines running after parent ctx is cancelled"). Without a REQ, there is no AC, and no binary test requirement. Add a REQ-SA-NNN covering goroutine lifecycle discipline.

---

## Regression Check (Iteration 2+ only)

N/A — this is iteration 1.

---

## Recommendation (FAIL — 4 major + 11 minor defects)

**Actionable fixes for manager-spec, ordered by severity:**

1. **[MP-3 FAIL] Add `labels` field to frontmatter** (spec.md:L1-13). Example:
   ```yaml
   labels: [subagent, runtime, isolation, memory, phase-2]
   ```
   Also rename `created` → `created_at` for schema conformance.

2. **[D1 major] Add REQ traceability footer to each AC** in spec.md:L161-219. Each AC should explicitly cite which REQ-SA-XXX it verifies, e.g.:
   ```
   **AC-SA-001 — Fork isolation spawn** (Verifies: REQ-SA-001, REQ-SA-005)
   ```

3. **[D1 major] Add AC coverage for 6 uncovered REQs**:
   - REQ-SA-004 (allowlist validation): add AC testing `LoadAgentsDir` rejects unknown frontmatter keys.
   - REQ-SA-012 (memdir atomic writes): add AC with concurrent writer goroutines confirming no torn lines.
   - REQ-SA-015 (worktree orphan cleanup): add AC simulating crash + SessionEnd handler invocation.
   - REQ-SA-018 (invalid agent name): add AC testing `_foo`, `foo bar`, `foo/bar` names rejected.
   - REQ-SA-019 (model inherit/alias): add AC for both `"inherit"` and explicit alias paths.
   - REQ-SA-020 (MCPServers init): add AC testing MCP tool namespacing `mcp__{server}__{tool}`.

4. **[D13 major] Reconcile `ResumeAgent` signature** across §3.1, §6.2, and AC-SA-006. Recommended: standardize on `ResumeAgent(ctx, agentID, opts...)` from §6.2 and update AC-SA-006 to show ctx and the scope-list to match.

5. **[D15 major] Either remove the "update" claim from overview (spec.md:L38) or add REQ-SA-NNN defining the memory-update tool contract** (how the model invokes `MemdirManager.Append`, tool naming, arguments, expected effect on next turn).

6. **[D17 major] Remove `PlanModeRequired` field or add supporting REQ** covering plan-mode approval-wait semantics, approval request protocol, and timeout behavior.

7. **[D18 major] Add Unwanted REQ** for goroutine lifecycle:
   ```
   REQ-SA-0XX [Unwanted] — The spawner shall not leave background goroutines running after parent ctx.Done() is signaled; every spawned goroutine shall observe ctx cancellation within bounded time (default 100ms) and release resources before returning.
   ```

8. **[D7 major] Strengthen REQ-SA-012** to explicitly address cross-process/peer-subagent concurrent writes. Commit to file-lock semantics (`golang.org/x/sys/unix.Flock`) or restrict REQ scope to single-writer and add REQ forbidding peer writes to same scope dir.

9. **[D4, D11 minor] Tighten REQ-SA-001 AgentID**: specify atomic monotonic `spawnIndex` allocation and define the delimiter/escape rule for agentName containing `-`.

10. **[D5 minor] Replace magic "5-second"** in REQ-SA-007 with named constant `DefaultBackgroundIdleThreshold` with configuration path.

11. **[D8 minor] Define "allow rule" source** in REQ-SA-016 / AC-SA-011 — file path, schema, and injection mechanism.

12. **[D9 minor] Reframe REQ-SA-015** as conditional on HOOK-001's SessionEnd event, or add a dependency assumption REQ.

13. **[D10 minor] Specify ordering** in REQ-SA-008 for steps (c) close channel and (d) mark state — prefer (d) before (c).

14. **[D12 minor] Add error-path REQ** for REQ-SA-005 failure modes.

15. **[D14 minor] Align AC-SA-004 "union"** wording with REQ-SA-003 "nearest wins" resolution semantics.

After fixes, re-invoke plan-auditor for iteration 2.

---

## Summary Table

| Category | Count |
|----------|-------|
| Must-Pass FAIL | 1 (MP-3) |
| Major defects | 4 (D1, D13, D15, D17, D18 — plus D7 concurrency) — effectively 6 |
| Minor defects | ~10 |
| N/A | 1 (MP-4 — single-language scope) |

**Verdict: FAIL**. Iteration 1 identifies 1 must-pass failure (frontmatter) and several traceability/contract gaps that warrant revision before approval.

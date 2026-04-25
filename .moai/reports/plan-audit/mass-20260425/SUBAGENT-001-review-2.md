# SPEC Review Report: SPEC-GOOSE-SUBAGENT-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.86

Reasoning context ignored per M1 Context Isolation. Audit is based solely on `spec.md` (v0.2.0) and the iter1 report `SUBAGENT-001-audit.md` for regression check.

---

## Must-Pass Results

### MP-1 REQ Number Consistency — **PASS**

23 REQ IDs `REQ-SA-001` through `REQ-SA-023` present. Verified by line-by-line read of spec.md:L111-170. Zero-padding consistent (3 digits). No duplicates. No gaps.

Note (informational, not a defect): REQ-SA-022 and REQ-SA-023 (Unwanted, spec.md:L156, L162) appear in document before REQ-SA-019/020/021 (Optional, spec.md:L166-170) because the new Unwanted REQs were grouped with §4.4. IDs themselves remain monotonic 001..023; this is an EARS category-grouped layout, not an MP-1 violation.

### MP-2 EARS Format Compliance — **PASS**

All 23 REQs match one of the five EARS patterns:
- Ubiquitous (4): REQ-SA-001 to 004 (spec.md:L111-117) — "Every X shall Y" / "The X shall Y"
- Event-Driven (6): REQ-SA-005 to 010 (spec.md:L121-131) — "When X, the Y shall Z"
- State-Driven (3): REQ-SA-011 to 013 (spec.md:L135-142) — "While X, Y shall Z"
- Unwanted (7): REQ-SA-014 to 018 + 022 + 023 (spec.md:L146-162) — "shall not"; REQ-SA-022 uses "If ... then ... shall not"
- Optional (3): REQ-SA-019, 020, 021 (spec.md:L166-170) — "Where X, Y shall Z"

REQ-SA-022 (spec.md:L156) explicitly uses the canonical "If undesired condition, then the system shall not..." Unwanted-EARS variant. Properly formed.

### MP-3 YAML Frontmatter Validity — **PASS**

spec.md:L1-14 frontmatter:
- `id: SPEC-GOOSE-SUBAGENT-001` ✔ (string)
- `version: 0.2.0` ✔ (string)
- `status: planned` ✔ (string)
- `created_at: 2026-04-21` ✔ (renamed from `created`, ISO date)
- `updated_at: 2026-04-25` ✔
- `priority: P0` ✔ (project-domain convention; non-blocking)
- `labels: [subagent, runtime, isolation, memory, phase-2]` ✔ (array, 5 entries)

D2 (labels missing) and D3 (created→created_at) from iter1 confirmed RESOLVED.

### MP-4 Language Neutrality — **N/A**

SPEC scoped to `internal/subagent/` Go package (spec.md:L72, L289). Not multi-language template-bound. Auto-pass.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75-1.0 band | ResumeAgent canonical signature now unified (spec.md:L88, L201-203, L402-406). Memory update path explicit (REQ-SA-021 spec.md:L170). PlanModeRequired fully specified (REQ-SA-022). Minor residual: D4/D11 AgentID parsing/atomicity unaddressed. |
| Completeness | 0.85 | 0.75-1.0 band | Frontmatter complete; HISTORY captures iter1 fix mapping (spec.md:L23); 11 specific Exclusions (spec.md:L594-606); REQ count grew 20→23 covering all iter1 majors. Minor: §6.2 AgentDefinition struct missing `MemoryScopes` field (see N2). |
| Testability | 0.90 | 0.75-1.0 band | 21 AC (was 12), each with Given/When/Then + explicit `(Verifies: REQ-SA-XXX)` cite. AC-SA-014 binary-tests flock semantics + ErrMemdirLockUnsupported (spec.md:L241-244). AC-SA-021 binary-tests goleak (spec.md:L276-279). |
| Traceability | 0.95 | 1.0 band | Every AC cites at least one REQ via `(Verifies: ...)`. Every REQ has at least one AC: REQ-SA-001→AC-001, 002→AC-005, 003→AC-004, 004→AC-013, 005→AC-001, 006→AC-002, 007→AC-003, 008→AC-005, 009→AC-006, 010→AC-007, 011→AC-008, 012→AC-014, 013→AC-010, 014→AC-009, 015→AC-015, 016→AC-011, 017→AC-012, 018→AC-016, 019→AC-017, 020→AC-018, 021→AC-019, 022→AC-020, 023→AC-021. 23/23 covered. |

---

## Defects Found

### Newly identified in iter 2

**N1. [spec.md:L156, L162 vs L166] REQ ordering non-monotonic in §4 — Severity: informational**

REQ-SA-022 and REQ-SA-023 are correctly placed under §4.4 Unwanted but appear at lines 156 and 162, before REQ-SA-019/020/021 at lines 166-170 (§4.5 Optional). Reading top-down, the ID sequence is 001..018, 022, 023, 019, 020, 021. IDs are unique and complete (MP-1 PASS), so this is presentational only. Recommend renumbering Optional REQs to 022/023/024 OR moving the Unwanted additions to the end of §4.4 and renumbering. Not blocking.

**N2. [spec.md:L323-339] `AgentDefinition` struct missing `MemoryScopes` field while REQ-SA-021 and AC-SA-019 reference it — Severity: major**

REQ-SA-021 (spec.md:L170) reads: "at least one of `def.MemoryScopes` is non-empty". AC-SA-019 (spec.md:L267) reads: "Given `def.MemoryScopes == [ScopeProject]`". But the §6.2 Go type listing for `AgentDefinition` (spec.md:L323-339) enumerates the fields AgentType, Name, Description, Tools, UseExactTools, Model, MaxTurns, PermissionMode, Effort, SystemPrompt, MCPServers, Isolation, Source, Background, CoordinatorMode — **no `MemoryScopes` field**.

Implementer cannot satisfy REQ-SA-021/AC-SA-019 without adding a field that the SPEC's canonical type does not declare. Either add `MemoryScopes []MemoryScope` to AgentDefinition, or revise REQ-SA-021/AC-SA-019 to derive scopes from another source (e.g., loader-time configuration).

**N3. [spec.md:L156-160] `PlanModeApprove(agentID)` API referenced by REQ-SA-022 but not declared in §6.2 — Severity: minor**

REQ-SA-022(c) (spec.md:L159) reads: "Approval arrives via a `PlanModeApprove(agentID)` API call from the parent (orchestrator) which sets `PlanModeRequired = false`." The §6.2 spawn-API section (spec.md:L394-406) lists `RunAgent` and `ResumeAgent` only — no `PlanModeApprove` function signature. AC-SA-020 (spec.md:L271-274) tests this API ("호출자가 2초 뒤 `PlanModeApprove(agentID)` 호출"). Add the function signature to §6.2 (e.g., `func PlanModeApprove(parentCtx context.Context, agentID string) error`).

### Regression failures from iter 1 (unresolved)

**D4 (UNRESOLVED). [spec.md:L111] REQ-SA-001 atomic spawnIndex unspecified — Severity: minor**

REQ-SA-001 unchanged. AgentID format `{agentName}@{sessionId}-{spawnIndex}` still does not specify atomic monotonic counter semantics. Concurrent `RunAgent` from same parent could race on spawnIndex.

**D5 (UNRESOLVED). [spec.md:L125] REQ-SA-007 still uses literal "5-second" — Severity: minor**

REQ-SA-007 text unchanged: "after a 5-second inactivity period". AC-SA-003 (spec.md:L189) introduces the named constant `DefaultBackgroundIdleThreshold(5 s)`, but the REQ itself was not updated to reference the configuration handle. Inconsistency between REQ literal and AC-named-constant.

**D8 (PARTIAL). [spec.md:L150 vs L227] "Allow rule" source partially defined — Severity: minor**

AC-SA-011 (spec.md:L227) now specifies "allow rule 소스는 `settings.json` `subagent.permissions.allow`" — partial improvement. However, REQ-SA-016 (spec.md:L150) itself still says "unless an explicit allow rule is present" without referencing the source. REQ should be tightened to cite the same `settings.json` path the AC tests against.

**D9 (UNRESOLVED). [spec.md:L148] REQ-SA-015 SessionEnd dependency framing — Severity: minor**

REQ-SA-015 unchanged. Still asserts "a SessionEnd hook handler (HOOK-001) shall invoke ..." rather than conditional ("Given HOOK-001 emits SessionEnd"). AC-SA-015 (spec.md:L246-249) implicitly assumes HOOK-001 emits the event. If HOOK-001 spec lacks SessionEnd, REQ is unimplementable.

**D10 (UNRESOLVED). [spec.md:L127] REQ-SA-008 ordering of (c) and (d) ambiguous — Severity: minor**

Steps still listed as (a) write transcript, (b) DispatchSubagentStop, (c) close output channel, (d) mark Subagent.State. A consumer observing channel close before state is updated may transiently see `Subagent.State == Running`. Either swap (c)/(d) or document the consumer contract.

**D11 (UNRESOLVED). [spec.md:L111] AgentID delimiter parsing ambiguity — Severity: minor**

REQ-SA-018 (spec.md:L154) allows `-` in agent name. Combined with format `{agentName}@{sessionId}-{spawnIndex}`, round-trip parsing is ambiguous when agentName contains `-`. Not addressed.

**D12 (UNRESOLVED). [spec.md:L121] REQ-SA-005 failure path missing — Severity: minor**

REQ-SA-005 still describes only the success path (a-e). No REQ specifies what happens when QueryEngine creation, DispatchSubagentStart, or goroutine spawn fails. No AC tests failure modes.

### Resolved from iter 1

| iter1 ID | Status | Evidence |
|---|---|---|
| D1 (traceability gap) | RESOLVED | All 23 REQs covered by AC; every AC bears `(Verifies: REQ-SA-XXX)` cite (spec.md:L176-279) |
| D2 (labels missing) | RESOLVED | `labels: [subagent, runtime, isolation, memory, phase-2]` (spec.md:L13) |
| D3 (created → created_at) | RESOLVED | `created_at: 2026-04-21` (spec.md:L5) |
| D6 | N/A | Already demoted in iter1 |
| D7 (REQ-SA-012 peer concurrency) | RESOLVED | REQ-SA-012(a)(b)(c) explicit flock + LockFileEx + ErrMemdirLockUnsupported (spec.md:L137-140); AC-SA-014 binary-tests it (spec.md:L241-244) |
| D13 (ResumeAgent signature 3-way mismatch) | RESOLVED | §3.1 #9 explicitly cites §6.2 as canonical and AC-SA-006 as conformant (spec.md:L88); AC-SA-006 updated to `(ctx, "researcher@sess-old-2")` (spec.md:L203) |
| D14 (AC-SA-004 union wording) | RESOLVED | "nearest wins": local→project→user 우선순위 + "합집합 의미의 union" (spec.md:L194) |
| D15 (memory update REQ missing) | RESOLVED | REQ-SA-021 introduces `memory.append` built-in tool with full contract (spec.md:L170); AC-SA-019 tests it (spec.md:L266-269) |
| D17 (PlanModeRequired field unbacked) | RESOLVED | REQ-SA-022 defines plan-mode approval semantics with timeout (spec.md:L156-160); AC-SA-020 tests both approval and timeout (spec.md:L271-274) |
| D18 (goroutine lifecycle) | RESOLVED | REQ-SA-023 defines ctx.Done() observation + 100ms grace + goleak CI (spec.md:L162); AC-SA-021 tests it (spec.md:L276-279) |

---

## Chain-of-Verification Pass

Second-pass findings:

1. **Re-verified all 23 REQ IDs** by sequential grep (spec.md:L111-170): 001..023 present, unique, zero-padded. Confirmed.

2. **Re-verified all 21 AC `(Verifies:)` annotations** (spec.md:L176-279): every AC explicitly cites at least one REQ-SA-NNN. Confirmed.

3. **Built REQ→AC reverse coverage matrix** (above): 23/23 REQs covered. Confirmed.

4. **Cross-checked AgentDefinition struct vs REQ references**: discovered N2 — `def.MemoryScopes` referenced in REQ-SA-021 and AC-SA-019 but absent from §6.2 type listing. Promoted to major.

5. **Cross-checked `PlanModeApprove` API**: discovered N3 — referenced in REQ-SA-022 but no signature in §6.2 spawn API listing.

6. **Re-checked Exclusions section** (spec.md:L594-606): 11 specific entries with cross-refs (QUERY-001, TEAM-001, PLUGIN-001, etc.). Specific, not vague. Confirmed.

7. **Re-checked HISTORY block** (spec.md:L18-23): version bump 0.1.0 → 0.2.0 with explicit defect-resolution mapping (D13, D15, D17, D18, D7, D1). Excellent traceability for an iter2 review.

8. **Cross-checked REQ-SA-012(b) flock semantics against AC-SA-014**: AC tests exactly the REQ contract (200 entries from peer A+B with race detector). Binary-testable. Confirmed.

9. **Cross-checked REQ-SA-022 timeout configuration vs AC-SA-020**: REQ specifies "default 300 s, configurable" (spec.md:L160), AC tests "300s 내 미승인 시" (spec.md:L274). Consistent.

10. **Verified iter1 minor defects D4/D5/D9/D10/D11/D12**: spec.md text at the cited lines is unchanged from v0.1.0. These are individual regression failures.

No further new major defects beyond N2 found in second pass.

---

## Regression Check (Iteration 2)

Defects from iter 1 (17 defects total: 1 must-pass + 4 major + 12 minor; D6 demoted in iter1):

| iter1 ID | Severity | Status |
|---|---|---|
| MP-3 (labels) | must-pass | RESOLVED |
| D1 | major | RESOLVED |
| D2 | major | RESOLVED (= MP-3) |
| D3 | minor | RESOLVED |
| D4 | minor | **UNRESOLVED** |
| D5 | minor | **UNRESOLVED** |
| D6 | n/a | (demoted in iter1) |
| D7 | major | RESOLVED |
| D8 | minor | **PARTIAL** (AC tightened, REQ still vague) |
| D9 | minor | **UNRESOLVED** |
| D10 | minor | **UNRESOLVED** |
| D11 | minor | **UNRESOLVED** |
| D12 | minor | **UNRESOLVED** |
| D13 | major | RESOLVED |
| D14 | minor | RESOLVED |
| D15 | major | RESOLVED |
| D17 | major | RESOLVED |
| D18 | major | RESOLVED |

Summary: **6 unresolved + 1 partial out of 17**. All 5 major defects (D1, D7, D13, D15, D17, D18) plus the must-pass (MP-3) are RESOLVED. The remaining 6 are all minor.

Per audit-rubric Regression rule: "Unresolved defects from a prior iteration are automatically FAIL regardless of other scores." Strictly applied → FAIL.

Stagnation analysis (across iter1→iter2): D4, D5, D9, D10, D11, D12 unchanged for one iteration. None yet meet the "blocking defect" threshold (3 iterations unchanged). Not flagged as stagnation yet.

---

## Recommendation (FAIL — must remediate before iter 3 final)

Blocker (major):

1. **N2** — Add `MemoryScopes []MemoryScope` field to `AgentDefinition` struct (spec.md:L323-339). Without it, REQ-SA-021 and AC-SA-019 cannot be implemented as specified. Suggested placement: after `MCPServers` (alphabetical-ish grouping).

Required (minor regression closures):

2. **D4** — Tighten REQ-SA-001 (spec.md:L111) to specify atomic monotonic spawnIndex allocation: e.g., "spawnIndex shall be allocated atomically per parent session via `atomic.AddInt64`".

3. **D5** — Replace "5-second" in REQ-SA-007 (spec.md:L125) with named constant `DefaultBackgroundIdleThreshold` (5 s, configurable via `subagent.background.idle_threshold`). Already used by AC-SA-003.

4. **D8** — Tighten REQ-SA-016 (spec.md:L150) to cite `settings.json` `subagent.permissions.allow` directly (matching AC-SA-011's specification).

5. **D9** — Reframe REQ-SA-015 (spec.md:L148) as conditional on HOOK-001's SessionEnd event, or add an explicit dependency-existence assumption.

6. **D10** — Specify ordering in REQ-SA-008 (spec.md:L127): "(d) shall happen before (c)" (mark state before closing channel) to prevent observer race.

7. **D11** — Tighten REQ-SA-001 AgentID format: forbid `-` in `agentName` (revise REQ-SA-018) OR define escape rule, OR use a non-conflicting delimiter (e.g., `~`).

8. **D12** — Add error-path REQ for REQ-SA-005 covering QueryEngine creation failure, DispatchSubagentStart failure, goroutine spawn failure modes.

Recommended (informational):

9. **N1** — Renumber Optional REQs to 022/023/024 OR move Unwanted additions to end of §4.4 to restore monotonic top-to-bottom ordering. Cosmetic only.

10. **N3** — Add `PlanModeApprove` function signature to §6.2 spawn API listing (spec.md:L394-406).

After these fixes, re-invoke plan-auditor for iter 3 (final).

---

## Summary Table

| Category | Iter 1 | Iter 2 |
|---|---|---|
| Must-Pass FAIL | 1 (MP-3) | 0 |
| Major defects | 5 (D1, D7, D13, D15, D17, D18) | 1 (N2) |
| Minor defects | 7 | 8 (6 regression + 2 new) |
| Resolved from iter1 | — | 9 of 16 actionable (incl. MP-3) |
| Unresolved from iter1 | — | 6 minor + 1 partial |
| Score | 0.74 | 0.86 |

**Verdict: FAIL**. Substantial improvement (5 major defects + must-pass all closed; traceability now near-perfect at 0.95). Remaining blockers: 1 new major (N2 — `MemoryScopes` type/REQ inconsistency), and 6 unresolved minor defects from iter 1 (regression rule auto-FAIL). Recommend one more revision cycle to close N2 and the 6 minor regression items.

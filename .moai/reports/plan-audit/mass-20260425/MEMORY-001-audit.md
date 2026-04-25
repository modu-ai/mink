# SPEC Review Report: SPEC-GOOSE-MEMORY-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Only spec.md and research.md were read.

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-MEMORY-001 through REQ-MEMORY-020 are sequential, no gaps, no duplicates, 3-digit zero-padded. Verified by enumerating spec.md:L97–L143.
- [FAIL] **MP-2 EARS format compliance (ACs)**: Acceptance Criteria AC-MEMORY-001…016 are written in **Given-When-Then** scenario form (spec.md:L149 "각 AC는 Given-When-Then", then L151–L229). Per M3 rubric, every AC must match one of the five EARS patterns. GWT ≠ EARS. All 16 ACs fail this rubric. REQ items themselves (L97–L143) ARE EARS-compliant and tagged correctly (Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional), but the AC layer is not.
- [FAIL] **MP-3 YAML frontmatter validity**: Two required fields violate the schema.
  - spec.md:L5 uses `created: 2026-04-21` — required key is `created_at` (ISO date string). Field name mismatch = missing field.
  - `labels` field is entirely absent (spec.md:L1–L13). Required field missing.
  - Present fields: id (L2), version (L3), status (L4), priority (L8). Extra fields (author, issue_number, phase, size, lifecycle, updated) are not required but do not substitute for `created_at` or `labels`.
- [N/A] **MP-4 Section 22 language neutrality**: Single-language SPEC (Go only). Auto-pass.

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band (minor ambiguity in 1–2 requirements) | Contradictory optional-method count (see D3). REQ-MEMORY-013 uses past tense "returned" — state vs event ambiguity. spec.md:L125 |
| Completeness | 0.50 | 0.50 band (multiple missing or substantively empty sections, frontmatter missing ≥2 fields) | YAML missing `created_at`, `labels` (L1–L13). No normative REQ/AC for GC/eviction/retention despite §3.2 exclusion implying `max_rows=10,000` default (L87). Section structure otherwise full. |
| Testability | 0.75 | 0.75 band (one AC not precisely binary-testable) | AC-MEMORY-015 "형식은 구현 결정" (spec.md:L224) — leaves output format to implementer, reducing binary testability. AC-MEMORY-012 "가시적 effect 확인" (spec.md:L209) — what constitutes "visible effect" is unspecified. |
| Traceability | 0.50 | 0.50 band (multiple REQs lack ACs) | **6 of 20 REQs (30%) have no corresponding AC**: REQ-MEMORY-004 (IsAvailable no-I/O), REQ-MEMORY-011 (OnPreCompress aggregate), REQ-MEMORY-013 (Init error suppression), REQ-MEMORY-017 (messages retention prohibition), REQ-MEMORY-019 (Go import path factory), REQ-MEMORY-020 ("## Memory Context" wrapping). |

## Defects Found

- **D1**. spec.md:L5 — `created: 2026-04-21` uses non-canonical key; required field is `created_at`. — Severity: **critical** (MP-3 FAIL).
- **D2**. spec.md:L1–L13 — YAML frontmatter lacks `labels` field entirely. — Severity: **critical** (MP-3 FAIL).
- **D3**. spec.md:L31 vs L71 vs §6.2 — Method-count contradiction. §1 line 31 and §4 line 71 both state "선택 7" (7 optional methods), but §4 line 71 enumerates nine names (`SystemPromptBlock, Prefetch, QueuePrefetch, SyncTurn, HandleToolCall, OnTurnStart, OnSessionEnd, OnPreCompress, OnDelegation`). §6.2 Go interface (L265–L290) also exposes 9 optional methods. Total = 4 required + 9 optional = 13, not 11. Three locations assert "11 methods". — Severity: **major** (internal inconsistency; REQ-MEMORY-003 and REQ coverage depends on correct count).
- **D4**. spec.md:L97–L143 — Six REQs have no AC coverage: REQ-MEMORY-004, 011, 013, 017, 019, 020. Traceability gap of 30%. — Severity: **major** (AC-5 / M3 Traceability rubric).
- **D5**. spec.md:L149, L151–L229 — All 16 ACs use Given-When-Then test scenario form rather than EARS patterns, violating M3 rubric AC-1. While EARS REQs are present and correct, the AC layer mixing forms is a plan-auditor MP-2 concern. — Severity: **major**.
- **D6**. spec.md:L129 — REQ-MEMORY-014 hardcodes quantitative budgets ("50ms", "40ms") as normative values without referencing a configurable knob or a tolerance band. AC-MEMORY-011 (L201–L204) tests the same numbers verbatim, coupling REQ to a timing-sensitive runtime value. Risk R3 (L598) later says "필요 시 budget을 config로 노출" — REQ and Risk are not aligned. — Severity: **minor**.
- **D7**. spec.md:L87 — Exclusion declares `max_rows` default of 10,000 but no REQ enforces this ceiling, no AC verifies it, and no GC/eviction semantics are defined (LRU? FIFO? error when full?). User's audit focus on "용량 한계, GC/eviction 정책" is not answered normatively. — Severity: **major** (normative gap on user-flagged focus area).
- **D8**. spec.md:L224 — AC-MEMORY-015 "형식은 구현 결정" delegates format to implementation, making the AC non-binary-testable. — Severity: **minor**.
- **D9**. spec.md:L209 — AC-MEMORY-012 "내부 goroutine이 100ms 후 fetch 완료(테스트에서 가시적 effect 확인)" — "visible effect" is unspecified; tester must infer. — Severity: **minor**.
- **D10**. spec.md:L97–L143 — REQs embed Go-specific identifiers (`ErrBuiltinRequired`, `RegisterPlugin`, `MemoryManager.GetToolSchemas()`, `errors.Is`). Per Group 3 RQ-3/RQ-4, REQs should express behavior/outcome, not implementation detail. Infrastructure SPECs commonly do this, but severity is noted. — Severity: **minor**.
- **D11**. Memory-type classification (user/feedback/project/reference) requested in user audit brief is absent. spec.md:§6.3 (L418) defines `source TEXT` with values `"user" | "inferred" | "tool_result"` — a different taxonomy. If the user's four-category classification was an expectation, the SPEC does not meet it; if not, the SPEC defines its own taxonomy but without rationale documentation. — Severity: **minor** (clarify vs user's expected taxonomy).
- **D12**. TRAJECTORY-001 boundary is mentioned (§2.1 L46, §7 L579, research.md §10 L406) but no REQ explicitly defines the contract between MEMORY-001 and TRAJECTORY-001 for session_id handoff (who creates, who owns, lifecycle). Risk boundary is informal. — Severity: **minor**.

## Chain-of-Verification Pass

Second-pass findings (re-reading by section, not skimming):

- Re-counted optional methods three independent times across §1, §4.2 intro, §6.2 Go interface block. Count is **9 optional**, not 7. The "선택 7" label is stated in two places but contradicted by enumeration in both. D3 confirmed.
- Re-traced every REQ against every AC line by line (spec.md:L97–L229). Confirmed six REQs without AC: 004, 011, 013, 017, 019, 020. D4 confirmed.
- Re-read Exclusions section (L637–L649) — 11 specific bullets; no issues with Exclusions section itself.
- Re-read HISTORY (L17–L21) — one entry, acceptable for initial draft.
- Checked for contradictions: §3.2 says Pruning is OUT except "max_rows 기본값만(10,000 행)" — but nowhere in REQ/AC. Consistency check: Risk R5 (L600) says `SystemPromptBlock` first N KB (8KB default) — also not in REQ. Two configuration defaults live only in prose, not normative REQs.
- Re-verified `created` vs `created_at` against schema. Not a typo concern — the schema requires the exact key name.
- No new major defects surfaced on second pass.

## Recommendation (Actionable Fixes for manager-spec)

1. **Fix YAML frontmatter (critical)**: Change `created: 2026-04-21` to `created_at: 2026-04-21` (spec.md:L5). Add `labels: [memory, infrastructure, phase-4]` (or appropriate array) before the closing `---`.
2. **Reconcile optional-method count (major)**: Either remove three methods from the enumeration at §4.2 (L71) and §6.2 interface to match "7", or update all three locations (L31, L71, §1 overview) to "선택 9" and total "13 methods". Choose based on whether `QueuePrefetch`, `OnPreCompress`, `OnDelegation` are actually needed.
3. **Close AC coverage gap (major)**: Add ACs for the six uncovered REQs:
   - REQ-MEMORY-004 → AC verifying `IsAvailable()` performs no network call (mock/spy network layer).
   - REQ-MEMORY-011 → AC verifying aggregated pre-compress hints appended to compaction prompt.
   - REQ-MEMORY-013 → AC where PluginA.Initialize returns error; verify subsequent OnTurnStart/SyncTurn hooks are skipped for that session, then re-invoked on next session.
   - REQ-MEMORY-017 → AC verifying post-hook `messages[]` slice mutation does not corrupt persisted state (providers must copy).
   - REQ-MEMORY-019 → AC verifying unknown plugin name returns `ErrUnknownPlugin`; known name invokes factory.
   - REQ-MEMORY-020 → AC verifying non-empty `OnPreCompress` string is wrapped in `## Memory Context` section prefix.
4. **Add normative GC/eviction requirements (major)**: Introduce REQ-MEMORY-021 stating the max_rows cap behavior (error on insert? silent LRU drop? warn-log?) plus matching AC. User's audit brief explicitly flagged this.
5. **Rewrite ACs in EARS or hybrid form (major)**: Either (a) convert GWT ACs to EARS "When X, the system shall Y" form, or (b) explicitly document that this SPEC uses GWT for ACs as a project-level convention and update the plan-auditor rubric input. Option (a) preferred.
6. **Parameterize timing constants (minor)**: REQ-MEMORY-014 should reference `config.memory.dispatch_budget_ms` (default 50) and `config.memory.provider_budget_ms` (default 40) rather than hardcoding magic numbers.
7. **Tighten non-binary ACs (minor)**: 
   - AC-MEMORY-015: specify the append-line format (e.g., `- [<session_id>] <content>` regex).
   - AC-MEMORY-012: replace "가시적 effect" with a concrete assertion (e.g., "a channel receives the prefetch result within 200ms").
8. **Clarify TRAJECTORY-001 contract (minor)**: Add a REQ stating who owns session_id creation, format, and propagation from TRAJECTORY-001 into MemoryManager.Initialize.
9. **Reconcile memory-type taxonomy (minor)**: Document rationale for the three-value `source` taxonomy (user/inferred/tool_result) and explicitly address whether the user-feedback/project/reference classification is out of scope or renamed.

## Regression Check

N/A — iteration 1.

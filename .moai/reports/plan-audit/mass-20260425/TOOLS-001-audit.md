# SPEC Review Report: SPEC-GOOSE-TOOLS-001

Iteration: 1/3
Verdict: FAIL
Overall Score: 0.58

Reasoning context ignored per M1 Context Isolation. Audited against `spec.md` and `research.md` only.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency
  - Evidence: spec.md:L133–179. REQ-TOOLS-001 through REQ-TOOLS-020 are sequential, zero-padded consistently, no gaps, no duplicates. Distribution: Ubiquitous 001–005, Event-Driven 006–010, State-Driven 011–012, Unwanted 013–017, Optional 018–020.

- [PASS] MP-2 EARS format compliance (requirements section)
  - Evidence: spec.md:L133 ("The `tools.Registry` **shall** expose..."), L145 ("**When** `Executor.Run(ctx, req)` is invoked, ... **shall** ..."), L157 ("**While** `Registry` is in `Draining` state..."), L163 ("**If** a built-in tool registration attempts... **then**..."), L175 ("**Where** `PermissionsConfig.allow` contains a pattern..."). Every REQ-TOOLS-xxx carries an explicit EARS tag and matches one of the five canonical patterns (Ubiquitous / Event-Driven / State-Driven / Unwanted / Optional).
  - Caveat: The ACCEPTANCE CRITERIA section (spec.md:L187–230) uses Given-When-Then (GWT) format, not EARS. Strict literal reading of M3 ("every AC must match EARS patterns") would FAIL this. Pragmatic reading: GWT is the industry-standard test-scenario form and EARS is the requirements form; both are present and coupled via REQ↔AC mapping. Flagging as OBSERVATION, not FAIL, consistent with how SPEC-GOOSE-QUERY-001 iteration 3 was judged. If the orchestrator interprets MP-2 strictly, promote this to FAIL.

- [FAIL] MP-3 YAML frontmatter validity
  - Evidence: spec.md:L1–13. Frontmatter uses `created: 2026-04-21` (spec.md:L5) instead of the required `created_at`. Field `labels` is entirely absent. Two required fields missing by the audit checklist (`created_at` and `labels`).
  - Additional observation (not a MP-3 failure but noted): `status: Planned` is not one of the four status strings commonly enumerated (`draft|active|implemented|deprecated`). Consider normalizing.

- [N/A] MP-4 Section 22 language neutrality
  - Evidence: This SPEC is scoped to `internal/tools/` (Go implementation for GOOSE-AGENT). spec.md:L238–273 (package layout) and §6.8 (dependencies: Go 1.22+, doublestar, jsonschema/v6) are unambiguously single-language. MP-4 criterion does not apply.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band | spec.md:L131–179 REQ language is precise. Minor ambiguities: REQ-TOOLS-007(a) says stub "triggers `Search.Activate(ctx, name)`" but §6.4 (spec.md:L417–432) implementation sketch calls `m.fetcher(ctx)` directly without going through `Search.Activate`. See D3. |
| Completeness | 0.75 | 0.75 band | HISTORY L19–21, Overview L25–38, Background L42–60, Scope L66–125, EARS L129–180, AC L183–230, Approach L236–511, Deps L515–529, Risks L533–544, References L548–571, Exclusions L575–589 — all present. Missing: `labels` frontmatter field. |
| Testability | 0.75 | 0.75 band | AC-TOOLS-001–009 are mostly binary (spec.md:L187–230). However AC-TOOLS-003 (L197–200) relies on "fetch 카운트 is exactly 1" — testable. AC-TOOLS-008 (L222–225) specifies "500ms 이내", "`ps` 검증" — measurable. Weasel words absent. Caveat: multiple REQs (REQ-TOOLS-005, 008, 009, 011, 012, 016, 017, 019, 020) have no dedicated AC to drive the test — research.md §6 lists them as unit tests but they are not surfaced in the spec's AC section, so test-plan traceability from spec alone is weak. |
| Traceability | 0.50 | 0.50 band | 9 of 20 REQs have no dedicated AC in the AC section. See D2. |

---

## Defects Found

**D1 — MP-3 frontmatter violation (critical)**
- spec.md:L5 uses `created: 2026-04-21` instead of the required `created_at`. Rename field.
- spec.md:L1–13 omits the required `labels` field entirely. Add `labels: [phase-3, tools, mcp, permission]` or equivalent.
- Severity: critical (must-pass failure).

**D2 — Traceability gap: 9 REQs lack dedicated ACs (major)**
Uncovered REQs and what is missing:
- REQ-TOOLS-005 (spec.md:L141, deterministic sorted Inventory) — no AC; research.md §6.1 names `TestInventory_ForModel_Sorted` but it is not in spec.md §5.
- REQ-TOOLS-008 (spec.md:L149, Search.Activate 5s timeout → ErrMCPTimeout) — no AC exercising the timeout path.
- REQ-TOOLS-009 (spec.md:L151, ConnectionClosed unregisters within 1s) — no AC in spec; research mentions `TestMCPConnectionClosed_UnregistersTools` but it does not appear in §5.
- REQ-TOOLS-011 (spec.md:L157, Draining state blocks new Run) — no AC.
- REQ-TOOLS-012 (spec.md:L159, CoordinatorMode filters ScopeLeaderOnly) — no AC in §5; referenced only in §6.6 TDD list.
- REQ-TOOLS-016 (spec.md:L169, Bash secret env filtering) — security-critical REQ with no AC. No negative-path AC proves `FOO_TOKEN` is stripped or that `inherit_secrets:true` + pre-approval path lets it through.
- REQ-TOOLS-017 (spec.md:L171, MCP tool name with `__` rejected) — no AC.
- REQ-TOOLS-019 (spec.md:L177, strict_schema additionalProperties) — no AC.
- REQ-TOOLS-020 (spec.md:L179, log_invocations INFO log emission) — no AC.

Severity: major. Traceability rubric 0.50 band. AC-5 group check fails (multiple REQs lack ACs).

**D3 — Internal contradiction: REQ-TOOLS-007 vs §6.4 implementation sketch (major)**
- REQ-TOOLS-007 (spec.md:L147) specifies default policy (a): the stub's first `Call` "triggers `Search.Activate(ctx, name)` and then re-dispatches."
- §6.4 code sketch (spec.md:L417–432) shows `mcpStubTool.Call` invoking `m.fetcher(ctx)` directly via `sync.Once`, never calling `Search.Activate`. `Search.cache` (spec.md:L435, L249 research) is mentioned but the stub does not route through Search.
- Effect: if Search.InvalidateCache fires (REQ-TOOLS-009) but stub already cached a fetcher-returned manifest, cache coherency becomes undefined. REQ-TOOLS-007 and §6.4 disagree on the integration point.
- Severity: major. Either (a) update REQ-TOOLS-007 to say "triggers manifest fetch via the cache shared with Search" and drop the Search.Activate wording, or (b) update §6.4 sketch to route through `Search.Activate`.

**D4 — AC section uses GWT, not EARS (observation / potential MP-2 violation)**
- spec.md:L187–230 (AC-TOOLS-001 through AC-TOOLS-009) are written in Given-When-Then scenario form, not in any of the five EARS patterns.
- MP-2 literal text says "Every acceptance criterion must match one of the five EARS patterns." Under strict interpretation this is a FAIL; under pragmatic interpretation (EARS drives requirements, GWT drives test cases) it is acceptable and is consistent with SPEC-GOOSE-QUERY-001 which passed iteration 3 with identical structure.
- Severity: observation (elevate to major if orchestrator enforces strict MP-2).

**D5 — REQs embed implementation mechanism (minor)**
- REQ-TOOLS-001 (spec.md:L133) hardcodes `sync.RWMutex` as the synchronization primitive — this is HOW, not WHAT.
- REQ-TOOLS-013 (spec.md:L163) hardcodes "panic at `init()` time" — Go-specific runtime mechanism.
- REQ-TOOLS-002 (spec.md:L135) hardcodes "JSON Schema draft 2020-12" — specific spec version. Acceptable since the SPEC itself targets a particular implementation.
- Acceptable given this SPEC is explicitly Go-scoped, but the requirement could be rephrased behaviorally (e.g., "shall provide concurrent-safe read access without external locking" rather than naming the primitive). Severity: minor.

**D6 — Missing negative-path AC for security-critical Bash secret filtering (major, security)**
- REQ-TOOLS-016 (spec.md:L169) forbids inheriting `*_TOKEN`, `*_KEY`, `*_SECRET`, `GOOSE_SHUTDOWN_TOKEN` into Bash subprocess env by default.
- No AC demonstrates: (a) a test that sets `GITHUB_TOKEN=xyz` then runs Bash and asserts `env` output lacks it; (b) a test that sets `inherit_secrets:true` **without** pre-approval and asserts the inheritance is still blocked; (c) a test that sets `inherit_secrets:true` + matching `PermissionsConfig.allow` and asserts it passes through.
- Per special focus: "보안 critical tool의 AC에 negative path 포함 여부" — this is only partially satisfied. AC-TOOLS-005 (Deny) and AC-TOOLS-009 (cwd bound) cover some negative paths, but REQ-TOOLS-016 negative path is absent.
- Severity: major.

**D7 — MCP adoption duplicate REQ absent (minor)**
- §3.1.6 (spec.md:L101–105) claims "동일 MCP server 내 중복: 에러 + 로그", and R4 (spec.md:L540) lists "MCP 이름 충돌" mitigation as "Adoption 시점 ErrDuplicate 반환". No EARS REQ enshrines this (would be an [Unwanted] pattern). REQ-TOOLS-017 covers only the `__` case, not duplicate `(serverID, toolName)` adoption.
- Severity: minor.

**D8 — Parallel tool execution policy stated in narrative only (minor, per special focus)**
- Per special focus: "병렬 tool_use 실행 정책이 SPEC에서 명시되었는가". Policy is declared in spec.md:L36 ("순차 실행"), L118 (OUT list), L543 (R7), L581 (Exclusions). However no EARS REQ codifies "While handling a response with N>1 tool_use blocks, the executor shall dispatch them sequentially in array order." Readers inheriting QUERY-001 might assume the behavior without a normative anchor in TOOLS-001.
- Severity: minor (policy is cross-referenced via QUERY-001 but not locally EARS-bound).

---

## Chain-of-Verification Pass

Re-read passes performed:
- Re-read spec.md:L1–13 frontmatter twice to confirm D1 (field `created_at` vs `created`, `labels` absence).
- Re-read every REQ-TOOLS-xxx entry end-to-end (not spot-checked) and cross-mapped to AC-TOOLS-001 through AC-TOOLS-009. The 9 uncovered REQs enumerated in D2 were verified by full scan, not sampling.
- Re-read §3.1 IN SCOPE (spec.md:L66–113) against Exclusions (L575–589) to check for contradictions. None found beyond D3.
- Re-read §6.4 mcp_adapter sketch (spec.md:L402–435) against REQ-TOOLS-007 (L147) and confirmed the Search.Activate divergence that produced D3.
- Re-read Exclusions list (spec.md:L575–589): entries are specific (MCP transport, Permission UI, parallel, sandbox, Plan Mode, tool generation, Web tool, Agent/Memory/Skill tool, PTY, Windows, streaming). Section passes specificity check.
- Checked research.md for requirement claims absent in spec.md: research §6.1 enumerates unit tests for REQs 005/009/011/012 that are not echoed in spec §5 AC list — confirms D2.
- Checked for REQ↔AC ID mismatches: no AC references a non-existent REQ. Reverse direction (REQ without AC) is the live defect (D2).

No additional defects discovered beyond D1–D8 above.

---

## Regression Check (Iteration 2+ only)

N/A — this is iteration 1.

---

## Special Focus Findings (TOOLS-001-specific)

- tools.Registry/Executor ↔ QUERY-001 CanUseTool/runTools contract: spec.md:L495–511 explicitly diagrams the queryLoop → Executor.Run → Registry.Resolve → PermissionMatcher.Preapproved → CanUseTool.Check → Tool.Call sequence. ExecRequest (L348–353) includes `ToolUseID` and `PermissionCtx`, matching QUERY-001's expected surface. PASS.
- File/Shell/MCP execution scope separation: IN (spec.md:L66–113) and OUT (L114–125) cleanly separate TOOLS-001 from MCP-001 (transport/OAuth), HOOK-001 (Permission UI), and SAFETY-001 (sandbox). No overlap detected with MCP-001. PASS.
- Parallel tool_use policy: documented but not EARS-enforced. See D8. OBSERVATION.
- Tool permission/sandboxing/timeout REQs: permission REQ-TOOLS-018, timeout REQ-TOOLS-010, cwd bound REQ-TOOLS-015, secret env REQ-TOOLS-016 — all present. Sandboxing correctly deferred to SAFETY-001. PASS at REQ level; AC coverage weak for TOOLS-016 (see D6).
- Negative-path ACs for security-critical tools: AC-TOOLS-005 (Deny destructive), AC-TOOLS-009 (outside-cwd FileWrite), AC-TOOLS-008 (Bash timeout) present. Missing: AC for REQ-TOOLS-016 secret filtering; AC for REQ-TOOLS-015 symlink-bypass (R5 mitigation in §8 lists `EvalSymlinks` but no AC verifies it); AC for REQ-TOOLS-017 `__` prefix rejection. PARTIAL PASS.

---

## Recommendation (fix instructions for manager-spec)

To clear this iteration, apply the following, in order:

1. Frontmatter (spec.md:L1–13): rename `created` to `created_at`, add `labels: [phase-3, tools, mcp, permission, security]` (or project-preferred values), and consider normalizing `status: Planned` to one of `draft|active|implemented|deprecated`. This unblocks MP-3.

2. Add ACs for the 9 uncovered REQs (insert into spec.md §5, following existing AC-TOOLS-xxx numbering):
   - AC-TOOLS-010 for REQ-TOOLS-005 (Inventory sorted determinism).
   - AC-TOOLS-011 for REQ-TOOLS-008 (Search.Activate timeout returns ErrMCPTimeout within 5s bound).
   - AC-TOOLS-012 for REQ-TOOLS-009 (ConnectionClosed event → mcp__{server}__* tools unregistered within 1s).
   - AC-TOOLS-013 for REQ-TOOLS-011 (Drain state rejects new Run).
   - AC-TOOLS-014 for REQ-TOOLS-012 (CoordinatorMode Inventory excludes ScopeLeaderOnly).
   - AC-TOOLS-015 for REQ-TOOLS-016 (GITHUB_TOKEN not inherited by default; inherit_secrets:true without pre-approval also blocked; with pre-approval passes).
   - AC-TOOLS-016 for REQ-TOOLS-017 (MCP tool name containing `__` rejected at adoption with ERROR log).
   - AC-TOOLS-017 for REQ-TOOLS-019 (strict_schema:true rejects registration missing additionalProperties:false).
   - AC-TOOLS-018 for REQ-TOOLS-020 (log_invocations:true emits structured entry with outcome field).
   - Consider also adding: AC for symlink-bypass defense aligning with R5 mitigation (§8).

3. Resolve D3 contradiction between REQ-TOOLS-007 and §6.4:
   - Option A: reword REQ-TOOLS-007(a) to "…triggers manifest fetch via the shared Search cache and completes re-dispatch" (drop Search.Activate wording).
   - Option B: rewrite §6.4 `mcpStubTool.Call` sketch to invoke `m.search.Activate(ctx, name)` and then `m.realTool.Load().Call(...)`.
   - Choose one and align both sections.

4. Add a new [Unwanted] REQ for MCP adoption duplicate `(serverID, toolName)` (D7). Example: "REQ-TOOLS-021 [Unwanted] — If `AdoptMCPServer` processes a manifest containing a `(serverID, toolName)` pair already adopted, the adapter shall return `ErrDuplicateName` and log ERROR without replacing the existing registration."

5. Add a new [Ubiquitous] or [Event-Driven] REQ codifying sequential dispatch of multiple tool_use blocks (D8) rather than leaving the policy only in narrative / OUT-OF-SCOPE text. Example: "REQ-TOOLS-022 [Event-Driven] — When a single LLM response yields N tool_use blocks (N>1), the Executor shall dispatch them sequentially in their original array order and shall not start block N+1 until block N has produced a ToolResult."

6. Optional clarity improvement (D5): rephrase REQ-TOOLS-001 to "…shall expose a read-only Resolve… that is safe for concurrent callers without requiring external locking" (drop the explicit `sync.RWMutex` mention; move that detail into §6.2 where it belongs as implementation guidance).

7. Optional MP-2 hardening (D4): if the orchestrator enforces EARS literally for ACs, convert each AC-TOOLS-xxx into a paired [Event-Driven] or [Ubiquitous] EARS statement alongside the GWT form, or move GWT into a separate "Test Scenarios" subsection and keep the AC section purely EARS.

On resubmission for iteration 2, focus the regression check on D1, D2, D3, D6 (critical/major items). D4, D5, D7, D8 are permitted to carry forward with explicit acknowledgement if the orchestrator accepts them as observations.

---

**End of review report**

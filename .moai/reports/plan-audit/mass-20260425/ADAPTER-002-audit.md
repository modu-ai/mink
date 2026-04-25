# SPEC Review Report: SPEC-GOOSE-ADAPTER-002
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.68

Note: Reasoning context from invocation prompt ignored per M1 Context Isolation. Audit conducted against `spec.md`, `tasks.md`, `progress.md`, `research.md`, and implementation source under `internal/llm/provider/`, `internal/llm/factory/registry_builder.go`, `internal/llm/router/registry.go`.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-ADP2-001 through REQ-ADP2-022 sequential with no gaps/duplicates (spec.md:L113-L163). AC-ADP2-001 through AC-ADP2-018 sequential (spec.md:L169-L257).
- [PASS] MP-2 EARS format compliance: All 22 REQs explicitly pattern-tagged ([Ubiquitous]/[Event-Driven]/[State-Driven]/[Unwanted]/[Optional]) and use canonical "shall/shall not" phrasing with When/While/If/Where triggers. Examples: REQ-ADP2-007 [Event-Driven] (spec.md:L127), REQ-ADP2-011 [State-Driven] (spec.md:L137), REQ-ADP2-014 [Unwanted] (spec.md:L145), REQ-ADP2-019 [Optional] (spec.md:L157).
- [FAIL] MP-3 YAML frontmatter validity:
  - `created_at` missing — SPEC uses `created: 2026-04-24` (spec.md:L5).
  - `labels` field entirely absent from frontmatter (spec.md:L1-L13).
  - `priority: P1` (spec.md:L8) not in standard set {critical, high, medium, low}.
  - `status: Planned` (spec.md:L4) non-standard (expected draft/active/implemented/deprecated). progress.md:L3 also shows `status: Completed` which further diverges. Three schema violations.
- [N/A] MP-4 Section 22 language neutrality: SPEC targets LLM-provider adapters, not multi-language code tooling. Auto-pass.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75 band (minor ambiguity) | EARS tagging strong, code samples concrete. Minor: REQ-ADP2-007 lists `glm-4.6/4.7/5` as thinking-capable (spec.md:L127) but §6.4 does not cross-reference `glm-4.5` — implementation extends list (thinking.go:L14-L19). |
| Completeness | 0.80 | 0.75 band | All 10 sections present + Exclusions (spec.md:L674-L689). Frontmatter missing labels + created_at (MP-3). Dependencies table complete (spec.md:L599-L610). |
| Testability | 0.90 | 1.0 band minus minor weasel | All 18 ACs Given/When/Then concrete + measurable URLs, headers, body fields. Minor: AC-ADP2-013 uses "token 합계 70K" but no precise token-counting contract for Kimi. |
| Traceability | 1.00 | 1.0 band | Every REQ-ADP2-0xx has at least one AC reference. Every AC cites its REQ (e.g., AC-ADP2-005 → REQ-ADP2-008 spec.md:L192; AC-ADP2-012 → REQ-ADP2-018 spec.md:L227). Reverse traceability complete. |

## Defects Found

### Critical (SPEC-vs-Implementation Divergence)

D1. `internal/llm/factory/registry_builder.go:L49-L155` — Severity: **critical**
RegisterAllProviders registers only **13 providers** (openai, xai, deepseek, ollama + 9 SPEC-002). Missing: `anthropic`, `google`. SPEC REQ-ADP2-005 (spec.md:L121) and AC-ADP2-016/017 (spec.md:L245-L252) explicitly mandate **15** AdapterReady providers post-RegisterAllProviders, and §1 line 31 states "15 provider 전부가 ProviderRegistry에 AdapterReady=true로 등록". The test in `registry_builder_test.go:L15,L28` doubles down by asserting `assert.Len(t, names, 13)`. Router metadata registry (router/registry.go) does show 15 AdapterReady=true meta entries — so meta registry matches SPEC, but the instance registry via RegisterAllProviders does not. progress.md:L68 claims "15" but test proves 13. Either SPEC must be amended to 13 (documenting anthropic/google credential deferral) or RegisterAllProviders must add those two factories.

D2. `internal/llm/provider/glm/thinking.go:L40-L44` — Severity: **critical**
REQ-ADP2-021 [Optional] (spec.md:L161) mandates: when `ThinkingBudget > 0` AND model supports budget thinking, inject `"thinking": {"type": "enabled", "budget_tokens": N}`. BuildThinkingField completely ignores `cfg.BudgetTokens` — only ever emits `{"type": "enabled"}`. The `provider.ThinkingConfig.BudgetTokens int` field exists (provider.go:L40) but is unread by GLM path. No test covers this case. Unimplemented Optional requirement.

D3. `internal/llm/provider/kimi/client.go` + `kimi/client_test.go` — Severity: **critical**
REQ-ADP2-022 [Optional] (spec.md:L163) and AC-ADP2-013 (spec.md:L229-L232) require: Kimi adapter logs an INFO-level advisory when routing to `moonshot-v1-128k`-class model with input messages exceeding 64K tokens. No such code exists in `kimi/client.go`. progress.md:L93 self-declares "AC-ADP2-013 ... N/A — 구현 범위 외 — REQ Optional" — but [Optional] EARS means "Where condition holds, shall" — it is still normative when the Where clause holds. Dropping without SPEC amendment is a contract violation.

D4. `internal/llm/provider/openrouter/client.go:L46-L83` — Severity: **major**
REQ-ADP2-020 [Optional] (spec.md:L159) mandates: when `OpenRouterOptions.PreferredProviders` non-empty, inject `"provider": {"order": [...], "allow_fallbacks": true}` into request body. The `Options` struct (openrouter/client.go:L22-L41) has no `PreferredProviders` field. No test covers it. Feature entirely missing.

### Major (SPEC Content Defects)

D5. spec.md:L5 — Severity: **major**
MP-3 schema violation. Frontmatter uses `created` instead of canonical `created_at` (ISO date string). No `labels` array field present. `priority: P1` uses non-standard enumeration. `status: Planned` not in {draft, active, implemented, deprecated}.

D6. spec.md:L127 vs `internal/llm/provider/glm/thinking.go:L14-L19` — Severity: **major**
REQ-ADP2-007 explicitly enumerates thinking-capable models as `glm-4.6, glm-4.7, glm-5`. Implementation expands this to also include `glm-4.5`. Either SPEC is wrong (should include glm-4.5) or implementation is wrong. A reasonable engineer cannot determine intent. Implementation drift from SPEC.

### Major (Test Coverage Gaps vs Acceptance Criteria)

D7. `internal/llm/provider/groq/client_test.go` (whole file) — Severity: **major**
AC-ADP2-004 (spec.md:L184-L187) explicitly requires: "stub이 `x-ratelimit-remaining-requests: 29` 헤더 반환" AND "`Tracker.Parse("groq", headers, now)` 호출" verification. Groq test (TestGroq_UsesCustomBaseURL) does neither — no stub headers set, no Tracker.Parse assertion. AC-ADP2-004 marked GREEN in progress.md:L84 without evidence.

D8. `internal/llm/provider/mistral/client_test.go` (whole file) — Severity: **major**
AC-ADP2-009 (spec.md:L209-L212) requires: "`req.ResponseFormat='json'`" + "HTTP body에 `\"response_format\": {\"type\": \"json_object\"}` 포함" verification. Test TestMistral_UsesCustomBaseURL does neither — no ResponseFormat set, no body inspection. REQ-ADP2-019 behavior untested.

### Minor

D9. `internal/llm/router/registry.go:L248` — Severity: **minor**
SPEC §6.3 example (spec.md:L463-L467) lists 8 Groq suggested_models; implementation has 3. Not a hard requirement (§10 refers to "research.md §2 참조하여 최신화"), but inconsistent with SPEC guidance.

D10. progress.md:L95 — Severity: **minor**
AC-ADP2-015 (Vision rejection) marked "N/A (인프라)". SPEC lists it as required AC. While the shared `llm_call.go:L57` implements the check (satisfying behavior), there is no per-adapter test covering AC-ADP2-015 for Groq specifically as SPEC's AC-ADP2-015 Given clause specifies.

D11. spec.md:L34 vs L20 — Severity: **minor**
HISTORY mentions "REQ-ADP2-022" but the Z.ai endpoint rationale text at L46 (§2.1) says "본 SPEC에서 교체 필수" — REQ-ADP2-022 in §4.5 is actually the Kimi long-context advisory, not the GLM endpoint requirement. Also, adapter.go:L19 references "REQ-ADP2-022" for the Z.ai endpoint, whereas the SPEC's REQ-ADP2-022 governs Kimi. Cross-reference mislabeling.

### No-defect (verified absent)

- REQ-ADP2-007 event injection (AC-ADP2-002) correctly implemented (adapter.go:L96-L116, thinking.go:L28-L45, test adapter_test.go:L91-L128).
- REQ-ADP2-014 graceful degradation correctly implemented (thinking.go:L33-L38, test adapter_test.go:L132-L167).
- REQ-ADP2-008 OpenRouter header injection correctly implemented + tested (openrouter/client.go:L52-L62, client_test.go:L84-L155).
- REQ-ADP2-011/012 region resolution correctly implemented + tested (qwen/client.go:L97-L113, kimi/client.go:L97-L113).
- REQ-ADP2-018 ErrInvalidRegion correctly implemented + tested (qwen/client_test.go, kimi/client_test.go:L92-L104).
- REQ-ADP2-001 openai embedding pattern uniformly applied across 9 adapters — zero HTTP logic duplicated; all use `openai.New(openai.OpenAIOptions{...})`.
- REQ-ADP2-016 duplicate rejection correctly implemented (registry.go:L33-L36, registry_builder_test.go:L61-L75).
- REQ-ADP2-017 no new external SDK — go.mod inspection confirms only `sashabaranov/go-openai`, `net/http`, `zap` used in new code.
- `go test ./internal/llm/provider/... ./internal/llm/factory/... ./internal/llm/router/...` all PASS.

## Chain-of-Verification Pass

Second-look findings:
- Re-read all 9 adapter client.go files — all use identical `openai.New(openai.OpenAIOptions{...})` pattern with BaseURL override. No HTTP duplication. PASS on REQ-ADP2-001.
- Re-verified REQ numbers end-to-end via grep: REQ-ADP2-001..022 complete, AC-ADP2-001..018 complete, no gaps, no duplicates.
- Re-verified traceability: every REQ has AC mapping; every AC cites REQ. Traceability 1.0 confirmed.
- Re-verified Exclusions (spec.md:L674-L689) — 14 specific entries, not vague.
- Re-examined GLM thinking.go model list vs SPEC — confirmed glm-4.5 extension is NOT SPEC-sanctioned (D6).
- Re-examined Kimi: confirmed long-context INFO log absent (D3).
- Re-examined OpenRouter: confirmed PreferredProviders absent (D4).
- Re-examined RegisterAllProviders 13 vs 15 discrepancy — verified against registry_builder.go:L49-L155 and registry_builder_test.go:L28 — unambiguous SPEC violation (D1).
- Re-examined contradictions: spec.md:L31-L32 "15 provider 전부" directly contradicts impl `Len == 13`. Test file reinforces the 13-count (test owned by same commit as SPEC) — no evidence SPEC was amended.

No additional defects discovered beyond those enumerated above. First-pass was comprehensive.

## Implementation Conformance Rate

| SPEC REQ | Implemented? | Evidence |
|----------|--------------|----------|
| REQ-ADP2-001 (reuse openai.NewWithBase) | YES | 9/9 adapters |
| REQ-ADP2-002 (Name/Capabilities) | YES | all adapters |
| REQ-ADP2-003 (ratelimit header) | YES (inherited) | openai embedding |
| REQ-ADP2-004 (non-PII logging) | YES (inherited) | — |
| REQ-ADP2-005 (15 AdapterReady) | PARTIAL | router meta: YES (15); RegisterAllProviders: NO (13) |
| REQ-ADP2-006 (error on invalid inputs) | YES | all factories return error |
| REQ-ADP2-007 (GLM thinking inject) | YES | adapter.go, thinking.go |
| REQ-ADP2-008 (OpenRouter ranking headers) | YES | openrouter/client.go |
| REQ-ADP2-009 (429 retry) | YES (inherited) | openai |
| REQ-ADP2-010 (RegisterAllProviders) | PARTIAL | registers 13 not 15 |
| REQ-ADP2-011 (Qwen region) | YES | qwen/client.go |
| REQ-ADP2-012 (Kimi region) | YES | kimi/client.go |
| REQ-ADP2-013 (Vision reject) | YES (inherited) | llm_call.go:57 |
| REQ-ADP2-014 (GLM graceful degradation) | YES | thinking.go |
| REQ-ADP2-015 (no live API tests) | YES | all httptest stubs |
| REQ-ADP2-016 (duplicate rejection) | YES | registry.go |
| REQ-ADP2-017 (no new external deps) | YES | verified |
| REQ-ADP2-018 (invalid region error) | YES | qwen, kimi |
| REQ-ADP2-019 (JSON response_format) | YES (inherited) | openai ExtraRequestFields |
| REQ-ADP2-020 (OpenRouter PreferredProviders) | **NO** | missing |
| REQ-ADP2-021 (GLM BudgetTokens) | **NO** | BuildThinkingField ignores field |
| REQ-ADP2-022 (Kimi long-context advisory) | **NO** | missing |

Conformance: **18/22 fully implemented**, **2/22 partial** (REQ-005, REQ-010), **3/22 not implemented** (REQ-020, REQ-021, REQ-022). Fully-conformant rate: **81.8%**; counting partials as half: **86.4%**.

## Regression Check (Iteration 2+ only)

N/A (iteration 1).

## Recommendation

Verdict: **FAIL** — four critical/major gaps between SPEC contract and implementation that the author self-acknowledged (progress.md:L93,L95) without SPEC amendment.

Required fixes before PASS:

1. **Resolve the 15-vs-13 discrepancy (D1)**: Either
   (a) extend `internal/llm/factory/registry_builder.go` to register `anthropic` and `google` factories (requires anthropic/google credential wiring), and update `registry_builder_test.go:L28` to expect 15; OR
   (b) amend spec.md:L31-L32, REQ-ADP2-005, AC-ADP2-016, AC-ADP2-017, and progress.md:L68 to state "13 providers fully wired via RegisterAllProviders; anthropic/google registered separately due to credential isolation".
2. **Implement REQ-ADP2-021 (D2)**: Extend `glm/thinking.go` BuildThinkingField to consume `cfg.BudgetTokens` and emit `budget_tokens: N` when > 0. Add test in adapter_test.go covering BudgetTokens=4096.
3. **Implement REQ-ADP2-022 / AC-ADP2-013 (D3)**: Add token-count estimation helper in kimi/client.go (or kimi/advisory.go), log INFO when `moonshot-v1-128k` model + estimated input tokens > 64K. Add test case. Alternatively amend SPEC Exclusions to remove this AC with explicit rationale.
4. **Implement REQ-ADP2-020 (D4)**: Add `PreferredProviders []string` to `openrouter.Options`, inject `"provider": {"order": [...], "allow_fallbacks": true}` via ExtraRequestFields. Add test.
5. **Fix YAML frontmatter (D5)**: Rename `created` → `created_at`, add `labels: [llm, adapter, provider]`, change `priority: P1` → `priority: high`, change `status: Planned` → `status: implemented` (given progress.md shows Completed).
6. **Resolve glm-4.5 thinking ambiguity (D6)**: Either add glm-4.5 to REQ-ADP2-007's model list in spec.md:L127 with HISTORY entry, or remove glm-4.5 from `thinking.go:L18`.
7. **Strengthen Groq and Mistral tests (D7, D8)**: Add header-assertion and body-assertion test cases matching AC-ADP2-004 and AC-ADP2-009 wording.
8. **Fix REQ cross-reference in adapter.go:L19 and spec.md:L34** (D11): The Z.ai endpoint ownership belongs to REQ-ADP2-005/§3.1.10, not REQ-ADP2-022.

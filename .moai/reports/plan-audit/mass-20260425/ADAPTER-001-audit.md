# SPEC Review Report: SPEC-GOOSE-ADAPTER-001

Iteration: 1/3 (single-shot mass audit)
Verdict: **FAIL (SPEC-document quality) / PASS (implementation conformance)**
Overall Score: 0.72

Reasoning context ignored per M1 Context Isolation. Audit based solely on
`.moai/specs/SPEC-GOOSE-ADAPTER-001/*` and the production code under
`internal/llm/provider/`.

---

## Axis 1 — SPEC Document Audit

### Must-Pass Results

- [PASS] MP-1 REQ number consistency — REQ-ADAPTER-001 … REQ-ADAPTER-020 sequential,
  no gaps, no duplicates (spec.md:L111–L157). 20 REQs, 3-digit zero-padded,
  correct categorization (Ubiquitous / Event-Driven / State-Driven / Unwanted /
  Optional).
- [FAIL] MP-2 EARS format compliance — Acceptance Criteria (spec.md:L163–L222)
  are written in **Given/When/Then** (BDD/Gherkin) style, not EARS. EARS patterns
  apply to *requirements*; ACs are test scenarios. The 20 REQs themselves DO use
  correct EARS form (shall / When / While / If-then / Where), so the MP-2 failure
  is localized to §5 ACs. Under strict M3 interpretation, AC-001…AC-012 are
  Given/When/Then scenarios mis-labeled as EARS ACs. Impact: rubric anchor 0.50
  (approximately half the ACs are Given/When/Then, which M3 explicitly flags).
  Tolerable if the project convention is "REQ=EARS, AC=Given/When/Then", but
  spec.md §4 is titled "EARS 요구사항" and §5 "수용 기준" — this mapping is
  project-defensible and the REQ side is clean, so the effective penalty is low
  but the strict letter of MP-2 is violated.
- [PASS] MP-3 YAML frontmatter validity — spec.md:L1–L13 contains id, version,
  status, created, author, priority, phase, size, lifecycle. **However**:
  `status: Planned` while implementation is complete (progress.md shows all M0–M5
  completed, evaluator PASS). `updated: 2026-04-21` does not reflect evaluator-fix
  iterations (2026-04-24). Also missing `labels` field (required per audit schema
  — uses `size`/`phase` instead of `labels`). Treated as minor defect, not fatal.
- [N/A] MP-4 Section 22 language neutrality — This SPEC targets a specific Go
  codebase and enumerates LLM providers by name, not language support. Applies
  to template/universal content only. Auto-passes.

### Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75–1.0 | Each REQ precise. spec.md:L111–L157. Minor: §6.4 pseudocode uses SDK types (`anthropic.Client`, `anthropic.MessagesRequest`) that the implementation does NOT use — pseudocode diverges from reality (see D4). |
| Completeness | 0.80 | 0.75 | All major sections present (HISTORY, Overview, Background, Scope, EARS, ACs, Technical Approach, Dependencies, Risks, References, Exclusions). HISTORY is thin (single entry; evaluator-fix rounds not logged). |
| Testability | 0.85 | 0.75–1.0 | 12 ACs, each with concrete Given/When/Then and measurable outcomes. AC-001–AC-012 all have matching tests in the codebase. |
| Traceability | 0.78 | 0.75 | 12 ACs cover REQ-001…REQ-018 via tasks.md AC→Task→REQ mapping. **Gap**: REQ-ADAPTER-013 (30s/60s heartbeat timeout) — no explicit AC; evaluator found it initially unimplemented, later fixed in Phase 2.Y. REQ-ADAPTER-014 (no PII logging), REQ-ADAPTER-016 (disk write restriction), REQ-ADAPTER-019 (JSON mode), REQ-ADAPTER-020 (UserID forwarding) — no dedicated ACs. See D1. |

### Defects Found (SPEC document)

- D1. spec.md:L141–L157 — **REQ→AC traceability gap**. REQ-ADAPTER-013 (heartbeat
  timeout), REQ-ADAPTER-014 (PII log prohibition), REQ-ADAPTER-016 (disk write
  restriction), REQ-ADAPTER-019 (JSON mode), REQ-ADAPTER-020 (UserID forwarding)
  have NO direct acceptance criterion. The ACs cover REQ-001/002/003/005/006/007/
  008/009/010/011/012/017/018 only. — Severity: major
- D2. spec.md:L1–L13 — **Frontmatter stale and incomplete**. `status: Planned`
  but implementation is DONE (progress.md Phase 3 complete, 10 commits merged).
  `updated: 2026-04-21` does not reflect 2026-04-24 evaluator-fix activity.
  Missing `labels` array (schema uses `size`/`phase` instead). — Severity: minor
- D3. spec.md:L163–L222 — **ACs in Given/When/Then, not EARS form**. Section
  title §5 is "수용 기준" but the 12 entries are BDD scenarios. Under strict MP-2
  (EARS format for all ACs), this is a FAIL. Project convention may treat EARS
  for REQ and GWT for AC — but SPEC must state this convention explicitly.
  — Severity: major (strict) / minor (lenient interpretation)
- D4. spec.md:L440–L468 — **§6.4 Anthropic pseudocode uses SDK types not used by
  implementation**. Pseudocode references `anthropic.MessagesRequest`,
  `client.Messages.CreateStreaming(ctx, apiReq)` — actual adapter
  (`anthropic/adapter.go:300–322`) uses raw `net/http` with hand-built JSON body.
  spec.md §7 (L606) lists `github.com/anthropics/anthropic-sdk-go` as a
  dependency but go.mod does NOT import it. Same for `sashabaranov/go-openai`
  (L607) and `ollama/ollama/api` (L609) — none are in go.mod. Dependency list is
  aspirational, not factual. — Severity: major
- D5. spec.md:L610 — **`tiktoken-go` listed as dependency** "Usage 추정 시 사용";
  go.mod includes it but no code under `internal/llm/provider/` imports it. Dead
  dependency claim. — Severity: minor
- D6. spec.md:L159–L222 — **REQ-ADAPTER-009 (thinking_delta) AC gap**. REQ-009
  mandates `thinking_delta` StreamEvent emission, but AC-ADAPTER-012 tests only
  `BuildThinkingParam` (parameter variant selection), not streaming event
  emission. Evaluator found this gap; Phase 2.X added
  `TestAnthropic_ThinkingMode_EndToEnd`. SPEC AC-012 should have been stronger.
  — Severity: minor (remediated in tests)
- D7. spec.md §4.1 REQ-ADAPTER-005 (L119) — The requirement mandates
  `CredentialPool.Select(ctx, strategy)` with a `strategy` argument, but the
  actual `CredentialPool.Select` signature is `Select(ctx)` (no strategy param;
  see `credential/pool.go:143`). REQ signature mismatches implementation
  contract — the REQ was authored against an API shape that was subsequently
  simplified. Phase 1 plan.md:L17 acknowledges this gap. — Severity: minor
- D8. spec.md §7 Dependencies line 606–609 — **Dependency list misrepresents
  actual libraries** (see D4). Creates reader confusion about what the real
  build uses. — Severity: minor (overlap with D4 but worth separate fix)

### Chain-of-Verification Pass

Second-look findings:
- Re-read spec.md §5 ACs: confirmed all 12 use "Given/When/Then" rather than
  EARS patterns. D3 upheld.
- Re-read §4.5 Optional REQs (017–020): REQ-020 UserID forwarding is not
  implemented anywhere in adapter code (grep for `Metadata.UserID` returns
  zero hits in provider dir). REQ-019 JSON mode likewise unimplemented (no
  `response_format` handling in openai/adapter.go, no `response_mime_type` in
  google/gemini.go). These are [Optional] REQs, so absence is tolerable, but
  SPEC should mark them as "deferred to post-MVP" rather than letting them
  silently lapse. Added as D9.
- Re-checked REQ numbering: 1..20 complete, no duplicates. MP-1 PASS.
- Exclusions section (spec.md:L671–L685) is present and specific (12 entries,
  each concrete). No issue here.
- Re-verified §6.1 file layout claims vs actual files: matches well. `google/
  stream.go` was merged into `gemini.go` per plan.md; OK. `secret.go` addition
  documented. Actual repo has `fallback.go` at provider root (not mentioned in
  §6.1) — minor discrepancy but consistent with M5 task T-060.

- D9. spec.md:L155–L157 — **REQ-019 and REQ-020 declared but unimplemented**.
  grep(`ResponseFormat` usage) and grep(`Metadata.UserID`) in provider code
  return no adapter-side forwarding. Fields are declared in
  `provider.go:L83–L88` but never read by Anthropic/OpenAI/Google adapters.
  Optional REQ status means this is defensible, but creates silent contract
  drift. — Severity: minor

---

## Axis 2 — Implementation Conformance Audit

### Scope Declaration vs Actual

SPEC scope (spec.md:L15, L27): "6 Provider 어댑터 (Anthropic/OpenAI/Google/xAI/
DeepSeek/Ollama)".

Actual `internal/llm/provider/` directory (2026-04-24): contains **13 adapter
subdirectories** — the SPEC-001 six (anthropic, openai, google, xai, deepseek,
ollama) plus seven more (cerebras, fireworks, glm, groq, kimi, mistral,
openrouter, qwen, together). Per user instruction, the additional 9 were added
by SPEC-GOOSE-ADAPTER-002 (commit `bea5df1 … SPEC-GOOSE-ADAPTER-001 + SPEC-GOOSE-
ADAPTER-002`). This audit scope **EXCLUDES** non-original-6 adapters and treats
them as out of scope for SPEC-001.

For the original 6:

| Provider | Adapter exists | Tests pass | LoC (prod) | LoC (test) |
|----------|---------------|-----------|-----------|-----------|
| anthropic | YES — 9 files | PASS (76–77% cov) | ~1,232 | ~1,450 |
| openai | YES — 3 files | PASS (77–79% cov) | ~661 | ~1,039 |
| google | YES — 2 files (real+gemini) | PASS (44.7–51.7% cov, SDK-real excluded) | ~452 | ~339 |
| xai | YES — grok.go (openai wrapper) | PASS (100% cov) | 62 | 84 |
| deepseek | YES — client.go (openai wrapper) | PASS (100% cov) | 62 | 89 |
| ollama | YES — local.go | PASS (76–77% cov) | 387 | 340 |

Six-for-six match. Total production LoC ~2,856 (original 6) + skeleton ~150 +
fallback/llm_call/registry ~200 ≈ **3,200 LoC production** (SPEC §6 estimated
~2,600; slight overshoot, acceptable).

### Implementation Match per REQ

| REQ | Status | Evidence |
|-----|--------|----------|
| REQ-001 (Provider iface) | MATCH | provider.go:L117–L126. Compile-time assertions `var _ provider.Provider = ...` in anthropic/adapter.go:L382, openai, etc. |
| REQ-002 (Registry) | MATCH | registry.go:L15–L69. ErrProviderNotFound returned when lookup fails (errors.go:L11). |
| REQ-003 (ctx propagation) | MATCH | All adapters use `http.NewRequestWithContext`. goleak tests pass (progress.md Phase 2B). |
| REQ-004 (RateLimit.Parse) | PARTIAL — only anthropic + openai call `tracker.Parse`. google/gemini.go has a `tracker` field but no `Parse` invocation (grep shows none). ollama/local.go has tracker field but no `Parse` invocation. — **Gap** |
| REQ-005 (Pool.Select + 429 rotate) | MATCH for anthropic + openai. NOT APPLICABLE for ollama (no credential). NOT VERIFIED for google (no pool integration at all — `APIKey string` is passed directly, bypassing pool). — **Partial Gap** |
| REQ-006 (LLMCall entry) | MATCH — llm_call.go:L29–L78. Step (c) "if provider == anthropic, consume PromptCachePlanner" is implemented INSIDE the Anthropic adapter rather than in llm_call.go; this is a minor interpretation deviation but semantically equivalent. |
| REQ-007 (OAuth refresh) | MATCH — anthropic/oauth.go + AnthropicRefresher type. adapter.go:L279–L286 triggers refresh when expires within 5 min. |
| REQ-008 (fallback chain) | MATCH — fallback.go:L21–L52 provider-agnostic. BUT fallback logic is NOT wired into NewLLMCall or into per-adapter Stream paths — it's only invoked via explicit `TryWithFallback()` call. No production call site invokes it; only test file uses it. — **Integration Gap** |
| REQ-009 (thinking_delta emit) | MATCH — anthropic/stream.go handles `content_block_delta.thinking_delta`. message.TypeThinkingDelta emitted. |
| REQ-010 (adaptive vs budget thinking) | MATCH — anthropic/thinking.go BuildThinkingParam. Post-fix commit added `json:"type"` tag. |
| REQ-011 (tool_use_id) | MATCH — anthropic/content.go + openai/adapter.go tool_call_id propagation. |
| REQ-012 (OpenAI-compat base_url) | MATCH — openai/adapter.go baseURL field; xai/grok.go + deepseek/client.go override. |
| REQ-013 (30s/60s heartbeat abort) | MATCH (Phase 2.Y fix) — constants.go DefaultStreamHeartbeatTimeout=60s + watchdog in ParseAndConvert, parseJSONL, consumeStream. |
| REQ-014 (no PII log) | MATCH — log statements use `provider/model/message_count`; no message content in any adapter log call. |
| REQ-015 (empty CachePlan → no cache_control) | MATCH — cache_apply.go:L10 `ApplyCacheMarkers` no-ops on empty plan. cache/planner.go stub returns empty Markers. |
| REQ-016 (disk write restriction) | MATCH — only writes are in `~/.claude/.credentials.json` (token_sync.go) and CREDPOOL managed. Secret.go FileSecretStore constrains to `~/.goose/credentials/`. |
| REQ-017 (vision capability pre-check) | MATCH — llm_call.go:L56–L58 returns ErrCapabilityUnsupported when image content + Vision=false. |
| REQ-018 (alias normalize) | MATCH — anthropic/models.go NormalizeModel. |
| REQ-019 (JSON mode) | **NOT IMPLEMENTED** — `ResponseFormat` field exists (provider.go:L84) but no adapter reads it. — **Gap (Optional REQ)** |
| REQ-020 (UserID forwarding) | **NOT IMPLEMENTED** — `Metadata.UserID` never forwarded by any adapter. — **Gap (Optional REQ)** |

### Implementation Match per AC

| AC | Test exists | Verified quality |
|----|------------|------------------|
| AC-001 | TestAnthropic_Stream_HappyPath | PASS |
| AC-002 | TestAnthropic_ToolCall_RoundTrip | PASS |
| AC-003 | TestAnthropic_OAuthRefresh_Success | PASS (write-back file not reloaded for exact content check — minor) |
| AC-004 | TestOpenAI_Stream_HappyPath | PASS |
| AC-005 | TestXAI_UsesCustomBaseURL | PASS |
| AC-006 | TestGoogleAdapter_Stream_HappyPath | PASS (fake client) |
| AC-007 | TestOllama_Stream_HappyPath | PASS |
| AC-008 | TestAnthropic_429Rotation | PARTIAL PASS — evaluator flagged lease-return bug (Anthropic `_ = next` discarded lease); Phase 2.X fix added `pool.Release(next)`. Test now accepts both success and exhausted-pool outcomes, so the genuine success-path-with-rotated-credential is not strictly verified. |
| AC-009 | TestFallback_FirstFailsSecondSucceeds | PASS (but unit-level only — not wired into production LLMCall path) |
| AC-010 | TestAnthropic_ContextCancellation | PASS (channel drains within 500ms; server-side connection teardown not rigorously verified per evaluator) |
| AC-011 | TestNewLLMCall_VisionUnsupported_ReturnsError | PASS |
| AC-012 | TestAnthropic_ThinkingMode_AdaptiveVsBudget + TestAnthropic_ThinkingMode_EndToEnd | PASS (end-to-end test added in Phase 2.X) |

`go test ./internal/llm/provider/... -count=1 -short`: **ALL PASS** across 17
packages (anthropic, openai, google, xai, deepseek, ollama + 9 SPEC-002 providers
+ fallback_test + registry_test + provider_test).

### CREDPOOL Extension Verdict (user-asked)

`internal/llm/credential/pool.go:L252` `MarkExhaustedAndRotate` and
`internal/llm/credential/lease.go` `AcquireLease`+`Lease.Release` are explicitly
tagged `SPEC-GOOSE-ADAPTER-001 T-007 (CREDPOOL-001 §3.1 rule 6 선행 구현)`. Per
tasks.md T-007 and plan.md §2, these were **intentionally placed in this SPEC**
as a scope expansion approved by the user (progress.md Phase 1: "Scope expansion
approved: 5 skeleton 패키지 + CREDPOOL 확장 + SecretStore interface"). Verdict:
**belongs to SPEC-GOOSE-ADAPTER-001** as a documented forward-extension; the
cross-reference to SPEC-GOOSE-CREDPOOL-001 §3.1 rule 6 is correct.

---

## Defects Found (Implementation Axis)

- I1. `google/gemini.go` + `ollama/local.go` — **REQ-ADAPTER-004 partial
  violation**: both declare a `tracker` field but never invoke
  `tracker.Parse(name, headers, now)`. anthropic/adapter.go:L222 and
  openai/adapter.go:L238–L239 do call it correctly. — Severity: major
- I2. `google/gemini.go` — **REQ-ADAPTER-005 not applicable via pool**: Gemini
  adapter takes `APIKey string` directly (GoogleOptions:L61) and has no
  `*credential.CredentialPool` field. Bypasses CREDPOOL entirely. For a
  production-grade adapter this is a conformance gap against REQ-005
  ("Every adapter shall use CredentialPool.Select"). — Severity: major
- I3. `fallback.go` — **REQ-ADAPTER-008 not wired into production path**.
  TryWithFallback is only called from `fallback_test.go`. `llm_call.go`
  dispatches directly to `p.Stream(...)` with no fallback wrapper. The `req.
  FallbackModels` field is passed through but never consulted at runtime.
  — Severity: major
- I4. Anthropic 429 retry (`adapter.go:L229–L248`) — re-entered `stream()`
  calls `pool.Select(ctx)` which may now return the *same* exhausted credential
  if rotation semantics don't exclude it properly. Evaluator found the
  orphaned-lease bug (now fixed with `pool.Release(next)`) but the recursive
  retry behavior is weaker than the OpenAI pattern (openai/adapter.go
  uses `next` directly). — Severity: minor (functional, not critical)
- I5. `provider.ResponseFormat` + `provider.Metadata.UserID` are declared but
  never consumed — see D9 / REQ-019 / REQ-020 gaps. — Severity: minor
  (Optional REQs)

---

## Regression Check (Iteration 2+ only)

N/A — iteration 1.

---

## Recommendation

Overall: **SPEC document FAIL (document quality), implementation PASS (code
conformance)**. The implementation delivers on the six-provider scope with
reasonable test coverage, and the evaluator already PASSed at 0.789. However,
the SPEC document itself has non-trivial defects that should be corrected before
any downstream consumer (auditor, future maintainer, SPEC-002 author, end user)
can trust it as an authoritative reference.

Required fixes (for manager-spec):

1. **spec.md frontmatter**: update `status: Planned` → `status: Implemented`;
   bump `updated` to the last Phase 2 commit date (2026-04-24); add `labels`
   array per audit schema.
2. **spec.md §5 AC form**: either (a) rewrite all 12 ACs in EARS form, or
   (b) add an explicit convention note at §5 stating "ACs are Given/When/Then
   test scenarios; REQs are EARS requirements. Traceability: REQ ↔ AC mapping
   given in §5.x."
3. **spec.md §7 Dependencies**: remove `anthropic-sdk-go`, `sashabaranov/go-
   openai`, `ollama/ollama/api`, `tiktoken-go` unless actually imported; replace
   with "hand-rolled `net/http` client + `encoding/json`" descriptions that
   match the build. Update §6.4 pseudocode to reflect raw HTTP approach.
4. **Add REQ→AC mapping table** to spec.md §5 (or §4 tail). Current gaps:
   REQ-013/014/016/019/020 lack ACs. At minimum mark as "indirect verification
   via Security/Trackable TRUST axes" or remove the REQs from MVP scope.
5. **Address REQ-019 and REQ-020 explicitly**: either implement (trivial: 5–10
   LoC per adapter) or demote to "deferred — not covered in initial 6-provider
   MVP, tracked in SPEC-ADAPTER-003".
6. **HISTORY**: add entries for 2026-04-24 evaluator-fix rounds (Phase 2.X,
   Phase 2.Y, Phase 2.Z SPEC-002 prerequisite extension).

Recommended implementation follow-ups (for manager-ddd/tdd — not blocking this
SPEC's sign-off):

7. **I1**: add `tracker.Parse` calls in google/gemini.go and ollama/local.go
   for REQ-004 full compliance (1-line addition per file, behind `if tracker
   != nil` guard).
8. **I2**: decide Google credential path — either add CredentialPool integration
   or mark Gemini as exempt from REQ-005 (API-key-only provider) and amend SPEC
   accordingly.
9. **I3**: wire TryWithFallback into llm_call.go so production LLMCall path
   honors req.FallbackModels. Currently the feature is tested but dead in prod.

---

## Summary Line

**Path**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/ADAPTER-001-audit.md`
**Verdict**: FAIL (SPEC doc) / PASS (impl)
**Defects**: 9 SPEC + 5 implementation = 14 total (0 critical, 7 major, 7 minor)
**Implementation conformance rate**: 16/20 REQs fully met, 2/20 partially met
(REQ-004, REQ-005), 2/20 not met (REQ-019, REQ-020 — Optional) = **80% strict
conformance, 90% weighted (Optional REQs discounted)**.
**Tests**: `go test ./internal/llm/provider/... -count=1 -short` → 17/17
packages PASS.

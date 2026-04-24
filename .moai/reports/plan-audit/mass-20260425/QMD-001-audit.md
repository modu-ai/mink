# SPEC Review Report: SPEC-GOOSE-QMD-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.68

Note: Reasoning context from the invocation prompt was ignored per M1 Context Isolation. Audit is based solely on `spec.md` at `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-QMD-001/spec.md`. No `research.md` present (confirmed via directory listing).

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-QMD-001 through REQ-QMD-017 are sequential with no gaps or duplicates. Distribution: Ubiquitous 001-004 (L110-116), Event-Driven 005-009 (L120-128), State-Driven 010-012 (L132-136), Unwanted 013-016 (L140-146), Optional 017 (L150). Three-digit zero-padding is consistent.

- **[PASS] MP-2 EARS format compliance**: 16 of 17 REQs match EARS patterns cleanly. Every REQ uses "shall/shall not" as the normative verb. Each REQ carries a bracketed EARS classifier ([Ubiquitous], [Event-Driven], [State-Driven], [Unwanted], [Optional]). Spot check:
  - Ubiquitous: L110 "The QMD subsystem shall be statically linked..." ✓
  - Event-Driven: L120 "When `goose qmd reindex [path]` is invoked, the system shall perform..." ✓
  - State-Driven: L132 "While a reindex operation is in progress, concurrent `qmd.Query` calls shall continue..." ✓
  - Unwanted (conditional): L140 "If a candidate path ... matches any pattern in ... `blocked_always` list, then the system shall not..." ✓
  - Optional: L150 "Where environment variable `QMD_MODEL_MIRROR` is defined, the download routine shall prefer..." ✓
  - Minor classification issue on REQ-QMD-016 (L146): labeled `[Unwanted]` but structurally written as a blanket negative Ubiquitous ("The QMD subsystem shall not expose MCP stdio server on any TCP/UDP port."). Still EARS-compatible; not a failure.

- **[FAIL] MP-3 YAML frontmatter validity**: The `labels` field is **absent** from the frontmatter (L2-L14). Rubric requires `labels` (array or string) as a required field. Additional deviations:
  - L5 uses `created` (not `created_at`); rubric requires `created_at` naming.
  - L4 `status: Planned` — not in the rubric's canonical set (`draft`, `active`, `implemented`, `deprecated`).
  - L8 `priority: P0` — not in the rubric's canonical set (`critical`, `high`, `medium`, `low`). Maps informally to `critical` but does not comply.
  - Missing required field `labels` = FAIL per MP-3.

- **[N/A] MP-4 Section 22 language neutrality**: SPEC is scoped to a single-binary component (`goosed`) using Go (1.26) + Rust (1.80) via CGO for one specific purpose (QMD hybrid search). It is not a multi-language LSP or template-bound universal concern. The enumerated build targets (macOS universal, Linux amd64/arm64) are platform targets, not the 16-language tooling surface. Auto-passes as N/A.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.60 | 0.50 band | Primary defect: term "QMD" is never expanded in the SPEC body (see D1). L29 introduces `tobi/qmd` and `qntx-labs/qmd` as project names but never defines the acronym. A reasonable engineer will not know from this document alone whether QMD denotes Quarto Markdown, Query-Markdown-Document, or simply a project codename. Secondary: Section 3.1 (IN SCOPE, L69) and Section 8 (Rollout, L468-L480) disagree on whether MCP stdio server and fsnotify watcher are M1-scoped or deferred to M3/M4 (see D2). |
| Completeness | 0.70 | 0.75 band (lower edge) | All required sections present (HISTORY L18, Overview L27, Background L39, Scope L74, EARS Requirements L104, Acceptance Criteria L154, Exclusions L607). Frontmatter is missing `labels` (see MP-3 above). Markdown parser selection (goldmark? tantivy's built-in? Rust crate's own?) and chunking algorithm provenance are not specified (see D3). SHA256 checksums for the two GGUF models are placeholder `<pinned-hash>` strings (L410, L416) — the SPEC cannot actually verify REQ-QMD-009 until these are filled (see D4). Rust crate version is unpinned: L224 "pinned tag(첫 통합 시 결정)" — contradicts the risk-mitigation posture and the upgrade policy referenced at L588 (`lsp-client.md`) (see D5). |
| Testability | 0.75 | 0.75 band | Most ACs are binary-testable: AC-QMD-001 (`otool -L` / `ldd` output, L160-L161), AC-QMD-003 (p50/p99 numeric thresholds, L170), AC-QMD-004 (<100ms, L176), AC-QMD-007 (50 concurrent requests, L188-L191), AC-QMD-010 (RSS <500MB, L203-L206), AC-QMD-011 (no `~/.ssh/**` files indexed + audit log entry, L208-L211), AC-QMD-012 (`ErrQMDDisabled` return, L213-L216). Weasel words largely absent. Weaknesses: AC-QMD-002 (L163-L166) stipulates "M-series Mac(M2 Pro **이상**)" which is testable on a single CI box but is not reproducible across varying M2/M3/M4 SKUs without further constraint. AC-QMD-005 (L178-L181) relies on an implicitly defined "trace 플래그 출력" whose schema is not documented in §7.6 alongside the MCP methods. |
| Traceability | 0.45 | 0.50 band (lower edge) | No AC explicitly cites a REQ-QMD-NNN identifier. AC-QMD-001 through AC-QMD-012 (L158-L216) rely on implicit topic matching. Multiple REQs are uncovered by any AC: REQ-QMD-002 (exactly four public functions, L112) — no AC verifies the four-function API surface; REQ-QMD-003 (persistence across restart, L114) — no AC exercises restart; REQ-QMD-004 (SQLite schema, L116) — no AC inspects `qmd_index_status` / `qmd_doc_tracking` rows; REQ-QMD-010 (reader-writer separation during reindex, L132) — no AC stresses concurrent query during reindex; REQ-QMD-011 (`ErrModelNotReady` during download, L134) — no AC covers this error path; REQ-QMD-016 (no TCP/UDP port exposure, L146) — no AC probes for open sockets; REQ-QMD-017 (`QMD_MODEL_MIRROR` preference, L150) — no AC exercises the env var. See D6. |

## Defects Found

D1. spec.md:L16, L29 — The acronym "QMD" is never expanded in the SPEC body. Title (L16) reads "SPEC-GOOSE-QMD-001 — QMD Embedded Hybrid Memory Search" and §2.1 (L41) headers "왜 QMD인가" but neither defines what QMD stands for. Section 12.2 (L592-L593) links to `qntx-labs/qmd` and `tobi/qmd` but those are project URLs, not an expansion. A downstream reader cannot distinguish whether QMD refers to Quarto Markdown (the RStudio .qmd format with executable code blocks), to a project codename, or to a novel acronym. This ambiguity has real scope implications: if QMD is Quarto-family, code-execution semantics become a security question; if it is merely tobi's project name, code execution is out of scope by construction. Neither answer is derivable from this SPEC alone. **Severity: major.**

D2. spec.md:L69 vs L474, L478-L479 — IN SCOPE list at §2.4 (L69) includes "fsnotify 기반 자동 재인덱스", "MCP stdio 서버", and "CLI 서브커맨드(`goose qmd ...`)" as part of M1 scope. But §8.1 "Phase 1 (M1)" (L468-L473) states "fsnotify watcher **비활성**" and omits MCP. §8.2 (M3) activates `qmd.Watch`. §8.3 (M4) opens `goose qmd mcp` externally. Readers cannot determine whether fsnotify and MCP are M1 deliverables or deferred. This also contaminates REQ-QMD-006 (fsnotify watcher) and REQ-QMD-008 (MCP server) — if they are Ubiquitous in M1 SPEC but disabled per the rollout plan, the requirements are effectively vacuous in M1. **Severity: major.**

D3. spec.md:L353-L357 (§7.5 chunking), L243-L272 (§7.1 package layout) — Markdown parsing responsibility is unspecified. §7.5 says "섹션 기반(H1/H2) 분할" and references "512 토큰 / 최소 64 토큰, overlap 64 토큰", but it does not say which markdown parser produces the AST: goldmark (Go stdlib of choice), blackfriday, or a Rust-side parser within the `qntx-labs/qmd` crate. Handling of front matter (YAML/TOML), fenced code blocks, tables, and nested lists is silent. §7.1's `internal/qmd/index/` has no `chunker.go` or equivalent. This leaves a correctness-relevant component undefined. **Severity: major.**

D4. spec.md:L410, L416 — The model manifest embeds placeholder `"sha256": "<pinned-hash>"` strings for both `bge-small-en-v1.5.gguf` and `bge-reranker-base.gguf`. REQ-QMD-009 (L128) mandates "verify SHA256 checksums **against pinned values**" and AC-QMD-008 (L193-L196) requires pinned-value comparison. The SPEC cannot verify AC-QMD-008 until actual SHA256 values are filled in. This is a SPEC-time gap, not an implementation gap. **Severity: major.**

D5. spec.md:L224 — The Rust crate `qntx-labs/qmd` is listed as "pinned tag(첫 통합 시 결정)" — i.e., not actually pinned yet. Risk R5 (L527) claims "pinned version + vendored source" as its mitigation, and §12.1 (L588) references `lsp-client.md` upgrade policy as a template. The SPEC's own risk mitigation is not satisfied by the SPEC itself. **Severity: minor.**

D6. spec.md:L158-L216 (all AC entries) — No acceptance criterion explicitly cites a REQ-QMD-NNN identifier. Mapping must be inferred by topic. Uncovered requirements: REQ-QMD-002 (four-function API surface), REQ-QMD-003 (restart persistence), REQ-QMD-004 (SQLite table schema), REQ-QMD-010 (reader-writer concurrency during reindex), REQ-QMD-011 (`ErrModelNotReady`), REQ-QMD-016 (no TCP/UDP port), REQ-QMD-017 (`QMD_MODEL_MIRROR` env var). Seven REQs out of seventeen have no binary-testable AC. This is the dominant traceability defect. **Severity: major.**

D7. spec.md:L2-L14 (YAML frontmatter) — `labels` field is absent (MP-3 FAIL root cause). Additionally: `created` should be `created_at` (rubric naming); `status: Planned` and `priority: P0` are outside the rubric's canonical value sets. **Severity: major** (because it is MP-3).

D8. spec.md:L607-L621 (Exclusions) — The Exclusions section does not explicitly state that **code execution of markdown document content is out of scope**. Given the D1 ambiguity about whether QMD is Quarto-family (which executes `.qmd` code blocks), an explicit exclusion would eliminate a nontrivial security question. Present exclusions cover Kuzu, PDF/DOCX, fine-tuning, remote APIs, Windows, encryption, TypeScript port, Web UI, TCP/UDP, index-format migration — but are silent on code execution. **Severity: minor** (safety-posture improvement).

D9. spec.md:L146 (REQ-QMD-016) — Labeled `[Unwanted]` but phrased as a blanket Ubiquitous negative ("The QMD subsystem shall not..."). The EARS Unwanted pattern is "If [undesired condition], then [response]"; a blanket prohibition without a triggering condition is structurally Ubiquitous. Classification-only issue; does not break MP-2 because language is still EARS-legal. **Severity: minor.**

D10. spec.md:L224 vs L588 — §6 Tech Stack lists `qntx-labs/qmd` without a version. §12.1 refers to `.claude/rules/moai/core/lsp-client.md` as the upstream version-pin policy template, but the SPEC itself does not carry an analogous "Upgrade Policy" subsection showing which integration tests gate a version bump. The reference-by-analogy is insufficient for a size-L / P0 SPEC. **Severity: minor.**

## Chain-of-Verification Pass

Second-look findings:

- Re-read every REQ entry end-to-end (not spot-check): confirmed REQ numbering is clean, confirmed REQ-QMD-016 classification subtlety (D9). Confirmed that every REQ uses `shall` / `shall not` normatively.
- Re-read every AC entry end-to-end: confirmed zero ACs cite REQ identifiers (D6). Verified the uncovered REQ list.
- Re-checked the Exclusions section (§12.1 title says "참고"; actual Exclusions is at L607-L621): present, multiple specific entries, but missing code-execution exclusion (D8).
- Cross-checked Scope (§3.1 L76-L88) against Rollout (§8 L466-L500): uncovered inconsistency for fsnotify and MCP (D2).
- Cross-checked manifest SHA256 pinning (L410, L416) against REQ-QMD-009 (L128) and AC-QMD-008 (L193): confirmed placeholder mismatch (D4).
- Cross-checked Rust crate pinning (L224) against Risk R5 (L527): confirmed self-contradiction (D5).
- Searched the body for any expansion of "QMD" or "Q.M.D." or parenthetical gloss: none found (D1).

No first-pass defects were missed on this re-read. No defects listed above were refuted on re-read.

## Regression Check

Iteration 1 — not applicable.

## Recommendation

**FAIL.** The author (manager-spec) must revise spec.md and resubmit. Required fixes, in priority order:

1. **Fix MP-3 frontmatter** (spec.md:L2-L14): add `labels:` (string or array), rename `created` to `created_at`, normalize `status` to one of `draft|active|implemented|deprecated`, normalize `priority` to one of `critical|high|medium|low`.

2. **Resolve D1 (QMD acronym)**: expand "QMD" on first use in §1 Overview (L27). State explicitly whether QMD refers to Quarto Markdown (with its code-execution heritage), tobi's project codename, or a fresh acronym. One sentence is sufficient.

3. **Resolve D2 (Scope vs Rollout contradiction)**: reconcile §3.1 (L76-L88) with §8 (L466-L500). Either (a) move fsnotify watcher and MCP server out of §3.1 IN SCOPE and mark them as deferred deliverables, or (b) declare that M1 includes fsnotify and MCP (contradicting §8.1).

4. **Resolve D3 (markdown parsing)**: add a §7.5a subsection naming the markdown parser (goldmark on the Go side, or the Rust crate's internal parser), and specify front-matter/code-block/table handling. Add a `chunker.go` entry under §7.1 `internal/qmd/index/` if Go-side.

5. **Resolve D4 (SHA256 placeholders)**: replace `<pinned-hash>` at L410 and L416 with actual SHA256 values from the pinned GGUF releases, or explicitly state "to be pinned in M1 PR #NNN" with an issue link.

6. **Resolve D5 and D10 (Rust crate version)**: replace "pinned tag(첫 통합 시 결정)" at L224 with a concrete tag (e.g., `v0.3.2`) and add a subsection modeled on `lsp-client.md`'s "Upgrade Policy" inside this SPEC.

7. **Resolve D6 (traceability)**: annotate each AC with the REQ(s) it covers (format: `AC-QMD-00X → REQ-QMD-00Y, REQ-QMD-00Z`). Add ACs for the seven uncovered REQs: REQ-QMD-002, REQ-QMD-003, REQ-QMD-004, REQ-QMD-010, REQ-QMD-011, REQ-QMD-016, REQ-QMD-017.

8. **Resolve D8 (code-execution exclusion)**: append one bullet to §Exclusions (L607-L621) stating "본 SPEC은 마크다운 문서 내 코드 블록을 실행하지 않는다. QMD는 인덱싱/임베딩/검색 전용이며, Quarto의 코드 실행 기능은 범위 밖이다."

9. **Resolve D9 (REQ-QMD-016 classification)**: either reclassify as `[Ubiquitous]` or rewrite conditionally as "If a caller attempts to bind the MCP server to a TCP/UDP port, then the system shall reject the bind and log..."

Upon resubmission, iteration 2 will verify resolution of D1-D10 and re-run all Must-Pass checks.

---

Report path: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/QMD-001-audit.md`

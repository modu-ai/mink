# SPEC-MINK-BRAND-RENAME-001 Plan Audit Report

**Auditor**: plan-auditor (independent, adversarial stance)
**Date**: 2026-05-12
**SPEC version audited**: v0.1.0 (research.md 544 lines, spec.md 981 lines)
**Verdict**: **CONDITIONAL_PASS**

> M1 Context Isolation: Reasoning context from the SPEC author (if any was passed in the prompt beyond the literal artifact paths) was ignored. Audit was performed only against `research.md`, `spec.md`, the predecessor SPEC frontmatter+HISTORY, and the live repo state at branch `plan/SPEC-MINK-BRAND-RENAME-001` (HEAD inherits main `e76febe`).
>
> Baseline ground-truth verification performed:
> - `git branch --show-current` → `plan/SPEC-MINK-BRAND-RENAME-001` ✓
> - `head -1 go.mod` → `module github.com/modu-ai/goose` ✓
> - `ls cmd/` → `goose goosed` (2 dirs, `goose-proxy` absent — matches research §2.5) ✓
> - `ls -d .moai/specs/SPEC-GOOSE-* | wc -l` → 88 ✓
> - `grep -rln 'github.com/modu-ai/goose' --include='*.go' | wc -l` → 456 ✓ (matches research §2.1)
> - `grep -rn 'github.com/modu-ai/goose' --include='*.go' | wc -l` → 958 ✓
> - `find proto -name '*.proto'` → 4 files in `proto/goose/v1/` ✓
> - `internal/transport/grpc/gen/goosev1/` directory present ✓
> - First .proto file header → `// Package goose.v1 ...` / `package goose.v1;` / `option go_package = "github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1;goosev1";` ✓
> - `grep -c '\.goose/' --include='*.go' -r .` → 118 (research says 117 — 1-line drift, immaterial)
> - `.moai/brain/` directory does **not** exist (research §10.3 / §3.2 item 7 correctly acknowledges this)
> - 4 flat-file SPECs present (`CMDCTX-DEPENDENCY-ANALYSIS.md`, `IMPLEMENTATION-ORDER.md`, `ROADMAP.md`, `SPEC-DOC-REVIEW-2026-04-21.md`) ✓
> - 3 non-`SPEC-GOOSE-*` SPEC dirs (`SPEC-AGENCY-ABSORB-001`, `SPEC-AGENCY-CLEANUP-002`, `SPEC-MINK-BRAND-RENAME-001`) ✓

---

## Dimension Scores

| Dimension | Score | Target | Status |
|-----------|-------|--------|--------|
| EARS Compliance | 0.82 | 0.85 | ✗ |
| AC Quality | 0.81 | 0.85 | ✗ |
| Phase Executability | 0.84 | 0.80 | ✓ |
| Risk Coverage | 0.78 | 0.80 | ✗ |

**Three of four dimensions miss target.** Defects are concrete and locally-fixable (no structural rewrite required), hence verdict is CONDITIONAL_PASS rather than REVISE. Critical findings are line-level inconsistencies, not architectural defects.

### Dimension 1 — EARS Compliance (0.82)

- Pattern bucketing is mostly correct (Ubiquitous 10, Event-Driven 5, State-Driven 4, Unwanted 5, Optional 3). All 27 REQs have legitimate EARS structure.
- **Deducted** for 3 orphan non-Optional REQs (REQ-005, REQ-011, REQ-023) with no AC mapping (-0.12 from baseline 0.95).
- **Deducted** for one Unwanted-REQ vs AC contradiction (REQ-020 vs AC-004): see C-1 (-0.03).
- IDs are unique and sequential (REQ-MINK-BR-001 through 027). No duplicates.

### Dimension 2 — AC Quality (0.81)

- All 17 ACs use Given/When/Then structure with concrete commands.
- Most ACs are binary-verifiable.
- **Deducted** for explicit escape hatch in AC-002 step 6 ("or skip with documented reason") — see H-3 (-0.04).
- **Deducted** for AC-015 step 1 vs scope contradiction — see C-2 (-0.06).
- **Deducted** for AC-016 step 4 verification gap (CLAUDE.local.md is gitignored, no CI verification path) — see M-2 (-0.03).
- **Deducted** for non-binary `≥` lower bounds with no upper bound (AC-010 step 2, AC-011 step 2) — see M-3 (-0.06).

### Dimension 3 — Phase Executability (0.84)

- 8 phases with explicit verification commands, rollback procedures, commit message templates that match CLAUDE.local.md §2.2 (English conventional type + Korean body + SPEC/REQ/AC trailers).
- Phase 2 atomicity is correctly enforced; Phase 4 atomicity is correctly enforced.
- Phase ordering is generally sound (1→2→{3,4}→5→6→7→8→post-merge).
- **Deducted** for sed command pattern issue / missed doc-comment refs — see H-4 (-0.06).
- **Deducted** for unjustified Phase 3 vs Phase 4 ordering — see L-1 (-0.02).
- **Deducted** for ambiguous "Phase 4.3 baseline" cross-reference in AC-013 — see L-2 (-0.02).
- **Deducted** for Phase 7 doc-sweep producing badge URLs that 404 between PR merge and post-merge GitHub rename — see H-2 (-0.06).

### Dimension 4 — Risk Coverage (0.78)

- 14 risks (R1–R14) enumerated with mitigations. Several research-confirmed risks (R3, R4, R5) cite the pre-rename evidence inline.
- **Deducted** for missing risk: brand-lint CI gate will block the orchestrator-emitted predecessor-supersede commit because §7.5 step 5 byte-level HISTORY-diff check + REQ-020 collide with AC-MINK-BR-004's explicit single-row-append (C-1) — *should* be in §9, isn't (-0.10).
- **Deducted** for understated risk on `GOOSE_*` env var + `./.goose/` path deferral — see H-1 (-0.05). The plan defers both to downstream SPECs, but the merged-state codebase will compile-call-itself "MINK" yet runtime-read `GOOSE_HOME` and write `./.goose/`. This is a user-facing inconsistency window between this PR's merge and the downstream SPECs' merges. R8/R9 mention this but do not size the inconsistency window or specify whether `mink --help` text must warn about it.
- **Deducted** for missing concrete check that `go mod tidy` produces no go.sum diff (R5 mentions "may reorder" — Phase 2 verification doesn't gate on go.sum diff) — see M-1 (-0.05).
- **Bonus credit** for explicitly modeling R10 (proto wire-format) and R7 (predecessor supersede commit timing).

---

## Findings (priority order)

### CRITICAL (verdict-driving)

#### C-1 — REQ-MINK-BR-020 + §7.5 step 5 will BLOCK the orchestrator-emitted predecessor-supersede commit (AC-MINK-BR-004)

**Where**: spec.md L179 (REQ-MINK-BR-020), L855 (§7.5 step 5), L246–256 (AC-MINK-BR-004 prescribes adding one HISTORY row to the predecessor).

REQ-MINK-BR-020 says: *"If any commit attempts to modify a row in any existing SPEC's `## HISTORY` table (status irrespective ...), then the brand-lint CI gate shall reject the change on immutable-history grounds."*

§7.5 step 5 (brand-lint algorithm) says: *"HISTORY 보존 검증: 모든 SPEC 의 `## HISTORY` 섹션 내부에서 brand 패턴 변경이 발생했는지 baseline 과 byte-level diff (REQ-MINK-BR-017 + REQ-MINK-BR-020). 변경 발견 시 즉시 fail."*

But AC-MINK-BR-004 explicitly requires appending one new HISTORY row to `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` post-merge. A byte-level diff against baseline will register the new row as a change and the CI gate will reject it.

The spec mitigates this implicitly with §6 Post-merge note "이 작업은 본 SPEC PR squash merge 이후 별도 orchestrator 단독 작업 (PR 외)" — but a "PR-외 작업" still produces a commit, and that commit *will* be pushed to main and *will* trigger PR-on-push CI on subsequent PRs that include main as base. Even if the supersede commit itself bypasses PR review (admin bypass), the very next PR opened against main will fail brand-lint against the new HISTORY baseline.

**Required correction**: One of —
- (a) Add explicit exemption in REQ-020 and §7.5 step 5 for APPEND-only modifications to HISTORY tables (e.g., adding rows beyond the previous last row is permitted; modifying existing rows is rejected); OR
- (b) Specify that the post-merge supersede commit is performed with `--admin` bypass AND the new HISTORY row is captured into a new baseline before the next PR opens (and document the baseline-refresh procedure); OR
- (c) Add the supersede commit *inside* the SPEC's own PR scope (which contradicts orchestrator-confirmed decision #2's wording "별도 후속 commit" but is the cleanest CI-wise).

Recommendation: option (a). REQ-020 wording should change "modify a row" → "modify an existing row (i.e., change to a row that existed at baseline)". §7.5 step 5 should add: "Appending new rows after the last existing row is permitted; modifications to row content at baseline indices are rejected."

**Severity**: CRITICAL — without correction, the plan as written has the SPEC's own follow-on workflow blocked by the SPEC's own CI gate.

#### C-2 — AC-MINK-BR-015 step 1 contradicts §1.3 Non-Goals item 10 and Phase 8 scope (user-data path migration is OUT but AC requires it IN)

**Where**:
- spec.md L60 (Non-Goal: "User-data 실제 마이그레이션 코드 (`./.goose/` → `./.mink/` 자동 마이그레이션 logic — 별도 SPEC `SPEC-MINK-USERDATA-MIGRATE-001`). 본 SPEC은 정책만 정의.")
- spec.md L400 (AC-015 step 1: "새 Go 코드 (`_test.go` 제외) 가 default user-data path 로 `./.mink/` 또는 `~/.mink/` 를 사용")
- spec.md §6 Phase 8 — task list does not include any path-default changes; only doc-comment / brand-token sed.

AC-015 step 1 is binary-testable but the SPEC's own Phase 8 does not produce the code change that would satisfy it. The work is silently passed to the downstream SPEC. Either:
- AC-015 step 1 is unsatisfiable within this SPEC's PR, OR
- Phase 8 must include the path-default change (which contradicts §1.3 and §11 item 10).

This is a logical contradiction, not just a wording issue. The two artifacts in the PR (Phase 8 commit + AC-015 verification) are incompatible.

**Required correction**: Either —
- (a) Reword AC-015 step 1 to "신규 작성된 Go 코드 중 default user-data path 가 신규 도입된 경우 `./.mink/` 또는 `~/.mink/` 를 사용한다 (본 SPEC 의 Phase 8 commit 범위 안에서 새로 추가되는 path 가 없으므로 vacuously true 가 허용된다)"; OR
- (b) Drop AC-015 entirely and replace it with an explicit non-verification statement that downstream SPEC owns this AC.

Recommendation: option (b) — drop AC-015 and add an explicit "Deferred to SPEC-MINK-USERDATA-MIGRATE-001" stub paragraph in §5 with a forward-reference. This removes the satisfiability hole.

**Severity**: CRITICAL — AC-015 will be marked "FAIL" or "untestable" at sync phase. As written the SPEC cannot achieve full AC coverage.

### HIGH (must-fix before merge)

#### H-1 — Brand/runtime split: `mink` binary will read `GOOSE_*` env vars and write `./.goose/` after PR merge

**Where**: spec.md L60 (Non-Goal user-data migration), §3.1 item 12 footnote alluding to "별도 SPEC 으로 분리 검토" (line 126), R8/R9 in §9 (line 906-907), AC-015 (line 397-406), AC-014.

After Phase 8 merges, the codebase says "MINK" everywhere in brand-position, the binary is called `mink`, but:
- `mink --help` output: brand says MINK ✓
- `mink` reads `GOOSE_HOME` env var (Phase 2-8 do not touch env var keys; SPEC-MINK-ENV-MIGRATE-001 is deferred)
- `mink` defaults to `./.goose/` workspace path (117 occurrences in .go untouched; SPEC-MINK-USERDATA-MIGRATE-001 deferred)
- A new user running `mink` on a fresh machine will look for `MINK_HOME` (per brand) → finds no such variable → falls back to default path → which is `./.goose/` (not `./.mink/`) → confusion.

R9 says "새 `MINK_*` env var 신설 + 옛 `GOOSE_*` deprecated alias loader. 본 SPEC 은 정책만 (§3.1 item 12 footnote, REQ-MINK-BR-027). 실제 loader 는 별도 SPEC `SPEC-MINK-ENV-MIGRATE-001`." But REQ-MINK-BR-027 is Optional and `policy only` — the SPEC commits to no actual loader.

**The question the prompt explicitly raises**: "Is this deferral genuinely safe, or does the renamed Go module break runtime config-file resolution if env vars and user paths still say 'goose'?"

Answer: It does not BREAK runtime — old env vars still work because Phase 2-8 do not touch them. But it creates a confusing brand/runtime split window. There is no functional regression, only a UX inconsistency.

**Required correction**: Add explicit acknowledgement in §1.3 Non-Goals OR §2.2 Why Now that "during the window between this SPEC's merge and SPEC-MINK-ENV-MIGRATE-001 + SPEC-MINK-USERDATA-MIGRATE-001 merges, the binary `mink` continues to read `GOOSE_*` env vars and `./.goose/` paths. This is functionally backward-compatible (no regression) but produces a brand-runtime split window." And update R8 + R9 mitigations to reference this window explicitly. Optionally specify that `mink --help` emits a one-line note ("Note: env var rename in progress, see SPEC-MINK-ENV-MIGRATE-001") during the window.

**Severity**: HIGH — UX/comms gap; not a hard defect but invites user confusion at v0.1.0 public window which §2.2 explicitly cites as motivation.

#### H-2 — README badge URLs will 404 between PR merge and post-merge `gh repo rename`

**Where**: spec.md §6 Phase 7 commit produces `README.md badges = modu-ai/mink` (AC-013 step 4, line 382); §6 Post-merge step 1 performs `gh repo rename` AFTER PR merge.

Sequence:
1. PR merged → main has README pointing to `modu-ai/mink` badges
2. GitHub still has `modu-ai/goose` repo (rename not yet executed)
3. README on main → badges request `img.shields.io/.../modu-ai/mink/...` → shields.io requests `api.github.com/repos/modu-ai/mink/...` → 404 (repo does not exist yet)
4. Time gap T = time between PR merge and orchestrator running `gh repo rename`. T could be minutes or hours depending on operator availability.

After post-merge step 1 finishes, GitHub redirect kicks in and badges resolve. But T-window badges are broken.

**Required correction**: Either —
- (a) Phase 7 commit message + commit must keep badge URLs at `modu-ai/goose` (or use `${{ github.repository }}` parameterization) until post-merge step 1 completes. Then a follow-up commit (post-rename) updates to `modu-ai/mink` literal. OR
- (b) Use `${{ github.repository }}` in README badge URLs (parameterized) so that the badge auto-adapts to repo rename. This is the cleanest. README badge currently hardcoded per research §5.4 evidence.
- (c) Sequence the post-merge `gh repo rename` AHEAD of PR merge (impossible because the PR rewrites `cmd/mink` paths that depend on Go module path being `github.com/modu-ai/mink`, which depends on... actually this is independent of repo name, just badge URL coupling).

Recommendation: option (b). Update AC-013 step 4 to require `${{ github.repository }}` parameterization rather than literal `modu-ai/mink`.

**Severity**: HIGH — temporary user-visible breakage on a brand-sensitive PR.

#### H-3 — AC-MINK-BR-002 step 6 has a weasel-word escape hatch

**Where**: spec.md L222 (AC-MINK-BR-002 step 6: "`go test ./...` exit 0 (or skip with documented reason if test infra not available in PR env)").

"or skip with documented reason" is unbounded — a tester could skip any failure with a one-sentence rationale. Phase 2 atomicity is THE highest-risk operation in the plan (R1 "매우 높"); the verification gate on it must not have an escape hatch.

**Required correction**: Replace step 6 with: `go test ./... exit 0`. If the test infra is genuinely unavailable in CI, fix the CI, do not weaken the AC. Alternatively: enumerate the specific tests that may legitimately fail (e.g., integration tests requiring external services) and gate the exemption on `-short` flag or test tag.

**Severity**: HIGH — undermines the core atomicity guarantee that R1 + REQ-021 establish.

#### H-4 — Phase 2 sed pattern leaves doc-comment / non-quoted refs to `github.com/modu-ai/goose` undetected before commit

**Where**: spec.md L495 (Phase 2 step 3): `sed -i.bak 's|"github.com/modu-ai/goose|"github.com/modu-ai/mink|g; s|"github.com/modu-ai/goose"|"github.com/modu-ai/mink"|g'`

The pattern requires a leading `"` to match. This is intentional (avoids matching docstring URLs that should also change but more cautiously). But the verification step (`grep -rln 'github.com/modu-ai/goose' --include='*.go' | wc -l = 0`) is strict and *will* catch leftover doc-comment refs.

The plan does not document what to do when verification fails on doc-comment refs (e.g., a comment `// see github.com/modu-ai/goose/internal/foo for details`). The implicit answer is "manual Edit". This should be explicit because partial-rename break of Phase 2 atomicity is the #1 risk.

**Required correction**: Add Phase 2 step 4.5 (between sed and verification): "Run `grep -rn 'github.com/modu-ai/goose' --include='*.go'` to enumerate any non-import references (doc-comments, package paths in comments). Manually Edit each remaining occurrence using the Edit tool. Then re-run sed cleanup and re-verify."

**Severity**: HIGH — Phase 2 is atomic, so if the commit lands with leftover refs in comments, `go build` does not break (comments are ignored) — but the verification step's strict `wc -l = 0` would block the commit before that point. So this is actually a CI/operator-procedure clarity issue, not a runtime correctness issue. Still high enough priority to fix because operator confusion at this exact step could lead to the operator weakening the verification.

#### H-5 — 3 orphan non-Optional REQs (REQ-005, REQ-011, REQ-023) have no AC mapping

**Where**:
- REQ-MINK-BR-005 (spec.md L155): "New SPECs **shall** use the `SPEC-MINK-XXX-NNN` directory prefix." — no AC tests this. The SPEC itself is the only positive example (proof by existence).
- REQ-MINK-BR-011 (spec.md L164): "**When** a SPEC author creates a new SPEC document, the SPEC template **shall** include a reference link to `.moai/project/brand/style-guide.md` and the directory **shall** use `SPEC-MINK-XXX-NNN` prefix." — no AC tests the SPEC template or the directory naming check.
- REQ-MINK-BR-023 (spec.md L182): "**If** any commit attempts to edit a published CHANGELOG entry (release section header older than today, 2026-05-12, at the time of this SPEC), **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds." — no AC tests CHANGELOG-edit rejection. AC-006 covers HISTORY rows (different artifact).

Orphan rate: 3/24 non-Optional = 12.5%. (REQ-025/026/027 are Optional so their lack of AC is acceptable.)

**Required correction**: Either —
- (a) Add 3 small ACs covering these REQs (REQ-005: AC verifies directory naming via `ls .moai/specs/SPEC-MINK-* | wc -l ≥ 1`; REQ-011: AC verifies SPEC template includes style-guide reference link; REQ-023: AC verifies brand-lint script rejects a synthetic CHANGELOG edit attempt); OR
- (b) Reclassify as informational / documentation REQs and explicitly tag them as "self-evident / out of AC scope".

Recommendation: option (a) for REQ-023 (it's the strongest claim — actual CI behavior); option (b) for REQ-005 and REQ-011 (more procedural).

**Severity**: HIGH — orphan REQs reduce SPEC integrity and weaken plan-auditor's traceability check.

### MEDIUM (should-fix in this PR or follow-up commit)

#### M-1 — Phase 2 verification omits `go mod tidy` go.sum check

**Where**: spec.md §6 Phase 2 (lines 484-525). R5 (line 903) says "Go module proxy 캐싱 stale ... 본인 환경: `go clean -modcache` 또는 `GOPROXY=direct`" but Phase 2 verification doesn't gate on `go.sum` line-reordering.

`go mod tidy` after module rename may produce go.sum reordering (low probability but possible). Plan does not specify whether Phase 2 commit should include the reordered go.sum or not.

**Required correction**: Add Phase 2 step 4.6: `go mod tidy && git diff go.sum`. If `go.sum` differs, include in the Phase 2 atomic commit. If not, no action.

#### M-2 — AC-MINK-BR-016 step 4 verifies CLAUDE.local.md which is gitignored (no CI verification path)

**Where**: spec.md L415 (AC-016 step 4: "CLAUDE.local.md h1 = `# CLAUDE Local Instructions — MINK Project` (또는 동등)").

CLAUDE.local.md is gitignored per its own §3.1 (not committed). CI cannot verify this AC. The verifier is implicit (manual operator check).

**Required correction**: Either —
- (a) Mark AC-016 step 4 as "manual operator verification" and add a checklist line in the PR description template; OR
- (b) Drop AC-016 step 4 (since the file is the operator's own local property and SPEC scope shouldn't dictate its h1).

#### M-3 — AC-010 step 2 and AC-011 step 2 use `≥` lower bounds with no upper bound

**Where**:
- AC-010 step 2 (line 341): `grep ... 'MinkHome' ... ≥ 64` — passing on accident if the rename produced 200 MinkHome references (e.g., due to a bug).
- AC-011 step 2 (line 355): `grep ... '\bMINK\b' ... ≥ 27`

`≥` allows the rename to add false positives (e.g., a buggy sed that introduced extra `MinkHome` strings). Need both lower and upper bound, or exact equality.

**Required correction**: Replace `≥ N` with `= N` (or `in range [N, N+5]` to accommodate doc-comment add-ons from manual Edit). Specifically AC-010 step 2 should be `= 64` (exact), AC-011 step 2 should be `= 27` (or `≥ 27` with explicit upper bound `≤ 50`).

#### M-4 — §7.5 brand-lint algorithm step 2 lists "본 SPEC 자체" as exempt but does not specify how brand-lint recognizes "본 SPEC"

**Where**: spec.md L847: "본 SPEC 자체 (`.moai/specs/SPEC-MINK-BRAND-RENAME-001/`) — 규범 설명 시 옛 표기 인용 필요".

The exemption is hardcoded by directory path. Future plan-time iterations of `SPEC-MINK-BRAND-RENAME-002` (if any) would not be exempt. This is fine for now but creates a long-term maintenance trap.

**Required correction**: Generalize exemption: "Any SPEC whose body explicitly discusses brand-rename normative rules (e.g., `SPEC-MINK-BRAND-RENAME-*`)". Or: keep hardcoded but add a TODO note for future generalization.

#### M-5 — R12 (large PR review burden) understates the diff size

**Where**: spec.md L910 (R12: "본 SPEC PR squash 가 거대해 review 부담").

Phase 2 alone modifies 458 files × ~2 lines/file = ~900 lines (modifying imports). Phase 4 deletes ~10 generated files + adds ~10 regenerated files + modifies 4 .proto. Phase 5 git mv 2 dirs. Phase 7 ≥ 30 files. Phase 8 ~27 files.

Squash PR diff: realistic ~600-1500 LOC excluding generated code, ~3000+ including generated. R12 mitigation says "review 는 phase 별로" — but squash merge collapses commits into a single commit. After squash, the phase-level commits are lost from main's history. Phase-level commits ARE preserved on the feature branch before squash, but reviewers must access the PR's "Commits" tab (not main's history) to review by phase.

**Required correction**: Clarify R12 mitigation: "Use PR's Commits tab during review; squash-merge collapses but pre-merge feature-branch commits remain visible on PR." Add to commit-message preamble: "이 SPEC PR 의 review 는 squash 전 phase 단위 commit 으로 검토하세요."

### LOW (optional polish)

#### L-1 — Phase 3 / Phase 4 ordering unjustified

**Where**: §6 Phase table (line 444-446). Both depend on Phase 2 only. Plan presents 3→4 but provides no rationale. The order is safe (no sed scope clash) but for plan-reader clarity, add 1 line: "Phase 3 and Phase 4 are independent and could be parallel commits; sequential 3→4 chosen for cognitive simplicity during review".

#### L-2 — AC-MINK-BR-013 step 1 cross-reference confusion

**Where**: AC-013 step 1 (spec.md L379): "`.github/workflows/*.yml` 중 `modu-ai/goose` 하드코딩 0건 (Phase 4.3 baseline 확인된 대로 원래도 0건)".

"Phase 4.3" likely means research.md §5.4 (which is `4.3` in older numbering or `5.4` in current numbering). Confusing. Use `research.md §5.4` explicitly.

#### L-3 — HISTORY row (v0.1.0) claims "27 EARS 요구사항 + 17 AC" — verified, but row is verbose and not in PR-style summary

**Where**: spec.md L23 (HISTORY row 0.1.0). Long sentence form makes future-row alignment harder. Minor stylistic.

#### L-4 — `gh repo rename` syntax in AC-003 (line 232)

**Where**: AC-003 step 1: `gh repo rename mink --repo modu-ai/goose`. Syntax is valid (`<new-name> --repo OWNER/REPO`). But add a one-line confirmation that operator must run from any directory (does not require being in the worktree).

### Findings summary

- CRITICAL: 2 (C-1 brand-lint blocks supersede commit; C-2 AC-015 unsatisfiable)
- HIGH: 5 (H-1 env/path UX split; H-2 badge 404 window; H-3 escape hatch in AC-002; H-4 Phase 2 sed doc-comment leftover; H-5 orphan REQs)
- MEDIUM: 5 (M-1 go.sum; M-2 CLAUDE.local.md gitignored; M-3 `≥` bounds; M-4 self-exempt pattern; M-5 R12 mitigation clarity)
- LOW: 4 (L-1 to L-4)

---

## Verbatim corrections required (line-level)

The CONDITIONAL_PASS verdict assumes the following six minimum corrections are made before PR merge. Each is line-localized.

### Correction 1 (resolves C-1) — REQ-MINK-BR-020 and §7.5 step 5

**File**: spec.md line 179 (REQ-MINK-BR-020)
**Change from**:
```
**If** any commit attempts to modify a row in any existing SPEC's `## HISTORY` table (status irrespective: planned/draft/implemented/closed/completed/superseded), **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds.
```
**Change to**:
```
**If** any commit attempts to modify an existing row (at a row-index that was present at baseline) in any existing SPEC's `## HISTORY` table (status irrespective: planned/draft/implemented/closed/completed/superseded), **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds. Appending new rows after the last baseline row is permitted (e.g., status-transition entries such as predecessor SPEC supersede annotations).
```

**File**: spec.md line 855 (§7.5 step 5)
**Change from**:
```
5. **HISTORY 보존 검증**: 모든 SPEC 의 `## HISTORY` 섹션 내부에서 brand 패턴 변경이 발생했는지 baseline 과 byte-level diff (REQ-MINK-BR-017 + REQ-MINK-BR-020). 변경 발견 시 즉시 fail.
```
**Change to**:
```
5. **HISTORY 보존 검증**: 모든 SPEC 의 `## HISTORY` 섹션 내부에서 baseline 행의 byte-level 변경이 발생했는지 검증. baseline 행 이후에 새 행이 추가된 경우 (append-only) 는 허용 (REQ-MINK-BR-020 의 modify 정의 제외). baseline 행의 cell 내용이 변경된 경우 즉시 fail.
```

**Rationale**: Resolves the contradiction with AC-MINK-BR-004's required predecessor SPEC HISTORY row append.

### Correction 2 (resolves C-2) — AC-MINK-BR-015 step 1

**File**: spec.md line 400 (AC-MINK-BR-015 step 1)
**Change from**:
```
1. 새 Go 코드 (`_test.go` 제외) 가 default user-data path 로 `./.mink/` 또는 `~/.mink/` 를 사용
```
**Change to**:
```
1. 본 SPEC Phase 8 commit 은 default user-data path 변경 작업을 포함하지 않으며, 이는 downstream SPEC-MINK-USERDATA-MIGRATE-001 의 scope 이다 (§1.3 Non-Goal, §11 item 10 참조). 본 AC 의 step 1 은 vacuously true (no new path additions in this PR).
```

Alternatively delete AC-015 entirely and add a stub: `### AC-MINK-BR-015 — DEFERRED — see SPEC-MINK-USERDATA-MIGRATE-001`.

### Correction 3 (resolves H-1) — Add §1.3 paragraph on brand-runtime split window

**File**: spec.md insert between line 60 and line 61 (in §1.3 Non-Goals)
**Insert**:
```

> **Brand-runtime split window acknowledgement**: between this SPEC PR merge and the merges of SPEC-MINK-ENV-MIGRATE-001 + SPEC-MINK-USERDATA-MIGRATE-001, the binary `mink` brand-positions all output as MINK but reads `GOOSE_*` env vars (21 keys, 824 lines) and defaults to `./.goose/` + `~/.goose/` workspace paths (117 lines). This is functionally backward-compatible (no runtime regression) but produces a temporary brand-runtime inconsistency window. Mitigation: downstream SPECs are pre-planned (§8.4); CHANGELOG entry for this SPEC explicitly documents the window.
```

### Correction 4 (resolves H-2) — AC-MINK-BR-013 step 4 parameterization

**File**: spec.md line 382 (AC-MINK-BR-013 step 4)
**Change from**:
```
4. `README.md` badge URL 4개가 `modu-ai/mink` 참조
```
**Change to**:
```
4. `README.md` badge URL 4개가 `${{ github.repository }}` 또는 동등 parameterization 으로 작성됨 (literal `modu-ai/mink` 도 허용하나 post-merge GitHub rename 까지 badge 404 window 가 발생 — parameterization 권장)
```

And update Phase 7 task list (line 693) to mirror this.

### Correction 5 (resolves H-3) — AC-MINK-BR-002 step 6 escape hatch

**File**: spec.md line 222 (AC-MINK-BR-002 step 6)
**Change from**:
```
6. `go test ./...` exit 0 (or skip with documented reason if test infra not available in PR env)
```
**Change to**:
```
6. `go test ./...` exit 0 — exemptions: only tests gated by `-tags integration` or requiring external services may be skipped, and only with `-short` flag explicitly enabled. Generic skip with prose rationale is not permitted.
```

### Correction 6 (resolves H-5 in part) — Add AC for REQ-MINK-BR-023

**File**: spec.md after line 432 (end of AC-MINK-BR-017)
**Insert** new AC:
```
### AC-MINK-BR-018 — CHANGELOG 기존 entry 무변경 + 신규 entry brand 통일

**Given** Phase 1 시작 시점에 baseline 으로 다음이 캡처된 상태에서:
- `CHANGELOG.md` 의 모든 release section header 와 그 본문의 SHA-256 hash (current 2026-05-12 시점 모든 entry)

**When** 본 SPEC PR squash merge 후 동일 추출을 재실행하면

**Then**:
1. baseline 모든 release section 의 header + 본문 SHA-256 hash 가 보존됨 (byte-identical)
2. 신규 entry (본 SPEC merge 가 추가하는 `## [Unreleased]` 또는 SPEC ID 참조 라인) 에 `MINK` 사용
3. 의도적으로 baseline release section 의 brand-position 토큰을 수정한 시험 PR 이 brand-lint check 실패로 merge 차단

REQ 매핑: REQ-MINK-BR-014, REQ-MINK-BR-023
```

This brings 4.4 Unwanted REQ-023 into AC coverage. REQ-005 and REQ-011 remain orphan but are reclassified to "procedural / self-evident" with footnote on respective REQ lines.

(Optional 6.5) — Add Phase 2 doc-comment cleanup step (resolves H-4):

**File**: spec.md after line 497 (Phase 2 도구 step 4)
**Insert** step 4.5:
```
4.5. **doc-comment 잔존 검사**: `grep -rn 'github.com/modu-ai/goose' --include='*.go'` → 이미 0 일 것이나, 만약 1+ 발견되면 모두 doc-comment 또는 string literal 이다. 각 위치를 Edit tool 로 manual edit (sed 의 `"` prefix 제약 회피). 재실행하여 0 확인 후 step 5 (go mod tidy) 진행.
```

---

## Faithfulness to 8 confirmed decisions

Re-check each:

### 1. Scope = MINK-BRAND-RENAME-001 only — **PASS**
**Evidence**: spec.md L31 "본 SPEC은 메타 SPEC", §1.3 Non-Goals lines 58-59 explicitly defer `SPEC-MINK-DISTANCING-STATEMENT-001` and `SPEC-MINK-PRODUCT-V7-001`. §11 items 8 and 9 reinforce. No conflation with PRODUCT-V7 or DISTANCING work.

### 2. Predecessor SPEC supersede + body immutable — **CONDITIONAL PASS (correction required)**
**Evidence for IN-compliance**: §1.4 lines 65-71 say "선행 SPEC의 body 와 HISTORY 는 immutable로 유지된다", "본 SPEC PR 의 squash commit 은 선행 SPEC body 의 어떤 byte도 건드리지 않는다". AC-MINK-BR-004 (lines 246-256) explicitly verifies this with byte-level snapshot + SHA-256 hash. §1.4 line 69 specifies the supersede commit is "별도 후속 commit (orchestrator 담당)".

**Evidence for the gap**: REQ-MINK-BR-020 + §7.5 step 5 will block the supersede commit per finding C-1. Decision #2 is conceptually present and correctly worded, but the implementation mechanism (brand-lint CI gate) contradicts itself.

**After Correction 1, this becomes a clean PASS.**

### 3. Branch / PR strategy — **PASS**
**Evidence**: spec.md §6 (line 438) "단일 PR (base=main, squash merge per CLAUDE.local.md §1.4)". §6 PR 분량 정책 (line 783) confirms squash merge. §6 Phase commit message templates use `<type>(<scope>): SPEC-MINK-BRAND-RENAME-001 Phase N — ...` + Korean body + `SPEC:` / `REQ:` / `AC:` trailers, matching CLAUDE.local.md §2.2.

Current branch: `plan/SPEC-MINK-BRAND-RENAME-001` (verified live). ✓

### 4. Solo mode — **PASS**
**Evidence**: spec.md does not invoke `--team` or reference Agent Teams. R12 mitigation (line 910) implicitly assumes single-operator review. The HISTORY row (line 23) reflects "--solo mode" in the decision list.

### 5. SPEC ID prefix policy — **PASS**
**Evidence**: REQ-MINK-BR-005 (line 155), §3.1 item 5 (line 119), §3.2 item 1 (line 132), AC-MINK-BR-005 (lines 258-270), research.md §6 (lines 380-403). 88 SPEC-GOOSE-* preserved, new = SPEC-MINK-*. AC-005 verifies count == 88 byte-level.

### 6. Immutable history — **CONDITIONAL PASS (correction required for AC-006 + REQ-020)**
**Evidence**: §3.2 items 2, 3, 4, 5 (lines 133-136) cover SPEC HISTORY rows, git commit messages, CHANGELOG entries, PR/issue titles. AC-006 (lines 272-284) byte-level verifies HISTORY rows across all 108 SPECs. AC-005 (line 258-270) verifies 88 SPEC-GOOSE dirs. AC-014 (line 388) verifies `.claude/agent-memory/**` SHA-256.

**Gap**: REQ-023 (CHANGELOG editing) has no AC (orphan, see H-5). After Correction 6 adds AC-018, this becomes a clean PASS.

### 7. Module / repo target — **PASS**
**Evidence**: REQ-MINK-BR-003 (line 153) `github.com/modu-ai/mink`. REQ-MINK-BR-004 (line 154) `modu-ai/mink`. REQ-MINK-BR-007 (line 157) `cmd/mink/` and `cmd/minkd/`. REQ-MINK-BR-008 (line 158) `mink.v1`. All four sub-targets named explicitly and verified by AC-002 (module), AC-003 (repo), AC-008 (binary dirs), AC-009 (proto).

### 8. Brand assets reuse — **PASS**
**Evidence**: §1.4 line 71 explicit reuse list. §3.1 item 12 (line 126). Phase 1 (lines 452-482) only rewrites style-guide and retargets check-brand.sh, does not rewrite brand-lint.yml or Makefile. Research §7 (lines 405-422) gives the asset-reuse matrix.

**Summary**: 6 of 8 decisions are unconditional PASS. 2 (decision #2, #6) require Correction 1 and Correction 6 respectively to be unambiguously aligned with the SPEC's own enforcement mechanism.

---

## Recommendations

1. **Apply Corrections 1, 2, 3, 4, 5, 6** (line-level edits per §Verbatim corrections required). Estimated effort: 30-60 minutes of careful editing in spec.md. Apply Optional 6.5 (Phase 2 doc-comment cleanup) if operator time permits.

2. **Update HISTORY row** to v0.1.1 after applying corrections, with a Change entry citing this audit report path and listing the 2 CRITICAL + 5 HIGH finding IDs resolved.

3. **Re-run plan-auditor** after corrections to verify each finding is closed. Expected: PASS verdict on second iteration.

4. **Operator pre-flight checklist** before opening the PR (suggest adding to PR description template):
   - [ ] `git status` clean except this SPEC's files
   - [ ] Phase 2 atomic commit verified locally (`go build ./...` + `go vet ./...` + `go test ./...` after Phase 2)
   - [ ] Baseline snapshots captured to `.moai/specs/SPEC-MINK-BRAND-RENAME-001/migration-log.md`
   - [ ] Predecessor SPEC SHA-256 captured pre-merge
   - [ ] `.claude/agent-memory/**` SHA-256 captured pre-merge
   - [ ] post-merge `gh repo rename` script prepared in shell history

5. **Decision documentation**: After PR merge, add an entry to project memory `~/.claude/projects/.../memory/project_minkmar_001_complete.md` documenting (a) the brand-runtime split window timing, (b) downstream SPEC ordering, (c) any drift between baseline counts (research.md §2) and post-merge verification.

6. **Long-term**: Once SPEC-MINK-ENV-MIGRATE-001 and SPEC-MINK-USERDATA-MIGRATE-001 merge, audit the v0.2.x branch state to confirm the brand-runtime split window is closed. Optionally add a final SPEC-MINK-RENAME-COMPLETE-001 status SPEC marking the multi-SPEC rename effort as fully complete.

---

**Auditor verdict (final)**: CONDITIONAL_PASS — apply the 6 line-level corrections above, then the plan is ready for `/moai run` entry. Without corrections, the plan as-merged would self-block via brand-lint CI on the predecessor supersede commit (C-1) and produce an unsatisfiable AC-015 (C-2).

# SPEC Review Report: SPEC-MINK-USERDATA-MIGRATE-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.84

Reasoning context ignored per M1 Context Isolation. Audit derived solely from `spec.md`, `acceptance.md`, `plan.md`, `spec-compact.md` at v0.1.1.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-MINK-UDM-001 ~ REQ-MINK-UDM-019 enumerated (spec.md:L199–L232). 19 entries, sequential, no gaps, no duplicates, 3-digit zero padding consistent. Verified by `grep -oE 'REQ-MINK-UDM-[0-9]+' spec.md | sort -u | wc -l = 19`.
- **[PASS] MP-2 EARS format compliance**: 18/19 use canonical EARS templates with correct tags. REQ-019 tagged `[Ubiquitous]` (spec.md:L232) has loose phrasing — Korean prose "마이그레이션은 원본 파일의 mode bits를 보존해야 한다" instead of strict "The [system] shall [response]". Mid-sentence "when … 가 atomic rename 이 실패하여" embeds an event-driven clause inside a Ubiquitous tag. Marginal pass — intent is clear, the "shall preserve" semantic is encoded as "보존해야 한다", which is the Korean rendering of "shall preserve" (EARS Ubiquitous). Other 18 REQs match their tagged patterns exactly.
- **[PASS] MP-3 YAML frontmatter validity**: spec.md:L1–L13. `id`, `version: "0.1.1"`, `status: draft`, `created_at: 2026-05-13`, `updated_at: 2026-05-13`, `author`, `priority: High`, `labels: [...]`. All required fields present with correct types.
- **[PASS/N-A] MP-4 Section 22 language neutrality**: N/A — single-language (Go) scope.

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75–1.0 (mostly unambiguous, 1 self-contradictory AC discovered) | New defect ND-1 introduces a hard contradiction within AC-001 #6: the rule "메시지가 단어 `goose` 를 포함하지 않는다" is violated by the AC's own "base 예시" examples (`~/.goose 디렉토리에서 ~/.mink 디렉토리로`, `Migrated user data from ~/.goose to ~/.mink.`). A literal implementation following the example fails the gate. |
| Completeness | 0.95 | 0.75–1.0 | HISTORY rows added in spec.md/plan.md/acceptance.md (v0.1.1). REQ-019 (mode bits) added. AC-006/007/008a/008b/009/010 added. Spec-compact.md regenerated to match. Defect Resolution Map added (spec-compact.md:L214–L230). |
| Testability | 0.85 | 0.75–1.0 | Most AC checks remain binary; AC-009 dual `stat -c`/`stat -f` notation correct (D6). AC-005 #1 phrased as "모든 라인이 legacy.go 에서만 나와야 함" — not a single-command binary gate (combined with #3/#4 which use `wc -l = 0` it is covered, but #1 itself is descriptive). AC-001 #6 example contradicts its grep gate (testability impaired by inconsistency). |
| Traceability | 1.00 | 1.0 band | All 19 REQs are referenced in `REQ 매핑:` lines in acceptance.md. Verified by set diff: `comm -23 <(grep -oE 'REQ-MINK-UDM-[0-9]+' spec.md \| sort -u) <(grep -oE 'REQ-MINK-UDM-[0-9]+' acceptance.md \| sort -u)` returns empty. REQ-004 → AC-006, REQ-016 → AC-007, REQ-018 → AC-008a/008b, REQ-019 → AC-009. |

## Regression Check (Iteration 2)

Iter-1 defect resolution status:

- **D1 (REQ-004 traceability)** — RESOLVED. AC-MINK-UDM-006 (acceptance.md:L165–L182) verifies tmp file prefix `.mink-` via `find ... -name '.mink-*'`. REQ 매핑 references REQ-MINK-UDM-004.
- **D2 (REQ-016 traceability)** — RESOLVED. AC-MINK-UDM-007 (acceptance.md:L186–L208) and Quality Gate row "Test file marker enforcement" (acceptance.md:L437) provide the enforcement grep. `grep -rEn '"\.goose' --include='*_test.go' . | grep -v '^./internal/userpath/legacy_test\.go:' | grep -v 'MINK migration fallback test' | wc -l = 0`. REQ-016 mapping cited.
- **D3 (REQ-018 traceability — MINK_HOME bypass)** — RESOLVED with EXPANSION. AC-008a (happy path) + AC-008b (4 negative cases: empty / legacy `.goose` / path-traversal / non-writable). Plan.md Phase 1 names typed errors (`ErrMinkHomeEmpty`, `ErrMinkHomePathTraversal`, `ErrMinkHomeIsLegacyPath`). Coverage is substantive.
- **D4 (grep `--exclude=<path>` basename limitation)** — RESOLVED. Replaced with post-filter pipeline `grep -rEn '"\.goose' ... . | grep -v '^./internal/userpath/legacy\.go:' | wc -l`. Verified on /usr/bin/grep (BSD) with `.` start path: prefix is `./internal/...`, the regex `^./internal/userpath/legacy\.go:` matches (`.` is regex-any but also matches literal `.`). Behavior is correct: legacy.go lines filtered, others retained. Acceptable across GNU and BSD grep variants.
- **D5 (REQ-008 vs AC-001 #6 contradiction on bilingual notice)** — REQ side RESOLVED (REQ-008 explicitly says "Korean (with optional English subtext)"). AC side INTRODUCES NEW CONTRADICTION — see ND-1 below.
- **D6 (file-mode preservation unverified)** — RESOLVED. REQ-MINK-UDM-019 added (spec.md:L232). AC-MINK-UDM-009 (acceptance.md:L279–L309) verifies mode 0600 for `permissions/grants.json`, `messaging/telegram.db`, `mcp-credentials/anthropic.json`, `ritual/schedule.json` with dual Linux `stat -c '%a'` / macOS `stat -f '%Lp'` notation. Plan.md Phase 1 §51–55 commits to `os.Chmod(dst, srcInfo.Mode().Perm())`. Risk R13 (plan.md:L215) added.
- **D7 (AC-001 #6 weasel "동등 영문")** — PARTIALLY RESOLVED. The exact weasel phrase "또는 동등 영문" was removed; replaced with grep gate `grep -c 'goose' = 0` + `grep -Ec 'mink|밍크' ≥ 1`. However, the new gate is self-contradictory (ND-1).
- **D8 (AC-004 CLI/daemon merge)** — RESOLVED. Split into AC-004a (acceptance.md:L97–L115, exit non-zero fail-fast) and AC-004b (L119–L138, exit 0 graceful degrade + warning + read-only fallback). Distinct exit code expectations stated.
- **D9 (REQ-017 brand-marker absent path)** — RESOLVED. AC-MINK-UDM-010 (acceptance.md:L313–L337) covers `~/.goose/.mink-managed` absence + best-effort warning + `brand_verified: false` marker field.
- **D10 (backtick imbalance L52/L178/L292)** — RESOLVED. spec.md:L53 ("**`.moai/specs/*` SPEC 디렉토리 변경**"), L179, L297 (now in §6) all balanced. Verified by reading lines 51–53, 177–179, 291–293.
- **D11 ("30+ 콜사이트" vs "18 files" inconsistency)** — PARTIALLY RESOLVED. spec.md/spec-compact.md updated to "30+ callsites across 18 distinct files". plan.md:L13 updated. BUT plan.md:L203 (R1 in Risk table) still reads "30+ 콜사이트의 path semantics 비균질" — unconverted body text. See ND-2.
- **D12 (`stat -c` GNU-only)** — RESOLVED. AC-003 (acceptance.md:L85–L88) and AC-009 (L298–L304) both provide Linux `stat -c '%a'` and macOS `stat -f '%Lp'` with portable `ls -ld` fallback noted in AC-003.
- **D13 (Phase numbering hazard)** — N/A (iter-1 was informational, no defect to resolve).

Of 13 iter-1 defects: **11 fully resolved, 2 partially resolved (D7/D11)**. No stagnation pattern (each defect shows substantive movement).

## Defects Found (Iteration 2)

ND-1. **acceptance.md:L40–L43 — AC-001 #6 base examples violate AC-001 #6 grep gate** — Severity: **major**. The new gate states "메시지가 단어 `goose` 를 **포함하지 않는다** (`grep -c 'goose' stderr.log` = 0)". Both base examples literally contain `goose`:
   - Korean: `INFO: 사용자 데이터가 ~/.goose 디렉토리에서 ~/.mink 디렉토리로 마이그레이션되었습니다.` ← contains `goose`
   - English subtext: `Migrated user data from ~/.goose to ~/.mink.` ← contains `goose`
   A literal implementation following the example fails the gate (`grep -c 'goose' = 2`, not 0). Implementer cannot tell which is normative — the example or the gate. This is a **self-contradiction introduced during D7 fix**. Two valid remediations:
   (a) Tighten gate to "no `goose` outside path-quoting literal context" (operationally harder); or
   (b) Update examples to use the legacy path quoting (`옛 디렉토리에서` / `previous directory`) to avoid the literal `goose` word.

ND-2. **plan.md:L203 — D11 phrasing regression in Risk table R1** — Severity: minor. spec.md and spec-compact.md adopted "30+ callsites across 18 distinct files" consistently. plan.md HISTORY (L8), §1 (L13), §2 P2 row (L30), §2 phase 2 body (L88) all updated. But the Risk table R1 (L203) still reads "30+ 콜사이트의 path semantics 비균질". Body content was missed during D11 sweep.

ND-3. **acceptance.md:L450 — Definition of Done says "11 main scenarios" but actual count is 12** — Severity: minor. DoD: "AC-MINK-UDM-001 ~ AC-MINK-UDM-010 (11 main scenarios — AC-004 split 포함)". Counting AC-001, 002, 003, 004a, 004b, 005, 006, 007, 008a, 008b, 009, 010 = **12 main**. HISTORY (acceptance.md:L8 and spec.md:L22) correctly says "12 main + 4 edge". Internal inconsistency: DoD line and HISTORY rows disagree.

ND-4. **spec.md:L44 stale "11+ binary-verifiable Acceptance Criteria"** — Severity: minor. With AC-010 added, total main AC = 12. The phrase "11+" is technically not false (12 ≥ 11) but is a stale draft remnant that does not reflect the v0.1.1 expansion. Should read "12 binary-verifiable Acceptance Criteria + 4 edge cases" for parity with acceptance.md HISTORY.

ND-5. **acceptance.md:L424 quality gate count "13개 항목" vs actual 15 rows** — Severity: minor. DoD says "Quality gate 13개 항목 모두 통과 (v0.1.1 신규 gate 3개 포함)". The Quality Gate table (acceptance.md:L427–L441) has **15 rows** (Coverage전체, Coverage userpath, Race, Lint, Vet, Build, Brand-lint, LSP, Path-resolver, Test marker, Mode bits, MINK_HOME boundary, TRUST Tested, TRUST Secured = 14; counted again: 14). On strict count it is 14 rows, not 13. Whichever way: DoD's "13" is wrong by 1–2.

ND-6. **spec.md:L314 self-reference stale "v0.1.0"** — Severity: minor. §7.1 lists "본 문서, v0.1.0" but the frontmatter and HISTORY are at v0.1.1. Self-version reference unsynced.

ND-7. **REQ-MINK-UDM-019 EARS form not canonical** — Severity: minor. Tagged `[Ubiquitous]` but the body is a Korean prose statement, not the canonical "The [system] **shall** [response]" template. It also embeds an event-driven clause ("when `userpath.MigrateOnce()` 가 atomic rename 이 실패하여 copy fallback 경로를 사용하더라도"). Other 18 REQs follow the template strictly with bolded `**shall**`. Minor — semantic intent is preserved and the trace is intact via AC-009.

ND-8. **AC-005 #1 is descriptive, not binary** — Severity: minor. acceptance.md:L152 says "출력의 모든 라인이 `internal/userpath/legacy.go` 파일에서만 나와야 함" — this is a property assertion without a single command emitting a binary result. The subsequent #2/#3/#4 (with `wc -l = 0` and `grep -c = 1`) are binary. Tightening #1 to "`grep -rEn ... . | grep -v '^./internal/userpath/legacy\.go:' | wc -l = 0`" would close the loop, but #3/#4 already cover it. Subsumed; tolerable.

## Chain-of-Verification Pass

Second-look findings:

- Re-read spec.md §4 EARS section (L192–L232) — all 19 REQ entries individually checked. ND-7 (REQ-019 EARS strictness) confirmed.
- Re-read acceptance.md AC-001 #6 (L36–L43) fully — discovered ND-1 (self-contradiction between gate and example). This is the most consequential new defect.
- Enumerated all 12 main AC headings via `grep '^## AC-MINK-UDM-' acceptance.md` and 4 EC via `grep '^### EC-MINK-UDM-'` — confirmed counts 12 + 4 = 16 GIVEN/WHEN/THEN scenario blocks (matches `grep -cE '^\*\*Given\*\*' = 16`). DoD line and HISTORY line disagree (ND-3).
- Cross-referenced REQ-mapping in acceptance.md vs REQ definitions in spec.md — all 19 REQs covered exactly once or more in `REQ 매핑:` lines. Traceability now clean (D1/D2/D3 resolved).
- Re-checked spec-compact.md regeneration: Identity reflects v0.1.1, EARS count 19, AC count 12 main + 4 edge, Defect Resolution Map added (L214–L230). Counts match spec.md/acceptance.md.
- Verified D4 grep pipeline behavior empirically with /usr/bin/grep (BSD) on macOS: when start path = `.`, output prefix is `./internal/userpath/legacy.go:` and the audit pattern `^./internal/userpath/legacy\.go:` correctly matches/excludes. Acceptable across GNU and BSD grep.
- Backtick balance check (D10): spec.md:L53/L179/L297 all balanced.
- D11 sweep: discovered plan.md:L203 still has legacy phrasing (ND-2).
- Stat dual notation (D6): both AC-003 and AC-009 carry Linux `stat -c '%a'` AND macOS `stat -f '%Lp'`.
- AC-004 split (D8): two distinct ACs with opposite exit-code expectations confirmed.

New defects discovered on second pass: ND-1, ND-2, ND-3, ND-4, ND-5, ND-6, ND-7, ND-8.

## Recommendation

The revision is substantively strong: 11 of 13 iter-1 defects fully resolved with concrete artifacts (new AC scenarios, dual-stat notation, post-filter pipeline, AC split, REQ-019 + R13, brand-marker AC). Traceability is now perfect (1.00). The single blocking defect is **ND-1**, which is a logical self-contradiction inside AC-001 #6 introduced by the well-intentioned D7 fix — the grep gate `grep -c 'goose' = 0` is immediately violated by the AC's own example messages that reference the legacy path `~/.goose`. An implementer reading the example will produce code that fails the gate; an implementer reading the gate will produce a message that contradicts the example.

Required for PASS (iteration 3):

1. **(blocking) Reconcile AC-001 #6 self-contradiction.** Two acceptable remediations:
   - (a) Replace the gate with a brand-stance rule: the message must contain `mink` or `밍크`; the only `goose` substrings permitted are inside a recognizable path-quoting context (e.g., must appear after `~/.` or `./` immediately). Add this allow-list as a regex: `goose` occurrences must all match `[~./]\.goose(/|$|\s)` style, else fail. Adjust gate to `grep -Pc '(?<![~./])goose(?!\b\.)' stderr.log = 0` or equivalent path-aware filter.
   - (b) Update the base examples so they don't include the literal word `goose` outside the path context. Easiest: keep the path references but adjust the gate to "tokenized `goose` (not preceded by `.`)" — or rewrite examples to "이전 디렉토리 (~/.goose)" with parenthesized quoting so the only `goose` occurrences are inside the quoted path. The acceptance check would then say "outside the quoted path string ... should not appear."
   - Preferred: (a) — keep examples truthful (they need to communicate the source path), tighten the gate semantically.

2. **(minor) plan.md:L203 — replace "30+ 콜사이트" with "30+ callsites across 18 distinct files"** (ND-2). One-line edit.
3. **(minor) acceptance.md:L450 — DoD bullet: change "11 main scenarios" to "12 main scenarios"** (ND-3).
4. **(minor) spec.md:L44 — change "11+ binary-verifiable" to "12 binary-verifiable"** (ND-4).
5. **(minor) acceptance.md:L452 — DoD bullet: change "Quality gate 13개 항목" to match the actual table row count (14 by my count: Coverage전체, Coverage userpath, Race, Lint, Vet, Build, Brand-lint, LSP, Path-resolver, Test marker, Mode bits, MINK_HOME boundary, TRUST Tested, TRUST Secured)** (ND-5). Recount and reconcile.
6. **(minor) spec.md:L314 — change "본 문서, v0.1.0" → "본 문서, v0.1.1"** (ND-6).
7. **(optional, minor) REQ-MINK-UDM-019 — restate as canonical Ubiquitous EARS: "The migration **shall** preserve source file mode bits (never weakened) in the destination across all copy fallback paths."** (ND-7). Move conditional explanation into a sub-bullet under the REQ.

After fixes, re-submit for iteration 3. ND-1 alone (without the minor count fixes) is sufficient to keep this at FAIL; the AC contradiction would mislead implementation.

---

Verdict: FAIL

# SPEC Review Report: SPEC-MINK-USERDATA-MIGRATE-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.78

Reasoning context ignored per M1 Context Isolation. Audit derived solely from `spec.md`, `acceptance.md`, `plan.md`, `spec-compact.md`.

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-MINK-UDM-001 through REQ-MINK-UDM-018 are sequential, no gaps, no duplicates. Verified by enumeration at spec.md:L198–L227 (18 lines, one per REQ). Zero-padding consistent (3-digit).
- **[PASS] MP-2 EARS format compliance**: Every REQ carries an explicit pattern tag and matches the corresponding EARS template.
  - Ubiquitous (REQ-001…006, spec.md:L198–L203): all use `The [X] shall [Y]`.
  - Event-Driven (REQ-007…010, spec.md:L207–L210): all use `When [trigger], the [X] shall [Y]`.
  - State-Driven (REQ-011…013, spec.md:L214–L216): all use `While [condition], … shall …`.
  - Unwanted (REQ-014…016, spec.md:L220–L222): all use `If [undesired], then … shall …`.
  - Optional (REQ-017…018, spec.md:L226–L227): both use `Where [feature exists], … shall …`.
- **[PASS] MP-3 YAML frontmatter validity**: spec.md:L1–L13. Required fields all present with correct types — `id: SPEC-MINK-USERDATA-MIGRATE-001` (string, matches SPEC-{DOMAIN}-{NUM} extended-domain pattern), `version: "0.1.0"` (string), `status: draft` (string), `created_at: 2026-05-13` (ISO date), `priority: High` (string), `labels: [brand, userdata, migration, brownfield, cross-cutting, path-resolver]` (array). Optional fields (`author`, `issue_number`, `depends_on`, `related_specs`, `updated_at`) are non-conflicting extensions.
- **[PASS] MP-4 Section 22 language neutrality**: N/A — SPEC is single-language scoped (Go-only, internal/* package). No multi-language LSP tooling enumeration applies.

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75–1.0 (mostly unambiguous, two minor inconsistencies) | Single, unambiguous interpretation for nearly all REQs/ACs. Minor contradictions: REQ-008 says "Korean and English" stderr notice but AC-001 #6 reads `…마이그레이션되었습니다.` "(또는 동등 영문)" suggesting either-or, not both (acceptance.md:L36). |
| Completeness | 0.90 | 0.75–1.0 | HISTORY (spec.md:L17–L21), WHY/Background (§2 L75–L99), WHAT/Scope (§3 L130–L188), REQUIREMENTS (§4 L192–L227, 18 entries), ACCEPTANCE (acceptance.md, 5 main + 4 edge), Exclusions (§3.5 L174–L188 + §6 L288–L301, 10+ entries). YAML complete. Minor gap: no AC explicitly verifies REQ-018 nor REQ-004 nor REQ-016 (see Traceability). |
| Testability | 0.85 | 0.75–1.0 | Every AC stated in binary form (exit codes, SHA-256, byte-identical, `test -d`, `grep … wc -l`). Quality Gate table (acceptance.md:L217–L229) lists 11 measurable gates. Two non-binary phrasings — AC-004 #1 (acceptance.md:L99) uses "non-zero 또는 graceful degrade" (testable but condition-branching), AC-004 #4 (L102) "user-actionable error" is partly subjective. Otherwise tight. |
| Traceability | 0.55 | 0.50–0.75 band | 15/18 REQs have AC coverage. Three REQs lack any AC reference (see Defects D1, D2, D3). EC mappings ok. Reverse direction clean — every AC's REQ map cites existing IDs. |

## Defects Found

D1. **acceptance.md (entire file) — REQ-MINK-UDM-004 has zero AC coverage** — Severity: major. REQ-MINK-UDM-004 (spec.md:L201) states the tmp file prefix **shall** be `.mink-`. No AC mentions tmp file prefix or `.mink-` literal anywhere. AC-005 only checks for absence of `.goose` literal in source, which is a code-presence test, not a runtime-behavior test for the new prefix. A reviewer cannot verify REQ-004 from the AC list. Traceability gap.

D2. **acceptance.md (entire file) — REQ-MINK-UDM-016 has zero AC coverage** — Severity: major. REQ-MINK-UDM-016 (spec.md:L222) defines the policy that test files using `.goose` literal without a `// MINK migration fallback test` marker shall fail. No AC verifies this enforcement (e.g., a grep gate on `*_test.go` with the marker check). AC-005 explicitly excludes `*_test.go` from its grep. The Quality Gate table (acceptance.md:L227) only enforces non-test files. Traceability gap.

D3. **acceptance.md (entire file) — REQ-MINK-UDM-018 has zero AC coverage** — Severity: major. REQ-MINK-UDM-018 (spec.md:L227) says when `MINK_HOME` is set, `UserHome()` returns it verbatim and `shall not` attempt migration. AC-001 explicitly unsets `MINK_HOME` and AC-003 does the same. No AC sets `MINK_HOME=...` and asserts that no migration occurs even when `~/.goose/` exists. Traceability gap with a non-trivial security/behavior consequence (an env-var-driven bypass path is unverified).

D4. **acceptance.md:L121, L122 — grep `--exclude` uses path, not basename** — Severity: major. AC-005 Then #3 specifies `grep -rEn 'filepath\.Join\([^)]*"\.goose"' --include='*.go' --exclude='*_test.go' --exclude='internal/userpath/legacy.go'`. GNU/BSD `grep --exclude` matches **basename glob only**; passing `internal/userpath/legacy.go` excludes nothing (no file is literally named `internal/userpath/legacy.go`). Same defect on #4 (`--exclude='internal/userpath/userpath.go'`). The intended exclusion requires `--exclude='legacy.go'` or `find … -path` piping, or `git grep -- ':!internal/userpath/'`. As written, the gate is **either silently broken (no exclusion) or false-positive prone**.

D5. **REQ-MINK-UDM-008 ↔ AC-MINK-UDM-001 #6 contradiction on bilingual notice** — Severity: minor. spec.md:L208 mandates "stderr notice **in Korean and English**" (both). acceptance.md:L36 (AC-001 #6) accepts the Korean line alone with the parenthetical "또는 동등 영문" (or equivalent English) — i.e., either-or. The AC weakens the REQ. Either tighten the AC to require both lines, or relax REQ-008 to "in Korean or English".

D6. **acceptance.md (entire file) — file-mode preservation across copy fallback is unverified** — Severity: major. REQ-MINK-UDM-009 (spec.md:L209) mandates SHA-256 **content** equality. plan.md:L50 (Phase 1) commits the package to writing files at `0600` and dirs at `0700`. But for `permissions/grants.json`, `telegram.db`, `mcp-credentials`, the original on-disk permissions may already be more restrictive (e.g., 0600 secrets-only). No AC verifies that **file mode bits are preserved (or hardened, never weakened)** during the copy-and-verify fallback. This is a security-relevant gap because the audit/permission/credential stores contain sensitive data. AC-004 only checks SHA-256 of the **source** (preservation of input), not the **destination mode bits** of migrated secrets. Recommend a new AC clause: `stat -f '%Lp'` on `~/.mink/permissions/grants.json` after copy-fallback ≤ source mode and ≤ 0600.

D7. **acceptance.md:L36 — stderr alert assertion is not byte-precise** — Severity: minor. AC-001 #6 reads "정확히 1줄 알림" but allows "(또는 동등 영문)". "동등" (equivalent) is a weasel word reintroducing subjectivity. Either fix the exact byte string or define an allow-list regex.

D8. **acceptance.md:L99 — exit-code branching by binary type is testable but underspecified** — Severity: minor. AC-004 #1 says "exit 코드는 non-zero 또는 graceful degrade — daemon (`minkd`) 의 경우 startup 계속 + ErrReadOnlyFilesystem warning, CLI (`mink`) 의 경우 fail-fast + user-actionable error message". The condition is binary (which binary?) but the AC bundles two distinct test scenarios into one. Splitting into AC-004a (CLI fail-fast) and AC-004b (daemon graceful degrade) would remove ambiguity.

D9. **spec.md:L57 §1.3 + spec.md:L179 — third-party `goose` (Block AI) collision policy is described but unverified** — Severity: minor. The SPEC repeatedly relies on the brand marker file `.mink-managed` for safe identification (REQ-017, EC implicit), but no AC tests the negative path (marker absent → warning emitted, migration still proceeds best-effort). REQ-017 ACL coverage stops at AC-001 #1 (where marker is implicitly present via `config.yaml` MINK-specific field, never spelled out). The brand-collision risk (R4 in plan.md) deserves a dedicated AC.

D10. **spec.md:L52, L178, L292 — repeated typo: `**`.moai/specs/* SPEC**` (backtick imbalance)** — Severity: minor. The opening backtick before `.moai/specs/*` has no closing backtick before "SPEC". Recurs 3 times (§1.3 first bullet, §3.5 item 1, §6 item 1). Markdown rendering corrupted in those sections.

D11. **plan.md ↔ spec.md count drift** — Severity: minor. spec.md §1.1 (L42) says "30+ production 콜사이트", §2.4 (L102) says "30+ production 콜사이트", and §3.2 (L149) again "30+". The Affected Files table (spec.md:L103–L124) lists 18 production-file rows; spec-compact.md §Affected Files lists "18+ production" (correct), but plan.md §1 (L13) writes "30+ 콜사이트" and §2 Phase 2 mentions "30+ 파일". This is an inconsistency between **call sites** (likely 30+ literal occurrences across 18 files) and **files** (18). Standardize on one of: "30+ call sites in 18 files" or "18 files / 30+ literals".

D12. **acceptance.md:L80 — `stat -c` is GNU-only** — Severity: minor. AC-003 #3 says `stat -c '%a' ~/.mink/ 또는 macOS 의 stat -f '%Lp'`. Acceptable as the spec acknowledges platform variance, but the SPEC then drops the alternative form in D6-area defects' lack of cross-platform mode-checking. Note also that the SPEC excludes Windows from Phase 1 release (plan.md R9), so cross-platform gating in AC-003 should explicitly say "Linux/macOS only".

D13. **acceptance.md (Definition of Done) — Phase numbering mismatch hazard** — Severity: minor. acceptance.md:L238 says "Phase 1-6 모두 commit + squash merge 완료" — plan.md §2 lists 6 phases (P1–P6). Consistent. But spec.md §1.1 (L44) refers to "6-phase 구현 계획" — also consistent. No defect here, but BRAND-RENAME-001 (a dependency, see memory) used 8 phases + hotfix; the reviewer should confirm 6-phase decomposition is sufficient given 18 [MODIFY] production files + entry-point wiring + lock semantics in P4. Probably ok; flagging for visibility only.

## Chain-of-Verification Pass

Second-look findings:

Re-read each section:
- §1.3 Non-Goals (spec.md:L50–L62): re-read fully — found D10 (backtick imbalance) recurs three times across spec.md:L52/178/292.
- §4 EARS (spec.md:L192–L227): re-checked every one of 18 entries individually; classification correct in all 18.
- acceptance.md Then-clauses: re-read all 5 main + 4 edge case scenarios — discovered D4 (broken `--exclude` semantics) which I would have missed on first scan.
- Traceability matrix: enumerated all 18 REQs against `REQ 매핑:` lines in acceptance.md. Discovered REQ-004, REQ-016, REQ-018 fully uncovered. First pass flagged this; second pass confirmed by exhaustive listing.
- Risks vs ACs: cross-checked plan.md R1–R12 against AC scenarios. R4 (third-party `goose` collision) maps weakly — no AC isolates the marker-absent path (D9). R6 (read-only fs) maps to AC-004 + REQ-013 — adequate. R8 (project-local `./.goose/` git-tracked) — no AC. Flagged as minor (subsumed by D2-class scope).
- Exclusions specificity: §3.5 has 11 items, §6 has 10 items, slightly redundant but each is concrete enough. No defect.
- Contradictions sweep: D5 (REQ-008 ↔ AC-001) is the only direct REQ↔AC contradiction.

New defects discovered on second pass: D4, D6, D9, D10 (rendering glitch), D11, D13.
Confirmed first-pass defects: D1, D2, D3, D5, D7, D8, D12.

## Regression Check

Iteration 1 — N/A (no prior iteration).

## Recommendation

This SPEC is structurally strong (PASS on all four must-pass criteria, high scores on Clarity/Completeness/Testability) but **FAILS overall on Traceability** (3 REQs uncovered) and has security/correctness defects in the acceptance test commands. Fix the following before re-audit:

1. **Add AC for REQ-MINK-UDM-004 (tmp prefix)**: New AC scenario asserting that after Phase 2, any tmp file created via `userpath.TempPrefix()` begins with `.mink-` and never `.goose-`. Binary test: e.g., `mink chat` writes a tmp file under `~/.mink/` whose basename matches `^\.mink-[a-zA-Z0-9]+$`.
2. **Add AC for REQ-MINK-UDM-016 (test file marker)**: New AC / Quality Gate row — `grep -rEn '"\\.goose' --include='*_test.go' --exclude='internal/userpath/legacy_test.go' | grep -v 'MINK migration fallback test' | wc -l` must equal 0.
3. **Add AC for REQ-MINK-UDM-018 (MINK_HOME bypass)**: New AC scenario — Given `~/.goose/` exists, `~/.mink/` absent, `MINK_HOME=/tmp/custom-mink` set; When `mink --version`; Then no migration occurs (`test -e ~/.goose/` remains, `! test -e ~/.mink/`), `userpath.UserHome()` returns `/tmp/custom-mink`, stderr migration notice 0 lines.
4. **Add AC for file-mode preservation (D6)**: After copy fallback (AC-004 scenario flipped to success path), verify `stat -f '%Lp' ~/.mink/permissions/grants.json` ≤ source mode and ≤ 0600. Cite OWASP path/file-permission concern explicitly.
5. **Fix grep commands in AC-005 (D4)**: Change `--exclude='internal/userpath/legacy.go'` to `--exclude='legacy.go'` AND combine with `--exclude-dir` or a `find -path` pipe. Suggested rewrite: `grep -rEn '"\\.goose' --include='*.go' --exclude='*_test.go' . | grep -v '^internal/userpath/legacy\\.go:' | wc -l` must equal 0. Apply the same fix to AC-005 #4 for `userpath.go`.
6. **Reconcile REQ-008 vs AC-001 bilingual notice (D5)**: Either tighten AC-001 #6 to require both Korean and English lines, or relax REQ-008 to "Korean or English". Then remove "(또는 동등 영문)" weasel phrase to satisfy AC-3 anchor.
7. **Split AC-004 by binary type (D8)**: Create AC-004a (CLI fail-fast non-zero exit) and AC-004b (daemon graceful degrade with warning).
8. **Add AC for brand-marker absent path (D9)**: Given `~/.goose/` exists without `.mink-managed` marker and without MINK-specific config field; When `mink --version`; Then migration proceeds, stderr emits exactly one additional warning line about best-effort brand verification.
9. **Fix backtick imbalance typos (D10)**: spec.md:L52 / L178 / L292 — change `**\`.moai/specs/* SPEC**` to `**\`.moai/specs/*\` SPEC**`.
10. **Standardize call-site count language (D11)**: Pick "18 files / 30+ literals" and update all references in spec.md §1.1, §2.4, §3.2 and plan.md §1, §2.

After fixes, re-submit for iteration 2. Expect MP-1..MP-4 to stay PASS; Traceability score should rise to 0.95+ and overall to ≥ 0.90.

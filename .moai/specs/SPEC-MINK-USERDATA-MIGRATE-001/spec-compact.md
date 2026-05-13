# SPEC-MINK-USERDATA-MIGRATE-001 — Compact

> Auto-extracted summary. Source: `spec.md` v0.1.3, `acceptance.md` v0.1.3, `plan.md` v0.1.2 (unchanged in this iteration).

## Identity

- **ID**: SPEC-MINK-USERDATA-MIGRATE-001
- **Status**: draft
- **Version**: 0.1.3 (plan-auditor iter 3 fast-track fix — ND3-1 AC-001 #6 brand 토큰 case-sensitivity + ND3-2 §7.1 stale AC count)
- **Priority**: High
- **Series**: MINK 리브랜드 3번째 SPEC (BRAND-RENAME → ENV-MIGRATE → 본 SPEC)
- **Depends on**: SPEC-MINK-BRAND-RENAME-001 (completed), SPEC-MINK-ENV-MIGRATE-001 (completed)

## Goal (1-line)

`~/.goose/` / `./.goose/` 디렉토리를 `~/.mink/` / `./.mink/` 로 자동 마이그레이션하고, 30+ literal occurrences across 18 distinct files 를 단일 `internal/userpath` 패키지로 수렴.

---

## EARS Requirements (19, v0.1.2 — REQ-019 EARS canonical 강화)

### Ubiquitous (6)

- **REQ-MINK-UDM-001** centralized path resolver `internal/userpath`
- **REQ-MINK-UDM-002** all production callsites use userpath functions
- **REQ-MINK-UDM-003** user-home = `~/.mink/`, project-local = `./.mink/`
- **REQ-MINK-UDM-004** tmp file prefix = `.mink-`
- **REQ-MINK-UDM-005** data schemas byte-identical across migration
- **REQ-MINK-UDM-006** `.goose` literal only in `internal/userpath/legacy.go`

### Event-Driven (4)

- **REQ-MINK-UDM-007** first-run detects `~/.goose/` and invokes `MigrateOnce()` once
- **REQ-MINK-UDM-008** post-migration stderr notice (Korean primary + optional English subtext; no `goose` word, contains `mink`/`밍크`)
- **REQ-MINK-UDM-009** rename fails → copy + SHA-256 verify + remove fallback
- **REQ-MINK-UDM-010** `ProjectLocal(cwd)` triggers lazy migration of `<cwd>/.goose/`

### State-Driven (3)

- **REQ-MINK-UDM-011** lock file blocks concurrent migration (max 30s)
- **REQ-MINK-UDM-012** both dirs exist → `~/.mink/` wins + warning
- **REQ-MINK-UDM-013** read-only fs → typed error + fallback read-only access

### Unwanted (3)

- **REQ-MINK-UDM-014** reject PRs with new hardcoded `~/.goose/` patterns
- **REQ-MINK-UDM-015** mid-copy crash → cleanup partial `~/.mink/`, preserve `~/.goose/`
- **REQ-MINK-UDM-016** test files using `.goose` literal must have migration-fallback marker

### Optional (2)

- **REQ-MINK-UDM-017** brand marker check (best-effort, warn if absent)
- **REQ-MINK-UDM-018** `MINK_HOME` env override skips auto-migration; empty / path-traversal / legacy-`.goose` / non-writable values rejected with typed error

### Ubiquitous — Security (1, new in v0.1.1; EARS canonical form in v0.1.2)

- **REQ-MINK-UDM-019** The system **shall** preserve source file mode bits (never weakened) in the destination across all copy fallback paths when migrating sensitive files. 민감 파일 0600, 디렉토리 0700 유지.

---

## Acceptance Criteria (12 main + 4 edge cases, v0.1.2)

### AC-MINK-UDM-001 — 자동 마이그레이션 + 일회성 알림
- GIVEN `~/.goose/` exists, `~/.mink/` absent, `MINK_HOME` unset
- WHEN `mink --version` 첫 실행
- THEN `~/.mink/` 생성 + `~/.goose/` 제거 + SHA-256 verified + 1줄 stderr 알림 (no `goose` word, contains `mink`/`밍크`; ND-1 fix — 예시 메시지에서 옛 경로 literal 인용 제거, "이전 디렉토리"/"새 MINK 디렉토리" prose 사용) + marker 파일 생성 + 멱등 보장

### AC-MINK-UDM-002 — 양쪽 디렉토리 동시 존재 시 `~/.mink/` 우선 + 경고
- GIVEN 양쪽 디렉토리 + marker 모두 존재
- WHEN `mink --version`
- THEN `~/.mink/` 우선 + 1줄 stderr warning + `~/.goose/` 보존

### AC-MINK-UDM-003 — Fresh install (옛 디렉토리 부재)
- GIVEN 둘 다 부재 + `MINK_HOME` 미설정
- WHEN `mink --help`
- THEN `~/.mink/` 0700 권한 신규 생성 + 알림 없음 + marker 부재 (stat 검증은 Linux `-c '%a'` / macOS `-f '%Lp'` dual notation)

### AC-MINK-UDM-004a — CLI fail-fast (D8 split)
- GIVEN disk space 부족 또는 read-only fs, binary = CLI (`mink`)
- WHEN `mink --version`
- THEN exit non-zero + `~/.goose/` byte-identical 보존 + 부분 `~/.mink/` cleanup + user-actionable error

### AC-MINK-UDM-004b — Daemon graceful degrade (D8 split)
- GIVEN 동일 fail 조건, binary = daemon (`minkd`)
- WHEN daemon startup
- THEN exit 0 (startup 계속) + warning log + read-only fallback + `~/.goose/` 보존

### AC-MINK-UDM-005 — Production source 의 `.goose` literal 0건 (legacy.go 예외)
- GIVEN 본 SPEC PR squash merge 완료
- WHEN `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` (D4 fix — post-filter pipeline; ND-8 fix — #1 descriptive → binary command)
- THEN 출력 = 0; legacy.go 의 매치 라인 = 1; brand-lint + build + vet + test 통과

### AC-MINK-UDM-006 — Tmp file prefix `.mink-` (REQ-004 trace, D1 fix)
- GIVEN Phase 2 완료
- WHEN `mink chat` 등 tmp file 작성 명령
- THEN 신규 tmp file basename 이 `^\.mink-[a-zA-Z0-9]+$` 매칭; `.goose-` prefix 0건

### AC-MINK-UDM-007 — 테스트 파일 marker 강제 (REQ-016 trace, D2 fix)
- GIVEN Phase 5 완료
- WHEN `grep -rEn '"\.goose' --include='*_test.go' . | grep -v '^./internal/userpath/legacy_test\.go:' | grep -v 'MINK migration fallback test' | wc -l`
- THEN 출력 = 0 (모든 test 파일의 `.goose` literal 은 marker comment 동반)

### AC-MINK-UDM-008a — `MINK_HOME` 정상 override (REQ-018 happy, D3 part 1)
- GIVEN `~/.goose/` exists, `MINK_HOME=/tmp/custom-mink` (writable)
- WHEN `mink --version`
- THEN `userpath.UserHome()` = `/tmp/custom-mink`; 자동 마이그레이션 0건; `~/.goose/` 보존; 알림 0건

### AC-MINK-UDM-008b — `MINK_HOME` 4 negative cases (REQ-018 boundary, D3 part 2 — security)
- Case 1 (empty string `MINK_HOME=""`): typed error, silent fallback 차단
- Case 2 (legacy `.goose` 경로 `MINK_HOME="$HOME/.goose"`): `ErrMinkHomeIsLegacyPath` reject
- Case 3 (path traversal `MINK_HOME=".../../etc/foo"`): `ErrMinkHomePathTraversal` reject (OWASP)
- Case 4 (non-writable `MINK_HOME=/tmp/ro-mink`): `ErrPermissionDenied` 또는 `ErrReadOnlyFilesystem`

### AC-MINK-UDM-009 — 파일 mode bits 보존 (REQ-019 trace, D6 fix — security gate)
- GIVEN `~/.goose/permissions/grants.json` mode 0600, copy fallback 강제
- WHEN `mink --version`
- THEN `~/.mink/permissions/grants.json` mode = 0600 (Linux `stat -c '%a'` / macOS `stat -f '%Lp'` 양쪽 검증); directory `~/.mink/permissions/` mode = 0700; mode weakened 0건

### AC-MINK-UDM-010 — Brand marker 부재 시 best-effort warning (D9 fix / R4 trace)
- GIVEN `~/.goose/` 존재하나 `.mink-managed` marker 부재 + MINK-specific config 키 부재
- WHEN `mink --version`
- THEN 마이그레이션 진행 + stderr 에 best-effort warning 1줄 추가 (`MINK_HOME` 안내 포함) + `~/.mink/.migrated-from-goose` 의 `brand_verified: false` 기록

### Edge Cases

- **EC-MINK-UDM-001**: Symlink 감지 시 graceful error, auto-resolve 안 함
- **EC-MINK-UDM-002**: 동시 실행 시 lock 획득, 정확히 1회 마이그레이션
- **EC-MINK-UDM-003**: Mid-copy crash 후 stale lock 정리 + retry
- **EC-MINK-UDM-004**: Project-local lazy migration per cwd

---

## Affected Files (Brownfield Delta Markers)

### [NEW] (5)

- `internal/userpath/userpath.go`
- `internal/userpath/migrate.go`
- `internal/userpath/legacy.go`
- `internal/userpath/errors.go`
- `internal/userpath/{userpath,migrate,legacy}_test.go`

### [MODIFY] Production (18 distinct files / 30+ literal occurrences)

- `internal/tools/web/search.go`
- `internal/tools/builtin/file/write.go`
- `internal/qmd/config.go`
- `internal/config/config.go`
- `internal/config/defaults.go`
- `internal/memory/config.go`
- `internal/cli/tui/sessionmenu/loader.go`
- `internal/cli/commands/audit.go`
- `internal/cli/session/session.go`
- `internal/cli/commands/messaging_telegram.go`
- `internal/mcp/credentials.go`
- `internal/cli/tui/model.go`
- `internal/audit/dual.go`
- `internal/command/adapter/aliasconfig/{merge,loader}.go`
- `internal/subagent/{memory,run}.go`
- `internal/ritual/scheduler/persist.go`
- `internal/permission/store/file.go`
- `internal/messaging/telegram/{config,store}.go`

### [MODIFY] Entry points + tests + docs

- `cmd/mink/main.go`, `cmd/minkd/main.go`
- `*_test.go` (전수 t.Setenv migration)
- `README.md`, `CHANGELOG.md`
- `.moai/project/structure.md`
- `scripts/check-brand.sh` (legacy.go exemption 추가 + REQ-016 test marker enforcement 추가)

### [NEW] Docs

- `.moai/project/codemaps/internal-userpath.md`

### [REMOVE]

- (없음)

---

## Exclusions (What NOT to Build)

1. `.moai/specs/*` SPEC 디렉토리 변경 (이미 MINK 네이밍 또는 PRESERVE) — D10 backtick fix
2. third-party `goose` (Block AI) CLI path 충돌 보호 (사용자 책임, AC-010 best-effort warning 만 제공)
3. 데이터 schema migration (디렉토리만 이동)
4. rollback / uninstall CLI 명령 (단방향 migration)
5. logo / visual asset (BRAND-RENAME OUT 계승)
6. env var loader 재작업 (ENV-MIGRATE-001 결과 소비)
7. CHANGELOG / SPEC HISTORY rows 수정 (immutable)
8. multi-user 분산 lock (단일 lock file 만)
9. symlink target 자동 resolve 후 migrate
10. SPEC-MINK-PRODUCT-V7-001 / SPEC-MINK-DISTANCING-STATEMENT-001 (별도 SPEC)
11. third-party plugin/extension hardcoded path 보호

---

## Reference Implementation Pattern

`internal/envalias` (ENV-MIGRATE-001 PR #171, main `0c24237`):
- 22-key alias map + 단일 resolver 함수
- 11 production + 33 test callsite migrated
- backward-compat 우선 정책

본 SPEC 의 `internal/userpath` 는 동일 패턴을 채택하되:
- read-only 가 아닌 fs mutation 동반
- verify-before-remove 정책 강제 (REQ-009)
- mode bits 보존 강제 (REQ-019, v0.1.1 신설 / v0.1.2 EARS canonical)
- `MINK_HOME` 경계 검증 강제 (REQ-018, v0.1.1)
- 동시성/락/부분실패 edge case 추가

---

## Defect Resolution Map

### v0.1.1 (iter 1 — D1 ~ D13)

| Defect | Location | Fix |
|--------|----------|-----|
| D1 | REQ-004 traceability gap | AC-006 신설 (tmp prefix `.mink-` 검증) |
| D2 | REQ-016 traceability gap | AC-007 신설 (test file marker enforcement) |
| D3 | REQ-018 traceability gap (security boundary) | AC-008a happy + AC-008b 4 negative cases (empty/legacy/traversal/non-writable) |
| D4 | `grep --exclude=<path>` basename glob 한계 | post-filter pipeline (`| grep -v '^./internal/userpath/legacy\.go:'`) |
| D5 | REQ-008 vs AC-001 #6 contradiction | REQ-008 "Korean with optional English subtext" 단일 source; AC-001 #6 weasel 제거 |
| D6 | mode bits 보존 미명시 | REQ-019 신설 + AC-009 stat 검증 (Linux/macOS dual) |
| D7 | AC-001 #6 weasel "또는 동등 영문" | byte-precise: `goose` 미포함 + `mink`/`밍크` 포함 grep gate |
| D8 | AC-004 CLI/daemon mixed | AC-004a (CLI fail-fast) + AC-004b (daemon graceful degrade) 분리 |
| D9 | R4 brand marker absent 미검증 | AC-010 신설 (best-effort warning + brand_verified flag) |
| D10 | spec.md L52/L178/L292 backtick imbalance | `` `.moai/specs/*` SPEC `` 3개소 정정 |
| D11 | "30+ 콜사이트" vs "18 files" 모호 | "30+ callsites across 18 distinct files" 일관 표기 |
| D12 | `stat -c` GNU-only | Linux `stat -c '%a'` / macOS `stat -f '%Lp'` dual notation, portable fallback `ls -ld` |
| D13 | D5/D7 family | REQ-008 + AC-001 #6 동시 정정 |

### v0.1.2 (iter 2 — ND-1 ~ ND-8)

| Defect | Severity | Location | Fix |
|--------|----------|----------|-----|
| ND-1 | major (blocking) | acceptance.md AC-001 #6 — gate vs 예시 self-contradiction | 예시 메시지에서 옛 경로 literal `~/.goose`/`~/.mink` 인용 제거. "이전 디렉토리"/"새 MINK 디렉토리" prose 사용. gate `grep -c 'goose' = 0` binary 유지. |
| ND-2 | minor | plan.md R1 stale phrase | "30+ 콜사이트" → "30+ literal occurrences across 18 distinct files" (spec.md / spec-compact.md 와 정합). |
| ND-3 | minor | acceptance.md DoD "11 main scenarios" | "12 main scenarios" + 12 AC enumerate (AC-001/002/003/004a/004b/005/006/007/008a/008b/009/010). |
| ND-4 | minor | spec.md §1.1 stale "11+ binary-verifiable" | "12 binary-verifiable". |
| ND-5 | minor | acceptance.md DoD "Quality gate 13개" | "14개" + 14 rows enumerate. |
| ND-6 | minor | spec.md §7.1 self-reference v0.1.0 | v0.1.2 sync. |
| ND-7 | minor | REQ-019 EARS form weak (Korean prose only) | Canonical Ubiquitous Security form: "The system **shall** preserve source file mode bits (never weakened) ..." (English first), Korean rendering 병기. |
| ND-8 | minor | AC-005 #1 descriptive (not binary) | Binary grep pipeline `... \| wc -l = 0` form 으로 전환. |

---

Version: 0.1.2
Status: draft
Last Updated: 2026-05-13

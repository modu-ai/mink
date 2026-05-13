# SPEC-MINK-USERDATA-MIGRATE-001 — Task Decomposition

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-13 | manager-strategy | 초안. plan.md v0.1.2 의 6-phase 계획을 19개 atomic task 로 분해. 각 task = 단일 TDD RED→GREEN→REFACTOR 사이클 = 단일 logical commit. REQ × AC traceability 매트릭스 동봉. |

---

## 1. Task Table

각 task 는 (a) 단일 TDD 사이클 (또는 brownfield TDD: Pre-RED → RED → GREEN → REFACTOR) 안에서 완료 가능한 atomic 단위이며, (b) 단일 logical commit 으로 마무리된다. Phase 별로 정렬되어 있으며, Status 는 `pending` 으로 시작한다.

| Task ID | Phase | Description | Requirements | Acceptance Criteria | Dependencies | Planned Files | Status |
|---------|-------|-------------|--------------|---------------------|--------------|---------------|--------|
| T-001 | P1 | `internal/userpath/errors.go` — typed errors 정의 (`ErrReadOnlyFilesystem`, `ErrPermissionDenied`, `ErrLockTimeout`, `ErrMinkHomeEmpty`, `ErrMinkHomeIsLegacyPath`, `ErrMinkHomePathTraversal`). 각 에러는 `errors.Is` 친화적 sentinel 또는 wrap 패턴. | REQ-013, REQ-018 | AC-008b (4 cases) | — | `internal/userpath/errors.go`, `internal/userpath/errors_test.go` | pending |
| T-002 | P1 | `internal/userpath/userpath.go` resolver 4종 — `UserHome()`, `ProjectLocal(cwd)`, `SubDir(name)`, `TempPrefix()`. `envalias.DefaultGet("HOME")` 우선 + `~/.mink/` fallback. **MINK_HOME 입력 검증 (4 negative cases)** 포함: empty / legacy `.goose` 경로 / path traversal (`..`) / non-writable. 모두 typed error 로 reject (silent fallback 금지). | REQ-001, REQ-003, REQ-018 | AC-003 (fresh install), AC-008a (happy path), AC-008b (4 negatives) | T-001 | `internal/userpath/userpath.go`, `internal/userpath/userpath_test.go` | pending |
| T-003 | P1 | `internal/userpath/legacy.go` — 단일 `LegacyHome()` 함수가 `.goose` 리터럴의 유일한 잔존 위치. brand-lint exemption target. | REQ-006 | AC-005 #2 (legacy.go 1 라인 매치) | T-001 | `internal/userpath/legacy.go`, `internal/userpath/legacy_test.go` | pending |
| T-004 | P1 | `internal/userpath/migrate.go` — `MigrationResult` struct + `MigrateOnce(ctx)` 코어. 감지 → atomic `os.Rename` → 성공 시 marker 파일 작성 + stderr 알림. 첫 시점에는 fallback / lock 미구현 (T-005, T-006 에서 추가). `sync.Once` 로 process-level 멱등 보장. | REQ-007, REQ-008 | AC-001 (자동 마이그레이션 + 알림) | T-001, T-002 | `internal/userpath/migrate.go`, `internal/userpath/migrate_test.go` | pending |
| T-005 | P1 | `MigrateOnce` 의 cross-filesystem copy fallback 경로 — `EXDEV` 감지 시 `io.Copy` + SHA-256 verify + source 삭제. **mode bits 보존 (REQ-019)**: `os.Chmod(dst, srcInfo.Mode().Perm())` 호출 + 0600 source → 0600 dest 시나리오 강제 검증. mid-copy 실패 시 `~/.mink/` 부분 cleanup + source `~/.goose/` 그대로. | REQ-009, REQ-015, REQ-019 | AC-004a (CLI fail-fast), AC-009 (mode bits) | T-004 | `internal/userpath/migrate.go`, `internal/userpath/migrate_test.go` | pending |
| T-006 | P1 | `MigrateOnce` 의 lock + recovery — `~/.mink/.migration.lock` (PID + timestamp). 동시 호출 시 blocking wait (max 30s) + stale lock 감지 (`os.Kill(pid, 0)` returns error → cleanup + retry). | REQ-011, REQ-015 | EC-002 (concurrent), EC-003 (mid-copy crash recovery) | T-004, T-005 | `internal/userpath/migrate.go` (확장), `internal/userpath/migrate_test.go` (확장) | pending |
| T-007 | P1 | Dual-existence state + symlink detection — 양쪽 디렉토리 동시 존재 시 `~/.mink/` 우선 + 1회 warning (REQ-012). `~/.goose` 가 symlink 인 경우 (`os.Lstat`) graceful error 반환 + 자동 resolve 0건 (R3). best-effort warning when no brand marker (REQ-017 negative). | REQ-012, REQ-017 | AC-002 (dual existence), AC-010 (no brand marker warning), EC-001 (symlink) | T-004 | `internal/userpath/migrate.go` (확장), `internal/userpath/migrate_test.go` (확장) | pending |
| T-008 | P2 | Migrate `internal/config/*.go` 콜사이트 (3 literals: config.go L230/L283/L285, defaults.go L42-47) → `userpath.UserHome()` / `userpath.ProjectLocal(cwd)`. 가장 fan_in 이 높은 영역 — 이걸 먼저 끝내야 후속 패키지가 `config.Load()` 결과 의존성을 통해 자연 전파. characterization test 로 기존 path 결과 보존 검증. | REQ-002, REQ-014 | AC-005 (post-migration grep gate) | T-002, T-003 | `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/config_test.go` | pending |
| T-009 | P2 | Migrate `internal/audit/dual.go` (L150, L157 — global + local audit log paths). `internal/cli/commands/audit.go` (L90, L122 — flag default). 이미 envalias 사용 중인 부분과의 정합 유지. | REQ-002, REQ-014 | AC-005 | T-002 | `internal/audit/dual.go`, `internal/cli/commands/audit.go`, `internal/audit/dual_test.go` | pending |
| T-010 | P2 | Migrate session subsystem — `internal/cli/session/session.go` (L39 default, L42 home-based, L62 tmp prefix `.goose-session-*` → `.mink-session-*`). `internal/cli/tui/sessionmenu/loader.go` (L60). `userpath.TempPrefix()` 사용. | REQ-002, REQ-004, REQ-014 | AC-005, AC-006 (tmp prefix) | T-002 | `internal/cli/session/session.go`, `internal/cli/tui/sessionmenu/loader.go`, 관련 `_test.go` | pending |
| T-011 | P2 | Migrate permissions + MCP credentials — `internal/permission/store/file.go` (L98), `internal/cli/tui/model.go` (L168), `internal/mcp/credentials.go` (L15 `credentialsDir` const). 0600 mode preservation 검증은 T-005 의 unit test 가 cover, 본 task 는 path resolver 만 교체. | REQ-002, REQ-014 | AC-005 | T-002 | `internal/permission/store/file.go`, `internal/cli/tui/model.go`, `internal/mcp/credentials.go`, 관련 `_test.go` | pending |
| T-012 | P2 | Migrate memory / ritual / subagent — `internal/memory/config.go` (L43), `internal/ritual/scheduler/persist.go` (L41), `internal/subagent/{memory.go L33-35, run.go L506}`. subagent 의 scope-user / scope-project / scope-local 3종은 각각 `UserHome()` / `ProjectLocal(projectRoot)` 매핑 결정 필요 (characterization test 로 동일 결과 검증). | REQ-002, REQ-014 | AC-005 | T-002 | `internal/memory/config.go`, `internal/ritual/scheduler/persist.go`, `internal/subagent/memory.go`, `internal/subagent/run.go`, 관련 `_test.go` | pending |
| T-013 | P2 | Migrate messaging + alias config + tools — `internal/messaging/telegram/{config.go L167, store.go L249}` (string concat 패턴: `home + "/.goose/..."` → `filepath.Join(userpath.UserHome(), ...)`). `internal/command/adapter/aliasconfig/{loader.go L30/L169, merge.go L39}`. `internal/tools/web/search.go` (L429), `internal/tools/builtin/file/write.go` (L82 `.goose-write-*` tmp prefix → `userpath.TempPrefix()`). `internal/cli/commands/messaging_telegram.go` (L326). | REQ-002, REQ-004, REQ-014 | AC-005, AC-006 | T-002 | `internal/messaging/telegram/config.go`, `internal/messaging/telegram/store.go`, `internal/command/adapter/aliasconfig/loader.go`, `internal/command/adapter/aliasconfig/merge.go`, `internal/tools/web/search.go`, `internal/tools/builtin/file/write.go`, `internal/cli/commands/messaging_telegram.go`, 관련 `_test.go` | pending |
| T-014 | P2 | Migrate qmd project-local paths — `internal/qmd/config.go` (L55-69, 7 literals: IndexPath/ModelsPath + 5 subdirs `memory/context/skills/tasks/rituals`). 모두 project-local → `userpath.ProjectLocal(cwd)` + subdirectory 조합. | REQ-002, REQ-010, REQ-014 | AC-005, EC-004 (project-local lazy migration) | T-002 | `internal/qmd/config.go`, `internal/qmd/config_test.go` | pending |
| T-015 | P3 | CLI 진입점 wiring (`cmd/mink/main.go` 또는 `internal/cli/run.go`) — `userpath.MigrateOnce(ctx)` 1회 호출 + **fail-fast on error**. `MigrationResult.Migrated == true` 시 stderr 알림 (Korean primary + optional English subtext). 알림 메시지는 `goose` 단어 포함 금지 + `mink` 또는 `밍크` 토큰 포함. | REQ-007, REQ-008, REQ-013 | AC-001 (full path), AC-004a (CLI fail-fast) | T-002, T-004, T-005, T-006, T-007 | `cmd/mink/main.go` 또는 `internal/cli/run.go`, `internal/cli/*_test.go` 또는 신규 `cmd/mink/main_test.go` | pending |
| T-016 | P3 | Daemon 진입점 wiring (`cmd/minkd/main.go` `runWithContext` 시작 시점) — `userpath.MigrateOnce(ctx)` 호출 + **graceful degrade on error**. 13-step lifecycle 의 step 0.5 (config Load 이전) 또는 step 1 직후 (config 의존성 검토 후) 배치. daemon log warning + read-only fallback. | REQ-007, REQ-013 | AC-004b (daemon graceful degrade) | T-002, T-004, T-005, T-006, T-007 | `cmd/minkd/main.go`, `cmd/minkd/integration_test.go` | pending |
| T-017 | P5 | Migrate 23 test files — `GOOSE_HOME` 사용 → `t.Setenv("MINK_HOME", t.TempDir())`, `.goose` 리터럴 → `t.TempDir()` + `userpath` 호출. `internal/userpath/legacy_test.go` 와 `// MINK migration fallback test` 주석 동반 케이스만 예외. ENV-MIGRATE-001 PR #171 의 33-callsite 패턴 그대로 적용. | REQ-016 | AC-007 (test marker enforcement) | T-008, T-009, T-010, T-011, T-012, T-013, T-014 | 23개 `*_test.go` 파일 (목록은 P5 시점 grep) | pending |
| T-018 | P6 | brand-lint 갱신 — `scripts/check-brand.sh` 에 (a) `.go` 파일 검사 라인 추가, (b) `internal/userpath/legacy.go` / `internal/userpath/legacy_test.go` exemption, (c) test marker enforcement (AC-007) gate 라인 추가. exit 0/1 boundary 검증 unit test. | REQ-006, REQ-016 | AC-005 #5 (brand-lint), AC-007 #4 (CI gate) | T-003, T-017 | `scripts/check-brand.sh`, (선택) `scripts/check-brand_test.sh` | pending |
| T-019 | P6 | Docs + CHANGELOG — `README.md` 의 `## Migration from .goose` section, `CHANGELOG.md` `[Unreleased]` BREAKING 마커, `.moai/project/structure.md` `internal/userpath` 등록, `.moai/project/codemaps/internal-userpath.md` 신규. spec.md HISTORY 에 v0.2.0 implemented row 추가. | (docs traceability) | (n/a — sync phase) | T-001~T-018 | `README.md`, `CHANGELOG.md`, `.moai/project/structure.md`, `.moai/project/codemaps/internal-userpath.md`, `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/spec.md` (HISTORY only) | pending |

총 19 atomic tasks. 권장 분포: P1=7, P2=7, P3=2, P5=1 (브렐드+grouping), P6=2. P4 는 T-005~T-007 에 흡수 (별도 `recovery.go` 신설 없이 `migrate.go` 안에 lock+recovery 통합 — over-engineering check §3 참조).

---

## 2. Coverage Cross-Check Matrix

### 2.1 EARS Requirement Coverage (19 REQs)

| Requirement | Task IDs |
|-------------|----------|
| REQ-MINK-UDM-001 (`internal/userpath` exists) | T-002, T-003, T-004 |
| REQ-MINK-UDM-002 (all callsites via userpath) | T-008, T-009, T-010, T-011, T-012, T-013, T-014 |
| REQ-MINK-UDM-003 (`~/.mink/` is home) | T-002, T-004 |
| REQ-MINK-UDM-004 (tmp prefix `.mink-`) | T-002 (`TempPrefix()`), T-010 (session tmp), T-013 (file write tmp) |
| REQ-MINK-UDM-005 (data schema unchanged, byte-identical) | T-004, T-005 (SHA-256 verify) |
| REQ-MINK-UDM-006 (`.goose` literal only in `legacy.go`) | T-003, T-018 |
| REQ-MINK-UDM-007 (MigrateOnce invoked at startup) | T-004, T-015, T-016 |
| REQ-MINK-UDM-008 (one-time stderr notice, Korean + optional English) | T-004, T-015 |
| REQ-MINK-UDM-009 (copy fallback with SHA-256 verify) | T-005 |
| REQ-MINK-UDM-010 (project-local lazy migration) | T-007 (in MigrateOnce branch) or T-014 (qmd-specific) — T-002 의 `ProjectLocal` 이 lazy 호출 진입점 |
| REQ-MINK-UDM-011 (lock + 30s blocking wait) | T-006 |
| REQ-MINK-UDM-012 (dual existence → `~/.mink/` wins) | T-007 |
| REQ-MINK-UDM-013 (typed error contract + read-only fallback) | T-001, T-005, T-007, T-015, T-016 |
| REQ-MINK-UDM-014 (review rejection rule, code-review level) | T-008~T-014 (모든 production migration), T-018 (brand-lint enforcement) |
| REQ-MINK-UDM-015 (partial cleanup on mid-copy failure) | T-005, T-006 |
| REQ-MINK-UDM-016 (test file marker enforcement) | T-017, T-018 |
| REQ-MINK-UDM-017 (brand marker best-effort) | T-007 |
| REQ-MINK-UDM-018 (MINK_HOME override + 4 negatives) | T-001, T-002 |
| REQ-MINK-UDM-019 (mode bits preservation) | T-005 |

모든 19 REQ 에 최소 1개 task 매핑. uncovered REQ = 0.

### 2.2 Acceptance Criteria Coverage (12 main + 4 edge = 16)

| AC | Task IDs |
|----|----------|
| AC-MINK-UDM-001 (자동 마이그레이션 + 알림) | T-004 (코어), T-015 (CLI wiring) |
| AC-MINK-UDM-002 (양쪽 동시 존재 → `~/.mink/` 우선) | T-007 |
| AC-MINK-UDM-003 (Fresh install) | T-002 (resolver), T-015/T-016 (no-op startup) |
| AC-MINK-UDM-004a (CLI fail-fast on migration error) | T-005 (cleanup), T-015 (fail-fast policy) |
| AC-MINK-UDM-004b (daemon graceful degrade) | T-016 |
| AC-MINK-UDM-005 (production `.goose` literal = 0건 except legacy.go) | T-003, T-008~T-014, T-018 |
| AC-MINK-UDM-006 (tmp prefix `.mink-`) | T-010, T-013 (`TempPrefix()` 호출 callsite), T-002 (정의) |
| AC-MINK-UDM-007 (test marker enforcement) | T-017, T-018 (CI gate) |
| AC-MINK-UDM-008a (MINK_HOME happy path) | T-002 |
| AC-MINK-UDM-008b (MINK_HOME 4 negatives) | T-001 (typed errors), T-002 (validation) |
| AC-MINK-UDM-009 (mode bits 0600/0700 preserved on copy fallback) | T-005 |
| AC-MINK-UDM-010 (brand marker best-effort warning) | T-007 |
| EC-MINK-UDM-001 (symlink graceful error) | T-007 |
| EC-MINK-UDM-002 (concurrent migration via lock) | T-006 |
| EC-MINK-UDM-003 (mid-copy crash recovery on next run) | T-006 |
| EC-MINK-UDM-004 (project-local lazy migration) | T-014 (qmd-specific), T-007 (general `ProjectLocal` path) |

모든 16 AC 에 최소 1개 task 매핑. uncovered AC = 0.

### 2.3 Dependency Graph

```
T-001 (errors)
  ↓
T-002 (resolvers)           T-003 (legacy.go)
  ↓                            ↓
T-004 (MigrateOnce core)  ────┘
  ↓ ↓ ↓
  T-005 (copy fallback + mode bits)
       ↓
  T-006 (lock + recovery)
       ↓
  T-007 (dual + symlink + brand marker)
       ↓
  T-008 ~ T-014 (P2 production migration, 병렬화 가능)
       ↓
  T-015 (CLI wiring) ─ T-016 (daemon wiring) [병렬]
       ↓
  T-017 (test migration)
       ↓
  T-018 (brand-lint) ─ T-019 (docs) [병렬]
```

P2 (T-008~T-014) 는 서로 독립이므로 병렬 가능. 단, T-008 (config) 가 다른 패키지의 `config.Load()` 결과를 통해 path 를 받는 경우가 있어 **T-008 을 먼저 끝내는 것이 안전** (그러면 후속 T-009~T-014 에서 stale config 가 잡혀 오류 발생을 막을 수 있음).

P3 (T-015, T-016) 는 P1 + P2 의 모든 task 가 끝나야 시작 가능 — `userpath.MigrateOnce` 의 모든 branch 가 안정화된 후에 wiring.

P5 (T-017) 는 P2 가 끝난 후 — test 마이그레이션 시점에 production 이 이미 `userpath` 를 호출하면 `t.Setenv("MINK_HOME", t.TempDir())` 패턴이 자연 동작.

P6 (T-018, T-019) 는 모든 production + test 변경이 끝난 후 — brand-lint 가 final state 를 검증.

---

## 3. Brownfield TDD Discipline

본 SPEC 은 **mixed pattern**:

- **[NEW] tasks (T-001~T-007, T-018, T-019 신설 부분)**: 순수 TDD (RED → GREEN → REFACTOR). 테스트 먼저 작성, 그 후 implementation. `t.TempDir()` + `t.Setenv("MINK_HOME", ...)` 로 fs 격리.
- **[MODIFY] tasks (T-008~T-014, T-017)**: brownfield TDD —
  1. **Pre-RED**: 기존 콜사이트 read (Grep / Read tool) → 현재 path 구성 패턴 파악
  2. **RED**: 새 `userpath.X()` 호출을 가정한 failing test 작성 (path 가 `t.TempDir()` 기반으로 변할 것을 기대)
  3. **GREEN**: Edit 으로 `filepath.Join(homeDir, ".goose", ...)` → `filepath.Join(userpath.UserHome(), ...)` 치환
  4. **REFACTOR**: 필요 시 도메인별 helper 추출 (예: `internal/audit` 의 audit log path 빌더), 단 scope creep 금지

---

## 4. Task ID Format

- T-NNN where NNN ∈ {001..019}
- 각 task 는 단일 commit message subject 에 매핑: `feat(userpath): SPEC-MINK-USERDATA-MIGRATE-001 T-NNN — <description>`
- `REQ:` trailer 에 task ID 의 REQ 매핑 컬럼 그대로 인용

---

Version: 0.1.0
Status: draft
Last Updated: 2026-05-13

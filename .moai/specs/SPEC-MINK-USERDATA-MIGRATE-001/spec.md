---
id: SPEC-MINK-USERDATA-MIGRATE-001
version: "0.1.3"
status: draft
created_at: 2026-05-13
updated_at: 2026-05-13
author: manager-spec
priority: High
labels: [brand, userdata, migration, brownfield, cross-cutting, path-resolver]
issue_number: null
depends_on: [SPEC-MINK-BRAND-RENAME-001, SPEC-MINK-ENV-MIGRATE-001]
related_specs: [SPEC-MINK-PRODUCT-V7-001, SPEC-MINK-DISTANCING-STATEMENT-001]
---

# SPEC-MINK-USERDATA-MIGRATE-001 — 사용자 데이터 디렉토리 마이그레이션 (`~/.goose/` → `~/.mink/`, `./.goose/` → `./.mink/`)

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-13 | manager-spec | 초안 작성. MINK 리브랜드 시리즈의 **3번째 SPEC** (BRAND-RENAME-001 v0.2.0 completed → ENV-MIGRATE-001 v0.2.0 completed → 본 SPEC). 자연스러운 진행 순서: brand identity → env vars → on-disk paths. 사용자 1라운드 인터뷰로 확정한 3개 결정사항 반영 — (A) `~/.goose/` 와 `./.goose/` 양쪽 모두 마이그레이션 in-scope, (B) 첫 실행 시 자동 마이그레이션 + 일회성 알림, (C) 중앙 path resolver 패키지 (`internal/userpath`) 신규 도입. ENV-MIGRATE-001 의 `internal/envalias` alias loader 패턴을 reference 로 채택 (alias map + central resolver). 30+ callsites across 18 distinct files 확인 + brownfield delta marker 적용. 18 EARS 요구사항 + 5 AC + 6-phase 구현 계획. |
| 0.1.1 | 2026-05-13 | manager-spec | plan-auditor iter 1 fix — 13 defects addressed (D1-D13). 보안 관련 REQ-019 신설 (mode bits 보존), AC-001/004 분리 (AC-004a CLI fail-fast / AC-004b daemon graceful degrade), MINK_HOME 경계 검증 보강 (AC-008a happy path + AC-008b non-writable / existing .goose / path traversal / empty string). REQ-008 단일 source 정정 (Korean with optional English subtext), AC-001 #6 weasel 워드 제거. AC-005 의 `grep --exclude=<path>` 형식 오류 fix (basename glob only — post-filter pipeline 으로 전환). AC-003 stat 명령 macOS/Linux dual notation. spec.md L52/L178/L292 backtick imbalance 정정. "30+ 콜사이트" → "30+ callsites across 18 distinct files" 일관 표기. AC count 5 main + 4 edge → 12 main + 4 edge 로 확장 (AC-004 split 포함, REQ-004/016/018/019 + brand marker). |
| 0.1.3 | 2026-05-13 | MoAI orchestrator | plan-auditor iter 3 fast-track fix — 2 residual defects (ND3-1, ND3-2). ND3-1 (blocking): AC-001 #6 예시 메시지에 한국어 토큰 `밍크` + 소문자 `mink` 포함하도록 정정하여 gate `grep -Ec 'mink|밍크' ≥ 1` 와 정합 (case-sensitivity mole-whack). ND3-2 (minor): spec.md §7.1 self-reference "5 scenarios" → "12 main + 4 edge scenarios" 동기화. plan-auditor 자체 권고에 따른 fast-track (1-line edit ×2). |
| 0.1.2 | 2026-05-13 | manager-spec | plan-auditor iter 2 fix — 8 new defects (ND-1 ~ ND-8) addressed. AC-001 #6 gate 와 예시 메시지 정합화 (ND-1: 예시에서 `~/.goose`/`~/.mink` 경로 인용을 "이전 디렉토리"/"새 디렉토리" 로 prose 치환, gate `grep -c 'goose' = 0` binary 유지), DoD count 동기화 (ND-3: 11 → 12 main scenarios, ND-5: 13 → 14 quality gate rows), spec.md L44 stale "11+" → "12" 정정 (ND-4), spec.md L314 self-version v0.1.0 → v0.1.2 sync (ND-6), REQ-019 EARS canonical 강화 (ND-7: "The system **shall** preserve source file mode bits ..." Ubiquitous Security form), AC-005 #1 descriptive → binary grep command 로 전환 (ND-8). plan.md L203 R1 "30+ 콜사이트" 잔존 문구 정정 (ND-2). spec-compact.md 재생성. |

---

## 1. Overview

### 1.1 Scope Clarity

본 SPEC 은 **brownfield 메타-SPEC** 으로, 사용자 데이터 파일시스템 레이아웃을 `~/.goose/` / `./.goose/` 에서 `~/.mink/` / `./.mink/` 로 마이그레이션한다. 코드 식별자나 환경 변수가 아닌, **실제 디스크 경로** 가 대상이다.

본 SPEC 은 MINK 리브랜드 시리즈의 자연스러운 마지막 layer:

1. **SPEC-MINK-BRAND-RENAME-001** (v0.2.0, completed 2026-05-13) — brand identity `goose` → `MINK`. binary, env namespace 토큰, prose 산문.
2. **SPEC-MINK-ENV-MIGRATE-001** (v0.2.0, completed 2026-05-13) — env var alias loader (`internal/envalias` 22-key map, 11 production + 33 test callsite).
3. **SPEC-MINK-USERDATA-MIGRATE-001** (본 SPEC) — on-disk path 마이그레이션.

명세는 다음을 포함한다:
- 단일 신규 패키지 `internal/userpath` 도입 (§3.1)
- 30+ callsites across 18 distinct files 마이그레이션 (§3.2)
- 첫 실행 자동 마이그레이션 로직 (§3.3)
- Dual-read fallback semantics (§3.4)
- 19 EARS 요구사항 (§4)
- 12 binary-verifiable Acceptance Criteria (acceptance.md) + 4 edge cases
- 6-phase 구현 계획 (plan.md)

### 1.2 Goal

`~/.goose/` (user-home) 및 `./.goose/` (project-local) 디렉토리의 실제 파일·하위 디렉토리를 `~/.mink/` / `./.mink/` 로 이전한다. 이전 후 30+ callsites across 18 distinct files 는 중앙 path resolver (`internal/userpath`) 의 단일 함수 호출로 수렴한다. 첫 실행 시 `~/.goose/` 존재가 감지되면 atomic rename (또는 copy + remove) 후 일회성 stderr 알림을 출력한다. 본 SPEC 머지 후 기존 사용자는 `~/.goose/` 데이터를 잃지 않으며, 신규 사용자는 `~/.mink/` 만 보고 자란다.

### 1.3 Non-Goals

- **`.moai/specs/*` SPEC 디렉토리 변경** — 이미 MINK 네이밍이거나 (`SPEC-MINK-*`) preserve 대상 (`SPEC-GOOSE-*`, BRAND-RENAME §3.2 item 1). 본 SPEC 은 사용자 데이터 디렉토리 (런타임 mutable state) 만 다룸.
- **CHANGELOG / git history / SPEC HISTORY rows** — 모두 immutable, BRAND-RENAME OUT-scope 계승.
- **`.moai/brain/`, `.claude/agent-memory/`, `.moai/project/`** — 프로젝트 메타데이터, 본 SPEC scope 외 (이미 MINK 네이밍 또는 PRESERVE).
- **env var loader 재작업** — `internal/envalias` 가 ENV-MIGRATE-001 에서 이미 완성. 본 SPEC 은 path resolver 만 신설하고 env loader 와는 독립.
- **CLI binary 자체 rename** — BRAND-RENAME-001 Phase 5 에서 이미 `cmd/mink/`, `cmd/minkd/` 로 전환 완료.
- **third-party CLI tool 외부 의존 보호** — `goose` (Block AI) 같은 third-party 가 우연히 `~/.goose/` 를 가리키더라도 본 SPEC 은 안전망을 제공하지 않음. 양쪽 brand 가 동일 경로를 사용한다는 외부 가정은 사용자 책임. 본 SPEC 마이그레이션은 MINK 가 만든 `~/.goose/` 만 대상으로 함 (가능하면 brand marker file 로 식별).
- **데이터 스키마 마이그레이션** — `memory.db`, `permissions/grants.json`, `telegram.db` 등의 내부 schema 는 변경 0건. 디렉토리 위치만 이동.
- **legacy `goose` binary 와의 동시 실행 호환성** — 본 SPEC 머지 후 사용자는 `mink` 만 실행 가능 (BRAND-RENAME 결과). 옛 `goose` binary 가 디스크에 남아 있어도 본 SPEC scope 외.
- **symlink-aware migration** — `~/.goose` 가 symlink 인 경우 fallback 정책만 정의, 자동 resolve 후 마이그레이션은 OUT (R3 참조).
- **여러 user account 동시 마이그레이션** — multi-user 환경에서 동시성 보호는 단일 lock file 만, 본격적 분산 lock 은 OUT.
- **uninstaller / rollback CLI** — `mink unmigrate` 같은 명령은 OUT. 마이그레이션은 단방향. 단, 본 SPEC 머지 직후 사용자가 수동으로 `mv ~/.mink ~/.goose` 하더라도 다음 실행 시 다시 마이그레이션 되는 멱등 동작은 보장.

### 1.4 Series 의존성

본 SPEC 은 두 선행 SPEC 에 **의존** 하지만 **supersede** 하지는 않는다:

- `SPEC-MINK-BRAND-RENAME-001` (completed) — binary 이름 `mink` 가 존재한다는 전제. 본 SPEC 의 first-run detection 은 binary `mink` 가 실행될 때 작동.
- `SPEC-MINK-ENV-MIGRATE-001` (completed) — `MINK_HOME` 등 env var 가 `internal/envalias` 로 resolve 된다는 전제. `userpath.UserHome()` 은 `envalias.Get("MINK_HOME")` 을 우선 사용하고 미설정 시 `~/.mink/` 로 fallback.

reference 인용 (alias loader 패턴): `internal/envalias/loader.go` (ENV-MIGRATE-001 PR #171 머지, main commit `0c24237`).

---

## 2. Background

### 2.1 왜 path 마이그레이션이 필요한가

BRAND-RENAME-001 §1.3 NOTE (brand-runtime split window) 가 명시한 일시적 부정합:

> binary `mink` 의 brand-position 출력은 모두 MINK 이지만 runtime 에서는 (a) `GOOSE_*` env vars 21개 를 읽고 (b) `./.goose/` / `~/.goose/` workspace path 117 occurrence 를 사용한다.

ENV-MIGRATE-001 머지로 (a) 항목은 해소되었다 (alias loader 도입, `MINK_*` 우선 + `GOOSE_*` deprecated). 본 SPEC 은 (b) 항목을 해소한다. 머지 후 `mink` binary 는 100% MINK-named runtime state 를 사용한다.

### 2.2 왜 자동 마이그레이션인가

사용자 인터뷰 결정사항 (B): 첫 실행 시 `~/.goose/` 감지 → 자동 마이그레이션 + 일회성 알림. 근거:

1. **Backward-compat 우선**: ENV-MIGRATE-001 의 alias loader 패턴과 일관성. 사용자 개입 0.
2. **단일 surface**: 마이그레이션 안내 README/CHANGELOG 1회만 — manual migration 의 "이전 데이터 어디 갔지?" 사용자 혼란 제거.
3. **단방향 신뢰**: 마이그레이션은 1회 이벤트. atomic rename 이 가능하면 즉시 완료, 실패 시 copy+verify+remove 의 정책 적용.

### 2.3 왜 중앙 path resolver 인가

사용자 인터뷰 결정사항 (C): 단일 `internal/userpath` 패키지 도입. 30+ callsites across 18 distinct files 의 hardcoded `~/.goose/` 분산을 단일 진입점으로 수렴. 근거:

1. **ENV-MIGRATE-001 선례**: `internal/envalias` 가 22-key alias map + 단일 resolver 로 코드 분산을 막은 사례. 동일 패턴 재사용.
2. **미래 리네임 안전망**: 만약 향후 `mink` 가 다른 이름으로 재변경되더라도 (R가 가능성 낮으나) resolver 함수 한 곳만 수정.
3. **테스트 가능성**: 콜사이트가 `userpath.UserHome()` 만 호출하면 테스트에서 `t.Setenv("MINK_HOME", tmpDir)` 로 격리 가능. 현재 30+ callsites across 18 distinct files 의 `os.UserHomeDir()` + `filepath.Join("...", ".goose")` 패턴은 테스트 격리 불가.

### 2.4 영향 범위 (30+ callsites across 18 distinct files)

| 경로 패턴 | 위치 (production) | Type |
|----------|------------------|------|
| `~/.goose/config/web.yaml` | `internal/tools/web/search.go:429` | `[MODIFY]` |
| `.goose-write-*` (tmp file prefix) | `internal/tools/builtin/file/write.go:82` | `[MODIFY]` |
| `./.goose/data/`, `./.goose/{memory,context,skills,tasks,rituals}/` | `internal/qmd/config.go:55-69` | `[MODIFY]` |
| `./.goose/config.yaml`, `~/.goose/` root resolver | `internal/config/config.go:228,279,281` | `[MODIFY]` |
| logs/security paths | `internal/config/defaults.go:42-47` | `[MODIFY]` |
| `~/.goose/memory/memory.db` | `internal/memory/config.go:43` | `[MODIFY]` |
| `~/.goose/sessions` | `internal/cli/tui/sessionmenu/loader.go:60` | `[MODIFY]` |
| `~/.goose/logs` | `internal/cli/commands/audit.go:90,122` | `[MODIFY]` |
| sessions root + `.goose-session-*` tmp prefix | `internal/cli/session/session.go:39,42,62` | `[MODIFY]` |
| `~/.goose/messaging` | `internal/cli/commands/messaging_telegram.go:326` | `[MODIFY]` |
| `.goose/mcp-credentials` (const) | `internal/mcp/credentials.go:15` | `[MODIFY]` |
| `~/.goose/permissions.json` | `internal/cli/tui/model.go:168` | `[MODIFY]` |
| `~/.goose` + `.goose/logs/audit.local.log` | `internal/audit/dual.go:146,155` | `[MODIFY]` |
| `.goose/aliases.yaml` | `internal/command/adapter/aliasconfig/{merge,loader}.go` | `[MODIFY]` |
| agent-memory paths | `internal/subagent/{memory,run}.go:33-35,506` | `[MODIFY]` |
| `~/.goose/ritual/schedule.json` | `internal/ritual/scheduler/persist.go:41` | `[MODIFY]` |
| `~/.goose/permissions/grants.json` | `internal/permission/store/file.go:98` | `[MODIFY]` |
| `~/.goose/telegram.yaml`, `telegram.db` | `internal/messaging/telegram/{config,store}.go` | `[MODIFY]` |
| `internal/userpath/` (신규 패키지) | (없음, `[NEW]`) | `[NEW]` |
| test files | `*_test.go` (전수, ENV-MIGRATE-001 선례에 따라 t.TempDir 또는 t.Setenv 활용) | `[MODIFY]` |

[NEW] = 본 SPEC 신설. [MODIFY] = 기존 코드 수정. [REMOVE] = 제거 (본 SPEC 에는 없음).

---

## 3. Scope

### 3.1 IN Scope — 신규 패키지 도입 `internal/userpath`

[HARD] [NEW] 다음 패키지를 신설한다. 모든 production 콜사이트가 본 패키지를 통해 경로를 해석하도록 한다.

| Element | Signature (개념적, 코드 작성은 Run phase 에서) | 책임 |
|---------|---------------------------------------|------|
| `userpath.UserHome() (string, error)` | `~/.mink/` 경로 반환 | `MINK_HOME` env → `~/.mink/` fallback (envalias 통과) |
| `userpath.ProjectLocal(cwd string) string` | `<cwd>/.mink/` 경로 반환 | project-local 디렉토리 (qmd, alias config 등) |
| `userpath.SubDir(name string) (string, error)` | `~/.mink/<name>/` ensure-exists | subdirectory 생성 + permission 0700 |
| `userpath.TempPrefix() string` | `.mink-` (옛 `.goose-`) | tmp file prefix |
| `userpath.MigrateOnce(ctx context.Context) (MigrationResult, error)` | 첫 실행 시 1회 호출 | 자동 마이그레이션 + 알림 |
| `userpath.LegacyHome() string` | `~/.goose/` 경로 반환 | fallback read-only 접근용 (R6) |

reference 인용: `internal/envalias/loader.go` (ENV-MIGRATE-001) — 22-key alias map + 단일 `Get()` 함수 패턴.

### 3.2 IN Scope — Production 콜사이트 마이그레이션 (30+ callsites across 18 distinct files)

[HARD] [MODIFY] 다음 콜사이트가 모두 `internal/userpath` 의 함수를 호출하도록 변경한다. 직접 `os.UserHomeDir()` + `filepath.Join("...", ".goose")` 패턴 0건이 목표. 영향 범위는 30+ literal occurrence 가 18개 distinct production file 에 분포한다 (production code only — `*_test.go` 별도 집계).

본 SPEC 머지 후 `grep -rEn '\.goose' --include='*.go' --exclude='*_test.go' .` 출력이 `internal/userpath/legacy.go` 의 `LegacyHome()` 정의 단 1곳으로 수렴 (REQ-MINK-UDM-006).

### 3.3 IN Scope — 자동 마이그레이션 로직 + first-run 감지

[HARD] [NEW] `userpath.MigrateOnce()` 가 다음을 수행한다:

1. **감지**: `~/.goose/` 존재 + `~/.mink/` 부재 → 마이그레이션 진입.
2. **Brand marker 검증** (가능한 경우): `~/.goose/.mink-managed` 또는 `~/.goose/config.yaml` 내부의 MINK-specific 필드로 식별. 없으면 best-effort 진행 + warning log.
3. **Atomic rename 시도**: `os.Rename("~/.goose", "~/.mink")` — 같은 filesystem 일 때 atomic. 실패 시 step 4.
4. **Fallback copy + remove**: cross-filesystem 시 `io/fs.WalkDir` + `os.MkdirAll` + `io.Copy` + checksum verify → `os.RemoveAll` (R2 참조).
5. **Lock file**: 마이그레이션 중 `~/.mink/.migration.lock` 보유. 동시 실행 차단 (R5).
6. **Marker**: 마이그레이션 완료 시 `~/.mink/.migrated-from-goose` (timestamp + binary version) 작성. 멱등 보장.
7. **알림**: stderr 에 `INFO: 사용자 데이터가 ~/.goose/ → ~/.mink/ 로 마이그레이션되었습니다. 이전 디렉토리는 안전하게 보존됩니다.` 한 줄. README 안내 링크.
8. **Project-local**: project `./.goose/` 는 cwd-scoped → daemon 시작 시점이 아닌 `userpath.ProjectLocal(cwd)` 첫 호출 시 점진 migration. project-local 은 사용자 작업 컨텍스트 안에서만 의미가 있어 lazy migration 이 적절.

### 3.4 IN Scope — Dual-read fallback semantics

[HARD] [NEW] 본 SPEC 머지 직후 (마이그레이션이 아직 1회도 안 일어난 시점) 의 안전망:

- `userpath.UserHome()` 호출 시 `MigrateOnce()` 가 lock 획득 실패 (다른 프로세스 마이그레이션 중) → blocking wait (max 30s) 후 `~/.mink/` 반환.
- 마이그레이션이 부분적으로 실패한 상태 (mid-copy crash) → `~/.mink/.migration.lock` 잔존 + `~/.goose/` 여전 존재 감지 → next run 에서 cleanup + retry.
- `~/.goose/` 와 `~/.mink/` 가 동시에 존재 (사용자가 수동 복원) → `~/.mink/` 우선, `~/.goose/` 무시 + warning 1회 (R7).

### 3.5 OUT Scope (Exclusions)

[HARD] 다음은 본 SPEC 범위 외 — 위반 시 scope creep.

1. **`.moai/specs/*` SPEC 디렉토리 변경**: 이미 SPEC-MINK-* / SPEC-GOOSE-* 보존 정책. 본 SPEC 은 user-data runtime state 만 다룸.
2. **third-party CLI (`goose` Block AI) 와의 path 공유 보호**: 본 SPEC 은 MINK 가 만든 `~/.goose/` 만 마이그레이션 시도. brand marker 부재 시 best-effort + warning. third-party 가 같은 경로를 쓰는 시나리오는 사용자 책임.
3. **데이터 schema migration**: `.db` 파일 내부 schema, JSON shape 등은 변경 0건.
4. **`uninstall` 또는 `unmigrate` CLI 명령**: 본 SPEC 은 단방향 migration 만. 사용자가 수동 `mv` 하면 다음 실행 시 다시 마이그레이션되는 멱등성만 보장.
5. **`SPEC-MINK-BRAND-RENAME-001` body 수정**: immutable.
6. **`SPEC-MINK-ENV-MIGRATE-001` body 수정**: immutable.
7. **CHANGELOG 기존 entry 수정**: immutable (BRAND-RENAME §3.2 item 4).
8. **`.moai/brain/IDEA-*/**`, `.claude/agent-memory/**`**: 본 SPEC 범위 외 (path 패턴 0건이며 마이그레이션 대상 아님).
9. **logs/audit 의 회고 분석**: 본 SPEC 머지 시점에 기록된 로그 메시지는 archival, 본 SPEC 은 신규 로그만 새 path 사용.
10. **MINK_HOME / GOOSE_HOME env var 자체 정의 변경**: ENV-MIGRATE-001 가 이미 alias loader 로 처리. 본 SPEC 은 그 결과를 소비할 뿐.
11. **third-party plugin / extension 의 hardcoded `~/.goose/`**: 사용자 책임. 본 SPEC 은 first-party (`internal/`) 코드만 다룸.

---

## 4. EARS Requirements

본 SPEC 은 19개 EARS 요구사항을 정의한다. 각 요구사항은 acceptance.md 의 AC 와 N:1 또는 1:1 매핑. (v0.1.1 에서 REQ-019 신설 — D6 fix.)

### 4.1 Ubiquitous (항상 성립)

- **REQ-MINK-UDM-001 [Ubiquitous]** The project **shall** provide a centralized path resolver package at `internal/userpath` after Phase 1 completes.
- **REQ-MINK-UDM-002 [Ubiquitous]** All production callsites in `internal/**/*.go` (excluding `_test.go` and `internal/userpath/legacy.go`) **shall** resolve user-data paths only through `userpath.UserHome()`, `userpath.ProjectLocal()`, `userpath.SubDir()`, or `userpath.TempPrefix()` after Phase 2 completes.
- **REQ-MINK-UDM-003 [Ubiquitous]** The user-home directory **shall** be `~/.mink/` after first-run migration; the project-local directory **shall** be `./.mink/` after first project access.
- **REQ-MINK-UDM-004 [Ubiquitous]** The tmp file prefix **shall** be `.mink-` (replacing `.goose-`) for all new files written after Phase 2 completes.
- **REQ-MINK-UDM-005 [Ubiquitous]** Internal data schemas (e.g., `memory.db`, `permissions/grants.json`, `telegram.db`) **shall** remain byte-identical across migration; only the parent directory path changes.
- **REQ-MINK-UDM-006 [Ubiquitous]** The hardcoded literal `.goose` **shall** appear in production source code (`*.go`, excluding `*_test.go`) only in `internal/userpath/legacy.go` after Phase 2 completes.

### 4.2 Event-Driven (트리거 발생 시)

- **REQ-MINK-UDM-007 [Event-Driven]** **When** `mink` or `minkd` starts and detects `~/.goose/` exists but `~/.mink/` does not, the runtime **shall** invoke `userpath.MigrateOnce()` exactly once per process lifetime.
- **REQ-MINK-UDM-008 [Event-Driven]** **When** `userpath.MigrateOnce()` completes successfully, the runtime **shall** emit a one-time stderr notice in Korean (with optional English subtext on a second line) referencing the migration source and destination paths. The notice **shall not** contain the legacy brand word "goose"; it **shall** contain the new brand identifier "mink" or its Korean form "밍크" verbatim.
- **REQ-MINK-UDM-009 [Event-Driven]** **When** atomic `os.Rename` fails (cross-filesystem or permission), `userpath.MigrateOnce()` **shall** fall back to copy-and-verify-and-remove with SHA-256 checksum equality before removing the source.
- **REQ-MINK-UDM-010 [Event-Driven]** **When** `userpath.ProjectLocal(cwd)` is called for a `cwd` containing `./.goose/` but no `./.mink/`, the resolver **shall** lazily migrate the project-local directory using the same algorithm as user-home migration.

### 4.3 State-Driven (상태 조건)

- **REQ-MINK-UDM-011 [State-Driven]** **While** `~/.mink/.migration.lock` exists, any concurrent `userpath.MigrateOnce()` call **shall** block (max 30 seconds) and then read the post-migration state.
- **REQ-MINK-UDM-012 [State-Driven]** **While** both `~/.goose/` and `~/.mink/` exist after migration (e.g., user manually restored), `userpath.UserHome()` **shall** return `~/.mink/` and emit a one-time stderr warning that `~/.goose/` is being ignored.
- **REQ-MINK-UDM-013 [State-Driven]** **While** the filesystem is read-only or the migration target path is unwritable, `userpath.MigrateOnce()` **shall** return a typed error (`ErrReadOnlyFilesystem` or `ErrPermissionDenied`) and the runtime **shall** fall back to read-only access of `~/.goose/` with a user-actionable error message.

### 4.4 Unwanted (금지 행동)

- **REQ-MINK-UDM-014 [Unwanted]** **If** a code review identifies a production callsite (`*.go` excluding `*_test.go` and `internal/userpath/legacy.go`) that constructs a path via `os.UserHomeDir() + ".goose"` or `filepath.Join(home, ".goose")`, **then** the review **shall** reject the change.
- **REQ-MINK-UDM-015 [Unwanted]** **If** `userpath.MigrateOnce()` fails mid-copy (verification mismatch or interrupted), **then** the partial `~/.mink/` directory **shall** be removed before returning the error; the source `~/.goose/` **shall** remain intact.
- **REQ-MINK-UDM-016 [Unwanted]** **If** any test file uses the literal path `~/.goose/` or `./.goose/` outside an `// MINK migration fallback test` comment or `internal/userpath/legacy_test.go`, **then** the test **shall** fail with a setup error and the reviewer **shall** require migration to `t.TempDir()` or `t.Setenv("MINK_HOME", ...)`.

### 4.5 Optional (해당 시)

- **REQ-MINK-UDM-017 [Optional]** **Where** `~/.goose/` contains a brand marker file (`.mink-managed` or `config.yaml` with MINK-specific field), `userpath.MigrateOnce()` **shall** proceed silently; if absent, the function **shall** emit an additional stderr warning that brand verification was best-effort.
- **REQ-MINK-UDM-018 [Optional]** **Where** the user explicitly sets `MINK_HOME` env var (resolved via `internal/envalias`), `userpath.UserHome()` **shall** return that path verbatim and **shall not** attempt automatic migration from `~/.goose/`. **When** `MINK_HOME` is set to an empty string, a non-writable path, or a value containing path-traversal segments (`..`), `userpath.UserHome()` **shall** reject the value with a typed error and **shall not** silently fall back to `~/.mink/`.

### 4.6 Ubiquitous — Security (file mode preservation)

- **REQ-MINK-UDM-019 [Ubiquitous]** The system **shall** preserve source file mode bits (never weakened) in the destination across all copy fallback paths when migrating sensitive files. (시스템은 sensitive 파일을 copy fallback 경로로 마이그레이션할 때 원본 mode bits 를 보존해야 한다 — destination mode 는 source mode 이하여야 한다.)
  - **Scope**: 본 invariant 는 atomic rename 이 실패하여 copy fallback 경로 (REQ-MINK-UDM-009) 가 사용될 때 모든 destination 파일에 적용된다. atomic rename 성공 시 inode 이동으로 자동 보존되므로 별도 검증 불필요.
  - **Sensitive files**: `permissions/grants.json`, `messaging/telegram.db`, `mcp-credentials/*`, `ritual/schedule.json` 의 0600 권한은 destination 에서도 0600 (또는 더 제한적) 으로 유지된다.
  - **Directory mode**: 0700 권한 규칙도 동일하게 적용된다.
  - **Ownership**: uid/gid 는 가능한 경우 보존, 불가능한 경우 (cross-uid copy 등) silent skip.

---

## 5. Affected Files

### 5.1 Production code (Go, `internal/**`)

[NEW] 신규:
- `internal/userpath/userpath.go` — main resolver
- `internal/userpath/migrate.go` — MigrateOnce + lock + copy-fallback
- `internal/userpath/legacy.go` — `LegacyHome()` 단일 정의 (`.goose` literal 의 유일한 잔존 위치)
- `internal/userpath/errors.go` — typed errors (`ErrReadOnlyFilesystem`, `ErrPermissionDenied`, `ErrLockTimeout`)
- `internal/userpath/userpath_test.go`, `migrate_test.go`, `legacy_test.go` — table-driven + fs isolation via t.TempDir

[MODIFY] 다음 18 production 파일 (30+ literal occurrence):
- `internal/tools/web/search.go`
- `internal/tools/builtin/file/write.go`
- `internal/qmd/config.go`
- `internal/config/config.go`, `internal/config/defaults.go`
- `internal/memory/config.go`
- `internal/cli/tui/sessionmenu/loader.go`
- `internal/cli/commands/audit.go`
- `internal/cli/session/session.go`
- `internal/cli/commands/messaging_telegram.go`
- `internal/mcp/credentials.go`
- `internal/cli/tui/model.go`
- `internal/audit/dual.go`
- `internal/command/adapter/aliasconfig/merge.go`, `loader.go`
- `internal/subagent/memory.go`, `run.go`
- `internal/ritual/scheduler/persist.go`
- `internal/permission/store/file.go`
- `internal/messaging/telegram/config.go`, `store.go`

### 5.2 Test files

[MODIFY] 다음 패턴이 적용된 모든 `*_test.go` (ENV-MIGRATE-001 의 33-callsite migration 선례 준수):
- `os.Setenv("GOOSE_HOME", ...)` 사용처 → `t.Setenv("MINK_HOME", t.TempDir())` 또는 직접 `userpath` mock
- `~/.goose/` 또는 `./.goose/` 리터럴 사용처 → `t.TempDir()` + `userpath` 호출
- Migration fallback test 만 예외 — `// MINK migration fallback test` 주석으로 marker

### 5.3 진입점 wiring

[MODIFY] `mink` / `minkd` 의 시작 시퀀스:
- `cmd/mink/main.go` 또는 `internal/cli/run.go` 에서 `userpath.MigrateOnce(ctx)` 1회 호출
- `cmd/minkd/main.go` 또는 daemon entry 동일

### 5.4 Documentation

[MODIFY]:
- `README.md` — Migration 안내 section (마이그레이션 후 ~/.mink/ 위치 안내)
- `CHANGELOG.md` — `[Unreleased]` 항목에 BREAKING 마커 (사용자 데이터 위치 변경, auto-migration 으로 완화)
- `.moai/project/structure.md` — 신규 패키지 등록
- `.moai/project/codemaps/internal-userpath.md` (신규)

### 5.5 Brand-lint exemption 갱신

[MODIFY] `scripts/check-brand.sh` — `internal/userpath/legacy.go` 와 `internal/userpath/legacy_test.go` 만 `.goose` 리터럴 허용. 다른 production 파일에서 `.goose` 등장 시 fail.

---

## 6. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **명시적으로 제외**:

1. **`.moai/specs/*` SPEC 디렉토리 마이그레이션** — 이미 MINK 네이밍 또는 immutable PRESERVE 대상. 본 SPEC 은 사용자 데이터 (런타임 mutable state) 만 다룸.
2. **third-party `goose` (Block AI) CLI 와의 path 충돌 보호** — 사용자가 두 brand 를 동시에 사용하는 시나리오는 본 SPEC 안전망 대상 외. brand marker 검증으로 best-effort.
3. **데이터 schema migration** — `.db` schema, JSON shape 변경 0건. 디렉토리 위치만 이동.
4. **rollback / uninstall CLI 명령** — 단방향 migration.
5. **logo / visual 자산** — BRAND-RENAME OUT 계승.
6. **env var loader 재작업** — ENV-MIGRATE-001 결과 소비만.
7. **legacy CHANGELOG / SPEC HISTORY rows 변경** — immutable.
8. **multi-user 분산 lock** — 단일 lock file 만, 본격적 distributed lock OUT.
9. **symlink target 자동 resolve 후 migrate** — symlink 감지 시 user-actionable error 만, auto-resolve 는 R3.
10. **`SPEC-MINK-PRODUCT-V7-001`, `SPEC-MINK-DISTANCING-STATEMENT-001`** — 별도 SPEC.

---

## 7. References

### 7.1 본 SPEC 산출물

- `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/spec.md` (본 문서, v0.1.3)
- `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/plan.md` (구현 계획)
- `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/acceptance.md` (Given/When/Then 12 main + 4 edge scenarios)
- `.moai/specs/SPEC-MINK-USERDATA-MIGRATE-001/spec-compact.md` (REQ + AC + affected files 요약)

### 7.2 선행 SPEC (의존, supersede 아님)

- `SPEC-MINK-BRAND-RENAME-001` v0.2.0 (completed 2026-05-13) — brand identity 통일. binary `mink` 가 존재한다는 전제 제공.
- `SPEC-MINK-ENV-MIGRATE-001` v0.2.0 (completed 2026-05-13) — env var alias loader. `internal/envalias` 패키지 (PR #171 머지, main `0c24237`). 본 SPEC 의 `userpath.UserHome()` 이 `envalias.Get("MINK_HOME")` 호출.

### 7.3 Reference 구현 패턴

- `internal/envalias/loader.go` (ENV-MIGRATE-001) — alias map + 단일 resolver 함수 패턴. 본 SPEC `internal/userpath` 의 설계 기반.
- 22-key alias map 의 backward-compat 정책 (key 우선순위, deprecation warning) — 본 SPEC 의 dual-read fallback 에 그대로 적용.

### 7.4 이슈 / PR / 결정 trail

- PR #168 (BRAND-RENAME-001 squash, main `0c24237`)
- PR #170 (ENV-MIGRATE-001 plan)
- PR #171 (ENV-MIGRATE-001 6-phase implementation, squash → main `0c24237`)
- 사용자 1라운드 인터뷰 결정사항 (2026-05-13):
  - (A) `~/.goose/` + `./.goose/` 양쪽 모두 마이그레이션 in-scope
  - (B) 첫 실행 시 자동 마이그레이션 + 일회성 알림
  - (C) 중앙 path resolver 패키지 `internal/userpath` 신설

### 7.5 산업 사례

- Helm 2 → Helm 3 migration: `~/.helm/` → `~/.config/helm/` (자동 마이그레이션 명령 `helm 2to3 convert`)
- nvm: `~/.nvm/` 위치 보존, env var (`NVM_DIR`) 로 override 허용
- Go modules: `$GOPATH` → `$GOMODCACHE` 점진 마이그레이션 (env var 우선)

---

Version: 0.1.2
Status: draft
Last Updated: 2026-05-13

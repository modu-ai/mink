# SPEC-MINK-USERDATA-MIGRATE-001 — Implementation Plan

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-13 | manager-spec | 초안 작성. 6-phase 분해 + brownfield delta marker + risk table 12개 + MX tag 계획. ENV-MIGRATE-001 (`internal/envalias`) 패턴을 reference 로 채택하여 단일 PR + squash merge 정책 계승. |
| 0.1.1 | 2026-05-13 | manager-spec | plan-auditor iter 1 fix — 13 defects 반영 후속 plan 동기화. "30+ 콜사이트" → "30+ callsites across 18 distinct files" 일관 표기 (D11). Phase 1 의 파일 권한 정책에 mode bits 보존 명시 추가 (REQ-019 신설, D6). Phase 3 fail-fast / graceful degrade 분리 명시 (AC-004 split, D8). Phase 4 의 `MINK_HOME` 경계 검증 추가 (D3). Risk table R13 신설 — mode bits 보존 위반 시 security regression 위험. |
| 0.1.2 | 2026-05-13 | manager-spec | plan-auditor iter 2 fix — ND-2 단일 잔존 stale phrase 정정. Risk table R1 의 "30+ 콜사이트" → "30+ literal occurrences across 18 distinct files" 일관 표기 (spec.md / plan.md 의 다른 모든 occurrence 와 정합). 본 plan 의 다른 sections 는 변경 없음. |

---

## 1. 구현 전략 개관

본 SPEC 은 **brownfield 마이그레이션** 이다. 30+ callsites across 18 distinct files 가 hardcoded `~/.goose/` 패턴을 가지고 있으며, 이를 모두 `internal/userpath` 패키지로 수렴시킨다. ENV-MIGRATE-001 의 `internal/envalias` 가 22-key alias map 으로 11 production + 33 test 콜사이트를 정리한 선례를 따른다.

전략 요지:
- **단일 PR + squash merge** — BRAND-RENAME / ENV-MIGRATE 와 동일 정책 (CLAUDE.local.md §1.4)
- **Phase 1 (foundation) 우선** — 패키지 + 테스트 먼저, 콜사이트 마이그레이션은 그 다음
- **Atomic per-phase commits** — phase 별 독립 commit, phase 내부는 logical 단위로 분할 가능
- **Test-first for `internal/userpath`** — TDD mode (quality.yaml 기본값) 적용, 신규 패키지는 RED-GREEN-REFACTOR 사이클 준수
- **Brownfield migration 콜사이트** — DDD characterization test 우선 (기존 동작 보존 보장 후 path 만 변경)

---

## 2. Phase 분해

| Phase | 작업명 | Priority | 위험도 | 의존성 |
|-------|--------|----------|--------|--------|
| P1 | `internal/userpath` 패키지 + 단위 테스트 | High (foundation) | 낮 | 없음 |
| P2 | Production 콜사이트 마이그레이션 (18 distinct files / 30+ literal occurrences) | High | 중 | P1 |
| P3 | Auto-migration logic + first-run detection 진입점 wiring | High | 중-높 | P1, P2 |
| P4 | Dual-read fallback + lock + recovery semantics | Medium | 중 | P3 |
| P5 | Test 파일 마이그레이션 (ENV-MIGRATE 선례 준수) | Medium | 낮 | P2 |
| P6 | Documentation + CHANGELOG + PR 준비 | Low | 낮 | P1-P5 |

### Phase 1 — `internal/userpath` 패키지 + 단위 테스트 [NEW]

목표: 단일 신규 패키지에 resolver 함수 + 마이그레이션 함수를 정의하고, 표준 unit test 로 격리 검증.

산출물 ([NEW]):
- `internal/userpath/userpath.go` — `UserHome()`, `ProjectLocal(cwd)`, `SubDir(name)`, `TempPrefix()`
- `internal/userpath/migrate.go` — `MigrateOnce(ctx)`, `MigrationResult` struct
- `internal/userpath/legacy.go` — `LegacyHome()` (단일 `.goose` 리터럴 정의 위치, REQ-MINK-UDM-006)
- `internal/userpath/errors.go` — typed errors
- `internal/userpath/*_test.go` — table-driven + `t.TempDir()` + `t.Setenv("MINK_HOME", ...)`

기술 접근:
- `envalias.Get("MINK_HOME")` 호출 → ENV-MIGRATE-001 결과 의존성
- 모든 함수는 stateless (process global 없음), 호출 시점에 env + fs 만 참조
- `MigrateOnce` 는 `sync.Once` 또는 file lock 으로 멱등 보장
- 파일 권한 (security 기본 + REQ-019, D6 fix):
  - 신규 home dir = 0700 (default umask 무시, 명시 `chmod 0700`)
  - 신규 subdir = 0700
  - 신규 files = 0600 (default umask 무시)
  - **Copy fallback 시 mode bits 보존** (REQ-MINK-UDM-019): source 파일의 mode bits 를 destination 에 명시 적용. `os.Chmod(dst, srcInfo.Mode().Perm())` 호출 필수. mode 가 더 제한적인 source (예: `permissions/grants.json` 0600) 의 경우 그대로 0600 유지. 약화 (`0644` 로 떨어짐) 금지.
  - ownership 은 가능한 경우만 보존 (cross-uid 시 `os.Chown` 실패 무시, silent skip)
- `MINK_HOME` 입력 검증 (REQ-018, D3 fix):
  - 빈 문자열 reject — typed error `ErrMinkHomeEmpty`
  - `..` segment 포함 reject — typed error `ErrMinkHomePathTraversal`
  - legacy `.goose` 경로 reject — typed error `ErrMinkHomeIsLegacyPath`
  - non-writable path 는 첫 write 시점에 `ErrPermissionDenied` 또는 `ErrReadOnlyFilesystem` 으로 fail

검증 (Phase 1 commit 전):
- `go test ./internal/userpath/... -race` exit 0
- `go vet ./internal/userpath/...` exit 0
- coverage ≥ 85% (quality.yaml 기본 임계)
- LSP errors = 0

MX 태그 계획:
- `@MX:ANCHOR` — `UserHome()`, `ProjectLocal()`, `MigrateOnce()` (high fan_in 예상 30+)
- `@MX:NOTE` — `legacy.go` (intentional exception, brand-lint exemption)
- `@MX:WARN` — `MigrateOnce()` copy fallback (cross-filesystem 위험 zone)

Commit message template:
```
feat(userpath): SPEC-MINK-USERDATA-MIGRATE-001 P1 — internal/userpath 패키지 신설

- UserHome / ProjectLocal / SubDir / TempPrefix resolver
- MigrateOnce + typed errors + sync.Once 멱등 보장
- legacy.go = 유일한 .goose 리터럴 정의 위치
- coverage ≥ 85%, race-free

SPEC: SPEC-MINK-USERDATA-MIGRATE-001
REQ:  REQ-MINK-UDM-001, REQ-MINK-UDM-005, REQ-MINK-UDM-006
```

### Phase 2 — Production 콜사이트 마이그레이션 [MODIFY]

목표: 18 distinct files (30+ literal occurrence) 의 `os.UserHomeDir() + ".goose"` 패턴을 `userpath.UserHome()` 등으로 일괄 치환.

대상 (§5.1 affected files 참조):
- 18개 production `.go` 파일 (test 제외, 30+ literal occurrence)
- 각 파일: characterization test (DDD ANALYZE-PRESERVE-IMPROVE 사이클) — 변경 전 기존 동작 capture, 변경 후 동일성 검증

작업 순서:
1. 각 콜사이트 grep + read → 현재 path 구성 방식 파악
2. `userpath` API 매핑 결정 (UserHome vs SubDir vs ProjectLocal)
3. characterization test 작성 (필요 시) — 기존 path 가 동일한 결과 반환하는지 capture
4. Edit 도구로 직접 치환 (sed 금지 — 도메인 컨텍스트별 함수 선택 필요)
5. `go vet ./internal/...` + `go test ./internal/...` 통과 확인

검증 (Phase 2 commit 전 — D4 fix: `grep --exclude=<path>` 는 basename glob 만 매칭하므로 post-filter pipeline 사용):
- `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` = 0
- `grep -rEn 'filepath\.Join\([^)]*"\.goose"' --include='*.go' --exclude='*_test.go' . | grep -v '^./internal/userpath/legacy\.go:' | wc -l` = 0
- `go build ./...` exit 0
- 기존 unit test 통과 (path 변경 외 logic 변경 0건)

Risk:
- false positive (`.goose` 가 일부 string literal 안에서 user-facing message 로 사용될 수 있음 — brand-position 정정은 BRAND-RENAME 결과 위반은 아니나 검토 필요)
- 함수 선택 실수 (UserHome 대신 ProjectLocal 사용 등) → characterization test 가 catch

MX 태그 계획:
- 변경된 콜사이트 중 fan_in ≥ 3 함수: `@MX:ANCHOR` 추가
- 복잡한 path 결정 로직 (`internal/qmd/config.go` 등): `@MX:NOTE`

### Phase 3 — Auto-migration + first-run detection [NEW]

목표: `mink` / `minkd` 진입점에서 `userpath.MigrateOnce(ctx)` 1회 호출 wiring.

대상 ([MODIFY]):
- `cmd/mink/main.go` — CLI 시작 시퀀스
- `cmd/minkd/main.go` — daemon 시작 시퀀스
- `internal/cli/run.go` 또는 동등한 wiring point (Phase 1 시점 코드 확인 필요)

기술 접근 (D8 fix — fail-fast / graceful degrade 분리 명시):
- 진입점 첫 줄에 `if result, err := userpath.MigrateOnce(ctx); ...` 호출
- 에러 시 분기 정책 (REQ-MINK-UDM-013):
  - **CLI (`mink`)**: fail-fast — 즉시 non-zero exit + stderr user-actionable error. 사용자가 정확히 어디서 막혔는지 즉시 인지 (AC-004a).
  - **Daemon (`minkd`)**: graceful degrade — startup 계속, stderr/log warning + read-only fallback. 단일 마이그레이션 실패로 daemon 자체가 죽으면 안 됨 (AC-004b).
  - 구분 방법: `cmd/mink/main.go` 와 `cmd/minkd/main.go` 의 각 진입점에서 분기 (process boot mode 가 명시적으로 다름)
- `MigrationResult` 가 `migrated == true` 면 stderr 알림 1회

검증:
- 실행 시나리오 (manual smoke test):
  - 시나리오 A: `~/.goose/` 존재 + `~/.mink/` 부재 → 마이그레이션 발생 + 알림
  - 시나리오 B: 둘 다 부재 → `~/.mink/` 생성, 알림 없음
  - 시나리오 C: `~/.mink/` 만 존재 → no-op
- integration test `cmd/mink/integration_test.go` 신설 또는 확장 (t.TempDir + HOME env 격리)

MX 태그:
- 진입점 호출: `@MX:ANCHOR` (process startup invariant)
- 알림 stderr 출력: `@MX:NOTE` (user-facing UX)

### Phase 4 — Dual-read fallback + lock + recovery [NEW]

목표: 동시성 + 부분 실패 + 사용자 수동 복원 같은 edge case 견고화.

대상 ([MODIFY]):
- `internal/userpath/migrate.go` — lock 획득/해제 + cleanup-on-failure
- `internal/userpath/recovery.go` (신규) — partial migration cleanup

기술 접근:
- File lock: `~/.mink/.migration.lock` flock-style (Go 의 `golang.org/x/sys/unix.Flock` 또는 별도 패키지)
- Cross-platform 검토 — Windows 미지원이면 skip + 단일 process 가정
- Mid-copy crash 감지: lock 파일에 PID + start timestamp → next run 에서 `os.Kill(pid, 0)` 으로 alive 검사
- Cleanup: `~/.mink/` 가 부분적으로 존재하면 전체 삭제 후 retry

검증:
- `internal/userpath/migrate_test.go` 의 lock + recovery 시나리오 (table-driven)
- Goroutine race test (`-race`)

Risk: 매우 중요 — 데이터 손실 위험. mitigation:
- copy 단계는 source 보존, verify 후에만 source 삭제
- checksum mismatch → return error + leave `~/.goose/` intact

### Phase 5 — Test 파일 마이그레이션 [MODIFY]

목표: ENV-MIGRATE-001 의 33-callsite test migration 선례 준수 — `os.Setenv("GOOSE_HOME", ...)` → `t.Setenv("MINK_HOME", t.TempDir())`.

대상:
- 모든 `*_test.go` 중 `~/.goose/` 또는 `./.goose/` 리터럴 또는 `GOOSE_HOME` 사용처
- migration fallback test 만 예외 (REQ-MINK-UDM-016) — `// MINK migration fallback test` 주석 + `internal/userpath/legacy_test.go`

작업 순서:
1. `grep -rEln '\bGOOSE_HOME\b|"\.goose' --include='*_test.go'` 로 후보 파일 list
2. 각 파일 t.Setenv 또는 t.TempDir 패턴 마이그레이션
3. `go test ./... -race` 전수 통과

검증:
- `grep -rEn '"\.goose' --include='*_test.go' --exclude='internal/userpath/legacy_test.go'` 출력의 모든 라인이 `// MINK migration fallback test` 주석 동반

### Phase 6 — Documentation + CHANGELOG + PR 준비 [MODIFY]

목표: 사용자 안내 + 후속 SPEC 참조 reference + brand-lint exemption 갱신.

산출물:
- `README.md` — `## Migration from .goose` section (자동 마이그레이션 안내, 옵션: `MINK_HOME` env 설정 방법)
- `CHANGELOG.md` — `[Unreleased]` 에 BREAKING 마커 + auto-migration 완화 명시
- `.moai/project/structure.md` — `internal/userpath` 신규 패키지 등록
- `.moai/project/codemaps/internal-userpath.md` (신규)
- `scripts/check-brand.sh` — `internal/userpath/legacy.go`, `legacy_test.go` exemption 추가

검증:
- `bash scripts/check-brand.sh` exit 0
- `grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go'` → `internal/userpath/legacy.go` 1건만

---

## 3. Risks & Mitigations

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|--------|--------|------|------------|
| R1 | 30+ literal occurrences across 18 distinct files 의 path semantics 비균질 (UserHome vs ProjectLocal vs SubDir 혼동) | 중 | 중 | Phase 2 시작 전 각 콜사이트 1라인 grep + read → API 매핑 표 작성. characterization test 로 동일 결과 검증. |
| R2 | Cross-filesystem rename 실패 → copy fallback 중단 시 데이터 손실 | 낮 | 매우 높 | Verify-before-remove 정책. SHA-256 checksum equality 후에만 source 삭제. mid-copy crash → next run 에서 `~/.mink/` 부분 삭제 + retry. (REQ-MINK-UDM-009, REQ-MINK-UDM-015) |
| R3 | `~/.goose` 가 symlink → 자동 resolve 위험 (target 도 같이 이동되면 안 됨) | 낮 | 높 | symlink 감지 시 `os.Lstat` 으로 분기. user-actionable error 반환 + 수동 마이그레이션 가이드. auto-resolve 는 OUT (§3.5 item 9). |
| R4 | 사용자가 third-party `goose` (Block AI) 와 같은 경로 공유 → brand marker 없으면 잘못 마이그레이션 | 낮 | 중 | brand marker 검증 (REQ-MINK-UDM-017). marker 부재 시 stderr warning + best-effort 진행. README 에 회피 방법 (`MINK_HOME` env) 안내. |
| R5 | 동시 프로세스 실행 시 race (예: `mink` CLI + `minkd` daemon 동시 first-run) | 중 | 중 | File lock (`~/.mink/.migration.lock`) + PID/timestamp 기반 stale lock 감지 (REQ-MINK-UDM-011). |
| R6 | read-only filesystem 환경 (CI sandbox, container) → migration 시도 자체가 fail | 중 | 중 | `ErrReadOnlyFilesystem` typed error + graceful fallback to `~/.goose/` read-only access (REQ-MINK-UDM-013). |
| R7 | 사용자가 수동으로 `mv ~/.mink ~/.goose` → 다음 실행 시 무한 재마이그레이션 | 낮 | 낮 | `~/.mink/.migrated-from-goose` marker 가 존재하면 두 디렉토리 동시 존재 상태로 인식 → `~/.mink/` 우선 + warning (REQ-MINK-UDM-012). |
| R8 | Project-local `./.goose/` 가 git-tracked → migration 이 dirty working tree 생성 | 중 | 낮 | `./.goose/` 가 `.gitignore` 에 등재되어 있다고 가정 (기존 정책). project-local 은 lazy migration 으로 사용자가 cwd 에 진입한 시점에만 처리. `./.goose/` 미존재 시 no-op. |
| R9 | macOS / Windows / Linux 권한 차이 (특히 Windows file lock semantics) | 낮 | 중 | 1차 릴리스는 Linux/macOS 만 검증. Windows lock 은 `os.OpenFile` `O_EXCL` 폴백 또는 skip + 단일 process 가정. cross-platform 본격 지원은 별도 SPEC. |
| R10 | `MigrateOnce` 가 30s lock timeout 초과 → 사용자 stuck | 낮 | 중 | 30s timeout 후 `ErrLockTimeout` + 사용자 안내 메시지 (`다른 mink 프로세스가 마이그레이션 중일 수 있습니다`). |
| R11 | 콜사이트 마이그레이션 누락 → 일부 파일이 여전히 `~/.goose/` 사용 | 중 | 중 | Phase 2 검증의 grep gate: production code 에 `"\.goose"` literal 0건 (legacy.go 예외). brand-lint workflow 에 path-resolver lint 추가. |
| R12 | PR diff 크기 (18 production files + 신규 패키지 + 30+ literal touch points) → review 부담 | 높 | 중 | Phase 별 commit 분리 → review 는 phase 별로 추적. 분량 부담 시 P1 (foundation) + P2-P5 (migration) + P6 (docs) 의 3-PR 분할 가능. |
| R13 | Copy fallback 경로에서 mode bits 가 default umask 로 채워져 sensitive 파일 (`permissions/grants.json` 등) 권한 약화 → security regression | 중 | 매우 높 | REQ-MINK-UDM-019 + AC-009 의 binary stat 검증. `internal/userpath/migrate.go` 의 copy 단계에서 `os.Chmod(dst, srcInfo.Mode().Perm())` 명시. Phase 1 unit test 가 0600 source → 0600 dest 시나리오를 강제 검증. (D6 fix) |
| R14 | `MINK_HOME` 가 신뢰 불가 값 (빈 문자열, path traversal, legacy `.goose` 경로, non-writable) 으로 설정 → silent fallback 시 path traversal / privilege escalation 위험 | 낮 | 높 | REQ-MINK-UDM-018 의 negative path 명시 + AC-008b 의 4 case 검증. typed error (`ErrMinkHomeEmpty` / `ErrMinkHomePathTraversal` / `ErrMinkHomeIsLegacyPath`) 로 명시적 reject. silent fallback 금지. (D3 fix) |

---

## 4. MX Tag Plan

본 SPEC 머지 후 추가/갱신될 @MX 태그 목록:

| 위치 | Tag Type | 사유 |
|------|----------|------|
| `internal/userpath/UserHome` | `@MX:ANCHOR` | fan_in 예상 30+, runtime path resolver invariant |
| `internal/userpath/ProjectLocal` | `@MX:ANCHOR` | fan_in 예상 5+, project-scoped invariant |
| `internal/userpath/MigrateOnce` | `@MX:ANCHOR` | process lifetime invariant (1회 호출) |
| `internal/userpath/MigrateOnce` (cross-fs branch) | `@MX:WARN` | 데이터 손실 위험 zone, verify-before-remove 강제 |
| `internal/userpath/legacy.go` | `@MX:NOTE` | brand-lint intentional exception, REQ-MINK-UDM-006 |
| `cmd/mink/main.go` migration call | `@MX:ANCHOR` | startup invariant, 1회 호출 |
| `cmd/minkd/main.go` migration call | `@MX:ANCHOR` | 동일 |
| `internal/userpath/recovery.go` (Phase 4) | `@MX:WARN` | cleanup-on-failure 위험 zone |

---

## 5. Coverage / Quality Gates

- `internal/userpath/**` coverage ≥ 90% (신규 코드 strict 임계)
- 전체 프로젝트 coverage 회귀 0 (현 baseline 유지)
- `go vet ./...` exit 0
- `golangci-lint run` exit 0 (existing rules)
- LSP errors / warnings = 0
- brand-lint exit 0
- `go test ./... -race` exit 0

---

## 6. PR / Branch 정책

- 단일 feature branch: `feature/SPEC-MINK-USERDATA-MIGRATE-001`
- Base: `main`
- Squash merge (CLAUDE.local.md §1.4)
- PR title: `feat(userpath): SPEC-MINK-USERDATA-MIGRATE-001 — ~/.goose/ → ~/.mink/ migration`
- Squash commit body 에 6 phase 의 commit message 요약 포함
- 분할 옵션 (R12): P1 + P2-P5 + P6 로 3-PR 분할 가능. 단, P1 (foundation) 머지 전에는 P2 시작 불가.

---

## 7. Reference 구현 패턴

### 7.1 `internal/envalias` 와의 유사성 (ENV-MIGRATE-001 PR #171)

| Aspect | `internal/envalias` | `internal/userpath` |
|--------|---------------------|---------------------|
| 신규 패키지 | yes (22-key alias map) | yes (5 resolver functions) |
| 단일 진입점 | `envalias.Get(key)` | `userpath.UserHome()`, `userpath.ProjectLocal()` 등 |
| Backward-compat | `MINK_X` 우선 + `GOOSE_X` deprecated | `~/.mink/` 우선 + `~/.goose/` migration source |
| Deprecation warning | stderr (env var 만 옛 키 설정 시) | stderr (양쪽 디렉토리 동시 존재 시) |
| 콜사이트 마이그레이션 | 11 production + 33 test | 18+ production + N test |
| 마이그레이션 도구 | t.Setenv migration | t.TempDir + userpath isolation |

### 7.2 핵심 차이점

- `envalias` 는 **read-only** (env var 조회만). `userpath` 는 **fs mutation** 동반 (rename / copy / remove).
- `userpath` 는 동시성, 락, 부분 실패, 사용자 수동 개입 등 추가 edge case 처리 필요.
- 데이터 손실 가능성이 있어 verify-before-remove 정책 강제.

---

Version: 0.1.2
Status: draft
Last Updated: 2026-05-13

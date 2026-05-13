---
id: SPEC-MINK-ENV-MIGRATE-001
version: "0.2.0"
status: implemented
created_at: 2026-05-13
updated_at: 2026-05-13
author: manager-spec (delegated by orchestrator)
priority: High
labels: [migration, env-vars, branding, deprecation, alias-loader, mink, post-brand-rename]
issue_number: null
pr_number: 171
implemented_at: 2026-05-13
depends_on: [SPEC-MINK-BRAND-RENAME-001]
related_specs: [SPEC-MINK-PRODUCT-V7-001, SPEC-MINK-DISTANCING-STATEMENT-001, SPEC-MINK-USERDATA-MIGRATE-001]
phase: meta
lifecycle: spec-anchored
---

# SPEC-MINK-ENV-MIGRATE-001 — `GOOSE_*` env vars → `MINK_*` deprecation alias loader

## HISTORY

| Version | Date | Author | Description |
|---------|------|--------|-------------|
| 0.1.0 | 2026-05-13 | manager-spec | 초기 plan 작성 — research.md 22-key 인벤토리 + 8 확정 정책 + EARS 9 REQ + AC 10 시나리오 + 6 phase atomic 분할 |
| 0.1.1 | 2026-05-13 | manager-spec | plan-auditor finding 12건 정정 (CONDITIONAL_GO → GO). D1: 3 sibling 파일 frontmatter 추가. D2: 10 → 11 read site (const-based 1개 누락 발견). D3: 50+ → 28 t.Setenv + 6 os.Setenv (grep -c 결과 기반 재작성). D4: REQ-MINK-EM-009 EARS Optional 패턴 재작성. D5: line drift anchor (as of f0f02e4). D6: REQ-EM-{004,005,006} ↔ REQ-MINK-BR-027 매핑 표. D7: R11/R12/R13 risks 추가. D8: Exclusion #8 (시간 추정) 삭제. D9: Phase 5 산출물 list runtime read OUT scope 명시. D10: AC-007 phase Phase 3,4 + Phase 1 strict mode test 보강. D11: OQ-PL-2 RESOLVED. D12: AC-008 "22-key" → "21-key + 1 prefix glob". |

---

## §1 Overview

### §1.1 Goal

`GOOSE_*` 환경변수 22개에 대응하는 `MINK_*` 키를 도입하고, 두 키 공존 기간 동안 alias loader 를 단일 read 경로로 채택한다. 사용자가 dotfile / CI / Docker manifest 의 `GOOSE_*` 표기를 즉시 갱신하지 않아도 동작이 깨지지 않도록 backward-compat 를 보장하면서, **per-key per-process 1회** 의 deprecation warning 을 통해 마이그레이션을 유도한다.

### §1.2 Non-Goals

- `GOOSE_*` env var 의 완전 제거 → 별도 후속 SPEC (SPEC-MINK-ENV-CLEANUP-001), post-1.0 release + 1+ minor cycle 후
- user-data path migration (`~/.goose → ~/.mink`) → SPEC-MINK-USERDATA-MIGRATE-001 (별도)
- 외부 운영 자산 (`docker-compose.yaml`, k8s manifests) 의 GOOSE_* update → 후속 docs SPEC
- viper.BindEnv 도입 (현재 viper 미사용)
- 신규 runtime read logic 추가 (TELEGRAM_BOT_TOKEN, HISTORY_SNIP, METRICS_ENABLED, GRPC_BIND 의 read 신설은 본 SPEC 외; alias 등록만)

### §1.3 Supersedes / Depends-on

| Type | Target | Note |
|------|--------|------|
| depends_on | SPEC-MINK-BRAND-RENAME-001 | §3.1 item 12 footnote + REQ-MINK-BR-027 (분리 정책 트레일) |
| related_specs | SPEC-MINK-USERDATA-MIGRATE-001 (예정) | user-data path 부분 분리 처리 |
| related_specs | SPEC-MINK-ENV-CLEANUP-001 (예정) | post-1.0 GOOSE_* 완전 제거 |

#### §1.3.1 REQ ↔ Supersede 매핑 (D6)

본 SPEC 의 EARS REQ 가 BRAND-RENAME-001 의 REQ-MINK-BR-027 (env var 분리 정책) 을 어떻게 구현하는지의 cross-trail:

| 본 SPEC REQ | BRAND-RENAME-001 REQ | 관계 |
|-------------|----------------------|------|
| REQ-MINK-EM-004 | REQ-MINK-BR-027 | "MINK_* takes precedence" 정책 구현체 (NEW > OLD when only OLD set 의 보완: deprecation warning 1회) |
| REQ-MINK-EM-005 | REQ-MINK-BR-027 | "GOOSE_* still works with deprecation" 정책 구현체 (OLD only → backward compat 동작 보장) |
| REQ-MINK-EM-006 | REQ-MINK-BR-027 | "alias 동시 설정 시 MINK_* wins" 정책 구현체 (NEW > OLD priority + conflict warning 1회) |

---

## §2 Background

### §2.1 사용자 결정 trail

BRAND-RENAME-001 plan 단계에서 사용자가 명시 결정 (PR #163 코멘트, 2026-05-12):

1. brand rename (식별자/모듈/proto/binary/산문) 은 BRAND-RENAME-001 한 번에 atomic 처리
2. **runtime env var 처리 변경 (alias loader)** 은 별도 SPEC 으로 분리 — 이유: (a) 사용자 설치 환경의 `.bashrc` / cron / Docker manifest 마이그레이션 시간 보장, (b) deprecation warning 의 정책 결정 (frequency, channel, removal timeline) 을 충분히 검토할 시간 확보
3. user-data path (`~/.goose → ~/.mink`) 도 별도 — 데이터 손실 위험 + 마이그레이션 도구 필요

본 SPEC = 위 결정 (2) 의 이행. CHANGELOG.md line 23 ("`SPEC-MINK-ENV-MIGRATE-001`: `GOOSE_*` 21개 env vars → `MINK_*` deprecation alias loader") 가 이 약속을 명시 기록.

### §2.2 현재 상태

- BRAND-RENAME 머지 후, 코드 식별자 / 산문 / 모듈 path 는 `mink` 로 정리 완료 (commit `f0f02e4`)
- 그러나 **env var 키 자체는 `GOOSE_*` 그대로 잔존** — 22개 unique key, 174 .go reference (research.md §1)
- 사용자가 brand 가 바뀐 빌드를 실행할 때 env 표기는 여전히 `GOOSE_*` 만 인식 — 의도된 일시적 상태 (BRAND-RENAME footnote 명시)

### §2.3 본 SPEC 의 가치

- backward-compat 100% (기존 GOOSE_* 사용자 환경 그대로 동작)
- forward-compat 진입점 (MINK_* 신규 표기 작동)
- 마이그레이션 신호 (deprecation warning) 를 **noise-free** 로 emit (per-key per-process 1회)
- 단일 코드 경로 (alias loader API) 로 이후 SPEC-MINK-ENV-CLEANUP-001 의 GOOSE_* 완전 제거가 1줄 수정 (loader option flip) 으로 가능

---

## §3 Scope

### §3.1 IN scope

| # | Item |
|---|------|
| 1 | `internal/envalias/` (new package) — alias loader API 신설 (§7) |
| 2 | envOverlay (5 key) 의 alias loader 채택 |
| 3 | 분산 production read site 11 곳 의 alias loader 채택 (8 `os.Getenv` direct + 3 const-based via qwen/kimi/aliasconfig const, 15 runtime-read keys 전체 처리) |
| 4 | 4 doc-only key (TELEGRAM_BOT_TOKEN, HISTORY_SNIP, METRICS_ENABLED, GRPC_BIND) 의 alias 등록 (runtime read 신설은 OUT) |
| 5 | `GOOSE_AUTH_*` prefix deny-list 에 `MINK_AUTH_*` 추가 (`internal/hook/isolation_unix.go`, `isolation_other.go`) |
| 6 | in-tree test 의 `t.Setenv("GOOSE_*", ...)` → `t.Setenv("MINK_*", ...)` migration (alias 검증 test 1~2 곳 제외) |
| 7 | 산문/주석/error message 의 GOOSE_* 표기 → MINK_* update (alias 검증 test 의 코멘트 제외, 본 SPEC 의 documents 제외) |
| 8 | 단위 test (envalias 패키지 자체 + 각 read site 의 alias 동작) |
| 9 | integration test (실제 env set + main wire-up + 1회 warning emit 검증) |

### §3.2 OUT scope

research.md §6.2 표 참조. 핵심: GOOSE_* 완전 제거 / user-data path / 외부 운영 자산 / viper 도입 / 신규 runtime read logic 신설.

---

## §4 EARS Requirements

> 모든 REQ 는 single-sentence EARS pattern. ID = `REQ-MINK-EM-NNN` (EM = Env Migration).

### §4.1 Ubiquitous (always-active)

- **REQ-MINK-EM-001**: MINK 시스템은 모든 GOOSE_* / MINK_* env var 접근을 alias loader (`internal/envalias`) 의 단일 API 를 통해 수행한다.
- **REQ-MINK-EM-002**: alias loader 는 22개 GOOSE_* key 각각에 대응하는 MINK_* key 를 1:1 매핑으로 등록한다 (예: `GOOSE_HOME ↔ MINK_HOME`, `GOOSE_AUTH_REFRESH ↔ MINK_AUTH_REFRESH`).
- **REQ-MINK-EM-003**: alias loader 는 process 종료까지 deprecation warning 을 per-key 최대 1회만 emit 한다 (sync.Once 패턴, 22 keys → max 22 warning).

### §4.2 Event-Driven (when X, then Y)

- **REQ-MINK-EM-004**: 사용자가 `GOOSE_X` 만 설정하고 `MINK_X` 를 미설정한 상태에서 alias loader 가 X 키 read 호출을 받으면, alias loader 는 `GOOSE_X` 의 값을 반환하고 deprecation warning ("`GOOSE_X` is deprecated; please rename to `MINK_X`") 을 1회 emit 한다.
- **REQ-MINK-EM-005**: 사용자가 `MINK_X` 만 설정한 상태에서 alias loader 가 X 키 read 호출을 받으면, alias loader 는 `MINK_X` 의 값을 반환하고 deprecation warning 을 emit 하지 않는다.

### §4.3 State-Driven (while X, do Y)

- **REQ-MINK-EM-006**: `MINK_X` 와 `GOOSE_X` 가 동시에 설정된 상태에서 alias loader 가 X 키 read 호출을 받으면, alias loader 는 `MINK_X` 값을 반환하고 (NEW > OLD priority, research.md §3.4) `GOOSE_X` 는 무시하며 conflict warning ("both `MINK_X` and `GOOSE_X` set; using `MINK_X`") 을 1회 emit 한다.

### §4.4 Unwanted (shall not)

- **REQ-MINK-EM-007**: alias loader 는 동일 process 내에서 동일 key 의 deprecation warning 또는 conflict warning 을 1회를 초과하여 emit 해서는 안 된다.
- **REQ-MINK-EM-008**: alias loader 는 logger 가 nil 이거나 logging channel 이 차단된 경우에도 read 호출 자체를 실패해서는 안 된다 (warning emit 실패는 silent skip; read 결과 반환은 보장).

### §4.5 Optional (where feature exists)

- **REQ-MINK-EM-009**: Where strict mode is enabled (`Options.StrictMode == true`), the alias loader shall log a warning and return `(value="", source=SourceDefault, ok=false)` for read calls on unregistered keys. Default value is `false`; future SPEC-MINK-ENV-CLEANUP-001 의 GOOSE_* 완전 제거 시 strict=true 로 flip 가능. (한국어: "strict mode 가 활성화된 환경에서, alias loader 는 등록되지 않은 key 에 대한 read 호출에 warning 을 로그하고 default 값을 반환한다.")

---

## §5 Acceptance Criteria

> 각 AC 는 verification command 를 명시 (Given/When/Then 형식). ID = `AC-MINK-EM-NNN`.

### AC-MINK-EM-001 — alias loader 패키지 존재 + API 일치

- **Given**: feature/SPEC-MINK-ENV-MIGRATE-001 branch
- **When**: `ls internal/envalias/ && grep -n "func.*Get\|func.*MustRegister\|type EnvSource" internal/envalias/*.go`
- **Then**:
  - `internal/envalias/loader.go` 존재
  - `func (l *Loader) Get(newKey string) (value string, source EnvSource, ok bool)` 시그니처 매칭
  - `EnvSource` enum 에 `SourceMink`, `SourceGoose`, `SourceDefault` 정의
  - test file `loader_test.go` 존재

### AC-MINK-EM-002 — GOOSE_X only 시나리오 (REQ-MINK-EM-004)

- **Given**: `MINK_LOG_LEVEL` 미설정, `GOOSE_LOG_LEVEL=debug` 설정
- **When**: `minkd` 프로세스 기동 + envOverlay 호출
- **Then**:
  - `cfg.Log.Level == "debug"` (값 반환 정상)
  - stderr (zap JSON line) 에 `"msg":"deprecated env var, please rename"` + `"old":"GOOSE_LOG_LEVEL"` + `"new":"MINK_LOG_LEVEL"` 출력 1회
  - 검증: `MINK_LOG_LEVEL= GOOSE_LOG_LEVEL=debug ./minkd 2>&1 | grep -c "GOOSE_LOG_LEVEL"` → 1

### AC-MINK-EM-003 — MINK_X only 시나리오 (REQ-MINK-EM-005)

- **Given**: `MINK_LOG_LEVEL=debug` 설정, `GOOSE_LOG_LEVEL` 미설정
- **When**: `minkd` 프로세스 기동
- **Then**:
  - `cfg.Log.Level == "debug"` (값 반환 정상)
  - stderr 에 GOOSE_LOG_LEVEL 관련 warning 0회
  - 검증: `MINK_LOG_LEVEL=debug GOOSE_LOG_LEVEL= ./minkd 2>&1 | grep -c "GOOSE_LOG_LEVEL"` → 0

### AC-MINK-EM-004 — 동시 설정 시나리오 (REQ-MINK-EM-006, NEW > OLD)

- **Given**: `MINK_LOG_LEVEL=info`, `GOOSE_LOG_LEVEL=debug` 동시 설정
- **When**: `minkd` 프로세스 기동
- **Then**:
  - `cfg.Log.Level == "info"` (MINK 우선)
  - stderr 에 conflict warning ("both `MINK_LOG_LEVEL` and `GOOSE_LOG_LEVEL` set; using `MINK_LOG_LEVEL`") 1회 emit
  - 검증: stderr grep 으로 `"both"` + `"MINK_LOG_LEVEL"` 동일 라인 1회

### AC-MINK-EM-005 — sync.Once 검증 (REQ-MINK-EM-003, REQ-MINK-EM-007)

- **Given**: `GOOSE_LOG_LEVEL=debug` 설정
- **When**: alias loader 의 `Get("LOG_LEVEL")` 를 동일 process 내에서 100회 호출 (table-driven test)
- **Then**:
  - 모든 호출이 동일 값 ("debug") 반환
  - logger 의 fake/observer 에 deprecation warning 1회만 기록
  - 검증: `go test -v ./internal/envalias -run TestSyncOncePerKey` PASS

### AC-MINK-EM-006 — logger nil safety (REQ-MINK-EM-008)

- **Given**: `Loader` 가 `Options{Logger: nil}` 으로 생성된 상태
- **When**: `Get("LOG_LEVEL")` 호출 (`GOOSE_LOG_LEVEL=debug` 설정)
- **Then**:
  - panic 발생 안 함
  - 값 반환 정상 ("debug")
  - 검증: `go test -v ./internal/envalias -run TestNilLoggerSafety` PASS

### AC-MINK-EM-007 — in-tree test migration 완료

- **Given**: feature branch 의 production .go 파일 + test .go 파일
- **When**: `grep -rn 't\.Setenv("GOOSE_' --include="*.go" . | grep -v "/vendor/" | grep -v "envalias/"`
- **Then**: 결과 0건 (alias 검증 전용 test 인 `internal/envalias/loader_test.go` 만 GOOSE_* 사용 허용)
- 검증: 위 grep 명령 + `wc -l` → 0

### AC-MINK-EM-008 — alias 동작 검증 test 존재

- **Given**: feature branch
- **When**: `go test -v ./internal/envalias`
- **Then**:
  - 최소 6 sub-test PASS (GOOSE only / MINK only / both / sync.Once / nil logger / strict mode)
  - 모든 22 key 의 mapping 등록 검증 (table-driven, `TestAllKeysRegistered`)

### AC-MINK-EM-009 — env scrub deny-list 확장 (`MINK_AUTH_*`)

- **Given**: `internal/hook/isolation_unix.go`, `isolation_other.go`
- **When**: `grep -n "MINK_AUTH_\|GOOSE_AUTH_" internal/hook/isolation_*.go`
- **Then**:
  - 두 파일 모두 `MINK_AUTH_*` prefix 도 deny-list 에 포함
  - 기존 `GOOSE_AUTH_*` prefix 유지 (backward compat)
  - 단위 test: `MINK_AUTH_TOKEN=zzz` 가 scrubEnv 결과에서 제외됨
- 검증: `go test -v ./internal/hook -run TestScrubEnv_DenyList` PASS

### AC-MINK-EM-010 — 산문/주석/error message migration 완료

- **Given**: feature branch
- **When**: `grep -rn "GOOSE_" --include="*.go" . | grep -v "/vendor/" | grep -v "envalias/" | grep -v "_test.go"`
- **Then**:
  - 결과는 alias 등록 (mapping 정의) 외 0건
  - error message / log message / Korean comments 모두 `MINK_*` 표기로 update
  - exception: `internal/envalias/keys.go` 의 alias mapping 정의 (필수)
- 검증: 위 grep + 수동 review (alias mapping 만 GOOSE_* 표기 허용)

---

## §6 Technical Approach (6 Phases, atomic commits)

각 Phase 는 1개 PR-mergeable atomic commit. commit 메시지는 `feat(env-alias): <phase 요약>` 형식.

### Phase 1 — Inventory + Loader Skeleton

산출물:
- `internal/envalias/loader.go` — `Loader` struct + `Options` + `New()` + `Get(newKey string) (value string, source EnvSource, ok bool)` + `EnvSource` enum
- `internal/envalias/keys.go` — 22 key mapping table (GOOSE_X ↔ MINK_X) const map
- `internal/envalias/doc.go` — package doc 주석 (REQ-MINK-EM-001 ~ REQ-MINK-EM-009 트레일)
- `internal/envalias/loader_test.go` — table-driven test skeleton (TestAllKeysRegistered + 6 sub-test 골격)
- 빈 production migration: 어떤 read site 도 변경하지 않음 (foundation only)

검증: `go build ./internal/envalias && go test ./internal/envalias`

### Phase 2 — MINK_* primary read 채택 (envOverlay 5 keys)

산출물:
- `internal/config/env.go` 의 5 key (LOG_LEVEL, HEALTH_PORT, GRPC_PORT, LOCALE, LEARNING_ENABLED) 를 alias loader 호출로 전환
- 기존 envLookup 함수 시그니처 유지 (test 주입성 보존), 단 envOverlay 내부 구현이 alias loader 통과
- envOverlay 의 logger 를 alias loader 의 logger 로 공유

검증:
- `go test ./internal/config -run TestLoad_GooseEnvLocale` PASS (backward compat)
- 신규 test: `TestLoad_MinkEnvLocale` (MINK_LOCALE 단독 시나리오)
- 신규 test: `TestLoad_BothEnvLocale_PrefersMink` (NEW > OLD)

### Phase 3 — GOOSE_* fallback + sync.Once warning 채택 (분산 read site 11 곳)

> **Frame anchor**: 모든 file:line 참조는 base commit `f0f02e4` (= origin/main, BRAND-RENAME-001 squash merge 직후) 기준이다. Phase 3 commit 직전 line 번호 drift 가능 — Phase 3 atomic commit 시 grep 으로 키워드 (`os.Getenv("GOOSE_X"`, `envQwenRegion`, `envKimiRegion`, `homeEnv`) 재확인 후 적용.

산출물 (모두 단일 commit, 8 `os.Getenv` direct callsite + 3 const-based callsite = 11 read sites):

`os.Getenv` direct callsite (8):
- `internal/audit/dual.go:140` — `os.Getenv("GOOSE_HOME")`
- `internal/config/config.go:274` — `os.Getenv("GOOSE_HOME")` (resolveGooseHome 함수 안)
- `internal/transport/grpc/server.go:169` — `os.Getenv("GOOSE_GRPC_REFLECTION")`
- `internal/transport/grpc/server.go:277` — `os.Getenv("GOOSE_GRPC_MAX_RECV_MSG_BYTES")`
- `internal/transport/grpc/server.go:297` — `os.Getenv("GOOSE_SHUTDOWN_TOKEN")`
- `internal/hook/handlers.go:270` — `os.Getenv("GOOSE_HOOK_TRACE")`
- `internal/hook/permission.go:251` — `os.Getenv("GOOSE_HOOK_NON_INTERACTIVE")`
- `cmd/minkd/main.go:89` — `os.Getenv("GOOSE_ALIAS_STRICT")`

const-based callsite (3, const 정의 + read 함수 묶음):
- `internal/command/adapter/aliasconfig/loader.go:32` (const `homeEnv = "GOOSE_HOME"`) + `loader.go:92` (callsite `os.Getenv(homeEnv)`)
- `internal/llm/provider/qwen/client.go:38` (const `envQwenRegion = "GOOSE_QWEN_REGION"`) + `client.go:99` (callsite)
- `internal/llm/provider/kimi/client.go:40` (const `envKimiRegion = "GOOSE_KIMI_REGION"`) + `client.go:135` (callsite)

각 호출부의 변경 패턴:
```go
// before
v := os.Getenv("GOOSE_HOME")

// after
v, _, _ := envaliasLoader.Get("HOME")  // returns "" + SourceDefault when neither set
```

검증: 각 패키지의 기존 test 모두 PASS + 신규 alias 동작 test (각 read site 마다 1개 sub-test)

### Phase 4 — env scrub deny-list 확장 + in-tree test migration

> **Frame anchor**: 아래 카운트는 base commit `f0f02e4` 기준 `grep -c` 결과. Phase 4 commit 직전 재실행으로 검증.

산출물:
- `internal/hook/isolation_unix.go`, `isolation_other.go` — `MINK_AUTH_*` prefix 추가
- `internal/hook/hook_test.go` — `TestScrubEnv_DenyList` 에 `MINK_AUTH_TOKEN`, `MINK_AUTH_REFRESH` 케이스 추가

in-tree test 의 `t.Setenv("GOOSE_*", ...)` 28 곳 전수 → `t.Setenv("MINK_*", ...)` 로 변경 (검증 명령: `grep -rEn 't\.Setenv\("GOOSE_' --include='*.go' .` → 28):

| File | Count | Note |
|------|-------|------|
| `cmd/minkd/integration_test.go` | 9 | minkd integration wire tests |
| `internal/command/adapter/aliasconfig/integration_test.go` | 5 | alias config integration |
| `internal/command/adapter/aliasconfig/loader_amend_test.go` | 3 | alias loader amend variants |
| `internal/command/adapter/aliasconfig/loader_test.go` | 2 | alias loader unit tests |
| `internal/command/adapter/aliasconfig/loader_p3_test.go` | 1 | P3 phase test |
| `internal/command/adapter/aliasconfig/merge_test.go` | 1 | alias merge logic |
| `internal/llm/provider/qwen/client_test.go` | 2 | qwen region resolution |
| `internal/llm/provider/kimi/client_test.go` | 2 | kimi region resolution |
| `internal/hook/hook_test.go` | 2 | hook env handling |
| `internal/config/config_test.go` | 1 | config env overlay |
| **Total** | **28** | |

`os.Setenv("GOOSE_*", ...)` 6 곳 전수 → `t.Setenv("MINK_*", ...)` 로 변경 (research.md §2.4 일관성, R10 mitigation: process-wide env 오염 회피):

| File | Count | Note |
|------|-------|------|
| `internal/audit/dual_test.go` | 3 | global audit path test |
| `internal/transport/grpc/server_test.go` | 2 | gRPC env override test |
| `internal/tools/builtin/terminal/bash_test.go` | 1 | bash builtin env test |
| **Total** | **6** | (모두 `t.Setenv` 로 전환 + parallel-safe) |

exception: `internal/envalias/loader_test.go` 의 alias 검증 test 만 GOOSE_* 직접 setenv 유지 (의도된 backward compat 검증)

검증: AC-MINK-EM-007 의 grep + 전체 `go test ./...` PASS

### Phase 5 — 산문/주석/error message + flag help text update

> **Boundary clarification (D9)**: 본 Phase 는 alias 등록 + 산문 정리만 처리. runtime read logic 신설 (현재 미구현 4 keys: `GOOSE_TELEGRAM_BOT_TOKEN`, `GOOSE_HISTORY_SNIP`, `GOOSE_METRICS_ENABLED`, `GOOSE_GRPC_BIND`) 은 OUT scope (§11 Exclusion #5/#6).
>
> **Frame anchor**: file:line 참조는 base commit `f0f02e4` 기준. Phase 5 commit 직전 line drift 가능 — keyword grep 으로 재확인.

산출물:
- error message: `internal/cli/commands/messaging_telegram.go:68` (`GOOSE_TELEGRAM_BOT_TOKEN` → `MINK_TELEGRAM_BOT_TOKEN` + alias hint 추가 "(legacy `GOOSE_TELEGRAM_BOT_TOKEN` also accepted)")
- flag help: 동 파일 line 138 동일 패턴
- error message: `internal/messaging/telegram/keyring_nokeyring.go:20,25`
- 주석 (한국어): research.md §1.1 의 모든 production read site 의 한글 주석 (`// GOOSE_X 환경변수 키` → `// MINK_X 환경변수 키 (legacy alias: GOOSE_X)`)
- @MX:NOTE / @MX:SPEC tag update — `internal/config/env.go:18` 의 "@MX:SPEC: SPEC-GOOSE-CONFIG-001 §6.2" 는 본 SPEC reference 추가 (`SPEC-MINK-ENV-MIGRATE-001 §7` 추가)
- READ-ONLY: `internal/envalias/keys.go` 의 alias mapping 정의 (GOOSE_* literal 등장 의도된 자료) 는 변경 금지
- READ-ONLY: 본 SPEC 의 산출 documents (`spec.md`, `research.md`) 의 GOOSE_* 표기는 의도된 인용

검증: AC-MINK-EM-010 grep + visual review

### Phase 6 — Integration test + final verification

산출물:
- `cmd/minkd/integration_test.go` — `TestMain_EnvAlias_GooseHomeOnly` (실제 minkd 기동 + alias 검증, exec 기반)
- `cmd/minkd/integration_test.go` — `TestMain_EnvAlias_MinkHomeOnly`
- `cmd/minkd/integration_test.go` — `TestMain_EnvAlias_BothSet_PrefersMink` (NEW > OLD 시나리오 + warning emit 검증)
- 전체 `go test ./...` PASS
- 전체 `go vet ./...` PASS
- `golangci-lint run` PASS (없는 경우 skip)

검증: AC-MINK-EM-001 ~ AC-MINK-EM-010 전수 PASS

---

## §7 Alias Loader API 설계 명세

### §7.1 Package layout

```
internal/envalias/
├── doc.go           # package doc + REQ trace
├── loader.go        # Loader, Options, New, Get
├── keys.go          # 22 key mapping table
└── loader_test.go   # table-driven test
```

### §7.2 Type definitions

```go
package envalias

// EnvSource indicates which env var supplied the returned value.
type EnvSource int

const (
    SourceDefault EnvSource = iota // 둘 다 unset
    SourceMink                     // MINK_X 사용
    SourceGoose                    // GOOSE_X 사용 (deprecated)
)

// Options configures the Loader.
type Options struct {
    Logger     *zap.Logger // nil safe
    EnvLookup  func(string) string // nil → os.Getenv (test 주입용)
    StrictMode bool        // true → 등록되지 않은 newKey read 시 error (default false; SPEC-MINK-ENV-CLEANUP-001 에서 flip)
}

// Loader is the single entry point for env var reads in MoAI/MINK.
type Loader struct {
    opts        Options
    warnedOnce  map[string]*sync.Once // per-key sync.Once
    warnedMu    sync.Mutex            // map 자체 보호
}

// New constructs a Loader with the given options.
func New(opts Options) *Loader { ... }

// Get reads the env var by its short newKey ("LOG_LEVEL", "HOME", ...).
//   - returns (MINK_X value, SourceMink, true) when MINK_X set
//   - returns (GOOSE_X value, SourceGoose, true) when only GOOSE_X set; emits deprecation warning per-key once
//   - returns ("", SourceDefault, false) when neither set
//   - if both set: returns (MINK_X value, SourceMink, true) + emits conflict warning per-key once
func (l *Loader) Get(newKey string) (value string, source EnvSource, ok bool)
```

### §7.3 keys.go — mapping table

```go
package envalias

// keyMappings maps the short newKey ("LOG_LEVEL") to the MINK_/GOOSE_ pair.
// All 22 keys from research.md §1.1 inventory are registered here.
var keyMappings = map[string]struct {
    Mink  string
    Goose string
}{
    "HOME":                       {Mink: "MINK_HOME",                       Goose: "GOOSE_HOME"},
    "LOG_LEVEL":                  {Mink: "MINK_LOG_LEVEL",                  Goose: "GOOSE_LOG_LEVEL"},
    "HEALTH_PORT":                {Mink: "MINK_HEALTH_PORT",                Goose: "GOOSE_HEALTH_PORT"},
    "GRPC_PORT":                  {Mink: "MINK_GRPC_PORT",                  Goose: "GOOSE_GRPC_PORT"},
    "LOCALE":                     {Mink: "MINK_LOCALE",                     Goose: "GOOSE_LOCALE"},
    "LEARNING_ENABLED":           {Mink: "MINK_LEARNING_ENABLED",           Goose: "GOOSE_LEARNING_ENABLED"},
    "CONFIG_STRICT":              {Mink: "MINK_CONFIG_STRICT",              Goose: "GOOSE_CONFIG_STRICT"},
    "GRPC_REFLECTION":            {Mink: "MINK_GRPC_REFLECTION",            Goose: "GOOSE_GRPC_REFLECTION"},
    "GRPC_MAX_RECV_MSG_BYTES":    {Mink: "MINK_GRPC_MAX_RECV_MSG_BYTES",    Goose: "GOOSE_GRPC_MAX_RECV_MSG_BYTES"},
    "SHUTDOWN_TOKEN":             {Mink: "MINK_SHUTDOWN_TOKEN",             Goose: "GOOSE_SHUTDOWN_TOKEN"},
    "HOOK_TRACE":                 {Mink: "MINK_HOOK_TRACE",                 Goose: "GOOSE_HOOK_TRACE"},
    "HOOK_NON_INTERACTIVE":       {Mink: "MINK_HOOK_NON_INTERACTIVE",       Goose: "GOOSE_HOOK_NON_INTERACTIVE"},
    "ALIAS_STRICT":               {Mink: "MINK_ALIAS_STRICT",               Goose: "GOOSE_ALIAS_STRICT"},
    "QWEN_REGION":                {Mink: "MINK_QWEN_REGION",                Goose: "GOOSE_QWEN_REGION"},
    "KIMI_REGION":                {Mink: "MINK_KIMI_REGION",                Goose: "GOOSE_KIMI_REGION"},
    "TELEGRAM_BOT_TOKEN":         {Mink: "MINK_TELEGRAM_BOT_TOKEN",         Goose: "GOOSE_TELEGRAM_BOT_TOKEN"},
    "AUTH_TOKEN":                 {Mink: "MINK_AUTH_TOKEN",                 Goose: "GOOSE_AUTH_TOKEN"},
    "AUTH_REFRESH":               {Mink: "MINK_AUTH_REFRESH",               Goose: "GOOSE_AUTH_REFRESH"},
    "HISTORY_SNIP":               {Mink: "MINK_HISTORY_SNIP",               Goose: "GOOSE_HISTORY_SNIP"},
    "METRICS_ENABLED":            {Mink: "MINK_METRICS_ENABLED",            Goose: "GOOSE_METRICS_ENABLED"},
    "GRPC_BIND":                  {Mink: "MINK_GRPC_BIND",                  Goose: "GOOSE_GRPC_BIND"},
    // Note: GOOSE_AUTH_ prefix is handled separately in internal/hook/isolation_*.go
    //       (env scrub deny-list, not single-key alias).
}
```

(21 keys + 1 prefix glob; AUTH_ prefix 는 별도 처리 = research.md §1.1 의 22nd entry.)

### §7.4 Warning channel + format

zap structured log (project 기존 logger). format:
```
{"level":"warn","ts":"2026-05-13T12:00:00.000+0900","msg":"deprecated env var, please rename","old":"GOOSE_LOG_LEVEL","new":"MINK_LOG_LEVEL","spec":"SPEC-MINK-ENV-MIGRATE-001"}
```
conflict format:
```
{"level":"warn","ts":"...","msg":"both legacy and new env var set; using new key","new":"MINK_LOG_LEVEL","old":"GOOSE_LOG_LEVEL","value_source":"MINK_LOG_LEVEL"}
```

WARN level — 사용자가 운영 환경에서 가시화 (INFO 면 default level 에서 silent), error level 은 동작 정상이므로 부적절.

### §7.5 sync.Once 메커니즘

```go
func (l *Loader) emitDeprecationWarning(newKey, oldKey string) {
    l.warnedMu.Lock()
    once, ok := l.warnedOnce[newKey]
    if !ok {
        once = &sync.Once{}
        l.warnedOnce[newKey] = once
    }
    l.warnedMu.Unlock()
    once.Do(func() {
        if l.opts.Logger != nil {
            l.opts.Logger.Warn("deprecated env var, please rename",
                zap.String("old", oldKey),
                zap.String("new", newKey),
                zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
            )
        }
    })
}
```

per-key per-process 1회 보장. logger nil 이면 silent skip (REQ-MINK-EM-008).

---

## §8 Dependencies

### §8.1 선행 SPEC

- **SPEC-MINK-BRAND-RENAME-001** (depends_on, hard) — 본 SPEC 의 `internal/envalias/` 패키지 신설은 BRAND-RENAME 가 정리한 module path (`github.com/modu-ai/mink`) 와 일관성 있는 import path 사용 전제.

### §8.2 외부 의존

- `go.uber.org/zap` (이미 프로젝트 의존성, `internal/core/logger.go` 사용 중)
- Go 표준 `sync.Once`, `os.Getenv` (추가 의존성 불필요)

### §8.3 Tooling

- `go vet`, `go test`, `golangci-lint` (있는 경우)
- 사전 검증: `gofmt -l internal/envalias` push 전 0 결과

---

## §9 Risks & Mitigations

| Risk ID | Risk | Severity | Mitigation |
|---------|------|----------|------------|
| R1 | alias loader 도입으로 기존 envOverlay 동작 미세 변경 (e.g., 동일 key 의 read 호출 횟수 증가) | Low | envLookup 함수 주입성 유지 + 기존 test PASS 확인 (Phase 2 검증) |
| R2 | sync.Once map 의 concurrent write 경합 (multi-goroutine env read) | Medium | warnedMu mutex 보호 + race detector 로 test (`go test -race`) |
| R3 | logger nil 상황에서 warning emit 실패가 read 자체를 실패시킬 위험 | Medium | REQ-MINK-EM-008 + AC-MINK-EM-006 명시 검증 |
| R4 | Phase 4 의 28 t.Setenv + 6 os.Setenv test migration 에서 누락 발생 시 backward compat regression | Medium | grep 자동화 (AC-MINK-EM-007) + 전체 `go test ./...` |
| R5 | `GOOSE_TELEGRAM_BOT_TOKEN` 의 runtime read 미구현 — alias 등록만 하면 사용자 혼란 가능 ("env 설정했는데 동작 안 함") | Low | error message 의 hint 만 update (Phase 5), runtime read 신설은 OUT scope 명시 |
| R6 | `GOOSE_AUTH_*` prefix glob 변경 시 기존 보안 테스트 회귀 | High | AC-MINK-EM-009 + 명시 sub-test 추가 + race detector |
| R7 | `internal/envalias/` 패키지 import cycle 위험 (logger → envalias → logger) | Low | logger 는 외부에서 주입 (Options.Logger), envalias 는 zap 만 import |
| R8 | zap logger 의 stderr 출력이 production 환경에서 noise 로 인식 | Low | per-key 1회만 emit + WARN level (DEBUG/INFO 가 아님) + structured key/value 로 grep 용이 |
| R11 | 사용자 dotfile (`.bashrc`, `.zshrc`) 에 `export GOOSE_HOME=...` 만 있을 때 첫 alias warning 출력 | Low | release notes 에 dotfile rewrite guidance 명시. AC-MINK-EM-005 의 warning 메시지 format 에 user-actionable 한 hint ("Replace `GOOSE_HOME` with `MINK_HOME` in your shell rc files") 포함 |
| R12 | Docker / k8s container env spec 에 `GOOSE_*` 만 정의된 경우 alias 동작은 OK 이나 manifest update 가 외부 작업 영역 | Low | 본 SPEC IN-scope 외 명시 (§11 #3). 후속 docs SPEC trigger 또는 release notes link 로 처리. alias loader 가 자동 흡수하므로 즉시 깨지지 않음 |
| R13 | SPEC-MINK-USERDATA-MIGRATE-001 와 머지 순서 의존 — `MINK_HOME=$HOME/.goose` 같은 hybrid 설정 가능 | Low | alias loader 는 value 를 opaque string 으로 처리 (path 해석 안 함). `~/.goose ↔ ~/.mink` 마이그레이션은 USERDATA-MIGRATE 영역. 본 SPEC + USERDATA-MIGRATE 의 머지 순서 무관 (독립 SPEC) |

---

## §10 References

### 본 repo

- `internal/config/env.go` — envOverlay reference impl
- `internal/hook/isolation_unix.go`, `isolation_other.go` — env scrub deny-list (AC-MINK-EM-009 대상)
- `internal/core/logger.go` — zap logger NewLogger API
- `cmd/minkd/main.go:78-110` — alias config loader wire-up (SPEC-GOOSE-ALIAS-CONFIG-001 패턴 참고)
- `CHANGELOG.md` line 23, 47 — 본 SPEC 의 후속성 + 분리 정책 명시

### 선행 SPEC

- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/` — §3.1 item 12 footnote, REQ-MINK-BR-027

### Industry pattern

- HashiCorp Vault hclog (https://github.com/hashicorp/go-hclog) — structured deprecation warning pattern
- HashiCorp Terraform CHANGELOG 0.10~0.13 — env var rename + removal timeline
- Kubernetes #34058 — kubectl rename pattern

### 본 SPEC 산출

- `research.md` (sibling) — 22 key 인벤토리, 모듈 분포, 위험 시나리오 전수
- `acceptance.md` (sibling) — Given/When/Then full test scenario

---

## §11 Exclusions (What NOT to Build)

[HARD] 본 SPEC 안에서 명시 금지 목록 (research.md §6.2 표 매핑):

| # | Exclusion | Rationale | 처리 경로 |
|---|-----------|-----------|----------|
| 1 | `GOOSE_*` env var 의 alias 등록 삭제 (완전 제거) | 사용자 마이그레이션 시간 보장 (최소 1+ minor release) | SPEC-MINK-ENV-CLEANUP-001 (후속) |
| 2 | user-data path migration (`~/.goose → ~/.mink`) | 데이터 손실 위험 + 별도 마이그레이션 도구 필요 | SPEC-MINK-USERDATA-MIGRATE-001 (별도) |
| 3 | `.env.example`, `Dockerfile`, `docker-compose.yaml`, k8s manifests 의 `GOOSE_*` 표기 update | 외부 운영 자원 (alias loader 가 자동 흡수) | 후속 docs SPEC + release notes |
| 4 | viper 도입 (현재 `os.Getenv` 직접 호출 → viper 추상화로 전환) | 본 SPEC 은 alias 도입만 — refactor 범위 분리 | 별도 refactor SPEC (도입 미정) |
| 5 | `GOOSE_TELEGRAM_BOT_TOKEN` 의 runtime read logic 신설 (현재는 error message hint 만) | behavior change scope minimization | 별도 SPEC (필요 시) |
| 6 | `GOOSE_HISTORY_SNIP`, `GOOSE_METRICS_ENABLED`, `GOOSE_GRPC_BIND` 의 runtime read logic 신설 | 동일 (현재 미구현 feature gate) | 각 feature 의 별도 SPEC |
| 7 | cobra flag 의 default value alias (e.g., `--token` flag default 가 GOOSE_TELEGRAM_BOT_TOKEN 참조 시) | 현재 코드에 해당 패턴 없음 (research.md §1.1 line 16 확인) | 본 SPEC 외 |

---

End of SPEC-MINK-ENV-MIGRATE-001 spec.md.

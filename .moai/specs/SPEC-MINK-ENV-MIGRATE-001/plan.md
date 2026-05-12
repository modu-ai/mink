---
id: SPEC-MINK-ENV-MIGRATE-001
version: "0.1.1"
status: draft
created_at: 2026-05-13
updated_at: 2026-05-13
author: manager-spec
priority: High
labels: [env-migration, deprecation, alias-loader, brand-cleanup, plan-document]
issue_number: null
depends_on: [SPEC-MINK-BRAND-RENAME-001]
related_specs: [SPEC-MINK-USERDATA-MIGRATE-001]
phase: meta
lifecycle: spec-anchored
description: "Implementation plan — phase roadmap, technical approach, risks for SPEC-MINK-ENV-MIGRATE-001"
---

# Plan — SPEC-MINK-ENV-MIGRATE-001

> 본 plan 은 spec.md §6 Technical Approach 의 6 phase 를 atomic commit 단위로 구체화한 구현 계획이다.
> 각 phase 는 1개 PR-mergeable commit (squash 가능) + 검증 명령 + 누적 누락 시나리오 / 회귀 위험 정리를 포함.

## §1 Implementation Roadmap

| Phase | Atomic Commit Title | Files Touched (estimated) | LSP/Test 검증 | Priority |
|-------|--------------------|---------------------------|----------------|----------|
| 1 | feat(env-alias): introduce envalias package skeleton + 22 key mapping table | +4 (new: loader.go, keys.go, doc.go, loader_test.go) | go build + go test ./internal/envalias | Critical |
| 2 | feat(env-alias): adopt alias loader in config.envOverlay (5 keys) | ~3 (env.go, env_test.go, config.go test additions) | go test ./internal/config | Critical |
| 3 | feat(env-alias): migrate 11 distributed production read sites | ~13 (audit/dual.go, config/config.go, aliasconfig/loader.go, transport/grpc/server.go (3 callsite), hook/handlers.go, hook/permission.go, cmd/minkd/main.go, llm/provider/qwen/client.go, kimi/client.go + each test) | go test ./... | Critical |
| 4 | feat(env-alias): extend env scrub deny-list (MINK_AUTH_*) + migrate 28 t.Setenv + 6 os.Setenv → t.Setenv("MINK_*") | ~14 (isolation_unix.go, isolation_other.go, hook_test.go + 10 test files with t.Setenv migration + 3 test files with os.Setenv → t.Setenv migration) | go test ./internal/hook + grep verification | High |
| 5 | docs(env-alias): update prose / error messages / @MX:SPEC tags for MINK_* | ~8 (messaging_telegram.go, keyring_nokeyring.go, qwen/kimi client.go comments, env.go @MX tag) | grep verification + visual review | Medium |
| 6 | test(env-alias): integration test for main wire-up + final pass | ~3 (integration_test.go new TestMain_EnvAlias_* + final cleanups) | go test ./... + go vet ./... | High |

총 6 commits → 1 squash PR (선택), 또는 6 chained PR (lessons #9 wave-split 정책 — 본 SPEC 은 단일 wave 권장: 22 key 의 atomicity 보장이 더 중요).

## §2 Phase 1 — Loader Skeleton

### §2.1 산출 detail

#### `internal/envalias/loader.go`

```go
// Package envalias provides a single entry point for GOOSE_* → MINK_* env var alias resolution.
//
// Per SPEC-MINK-ENV-MIGRATE-001, every env var read in MoAI/MINK MUST go through this loader.
// The loader implements per-key per-process sync.Once deprecation warnings to guide migration
// without spamming production logs.
package envalias

import (
    "os"
    "sync"

    "go.uber.org/zap"
)

type EnvSource int

const (
    SourceDefault EnvSource = iota
    SourceMink
    SourceGoose
)

func (s EnvSource) String() string {
    switch s {
    case SourceMink:
        return "mink"
    case SourceGoose:
        return "goose"
    default:
        return "default"
    }
}

type Options struct {
    Logger     *zap.Logger
    EnvLookup  func(string) string
    StrictMode bool
}

type Loader struct {
    opts       Options
    warnedOnce map[string]*sync.Once
    warnedMu   sync.Mutex
}

func New(opts Options) *Loader {
    if opts.EnvLookup == nil {
        opts.EnvLookup = os.Getenv
    }
    return &Loader{
        opts:       opts,
        warnedOnce: make(map[string]*sync.Once),
    }
}

// Get 은 alias loader 의 단일 진입점. newKey 는 keys.go 에 등록된 short key ("LOG_LEVEL", "HOME", ...).
func (l *Loader) Get(newKey string) (value string, source EnvSource, ok bool) {
    pair, registered := keyMappings[newKey]
    if !registered {
        if l.opts.StrictMode {
            // future SPEC-MINK-ENV-CLEANUP-001 에서 활용
            l.logUnknownKey(newKey)
        }
        return "", SourceDefault, false
    }

    minkVal := l.opts.EnvLookup(pair.Mink)
    gooseVal := l.opts.EnvLookup(pair.Goose)

    switch {
    case minkVal != "" && gooseVal != "":
        l.emitConflictWarning(pair.Mink, pair.Goose)
        return minkVal, SourceMink, true
    case minkVal != "":
        return minkVal, SourceMink, true
    case gooseVal != "":
        l.emitDeprecationWarning(pair.Mink, pair.Goose)
        return gooseVal, SourceGoose, true
    default:
        return "", SourceDefault, false
    }
}

func (l *Loader) emitDeprecationWarning(newFullKey, oldFullKey string) {
    once := l.onceFor(newFullKey)
    once.Do(func() {
        if l.opts.Logger == nil {
            return
        }
        l.opts.Logger.Warn("deprecated env var, please rename",
            zap.String("old", oldFullKey),
            zap.String("new", newFullKey),
            zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
        )
    })
}

func (l *Loader) emitConflictWarning(newFullKey, oldFullKey string) {
    once := l.onceFor(newFullKey + "::conflict")
    once.Do(func() {
        if l.opts.Logger == nil {
            return
        }
        l.opts.Logger.Warn("both legacy and new env var set; using new key",
            zap.String("new", newFullKey),
            zap.String("old", oldFullKey),
            zap.String("value_source", newFullKey),
            zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
        )
    })
}

func (l *Loader) onceFor(token string) *sync.Once {
    l.warnedMu.Lock()
    defer l.warnedMu.Unlock()
    once, ok := l.warnedOnce[token]
    if !ok {
        once = &sync.Once{}
        l.warnedOnce[token] = once
    }
    return once
}

func (l *Loader) logUnknownKey(newKey string) {
    if l.opts.Logger != nil {
        l.opts.Logger.Warn("envalias.Get called with unregistered key",
            zap.String("newKey", newKey),
            zap.String("spec", "SPEC-MINK-ENV-MIGRATE-001"),
        )
    }
}
```

#### `internal/envalias/keys.go`

spec.md §7.3 의 22-key mapping table 그대로 (21 single-key + AUTH_ prefix 는 hook isolation 에서 별도 처리, doc.go 에 명시).

#### `internal/envalias/doc.go`

REQ-MINK-EM-001 ~ REQ-MINK-EM-009 의 일대일 트레일 + @MX:SPEC tag 부착.

#### `internal/envalias/loader_test.go`

Phase 1 의 test:
- `TestEnvSourceString` — enum String() 검증
- `TestNew_DefaultsEnvLookupToOsGetenv` — opts.EnvLookup nil → os.Getenv fallback
- `TestGet_UnregisteredKey_ReturnsDefault` — 등록되지 않은 key 시 (default false, strict mode off)
- `TestStrictMode_UnknownKey_Logs` — REQ-MINK-EM-009 (strict mode on → unregistered key 시 warning log + default 값 반환). Phase 1 안에서 strict mode 구현 + test 동시 작성
- `TestAllKeysRegistered` — 21 single-key + 1 prefix glob (AUTH_, isolation 별도 처리) 매핑 검증 (table-driven). REQ-MINK-EM-002 의 22-key 매핑은 21 single-key entries (keys.go) + 1 AUTH_ prefix glob (isolation_*.go 별도 처리) 의 합으로 해석.

### §2.2 검증

```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
go build ./internal/envalias
go test -race ./internal/envalias
gofmt -l ./internal/envalias
```

## §3 Phase 2 — envOverlay adoption

### §3.1 변경 detail

`internal/config/env.go` 의 `envOverlay` 함수 시그니처는 envLookup 주입 유지하되, 내부에서 `envalias.Loader` 를 사용:

```go
// before
v := envLookup("GOOSE_LOG_LEVEL")
if v != "" { cfg.Log.Level = v }

// after (loader 주입 또는 envOverlay 내부 생성)
loader := envalias.New(envalias.Options{Logger: logger, EnvLookup: envLookup})
if v, _, ok := loader.Get("LOG_LEVEL"); ok {
    cfg.Log.Level = v
}
```

5 keys 모두 동일 패턴. `cfg.Transport.HealthPort` 의 strconv.Atoi 처럼 type conversion 이 필요한 곳은 alias loader 가 string 반환 후 호출부에서 변환 (loader 가 typed API 신설하면 over-engineering — 호출부 conversion 유지).

### §3.2 신규 test

`internal/config/env_test.go` 신설:
- `TestLoad_MinkEnvLocale` — MINK_LOCALE 단독 시나리오
- `TestLoad_BothEnvLocale_PrefersMink` — NEW > OLD
- `TestLoad_EnvOverlay_DeprecationWarningOnGooseOnly` — observer logger 로 warning 1회 emit 검증

기존 `TestLoad_GooseEnvLocale` 는 backward compat 보존 검증 (alias 통해 동작 유지).

## §4 Phase 3 — 분산 production read site migration

### §4.1 변경 site 목록 (각 줄 = 1 production callsite, 총 11)

> **Frame anchor**: 모든 file:line 참조는 base commit `f0f02e4` 기준. Phase 3 commit 직전 grep 으로 keyword 재확인 (line drift 가능).

`os.Getenv` direct callsite (8):

1. `internal/audit/dual.go:140` — `os.Getenv("GOOSE_HOME")` → `loader.Get("HOME")`
2. `internal/config/config.go:274` — `os.Getenv("GOOSE_HOME")` → `loader.Get("HOME")` (resolveGooseHome 함수 안)
3. `internal/transport/grpc/server.go:169` — `GOOSE_GRPC_REFLECTION`
4. `internal/transport/grpc/server.go:277` — `GOOSE_GRPC_MAX_RECV_MSG_BYTES`
5. `internal/transport/grpc/server.go:297` — `GOOSE_SHUTDOWN_TOKEN`
6. `internal/hook/handlers.go:270` — `GOOSE_HOOK_TRACE`
7. `internal/hook/permission.go:251` — `GOOSE_HOOK_NON_INTERACTIVE`
8. `cmd/minkd/main.go:89` — `GOOSE_ALIAS_STRICT`

const-based callsite (3):

9. `internal/command/adapter/aliasconfig/loader.go:32` (const `homeEnv = "GOOSE_HOME"`) + `loader.go:92` (callsite `os.Getenv(homeEnv)`) — 정정 패턴은 §4.5 OQ-PL-2 RESOLVED 참조 (const 이름 유지, 값만 short key 변경)
10. `internal/llm/provider/qwen/client.go:38` (const `envQwenRegion`) + `client.go:99` (callsite) — OQ-PL-2 RESOLVED 동일 패턴
11. `internal/llm/provider/kimi/client.go:40` (const `envKimiRegion`) + `client.go:135` (callsite) — OQ-PL-2 RESOLVED 동일 패턴

### §4.2 Loader 인스턴스 공유 전략

각 패키지가 alias loader 를 어떻게 받을 것인가?

Option A — global loader (싱글톤):
- 장점: 호출부 수정 최소화
- 단점: test 시 환경변수 격리 어려움 (envLookup 주입 어려움)

Option B — DI 통한 주입 (struct field, function parameter):
- 장점: 테스트 격리 보장, 명시적 의존성
- 단점: 함수 시그니처 변경 필요한 경우 발생

**채택: Option B + 일부 package-level convenience function**

- `internal/envalias` 패키지가 `package-level` `Default *Loader` 변수 제공 + 초기화 함수 `Init(logger *zap.Logger)` 노출
- production code (`cmd/minkd/main.go`) 에서 logger 준비 직후 `envalias.Init(logger)` 호출
- 각 read site 는 `envalias.DefaultGet("KEY")` 호출 (편의 함수)
- test 코드는 직접 `envalias.New(...)` + 주입 (DefaultGet 사용 안 함)

이로써 함수 시그니처 변경 최소화 + test 격리 보장.

### §4.3 검증

- 각 패키지 기존 test PASS (backward compat)
- 각 read site 마다 신규 sub-test 1개 (`TestX_AliasLoader_MinkOnly`, `TestX_AliasLoader_GooseOnly_WarnsOnce`)
- 전체 `go test -race ./...`

### §4.4 회귀 위험

| 회귀 시나리오 | Mitigation |
|--------------|-----------|
| `envalias.Init` 미호출 상태에서 `DefaultGet` 호출 → nil panic | `DefaultGet` 가 `Default == nil` 시 fallback `os.Getenv("MINK_X")` + `os.Getenv("GOOSE_X")` 직접 호출 (warning 없음, 안전 fallback) |
| race condition: `envalias.Init` 와 `DefaultGet` 동시 호출 | `sync.Once` 로 Init 보호 + `atomic.Pointer[Loader]` 로 Default 보호 |
| qwen/kimi client_test 가 const `envQwenRegion` 을 직접 참조 | const 유지 + 호출 site 만 alias loader 사용 (test 의 t.Setenv 도 alias 통과) |

## §5 Phase 4 — env scrub deny-list + test migration

### §5.1 isolation_unix.go / isolation_other.go 변경

```go
// before
if strings.HasPrefix(upper, "GOOSE_AUTH_") {
    return true
}

// after
if strings.HasPrefix(upper, "MINK_AUTH_") || strings.HasPrefix(upper, "GOOSE_AUTH_") {
    return true
}
```

### §5.2 hook_test.go 의 `TestScrubEnv_DenyList` 확장

```go
env := []string{
    "ANTHROPIC_API_KEY=xyz",
    "OPENAI_API_KEY=abc",
    "GOOSE_AUTH_TOKEN=zzz",
    "GOOSE_AUTH_REFRESH=refresh",
    "MINK_AUTH_TOKEN=new-zzz",     // 신규
    "MINK_AUTH_REFRESH=new-ref",   // 신규
    "MY_TOKEN=t",
    ...
}

denyListed := []string{
    "ANTHROPIC_API_KEY", "OPENAI_API_KEY",
    "GOOSE_AUTH_TOKEN", "GOOSE_AUTH_REFRESH",
    "MINK_AUTH_TOKEN", "MINK_AUTH_REFRESH",  // 신규
    "MY_TOKEN", ...
}
```

### §5.3 t.Setenv migration 자동화 안전 검사

```bash
# before-state grep (Phase 4 시작 전, base commit f0f02e4 기준)
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
grep -rn 't\.Setenv("GOOSE_' --include="*.go" . | grep -v "/vendor/" | grep -v "envalias/loader_test.go" | wc -l
# 예상: 28 (10 test files: cmd/minkd/integration_test.go (9), internal/command/adapter/aliasconfig/integration_test.go (5), aliasconfig/loader_amend_test.go (3), aliasconfig/loader_test.go (2), aliasconfig/loader_p3_test.go (1), aliasconfig/merge_test.go (1), llm/provider/qwen/client_test.go (2), kimi/client_test.go (2), hook/hook_test.go (2), config/config_test.go (1))

# os.Setenv 호출도 함께 migrate (research.md §2.4, R10 mitigation)
grep -rn 'os\.Setenv("GOOSE_' --include="*.go" . | grep -v "/vendor/" | wc -l
# 예상: 6 (3 test files: internal/audit/dual_test.go (3), internal/transport/grpc/server_test.go (2), internal/tools/builtin/terminal/bash_test.go (1))

# after-state grep (Phase 4 commit 직전)
grep -rn 't\.Setenv("GOOSE_' --include="*.go" . | grep -v "/vendor/" | grep -v "envalias/loader_test.go" | wc -l
# 목표: 0
grep -rn 'os\.Setenv("GOOSE_' --include="*.go" . | grep -v "/vendor/" | wc -l
# 목표: 0 (모두 t.Setenv("MINK_*") 로 migrate)
```

`sed -i` 대신 `Edit` tool 사용 (BSD/GNU sed 호환성 회피, MoAI 운영 규칙 § Tool Selection 일치).

각 test 의 t.Setenv 변경 후 그 test 가 실제로 alias 동작 (MINK_*) 을 검증하도록 보장 — 단순 string replace 시 test 가 alias loader 경로 우회 가능성 (envOverlay 가 아닌 `os.Getenv` 직접 호출하는 함수 test 면 의미 없음). Phase 3 의 production read site migration 이 선행되면 자연스럽게 흡수.

## §6 Phase 5 — 산문/주석/error message migration

### §6.1 변경 대상 (verbatim)

1. `internal/cli/commands/messaging_telegram.go:68` — `"--token is required (or set GOOSE_TELEGRAM_BOT_TOKEN env var)"` → `"--token is required (or set MINK_TELEGRAM_BOT_TOKEN; legacy GOOSE_TELEGRAM_BOT_TOKEN also accepted)"`
2. `internal/cli/commands/messaging_telegram.go:138` — flag help 동일 패턴
3. `internal/messaging/telegram/keyring_nokeyring.go:20, 25` — error message 동일 패턴
4. `internal/llm/provider/qwen/client.go:37, 38, 52, 62, 95` — 한국어 주석 `GOOSE_QWEN_REGION` → `MINK_QWEN_REGION (legacy: GOOSE_QWEN_REGION)`
5. `internal/llm/provider/kimi/client.go:39, 40, 54, 72, 131` — 동일
6. `internal/config/env.go` 의 한국어 주석 5개 — `GOOSE_LOG_LEVEL → log.level` → `MINK_LOG_LEVEL (legacy: GOOSE_LOG_LEVEL) → log.level`
7. `internal/config/env.go` 의 `@MX:SPEC: SPEC-GOOSE-CONFIG-001 §6.2` → `@MX:SPEC: SPEC-GOOSE-CONFIG-001 §6.2 + SPEC-MINK-ENV-MIGRATE-001 §7`
8. `internal/audit/dual.go:137-139` — 주석의 `GOOSE_HOME` → `MINK_HOME (legacy: GOOSE_HOME)`
9. `internal/transport/grpc/server.go:42, 47` — 동일 패턴
10. `internal/hook/handlers.go:135, 267` 와 `internal/hook/permission.go:248,250` — 동일 패턴
11. `internal/command/adapter/aliasconfig/loader.go:131` — 주석 `GOOSE_HOME` → `MINK_HOME (legacy alias)`
12. `internal/audit/dual_test.go:290` — `TestDefaultGlobalAuditPath_NoGOOSE_HOME` 함수명 → 그대로 유지 (test 함수명은 backward compat 의미 보존; 검증 의도 명확)

### §6.2 변경 제외 (의도된 GOOSE_* literal 유지)

- `internal/envalias/keys.go` — alias mapping 정의 (필수)
- `internal/envalias/loader.go` 의 deprecation warning format string — "GOOSE_*" 표기는 사용자에게 무엇이 deprecated 인지 알려주는 데 필요
- `internal/hook/isolation_unix.go:54` 주석 — "GOOSE_AUTH_* glob" 은 backward compat 의 의미 — 단, "MINK_AUTH_* and GOOSE_AUTH_* prefix glob" 으로 update
- 본 SPEC 의 `.moai/specs/SPEC-MINK-ENV-MIGRATE-001/*` documents — 의도된 인용
- 기존 SPEC documents (`.moai/specs/SPEC-GOOSE-*/`, `SPEC-MINK-BRAND-RENAME-001/`) — immutable per CLAUDE-style HARD rule

### §6.3 검증

```bash
# 산문 정리 후 grep 검증 (alias mapping 외 GOOSE_* literal 잔존 없는지)
grep -rn "GOOSE_" --include="*.go" . | grep -v "/vendor/" | grep -v "envalias/" | grep -v "_test.go" | grep -v "isolation_.*\.go.*GOOSE_AUTH_"
# 목표: alias 호출 경로 외 0건 (alias loader format string 의 "GOOSE_X" 표기 제외)
```

## §7 Phase 6 — Integration test + final verification

### §7.1 신규 integration test

`cmd/minkd/integration_test.go` 에 3개 신규 test 함수:

1. `TestMain_EnvAlias_GooseHomeOnly` — env `GOOSE_HOME=<tmp>` 만 설정 → minkd 기동 → AC-MINK-EM-002 검증
2. `TestMain_EnvAlias_MinkHomeOnly` — env `MINK_HOME=<tmp>` 만 설정 → AC-MINK-EM-003 검증
3. `TestMain_EnvAlias_BothSet_PrefersMink` — 동시 설정 → AC-MINK-EM-004 검증 + warning emit log line grep

stderr 의 zap JSON line capture 패턴은 기존 `cmd/minkd/integration_test.go` 의 wire test helper 참조 (이미 stderr capture 패턴 정립).

### §7.2 최종 검증 명령 (CI-equivalent)

```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001

# 1) compile
go build ./...

# 2) format
gofmt -l . | (! grep .)

# 3) vet
go vet ./...

# 4) test (race detector 포함)
go test -race ./...

# 5) lint (optional, golangci-lint 가 설치된 경우)
which golangci-lint && golangci-lint run --timeout 5m

# 6) AC verification
ls internal/envalias/                                              # AC-MINK-EM-001
grep -rn 't\.Setenv("GOOSE_' --include="*.go" . \
   | grep -v "/vendor/" | grep -v "envalias/loader_test.go" | wc -l   # AC-MINK-EM-007, 목표 0
grep -rn "GOOSE_" --include="*.go" . | grep -v "/vendor/" \
   | grep -v "envalias/" | grep -v "_test.go" \
   | grep -v "isolation_.*GOOSE_AUTH_" | wc -l                       # AC-MINK-EM-010
```

### §7.3 PR open 체크리스트

- [ ] commit history clean (6 commits, atomic, 각 commit 의 message 가 phase 와 1:1)
- [ ] CHANGELOG.md 의 unreleased section 에 entry 추가:
   ```markdown
   ### Changed
   - `SPEC-MINK-ENV-MIGRATE-001` v0.2.0: Introduced `internal/envalias` alias loader.
     All `GOOSE_*` env vars now have `MINK_*` equivalents. Per-process deprecation
     warnings (sync.Once) guide migration. Backward-compatible.
   ```
- [ ] PR description 에 22 key migration 표 + AC verification result 포함
- [ ] 본 SPEC frontmatter 의 `status: draft` → `status: approved` (orchestrator 가 plan-auditor PASS 후 update)
- [ ] `issue_number` frontmatter 필드 update (GH Issue 생성 후 값 채움)

## §8 Cumulative Open Questions (plan 단계)

| OQ # | Question | 결정 시점 |
|------|----------|----------|
| OQ-PL-1 | Loader 의 typed API 신설 (e.g., `GetInt`, `GetBool`) — 호출부 strconv.Atoi 중복 줄임 — 도입할지? | spec.md §6.2 OUT scope 명시 결정: 본 SPEC 외 (over-engineering 회피). 호출부 conversion 유지. |
| OQ-PL-2 | qwen/kimi 의 const `envQwenRegion = "GOOSE_QWEN_REGION"` 을 const `envQwenKey = "QWEN_REGION"` (alias short key) 로 변경할지? | **RESOLVED (v0.1.1, plan-auditor cycle D11)**: const **이름 유지** (`envQwenRegion`, `envKimiRegion`, `homeEnv`) + **값만 short key 변경** (`"QWEN_REGION"`, `"KIMI_REGION"`, `"HOME"`). 결정 trail: (a) const 이름 변경 시 외부 import 영향 — `grep -rn "envQwenRegion\|envKimiRegion\|homeEnv"` → import 0건 (package-private 이므로 안전). (b) 그러나 이름 유지로 git blame 연속성 보존 + minimal diff (rename refactor 분리). (c) const 의 의미는 "alias loader Get 의 newKey argument" — 값이 short key 인 게 일관성 유지. **결정**: 첫번째 옵션 채택 = const 이름 유지 + 값만 변경 (`envQwenRegion = "QWEN_REGION"` 형태). Phase 3 commit 시 적용. |
| OQ-PL-3 | `internal/envalias` 가 zap 외 logger interface 추상화 (`Logger interface { Warn(...) }`) 도입할지? | NO — 프로젝트 전체가 zap 단일 사용, abstraction 도입은 over-engineering. |
| OQ-PL-4 | `cmd/minkd/main.go` 의 `envalias.Init(logger)` 호출 위치 (logger 생성 직후 vs config load 직후)? | logger 생성 직후 — envOverlay 가 logger 를 받기 전에 alias loader 가 준비되어야 함 (chicken-egg 회피). |

## §9 Risks (plan-level 누적)

spec.md §9 risk table 13개 (R1~R13, plan-auditor cycle D7 에서 R11/R12/R13 추가) 외 plan-level 추가:

- R9: Phase 4 의 28 t.Setenv + 6 os.Setenv 변경 중 1~2 곳 누락 → CI 통과하지만 backward compat 미검증. Mitigation: AC-MINK-EM-007 의 grep 자동화 + plan-auditor 가 Phase 4 commit 의 diff 를 grep 결과와 cross-check.
- R10: `cmd/minkd/integration_test.go` 의 신규 test 가 `os.Setenv` (not `t.Setenv`) 를 사용하면 병렬 test 간 env 오염. Mitigation: 강제 `t.Setenv` 사용 + `t.Parallel()` 호출 분리. (research.md §2.4 의 6 `os.Setenv` callsite 도 본 SPEC 에서 `t.Setenv` 로 migrate.)

## §10 References

- spec.md (sibling) — EARS REQ + AC
- research.md (sibling) — 22 key 인벤토리 + 모듈 분포
- acceptance.md (sibling) — Given/When/Then 전수 시나리오

End of plan.md.

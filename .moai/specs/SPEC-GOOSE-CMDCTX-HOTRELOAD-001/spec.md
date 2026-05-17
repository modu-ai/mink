---
id: SPEC-GOOSE-CMDCTX-HOTRELOAD-001
version: 0.1.0
status: planned
created_at: 2026-04-27
updated_at: 2026-04-27
author: manager-spec
priority: P4
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-anchored
labels: [area/runtime, area/config, type/feature, priority/p4-low]
---

# SPEC-GOOSE-CMDCTX-HOTRELOAD-001 — ContextAdapter Registry / AliasMap Hot-Reload (CMDCTX v0.2 Amendment)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. SPEC-GOOSE-CMDCTX-001 v0.1.1 (implemented, FROZEN) §Exclusions #8 의 후속 amendment. ContextAdapter `registry`, `aliasMap` 필드를 `*atomic.Pointer[T]` 로 전환하여 atomic swap 기반 hot-reload 도입. fsnotify watcher (1차) + `/reload aliases` slash command (2차) 트리거. ALIAS-CONFIG-001 의 데이터 소스에 의존. 본 SPEC implementation 시점에 CMDCTX-001 본문도 v0.1.1 → v0.2.0 (또는 그 이후, PERMISSIVE-ALIAS-001 / TELEMETRY 머지 순서에 의존) amendment 가 동시 발생함. | manager-spec |

---

## 1. 개요 (Overview)

본 SPEC 은 `SPEC-GOOSE-CMDCTX-001` v0.1.1 (implemented, FROZEN) 의 ContextAdapter 가 가지는 다음 결손을 해소하는 amendment 이다:

> ContextAdapter 의 `registry *router.ProviderRegistry` 와 `aliasMap map[string]string` 필드는 `New(opts)` 시점에 immutable 하다. 사용자가 alias 파일을 수정하거나 새 provider 가 등록되어도 daemon 재시작 없이는 반영되지 않는다.

본 SPEC 은:

- ContextAdapter 의 두 필드를 `*atomic.Pointer[router.ProviderRegistry]` / `*atomic.Pointer[map[string]string]` 로 전환한다.
- `ReloadAliases(newMap)` / `ReloadRegistry(newReg)` API 를 신설한다.
- WithContext 로 파생된 child adapter 도 부모와 동일 atomic.Pointer 를 공유하여 reload 결과를 즉시 관찰한다 (planMode 와 동일 패턴).
- fsnotify watcher 가 alias 파일 변경을 감지하면 `aliasconfig.LoadDefault` → `aliasconfig.Validate` → `ReloadAliases` 체인을 호출한다.
- reload 실패 시 (yaml malformed / Validate strict 실패 등) 기존 map 을 유지하고 error 를 반환한다 (fail-safe).
- in-flight `ResolveModelAlias` 호출은 reload 와 race 없이 일관된 snapshot 으로 동작한다 (atomic.Pointer 보장).

본 SPEC 의 수락은 CMDCTX-001 SPEC 본문의 v0.1.1 → v0.2.0 (또는 그 이후) amendment 와 함께 발생한다 (run phase 시점, PERMISSIVE-ALIAS-001 / TELEMETRY SPEC 와의 머지 순서 governance).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **ALIAS-CONFIG-001 직후 가치 회수**: ALIAS-CONFIG-001 (planned) 가 alias 파일 로더를 정의하지만, daemon 재시작 없이는 변경이 반영되지 않는다. ALIAS-CONFIG-001 §10 Exclusions #1 / §2.3 이 hot-reload 책임을 본 SPEC 에 명시적으로 위임하였다.
- **사용자 UX**: 파일 편집기로 alias 추가/삭제 후 즉시 `/model new-alias` 가 동작해야 슬래시 명령 시스템의 가치가 완성된다.
- **운영 환경**: scheduled restart 없이 alias 갱신 가능 → 멀티세션 동시 실행 / 무중단 운영 지원.
- **CMDCTX-001 §Exclusions #8 의 명시적 후속 약속**: "Hot-reload of registry / aliasMap — `New(...)` 시점 immutable. 후속 SPEC (TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요)."

### 2.2 상속 자산

- **SPEC-GOOSE-CMDCTX-001** v0.1.1 (implemented): `ContextAdapter`, `Options`, `ResolveModelAlias`, alias.go, planMode `*atomic.Bool` 패턴. **본 SPEC 의 amendment 대상**. implementation 시점 frontmatter version 0.1.1 → 0.2.0 (또는 그 이후) 로 동시 갱신.
- **SPEC-GOOSE-ALIAS-CONFIG-001** (planned, Batch A): alias config 파일 로더 + `Validate`. 본 SPEC 의 reload 데이터 소스. surface 변경 없음, read-only consume.
- **SPEC-GOOSE-ROUTER-001** (implemented, FROZEN): `*router.ProviderRegistry` API. read-only 사용. **변경 없음**.
- **SPEC-GOOSE-COMMAND-001** (implemented, FROZEN): `command.SlashCommandContext` 인터페이스. **변경 없음** — 본 SPEC 은 ContextAdapter 내부만 변경.

### 2.3 범위 경계 (한 줄)

- **IN**: ContextAdapter `registry`/`aliasMap` 의 `*atomic.Pointer[T]` 전환, `ReloadAliases`/`ReloadRegistry` API, fsnotify watcher 패키지, debounce 100ms, `/reload aliases` Optional slash command, AC 신규 + CMDCTX-001 v0.2.0 amendment governance.
- **OUT**: alias config 파일 자체 (ALIAS-CONFIG-001 책임), Validate 알고리즘, registry hot-reload via SIGHUP/HTTP API, provider plugin hot-load, multi-session adapter, active model auto-fallback (Optional REQ).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC 이 정의/구현하는 것)

1. **ContextAdapter struct amendment** (CMDCTX-001 §6.2):
   - `registry *router.ProviderRegistry` → `registry *atomic.Pointer[router.ProviderRegistry]`
   - `aliasMap map[string]string` → `aliasMap *atomic.Pointer[map[string]string]`
   - `New(opts)` 가 두 atomic.Pointer 를 항상 non-nil 로 초기화.

2. **신규 API**:
   - `ReloadAliases(newMap map[string]string) error` — newMap 을 deep copy 후 atomic swap.
   - `ReloadRegistry(newReg *router.ProviderRegistry) error` — newReg 포인터 atomic swap.
   - `ErrNilAliasMap`, `ErrNilRegistry` sentinel errors.

3. **ResolveModelAlias 알고리즘 read-side adaptation** (CMDCTX-001 §6.4):
   - step 1, 2 의 `a.registry` / `a.aliasMap` 직접 참조를 `a.registry.Load()` / `a.aliasMap.Load()` snapshot 확보로 변경.
   - 함수 내내 일관된 snapshot 사용 (eventual consistency).

4. **WithContext shallow copy 일관성** (CMDCTX-001 §6.5):
   - shallow copy 가 `*atomic.Pointer[T]` 포인터를 공유하여 부모/자식 단일 진실 공급원 보장.
   - 추가 코드 변경 없음 (planMode 와 동일 패턴).

5. **신규 패키지** `internal/command/adapter/hotreload/`:
   - `Watcher` struct — fsnotify 기반 디렉토리 단위 watcher.
   - `Options` — alias 파일 경로, debounce window, logger, ContextAdapter 참조.
   - `Run(ctx)` / `Stop()` 라이프사이클.
   - debounce 100ms timer logic.
   - reload 실패 시 기존 map 유지 + warn-log.

6. **`/reload aliases` slash command** (Optional REQ-HOTRELOAD-013):
   - `internal/command/builtin/reload.go` (가칭) 신규 builtin 등록.
   - 사용자가 명시적으로 reload 트리거 가능.
   - watcher 비활성/장애 환경의 fallback 채널.

7. **fsnotify 외부 의존성 신규 require**:
   - `github.com/fsnotify/fsnotify` 를 `go.mod` 에 추가.
   - 신규 외부 의존성 1건 — research §8 D-9 의 명시적 결정.

8. **CMDCTX-001 amendment governance**:
   - implementation 시점 CMDCTX-001 spec.md 의 frontmatter `version: 0.1.1 → 0.X.0` (X 는 PERMISSIVE-ALIAS-001 / TELEMETRY 와의 머지 순서에 의존).
   - HISTORY 항목 1줄 추가 (본 SPEC ID 인용).
   - §1 Overview, §6.2 struct, §6.4 알고리즘, §6.6 race 안전성, §Exclusions 갱신.

### 3.2 OUT OF SCOPE (명시적 제외)

§10 Exclusions 절 참조. 핵심 OUT:

- alias config 파일 schema / loader / Validate — ALIAS-CONFIG-001 책임 (FROZEN).
- registry hot-reload via SIGHUP / HTTP API — 후속 SPEC.
- provider plugin hot-load (런타임 .so / Go plugin 시스템).
- multi-session adapter (CMDCTX-001 §Exclusions #9 와 동일 가정).
- active model auto-fallback (reload 후 active model 이 새 registry 에 부재 시) — Optional REQ-HOTRELOAD-040, 후속 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

#### REQ-HOTRELOAD-001 — atomic.Pointer 전환

The `ContextAdapter` **shall** store `registry` and `aliasMap` fields as `*atomic.Pointer[router.ProviderRegistry]` and `*atomic.Pointer[map[string]string]` respectively, both initialized non-nil by `New(...)`.

#### REQ-HOTRELOAD-002 — ReloadAliases API

The `ContextAdapter` **shall** expose `ReloadAliases(newMap map[string]string) error` that atomically swaps the alias map. The method **shall** deep-copy `newMap` before the swap to prevent post-swap caller mutation from racing concurrent readers.

#### REQ-HOTRELOAD-003 — ReloadRegistry API

The `ContextAdapter` **shall** expose `ReloadRegistry(newReg *router.ProviderRegistry) error` that atomically swaps the registry pointer.

#### REQ-HOTRELOAD-004 — WithContext children 일관성

After `parent.ReloadAliases(m)` 또는 `parent.ReloadRegistry(r)` 호출, every WithContext child of `parent` **shall** observe the new value on its next `ResolveModelAlias` call (atomic ordering boundary).

#### REQ-HOTRELOAD-005 — race-free read

Every call to `ResolveModelAlias` **shall** load `registry` and `aliasMap` via `atomic.Pointer.Load()` once at function entry and use the resulting snapshot for the entire function execution.

#### REQ-HOTRELOAD-006 — race detector clean

All Reload* and ResolveModelAlias paths **shall** be safe for concurrent invocation as verified by `go test -race -count=10`.

### 4.2 Event-Driven (이벤트 기반)

#### REQ-HOTRELOAD-010 — fsnotify watcher trigger

**When** the alias file resolved by ALIAS-CONFIG-001 (`$MINK_ALIAS_FILE` or `$MINK_HOME/aliases.yaml` or `$HOME/.goose/aliases.yaml`) emits a `Write` / `Create` / `Rename` fsnotify event, the watcher **shall** schedule a debounced reload.

#### REQ-HOTRELOAD-011 — reload chain on event

**When** the debounce window expires without further events, the watcher **shall** invoke `aliasconfig.LoadDefault()` → `aliasconfig.Validate(m, registry, strict)` → `ContextAdapter.ReloadAliases(m)` in sequence.

#### REQ-HOTRELOAD-012 — reload success log

**When** `ReloadAliases` returns `nil`, the watcher **shall** emit an info-level log entry containing the new map size.

#### REQ-HOTRELOAD-013 — `/reload aliases` slash command (Optional)

**When** a user dispatches `/reload aliases`, the dispatcher **shall** invoke the same reload chain as REQ-HOTRELOAD-011 and report success/failure to the user via the dispatcher's standard output channel.

### 4.3 State-Driven (상태 기반)

#### REQ-HOTRELOAD-020 — debounce window 100ms

**While** fsnotify events arrive within 100 milliseconds of each other, the watcher **shall** reset the debounce timer and **shall not** trigger reload until 100ms of quiescence.

#### REQ-HOTRELOAD-021 — directory-level watch

**While** the watcher is active, it **shall** subscribe to the parent directory of the alias file (not the file itself) and filter events by basename to handle atomic-rename save patterns.

#### REQ-HOTRELOAD-022 — concurrent ResolveModelAlias safety

**While** `ReloadAliases` is in progress (between deep-copy start and `atomic.Store`), concurrent `ResolveModelAlias` calls **shall** observe either the previous map or the new map, never a partial state.

### 4.4 Unwanted Behaviour (방지)

#### REQ-HOTRELOAD-030 — nil newMap reject

**If** `ReloadAliases(nil)` is called, **then** the method **shall** return `ErrNilAliasMap` and **shall not** mutate the existing `aliasMap` atomic pointer.

#### REQ-HOTRELOAD-031 — nil newRegistry reject

**If** `ReloadRegistry(nil)` is called, **then** the method **shall** return `ErrNilRegistry` and **shall not** mutate the existing `registry` atomic pointer.

#### REQ-HOTRELOAD-032 — Validate failure → existing map preserved

**If** the watcher's `aliasconfig.Validate(m, registry, strict)` returns errors in strict mode, **then** `ReloadAliases` **shall not** be called and the existing alias map **shall** remain unchanged. The watcher **shall** emit a warn-level log entry containing the validation errors.

#### REQ-HOTRELOAD-033 — Loader failure → existing map preserved

**If** `aliasconfig.LoadDefault()` returns a non-sentinel error (malformed YAML, oversize file, permission error), **then** `ReloadAliases` **shall not** be called and the existing alias map **shall** remain unchanged. The watcher **shall** emit a warn-level log entry containing the load error.

#### REQ-HOTRELOAD-034 — no panic on watcher errors

**If** fsnotify emits an error event (e.g., watch removed by external `rm -rf`), **then** the watcher **shall** log the error and attempt a single re-watch (`Add` retry) without panicking. If the retry fails, the watcher **shall** log a fatal-impact warn entry and terminate gracefully.

#### REQ-HOTRELOAD-035 — atomic.Pointer noCopy compliance

**If** `go vet copylocks` is run on `internal/command/adapter/`, **then** the analysis **shall** report zero violations. (atomic.Pointer 의 noCopy 가드를 위반하는 ContextAdapter shallow copy 가 발생하지 않음을 정적 검증.)

### 4.5 Optional (선택적)

#### REQ-HOTRELOAD-040 — active model auto-fallback

**Where** the active model after a reload no longer exists in the new registry, the system **may** automatically fall back to a default model and emit a warn-level log entry. v0.1.0 implementation **may** stub this and leave the active model unchanged (caller responsibility on next `/model` invocation).

#### REQ-HOTRELOAD-041 — fsnotify build tag

**Where** the binary is built with `// +build !fsnotify` build tag, the watcher subsystem **shall** be excluded from the build, leaving only the `/reload aliases` slash command path active. v0.1.0 **may** stub this — fsnotify default-on is acceptable for v0.1.0.

#### REQ-HOTRELOAD-042 — Logger 주입

**Where** the watcher's `Options.Logger` is non-nil, the watcher **shall** emit zap log entries for: file resolution, debounce reset events, reload success/failure, and watcher lifecycle (Start/Stop).

---

## 5. 수용 기준 (Acceptance Criteria)

| AC ID | 검증 대상 REQ | Given-When-Then |
|-------|---------------|-----------------|
| **AC-HOTRELOAD-001** | REQ-HOTRELOAD-001 | **Given** ContextAdapter struct 정의 코드 **When** `go vet ./internal/command/adapter/...` 실행 **Then** `registry`, `aliasMap` 필드 타입이 `*atomic.Pointer[T]` 임을 reflect / godoc grep 으로 확인, `New(...)` 직후 `Load()` 호출이 non-nil 반환 |
| **AC-HOTRELOAD-002** | REQ-HOTRELOAD-002 | **Given** ContextAdapter 인스턴스, alias map `{"a": "p/m"}` **When** `ReloadAliases(map[string]string{"b": "p/m2"})` 호출 → 호출 후 caller 가 원본 map 을 mutate (`m["b"] = "p/m3"`) **Then** ResolveModelAlias("b") 결과가 "p/m2" 로 stable (deep-copy 검증) |
| **AC-HOTRELOAD-003** | REQ-HOTRELOAD-003 | **Given** ContextAdapter 인스턴스, registry r1 **When** ReloadRegistry(r2) 호출 후 ResolveModelAlias 호출 **Then** r2 기준 lookup 결과 반환 |
| **AC-HOTRELOAD-004** | REQ-HOTRELOAD-004 | **Given** parent ContextAdapter, child = parent.WithContext(ctx) **When** parent.ReloadAliases({"x": "p/m"}) 호출 후 child.ResolveModelAlias("x") 호출 **Then** "p/m" 결과 반환 (parent/child 동일 atomic.Pointer 공유 검증) |
| **AC-HOTRELOAD-005** | REQ-HOTRELOAD-005 | **Given** ResolveModelAlias 함수 코드 **When** 정적 분석: 함수 본문 내 `a.registry.` / `a.aliasMap.` 접근 횟수 카운트 **Then** 각 1회 (Load 1회만 호출, 이후 local 변수 사용) |
| **AC-HOTRELOAD-006** | REQ-HOTRELOAD-006, REQ-HOTRELOAD-022 | **Given** ContextAdapter 인스턴스, 100 goroutine 동시에 ResolveModelAlias + 50 goroutine 동시에 ReloadAliases (각 1000 iteration) **When** `go test -race -count=10` 실행 **Then** race condition 0건, panic 0건 |
| **AC-HOTRELOAD-010** | REQ-HOTRELOAD-010 | **Given** Watcher 가 `/tmp/aliases.yaml` 디렉토리 watch 중 **When** 파일을 atomic write (`os.Rename(tmpfile, "/tmp/aliases.yaml")`) **Then** 100ms+50ms 후 reload chain 호출 1회 발생 (fakeReloader spy) |
| **AC-HOTRELOAD-011** | REQ-HOTRELOAD-011 | **Given** Watcher 가 fakeLoader / fakeValidator / fakeReloader 와 함께 인스턴스화 **When** debounce expire **Then** Loader.LoadDefault → Validator.Validate → Reloader.ReloadAliases 순서로 정확히 1회씩 호출 (call sequence spy) |
| **AC-HOTRELOAD-012** | REQ-HOTRELOAD-012 | **Given** 위와 동일, fakeReloader 가 nil 반환 **When** reload 완료 **Then** zaptest observer 에 info-level 로그 1건 (메시지 "alias hot-reload succeeded" + size 필드 포함) |
| **AC-HOTRELOAD-013** | REQ-HOTRELOAD-013 | **Given** dispatcher 에 `/reload aliases` builtin 등록, fakeReloader spy **When** dispatcher.ProcessUserInput("/reload aliases", sctx) 호출 **Then** Reloader.ReloadAliases 호출 1회, dispatcher 출력에 success message 포함 |
| **AC-HOTRELOAD-020** | REQ-HOTRELOAD-020 | **Given** Watcher debounce window 100ms, fakeClock 주입 **When** 0ms / 50ms / 99ms 시점 fsnotify 이벤트 3건 emit (3번째 이벤트 후 110ms 대기) **Then** reload chain 호출 정확히 1회 발생 (debounce coalescing 검증) |
| **AC-HOTRELOAD-021** | REQ-HOTRELOAD-021 | **Given** Watcher 가 `/tmp/aliases.yaml` 을 watch target 으로 입력받음 **When** Watcher.Run(ctx) 호출 후 fsnotify call log 검증 **Then** fsnotify.Add 호출 인자가 `/tmp/` (디렉토리), 본 SPEC 의 basename 필터가 활성 |
| **AC-HOTRELOAD-022** | REQ-HOTRELOAD-022 | AC-HOTRELOAD-006 race 검증과 합쳐 검증 |
| **AC-HOTRELOAD-030** | REQ-HOTRELOAD-030 | **Given** ContextAdapter, 기존 aliasMap `{"x": "p/m"}` **When** ReloadAliases(nil) 호출 **Then** ErrNilAliasMap 반환, panic 없음, ResolveModelAlias("x") 결과 여전히 "p/m" |
| **AC-HOTRELOAD-031** | REQ-HOTRELOAD-031 | **Given** ContextAdapter, 기존 registry r1 **When** ReloadRegistry(nil) 호출 **Then** ErrNilRegistry 반환, panic 없음, ResolveModelAlias 가 r1 기준 동작 |
| **AC-HOTRELOAD-032** | REQ-HOTRELOAD-032 | **Given** Watcher with strict=true, fakeValidator 가 1건 error 반환 **When** debounce expire 후 reload chain 시작 **Then** Reloader.ReloadAliases 호출 0회, zaptest observer 에 warn-level 로그 1건 (error 내용 포함) |
| **AC-HOTRELOAD-033** | REQ-HOTRELOAD-033 | **Given** Watcher with fakeLoader 가 ErrMalformedAliasFile 반환 **When** 위와 동일 **Then** Validator.Validate 호출 0회, Reloader.ReloadAliases 호출 0회, warn-level 로그 1건 (load error 포함) |
| **AC-HOTRELOAD-034** | REQ-HOTRELOAD-034 | **Given** Watcher 활성 상태, 외부에서 watch 디렉토리 `rm -rf` 시뮬레이션 (fakeFsnotify Error 이벤트 emit) **When** Watcher 가 이벤트 수신 **Then** Watcher.Add 재시도 1회, 재시도 실패 시 warn-level 로그 + Watcher 정상 종료, panic 0건 |
| **AC-HOTRELOAD-035** | REQ-HOTRELOAD-035 | **Given** `internal/command/adapter/` 패키지 코드 **When** `go vet -copylocks ./internal/command/adapter/...` 실행 **Then** 위반 0건 (atomic.Pointer 사용이 noCopy 를 위반하지 않음) |
| **AC-HOTRELOAD-042** | REQ-HOTRELOAD-042 | **Given** Watcher.Options.Logger = zaptest observer **When** Watcher.Run / Stop / 첫 reload 사이클 실행 **Then** 다음 로그 entry 가 각각 발생: "watcher started", "fsnotify event received", "debounce reset", "reload succeeded", "watcher stopped" |
| **AC-HOTRELOAD-050** | CMDCTX-001 v0.X.0 amendment governance | **When** 본 SPEC implementation 머지 **Then** CMDCTX-001 spec.md 의 frontmatter `version` 이 0.1.1 → 0.X.0 (X >= 2) 으로 갱신, HISTORY 에 본 SPEC ID 인용한 1줄 추가, §6.2 / §6.4 / §6.6 본문 갱신, §Exclusions #8 항목 삭제 |
| **AC-HOTRELOAD-051** | 신규 외부 의존성 1건 (fsnotify) | **When** 본 SPEC implementation 후 `go.mod` diff **Then** `github.com/fsnotify/fsnotify` 1건 신규 require 추가, 그 외 신규 외부 의존성 0건 |
| **AC-HOTRELOAD-052** | CMDCTX-001 회귀 금지 | **When** 본 SPEC implementation 후 CMDCTX-001 의 19개 AC (AC-CMDCTX-001 ~ 019) 재실행 **Then** 모두 PASS (기존 행동 보존) |

**커버리지 매트릭스**:

| REQ | AC들 |
|-----|------|
| REQ-HOTRELOAD-001 | AC-HOTRELOAD-001 |
| REQ-HOTRELOAD-002 | AC-HOTRELOAD-002, AC-HOTRELOAD-030 |
| REQ-HOTRELOAD-003 | AC-HOTRELOAD-003, AC-HOTRELOAD-031 |
| REQ-HOTRELOAD-004 | AC-HOTRELOAD-004 |
| REQ-HOTRELOAD-005 | AC-HOTRELOAD-005 |
| REQ-HOTRELOAD-006 | AC-HOTRELOAD-006 |
| REQ-HOTRELOAD-010 | AC-HOTRELOAD-010 |
| REQ-HOTRELOAD-011 | AC-HOTRELOAD-011 |
| REQ-HOTRELOAD-012 | AC-HOTRELOAD-012 |
| REQ-HOTRELOAD-013 | AC-HOTRELOAD-013 |
| REQ-HOTRELOAD-020 | AC-HOTRELOAD-020 |
| REQ-HOTRELOAD-021 | AC-HOTRELOAD-021 |
| REQ-HOTRELOAD-022 | AC-HOTRELOAD-022 (= AC-HOTRELOAD-006) |
| REQ-HOTRELOAD-030 | AC-HOTRELOAD-030 |
| REQ-HOTRELOAD-031 | AC-HOTRELOAD-031 |
| REQ-HOTRELOAD-032 | AC-HOTRELOAD-032 |
| REQ-HOTRELOAD-033 | AC-HOTRELOAD-033 |
| REQ-HOTRELOAD-034 | AC-HOTRELOAD-034 |
| REQ-HOTRELOAD-035 | AC-HOTRELOAD-035 |
| REQ-HOTRELOAD-040 | (Optional, v0.1.0 stub 허용 — AC 없음) |
| REQ-HOTRELOAD-041 | (Optional, v0.1.0 stub 허용 — AC 없음) |
| REQ-HOTRELOAD-042 | AC-HOTRELOAD-042 |

**총 22 REQ (Ubiquitous 6, Event-Driven 4, State-Driven 3, Unwanted 6, Optional 3 — Optional 중 1개 (REQ-HOTRELOAD-042) 는 AC 보유) / 23 AC**. 모든 non-Optional REQ 가 최소 1개의 AC 로 검증된다. CMDCTX-001 amendment governance 와 fsnotify 의존성, 회귀 금지를 검증하는 별도 AC (AC-HOTRELOAD-050/051/052) 포함.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 layout (변경 후)

```
internal/command/
├── adapter/
│   ├── adapter.go              # ⬆ amendment: registry/aliasMap → atomic.Pointer
│   ├── adapter_test.go         # ⬆ amendment: race + reload 테스트 추가
│   ├── alias.go                # ⬆ amendment: resolveAlias step 1-2 read adaptation
│   ├── controller.go           # unchanged
│   ├── errors.go               # ⬆ amendment: ErrNilAliasMap, ErrNilRegistry 추가
│   ├── hotreload/              # ⬅ 본 SPEC 신규
│   │   ├── watcher.go          # Watcher struct + fsnotify integration
│   │   ├── watcher_test.go     # fakeFsnotify / fakeClock / fakeReloader 기반
│   │   ├── debounce.go         # debounce timer
│   │   └── debounce_test.go
│   └── ...
└── builtin/
    └── reload.go               # ⬅ 본 SPEC 신규 (Optional REQ-HOTRELOAD-013)
```

### 6.2 데이터 모델 — ContextAdapter struct (amendment AFTER)

```go
// AFTER amendment (CMDCTX-001 v0.X.0)
//
// @MX:ANCHOR: Hot-reloadable state surface for ContextAdapter.
// @MX:REASON: registry / aliasMap atomic.Pointer enable race-free hot-swap.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-HOTRELOAD-001 REQ-HOTRELOAD-001
type ContextAdapter struct {
    // registry is *atomic.Pointer (pointer indirection) so that WithContext
    // children share the same underlying atomic without copying it (sync/atomic
    // pointer types carry a noCopy guard that go vet copylocks would flag).
    registry   *atomic.Pointer[router.ProviderRegistry]   // CHANGED v0.X.0

    // aliasMap is *atomic.Pointer for the same reason. The pointed-to value is
    // a *map[string]string — atomic swap of the entire map is a single pointer
    // store. ReloadAliases deep-copies caller input before storing.
    aliasMap   *atomic.Pointer[map[string]string]         // CHANGED v0.X.0

    loopCtrl   LoopController                             // unchanged
    planMode   *atomic.Bool                               // unchanged
    getwdFn    func() (string, error)                     // unchanged
    logger     Logger                                     // unchanged
    ctxHook    context.Context                            // unchanged
}
```

### 6.3 신규 API surface

```go
// errors.go (amendment AFTER)
var (
    ErrLoopControllerUnavailable = errors.New("adapter: LoopController is nil") // existing
    ErrNilAliasMap               = errors.New("adapter: nil alias map")          // NEW
    ErrNilRegistry               = errors.New("adapter: nil registry")           // NEW
)

// adapter.go (amendment AFTER)

// ReloadAliases atomically swaps the alias map. Caller must invoke
// aliasconfig.Validate before this call; this method does NOT re-validate.
//
// The newMap is deep-copied to ensure caller cannot race concurrent readers
// by mutating the source after the swap.
//
// Returns ErrNilAliasMap if newMap is nil. The empty map (len == 0) is valid
// and clears all aliases.
func (a *ContextAdapter) ReloadAliases(newMap map[string]string) error {
    if newMap == nil {
        return ErrNilAliasMap
    }
    cloned := make(map[string]string, len(newMap))
    for k, v := range newMap {
        cloned[k] = v
    }
    a.aliasMap.Store(&cloned)
    return nil
}

// ReloadRegistry atomically swaps the registry pointer. The previous registry
// is not torn down; caller is responsible for any cleanup (typically none —
// registry is a read-only data structure).
//
// Returns ErrNilRegistry if newReg is nil. To clear the registry, the design
// intentionally does NOT support nil — empty registry must be a constructed
// non-nil registry with zero providers.
func (a *ContextAdapter) ReloadRegistry(newReg *router.ProviderRegistry) error {
    if newReg == nil {
        return ErrNilRegistry
    }
    a.registry.Store(newReg)
    return nil
}
```

### 6.4 ResolveModelAlias read-side amendment (CMDCTX-001 §6.4 step 1-2)

```
ResolveModelAlias(alias):
  1. reg := a.registry.Load()                  // CHANGED v0.X.0: atomic snapshot
     if reg == nil:
       return nil, ErrUnknownModel
  2. aliasMapPtr := a.aliasMap.Load()           // CHANGED v0.X.0
     var aliasMap map[string]string
     if aliasMapPtr != nil:
       aliasMap = *aliasMapPtr
     // else aliasMap remains nil — len(nil) == 0, lookup behaves as empty
  3-8. (unchanged from v0.1.1)
```

함수 본문 내 `a.registry` / `a.aliasMap` 직접 참조는 step 1-2 의 `Load()` 단 1회만. 이후 local 변수 (`reg`, `aliasMap`) 사용 (REQ-HOTRELOAD-005, AC-HOTRELOAD-005).

### 6.5 New(...) 초기화

```go
// New (amendment AFTER, CMDCTX-001 §6.2)
func New(opts Options) *ContextAdapter {
    regPtr := new(atomic.Pointer[router.ProviderRegistry])
    if opts.Registry != nil {
        regPtr.Store(opts.Registry)
    }

    aliasPtr := new(atomic.Pointer[map[string]string])
    aliasMap := opts.AliasMap
    if aliasMap == nil {
        aliasMap = map[string]string{}
    }
    aliasPtr.Store(&aliasMap)

    return &ContextAdapter{
        registry: regPtr,
        aliasMap: aliasPtr,
        loopCtrl: opts.LoopController,
        planMode: new(atomic.Bool),
        getwdFn:  resolveGetwdFn(opts.GetwdFn),
        logger:   opts.Logger,
    }
}
```

### 6.6 Race-clean 시나리오 (AC-HOTRELOAD-006)

```go
// adapter_test.go (의도 시그니처)
func TestContextAdapter_ConcurrentReloadAndResolve(t *testing.T) {
    adapter := New(Options{Registry: defaultReg, AliasMap: nil})

    var wg sync.WaitGroup
    // 100 readers
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                _, _ = adapter.ResolveModelAlias("opus")
            }
        }()
    }
    // 50 reloaders
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                _ = adapter.ReloadAliases(map[string]string{
                    "opus": "anthropic/claude-opus-4-7",
                })
            }
        }()
    }
    wg.Wait()
}
// CI: go test -race -count=10 → race 0건 보장
```

### 6.7 Watcher 패키지 (`internal/command/adapter/hotreload/`)

```go
// Package hotreload watches alias config files and triggers ContextAdapter
// reload via aliasconfig.LoadDefault → aliasconfig.Validate → ReloadAliases.
//
// SPEC-GOOSE-CMDCTX-HOTRELOAD-001
// @MX:ANCHOR: fsnotify-based hot-reload entry point.
// @MX:REASON: Single watcher per daemon. Misroute breaks alias UX.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-HOTRELOAD-001 REQ-HOTRELOAD-010 / 011
package hotreload

import (
    "context"
    "errors"
    "path/filepath"
    "time"

    "github.com/fsnotify/fsnotify"

    "github.com/modu-ai/goose/internal/command/adapter"
    "github.com/modu-ai/goose/internal/command/adapter/aliasconfig"
    "github.com/modu-ai/goose/internal/llm/router"
)

// Reloader is the contract this watcher needs from the adapter. ContextAdapter
// satisfies this interface; tests use a fake.
type Reloader interface {
    ReloadAliases(m map[string]string) error
}

// Loader is the contract for fetching the latest alias map. Backed by
// aliasconfig.Loader in production.
type Loader interface {
    LoadDefault() (map[string]string, error)
}

// Validator is the contract for validating a reloaded map. Backed by
// aliasconfig.Validate in production.
type Validator interface {
    Validate(m map[string]string, registry *router.ProviderRegistry, strict bool) []error
}

// Options configure a Watcher.
type Options struct {
    AliasFilePath  string                 // absolute path resolved by ALIAS-CONFIG-001
    Registry       *router.ProviderRegistry
    Reloader       Reloader
    Loader         Loader
    Validator      Validator
    Strict         bool
    DebounceWindow time.Duration          // default 100ms
    Logger         *zap.Logger
}

// Watcher subscribes to fsnotify events on the alias file's parent directory
// and triggers reload after debounce.
type Watcher struct {
    opts    Options
    fsw     *fsnotify.Watcher
    cancel  context.CancelFunc
    done    chan struct{}
}

// New constructs a Watcher. Returns ErrInvalidOptions on missing required fields.
func New(opts Options) (*Watcher, error) { /* ... */ }

// Run starts the watcher loop. Blocks until ctx is canceled or fatal error occurs.
func (w *Watcher) Run(ctx context.Context) error { /* ... */ }

// Stop signals the watcher to terminate gracefully.
func (w *Watcher) Stop() { /* ... */ }

// debounce.go
//
// debounceTimer wraps time.Timer to support reset-on-event semantics.
type debounceTimer struct { /* ... */ }
func (d *debounceTimer) Reset(window time.Duration) { /* ... */ }
func (d *debounceTimer) C() <-chan time.Time { /* ... */ }

// Sentinel errors
var (
    ErrInvalidOptions = errors.New("hotreload: invalid options")
)
```

### 6.8 `/reload aliases` slash command (Optional REQ-HOTRELOAD-013)

```go
// internal/command/builtin/reload.go (가칭)
//
// SPEC-GOOSE-CMDCTX-HOTRELOAD-001 REQ-HOTRELOAD-013
package builtin

import (
    "github.com/modu-ai/goose/internal/command"
)

// reloadAliasesCmd implements /reload aliases.
type reloadAliasesCmd struct {
    loader    Loader     // injected at registration time
    validator Validator
    reloader  Reloader
    registry  *router.ProviderRegistry
    strict    bool
}

func (c *reloadAliasesCmd) Metadata() command.Metadata {
    return command.Metadata{
        Name:    "/reload",
        Subcmd:  "aliases",
        Mutates: false, // does not mutate loop.State; aliasMap mutation is via adapter
    }
}

func (c *reloadAliasesCmd) Execute(ctx context.Context, sctx command.SlashCommandContext, args []string) error {
    m, err := c.loader.LoadDefault()
    if err != nil {
        return fmt.Errorf("alias reload: load failed: %w", err)
    }
    if errs := c.validator.Validate(m, c.registry, c.strict); len(errs) > 0 && c.strict {
        return fmt.Errorf("alias reload: validation failed: %w", errors.Join(errs...))
    }
    if err := c.reloader.ReloadAliases(m); err != nil {
        return fmt.Errorf("alias reload: store failed: %w", err)
    }
    return nil
}
```

### 6.9 fsnotify 이벤트 처리 의사코드

```
Watcher.Run(ctx):
  fsw, err = fsnotify.NewWatcher()
  if err != nil: return err
  fsw.Add(filepath.Dir(opts.AliasFilePath))  // 디렉토리 단위
  baseName = filepath.Base(opts.AliasFilePath)

  debouncer = newDebounceTimer(opts.DebounceWindow)

  for:
    select:
      case <-ctx.Done():
        fsw.Close()
        return nil

      case ev := <-fsw.Events:
        if filepath.Base(ev.Name) != baseName: continue  // 다른 파일 무시
        if ev.Op & (Write | Create | Rename) == 0: continue
        debouncer.Reset(opts.DebounceWindow)  // burst coalescing

      case err := <-fsw.Errors:
        log.Warn("fsnotify error", err)
        // attempt re-watch (REQ-HOTRELOAD-034)
        if retryErr := fsw.Add(filepath.Dir(opts.AliasFilePath)); retryErr != nil:
          log.Warn("re-watch failed, terminating", retryErr)
          return retryErr

      case <-debouncer.C():
        m, err := opts.Loader.LoadDefault()
        if err != nil:
          log.Warn("load failed; existing map preserved", err)
          continue  // REQ-HOTRELOAD-033
        if errs := opts.Validator.Validate(m, opts.Registry, opts.Strict); len(errs) > 0 && opts.Strict:
          log.Warn("validation failed; existing map preserved", errs)
          continue  // REQ-HOTRELOAD-032
        if err := opts.Reloader.ReloadAliases(m); err != nil:
          log.Warn("reload failed", err)
          continue
        log.Info("alias hot-reload succeeded", size=len(m))  // REQ-HOTRELOAD-012
```

### 6.10 TDD 진입 순서 (RED → GREEN → REFACTOR)

| 순서 | 작업 | 검증 |
|------|------|-----|
| T-001 | `errors.go` 의 ErrNilAliasMap, ErrNilRegistry 추가 | 컴파일 |
| T-002 | ContextAdapter struct 의 atomic.Pointer 전환 + New() 초기화 | 기존 CMDCTX-001 AC 회귀 (AC-HOTRELOAD-052) |
| T-003 | ResolveModelAlias step 1-2 read adaptation | AC-HOTRELOAD-005, AC-CMDCTX-002~003 회귀 |
| T-004 | ReloadAliases / ReloadRegistry API 구현 | AC-HOTRELOAD-002, 003, 030, 031 |
| T-005 | WithContext children 일관성 검증 | AC-HOTRELOAD-004 |
| T-006 | race detector 검증 | AC-HOTRELOAD-006, AC-HOTRELOAD-035 |
| T-007 | hotreload 패키지 — debounce timer | AC-HOTRELOAD-020 |
| T-008 | hotreload 패키지 — fsnotify integration (fakeFsnotify) | AC-HOTRELOAD-010, 021 |
| T-009 | hotreload 패키지 — reload chain 호출 순서 | AC-HOTRELOAD-011, 012, 032, 033, 042 |
| T-010 | hotreload 패키지 — watcher error 복구 | AC-HOTRELOAD-034 |
| T-011 | `/reload aliases` builtin 등록 | AC-HOTRELOAD-013 |
| T-012 | CMDCTX-001 spec.md amendment governance | AC-HOTRELOAD-050 |
| T-013 | go.mod 신규 의존성 추가 | AC-HOTRELOAD-051 |

### 6.11 TRUST 5 매핑

| 차원 | 본 SPEC 적용 |
|------|-----------|
| Tested | 19 REQ / 23 AC, ≥ 90% coverage 목표, race detector 100 reader + 50 reloader 동시 검증 |
| Readable | godoc on every exported type, English code comments per `language.yaml` |
| Unified | gofmt + golangci-lint clean, `go vet -copylocks` 0 violation (AC-HOTRELOAD-035) |
| Secured | nil-input graceful, fail-safe (reload 실패 시 기존 map 유지), watcher error 단일 retry 후 graceful 종료 |
| Trackable | conventional commits, SPEC ID in commit body, MX:ANCHOR on Watcher / atomic.Pointer fields |

### 6.12 의존성 결정 (라이브러리)

- `sync/atomic` (stdlib) — atomic.Pointer
- `context` (stdlib)
- `path/filepath` (stdlib)
- `time` (stdlib) — debounce
- `errors` (stdlib)
- `github.com/fsnotify/fsnotify` — **신규 외부 의존성 1건** (research §8 D-9)
- `github.com/modu-ai/goose/internal/command` — interface, ModelInfo
- `github.com/modu-ai/goose/internal/command/adapter` — ContextAdapter (cyclic 회피 위해 hotreload 패키지가 adapter 를 import)
- `github.com/modu-ai/goose/internal/command/adapter/aliasconfig` — Loader, Validate (ALIAS-CONFIG-001 산출)
- `github.com/modu-ai/goose/internal/llm/router` — ProviderRegistry
- `go.uber.org/zap` — logger (기존 사용)
- `github.com/stretchr/testify/assert` — 기존 사용 패턴 (테스트 only)

신규 외부 의존성: **1건 (fsnotify)**.

---

## 7. 의존성 (Dependencies)

| 종류 | 대상 SPEC | 관계 |
|------|---------|------|
| amendment 대상 | SPEC-GOOSE-CMDCTX-001 (implemented v0.1.1, FROZEN) | 본 SPEC 이 ContextAdapter struct + ResolveModelAlias 알고리즘 amendment. v0.1.1 → v0.X.0 (X >= 2) 동시 갱신. |
| 데이터 소스 | SPEC-GOOSE-ALIAS-CONFIG-001 (planned, Batch A) | `aliasconfig.LoadDefault` / `aliasconfig.Validate` 를 watcher 가 호출. read-only consume. |
| 위임 대상 | SPEC-GOOSE-ROUTER-001 (implemented, FROZEN) | `*router.ProviderRegistry` read-only 사용. **변경 없음**. |
| 위임 대상 | SPEC-GOOSE-COMMAND-001 (implemented, FROZEN) | `command.SlashCommandContext` 인터페이스 (변경 없음), `command.Metadata` (Optional `/reload aliases` builtin 등록). |
| amendment 충돌 가능 | SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 (planned) | 동시 amendment 시 §6.4 본문 충돌 가능. governance: 직렬 머지 + 직렬 버전 bump (research §7.3). |
| amendment 충돌 가능 | SPEC-GOOSE-CMDCTX-TELEMETRY-001 (가칭, TBD) | 동시 amendment 시 §6.4 본문 후처리 충돌 가능. 동일 governance. |

본 SPEC 이 amendment 하는 SPEC: **CMDCTX-001 (1건)**. 본 SPEC 이 변경하지 않는 SPEC: ALIAS-CONFIG-001, ROUTER-001, COMMAND-001 (모두 read-only consume).

---

## 8. Acceptance Test 전략

### 8.1 ContextAdapter unit tests (adapter_test.go amendment)

기존 CMDCTX-001 의 AC-CMDCTX-001 ~ 019 회귀 보존 + 신규 AC-HOTRELOAD-001 ~ 006, 030, 031 추가.

### 8.2 hotreload 패키지 unit tests (watcher_test.go)

테이블 드리븐 + 다음 fake injection:

- `fakeFsnotify` — fsnotify.Watcher 의 mock (Events / Errors 채널 직접 주입)
- `fakeClock` — debounce timer 의 시간 의존성 추상화
- `fakeLoader` — aliasconfig.Loader 의 stub
- `fakeValidator` — aliasconfig.Validate 의 stub
- `fakeReloader` — ContextAdapter.ReloadAliases 의 stub (호출 카운터)

### 8.3 race detector

```bash
go test -race -count=10 ./internal/command/adapter/...
go test -race -count=10 ./internal/command/adapter/hotreload/...
```

AC-HOTRELOAD-006 의 100 reader + 50 reloader 시나리오 포함.

### 8.4 정적 검증

- AC-HOTRELOAD-035: `go vet -copylocks ./internal/command/adapter/...` 0 violation
- AC-HOTRELOAD-051: `go.mod` diff 검증 — fsnotify 1건만 신규 require
- AC-HOTRELOAD-005: 정적 grep — ResolveModelAlias 함수 본문 내 `a.registry` / `a.aliasMap` 직접 접근 횟수

### 8.5 Coverage 목표

- 라인 커버리지: ≥ 90% (`internal/command/adapter/`, `internal/command/adapter/hotreload/`)
- branch 커버리지: ≥ 85%
- ReloadAliases / ReloadRegistry / Watcher.Run / debounce 모두 happy path + nil path + error path 검증

### 8.6 Lint / Format 게이트

- `gofmt -l . | grep . && exit 1` — clean
- `golangci-lint run ./internal/command/adapter/... ./internal/command/adapter/hotreload/...` — 0 issues
- godoc on every exported identifier

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 영향 | 완화 |
|---|------|----|------|
| R-1 | CMDCTX-001 v0.X.0 amendment 가 PERMISSIVE-ALIAS-001 / TELEMETRY 와 충돌 | 중 | research §7.3 의 직렬 머지 + 직렬 버전 bump 트랙 채택. 본 SPEC 머지 시점에 다른 amendment SPEC 의 plan/run 상태를 확인하고 실제 버전 숫자 결정. |
| R-2 | fsnotify cross-platform 의미론 불일치 (macOS coalescing vs Linux 개별 이벤트) | 중 | research §5.1 분석. 디렉토리 단위 watch + basename 필터 + debounce 100ms 로 OS 차이 흡수. fakeFsnotify 기반 unit test 가 OS 의존성 제거. integration test 는 OS 별 CI matrix. |
| R-3 | atomic.Pointer 전환이 기존 CMDCTX-001 race 검증 (AC-CMDCTX-014) 회귀 | 중 | AC-HOTRELOAD-052 가 회귀 금지 검증. T-002 단계에서 기존 race 테스트 먼저 PASS 검증 후 신규 reload 테스트 추가. |
| R-4 | fsnotify 의존성이 build size / cross-compile 에 부담 | 저 | REQ-HOTRELOAD-041 의 build tag 분기 (Optional, v0.1.0 stub) 가 후속 완화. v0.1.0 은 default-on. |
| R-5 | reload 빈도가 매우 높을 때 (예: log rotation 이 alias 파일과 같은 디렉토리) debounce 가 영구 reset | 저 | basename 필터가 다른 파일 이벤트 무시. log rotation 이 같은 파일에 발생하는 경우는 구조적 비현실 — alias 파일은 사용자 편집 대상. |
| R-6 | reload 후 active model 이 새 registry 에 부재 (active 모델 invalidation) | 중 | v0.1.0 비대상 (REQ-HOTRELOAD-040 Optional). 사용자가 다음 `/model` 호출 시 새 registry 기준 lookup. UX 결손은 후속 SPEC. |
| R-7 | 다중 ContextAdapter 인스턴스가 동일 alias 파일을 watch (multi-session) | 저 | CMDCTX-001 §Exclusions #9 와 동일 가정 — v0.1.0 은 단일 ContextAdapter. multi-session 은 별도 SPEC 책임. |
| R-8 | Optional REQ (040, 041, 042) 가 v0.1.0 에서 stub 되어 후속 가치 누수 | 저 | research §9 Open Questions 에 명시. plan 단계에서 사용자 결정 권장. v0.1.0 implementation 은 REQ-HOTRELOAD-042 (logger) 만 필수, 나머지 stub 허용. |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 이 **명시적으로 제외**하는 항목 (어느 후속 SPEC 이 채워야 하는지 명시):

1. **alias config 파일 schema / loader / Validate 알고리즘** — `SPEC-GOOSE-ALIAS-CONFIG-001` (planned) 책임. 본 SPEC 은 `aliasconfig.LoadDefault` / `aliasconfig.Validate` 를 호출만.
2. **registry hot-reload via SIGHUP signal handler** — POSIX 전용, Windows 미지원. 후속 SPEC (TBD-SIGHUP).
3. **registry hot-reload via HTTP API** (예: POST `/api/v1/registry/reload`) — daemon 에 HTTP control plane 추가. 후속 SPEC (TBD-CONTROL-API).
4. **provider plugin hot-load** — 런타임 `.so` 로드 / Go plugin 시스템. 별도 architecture 결정 필요. 본 SPEC 비대상.
5. **multi-session adapter** — 단일 프로세스 다중 세션 multiplexing. CMDCTX-001 §Exclusions #9 와 동일 위임.
6. **active model auto-fallback** — reload 후 active model 이 새 registry 에 부재 시 자동 default 모델 전환. v0.1.0 Optional REQ-HOTRELOAD-040, stub. 후속 SPEC.
7. **fsnotify build tag 분기** — `// +build !fsnotify` 환경 빌드. v0.1.0 Optional REQ-HOTRELOAD-041, stub. 후속 결정.
8. **`Validate` 알고리즘 변경** — ALIAS-CONFIG-001 §6 의 strict / lenient 의미론은 FROZEN. 본 SPEC 은 호출만.
9. **multi-file alias overlay reload** — ALIAS-CONFIG-001 의 user file + project file overlay 가 reload 시점에 어떻게 다시 merge 되는지. 본 SPEC 은 `aliasconfig.LoadDefault` 가 매번 호출 시 fresh overlay 를 반환한다고 가정. 별도 검증은 ALIAS-CONFIG-001 책임.
10. **Telemetry / metrics emission** — reload 카운트 / latency 수집. CMDCTX-001 §Exclusions #6 와 동일 후속 위임.
11. **Hot-reload of LoopController / loop.State** — REQ-CMDCTX-016 invariant (loop state 단일 owner) 위반. 본 SPEC 비대상.
12. **사용자에게 reload UX 가이드 / 문서** — README / docs 별도 작업.

---

## 11. References

### 11.1 의존 SPEC

- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — amendment 대상 (FROZEN reference, v0.1.1)
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` — 데이터 소스 (planned, Batch A)
- `.moai/specs/SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001/spec.md` — amendment 충돌 분석 대상 (planned)
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — ProviderRegistry API (FROZEN)
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — SlashCommandContext (FROZEN)

### 11.2 코드 anchor (CMDCTX-001 implementation, PR #52 c018ec5)

- `internal/command/adapter/adapter.go` — ContextAdapter struct (amendment 대상)
- `internal/command/adapter/alias.go` — resolveAlias 알고리즘 (step 1-2 amendment)
- `internal/command/adapter/adapter_test.go` — race detector 검증 패턴
- `internal/command/adapter/controller.go` — LoopController (변경 없음)
- `internal/command/errors.go` — ErrUnknownModel sentinel (변경 없음)

### 11.3 외부 참조

- `https://pkg.go.dev/sync/atomic#Pointer` — atomic.Pointer Go stdlib doc
- `https://pkg.go.dev/github.com/fsnotify/fsnotify` — fsnotify cross-platform watcher
- research §5.1 fsnotify cross-platform 의미론 분석

### 11.4 부속 문서

- `research.md` (본 디렉토리) — 8 design decisions, fsnotify 분석, amendment governance
- `progress.md` (본 디렉토리) — phase log

### 11.5 Local convention

- `CLAUDE.local.md §2.5` — 코드 주석 영어 정책

---

## 12. Constitution Alignment

본 SPEC 은 다음 프로젝트 헌법(`.moai/project/tech.md`) 항목을 준수한다:

- **Go 1.23+ tooling**: stdlib `sync/atomic` 의 `atomic.Pointer[T]` (Go 1.19+ available, Go 1.23 OK).
- **TRUST 5**: §6.11 매핑 참조.
- **Code comment 영어 정책** (`language.yaml code_comments: en`): 모든 신규 godoc / @MX 본문 영어.
- **TIME ESTIMATION 금지**: §6.10 TDD 진입 순서는 priority + ordering 만 사용, 시간 단위 미사용.
- **신규 외부 의존성 최소화**: 1건 (fsnotify) — research §8 D-9 의 명시적 결정. AC-HOTRELOAD-051 가 정적 검증.

---

## 13. Acceptance Summary

- **REQ count**: 22 (Ubiquitous 6, Event-Driven 4, State-Driven 3, Unwanted 6, Optional 3 — `REQ-HOTRELOAD-{001..006, 010..013, 020..022, 030..035, 040..042}`)
- **AC count**: 23 (`AC-HOTRELOAD-{001..006, 010..013, 020..022, 030..035, 042, 050..052}`)
- **신규 외부 의존성**: 1 (fsnotify)
- **신규 패키지**: 2 (`internal/command/adapter/hotreload/`, `internal/command/builtin/reload.go` — Optional)
- **수정 기존 패키지**: 1 (`internal/command/adapter/` — atomic.Pointer 전환 + Reload* API + read-side adaptation)
- **amendment 대상 SPEC**: 1 (CMDCTX-001 v0.1.1 → v0.X.0, X >= 2)
- **FROZEN consume SPEC**: 3 (ROUTER-001, COMMAND-001, ALIAS-CONFIG-001 — read-only)

---
id: SPEC-GOOSE-DAEMON-WIRE-001
version: 0.1.0
status: completed
completed: 2026-04-27
created_at: 2026-04-25
updated_at: 2026-04-27
author: manager-spec
priority: P1
phase: 0
size: 중(M)
lifecycle: spec-anchored
labels: [phase-0, area/runtime, area/core, area/integration, type/feature, priority/p1-high]
issue_number: null
---

# SPEC-GOOSE-DAEMON-WIRE-001 — goosed Production Daemon Wire-up (CORE × CONFIG × HOOK × TOOLS × SKILLS × CONTEXT × QUERY)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-25 | 초안 작성. 7건 cross-package SPEC implementation의 production daemon wire-up 정합화. cross-pkg audit `REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25` 의 D-CORE-IF-1/2 결함을 main.go 통합 단계에서 종결한다. CORE-001 v1.1.0 OI-CORE-1/2가 신규 코드(`internal/core/session.go`, `internal/core/drain.go`)로 닫혔으나, **그 코드가 production daemon main.go의 등록 시퀀스와는 아직 결합되어 있지 않다**는 잔여 gap을 본 SPEC이 인수한다. plan 단계 산출물만 작성 — implementation은 후속 별도 세션. | manager-spec |

---

## 1. 개요 (Overview)

본 SPEC은 **신규 코드 로직을 도입하지 않는다.** 이미 머지된 7건의 cross-package SPEC(CORE-001 v1.1.0 / CONFIG-001 v0.3.1 / CONTEXT-001 v0.1.3 / QUERY-001 v0.1.3 / HOOK-001 v0.3.1 / TOOLS-001 v0.1.2 / SKILLS-001 v0.3.1)은 각자 자신의 패키지 내부에서 **GREEN**이며, 패키지 외부에 노출하는 cross-package 인터페이스 또한 모두 정의되어 있다. 그러나 production daemon인 `cmd/goosed/main.go`는 아직 다수 패키지를 import하지 않고 있고, 따라서 **인터페이스 자체는 존재하나 등록 호출이 빠져 있어 런타임 효과가 없다**.

본 SPEC이 통과한 시점에서 `goosed`는:

- 부팅 시 `core.Runtime` / `hook.HookRegistry` / `tools.Registry` / `skill.SkillRegistry` 4종 레지스트리를 모두 초기화하고,
- HOOK ↔ CORE 사이의 시그니처 적응을 위한 `WorkspaceRootResolverAdapter`를 등록하며,
- TOOLS의 `Registry.Drain`을 `core.Runtime.Drain.RegisterDrainConsumer`로 등록하여 SIGTERM 경로의 in-flight tool call 보호를 활성화하고,
- SKILLS의 `FileChangedConsumer`를 `hookRegistry.SetSkillsFileChangedConsumer`로 등록하여 conditional skill의 FileChanged trigger 경로를 닫고,
- nil consumer 등록 거부·EX_CONFIG fail-fast·`ErrHookSessionUnresolved` fail-closed 의무를 모두 보존한다.

본 SPEC은 **wire-up + adapter + 통합 테스트**만 담당하며, 각 SPEC의 도메인 로직(예: QueryEngine 호출, Agent runtime, gRPC 서비스 등록)은 후속 SPEC으로 분리된다.

---

## 2. 배경 (Background)

### 2.1 Audit-loop 닫기

`REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25`(이하 "audit")는 TOOLS-001 / HOOK-001 머지 시 도입된 10건의 cross-package interface stub을 4개 consumer SPEC(SKILLS-001 / CORE-001 / CLI-001 / SUBAGENT-001)에 매핑하고, **CORE-001 SPEC 본문에 S-2(`WorkspaceRoot`)와 S-10(`Registry.Drain`) 계약이 누락되어 있음**을 결함 D-CORE-IF-1 / D-CORE-IF-2로 등록하였다. CORE-001 v1.1.0 amendment(2026-04-25)는 SPEC 문서 차원에서 두 결함을 닫고 OI-CORE-1/2를 신설하였으며, 같은 날 PR #16에서 `internal/core/session.go` + `internal/core/drain.go` + `core/runtime.go` Sessions/Drain 필드 추가로 implementation을 완료하였다.

그러나 audit이 §"Implementation 진입 전 체크포인트"에서 명시한 **production daemon에서의 등록 시퀀스 검증**(`hookRegistry.SetSkillsFileChangedConsumer(skillRegistry.FileChangedConsumer)` 호출, `tools.Registry.Drain`이 shutdown hook chain에 등록되어 SIGTERM 시 호출되는지 검증)은 **CORE-001 v1.1.0 implementation 범위에 포함되지 않았다**. CORE-001은 등록 인프라(`DrainCoordinator`, `RunAllDrainConsumers`)만 제공하고, 어떤 consumer가 어떤 시점에 등록되는지는 daemon 통합 SPEC에 위임한다. 본 SPEC이 그 위임을 인수한다.

### 2.2 시그니처 mismatch 일람 (HOOK ↔ CORE)

| 항목 | HOOK 측 | CORE 측 | 정합 방식 |
|------|--------|--------|---------|
| WorkspaceRoot | `interface { WorkspaceRoot(sessionID string) (string, error) }` (`hook/types.go:236-238`) | 패키지 레벨 헬퍼 `core.WorkspaceRoot(id) string` (`core/session.go`) | adapter struct가 CORE 함수를 호출 후 빈 문자열을 `ErrHookSessionUnresolved`로 변환 |
| FileChangedConsumer | `type SkillsFileChangedConsumer func(ctx, paths) []string` (`hook/types.go:228-231`) | `func (r *SkillRegistry) FileChangedConsumer(ctx, changed) []string` 메서드 | 메서드 값을 함수로 직접 등록 (시그니처 일치) |
| Drain | `core.RegisterDrainConsumer(DrainConsumer{Name, Fn, Timeout})` | `tools.Registry.Drain()` 메서드 | adapter `DrainConsumer{Name: "tools.Registry", Fn: func(ctx) error { r.Drain(); return nil }}` |

**중요 정정**: 호출자가 사양에서 "`WorkspaceRoot(id) string` ↔ `WorkspaceRoot(id) (string, error)`" mismatch라고 기술하였으나, 실제 코드(`hook/types.go:236-238`)는 이미 `(string, error)` 시그니처로 정의되어 있다. 따라서 mismatch는 "에러 시그니처 유무"가 아니라 "**resolver는 interface, CORE는 패키지 레벨 함수**"이며, adapter struct로 interface를 만족시키면 된다. 이 mismatch 분류는 §6.2 adapter 패턴에서 정확히 처리된다.

### 2.3 현재 main.go 진입점 (9-step 흐름)

`cmd/goosed/main.go::run()`는 현재 9단계로 구성된다 (commit `0a71e8e` 기준):

1. `config.Load()` — CONFIG-001 계층형 로더
2. `core.NewLogger(cfg.Log.Level)` — 구조화 zap 로거
3. `signal.NotifyContext(SIGINT, SIGTERM)` — root context
4. `core.NewRuntime(logger, rootCtx)` — Runtime + Sessions + Drain + Shutdown
5. `health.New().ListenAndServe(cfg.Transport.HealthPort)` — 헬스 서버
6. `rt.State.Store(StateServing)` — serving 상태 전환
7. `<-rootCtx.Done()` — SIGINT/SIGTERM 대기
8. `rt.State.Store(StateDraining)` + `healthSrv.Shutdown(shutdownCtx)`
9. `rt.Drain.RunAllDrainConsumers(shutdownCtx)` + `rt.Shutdown.RunAllHooks(shutdownCtx)` + exit

**미통합 항목**: `internal/hook`, `internal/tools`, `internal/skill`, `internal/context`, `internal/query`. main.go에서 이들 패키지의 import 자체가 없으므로 — 패키지 내부의 cross-pkg surface(`SetSkillsFileChangedConsumer`, `Registry.Drain`)가 정의되어 있어도 **production daemon에서는 호출되지 않는다**. 본 SPEC이 닫는 gap이 정확히 이것이다.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `cmd/goosed/main.go`에 `internal/hook`, `internal/tools`, `internal/skill` 3개 패키지를 import하고, 부트스트랩 시퀀스에 초기화 단계를 추가한다 (CONTEXT-001 / QUERY-001은 consumer SPEC 영역이므로 본 SPEC에서는 instance 생성만 옵션, 등록은 OUT).
2. `WorkspaceRootResolverAdapter` 구현 — CORE의 패키지 레벨 헬퍼 `core.WorkspaceRoot(id) string`을 HOOK의 `WorkspaceRootResolver` interface(`(string, error)`)로 변환하는 adapter struct를 신설한다. CORE 헬퍼가 빈 문자열을 반환하면 adapter는 `hook.ErrHookSessionUnresolved`로 변환하여 fail-closed 정책(HOOK-001 REQ-HK-021(b))을 보존한다.
3. `tools.Registry.Drain` → `core.Runtime.Drain.RegisterDrainConsumer` 등록 — `DrainConsumer{Name: "tools.Registry", Fn: func(ctx) error { toolsRegistry.Drain(); return nil }, Timeout: 10*time.Second}` 형태로 wrap하여 등록.
4. `skillRegistry.FileChangedConsumer` → `hookRegistry.SetSkillsFileChangedConsumer` 등록 — SkillRegistry 메서드 값을 그대로 consumer 함수 타입에 대입.
5. lifecycle 순서 명시 — `init → bootstrap(config/logger) → wire-up(registries, adapters, consumers) → serve(health) → drain(consumers) → shutdown(hooks)` 6-단계 상태 머신으로 main.go 흐름을 13단계로 확장.
6. Wire-up 통합 테스트 — `cmd/goosed/integration_test.go` (또는 `tests/integration/wire_test.go`)에 ① SIGTERM → drain consumer 호출 검증, ② FileChanged dispatch 검증, ③ WorkspaceRoot adapter empty-string → `ErrHookSessionUnresolved` 검증, ④ nil consumer 등록 거부 → EX_CONFIG 검증 4개 테스트 추가.

### 3.2 OUT OF SCOPE (명시적 제외)

본 SPEC이 **다루지 않는** 항목 (모두 후속 SPEC):

- gRPC 서버 등록 (TRANSPORT-001) — health server 외 transport는 본 SPEC 범위 밖.
- QueryEngine 실제 호출 — QUERY-001은 streaming engine을 제공하나, 본 SPEC은 consumer가 아니다. `internal/query` import는 옵션, 호출은 OUT.
- Agent runtime 통합 (AGENT-001) — `runAgent` 진입점은 SUBAGENT-001 + AGENT-001 합작이며 본 SPEC 범위 밖.
- InteractiveHandler 본체 구현 (CLI-001) — `hook.InteractiveHandler` interface 등록은 nil 허용. CLI-001 implementation 시점에 본체 등록.
- CoordinatorHandler / SwarmWorkerHandler 본체 (SUBAGENT-001) — permission bubbling 경로는 SUBAGENT-001 영역.
- PluginLoader 실 구현 (PLUGIN-001) — `hook.PluginHookLoader` interface는 정의만 되어 있으며 등록 OUT.
- LLM streaming 통합 (LLM-001 / ADAPTER-001/002) — provider 라우팅 wire-up은 별도 SPEC.
- CLI client 측 wire-up (CLI-001) — `goose` CLI 바이너리는 daemon과 별개 프로세스.
- ContextCompactor 등록 (CONTEXT-001 consumer는 QueryEngine, daemon 직접 등록 OUT).

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 §6.1-6.3의 Go 통합 시퀀스를 정본으로 참조한다. 본 §4 REQ 문장은 정본의 관측 가능한 행위만 표현한다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-WIRE-001 [Ubiquitous]** — The `goosed` daemon **shall** initialize all five cross-package registries (CORE-001 `core.Runtime`, CONFIG-001 `config.Config`, HOOK-001 `hook.HookRegistry`, TOOLS-001 `tools.Registry`, SKILLS-001 `skill.SkillRegistry`) at startup before transitioning to `StateServing`.

**REQ-WIRE-002 [Ubiquitous]** — The bootstrap sequence **shall** execute steps in the following total order: (1) `config.Load`, (2) `core.NewLogger`, (3) `signal.NotifyContext`, (4) `core.NewRuntime`, (5) `hook.NewHookRegistry`, (6) `tools.NewRegistry`, (7) `skill.LoadSkillsDir`, (8) `WorkspaceRootResolverAdapter` registration on the hook registry, (9) `core.Runtime.Drain.RegisterDrainConsumer` for `tools.Registry.Drain`, (10) `hookRegistry.SetSkillsFileChangedConsumer(skillRegistry.FileChangedConsumer)`, (11) `health.New().ListenAndServe`, (12) `rt.State.Store(StateServing)`, (13) `<-rootCtx.Done()`. Re-ordering is prohibited because each later step assumes resources from the earlier step are non-nil and ready.

### 4.2 Event-Driven (이벤트 기반)

**REQ-WIRE-003 [Event-Driven]** — **When** `core.NewRuntime(logger, rootCtx)` is invoked, the package-level default `core.SessionRegistry` **shall** be wired automatically such that subsequent `core.WorkspaceRoot(sessionID)` helper calls return either the registered workspace root or the empty string in a nil-safe manner without panicking.

**REQ-WIRE-004 [Event-Driven]** — **When** `tools.NewRegistry` returns a non-nil `*tools.Registry`, the daemon **shall** immediately invoke `core.Runtime.Drain.RegisterDrainConsumer(DrainConsumer{Name: "tools.Registry", Fn: <wrap>, Timeout: 10*time.Second})` where `<wrap>` is a closure invoking `toolsRegistry.Drain()` and returning `nil`. **When** the daemon subsequently receives `SIGTERM`, `core.Runtime.Drain.RunAllDrainConsumers(ctx)` **shall** invoke this registered consumer exactly once, and `toolsRegistry.IsDraining()` **shall** report `true` after the call returns.

**REQ-WIRE-005 [Event-Driven]** — **When** `skill.LoadSkillsDir(skillsRoot)` returns a non-nil `*skill.SkillRegistry`, the daemon **shall** invoke `hookRegistry.SetSkillsFileChangedConsumer(skillRegistry.FileChangedConsumer)` to wire the cross-package consumer. **When** subsequent `hookRegistry.DispatchFileChanged(ctx, paths)` is invoked with paths that match a registered conditional skill's `paths:` field, the dispatcher **shall** route the call through the registered consumer and return the matched skill ID list.

### 4.3 State-Driven (상태 기반)

**REQ-WIRE-006 [State-Driven]** — **While** the daemon is in `StateServing`, every cross-package consumer registration slot (`hookRegistry.skillsConsumer`, `core.Runtime.Drain.consumers` for the `tools.Registry` entry, `hookRegistry.handlers[*].Resolver`) **shall** hold a non-nil function pointer or non-nil interface value. A nil pointer in any slot at `StateServing` is a contract violation and **shall** be unreachable by construction (verified by the wire-up test in §6.4).

### 4.4 Unwanted Behavior (방지)

**REQ-WIRE-007 [Unwanted]** — **If** `WorkspaceRootResolverAdapter.WorkspaceRoot(sessionID)` is invoked and the underlying CORE helper `core.WorkspaceRoot(sessionID)` returns the empty string, **then** the adapter **shall** return `("", hook.ErrHookSessionUnresolved)` and **shall not** fall back to any fabricated path (e.g., process CWD, `/tmp`, `os.Getenv("HOME")`). This preserves HOOK-001 REQ-HK-021(b) fail-closed semantics.

**REQ-WIRE-008 [Unwanted]** — **If** any wire-up step (REQ-WIRE-002 steps 8 / 9 / 10) attempts to register a `nil` consumer or `nil` resolver — for example `hookRegistry.SetSkillsFileChangedConsumer(nil)` — **then** the registry **shall** return an explicit error (e.g., `hook.ErrInvalidConsumer`) without panicking, and `main.go::run()` **shall** log the error at ERROR level and return exit code `78` (`core.ExitConfig`, EX_CONFIG). The daemon **shall not** transition to `StateServing` with a half-wired registry.

### 4.5 Optional (선택적)

**REQ-WIRE-009 [Optional]** — **Where** a future SPEC (CLI-001) provides a non-nil `hook.InteractiveHandler` implementation, the wire-up sequence **shall** expose a hook point in `main.go` (e.g., a named `wireInteractiveHandler(rt, hookRegistry)` function call site) that allows registration without modifying the steps in REQ-WIRE-002. In the present SPEC scope, this hook point **may** invoke registration with a `nil` handler, in which case `hookRegistry` MUST accept the explicit-nil registration as a no-op (distinguished from REQ-WIRE-008's accidental-nil error) — the distinction is that REQ-WIRE-009 is a *placeholder* whereas REQ-WIRE-008 is an *unwired bug*.

---

## 5. Acceptance Criteria

이 섹션의 모든 AC는 **EARS 헤더(외층, 정본) + Test Scenario(내층, Given-When-Then)**의 두 층 구조를 따른다. SKILLS-001 v0.3.0 / CORE-001 v1.1.0 패턴 계승.

### 5.1 REQ → AC Traceability Matrix

| REQ | AC (primary) | AC (supplementary) | Note |
|-----|-------------|--------------------|------|
| REQ-WIRE-001 (5 registry init) | AC-WIRE-001 | — | 부트스트랩 정상 경로 |
| REQ-WIRE-002 (13-step total order) | AC-WIRE-001 | AC-WIRE-002, AC-WIRE-003 | 순서는 정상경로 + drain + filechanged 합산 검증 |
| REQ-WIRE-003 (SessionRegistry auto-wire) | AC-WIRE-004 | — | core.NewRuntime side effect |
| REQ-WIRE-004 (tools.Drain 등록) | AC-WIRE-002 | — | SIGTERM → drain 통합 |
| REQ-WIRE-005 (skills FileChanged 등록) | AC-WIRE-003 | — | dispatch 통합 |
| REQ-WIRE-006 (serving 상태 non-nil 보장) | AC-WIRE-001, AC-WIRE-006 | — | 정상경로 + nil 거부 합산 |
| REQ-WIRE-007 (empty → ErrHookSessionUnresolved) | AC-WIRE-005 | — | fail-closed |
| REQ-WIRE-008 (nil consumer 거부) | AC-WIRE-006 | — | EX_CONFIG fail-fast |
| REQ-WIRE-009 (InteractiveHandler placeholder) | AC-WIRE-007 | — | optional, 후속 CLI-001 호환성 |

### 5.2 Acceptance Criteria

#### AC-WIRE-001 — 정상 부트스트랩 (5 registry + health server) _(covers REQ-WIRE-001, REQ-WIRE-002, REQ-WIRE-006)_

**[Event-Driven]** **When** `goosed` is launched with a valid `~/.goose/config.yaml`, an empty `~/.goose/skills/` directory, and an unbound health port, the daemon **shall** transition through `init → bootstrap → wire-up → serving` within 500ms, **shall** complete steps (1) through (12) of REQ-WIRE-002 in the declared order with all five registries non-nil, **shall** respond `200 OK` with `{"status":"ok"}` on `GET /healthz`, and **shall** report `rt.State.Load() == StateServing`.

**Test Scenario (verification)**:
- **Given** 임시 디렉토리에 `~/.goose/config.yaml` (default values), 빈 `~/.goose/skills/`, 자유 포트(테스트가 사전에 listen+close로 확보), `GOOSE_HOME=<tmpdir>` 환경변수
- **When** `cmd := exec.Command("goosed")` 실행, 500ms 대기 후 `http.Get("/healthz")`
- **Then** (a) HTTP 200, body에 `"status":"ok"` 포함, (b) test harness가 daemon process를 attach하여 reflect/inspect 시 `rt.Sessions != nil`, `rt.Drain != nil`, `hookRegistry != nil`, `toolsRegistry != nil`, `skillRegistry != nil` 모두 관측, (c) zap 로그에서 `"goosed started"` INFO 라인 1건 + `"hook registry initialized"`, `"tools registry initialized"`, `"skills loaded"` 3건 INFO 라인이 declared 순서대로 출력 (zap log capture로 검증).

---

#### AC-WIRE-002 — SIGTERM → tools.Registry.Drain 호출 검증 _(covers REQ-WIRE-004, REQ-WIRE-002 step 9)_

**[Event-Driven]** **When** the running `goosed` daemon receives `SIGTERM`, the daemon **shall** transition state to `StateDraining` within 100ms, **shall** invoke the registered `tools.Registry.Drain` drain consumer exactly once via `core.Runtime.Drain.RunAllDrainConsumers`, and `toolsRegistry.IsDraining()` **shall** return `true` immediately after the consumer returns. Subsequent `tools.Executor.Run` invocations **shall** return `tools.ErrRegistryDraining`.

**Test Scenario (verification)**:
- **Given** 정상 부트스트랩된 daemon (AC-WIRE-001 setup), test harness가 `toolsRegistry`에 직접 reference 보유 (in-process integration test)
- **When** `syscall.Kill(daemonPID, syscall.SIGTERM)` 호출, 100ms 대기
- **Then** (a) `rt.State.Load() == StateDraining`, (b) `toolsRegistry.IsDraining() == true`, (c) zap 로그에 `"drain consumer completed" consumer="tools.Registry"` 라인 1건, (d) shutdown 완료 후 daemon exit code = 0, (e) drain consumer가 정확히 1회만 호출됨 (counter 또는 `sync.Once.Do` 관측).

---

#### AC-WIRE-003 — SkillsFileChanged dispatch 통합 _(covers REQ-WIRE-005)_

**[Event-Driven]** **When** the wired `hookRegistry.DispatchFileChanged(ctx, []string{"src/foo.ts"})` is invoked on a daemon whose `skillRegistry` contains a skill with `paths: ["src/**/*.ts"]` and ID `ts-helper`, the dispatcher **shall** route through the registered consumer (`skillRegistry.FileChangedConsumer`) and **shall** return a slice containing exactly `["ts-helper"]`. **When** the registered consumer is nil (negative case), the dispatcher **shall** return an empty slice without panicking.

**Test Scenario (verification)**:
- **Given** 임시 skills 디렉토리에 `ts-helper/SKILL.md` (frontmatter `name: ts-helper`, `description: "ts assistant"`, `paths: ["src/**/*.ts"]`) + 정상 부트스트랩
- **When** test harness가 `hookRegistry.DispatchFileChanged(ctx, []string{"src/foo.ts", "README.md"})` 호출
- **Then** (a) 반환 slice == `["ts-helper"]` (정확히 1건, README.md 매칭 안됨), (b) 동일 dispatch를 nil consumer 상태에서 호출하면 reflection으로 consumer를 nil로 만든 후 호출 시 panic 없이 빈 slice 반환, (c) zap 로그에 `"FileChanged dispatched, matched skills=1"` INFO 1건.

---

#### AC-WIRE-004 — WorkspaceRootResolver adapter (정상 경로) _(covers REQ-WIRE-003, REQ-WIRE-007 정상 분기)_

**[Event-Driven]** **When** `core.SessionRegistry.Register("sess-1", "/tmp/work")` has been called and `WorkspaceRootResolverAdapter.WorkspaceRoot("sess-1")` is subsequently invoked, the adapter **shall** return `("/tmp/work", nil)`. The adapter **shall** delegate to the package-level helper `core.WorkspaceRoot(sessionID string) string` and **shall not** maintain its own state cache.

**Test Scenario (verification)**:
- **Given** 정상 부트스트랩 후 test harness가 `core.DefaultSessions().Register("sess-1", "/tmp/work")` 호출 (또는 in-process daemon에서 동일 호출)
- **When** `adapter := workspaceRootResolverAdapter{}; path, err := adapter.WorkspaceRoot("sess-1")`
- **Then** (a) `path == "/tmp/work"`, (b) `err == nil`, (c) adapter struct에는 어떤 필드도 없음(empty struct, reflect로 검증), (d) 동일 sessionID에 대한 반복 호출이 동일 결과 반환(idempotent).

---

#### AC-WIRE-005 — Empty session → ErrHookSessionUnresolved (fail-closed) _(covers REQ-WIRE-007)_

**[Unwanted]** **If** `WorkspaceRootResolverAdapter.WorkspaceRoot("missing-session")` is invoked for a sessionID that has not been registered in `core.SessionRegistry`, **then** the adapter **shall** return `("", hook.ErrHookSessionUnresolved)`, **shall not** invoke `os.Getenv`, **shall not** fall back to process CWD, and **shall not** log the secret-bearing sessionID at INFO or higher level.

**Test Scenario (verification)**:
- **Given** 정상 부트스트랩 후 `core.SessionRegistry`가 비어 있음(또는 다른 세션만 등록)
- **When** `adapter.WorkspaceRoot("missing-session")`
- **Then** (a) `path == ""`, (b) `errors.Is(err, hook.ErrHookSessionUnresolved) == true`, (c) test harness가 `os.Getenv` 호출 횟수 monkey-patch counter로 관측 시 delta = 0, (d) test harness가 process working directory를 `/tmp/sandbox`로 설정한 상태에서 호출해도 반환 path에 `/tmp/sandbox`가 포함되지 않음, (e) zap 로그 capture에서 `"missing-session"` 문자열이 INFO 이상 레벨에 출현하지 않음(DEBUG는 허용).

---

#### AC-WIRE-006 — nil consumer 등록 거부 (EX_CONFIG fail-fast) _(covers REQ-WIRE-008, REQ-WIRE-006 nil-rejection 분기)_

**[Unwanted]** **If** `hookRegistry.SetSkillsFileChangedConsumer(nil)` is invoked at wire-up time, **then** the registry **shall** return `hook.ErrInvalidConsumer` (or equivalent sentinel error) without panicking, `main.go::run()` **shall** log an ERROR-level entry containing the consumer slot name and return exit code `78` (`core.ExitConfig`), and the daemon process **shall not** transition to `StateServing`.

**Test Scenario (verification)**:
- **Given** 부트스트랩 시퀀스가 step 10(`SetSkillsFileChangedConsumer`)에 도달, test harness가 `skillRegistry.FileChangedConsumer`를 reflection으로 nil function value로 swap
- **When** main.go가 `hookRegistry.SetSkillsFileChangedConsumer(nil)` 호출
- **Then** (a) `SetSkillsFileChangedConsumer` 반환 에러 `errors.Is(err, hook.ErrInvalidConsumer) == true` (또는 동등 sentinel), (b) `main.go::run()` 반환값 == `core.ExitConfig` (78), (c) zap 로그에 ERROR 레벨 `"wire-up failed: nil skills consumer"` 라인 1건, (d) `rt.State.Load() != StateServing` (`StateBootstrap` 또는 `StateStopped`), (e) health port에 listen 시도가 발생하지 않음(`netstat`로 검증 또는 health.New 호출 카운터로 관측).

추가 변형 케이스(별도 sub-test):
- `core.Runtime.Drain.RegisterDrainConsumer(DrainConsumer{Fn: nil, ...})` 호출 시 동일 EX_CONFIG fail-fast.
- `WorkspaceRootResolverAdapter`를 등록할 hook handler 슬롯에 nil resolver 등록 시 동일 EX_CONFIG fail-fast.

---

#### AC-WIRE-007 — InteractiveHandler placeholder (CLI-001 hook point) _(covers REQ-WIRE-009)_

**[Optional]** **Where** a future CLI-001 implementation provides a non-nil `hook.InteractiveHandler`, the daemon's wire-up sequence **shall** expose a stable function call site (e.g., `wireInteractiveHandler(rt, hookRegistry, handler)`) such that registration of the handler does not require modifying any of the 13 steps in REQ-WIRE-002. **Where** the present SPEC is shipped without CLI-001, the call site **may** pass `nil` and the registration **shall** be treated as an explicit no-op (distinguished from REQ-WIRE-008's nil rejection by the registration call's *intent flag* — e.g., a `wireInteractiveHandler(handler hook.InteractiveHandler, opts ...InteractiveOpt)` signature where `nil` handler with `WithExplicitNoOp()` is accepted).

**Test Scenario (verification)**:
- **Given** main.go에 `wireInteractiveHandler(rt, hookRegistry, nil, hook.WithExplicitNoOp())` 호출 (placeholder)
- **When** 정상 부트스트랩 진행
- **Then** (a) daemon이 정상적으로 `StateServing`에 도달, (b) `hookRegistry`의 InteractiveHandler 슬롯은 nil 상태로 유지(reflect로 관측), (c) `goose tool list` 등 InteractiveHandler를 요구하지 않는 경로는 정상 작동, (d) InteractiveHandler가 필요한 경로(CLI-001 영역)는 본 SPEC 범위 밖이므로 negative test는 CLI-001로 위임됨.

---

#### AC-WIRE-008 — Wire-up 회귀 차단 (full integration smoke) _(covers REQ-WIRE-001 ~ REQ-WIRE-008 합산 회귀)_

**[Ubiquitous]** A single end-to-end integration test in `cmd/goosed/integration_test.go` **shall** exercise the entire bootstrap → SIGTERM → drain → shutdown cycle in process, **shall** assert all observable invariants from AC-WIRE-001 through AC-WIRE-005 in one run, and **shall** complete within 5 seconds wall-clock.

**Test Scenario (verification)**:
- **Given** test harness가 `cmd/goosed/main.go::run()`를 in-process로 호출(별도 binary 빌드 없음), `os.Args`와 환경변수를 제어, `time.Sleep` 대신 channel-based readiness signal 사용
- **When** harness가 ① bootstrap 완료 대기 → ② AC-WIRE-001 / 003 / 004 assertion → ③ `signal.Stop` + manual `rt.Sessions.Register` → ④ AC-WIRE-005 assertion → ⑤ harness가 `cancel()`로 rootCtx 취소 → ⑥ AC-WIRE-002 assertion (Drain 호출) → ⑦ exit code 검증
- **Then** (a) 모든 sub-assertion이 5초 wall-clock 내 통과, (b) `-race` detector 활성화 시 race 0건, (c) goroutine leak 검증(test 종료 후 `runtime.NumGoroutine()` delta ≤ 2 — http server graceful shutdown 잔여 허용), (d) 임시 디렉토리는 `t.Cleanup`으로 모두 회수.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 통합 시퀀스 (목표 13-step 흐름)

```
┌─ 1. config.Load(LoadOptions{})                         (CONFIG-001)
├─ 2. core.NewLogger(cfg.Log.Level, "goosed", version)   (CORE-001)
├─ 3. signal.NotifyContext(SIGINT, SIGTERM)              (CORE-001)
├─ 4. core.NewRuntime(logger, rootCtx)                    (CORE-001 + Sessions + Drain)
├─ 5. hook.NewHookRegistry(logger)                        (HOOK-001)
├─ 6. tools.NewRegistry(logger)                           (TOOLS-001)
├─ 7. skill.LoadSkillsDir(cfg.SkillsRoot, logger)         (SKILLS-001)
├─ 8. resolver := workspaceRootResolverAdapter{}          (NEW: adapter)
│     hookRegistry.SetWorkspaceRootResolver(resolver)
├─ 9. rt.Drain.RegisterDrainConsumer(DrainConsumer{       (NEW: TOOLS → CORE)
│         Name: "tools.Registry",
│         Fn:   func(ctx context.Context) error { toolsRegistry.Drain(); return nil },
│         Timeout: 10 * time.Second,
│     })
├─10. err := hookRegistry.SetSkillsFileChangedConsumer(   (NEW: SKILLS → HOOK)
│         skillRegistry.FileChangedConsumer)
│     if err != nil { return core.ExitConfig }
├─11. healthSrv := health.New(rt.State, version, logger)  (CORE-001)
│     healthSrv.ListenAndServe(cfg.Transport.HealthPort)
├─12. rt.State.Store(core.StateServing)
└─13. <-rootCtx.Done()                                    (CORE-001)
       │
       ▼
   shutdown:
   ├─ rt.State.Store(StateDraining)
   ├─ healthSrv.Shutdown(shutdownCtx)
   ├─ rt.Drain.RunAllDrainConsumers(shutdownCtx)          ← step 9에서 등록한 tools.Drain 호출
   └─ rt.Shutdown.RunAllHooks(shutdownCtx)
```

### 6.2 `WorkspaceRootResolverAdapter` 구현 패턴

```go
// cmd/goosed/main.go (또는 별도 wire.go)
//
// workspaceRootResolverAdapter는 CORE의 패키지 레벨 헬퍼
//   core.WorkspaceRoot(sessionID string) string
// 를 HOOK이 요구하는 interface
//   hook.WorkspaceRootResolver { WorkspaceRoot(sessionID) (string, error) }
// 로 변환한다.
//
// 빈 문자열은 "session not found" 의미이며, HOOK-001 REQ-HK-021(b)의
// fail-closed 의무에 따라 ErrHookSessionUnresolved로 변환된다.
//
// adapter는 무상태(empty struct)이며 CORE의 default SessionRegistry에
// 의존한다. 별도 SessionRegistry가 필요한 경우 future SPEC에서
// 필드를 추가할 수 있다.
type workspaceRootResolverAdapter struct{}

func (workspaceRootResolverAdapter) WorkspaceRoot(sessionID string) (string, error) {
    path := core.WorkspaceRoot(sessionID)
    if path == "" {
        return "", hook.ErrHookSessionUnresolved
    }
    return path, nil
}
```

대안적으로 `func` 타입 어댑터로도 표현 가능 — 의도가 "interface 구현"이므로 struct를 채택한다(reflect 친화 + 향후 필드 추가 여유).

### 6.3 main.go diff 예시 (현재 9 → 목표 13 단계)

```go
// AS-IS (commit 0a71e8e, line 30-72):
import (
    "github.com/modu-ai/goose/internal/config"
    "github.com/modu-ai/goose/internal/core"
    "github.com/modu-ai/goose/internal/health"
)
// ... 9-step 흐름 ...

// TO-BE (이 SPEC implementation 후):
import (
    "github.com/modu-ai/goose/internal/config"
    "github.com/modu-ai/goose/internal/core"
    "github.com/modu-ai/goose/internal/health"
    "github.com/modu-ai/goose/internal/hook"     // NEW
    "github.com/modu-ai/goose/internal/skill"    // NEW
    "github.com/modu-ai/goose/internal/tools"    // NEW
)

func run() int {
    // 1-4: AS-IS와 동일 (config/logger/signal/runtime)
    // ...

    // 5. hook registry
    hookRegistry := hook.NewHookRegistry(logger)

    // 6. tools registry
    toolsRegistry := tools.NewRegistry(logger)

    // 7. skill registry
    skillRegistry, skillErrs := skill.LoadSkillsDir(cfg.SkillsRoot, skill.WithLogger(logger))
    for _, e := range skillErrs {
        logger.Warn("skill load partial error", zap.Error(e))
    }

    // 8. WorkspaceRoot adapter
    hookRegistry.SetWorkspaceRootResolver(workspaceRootResolverAdapter{})

    // 9. tools.Registry.Drain → core.Drain
    rt.Drain.RegisterDrainConsumer(core.DrainConsumer{
        Name:    "tools.Registry",
        Fn:      func(ctx context.Context) error { toolsRegistry.Drain(); return nil },
        Timeout: 10 * time.Second,
    })

    // 10. skills.FileChanged → hook
    if err := hookRegistry.SetSkillsFileChangedConsumer(skillRegistry.FileChangedConsumer); err != nil {
        logger.Error("wire-up failed: nil skills consumer", zap.Error(err))
        return core.ExitConfig
    }

    // 11-13: AS-IS의 health/serving/wait
    // ...
}
```

### 6.4 통합 테스트 위치

본 SPEC의 통합 테스트는 다음 두 위치 중 하나에 배치한다(implementation 시점에 결정, plan 단계에서는 옵션으로 명시):

| 옵션 | 위치 | 장점 | 단점 |
|------|------|------|------|
| A | `cmd/goosed/integration_test.go` | main.go와 동일 디렉토리, 빌드 의존성 없음 | `package main` 테스트는 internal symbol export 필요 |
| B | `tests/integration/wire_test.go` (신규 디렉토리) | 다중 SPEC 통합 테스트 적치 가능 | go.mod에 신규 path 등록 필요 |

권고: **옵션 A**. main.go와 동일 패키지에서 in-process로 `run()`를 호출하여 SIGTERM 시뮬레이션이 용이.

테스트 fixture는 `t.TempDir()` 기반으로 `~/.goose/config.yaml` + 빈 `~/.goose/skills/` + `ts-helper/SKILL.md`(AC-WIRE-003용)를 생성. 자유 포트는 `net.Listen("tcp", ":0")`로 확보 후 close.

---

## 7. 의존성 (Dependencies)

### 7.1 선행 SPEC (모두 implemented)

| SPEC | 버전 | 본 SPEC이 사용하는 surface |
|------|------|-------------------------|
| SPEC-GOOSE-CORE-001 | v1.1.0 | `core.NewRuntime`, `core.Runtime.{State, Drain, Shutdown, Sessions}`, `core.WorkspaceRoot`, `core.DrainConsumer`, `core.ExitConfig`, `core.StateServing/Draining` |
| SPEC-GOOSE-CONFIG-001 | v0.3.1 | `config.Load`, `config.Config.{Log, Transport, SkillsRoot}` |
| SPEC-GOOSE-HOOK-001 | v0.3.1 | `hook.NewHookRegistry`, `hookRegistry.SetWorkspaceRootResolver`, `hookRegistry.SetSkillsFileChangedConsumer`, `hookRegistry.DispatchFileChanged`, `hook.WorkspaceRootResolver`, `hook.ErrHookSessionUnresolved`, `hook.ErrInvalidConsumer`(존재 확인 필요) |
| SPEC-GOOSE-TOOLS-001 | v0.1.2 | `tools.NewRegistry`, `toolsRegistry.Drain`, `toolsRegistry.IsDraining`, `tools.ErrRegistryDraining` |
| SPEC-GOOSE-SKILLS-001 | v0.3.1 | `skill.LoadSkillsDir`, `skillRegistry.FileChangedConsumer` |
| SPEC-GOOSE-CONTEXT-001 | v0.1.3 | (OUT — daemon 직접 등록 X) |
| SPEC-GOOSE-QUERY-001 | v0.1.3 | (OUT — daemon 직접 등록 X) |

### 7.2 후속 SPEC (본 SPEC이 hook point만 노출)

| SPEC | 본 SPEC이 노출하는 hook point |
|------|----------------------------|
| SPEC-GOOSE-TRANSPORT-001 | gRPC server 등록 site (REQ-WIRE-002 step 11 직후 또는 step 12 직전) |
| SPEC-GOOSE-CLI-001 | `wireInteractiveHandler(rt, hookRegistry, handler)` placeholder (REQ-WIRE-009) |
| SPEC-GOOSE-SUBAGENT-001 | `CoordinatorHandler` / `SwarmWorkerHandler` 등록 site (HOOK-001 면) |
| SPEC-GOOSE-AGENT-001 | Agent runtime 등록 + RunAgent 진입점 |
| SPEC-GOOSE-PLUGIN-001 | `hookRegistry.SetPluginLoader(loader)` 등록 site |
| SPEC-GOOSE-LLM-001 / ADAPTER-001/002 | provider routing 등록 site (이미 머지된 router는 process-internal, daemon wire-up 옵션) |

### 7.3 외부 의존성

본 SPEC은 신규 외부 라이브러리를 **도입하지 않는다**. 모든 import는 stdlib + 기존 패키지(`go.uber.org/zap`)로 충족된다.

---

## 8. 리스크 및 완화 (Risks & Mitigation)

| ID | 리스크 | 영향 | 완화 |
|----|------|-----|------|
| R1 | nil consumer 등록 시 daemon panic — wire-up 실패가 graceful EX_CONFIG가 아닌 segfault로 이어지면 supervisor 재시작 루프 발생 | High | REQ-WIRE-008로 명시적 nil-check + EX_CONFIG 종료. AC-WIRE-006이 회귀 차단. `hook.ErrInvalidConsumer` sentinel을 HOOK-001 v0.3.2 minor로 신설(implementation 시점에 HOOK-001 amendment 또는 본 SPEC의 wire 코드에서 fallback wrapping). |
| R2 | 시그니처 mismatch 추가 변경 — 향후 HOOK-001이 `WorkspaceRootResolver`에 새 메서드(`UpdateRoot`, `ListSessions` 등)를 추가하면 adapter struct가 컴파일 실패 | Medium | adapter struct를 `cmd/goosed/wire.go`에 단일 파일로 격리. adapter 단위 테스트가 컴파일 단계에서 실패 → CI 회귀 차단. HOOK-001 SPEC amendment 시점에 본 SPEC도 amendment minor 발행. |
| R3 | SIGTERM 순서 race — drain consumer 등록(step 9)이 step 13의 SIGTERM 도착보다 늦으면 drain이 호출되지 않음. 부트스트랩이 매우 느린 환경(CI cold-start)에서 발생 가능 | Medium | step 9는 step 11(`ListenAndServe`)보다 먼저 실행됨이 REQ-WIRE-002 total order로 강제됨. 추가로 AC-WIRE-002가 in-process integration test에서 SIGTERM을 step 12 도달 후 송신하여 race 윈도우를 닫는다. signal handler 자체는 step 3(`signal.NotifyContext`)에서 이미 활성화. |
| R4 | skillRegistry 미적재 상태에서 FileChanged dispatch 호출 — 사용자가 빈 skills 디렉토리로 부팅 시 `LoadSkillsDir`가 빈 registry 반환, 그 상태에서 dispatch가 호출되면 빈 slice 반환되어야 함 | Low | SKILLS-001 §6.2 `SkillRegistry.FileChangedConsumer`는 빈 registry에서도 nil-safe (paths 매칭 0건 → 빈 slice). AC-WIRE-003의 negative case가 회귀 차단. |
| R5 | adapter struct empty 인스턴스 호출 — Go의 zero value 접근 패턴(`workspaceRootResolverAdapter{}.WorkspaceRoot(id)`)이 매 호출마다 zero value 생성으로 GC 압력 발생 | Trivial | 무시. struct가 무상태이므로 escape analysis로 stack 할당. Benchmark 시 1ns 미만. 만약 측정 이슈 발생 시 package-level singleton 변수로 전환. |
| R6 | SkillsRoot 환경별 차이 — `cfg.SkillsRoot`가 절대 경로 vs 상대 경로 시 daemon CWD 변경에 취약 | Low | CONFIG-001 `config.Config.SkillsRoot`가 절대 경로로 정규화되어 있는지 확인(implementation 시점에 CONFIG-001 코드 read-back). 상대 경로 발견 시 step 7 직전에 `filepath.Abs` 호출 추가. |

---

## 9. 참고 (References)

### 9.1 SPEC 문서

- `.moai/reports/cross-package-interface-audit-2026-04-25.md` — 본 SPEC이 닫는 audit 결함 D-CORE-IF-1/2의 정의 + Implementation 진입 전 체크포인트
- `.moai/specs/SPEC-GOOSE-CORE-001/spec.md` v1.1.0 — §3.1(9)/(10), §4.6, §6.2(`WorkspaceRootResolver`/`DrainConsumer`), §12 Open Items OI-CORE-1/2(closed)
- `.moai/specs/SPEC-GOOSE-HOOK-001/spec.md` v0.3.1 — REQ-HK-008(`SetSkillsFileChangedConsumer`), REQ-HK-021(b)(`ErrHookSessionUnresolved` fail-closed), §6.2 dispatchers
- `.moai/specs/SPEC-GOOSE-TOOLS-001/spec.md` v0.1.2 — REQ-TOOLS-011(`Registry.Draining`), AC-TOOLS-013(drain after Drain)
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` v0.3.1 — REQ-SK-007(`FileChangedConsumer`), AC-SK-004(gitignore matching)
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` v0.3.1 — `config.Load`, `Config.{Log, Transport, SkillsRoot}` 표면

### 9.2 코드 레퍼런스

- `cmd/goosed/main.go` (commit `0a71e8e`) — 현재 9-step 흐름, REQ-WIRE-002 정본의 baseline
- `internal/core/runtime.go` — `Runtime.{Sessions, Drain, Shutdown, State}`
- `internal/core/session.go` — `SessionRegistry`, 패키지 레벨 `WorkspaceRoot` 헬퍼
- `internal/core/drain.go` — `DrainCoordinator.RegisterDrainConsumer / RunAllDrainConsumers`
- `internal/hook/types.go:228-238` — `SkillsFileChangedConsumer`, `WorkspaceRootResolver`, `ErrHookSessionUnresolved`
- `internal/hook/registry.go:149-160` — `SetSkillsFileChangedConsumer`, `SkillsConsumer` getter
- `internal/tools/registry.go` — `Registry.Drain`, `Registry.IsDraining`
- `internal/skill/registry.go` — `SkillRegistry.FileChangedConsumer`

---

## 10. Exclusions (What NOT to Build)

본 SPEC이 **명시적으로 다루지 않는** 항목 (모두 후속 SPEC 위임):

1. **gRPC server 등록** — `internal/transport/` 또는 향후 `internal/grpc/` 패키지 도입은 SPEC-GOOSE-TRANSPORT-001 책임. 본 SPEC은 health server 외 어떤 transport listener도 추가하지 않는다.
2. **QueryEngine streaming 실 호출** — CONTEXT-001 / QUERY-001은 패키지 내부 GREEN이며 daemon 직접 등록 대상이 아니다. QueryEngine consumer는 후속 AGENT-001이 인수.
3. **Agent runtime** — `runAgent` 진입점, `AgentDefinition` 로더, swarm worker 라이프사이클은 SUBAGENT-001 + AGENT-001 합작이며 본 SPEC 범위 밖.
4. **InteractiveHandler 본체 구현** — y/n permission TUI prompt는 CLI-001 영역. 본 SPEC은 nil 등록 placeholder만 노출(REQ-WIRE-009).
5. **PluginLoader 실 구현** — `hook.PluginHookLoader` interface 정의는 HOOK-001에 있으나, 본 SPEC은 `SetPluginLoader` 등록 site만 후속에 노출하고 본체는 PLUGIN-001로 위임.
6. **LLM streaming 통합** — `internal/llm/router` provider 라우팅은 process-internal로 이미 동작하나, daemon 부트스트랩 시점의 wire-up은 LLM-001 / ROUTER-001 / ADAPTER-001/002의 통합 SPEC으로 분리.
7. **CLI client 측** — `goose` CLI 바이너리는 daemon과 별개 프로세스(stdio 또는 gRPC over UDS). 본 SPEC은 daemon 측만 다룬다.
8. **Coordinator/SwarmWorker handler** — permission bubbling 경로는 SUBAGENT-001 §6.7 책임. 본 SPEC의 hook point는 placeholder.
9. **워크플로우 yaml team orchestrator** — `workflow.yaml` team 구성 로더는 별도 SPEC.
10. **ContextCompactor 등록** — CONTEXT-001 `DefaultCompactor`는 QueryEngine consumer이며 daemon 직접 등록 OUT.

---

Version: 0.1.0
Last Updated: 2026-04-25
Status: planned

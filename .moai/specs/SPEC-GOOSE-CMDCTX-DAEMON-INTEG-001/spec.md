---
id: SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001
version: 0.1.0
status: completed
created_at: 2026-04-27
updated_at: 2026-05-04
author: manager-spec
priority: P2
issue_number: null
phase: 2
size: 중(M)
lifecycle: spec-anchored
labels: [area/runtime, area/cli, type/feature, priority/p2-medium]
---

# SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 — Daemon 진입점에서 ContextAdapter wiring

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-27 | 초안 작성. CMDCTX-001 (PR #52, c018ec5 implemented FROZEN) 의 `ContextAdapter` 와 그 의존성을 daemon 진입점 (`cmd/goosed/main.go`, DAEMON-WIRE-001 의 13-step bootstrap) 에서 instantiate 하고 RPC handler 에 주입하는 통합 wiring SPEC. ID 충돌 회피: 별도 존재 SPEC `SPEC-GOOSE-DAEMON-WIRE-001` (planned, P1, "goosed Production Daemon Wire-up") 와는 분리되며 본 SPEC 은 그 위에 ContextAdapter / dispatcher / LoopController / alias config 4 종 wiring 만 다룬다. single-session 가정 유지. multi-session 은 별도 SPEC. | manager-spec |

---

## 1. 개요 (Overview)

PR #52 (SPEC-GOOSE-CMDCTX-001 implemented) 머지로 `internal/command/adapter/` 에 `*ContextAdapter` 가 존재한다. 그러나 daemon 진입점은 그 adapter 를 instantiate 하지 않고, dispatcher (PR #50 SPEC-GOOSE-COMMAND-001) 도 RPC handler 에 wired 되어 있지 않다. 결과적으로 사용자가 `/clear`, `/compact`, `/model`, `/status` 같은 슬래시 명령을 daemon 으로 보내도 어떤 부수 효과도 발생하지 않는다.

본 SPEC 은 그 wiring 을 닫는다. 본 SPEC 수락 시점에서:

- daemon bootstrap 이 `*ContextAdapter` 를 process 단일 인스턴스로 instantiate 한다.
- dispatcher 가 RPC handler 에 주입되어 `ProcessUserInput` 요청에서 호출된다.
- LoopController (CMDLOOP-WIRE-001 implementation) 가 `core.Runtime.Drain` 에 등록되어 SIGTERM 시 graceful drain 된다.
- alias config (ALIAS-CONFIG-001 implementation) 가 `Options.AliasMap` 에 주입된다. 부재 시 빈 맵 fallback.
- plan mode 가 RPC response 의 `plan_mode` metadata key 로 노출된다.
- nil 의존성 / wiring 실패 시 EX_CONFIG fail-fast 한다.

본 SPEC 은 **wiring + ctx 전파 + drain 순서 + RPC error 매핑** 만 다룬다. dispatcher / adapter / LoopController / alias loader 의 본체 구현은 모두 의존 SPEC 의 책임. RPC framework (Connect-gRPC) 의 도입은 본 SPEC 범위 외 — TRANSPORT-001 후속.

---

## 2. 배경 (Background)

### 2.1 ID 충돌 회피 / 별 SPEC

기존에 머지된 `SPEC-GOOSE-DAEMON-WIRE-001` (v0.1.0, planned, P1) 은 "goosed Production Daemon Wire-up — CORE × CONFIG × HOOK × TOOLS × SKILLS × CONTEXT × QUERY" 를 다룬다. 그 SPEC 은 5 레지스트리 + adapter + drain consumer 등록까지 13-step bootstrap 을 정의한다. 그러나 **dispatcher / ContextAdapter / LoopController / alias config 의 4 종 wiring 은 OUT OF SCOPE** 이며 후속 SPEC 에 위임된다 (DAEMON-WIRE-001 §3.2 항목 4 InteractiveHandler 본체, §3.2 항목 9 CLI 진입점 등).

본 SPEC 은 그 4 종 wiring 만 인수한다. ID 는 **`SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001`** 로 명확히 분리하여 DAEMON-WIRE-001 과의 작업 단위를 구분한다.

DAEMON-WIRE-001 의 SPEC 본문은 본 SPEC 작업 중 **변경하지 않는다**. 본 SPEC 의 implementation 단계에서 `cmd/goosed/main.go` 에 `wireSlashCommandSubsystem(rt, cfg, logger) (*command.Dispatcher, error)` 같은 helper function call site 를 step 10 직후에 1 줄 추가하는 것으로 통합된다 (DAEMON-WIRE-001 REQ-WIRE-009 InteractiveHandler placeholder 패턴과 동일).

### 2.2 의존 SPEC 자산 (research §1.3 참조)

| SPEC | 상태 | 본 SPEC 이 사용하는 surface |
|------|-----|---------------------------|
| SPEC-GOOSE-DAEMON-WIRE-001 | planned, P1 | 13-step bootstrap framework, EX_CONFIG fail-fast 의무, drain 등록 패턴 |
| SPEC-GOOSE-CMDCTX-001 | **implemented**, FROZEN, v0.1.1 | `adapter.New(Options{...}) *ContextAdapter`, `command.SlashCommandContext` 6 메서드, plan mode atomic flag 포인터 공유 |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | planned, P0 | `cmdctrl.LoopControllerImpl` 구현체 — `adapter.LoopController` 4 메서드 |
| SPEC-GOOSE-ALIAS-CONFIG-001 | planned, P2 | `aliasconfig.LoadDefault(opts)` graceful 부재 fallback |
| SPEC-GOOSE-COMMAND-001 | implemented, FROZEN | `command.NewDispatcher(sctx)`, `command.ErrUnknownModel`, `command.ErrUnknownCommand` |
| SPEC-GOOSE-ROUTER-001 | implemented, FROZEN | `router.DefaultRegistry()` |

### 2.3 single-session 가정 유지 (research §4 참조)

CMDCTX-001 §6 (FROZEN) 은 ContextAdapter 의 atomic flag 와 `*atomic.Pointer[command.ModelInfo]` 가 process 전역임을 가정한다. 즉 같은 daemon process 안에서 동시에 여러 session 이 다른 model / plan mode 를 가질 수 없다.

본 SPEC 은 이 가정을 **유지** 한다. multi-session 으로 확장하려면:

- ContextAdapter 의 atomic 자산을 `sync.Map[SessionID]` 로 교체 — CMDCTX-001 v0.2.0 amendment 또는 별도 SPEC 필요.
- RPC interceptor 가 ctx 에 SessionID 를 inject — TRANSPORT-001 후속.

이 확장은 가칭 `SPEC-GOOSE-CMDCTX-MULTI-SESSION-001` (별도 SPEC) 가 다룬다. 본 SPEC §10 Exclusions 에 명시.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `cmd/goosed/main.go` (또는 `cmd/goosed/wire.go` 헬퍼) 에 `wireSlashCommandSubsystem(rt *core.Runtime, cfg *config.Config, logger *zap.Logger) (*command.Dispatcher, *adapter.ContextAdapter, error)` 함수 신설. DAEMON-WIRE-001 의 13-step bootstrap 의 step 10 (`hookRegistry.SetSkillsFileChangedConsumer`) 직후, step 11 (`health.New().ListenAndServe`) 직전에 호출.

2. wiring 시퀀스 (4 sub-step):
   - 10.5: `aliasMap, err := aliasconfig.LoadDefault(aliasconfig.Options{Logger: logger, Registry: registry})` — graceful 부재.
   - 10.6: `loopCtrl := cmdctrl.New(rt, cmdctrl.Options{...})` — LoopController 구현체. nil 반환 시 EX_CONFIG.
   - 10.7: `ctxAdapter := adapter.New(adapter.Options{Registry: registry, LoopController: loopCtrl, AliasMap: aliasMap, Logger: loggerFacade})` — nil 반환 시 EX_CONFIG.
   - 10.8: `dispatcher := command.NewDispatcher(ctxAdapter)` — nil 반환 시 EX_CONFIG.

3. `core.Runtime.Drain` 에 LoopController.Drain 등록:
   - `rt.Drain.RegisterDrainConsumer(core.DrainConsumer{Name: "command.LoopController", Fn: func(ctx) error { return loopCtrl.Drain(ctx) }, Timeout: 5*time.Second})`.
   - 등록 순서는 tools.Registry.Drain (DAEMON-WIRE-001 step 9) 보다 먼저 호출되도록 결정 — research §5.4.

4. RPC handler 통합 (TRANSPORT-001 후속에 의존하지만 본 SPEC 이 인터페이스만 정의):
   - `ProcessUserInput(req)` 핸들러가 dispatcher 를 closure capture 또는 service struct field 로 주입받는다.
   - input 이 slash command 면 `dispatcher.Dispatch(ctx, input)` 호출. 결과 또는 error 를 RPC stream 으로 변환.
   - `defer recover()` 로 dispatcher panic 보호.
   - `plan_mode` metadata key 를 response 에 포함 (`ctxAdapter.PlanModeActive()` 결과).

5. nil/error 처리:
   - alias config 로드 실패: 빈 맵 fallback + warn log (graceful).
   - loopCtrl / ctxAdapter / dispatcher / registry nil 반환: EX_CONFIG fail-fast (DAEMON-WIRE-001 REQ-WIRE-008 일관성).
   - dispatcher panic: defer recover → RPC `codes.Internal`.

6. 통합 테스트 (`cmd/goosed/integration_test.go` 또는 등가):
   - 정상 부트스트랩 → dispatcher 인스턴스화 검증.
   - SIGTERM → LoopController.Drain 호출 + tools.Registry.Drain 보다 먼저 호출 검증.
   - alias config 로드 실패 → daemon 정상 부팅 + 빈 AliasMap 검증.
   - loopCtrl nil → EX_CONFIG (78) 검증.
   - dispatcher panic 시뮬레이션 → RPC `Internal` 매핑 검증.

### 3.2 OUT OF SCOPE (§10 Exclusions 에서 상세)

- DAEMON-WIRE-001 의 본문 변경 (FROZEN-등급 처리).
- multi-session multiplexing — `SPEC-GOOSE-CMDCTX-MULTI-SESSION-001` (별도 SPEC).
- CLI mode / `goose` CLI 바이너리 측 wiring — `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` (별도 SPEC).
- TRANSPORT-001 의 Connect-gRPC 채택 / 서비스 정의 / proto 파일 — TRANSPORT-001 SPEC 영역.
- LoopController / dispatcher / ContextAdapter / aliasconfig 의 본체 구현 — 각 의존 SPEC.
- alias hot-reload — `SPEC-GOOSE-HOTRELOAD-001`.
- per-RPC-call alias overlay 의 default 활성화 — Optional REQ 로만 노출, default off.
- RPC interceptor (auth, tracing, rate limit) 본체 — TRANSPORT-001 후속.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-DINTEG-001** — The `goosed` daemon **shall** instantiate exactly one `*adapter.ContextAdapter` instance during bootstrap (step 10.7) and **shall** share it across all RPC handler goroutines for the entire process lifetime.

**REQ-DINTEG-002** — The `goosed` daemon **shall** instantiate exactly one `*command.Dispatcher` instance during bootstrap (step 10.8) and **shall** inject it into the RPC handler (closure capture or service struct field) before transitioning to `StateServing`.

**REQ-DINTEG-003** — The wiring sequence **shall** execute in the following total order between DAEMON-WIRE-001 step 10 and step 11: (10.5) `aliasconfig.LoadDefault`, (10.6) `cmdctrl.New` (LoopController), (10.7) `adapter.New` (ContextAdapter), (10.8) `command.NewDispatcher`. Re-ordering is prohibited because each later sub-step requires the result of the earlier sub-step.

**REQ-DINTEG-004** — The `*adapter.ContextAdapter` instance **shall** be constructed with `Options.Registry = router.DefaultRegistry()`, `Options.LoopController = loopCtrl` (the cmdctrl instance from step 10.6), `Options.AliasMap = aliasMap` (the result from step 10.5; empty map if load failed), and `Options.Logger = loggerFacade` (a facade wrapping the daemon's `*zap.Logger` to the `adapter.Logger` interface).

**REQ-DINTEG-005** — The wiring SHALL maintain CMDCTX-001's single-session assumption: the `*ContextAdapter`'s atomic flag and active model pointer represent process-wide state. The daemon **shall not** introduce per-session adapter instances or per-session atomic flag splitting in this SPEC's scope.

### 4.2 Event-Driven (이벤트 기반)

**REQ-DINTEG-010** — **When** `aliasconfig.LoadDefault` returns an error or a nil map, the wiring **shall** substitute an empty `map[string]string{}` for `Options.AliasMap`, **shall** log the underlying error at WARN level via the daemon logger, and **shall** continue to step 10.6 without aborting the bootstrap. (Graceful degradation — alias is a UX convenience, daemon must still start.)

**REQ-DINTEG-011** — **When** the wired RPC handler receives a `ProcessUserInput` request whose `Input` field begins with `/` (slash command prefix as defined by COMMAND-001), the handler **shall** invoke `dispatcher.Dispatch(ctx, input)` exactly once with the request's ctx, **shall** stream the dispatcher's structured result to the RPC response stream, and **shall** propagate any returned error to the RPC stream via the error mapping table in §6.5.

**REQ-DINTEG-012** — **When** the wired daemon receives `SIGTERM` and `core.Runtime.Drain.RunAllDrainConsumers(ctx)` is invoked, the registered consumer `command.LoopController` **shall** be invoked exactly once, **shall** complete or its 5-second timeout **shall** elapse, and **shall** return before the `tools.Registry` drain consumer is invoked. (Order: LoopController → tools.Registry, because LoopController may issue tool calls during its drain.)

**REQ-DINTEG-013** — **When** the RPC handler streams a `ProcessUserInputResponse`, the response's `Metadata` map **shall** include the key-value pair `("plan_mode", "1")` if `ctxAdapter.PlanModeActive()` returned `true` at the time of streaming. **When** `PlanModeActive()` returned `false`, the key **shall** be absent from the metadata (not `"0"`, not present at all). This distinction allows CLI-001 to detect plan mode without parsing string values.

### 4.3 State-Driven (상태 기반)

**REQ-DINTEG-020** — **While** the daemon is in `StateServing`, the `*ContextAdapter`, `*command.Dispatcher`, and `LoopController` references held by the RPC handler **shall** all be non-nil. A nil reference at `StateServing` is a contract violation and **shall** be unreachable by construction (verified by the wire-up integration test).

**REQ-DINTEG-021** — **While** the daemon is transitioning from `StateServing` to `StateDraining`, new slash command dispatch requests **shall** be rejected: the RPC handler **shall** return `codes.Unavailable` for any `ProcessUserInput` request whose `Input` is a slash command and whose ctx is derived from a root ctx that is already canceled.

### 4.4 Unwanted Behavior (방지)

**REQ-DINTEG-030** — **If** any of `cmdctrl.New`, `adapter.New`, `command.NewDispatcher` returns a `nil` instance during bootstrap, **then** the wiring helper **shall** log an ERROR-level entry containing the failed constructor's name, **shall** return `core.ExitConfig` (78, EX_CONFIG) from `main.go::run()`, and the daemon **shall not** transition to `StateServing`. (Consistency with DAEMON-WIRE-001 REQ-WIRE-008.)

**REQ-DINTEG-031** — **If** the dispatcher's `Dispatch(ctx, input)` invocation panics during RPC request handling, **then** the RPC handler **shall** recover the panic via `defer recover()`, **shall** log an ERROR-level entry with the panic value (without leaking stack to the client), **shall** return RPC error code `codes.Internal` with a generic "internal error" message, and the daemon process **shall not** crash. The recovered panic **shall not** propagate to the daemon root goroutine.

**REQ-DINTEG-032** — **If** the RPC request's metadata contains `alias_overlay` but the daemon configuration has not opted into per-RPC-call alias overlay (REQ-DINTEG-040), **then** the handler **shall** ignore the metadata (no overlay applied) and **shall not** error. This prevents accidental privilege escalation when overlay is disabled by default.

**REQ-DINTEG-033** — **If** the RPC handler is invoked before bootstrap step 12 (`rt.State.Store(StateServing)`), **then** the handler **shall** return `codes.Unavailable` for every request without dereferencing dispatcher (which may still be nil). This guards against a race where health server (step 11) accepts traffic before wiring (step 10.5–10.8) completes — although REQ-DINTEG-003 mandates ordering, this REQ adds defense in depth.

### 4.5 Optional (선택적)

**REQ-DINTEG-040** — **Where** the daemon configuration enables per-RPC-call alias overlay (config flag `alias.allow_per_call_overlay: true`, default `false`), the RPC handler **may** parse the request's `alias_overlay` metadata field and **may** invoke `ctxAdapter.WithContext(ctx)` to produce a child adapter with the overlay applied for the duration of the single RPC call. The default mode **shall** be disabled (REQ-DINTEG-032) for security baseline.

**REQ-DINTEG-041** — **Where** TRANSPORT-001 (future SPEC) provides RPC interceptors for auth / tracing / rate limit, the wiring helper **shall** expose a stable injection point (e.g., `wireSlashCommandSubsystem(..., interceptors []grpc.UnaryServerInterceptor)`) such that interceptor registration does not require modifying steps 10.5–10.8. In the present SPEC scope, this injection point **may** be empty (no interceptors).

---

## 5. Acceptance Criteria

### 5.1 REQ → AC Traceability

| REQ | AC (primary) | AC (supplementary) | 비고 |
|-----|-------------|--------------------|------|
| REQ-DINTEG-001 (단일 인스턴스) | AC-DINTEG-001 | — | 부트스트랩 정상 경로 |
| REQ-DINTEG-002 (dispatcher 단일 인스턴스 + RPC 주입) | AC-DINTEG-001 | AC-DINTEG-002 | 정상 dispatch 합산 |
| REQ-DINTEG-003 (4 sub-step 순서) | AC-DINTEG-001 | — | 부트스트랩 로그 검증 |
| REQ-DINTEG-004 (Options 4 필드 주입) | AC-DINTEG-001 | — | 인스턴스 reflect 검증 |
| REQ-DINTEG-005 (single-session 가정 유지) | AC-DINTEG-006 | — | adapter atomic flag 공유 검증 |
| REQ-DINTEG-010 (alias config 부재 graceful) | AC-DINTEG-003 | — | 빈 맵 fallback |
| REQ-DINTEG-011 (RPC dispatch 정상) | AC-DINTEG-002 | — | dispatcher 호출 검증 |
| REQ-DINTEG-012 (drain 순서) | AC-DINTEG-004 | — | LoopController → tools |
| REQ-DINTEG-013 (plan_mode metadata) | AC-DINTEG-005 | — | 양/음 케이스 |
| REQ-DINTEG-020 (StateServing non-nil) | AC-DINTEG-001 | — | 정상 부팅 |
| REQ-DINTEG-021 (StateDraining 거부) | AC-DINTEG-004 | — | 종료 흐름 |
| REQ-DINTEG-030 (nil 반환 EX_CONFIG) | AC-DINTEG-007 | — | fail-fast |
| REQ-DINTEG-031 (panic recovery) | AC-DINTEG-008 | — | RPC Internal |
| REQ-DINTEG-032 (overlay 기본 off) | AC-DINTEG-009 | — | metadata 무시 |
| REQ-DINTEG-033 (StateServing 전 거부) | AC-DINTEG-010 | — | race 보호 |
| REQ-DINTEG-040 (overlay opt-in) | AC-DINTEG-009 | — | 동일 케이스의 ON 분기 |
| REQ-DINTEG-041 (interceptor 주입점) | AC-DINTEG-011 | — | placeholder |

### 5.2 Acceptance Criteria

#### AC-DINTEG-001 — 정상 부트스트랩 (4 sub-step + dispatcher 인스턴스화) _(covers REQ-DINTEG-001~004, 020)_

**[Event-Driven]** **When** `goosed` is launched with a valid `~/.goose/config.yaml`, an empty `~/.goose/skills/`, an unbound health port, and an absent `~/.goose/aliases.yaml`, the daemon **shall** complete bootstrap step 10.5–10.8 in declared order, **shall** instantiate `*ContextAdapter` and `*Dispatcher` exactly once each, **shall** transition to `StateServing` within 500 ms wall-clock, and **shall** report all of `ctxAdapter != nil`, `dispatcher != nil`, `loopCtrl != nil` via test harness reflection.

**Test Scenario (verification)**:
- **Given** 임시 `MINK_HOME=<tmpdir>` 에 `config.yaml` (default), 빈 `skills/`, alias 파일 없음, 자유 포트 확보
- **When** `goosed` 를 in-process `run()` 호출, 500 ms 대기
- **Then** (a) zap 로그에서 다음 INFO 라인이 declared 순서대로 출력: `"alias config loaded"` (또는 `"alias config absent, using empty map"`) → `"loop controller initialized"` → `"context adapter initialized"` → `"dispatcher initialized"`, (b) reflect 로 wiring helper 의 반환값 inspection 시 `*ContextAdapter`, `*Dispatcher`, `LoopController` 모두 non-nil, (c) `rt.State.Load() == StateServing`, (d) `dispatcher.Commands()` (existing API in COMMAND-001) 가 4 개 빌트인 (`/clear`, `/compact`, `/model`, `/status`) 을 반환.

---

#### AC-DINTEG-002 — RPC ProcessUserInput → dispatcher 정상 dispatch _(covers REQ-DINTEG-002, 011)_

**[Event-Driven]** **When** the wired RPC handler receives `ProcessUserInput{Input: "/status"}` after bootstrap, the handler **shall** invoke `dispatcher.Dispatch(ctx, "/status")` exactly once with the request's ctx, **shall** stream a structured `command_result` response containing the SessionSnapshot, and **shall** complete within 100 ms.

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup + test harness 가 fake RPC client 를 통해 daemon 의 RPC handler 호출 (in-process or local UDS)
- **When** `client.ProcessUserInput(&Request{Input: "/status"})`
- **Then** (a) response stream 의 첫 메시지가 `Type == "command_result"`, (b) payload 가 SessionSnapshot 직렬화 (TurnCount / Model / CWD 필드 포함), (c) dispatcher.Dispatch 호출 카운터 == 1 (test fake 로 관측), (d) response context 의 cancel 이 dispatcher 의 ctx 로 전파됨 (race detector clean).

---

#### AC-DINTEG-003 — alias config 부재 → 빈 맵 graceful fallback _(covers REQ-DINTEG-010)_

**[Event-Driven]** **When** `aliasconfig.LoadDefault` returns `(nil, ErrAliasFileNotFound)` (or equivalent absent-file error) during step 10.5, the wiring **shall** substitute an empty map for `Options.AliasMap`, **shall** log a WARN-level entry containing `"alias config absent"` and the attempted path, and **shall** complete the remaining bootstrap steps without error. After bootstrap, `ctxAdapter.ResolveModelAlias("opus")` **shall** return `(nil, command.ErrUnknownModel)` (no alias known) but `ctxAdapter.ResolveModelAlias("anthropic/claude-opus-4-7")` **shall** succeed via `ProviderRegistry.SuggestedModels` lookup.

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup, `~/.goose/aliases.yaml` 부재
- **When** in-process `run()` 호출
- **Then** (a) zap 로그에 WARN `"alias config absent"` 1 건, (b) `rt.State.Load() == StateServing`, (c) test harness 가 `ctxAdapter.ResolveModelAlias("opus")` 호출 시 `(nil, command.ErrUnknownModel)`, (d) `ctxAdapter.ResolveModelAlias("anthropic/claude-opus-4-7")` 호출 시 `(*ModelInfo, nil)` (canonical lookup 성공), (e) `Options.AliasMap` reflect 검증 시 empty map (not nil — 빈 맵).

---

#### AC-DINTEG-004 — SIGTERM → LoopController.Drain → tools.Registry.Drain 순서 검증 _(covers REQ-DINTEG-012, 021)_

**[Event-Driven]** **When** the running daemon receives `SIGTERM`, the daemon **shall** transition to `StateDraining`, **shall** invoke the registered `command.LoopController` drain consumer before `tools.Registry` drain consumer, **shall** complete both within their respective timeouts (5 s and 10 s), and **shall** subsequently reject new slash command RPC requests with `codes.Unavailable`.

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup, fake LoopControllerImpl 이 drain 시작 / 종료를 timestamp 로 기록, fake tools.Registry 동일
- **When** `syscall.Kill(daemonPID, syscall.SIGTERM)` (또는 in-process `cancel()`)
- **Then** (a) zap 로그의 `"drain consumer completed" consumer="command.LoopController"` timestamp 가 `consumer="tools.Registry"` timestamp 보다 먼저, (b) 두 timestamp 간 간격 ≥ 0 (즉 LoopController 가 끝난 후 tools 시작), (c) drain 중 도착한 RPC 요청은 `codes.Unavailable` 반환 (test 가 SIGTERM 후 1 ms 내에 RPC 호출), (d) daemon exit code == 0.

---

#### AC-DINTEG-005 — plan_mode metadata 양/음 케이스 _(covers REQ-DINTEG-013)_

**[Event-Driven]** **When** the RPC handler streams a `ProcessUserInputResponse` for any input, the response's `Metadata` map **shall** include `"plan_mode": "1"` if and only if `ctxAdapter.PlanModeActive()` returned `true` at streaming time. **When** `PlanModeActive()` returned `false`, the key `"plan_mode"` **shall** be absent from `Metadata` (not present with value `"0"`).

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup
- **When-A** plan mode flag 를 atomic 으로 set (`ctxAdapter.SetPlanMode(true)` — CMDCTX-001 의 internal API 사용 또는 `WithContext` 헬퍼) → RPC `ProcessUserInput{Input: "hello"}` 호출
- **Then-A** response Metadata `["plan_mode"] == "1"`, key 존재
- **When-B** plan mode flag clear → RPC 동일 호출
- **Then-B** response Metadata 에 `"plan_mode"` key **부재** (test: `_, ok := metadata["plan_mode"]; assert ok == false`)

---

#### AC-DINTEG-006 — single-session atomic flag 공유 검증 _(covers REQ-DINTEG-005)_

**[Ubiquitous]** Two RPC handler goroutines that both consult `ctxAdapter.PlanModeActive()` after one of them has set the flag **shall** observe the same `true` value, demonstrating the process-wide single-session model. The test **shall** confirm that no per-session adapter cloning occurs.

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup, test harness 가 2 개의 RPC client (다른 fake SessionID 를 metadata 에 포함) 를 동시 호출
- **When** Client A 가 plan mode 를 set (예: future `/plan on` slash command 또는 internal API), Client B 가 별도 RPC 로 SessionSnapshot 요청
- **Then** (a) Client B 의 response metadata 에 `"plan_mode": "1"` 포함 (즉 단일 atomic flag 가 공유됨), (b) reflect 로 `*ContextAdapter` 의 atomic flag 의 메모리 주소가 두 호출에서 동일.

---

#### AC-DINTEG-007 — nil 반환 → EX_CONFIG fail-fast _(covers REQ-DINTEG-030)_

**[Unwanted]** **If** `cmdctrl.New(...)` returns nil (test harness 가 fake constructor 로 강제), **then** the wiring helper **shall** log ERROR `"loop controller construction failed"`, **shall** return `core.ExitConfig` (78), the daemon **shall not** transition to `StateServing`, and the health port **shall not** be opened.

**Test Scenario (verification)**:
- **Given** test harness 가 build-time injection 으로 `cmdctrl.New` 를 nil-return stub 으로 교체 (또는 wiring helper 가 `func(rt) LoopController` 를 받도록 testable 하게 설계됨)
- **When** in-process `run()` 호출
- **Then** (a) `run()` 반환값 == `core.ExitConfig` (78), (b) zap 로그 ERROR 1 건 (`"loop controller construction failed"` 또는 동등), (c) `rt.State.Load() != StateServing`, (d) health port (test 가 미리 `net.Listen` 으로 free 확인한 포트) 가 여전히 free.

추가 변형 (별도 sub-test):
- `adapter.New(...)` 가 nil 반환 — 동일 EX_CONFIG.
- `command.NewDispatcher(...)` 가 nil 반환 — 동일 EX_CONFIG.
- `router.DefaultRegistry()` 가 nil 반환 — 동일 EX_CONFIG.

---

#### AC-DINTEG-008 — dispatcher panic → RPC Internal _(covers REQ-DINTEG-031)_

**[Unwanted]** **If** the dispatcher's `Dispatch(ctx, input)` invocation panics during an RPC request, **then** the RPC handler **shall** recover the panic, **shall** log ERROR with the panic value, **shall** return `codes.Internal` to the client, and the daemon **shall not** crash. Subsequent RPC requests **shall** continue to be served normally.

**Test Scenario (verification)**:
- **Given** AC-DINTEG-001 setup, test harness 가 fake dispatcher 를 wiring helper 의 testable injection point 로 주입 (panic 을 발생시키는 fake)
- **When** RPC `ProcessUserInput{Input: "/panic-test"}` (fake dispatcher 가 panic)
- **Then** (a) RPC client 가 받는 error code == `codes.Internal`, (b) error message 가 generic ("internal error" 류 — stack trace 또는 panic value 가 client 에 노출되지 않음), (c) zap 로그 ERROR 에 `"dispatcher panic"` + panic value 포함, (d) test 가 후속 `ProcessUserInput{Input: "/status"}` 호출 시 정상 응답 (daemon 이 살아 있음), (e) `runtime.NumGoroutine()` 이 panic 전후로 leak 없음.

---

#### AC-DINTEG-009 — alias overlay 기본 off / opt-in on _(covers REQ-DINTEG-032, 040)_

**[Unwanted/Optional]** **If** the daemon config does not enable per-RPC-call alias overlay (default), **then** the RPC handler **shall** ignore any `alias_overlay` metadata field and **shall not** apply the overlay. **Where** the config enables overlay, the handler **shall** apply the overlay for the single RPC call duration without mutating the shared `*ContextAdapter`'s AliasMap.

**Test Scenario (verification)**:
- **Given-A** AC-DINTEG-001 setup (default config, overlay off)
- **When-A** RPC `ProcessUserInput{Input: "/model opus", Metadata: {"alias_overlay": "opus=fake-provider/fake-model"}}`
- **Then-A** dispatcher 가 `alias_overlay` 를 무시하고 default AliasMap 으로 `opus` 해석 (alias 부재 시 `command.ErrUnknownModel`, alias 존재 시 default canonical 반환)
- **Given-B** config `alias.allow_per_call_overlay: true`
- **When-B** 동일 RPC 호출
- **Then-B** overlay 적용되어 `opus` → `fake-provider/fake-model` 매핑이 **이번 RPC 한정** 으로만 동작, (c) RPC 종료 후 별도 RPC 가 default AliasMap 으로 동작 (shared adapter 의 AliasMap 이 mutate 안 됨, reflect 로 검증).

---

#### AC-DINTEG-010 — StateServing 진입 전 RPC → Unavailable _(covers REQ-DINTEG-033)_

**[Unwanted]** **If** the RPC handler is invoked before bootstrap step 12 (`rt.State.Store(StateServing)`), **then** the handler **shall** return `codes.Unavailable` for every request without dereferencing dispatcher.

**Test Scenario (verification)**:
- **Given** test harness 가 step 11 (health.New().ListenAndServe) 직후 step 12 직전에 RPC 요청을 보낼 수 있도록 channel-based readiness signal 사용 (실제로는 race window 가 매우 좁아 단위 테스트로 강제 시뮬레이션 필요)
- **When** RPC `ProcessUserInput{Input: "/status"}` in pre-serving 상태
- **Then** (a) 반환 error code == `codes.Unavailable`, (b) dispatcher.Dispatch 호출 카운터 == 0 (handler 가 dispatcher dereference 없이 early return), (c) panic 없음.

---

#### AC-DINTEG-011 — TRANSPORT-001 interceptor placeholder _(covers REQ-DINTEG-041)_

**[Optional]** **Where** TRANSPORT-001 future SPEC provides interceptors, the wiring helper **shall** expose `wireSlashCommandSubsystem(rt, cfg, logger, interceptors []grpc.UnaryServerInterceptor)` (or equivalent) such that interceptor injection does not modify steps 10.5–10.8. In the present SPEC scope, the call site **may** pass `nil` or empty slice as a no-op.

**Test Scenario (verification)**:
- **Given** main.go 가 `wireSlashCommandSubsystem(rt, cfg, logger, nil)` 호출 (placeholder)
- **When** AC-DINTEG-001 setup 으로 부트스트랩
- **Then** (a) daemon 정상 `StateServing` 진입, (b) 별도 interceptor 없음, (c) 미래 TRANSPORT-001 SPEC 가 `[]grpc.UnaryServerInterceptor{authInterceptor, traceInterceptor}` 를 넘기는 형태로 확장 가능 (compile-time signature 호환), (d) negative test 는 TRANSPORT-001 영역으로 위임.

---

## 6. 데이터 모델 / API 설계

### 6.1 wiring helper 시그니처

```go
// cmd/goosed/wire.go (또는 main.go 동일 패키지)
//
// wireSlashCommandSubsystem instantiates the slash command subsystem
// (alias loader, LoopController, ContextAdapter, Dispatcher) and
// registers the LoopController.Drain consumer with the runtime.
// Returns the dispatcher (for RPC handler injection) and the adapter
// (for plan_mode metadata extraction during streaming).
func wireSlashCommandSubsystem(
    rt *core.Runtime,
    cfg *config.Config,
    logger *zap.Logger,
) (*command.Dispatcher, *adapter.ContextAdapter, error)
```

return 값 처리 정책:

- error != nil → main.go::run() 가 `core.ExitConfig` (78) 반환.
- (dispatcher, adapter, nil) → main.go 가 RPC handler service struct 에 양자 주입.

### 6.2 step 10.5: alias config 로드

```go
aliasMap, err := aliasconfig.LoadDefault(aliasconfig.Options{
    Logger:   logger,
    Registry: registry,
    // strict 모드는 aliasconfig.Options 내부의 별도 필드로 결정 (ALIAS-CONFIG-001)
})
if err != nil {
    logger.Warn("alias config absent or invalid, using empty map",
        zap.Error(err))
    aliasMap = map[string]string{}
}
```

### 6.3 step 10.6: LoopController instantiate

```go
loopCtrl := cmdctrl.New(rt, cmdctrl.Options{
    Logger: logger,
    // engine handle 등 CMDLOOP-WIRE-001 의 추가 필드 — 본 SPEC 은 시그니처
    // 만 가정하고 정확한 필드 셋은 CMDLOOP-WIRE-001 의 v0.1.0 spec.md 가
    // 결정한다. cmdctrl.New 가 nil 반환 시 본 SPEC 은 EX_CONFIG fail-fast.
})
if loopCtrl == nil {
    logger.Error("loop controller construction failed")
    return nil, nil, errLoopCtrlNil
}

rt.Drain.RegisterDrainConsumer(core.DrainConsumer{
    Name:    "command.LoopController",
    Fn:      func(ctx context.Context) error { return loopCtrl.Drain(ctx) },
    Timeout: 5 * time.Second,
})
```

### 6.4 step 10.7: ContextAdapter instantiate

```go
ctxAdapter := adapter.New(adapter.Options{
    Registry:       router.DefaultRegistry(),
    LoopController: loopCtrl,
    AliasMap:       aliasMap,
    Logger:         zapLoggerToAdapterLogger(logger),
    // GetwdFn: nil → adapter 가 os.Getwd 사용 (default)
})
if ctxAdapter == nil {
    logger.Error("context adapter construction failed")
    return nil, nil, errAdapterNil
}
```

### 6.5 step 10.8: Dispatcher + RPC error 매핑

```go
dispatcher := command.NewDispatcher(ctxAdapter)
if dispatcher == nil {
    logger.Error("dispatcher construction failed")
    return nil, nil, errDispatcherNil
}

// RPC handler (TRANSPORT-001 후속이지만 본 SPEC 이 매핑 정책 정의)
func (s *AgentService) ProcessUserInput(req *Request, stream Stream) error {
    defer func() {
        if r := recover(); r != nil {
            s.Logger.Error("dispatcher panic", zap.Any("panic", r))
            // codes.Internal 로 매핑 (test 가 검증)
        }
    }()
    if s.RT.State.Load() != core.StateServing {
        return codes.Unavailable.Err()  // REQ-DINTEG-033
    }
    if !strings.HasPrefix(req.Input, "/") {
        // 일반 input → query loop 진입 (본 SPEC 범위 외)
        return nil
    }
    result, err := s.Dispatcher.Dispatch(stream.Context(), req.Input)
    if err != nil {
        return mapDispatchErrorToRPC(err)
    }
    // result → stream + plan_mode metadata
    md := map[string]string{}
    if s.Adapter.PlanModeActive() {
        md["plan_mode"] = "1"  // REQ-DINTEG-013
    }
    return stream.Send(&Response{Type: "command_result", Payload: serialize(result), Metadata: md})
}

// RPC error 매핑 표 (REQ-DINTEG-031, AC-DINTEG-002)
func mapDispatchErrorToRPC(err error) error {
    switch {
    case errors.Is(err, command.ErrUnknownCommand):  return codes.InvalidArgument
    case errors.Is(err, command.ErrUnknownModel):    return codes.InvalidArgument
    case errors.Is(err, adapter.ErrLoopControllerUnavailable): return codes.Unavailable
    case errors.Is(err, context.Canceled):           return codes.Canceled
    case errors.Is(err, context.DeadlineExceeded):   return codes.DeadlineExceeded
    default:                                          return codes.Internal
    }
}
```

위 코드 sketch 는 RPC framework 가 Connect-gRPC 라고 가정한다. TRANSPORT-001 가 다른 framework 를 채택하면 error 매핑 표만 minor update.

---

## 7. 의존성 (Dependencies)

### 7.1 선행 SPEC

| SPEC | 상태 | 본 SPEC 이 사용하는 surface | 변경 가능성 |
|------|-----|-------------------------|-----------|
| SPEC-GOOSE-DAEMON-WIRE-001 | planned, P1 | 13-step bootstrap framework | **변경 안 함** (FROZEN-등급 처리, implementation 시점에 1 줄 helper call site 추가만) |
| SPEC-GOOSE-CMDCTX-001 | implemented, FROZEN | `adapter.New(Options{...}) *ContextAdapter` | **변경 안 함** |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | planned, P0 | `cmdctrl.New(rt, Options) LoopController` | 본 SPEC 의 plan 단계에서는 미구현 가능. run 진입 전 implementation 필요 |
| SPEC-GOOSE-ALIAS-CONFIG-001 | planned, P2 | `aliasconfig.LoadDefault(Options) (map, error)` | 본 SPEC 의 run 진입 시점에 부재해도 graceful (REQ-DINTEG-010) — implementation 권장이지만 hard block 아님 |
| SPEC-GOOSE-COMMAND-001 | implemented, FROZEN | `command.NewDispatcher`, `command.ErrUnknownModel/Command` | **변경 안 함** |
| SPEC-GOOSE-ROUTER-001 | implemented, FROZEN | `router.DefaultRegistry()` | **변경 안 함** |

### 7.2 후속 SPEC (본 SPEC 이 hook point 또는 placeholder 노출)

| SPEC | 본 SPEC 이 노출 | 비고 |
|------|---------------|------|
| SPEC-GOOSE-CMDCTX-MULTI-SESSION-001 | (가칭) | atomic flag 를 session 별로 분리. 본 SPEC 은 single-session 가정 유지 |
| SPEC-GOOSE-CMDCTX-CLI-INTEG-001 | (가칭) | CLI 진입점에서 ContextAdapter 와 dispatcher 주입 (daemon 과 동일 패턴) |
| SPEC-GOOSE-TRANSPORT-001 | RPC framework 도입 + interceptor 주입점 | REQ-DINTEG-041 placeholder |
| SPEC-GOOSE-HOTRELOAD-001 | alias hot-reload | 본 SPEC 은 load-once 만 |
| SPEC-GOOSE-CREDPOOL-001 후속 | OAuth refresh / credential pool swap | OnModelChange 후속 효과 |

### 7.3 외부 의존성

본 SPEC 은 신규 외부 라이브러리를 도입하지 않는다. RPC framework 가 Connect-gRPC 이면 `connectrpc.com/connect`, gRPC stdlib 이면 `google.golang.org/grpc`. 이는 TRANSPORT-001 가 결정.

---

## 8. 비기능 / 정합성

| 항목 | 기준 |
|------|------|
| race detector | `go test -race -count=10` clean (concurrent RPC handler test) |
| coverage | wiring helper + RPC handler glue ≥ 90% |
| EX_CONFIG fail-fast | `core.ExitConfig` (78) 반환, `StateServing` 미진입 (DAEMON-WIRE-001 일관성) |
| graceful shutdown | LoopController.Drain → tools.Registry.Drain 순서 보장, daemon exit 5 s 이내 |
| daemon 부팅 시간 | wiring helper 본체 ≤ 100 ms (alias load + 4 sub-step 합산) |
| log 정책 | bootstrap 4 sub-step 각각 INFO 1 건, 실패 시 ERROR 1 건. plan_mode 변경 / dispatcher dispatch 자체는 DEBUG 권장 |

---

## 9. Risks (위험)

| ID | 리스크 | 영향 | 완화 |
|----|------|-----|------|
| R-1 | DAEMON-WIRE-001 implementation 의존 — 본 SPEC plan 작성은 가능하나 run 진입은 DAEMON-WIRE-001 의 wiring helper hook point 가 main.go 에 만들어진 이후에야 가능 | High | 본 SPEC §3.2 에서 명시. DAEMON-WIRE-001 implementation 시점에 §6.1 의 helper signature 반영 (1 줄 call site). 본 SPEC 의 run phase task 분해는 DAEMON-WIRE-001 의 implementation status 를 매 iteration 진입 전 확인 |
| R-2 | RPC framework 미정 — TRANSPORT-001 가 Connect-gRPC 가 아닌 다른 framework 채택 시 §6.5 의 error 매핑 표 minor update 필요 | Medium | 본 SPEC §6.5 의 매핑 코드는 sketch. AC-DINTEG-002/008/010 의 verification 은 framework-agnostic 한 codes.* 로만 표현. TRANSPORT-001 결정 시 본 SPEC v0.1.1 minor amendment |
| R-3 | LoopController.Drain 시그니처 미정 — CMDLOOP-WIRE-001 v0.1.0 SPEC 본문에 Drain 메서드 정의가 명시되지 않음 (interface 4 메서드는 RequestClear / RequestReactiveCompact / RequestModelChange / Snapshot) | Medium | 본 SPEC 의 §4 REQ-DINTEG-012 는 Drain 메서드를 가정한다. CMDLOOP-WIRE-001 의 v0.1.0 SPEC 에 Drain 이 부재하면 본 SPEC 은 graceful no-op fallback (drain consumer 가 즉시 nil 반환) 으로 후퇴하고, CMDLOOP-WIRE-001 v0.2.0 amendment 에서 Drain 추가. AC-DINTEG-004 의 verification 은 CMDLOOP-WIRE-001 의 최종 시그니처에 맞춰 implementation 시점 결정 |
| R-4 | single-session 가정의 미래 부담 — multi-tenant daemon 시나리오에서 CMDCTX-001 의 atomic flag 가 cross-session 누수를 초래 | Medium | §10 #1 Exclusion 으로 명시. MULTI-SESSION-001 별도 SPEC 가 ContextAdapter 의 atomic 자산을 session map 으로 교체하는 amendment 를 제안. 본 SPEC 은 single-session 임을 사용자 / CLI 클라이언트 가 인지하도록 daemon `/healthz` response body 또는 startup banner 에 명시 (옵션) |
| R-5 | drain 순서 race — CORE-001 의 DrainCoordinator 가 LIFO 인지 FIFO 인지 SPEC 본문에 명시 부족 | Low | 본 SPEC §6.3 implementation 시점에 `internal/core/drain.go` 의 호출 순서 의미론을 read-back. LIFO 라면 등록 순서를 inversion (LoopController 등록을 tools 등록 앞에 이동), FIFO 라면 그대로. AC-DINTEG-004 가 timestamp 비교로 회귀 차단 |
| R-6 | dispatcher panic recovery 의 stack leak — `defer recover` 가 panic value 를 log 에 출력 시 secret-bearing payload 가 노출 | Low | REQ-DINTEG-031 의 "without leaking stack to the client" 명시. logger 출력은 server-side 만, RPC error message 는 generic. AC-DINTEG-008 의 verification 이 client error message 의 secret 부재를 검증 |
| R-7 | per-RPC-call alias overlay 의 보안 — 외부 client 가 임의 alias 를 inject 하여 model swap 권한 우회 | Low | REQ-DINTEG-040 default off + `alias.allow_per_call_overlay: true` 명시 opt-in. AC-DINTEG-009 가 default off 동작 검증. 보안 검토는 SECURITY-001 또는 별도 SPEC |
| R-8 | wiring helper 의 testable injection point 부재 — fake constructor 주입 어려움 | Low | §6.1 helper 시그니처가 4 가지 의존성을 외부에서 주입 가능한 형태로 설계. test 시점에는 helper 의 internal `cmdctrl.New` / `adapter.New` / `command.NewDispatcher` 호출을 build tag 또는 interface 추상화로 wrap. AC-DINTEG-007 verification 이 stub 주입 패턴 활용 |

---

## 10. Exclusions (What NOT to Build)

본 SPEC 이 **명시적으로 다루지 않는** 항목:

1. **Multi-session multiplexing** — `*ContextAdapter` 의 atomic flag 를 session 별로 분리하는 작업은 가칭 `SPEC-GOOSE-CMDCTX-MULTI-SESSION-001` 이 인수. 본 SPEC 은 single-session 가정 유지 (REQ-DINTEG-005).
2. **CLI mode wiring** — `goose` CLI 바이너리 측에서 ContextAdapter / dispatcher 를 instantiate 하는 작업은 가칭 `SPEC-GOOSE-CMDCTX-CLI-INTEG-001` 이 인수. 본 SPEC 은 daemon (`goosed`) 측만.
3. **TRANSPORT-001 의 RPC framework 결정** — Connect-gRPC / vanilla gRPC / HTTP+JSON 등 framework 채택은 SPEC-GOOSE-TRANSPORT-001 영역. 본 SPEC 은 RPC handler 에서 dispatcher 를 호출하는 추상 패턴만 정의.
4. **DAEMON-WIRE-001 본문 변경** — DAEMON-WIRE-001 v0.1.0 의 13-step 정본은 변경 금지. 본 SPEC 의 implementation 은 DAEMON-WIRE-001 implementation 시점에 main.go 에 1 줄 helper call site 를 추가하는 형태로 통합.
5. **alias hot-reload** — 파일 watch 기반 in-process map 교체는 `SPEC-GOOSE-HOTRELOAD-001`. 본 SPEC 은 부트스트랩 1 회 load only.
6. **LoopController / ContextAdapter / dispatcher / aliasconfig 의 본체 구현** — 각 의존 SPEC (CMDLOOP-WIRE-001 / CMDCTX-001 / COMMAND-001 / ALIAS-CONFIG-001) 영역. 본 SPEC 은 wiring 만.
7. **OnModelChange 후 OAuth refresh / credential pool swap** — CREDPOOL-001 후속 wiring. 본 SPEC 은 dispatcher → LoopController.RequestModelChange 호출까지만.
8. **plan mode top-level setter** — `/plan on` 또는 `/plan off` 같은 명시적 slash command 는 COMMAND-001 후속 SPEC. 본 SPEC 은 PlanModeActive 의 RPC metadata 노출만.
9. **RPC interceptor 본체 (auth, tracing, rate limit)** — TRANSPORT-001 후속. 본 SPEC 은 REQ-DINTEG-041 의 injection point placeholder 만.
10. **session ID 의 ctx 주입 메커니즘** — multi-session 가정 부재로 본 SPEC 에서는 불필요. TRANSPORT-001 + MULTI-SESSION-001 합작.
11. **CONFIG-001 의 `Config` struct 에 alias 또는 dispatcher 필드 추가** — 본 SPEC 은 CONFIG-001 본문을 변경하지 않는다. 필요 시 ALIAS-CONFIG-001 또는 별도 minor amendment.
12. **dispatcher result 의 RPC stream serialization 형식** — `command_result` payload 의 binary 형식 (protobuf / JSON / msgpack) 은 TRANSPORT-001 결정. 본 SPEC §6.5 의 `serialize(result)` 는 sketch.

---

## 11. 참고 (References)

### 11.1 SPEC 문서

- `.moai/specs/SPEC-GOOSE-DAEMON-WIRE-001/spec.md` — 13-step bootstrap, EX_CONFIG fail-fast 정책, drain 등록 패턴
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` v0.1.1 — `*ContextAdapter`, `Options`, `command.SlashCommandContext` 6 메서드 (FROZEN)
- `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` v0.1.0 — LoopController 구현체 (planned)
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` v0.1.0 — aliasconfig.LoadDefault, graceful 부재 (planned)
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — dispatcher, slash command 빌트인 (implemented)

### 11.2 코드 레퍼런스 (read-only)

- `cmd/goosed/main.go` — 현재 9-step (DAEMON-WIRE-001 baseline) flow
- `internal/command/adapter/adapter.go` — `*ContextAdapter`, `Options` (CMDCTX-001 implemented)
- `internal/command/adapter/controller.go` — `LoopController` interface (FROZEN)
- `internal/command/dispatcher.go` — `command.NewDispatcher`, `Dispatch` (COMMAND-001 implemented)
- `internal/core/runtime.go` — `*Runtime`, `Drain.RegisterDrainConsumer`
- `internal/core/drain.go` — `DrainCoordinator` 호출 순서 의미론 (LIFO/FIFO 미정)
- `internal/llm/router/registry.go` — `DefaultRegistry()`

### 11.3 Research

- `.moai/specs/SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001/research.md` — 본 SPEC 의 사전 분석 (10 결정 D-1 ~ D-10)

---

Version: 0.1.0
Last Updated: 2026-05-04
Status: completed

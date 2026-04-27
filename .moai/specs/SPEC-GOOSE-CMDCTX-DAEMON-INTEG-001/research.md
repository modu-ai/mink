# SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 — Research

> Read-only 분석. 본 research 는 daemon 진입점(`cmd/goosed/main.go`)에서
> `internal/command/adapter` 패키지의 `ContextAdapter` 와 그 의존성
> (`LoopController` 구현체, `aliasconfig.Loader`, dispatcher) 을 어떻게
> instantiate 하고 RPC handler 에 주입할 것인지를 결정하기 위한 사전 조사.
> 새 코드를 도입하지 않으며 의존 SPEC 의 본문에도 손대지 않는다.

---

## 0. Research scope (본 research 가 답해야 하는 질문)

| ID | 질문 | 본 research 가 다루는 §|
|----|-----|--------|
| Q-1 | DAEMON-WIRE-001 의 13-step bootstrap 안에서 ContextAdapter wiring 은 어느 단계 직후에 들어가야 하는가? | §1, §2 |
| Q-2 | RPC handler (Connect-gRPC 가정) 가 dispatcher 를 어떤 시점에 호출하는가? ctx 전파 경로는? | §3 |
| Q-3 | daemon 은 multi-session 인가 single-session 인가? CMDCTX-001 의 single-session 가정은 daemon 에서 유지 가능한가? | §4 |
| Q-4 | graceful shutdown 시 LoopController drain 전략. SIGTERM → in-flight RPC → tools.Drain → loop drain 순서는? | §5 |
| Q-5 | alias config (ALIAS-CONFIG-001) 로드 실패 시 daemon 부팅이 EX_CONFIG fail-fast 해야 하는가? | §6 |
| Q-6 | per-RPC-call alias overlay 는 가능한가? request 헤더 기반 dynamic injection 의 위험은? | §7 |
| Q-7 | dispatcher panic 시 RPC error 매핑 정책은? | §8 |
| Q-8 | ContextAdapter 의 wiring 실패가 EX_CONFIG fail-fast 의무를 트리거해야 하는가? | §9 |

---

## 1. DAEMON-WIRE-001 13-step 흐름 안에서 ContextAdapter 의 위치

### 1.1 DAEMON-WIRE-001 §6.1 의 13-step 정본 (FROZEN, 본 SPEC 변경 금지)

```
1. config.Load(LoadOptions{})                         (CONFIG-001)
2. core.NewLogger(cfg.Log.Level, "goosed", version)   (CORE-001)
3. signal.NotifyContext(SIGINT, SIGTERM)              (CORE-001)
4. core.NewRuntime(logger, rootCtx)                    (CORE-001)
5. hook.NewHookRegistry(logger)                        (HOOK-001)
6. tools.NewRegistry(logger)                           (TOOLS-001)
7. skill.LoadSkillsDir(cfg.SkillsRoot, logger)         (SKILLS-001)
8. WorkspaceRootResolverAdapter 등록                   (NEW: adapter)
9. tools.Registry.Drain → core.Drain 등록             (NEW)
10. skills.FileChanged → hook 등록                     (NEW)
11. health.New().ListenAndServe                        (CORE-001)
12. rt.State.Store(StateServing)                       (CORE-001)
13. <-rootCtx.Done()                                   (CORE-001)
```

### 1.2 본 SPEC 이 추가하려는 단계 (DAEMON-WIRE-001 후속, FROZEN 변경 없음)

DAEMON-WIRE-001 §3.2 OUT OF SCOPE 는 **CLI 진입점 wiring** 과 **InteractiveHandler 본체** 를 후속 SPEC 에 위임한다. dispatcher / ContextAdapter / LoopController / alias config 4 종 wiring 도 본 OUT OF SCOPE 에 자연 포함된다 (CLI-001 / CMDCTX-001 / CMDLOOP-WIRE-001 / ALIAS-CONFIG-001 후속). 본 SPEC 이 그 자리를 채운다.

본 SPEC 이 제안하는 추가 단계는 DAEMON-WIRE-001 의 step 10 (`hookRegistry.SetSkillsFileChangedConsumer`) 이후, step 11 (`health.ListenAndServe`) 이전에 다음 4 단계를 삽입하는 것이다:

```
10.5  aliasMap, _ := aliasconfig.LoadDefault(...)         (ALIAS-CONFIG-001)
10.6  loopCtrl   := cmdctrl.New(rt, ...)                  (CMDLOOP-WIRE-001)
10.7  ctxAdapter := adapter.New(adapter.Options{
                       Registry:       router.DefaultRegistry(),
                       LoopController: loopCtrl,
                       AliasMap:       aliasMap,
                       Logger:         loggerFacade,
                   })                                      (CMDCTX-001)
10.8  dispatcher := command.NewDispatcher(ctxAdapter)      (COMMAND-001)
```

이후 step 11 의 `health` 가 listen 되기 전에 이미 dispatcher 가 준비됨을 보장한다 (RPC handler 가 dispatcher 를 call 하기 전에 wiring 이 끝나야 하므로).

DAEMON-WIRE-001 SPEC 본문은 **수정하지 않는다**. 본 SPEC 은 DAEMON-WIRE-001 implementation 시점에 `wireSlashCommandSubsystem(rt, registry, logger) (*command.Dispatcher, error)` 같은 helper function call site 를 step 10 직후에 1 줄 추가하는 패턴으로 통합된다 (DAEMON-WIRE-001 의 REQ-WIRE-009 InteractiveHandler placeholder 와 동일한 hook point 패턴).

### 1.3 의존 SPEC 의 implementation 상태

| SPEC | 상태 | 본 SPEC 이 사용하는 surface |
|------|-----|---------------------------|
| SPEC-GOOSE-DAEMON-WIRE-001 | **planned** | step 10 직후 hook point. SPEC 본문은 변경하지 않으나 implementation 시점에 1 줄 추가 필요 |
| SPEC-GOOSE-CMDCTX-001 | **implemented** v0.1.1 | `adapter.New(Options{...})`, `*ContextAdapter` |
| SPEC-GOOSE-CMDLOOP-WIRE-001 | **planned** | `cmdctrl.LoopControllerImpl` (구현체) |
| SPEC-GOOSE-ALIAS-CONFIG-001 | **planned** | `aliasconfig.LoadDefault()`, `aliasconfig.Validate()` |
| SPEC-GOOSE-COMMAND-001 | **implemented** | `command.NewDispatcher(sctx)`, `command.SlashCommandContext` |
| SPEC-GOOSE-ROUTER-001 | **implemented** | `router.DefaultRegistry()` |

본 SPEC 은 `planned` 의존 3 종(DAEMON-WIRE-001, CMDLOOP-WIRE-001, ALIAS-CONFIG-001) 의 implementation 진행을 **선행 요건** 으로 명시한다. 본 SPEC plan 은 작성 가능하나 run 진입은 해당 3 종 중 최소 DAEMON-WIRE-001 + CMDLOOP-WIRE-001 가 implemented 인 시점까지 보류한다 (ALIAS-CONFIG-001 은 graceful empty-map fallback 으로 부재 시에도 daemon 정상 부팅 가능 — §6 참조).

---

## 2. ContextAdapter 의 lifecycle 모델

### 2.1 single-instance vs per-RPC-call

CMDCTX-001 §6 (FROZEN) 의 `ContextAdapter` 는 다음 특성을 가진다:

- struct 가 **stateless** 에 가까움. 내부 atomic flag (plan mode) + `*atomic.Pointer[command.ModelInfo]` (active model) 만 mutable. `LoopController` / `*ProviderRegistry` / `AliasMap` / `Logger` 는 모두 read-only 의존성.
- `WithContext(ctx)` 메서드로 child adapter 를 만들지만 atomic flag 는 **포인터 공유** (CMDCTX-001 v0.1.1 M2 fix). 즉 부모 자식이 같은 plan mode 상태를 본다.

따라서 daemon 에서 `*ContextAdapter` 는 **process 단일 인스턴스** 로 instantiate 하는 것이 자연스럽다:

- multiple RPC handler goroutines 가 동시에 동일 인스턴스의 메서드를 호출 — race-free (REQ-CMDCTX-005).
- per-RPC-call instantiation 은 atomic flag 가 분리되어 plan mode 가 고립된 RPC 단위에서만 의미를 가지게 됨 — 이는 의도와 다름.

**결론**: daemon 은 step 10.7 에서 `*ContextAdapter` 를 1 회 instantiate 하고 RPC handler 들이 share 한다.

### 2.2 dispatcher 의 lifecycle

`command.NewDispatcher(sctx)` 는 dispatcher 1 인스턴스. dispatcher 는 string command-name → handler function 의 map 만 보유 (PR #50 SPEC-GOOSE-COMMAND-001) 하므로 stateless. ContextAdapter 와 동일하게 daemon 단일 인스턴스로 process 전체에서 공유.

### 2.3 RPC handler 주입 패턴

가정: daemon 이 Connect-gRPC 또는 유사한 RPC framework 를 사용한다 (DAEMON-WIRE-001 §3.2 OUT 에 따라 transport 는 본 SPEC 도 미정. health server 는 net/http stdlib 만 확정).

RPC handler 는 보통 다음 두 형태 중 하나로 dispatcher 의존성을 받는다:

- **closure capture**: handler factory 가 dispatcher 를 closure 로 capture 후 RPC service struct 의 메서드로 등록. 가장 단순.
- **service struct field**: `type ServiceImpl struct { Dispatcher *command.Dispatcher; ... }` 후 method receiver 로 접근. test friendly, mock 주입 용이.

본 SPEC 은 closure 또는 struct field 둘 다 허용한다. test 용이성을 위해 struct field 권장.

---

## 3. RPC ProcessUserInput 흐름과 ctx 전파

### 3.1 가정한 RPC 시그니처 (DAEMON-WIRE-001 / TRANSPORT-001 미정 영역)

```go
// 본 research 가 가정하는 시그니처. TRANSPORT-001 실 SPEC 이 결정될 때
// 본 SPEC 의 §6 데이터 모델은 그 결정에 맞춰 minor update 가능.
service AgentService {
    rpc ProcessUserInput(ProcessUserInputRequest) returns (stream ProcessUserInputResponse);
}

type ProcessUserInputRequest struct {
    SessionID string
    Input     string  // raw user input, slash command 포함 가능
    Metadata  map[string]string  // optional: alias overlay, plan mode hint 등
}

type ProcessUserInputResponse struct {
    Type      string  // "text" / "tool_call" / "command_result" / "metadata"
    Payload   []byte
    Metadata  map[string]string  // optional: plan mode flag, model id 등
}
```

### 3.2 dispatcher 호출 경로

handler 는 input 을 검사해서 slash command 인지 판별한다. PR #50 SPEC-GOOSE-COMMAND-001 의 dispatcher 가 이 판별과 routing 을 담당한다:

```
ProcessUserInput(req) {
    ctx := req.Ctx()
    if dispatcher.IsCommand(req.Input) {
        result, err := dispatcher.Dispatch(ctx, req.Input)
        // result → RPC stream 으로 변환
        return
    }
    // 일반 input → query loop 진입 (LoopController.SubmitMessage)
}
```

ctx 전파:

- RPC framework 가 제공하는 ctx 는 client cancellation / deadline 을 반영. dispatcher 는 이 ctx 를 그대로 받아 LoopController 에 전달.
- LoopController 의 4 메서드 (Request* + Snapshot) 는 모두 ctx 첫 인자. CMDCTX-001 / CMDLOOP-WIRE-001 양쪽 SPEC 이 동일.
- daemon root ctx 는 SIGTERM 발생 시 cancel 됨. RPC handler 는 root ctx 를 child 로 derive 하여 사용 (RPC framework 가 알아서 처리).

### 3.3 plan mode metadata flag

ContextAdapter.PlanModeActive() 는 RPC response 의 metadata 에 추가 노출한다:

```
response.Metadata["plan_mode"] = "1"  // PlanModeActive() == true 시
```

CLI client (CLI-001) 가 이 metadata 를 보고 prompt 표시 등의 UX 변경을 적용. 본 SPEC 은 metadata key naming 만 정의하고 UX 는 CLI-001 에 위임.

---

## 4. single-session vs multi-session

### 4.1 CMDCTX-001 의 single-session 가정

CMDCTX-001 §6 (FROZEN) 은 다음을 가정한다:

- `ContextAdapter` 의 `*atomic.Pointer[command.ModelInfo]` 는 **process 전역 active model** 을 의미.
- `*atomic.Bool` plan mode flag 도 process 전역.
- 즉 같은 process 안에서 동시에 여러 session 이 다른 model 을 쓸 수 없다.

이 가정은 single-user daemon (한 사람의 CLI 가 daemon 에 attach) 에서는 문제 없다. multi-user / multi-tab 환경에서는 session 별로 model / plan mode 를 분리해야 한다.

### 4.2 본 SPEC 의 결정

본 SPEC 은 **single-session 가정을 유지** 한다. 근거:

1. CMDCTX-001 의 FROZEN 자산이 single-session 이고, 본 SPEC 은 그것을 변경할 권한이 없다.
2. multi-session 으로 확장하려면 `ContextAdapter` 의 atomic flag 를 `map[SessionID] -> *atomic.Bool` 로 바꾸거나 session 별 adapter instance 를 만들어야 한다 — 이는 별도 SPEC (가칭 `SPEC-GOOSE-CMDCTX-MULTI-SESSION-001`) 의 영역.
3. 0.1.0 release 이전 단계에서는 single-user daemon 이 충분.

**MULTI-SESSION-001 별도 SPEC** 가 본 SPEC 에서 명시적으로 제외된다 (Exclusions §10).

### 4.3 multi-session 으로 진화 시 변경점 (참고용)

본 SPEC 이 single-session 으로 남으므로 변경 불필요하나, 미래 확장 시 다음이 필요:

- `ContextAdapter` 가 `Options.SessionResolver func(ctx) SessionID` 를 받음.
- atomic flag 를 `sync.Map[SessionID, *atomic.Bool]` 로 교체.
- RPC handler 가 ctx 에 SessionID 를 inject (Connect-gRPC interceptor).

이 변경은 CMDCTX-001 v0.2.0 amendment 또는 별도 SPEC 으로 처리.

---

## 5. graceful shutdown 시 LoopController drain 전략

### 5.1 SIGTERM 흐름 (DAEMON-WIRE-001 §6.1 정본)

```
SIGTERM
  → rootCtx cancel
  → step 13 의 <-rootCtx.Done() 해제
  → rt.State.Store(StateDraining)
  → healthSrv.Shutdown(shutdownCtx)
  → rt.Drain.RunAllDrainConsumers(shutdownCtx)
      ↳ tools.Registry.Drain (DAEMON-WIRE-001 step 9 등록)
      ↳ ??? loop drain 은 어디?
  → rt.Shutdown.RunAllHooks(shutdownCtx)
  → exit
```

### 5.2 LoopController drain 의 등록 위치

LoopController (CMDLOOP-WIRE-001) 의 구현체는 in-flight slash command request 를 가질 수 있다 (예: `/compact` 가 진행 중). SIGTERM 시 다음 동작이 필요:

1. 새 slash command request 거부 (이미 RPC handler 가 ctx cancel 로 자연 거부).
2. enqueued request 의 drain — pending command 를 buffer 에서 제거하거나 1회 처리 후 종료.
3. snapshot 호출은 즉시 반환.

본 SPEC 은 LoopController 의 drain 을 `core.Runtime.Drain.RegisterDrainConsumer` 에 등록한다:

```
rt.Drain.RegisterDrainConsumer(core.DrainConsumer{
    Name:    "command.LoopController",
    Fn:      func(ctx context.Context) error { return loopCtrl.Drain(ctx) },
    Timeout: 5 * time.Second,
})
```

`loopCtrl.Drain(ctx)` 의 시그니처는 CMDLOOP-WIRE-001 의 SPEC 에서 결정. 만약 CMDLOOP-WIRE-001 v0.1.0 이 Drain 메서드를 제공하지 않으면 본 SPEC 은 "Drain 호출은 옵션, no-op 도 허용" 으로 처리하고, 후속 minor amendment 에서 추가.

### 5.3 in-flight RPC 처리

RPC framework 의 graceful shutdown 시퀀스:

- `healthSrv.Shutdown(shutdownCtx)` 호출 → `/healthz` 가 5xx 반환 시작 → load balancer 가 traffic 차단.
- 기존 in-flight RPC 는 ctx deadline 까지 처리 시도.
- ctx cancel 시 dispatcher 의 dispatch 중인 command 도 cancel 신호 전파.

dispatcher 자체는 stateless 이므로 별도 drain 불요. ContextAdapter 도 stateless. drain 대상은 LoopController 뿐.

### 5.4 등록 순서 (본 SPEC 이 정의)

```
step 9      tools.Registry.Drain          (DAEMON-WIRE-001)
step 9.5    command.LoopController.Drain  (본 SPEC 추가)
            // tools 보다 먼저 drain 되어야 함:
            //   - LoopController 가 tools 호출 중일 수 있으므로
            //   - LoopController 먼저 drain → 새 tool 호출 막힘 → tools.Drain 안전
```

근거: `core.Runtime.Drain.RunAllDrainConsumers` 가 LIFO 로 호출하면 (또는 FIFO 든) 이 순서를 자연 만족시키도록 등록 순서를 결정. LIFO 라면 step 9.5 를 step 9 앞에 등록, FIFO 라면 step 9 다음에 등록. CORE-001 의 DrainCoordinator 의 호출 순서 의미론을 implementation 시점에 read-back 후 결정.

---

## 6. alias config 로드 실패 처리

### 6.1 ALIAS-CONFIG-001 의 graceful 정책 (FROZEN 가정)

ALIAS-CONFIG-001 §4.1 (planned) 는 다음을 명시:

- 파일 부재 → 빈 맵 + warn log.
- 빈 파일 → 빈 맵.
- 권한 없음 → 빈 맵 + warn log.
- malformed YAML → strict 모드에서 reject (loader error 반환).

### 6.2 본 SPEC 의 정책 결정

본 SPEC 은 daemon bootstrap 에서 ALIAS-CONFIG-001 의 default behavior 를 그대로 수용한다:

- `aliasconfig.LoadDefault(opts)` 의 반환값이 error 라도 daemon 부팅을 fail-fast 시키지 않는다.
- error 시 빈 맵으로 fallback 하고 logger.Warn 으로 사용자에게 알림.
- 사용자가 strict 모드를 요구하면 별도 config flag (`alias.strict_mode: true`) 로 opt-in. 이 경우 loader error → EX_CONFIG fail-fast.

근거:

- alias 는 UX 편의 기능. 없어도 dispatcher 는 `ResolveModelAlias` 가 `ErrUnknownModel` 반환 → 사용자가 canonical name 입력하면 동작 (CMDCTX-001 §6.4 `resolveAlias` 함수).
- daemon 부팅 실패는 supervisor restart loop 위험. 비-critical 의존성에 대해서는 graceful 이 안전.

### 6.3 strict 모드 표면

본 SPEC 은 `cfg.Alias.StrictMode bool` (CONFIG-001 의 `Config` struct 에 alias 필드 추가) 을 새로 정의하지 않는다. ALIAS-CONFIG-001 의 strict/lenient toggle 은 alias config 파일 자체에서 정의되므로 daemon 측은 받기만 한다. CONFIG-001 변경이 발생하면 ALIAS-CONFIG-001 SPEC 본문에서 처리.

---

## 7. per-RPC-call alias overlay

### 7.1 use case

- 사용자가 RPC 호출 시 1회만 다른 alias map 을 적용하고 싶음.
- 예: A/B 테스트 환경에서 client 가 헤더로 alias overlay 를 보내서 특정 RPC 만 다른 모델 매핑을 사용.

### 7.2 가능한 wiring 패턴

- request metadata 에 `alias_overlay: "opus=anthropic/claude-opus-4-7,sonnet=..."` 를 받음.
- handler 가 ContextAdapter 를 wrap 하여 per-call AliasMap 을 overlay.
- ContextAdapter 의 `WithContext(ctx)` 메서드가 child adapter 를 만들 수 있으므로 child 에 새 AliasMap 을 주입.

### 7.3 위험 요소

- atomic flag 의 포인터 공유 (CMDCTX-001 v0.1.1 M2) 와의 상호작용: child 가 alias map 만 다르고 atomic flag 는 공유 → 의도된 동작.
- RPC client 가 임의 alias 를 inject → security concern (alias 가 model 식별자를 비틀어 권한 우회 가능?).
- 본 SPEC 은 **REQ 레벨에서 옵션** 으로 정의하고, security 검토는 SECURITY-001 또는 별도 SPEC 으로 위임.

### 7.4 본 SPEC 의 결정

`Optional` REQ 로 per-RPC-call alias overlay 를 노출. handler 가 metadata 를 받아 ContextAdapter.WithContext(ctx) + alias overlay 적용. **default 는 비활성** — config flag 로 opt-in.

---

## 8. dispatcher panic 시 RPC error 매핑

### 8.1 panic 발생 가능 지점

CMDCTX-001 의 ContextAdapter 는 모든 메서드에서 panic 을 회피하지만 (REQ-CMDCTX-005), dispatcher 자체는 PR #50 SPEC-GOOSE-COMMAND-001 의 영역. dispatcher 의 handler function 이 panic 한다면:

- `ContextAdapter` 는 read-only 호출이므로 panic 영향 없음.
- `LoopController.RequestClear/Compact/...` 는 panic 안전성을 본 SPEC 의 의존인 CMDLOOP-WIRE-001 에서 보장.
- dispatcher 가 panic 하면 RPC handler goroutine crash → daemon process crash 가능.

### 8.2 보호 패턴

RPC handler 는 dispatcher.Dispatch 호출을 `defer recover()` 로 감싼다:

```go
func (s *AgentService) ProcessUserInput(...) (...) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("dispatcher panic", zap.Any("panic", r))
            // RPC error 로 변환
            err = status.Error(codes.Internal, "internal error")
        }
    }()
    return s.Dispatcher.Dispatch(ctx, input)
}
```

### 8.3 RPC error code 매핑

| dispatcher 결과 | RPC error code | 비고 |
|---------------|---------------|------|
| nil (정상) | OK | |
| `command.ErrUnknownCommand` | InvalidArgument | client 입력 오류 |
| `command.ErrUnknownModel` | InvalidArgument | alias 미등록 |
| `adapter.ErrLoopControllerUnavailable` | Unavailable | wiring 미완료 — fail-fast 후 복구 불가 |
| panic recovery | Internal | log 후 사용자에게는 일반 메시지 |
| ctx canceled | Canceled | client cancel |
| ctx deadline exceeded | DeadlineExceeded | timeout |

본 SPEC §4 Unwanted REQ 에 panic recovery 를 명시.

---

## 9. ContextAdapter wiring 실패의 EX_CONFIG fail-fast

### 9.1 wiring 실패 시나리오

| 시나리오 | 처리 |
|--------|------|
| `router.DefaultRegistry()` 가 nil 반환 | EX_CONFIG fail-fast (router 부재는 critical) |
| `cmdctrl.New(rt, ...)` 가 nil 반환 | EX_CONFIG fail-fast (LoopController 부재는 dispatcher 무력화) |
| `adapter.New(...)` 가 nil 반환 | EX_CONFIG fail-fast |
| `command.NewDispatcher(...)` 가 nil 반환 | EX_CONFIG fail-fast |
| `aliasconfig.LoadDefault()` 가 error 반환 | warn + 빈 맵 (graceful, §6 참조) |

### 9.2 step 13 의 적용

DAEMON-WIRE-001 REQ-WIRE-008 (Unwanted) 는 nil consumer 등록 시 EX_CONFIG fail-fast 를 의무화한다. 본 SPEC 의 step 10.6/10.7/10.8 도 동일 의무로 확장:

- 새 nil 검사 + EX_CONFIG return 패턴을 main.go 의 helper function 안에 적용.
- 부분 wiring 으로 StateServing 진입 금지.

---

## 10. 결론 / decision summary

| ID | 결정 | 근거 |
|----|-----|------|
| D-1 | ContextAdapter / dispatcher / LoopController 는 daemon process 단일 인스턴스 | CMDCTX-001 의 stateless + atomic flag 포인터 공유 자산을 그대로 활용 |
| D-2 | wiring 위치는 DAEMON-WIRE-001 step 10 ~ step 11 사이의 step 10.5 / 10.6 / 10.7 / 10.8 | health server listen 전에 dispatcher 가 준비되어야 함 |
| D-3 | single-session 가정 유지. multi-session 은 별도 SPEC | CMDCTX-001 FROZEN 자산 보존 |
| D-4 | LoopController.Drain 을 core.Runtime.Drain 에 등록. tools.Drain 보다 먼저 호출되도록 순서 조정 | 의존 순서: LoopController → tools |
| D-5 | alias config 로드 실패 → 빈 맵 fallback (graceful). strict 모드는 ALIAS-CONFIG-001 의 file-level toggle 로 위임 | UX 편의 기능, 부팅 실패는 과도 |
| D-6 | per-RPC-call alias overlay 는 Optional REQ 로 노출, default 비활성 | 보안 검토 미완 |
| D-7 | dispatcher panic 은 RPC handler 의 defer recover 로 보호. RPC error code 는 codes.Internal | daemon process crash 회피 |
| D-8 | ContextAdapter / LoopController / dispatcher / Registry nil 반환 시 EX_CONFIG fail-fast | DAEMON-WIRE-001 REQ-WIRE-008 정책 일관성 |
| D-9 | plan mode 는 RPC response metadata 의 `plan_mode` key 로 노출 | CLI-001 UX 와 분리 |
| D-10 | 본 SPEC 의 wiring 통합 테스트는 in-process daemon harness + fake RPC handler 로 작성 | DAEMON-WIRE-001 §6.4 옵션 A 와 동일 패턴 |

---

## 11. 참고

### 11.1 SPEC 문서

- `.moai/specs/SPEC-GOOSE-DAEMON-WIRE-001/spec.md` — 13-step bootstrap, EX_CONFIG fail-fast 의무, step ordering FROZEN
- `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — `*ContextAdapter`, `Options`, `SlashCommandContext` 6 메서드 (FROZEN)
- `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` — `LoopControllerImpl` 구현체 (planned)
- `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` — `aliasconfig.LoadDefault`, graceful fallback (planned)
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` — dispatcher, slash command 빌트인 (implemented)
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` — `router.DefaultRegistry()` (implemented)

### 11.2 코드 레퍼런스 (read-only 분석 대상)

- `cmd/goosed/main.go` — 현재 9-step (DAEMON-WIRE-001 baseline) flow
- `internal/command/adapter/adapter.go` — `ContextAdapter`, `Options.AliasMap`, `New()` (CMDCTX-001 v0.1.1)
- `internal/command/adapter/controller.go` — `LoopController` interface (FROZEN)
- `internal/core/drain.go` — `DrainCoordinator.RegisterDrainConsumer` 호출 순서 의미론
- `internal/llm/router/registry.go` — `DefaultRegistry()`

### 11.3 미정 영역 (본 SPEC 이 의존하는 후속 결정)

- TRANSPORT-001 의 Connect-gRPC 채택 여부 — 본 SPEC 은 RPC framework 를 가정만 하고 명시 의존 안 함. 결정되면 §6 데이터 모델 minor update.
- core.DrainConsumer 의 LIFO/FIFO 의미론 — DAEMON-WIRE-001 implementation 시점 read-back 필요.
- CMDLOOP-WIRE-001 의 `loopCtrl.Drain(ctx)` 메서드 시그니처 — CMDLOOP-WIRE-001 v0.1.0 SPEC 본문에 명시 안 되어 있음. 본 SPEC 은 "Drain 메서드 부재 시 no-op fallback" 으로 graceful.

---

Version: 0.1.0
Last Updated: 2026-04-27
Status: research only — no code changes

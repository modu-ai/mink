# Research — SPEC-GOOSE-DAEMON-WIRE-001

> Plan-phase artifact. Implementation은 후속 별도 세션. 본 문서는 SPEC 본문(spec.md)이 정본을 두는 통합 결정의 **근거 기록**이며, REQ/AC 신설의 일대일 추적성을 보장한다.

---

## 1. 현재 main.go 9-step 흐름 (commit `0a71e8e` 기준)

`cmd/goosed/main.go::run()`의 baseline 흐름. 각 단계는 명확한 단일 책임을 가지며, 상태 머신은 `init → bootstrap → serving → draining → stopped`의 5상태를 통과한다.

| 단계 | 호출 | 상태 전이 | 패키지 |
|------|-----|----------|--------|
| 1 | `config.Load(LoadOptions{})` | init | `internal/config` (CONFIG-001 v0.3.1) |
| 2 | `core.NewLogger(cfg.Log.Level, "goosed", version)` | init | `internal/core` (CORE-001 v1.1.0) |
| 3 | `signal.NotifyContext(SIGINT, SIGTERM)` | init | stdlib |
| 4 | `core.NewRuntime(logger, rootCtx)` + `rt.State.Store(StateBootstrap)` | bootstrap | `internal/core` |
| 5 | `health.New(rt.State, version, logger).ListenAndServe(cfg.Transport.HealthPort)` | bootstrap → serving | `internal/health` |
| 6 | `rt.State.Store(StateServing)` + `logger.Info("goosed started")` | serving | — |
| 7 | `<-rootCtx.Done()` | (대기) | — |
| 8 | `rt.State.Store(StateDraining)` + `healthSrv.Shutdown(ctx)` | draining | `internal/core` + `internal/health` |
| 9 | `rt.Drain.RunAllDrainConsumers(ctx)` + `rt.Shutdown.RunAllHooks(ctx)` + `rt.State.Store(StateStopped)` | stopped | `internal/core` |

**관찰**: 단계 9는 이미 `RunAllDrainConsumers`를 호출한다. 그러나 호출되는 consumer 목록은 비어 있다 — `RegisterDrainConsumer`는 어디에서도 호출되지 않기 때문. 따라서 단계 9는 **인프라는 살아 있으나 페이로드가 없는 dead branch**다.

**Import 목록**: `config`, `core`, `health` 3개. `hook`, `tools`, `skill`, `context`, `query`는 main.go에서 import되지 않는다.

---

## 2. 7건 SPEC implementation 상태 매트릭스 (2026-04-25)

| SPEC | 패키지 | 버전 | 상태 | cross-pkg surface | daemon 통합 여부 |
|------|--------|------|------|------------------|----------------|
| CORE-001 | `internal/core/` | v1.1.0 | implemented (PR #15, #16) | `Runtime{Sessions,Drain,Shutdown}`, `WorkspaceRoot`, `DrainConsumer`, `RegisterDrainConsumer`, `RunAllDrainConsumers` | **부분** — Runtime/State는 통합, Drain/Sessions는 인프라만 wire(consumer 미등록) |
| CONFIG-001 | `internal/config/` | v0.3.1 | implemented (PR #20, #21) | `config.Load`, `Config.{Log,Transport,SkillsRoot}` | **완전** — main.go step 1 |
| CONTEXT-001 | `internal/context/` | v0.1.3 | implemented (PR #9) | `DefaultCompactor` | **미통합** — QueryEngine consumer 영역 |
| QUERY-001 | `internal/query/` | v0.1.3 | implemented (PR #6, #7) | `QueryEngine streaming` | **미통합** — Agent consumer 영역 |
| HOOK-001 | `internal/hook/` | v0.3.1 | implemented (PR #11) | `HookRegistry`, `SetWorkspaceRootResolver`, `SetSkillsFileChangedConsumer`, `DispatchFileChanged` 등 24개 이벤트 | **미통합** — main.go에서 import 없음 |
| TOOLS-001 | `internal/tools/` | v0.1.2 | implemented (PR #10) | `Registry`, `Drain`, `IsDraining`, `Executor.Run` | **미통합** — drain consumer 미등록 |
| SKILLS-001 | `internal/skill/` | v0.3.1 | implemented (PR #22) | `SkillRegistry`, `LoadSkillsDir`, `FileChangedConsumer` | **미통합** — main.go에서 import 없음 |
| ROUTER-001 / ADAPTER-001/002 | `internal/llm/` | — | implemented | provider routing | (out of scope — 별도 wire-up SPEC) |
| TRANSPORT-001 | (없음) | v0.1.1 SPEC ready | **코드 미구현** | — | Phase 0 잔여 |

**결론**: 7건 SPEC 중 5건(HOOK / TOOLS / SKILLS / CONTEXT / QUERY)은 패키지는 살아 있으나 daemon main.go에는 결합되지 않은 **고립 상태**다. 본 SPEC이 그중 3건(HOOK / TOOLS / SKILLS)의 결합을 인수한다. CONTEXT / QUERY는 consumer가 daemon이 아닌 후속 SPEC(QUERY consumer는 AGENT)이므로 본 SPEC 범위 밖.

---

## 3. Cross-pkg audit 결함 매핑

### 3.1 `REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25` 식별 결함

| 결함 ID | 심각도 | 대상 SPEC | 본 SPEC이 닫는가? |
|---------|------|---------|-----------------|
| D-CORE-IF-1 | Major | CORE-001 (`WorkspaceRoot` 누락) | **닫힘** — CORE-001 v1.1.0 amendment에서 SPEC 문서 차원 닫힘 + PR #16 코드 닫힘. 본 SPEC은 daemon 통합 차원에서 **추가 닫음**(adapter 등록을 main.go 시퀀스에 명시). |
| D-CORE-IF-2 | Minor | CORE-001 (`Registry.Drain` 등록 의무) | **닫힘** — 동일하게 CORE-001 v1.1.0이 인프라 닫음. 본 SPEC은 **consumer 등록 호출**을 main.go 시퀀스에 명시(REQ-WIRE-004). |

### 3.2 본 SPEC이 명시 책임지는 stub interface

audit §"Implementation 진입 전 체크포인트"의 항목 중 본 SPEC 범위:

- **CORE-001 RED #1** — `core.WorkspaceRoot("test-session")` 시그니처 + 기본 fallback 동작 → CORE-001 implementation에서 닫힘. 본 SPEC은 **adapter 통합** 추가.
- **CORE-001 RED #2** — `tools.Registry.Drain()`이 shutdown hook chain에 등록되어 SIGTERM 시 호출되는지 검증 → AC-WIRE-002로 인수.
- **SKILLS-001 RED #1** — `hookRegistry.SetSkillsFileChangedConsumer(skillsRegistry.FileChangedConsumer)` 호출 + nil 거부 + dispatch 통합 → AC-WIRE-003 + AC-WIRE-006으로 인수.

본 SPEC이 **닫지 않는** audit 항목:

- CLI-001 RED #1/#2 (goose tool list, permission_request) — CLI-001 영역
- SUBAGENT-001 RED #1/#2 (permission bubbling, Worktree dispatch) — SUBAGENT-001 영역

---

## 4. WorkspaceRoot 시그니처 mismatch 분석 + adapter 결정 근거

### 4.1 실제 시그니처 확인 (코드 read-back 2026-04-25)

`internal/hook/types.go:233-238`:
```go
// WorkspaceRootResolver는 sessionID로 workspace root를 반환하는 인터페이스이다.
// SPEC-GOOSE-CORE-001이 구현한다. 본 SPEC은 consumer.
// D15 resolution / REQ-HK-021 b clause
type WorkspaceRootResolver interface {
    WorkspaceRoot(sessionID string) (string, error)
}
```

`internal/core/session.go` (PR #16):
```go
// 패키지 레벨 헬퍼 — nil-safe, default registry 사용
func WorkspaceRoot(sessionID string) string { ... }
```

### 4.2 호출자(사용자) 사양과의 차이

호출자 사양은 시그니처 mismatch를 "`(string, error)` ↔ `string`"으로 기술했으나, 실제 mismatch의 본질은:

| 차원 | HOOK 측 | CORE 측 |
|------|--------|--------|
| 호출 대상 | interface method (instance dispatch) | 패키지 레벨 함수 (no receiver) |
| 반환 타입 | `(string, error)` | `string` only |
| 의미 체계 | "session not found"는 명시적 에러 | "session not found"는 빈 문자열 |

즉 **시그니처 + 호출 형식 + 의미 체계의 3중 mismatch**이며, adapter 패턴이 정확히 처리한다 — adapter struct가 interface를 구현하면서, CORE 함수의 return value를 의미적으로 변환.

### 4.3 대안적 접근 검토

| 대안 | 장점 | 단점 | 채택 여부 |
|------|-----|------|---------|
| (A) **adapter struct in main.go** | 영향 범위 좁음, CORE/HOOK 변경 불필요 | adapter 코드가 daemon 측에 위치 | **채택** |
| (B) HOOK 측 시그니처를 `string`으로 변경 | adapter 불필요 | HOOK-001 v0.3.1 spec amendment 필요, fail-closed 명시성 약화 | 기각 |
| (C) CORE 측에 `(string, error)` 메서드 추가 | adapter 불필요 | CORE-001 amendment 필요, 두 시그니처 공존 시 혼동 | 기각 |
| (D) 별도 `internal/wire` 패키지 신설 | 다중 SPEC 통합 시 확장 가능 | 단일 adapter에 패키지 도입은 과잉 | Phase 0 단계에서 보류, Phase 1+에서 검토 |

**결정 근거**: 본 SPEC은 wire-up only이고 신규 코드 표면은 최소화해야 한다. adapter struct 한 개로 main.go 내에 격리하는 (A)가 영향범위/유지보수/회귀 차단 모든 면에서 우수.

---

## 5. main.go 13-step 신규 흐름 (목표 상태)

`spec.md §6.1` 정본. 본 research.md는 결정 근거를 추가:

```
AS-IS (9-step):  config → logger → signal → runtime → health → serving → wait → drain → shutdown
TO-BE (13-step): config → logger → signal → runtime → hook → tools → skill → adapter → drain-register → consumer-register → health → serving → wait → drain → shutdown
                                              └─ 추가 4 step ─┘                            └─ 기존 흐름 ─┘
```

### 5.1 신규 4단계의 위치 결정

| 신규 단계 | 위치 | 이유 |
|---------|------|------|
| step 5 (`hook.NewHookRegistry`) | step 4(Runtime) 직후 | HookRegistry는 logger만 의존, 다른 패키지에 의존하지 않음. 가장 먼저 인스턴스화 가능 |
| step 6 (`tools.NewRegistry`) | step 5 직후 | ToolsRegistry는 logger만 의존. HookRegistry와 독립 |
| step 7 (`skill.LoadSkillsDir`) | step 6 직후 | SkillsRegistry는 디스크 walk 필요(I/O), step 6보다 약간 비싸므로 마지막에 배치. 다른 registry에 의존하지 않음 |
| step 8 (adapter 등록) | step 7 직후 | adapter는 무상태이지만 hookRegistry에 등록되어야 하므로 hookRegistry 생성 후 |
| step 9 (`Drain.Register`) | step 8 직후 | toolsRegistry + rt.Drain 둘 다 필요. 둘 다 step 6 / step 4에서 준비됨 |
| step 10 (`SetSkillsFileChangedConsumer`) | step 9 직후 | hookRegistry + skillRegistry 둘 다 필요. step 5 + step 7에서 준비됨 |

**총 순서 invariant**: `Runtime < (Hook, Tools, Skill < Adapter, DrainConsumer, FileChangedConsumer) < Health < Serving`

### 5.2 health server를 step 11(adapter 이후)에 둔 근거

대안: health server를 step 5 직후로 앞당기면 부팅 latency 감소.
**기각**: serving 상태 응답 전에 모든 wire-up이 끝나야 health 응답이 의미를 갖는다. 부분 wire 상태에서 `200 OK`를 반환하면 supervisor는 daemon이 정상이라 판단하나 실제로는 SkillsFileChanged 경로가 죽어 있을 수 있음. **fail-fast > fast-boot**.

---

## 6. 다른 architecture 패턴 검토

### 6.1 왜 별도 SPEC인가? (TRANSPORT-001 / AGENT-001과 분리한 근거)

| 후보 | 통합 SPEC 시 단점 | 분리 시 장점 |
|------|----------------|-------------|
| TRANSPORT-001과 통합 | gRPC/UDS listener 등록 + wire-up이 한 SPEC에 섞이면 SPEC 사이즈 폭증, 각각 독립적으로 review/revert 어려움 | gRPC는 후속 ADAPTER-002 / SUBAGENT-001 등 cross-pkg 영향 크므로 별도 SPEC이 낫다 |
| AGENT-001과 통합 | Agent runtime은 QueryEngine + Hook + Tools 모두를 consumer로 사용. wire-up이 Agent SPEC에 섞이면 Agent의 핵심 로직이 묻힘 | 본 SPEC은 daemon "껍데기"의 wire-up이며, AGENT-001은 그 껍데기 위에 올라탈 책임만 가짐 — 분리가 자연스러움 |
| CORE-001 v1.2.0 amendment | CORE-001은 부트스트랩 인프라(`Runtime`, `Drain`, `Sessions`)만 책임하는 좁은 SPEC. consumer 등록 의무는 CORE 책임 영역이 아님 | CORE-001은 인프라 제공자, 본 SPEC은 인프라 사용자. 책임 분리. |

**결론**: 분리가 SDD(Specification-Driven Development) 원칙(단일 책임 + 추적성)에 부합.

### 6.2 왜 in-process integration test인가? (binary E2E test 대안 기각)

| 대안 | 장점 | 단점 | 채택 |
|------|-----|------|-----|
| binary E2E (testcontainers 또는 `os/exec`) | 실제 데몬 동작 검증 | 빌드 시간 + 환경 의존성 + flaky | 기각 |
| in-process (`run()`를 직접 호출) | 빠름, deterministic, race detector 적용 가능 | `run()`이 `os.Exit` 호출하면 안됨 → `main()` / `run()` 분리 필요 | **채택** (`main()`이 이미 `run()` 호출 후 `os.Exit` 호출하는 구조) |

`run() int` 시그니처가 이미 분리되어 있어 in-process 호출이 가능. signal 시뮬레이션은 `signal.NotifyContext` 대신 test에서 직접 cancel 함수 호출로 우회.

---

## 7. 각 cross-pkg consumer 등록 위치 결정 근거

### 7.1 `tools.Registry.Drain` → `core.Runtime.Drain.RegisterDrainConsumer`

**왜 step 9인가?** (step 6 `tools.NewRegistry` 직후가 아닌)

- step 6 직후에 등록해도 동작은 동일. 그러나 등록은 "wire-up phase"의 의미를 가지므로 **adapter 등록 직후, consumer 등록 그룹**에 묶는 것이 가독성에 유리.
- 향후 다른 consumer(예: `subagentSpawner.Drain`, `sessionRegistry.Persist`) 추가 시 step 9 블록에 함께 모아두면 검토 용이.

**왜 `Timeout: 10*time.Second`인가?**

- TOOLS-001 §6.x 기본 graceful drain 시간이 명시되어 있지 않음. CORE-001 `DrainCoordinator.RegisterDrainConsumer`는 zero timeout 시 10s default를 자동 적용(코드 `drain.go:41-43`).
- 본 SPEC은 명시적으로 10s를 전달하여 의도를 코드에 남김. 향후 변경 시 이 값을 조정.

### 7.2 `skillRegistry.FileChangedConsumer` → `hookRegistry.SetSkillsFileChangedConsumer`

**왜 메서드 값을 직접 등록 가능한가?**

`internal/skill/registry.go`의 메서드 시그니처:
```go
func (r *SkillRegistry) FileChangedConsumer(ctx context.Context, changed []string) []string
```

`internal/hook/types.go`의 함수 타입:
```go
type SkillsFileChangedConsumer func(ctx context.Context, changed []string) []string
```

Go의 method value 캡처 규칙에 의해, `skillRegistry.FileChangedConsumer`(receiver-less reference)는 자동으로 `func(ctx, changed) []string` 타입의 값으로 변환되며, receiver `r`은 closure에 캡처된다. 시그니처 일치이므로 adapter 불필요.

**REQ-WIRE-005에서 이를 명시한 이유**: `hookRegistry.SetSkillsFileChangedConsumer(skillRegistry.FileChangedConsumer)`라는 한 줄이 미묘한 호출이며 (메서드 값 vs 메서드 호출), 향후 누군가 `skillRegistry.FileChangedConsumer(ctx, changed)`처럼 호출 결과를 등록하는 실수를 방지하기 위해 EARS REQ로 형식화.

### 7.3 `WorkspaceRootResolverAdapter` 등록

`hookRegistry.SetWorkspaceRootResolver(resolver hook.WorkspaceRootResolver)` 메서드는 HOOK-001 v0.3.1에서 이미 정의되어 있다(또는 본 SPEC implementation 시점에 추가 필요 — implementation 시 read-back으로 확인). 만약 미정의 시 HOOK-001 v0.3.2 minor amendment로 추가하고 본 SPEC v0.1.1 amendment 발행.

**adapter를 main.go에 두는 이유**: `cmd/goosed/wire.go` 또는 `cmd/goosed/main.go` 내부의 unexported struct로 격리. `internal/hook/`나 `internal/core/`에 두면 두 패키지가 서로의 시그니처에 의존하게 되어 import cycle 또는 cross-pkg 결합도 증가. main.go가 두 패키지 모두를 import하는 유일한 지점이므로 자연스러운 위치.

---

## 8. 통합 테스트 전략

### 8.1 테스트 목록

| 테스트 | AC 매핑 | 위치 | 우선순위 |
|--------|---------|-----|--------|
| `TestWireUp_Bootstrap_AllRegistriesNonNil` | AC-WIRE-001 | `cmd/goosed/integration_test.go` | P0 |
| `TestWireUp_SIGTERM_DrainsTools` | AC-WIRE-002 | 동상 | P0 |
| `TestWireUp_FileChanged_DispatchesToSkills` | AC-WIRE-003 | 동상 | P0 |
| `TestWireUp_Adapter_RegisteredSession` | AC-WIRE-004 | `cmd/goosed/wire_test.go` (unit) | P0 |
| `TestWireUp_Adapter_EmptySession_ErrUnresolved` | AC-WIRE-005 | 동상 | P0 |
| `TestWireUp_NilConsumer_RejectedWithEXCONFIG` | AC-WIRE-006 | `cmd/goosed/integration_test.go` | P1 |
| `TestWireUp_InteractiveHandler_NilNoop` | AC-WIRE-007 | `cmd/goosed/wire_test.go` | P2 |
| `TestWireUp_FullCycle_Smoke` | AC-WIRE-008 | `cmd/goosed/integration_test.go` | P0 |

### 8.2 fixture 전략

- `t.TempDir()` 기반 isolated home — `GOOSE_HOME` 환경변수로 격리
- 자유 포트 — `net.Listen("tcp", ":0")` + 즉시 close + 포트 추출 → race 윈도우 존재하나 P0 테스트에서 acceptable
- skills fixture — `ts-helper/SKILL.md` 한 개 파일을 `t.TempDir()` 하위에 작성
- in-process `run()` 호출 — `main()` 우회. signal 시뮬레이션은 별도 cancel 함수 직접 호출(`signal.NotifyContext`가 반환한 `stop` 함수 캡처).

### 8.3 race condition 시나리오

- `-race` detector 활성화 필수
- drain consumer 호출 vs 새 tool execution race: AC-WIRE-002의 sub-test로 `toolsRegistry.IsDraining()` 직후 `executor.Run` 호출하여 `ErrRegistryDraining` 반환 검증
- adapter 동시 호출: AC-WIRE-004의 sub-test로 100 goroutine이 동시에 `adapter.WorkspaceRoot(id)` 호출 → race 0건 + 결과 일관성

---

## 9. 미해결 의문점 (Implementation 시점에 확정)

| 항목 | 의문 | 해결 방법 |
|------|-----|---------|
| Q1 | `hook.ErrInvalidConsumer` sentinel이 HOOK-001 v0.3.1에 존재하는가? | implementation 시점에 `internal/hook/types.go` read-back. 미존재 시 본 SPEC의 wire 코드에서 fallback wrapping 또는 HOOK-001 v0.3.2 minor amendment. |
| Q2 | `hookRegistry.SetWorkspaceRootResolver(resolver)` 메서드가 HOOK-001 v0.3.1에 존재하는가? | implementation 시점에 `internal/hook/registry.go` read-back. 존재 확인 필요 — 본 SPEC은 존재 가정. |
| Q3 | `cfg.SkillsRoot`가 CONFIG-001 v0.3.1에 정의되어 있는가? | implementation 시점에 `internal/config/types.go` read-back. 미정의 시 default `~/.goose/skills`로 fallback (env `GOOSE_HOME` 기반). |
| Q4 | `wireInteractiveHandler` placeholder의 함수 시그니처는? | CLI-001 SPEC implementation 시점에 확정. 본 SPEC은 placeholder 호출 site만 노출. |
| Q5 | drain timeout 10초가 적절한가? | TOOLS-001 §x.x 기본값 확인 후 결정. 미명시 시 10초 default를 채택하고 후속 운영 데이터로 조정. |

---

## 10. Implementation 진입 시 체크리스트

본 SPEC의 implementation 세션 진입 시 첫 단계로 수행할 read-back:

1. `internal/hook/registry.go` → `SetSkillsFileChangedConsumer` nil 거부 동작 확인 (sentinel 에러 반환 여부)
2. `internal/hook/registry.go` → `SetWorkspaceRootResolver` 메서드 존재 확인
3. `internal/config/types.go` → `Config.SkillsRoot` 필드 존재 확인
4. `internal/skill/loader.go` → `LoadSkillsDir`의 부분 실패 동작 확인 (slice error 반환 여부)
5. `internal/tools/registry.go` → `Drain()` 메서드의 idempotency 확인 (반복 호출 안전성)
6. `internal/core/session.go` → `WorkspaceRoot(id)` 헬퍼의 nil-safe 동작 + `core.DefaultSessions().Register` API 확인

위 read-back 결과에 따라 SPEC v0.1.1 minor amendment가 필요할 수 있다. 그 경우 HISTORY에 entry 추가 + 영향 받는 REQ/AC만 정합화.

---

Version: 1.0.0 (plan-phase research artifact)
Last Updated: 2026-04-25
Status: complete (plan phase)

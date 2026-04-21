# SPEC-GOOSE-HOOK-001 — Research & Porting Analysis

> **목적**: Claude Code의 24 runtime event + 40KB `useCanUseTool` 권한 플로우 → GOOSE Go 포팅 계약. `.moai/project/research/claude-primitives.md` §5를 본 SPEC REQ와 1:1 매핑한다.
> **작성일**: 2026-04-21
> **범위**: `internal/hook/` 단일 패키지.

---

## 1. 레포 현재 상태 스캔

```
/Users/goos/MoAI/AgentOS/
├── claude-code-source-map/    # hooks/, useCanUseTool, toolPermission/ 확인
├── .claude/hooks/             # MoAI-ADK 기존 hook 핸들러 (Python, 소비 대상)
├── hermes-agent-main/         # Hook 자체 구현 없음
└── .moai/specs/               # QUERY-001, SKILLS-001 합의 인터페이스 존재
```

- `internal/hook/` → **전부 부재**. Phase 2 신규.
- Claude Code source map에 `useCanUseTool` TS 파일 ~40KB 확인. 직접 포트 대상 아님(React hook 개념 Go 무관). 상태 머신과 분기 로직만 번역.
- MoAI-ADK `.claude/hooks/moai/*.py/sh` 기존 핸들러들은 본 SPEC의 **소비 대상**(InlineCommandHandler가 실행할 파일 경로).

**결론**: GREEN 단계는 `internal/hook/` 6개 파일 **zero-to-one** 신규 + 24 event 상수 + permission flow 상태 머신.

---

## 2. claude-primitives.md §5 원문 인용 → REQ 매핑

### 2.1 24개 런타임 이벤트 (§5.1)

원문 분류:

```
[초기화]
- Setup, SessionStart, SubagentStart

[쿼리 루프]
- UserPromptSubmit
- PreToolUse, PostToolUse, PostToolUseFailure

[컨텍스트 변화]
- CwdChanged, FileChanged
- WorktreeCreate, WorktreeRemove

[권한 & 사용자 상호작용]
- PermissionRequest, PermissionDenied
- Notification
- Elicitation, ElicitationResult

[종료]
- PreCompact, PostCompact
- Stop, StopFailure

[팀 & 백그라운드]
- SubagentStop, TeammateIdle
- TaskCreated, TaskCompleted

[기타]
- SessionEnd, ConfigChange, InstructionsLoaded
```

원문은 28개 이벤트 나열. 본 SPEC은 **24 핵심 이벤트**만 지원 (Elicitation, ElicitationResult, InstructionsLoaded, Setup의 일부 subtype 4개는 추후 SPEC 확장).

| 원문 이벤트 | 본 SPEC 포함 | REQ-HK-001에 확정된 enum 멤버 |
|---|---|---|
| Setup | ✅ | `EvSetup` |
| SessionStart/End | ✅ | `EvSessionStart`, `EvSessionEnd` |
| SubagentStart/Stop | ✅ | `EvSubagentStart`, `EvSubagentStop` |
| UserPromptSubmit | ✅ | `EvUserPromptSubmit` |
| PreToolUse/PostToolUse/PostToolUseFailure | ✅ | 3개 |
| CwdChanged, FileChanged | ✅ | 2개 |
| WorktreeCreate/Remove | ✅ | 2개 |
| PermissionRequest/Denied | ✅ | 2개 |
| Notification | ✅ | 1개 |
| PreCompact/PostCompact | ✅ | 2개 |
| Stop/StopFailure | ✅ | 2개 |
| TeammateIdle | ✅ | 1개 |
| TaskCreated/Completed | ✅ | 2개 |
| ConfigChange | ✅ | 1개 |
| Elicitation/ElicitationResult/InstructionsLoaded | **OUT** | v0.2에 포함 예정 |

→ **REQ-HK-001**이 정확히 24개 상수 확보를 enforce.

### 2.2 Permission Hook Flow (§5.2)

원문:

```
1. hasPermissionsToUseTool() → PermissionResult
   - behavior: "allow" | "deny" | "ask"
   - decisionReason?: { type, reason, ... }

2. Permission Decision 핸들러:
   - interactiveHandler (사용자 터미널 프롬프트)
   - coordinatorHandler (Classifier 통과 대기)
   - swarmWorkerHandler (팀장 권한 버블링)

3. PermissionQueueOps:
   - setYoloClassifierApproval() (auto-mode)
   - recordAutoModeDenial() (거부 추적)
   - logPermissionDecision() (텔레메트리)
```

→ **REQ-HK-009, REQ-HK-012, REQ-HK-017**가 3단 흐름을 Go 상태 머신으로 구현. §6.3의 다이어그램 참조.

### 2.3 Hook Callback 페이로드 (§5.3)

원문 TS 타입:

```typescript
type HookInput = {
  hookEvent: HookEvent
  toolUseID?: string
  tool?: SdkToolInfo
  input?: Record<string, unknown>
  output?: string | Record<string, unknown>
  error?: { code?: string; message: string }
  message?: SDKMessage
}

type HookJSONOutput =
  | { continue?: boolean; suppressOutput?: boolean }
  | { async: true; asyncTimeout?: number }
```

→ **본 SPEC §6.2의 Go 타입이 1:1 포팅**. `any`는 Go 1.18+ type alias. `boolean` → `*bool` (선택 표현).

**Blocking 방식** 원문:

- PreToolUse: `{ continue: false }` → 도구 호출 차단 + `permissionDecision`
- SessionStart: `initialUserMessage`, `watchPaths`
- PostToolUse: `updatedMCPToolOutput`, `additionalContext`

→ **REQ-HK-005, REQ-HK-007, REQ-HK-011** 및 AC-HK-002, AC-HK-005에서 각각 검증.

### 2.4 설계 원칙 (§10)

원문:

- **Progressive Disclosure**: Effort 기반 토큰 예산 (Skills 전용)
- **Atomic State Transitions**: `clearThenRegister` (hot-reload) → **REQ-HK-003, AC-HK-010**
- **Deferred Loading**: MCP deferred
- **Allowlist-Default Deny**: Skills 전용
- **Isolation + Bubbling**: Fork/Worktree/bubble — Subagent 관련 (본 SPEC은 `swarmWorkerHandler`의 bubbling 경로만)
- **Composable Plugins**: 4 primitive 각각 plugin-loadable

본 SPEC은 **Atomic State Transitions**와 **Isolation + Bubbling** 두 원칙을 직접 구현한다.

---

## 3. Go 포팅 매핑표 (claude-primitives.md §7)

| Claude Code (TS) | GOOSE (Go) | 결정 |
|---|---|---|
| `hooks/` 파일군 | `internal/hook/registry.go`, `dispatchers.go`, `handlers.go` | 책임 분리 |
| `useCanUseTool` 40KB React hook | `internal/hook/permission.go` | React 패턴 제거, 순수 상태 머신 |
| `toolPermission/` | `internal/hook/permission.go` 내 `PermissionQueueOps` | 단일 파일 집약 |
| Plugin hook loader | `internal/hook/plugin_loader.go` interface | PLUGIN-001이 구현 |

---

## 4. Go 이디엄 선택 (상세 근거)

### 4.1 `atomic.Pointer[map[...]]` 기반 registry

**대안 비교**:

| 대안 | 장점 | 단점 | 결정 |
|---|---|---|---|
| `sync.RWMutex` 보호 map | 구현 단순 | `Handlers()`가 lock 보유하며 handler 실행 → 외부 lock acquire 시 deadlock 가능 | 탈락 |
| `atomic.Pointer[map[...]]` | lock-free read, handler 실행 중 registry 교체 안전 | write 경로는 여전히 mutex | **채택** |
| `sync.Map` | concurrent-safe primitive | event별 slice ordering 보장 안 됨 | 탈락 |

`atomic.Pointer`가 Go 1.19+에서 generic pointer atomic 제공. 본 SPEC은 Go 1.22+ 타겟이므로 안전.

### 4.2 Shell Command Hook 실행

`exec.CommandContext`는 context cancel 시 자동 SIGKILL 전송. timeout 통합.

- stdin은 `cmd.StdinPipe()` 후 별도 goroutine에서 JSON 쓰기 → 쓰기 완료 시 close. deadlock 방지.
- stderr는 별도 `bytes.Buffer` 수집 후 에러 메시지 구성.
- `cmd.ProcessState.ExitCode()`로 exit code 확인.

**Shell selection**:

- 기본 `/bin/sh` (POSIX 호환).
- `cfg.Shell`로 override 가능 (e.g., `/bin/bash`, `/bin/zsh`).
- Windows 시 `cmd.exe /c`.

### 4.3 Matcher: glob default

`path/filepath.Match`는 간결하지만 `**`(다계층) 미지원. SKILLS-001은 gitignore 라이브러리 사용. 본 SPEC은 더 단순한 이벤트 매처:

- `FileChanged`: `**` 지원 필요 → gitignore 라이브러리 의존 (SKILLS-001과 일치).
- 기타 이벤트: 단순 wildcard `*` 정도만 필요 → `filepath.Match`.
- `regex:` prefix는 opt-in.

구현: Matcher 추상 인터페이스 + event-type별 default 매처 선택.

### 4.4 비-TTY 환경 탐지

```go
func isInteractive() bool {
    if os.Getenv("GOOSE_HOOK_NON_INTERACTIVE") == "1" { return false }
    fi, err := os.Stdout.Stat()
    if err != nil { return false }
    return (fi.Mode() & os.ModeCharDevice) != 0
}
```

CI 환경에서는 `GOOSE_HOOK_NON_INTERACTIVE=1` 설정 권장 → 기본 deny (REQ-HK-017).

### 4.5 Deep copy of HookInput (REQ-HK-014)

```go
func copyHookInput(in HookInput) HookInput {
    data, _ := json.Marshal(in)
    var out HookInput
    _ = json.Unmarshal(data, &out)
    return out
}
```

JSON roundtrip으로 deep copy. 성능상 비용이 있으나 (`HookInput`은 작음 < 10KB), 안전성 우선. handler가 mutate해도 downstream에 영향 없음.

---

## 5. 참조 가능한 외부 자산 분석

### 5.1 Claude Code TypeScript

- `useCanUseTool` 40KB: 상태 관리가 React hook에 분산. Go 포팅 시 단일 함수 + 상태 머신으로 통합 가능.
- `toolPermission/` 디렉토리 파일들: `PermissionQueueOps` 인터페이스의 직접 영감 source.
- **직접 포트 대상 없음**, 패턴만.

### 5.2 MoAI-ADK `.claude/hooks/`

- 기존 Python/Shell hook 스크립트 다수 (e.g., `handle-session-start.sh`, `stop-handler.py`).
- 본 SPEC이 제공하는 `InlineCommandHandler`가 이들을 실행 가능.
- 테스트 fixture로 재활용 가능: `testdata/hooks/echo-input.sh`, `testdata/hooks/deny-all.sh`.

### 5.3 Claude Code source map 기타

- `hooks/` 디렉토리 내 개별 이벤트 처리 파일 확인. Go 포팅 시 단일 `dispatchers.go`에 집약(파일 폭발 방지).

---

## 6. 외부 의존성 합계

| 모듈 | 버전 | 본 SPEC 사용 | 결정 근거 |
|------|-----|-----------|---------|
| `go.uber.org/zap` | v1.27+ | ✅ 로깅 | CORE-001 계승 |
| `github.com/stretchr/testify` | v1.9+ | ✅ 테스트 | CORE-001 계승 |
| `github.com/denormal/go-gitignore` | v0.3+ | ✅ FileChanged matcher (SKILLS-001과 공유) | 부정 패턴 |
| Go stdlib `os/exec` | 1.22+ | ✅ subprocess | 표준 |
| Go stdlib `sync/atomic` | 1.22+ | ✅ atomic.Pointer | Go 1.19+ |
| Go stdlib `encoding/json` | 1.22+ | ✅ HookInput/Output | 표준 |
| Go stdlib `regexp` | 1.22+ | ✅ matcher `regex:` prefix | 표준 |

**의도적 미사용**:

- `fsnotify`: 본 SPEC은 FileChanged 이벤트 **수신자**. watcher는 CORE-001.
- 외부 plugin host 라이브러리(`hashicorp/go-plugin` 등): gRPC 기반 과설계. 본 SPEC의 plugin은 단순 shell command subprocess.

---

## 7. 테스트 전략 (TDD RED → GREEN)

### 7.1 Unit 테스트 (25~32개)

**Types 레이어**:
- `TestHookEventNames_Exactly24` — REQ-HK-001
- `TestHookJSONOutput_MarshalUnmarshal_Roundtrip`
- `TestHookInput_DeepCopy_IsolatesMutations` — REQ-HK-014

**Registry 레이어**:
- `TestRegistry_Register_FIFOOrder` — REQ-HK-002
- `TestRegistry_ClearThenRegister_Atomic` — AC-HK-010
- `TestRegistry_Handlers_ReturnsNewSlice` — defensive copy
- `TestRegistry_Handlers_MatcherFilters`

**Handlers 레이어**:
- `TestInlineCommandHandler_Stdin_JSONDelivered`
- `TestInlineCommandHandler_Stdout_JSONParsed`
- `TestInlineCommandHandler_Timeout_KillsProcess`
- `TestInlineCommandHandler_NonZeroExit_HandlerError`
- `TestInlineCommandHandler_MalformedOutput_HandlerError`
- `TestInlineFuncHandler_Invoke`

**Permission 레이어**:
- `TestUseCanUseTool_YOLOAutoApprove` — AC-HK-007
- `TestUseCanUseTool_HandlerDenyShortCircuit` — AC-HK-008
- `TestUseCanUseTool_InteractiveFallback_TTY`
- `TestUseCanUseTool_NonTTY_DefaultDeny` — AC-HK-009
- `TestPermissionQueueOps_SetYoloClassifierApproval`
- `TestPermissionQueueOps_RecordAutoModeDenial`
- `TestPermissionQueueOps_LogPermissionDecision`

**Dispatchers 레이어**:
- `TestDispatchPreToolUse_BlocksOnContinueFalse` — AC-HK-002
- `TestDispatchPreToolUse_NoBlockAggregatesOutputs`
- `TestDispatchPostToolUse_AsyncHandlerSpawned`
- `TestDispatchSessionStart_AggregatesOutputs` — AC-HK-005
- `TestDispatchFileChanged_InvokesSkillsConsumer` — AC-HK-006
- `TestDispatchSubagentStop_OrdersHandlers`

### 7.2 Integration 테스트 (AC 1:1, `integration` build tag)

| AC | Test |
|---|---|
| AC-HK-001 | `TestHook_24Events_Exactly` |
| AC-HK-002 | `TestHook_PreToolUse_BlockingHandler` |
| AC-HK-003 | `TestHook_InlineCommand_EndToEnd` — real `/bin/sh` + `jq`  |
| AC-HK-004 | `TestHook_InlineCommand_Timeout` — `sleep 60` + timeout 2s |
| AC-HK-005 | `TestHook_SessionStart_MultiHandler_Aggregation` |
| AC-HK-006 | `TestHook_FileChanged_TriggersSkillsConsumer` |
| AC-HK-007 | `TestHook_UseCanUseTool_YOLOAutoApprove` |
| AC-HK-008 | `TestHook_UseCanUseTool_HandlerDeny` |
| AC-HK-009 | `TestHook_UseCanUseTool_NonTTY_Deny` |
| AC-HK-010 | `TestHook_Registry_ClearThenRegister_RaceFree` — goroutine stress test |

### 7.3 Race detector

`go test -race ./internal/hook/...` 필수. 특히:

- Registry `ClearThenRegister` vs `Handlers` concurrent access (AC-HK-010 core).
- Async handler goroutine 누수 확인 (`goleak` 라이브러리 도입 고려).

### 7.4 커버리지 목표

- `internal/hook/`: 90%+
- Permission flow (`permission.go`): 95%+ (보안 크리티컬)

---

## 8. 오픈 이슈

1. **24 vs 28 이벤트**: 본 SPEC은 핵심 24개. Elicitation/ElicitationResult/InstructionsLoaded/Setup sub-types는 v0.2. 근거: MVP milestone 1~4에서 이들 이벤트 소비자 없음.
2. **InteractiveHandler 구현 경계**: CLI-001이 구현. 본 SPEC은 `PermissionResult`를 반환하는 인터페이스만 정의. 구현 시점에서 prompts의 UX는 별도 논의.
3. **CoordinatorHandler 경로**: SUBAGENT-001의 coordinator mode와 연동. `permCtx.Role`에 `coordinator` 값이 있을 때 호출되는 handler는 SUBAGENT-001이 등록.
4. **Plugin hook 실행 샌드박싱**: 현재는 shell command 실행(privilege 미상승). 향후 WASM 기반 샌드박스 고려(별도 SPEC).
5. **Async hook 결과 활용**: `PostToolUse.additionalContext`는 async 수집하여 다음 turn의 context에 append. 타이밍 계약(어느 turn에 반영되는가)은 QUERY-001과의 교차 검증 필요.
6. **Event ordering 보장**: 단일 세션 내 event 순서 보장은 QueryEngine의 goroutine 순차성에 의존. 멀티 세션 간 ordering은 미보장.
7. **Deep copy 성능**: JSON roundtrip은 대용량 payload(수백 KB)에서 느림. PostToolUse에 큰 결과가 있을 때 성능 영향 측정 필요 → 초기 measurement 후 struct-level copy로 전환 판단.

---

## 9. 구현 규모 예상

| 영역 | 파일 수 | 신규 LoC | 테스트 LoC |
|---|---|---|---|
| `types.go` (24 event + input/output) | 1 | 200 | 150 |
| `registry.go` (atomic swap) | 1 | 180 | 300 |
| `handlers.go` (3 구현) | 1 | 350 | 500 |
| `permission.go` (`useCanUseTool` flow) | 1 | 300 | 450 |
| `dispatchers.go` (Dispatch* 함수군) | 1 | 400 | 550 |
| `plugin_loader.go` (interface) | 1 | 50 | 80 |
| **합계** | **6** | **~1,480** | **~2,030** |

테스트 비율: 58%.

---

## 10. 결론

- **상속 자산**: TypeScript source map 설계 참조만. React hook 개념 제거, 순수 상태 머신.
- **핵심 결정**:
  - 24개 핵심 이벤트로 MVP 확정, 확장은 v0.2.
  - `atomic.Pointer[map]` 기반 registry — lock-free read, race-free swap.
  - Permission flow 상태 머신: YOLO → hook dispatch → role-based route → non-TTY fallback deny.
  - Deep copy of `HookInput` — handler mutation isolation 보장.
  - `useCanUseTool`은 **순수 함수 + injectable handler**. React hook 개념 없음.
- **Go 버전**: 1.22+ (CORE-001 정합).
- **다음 단계 선행 요건**: QUERY-001이 `DispatchPreToolUse` 결과를 loop에 소비하는 경계 확정. SKILLS-001이 `FileChangedConsumer` 함수 타입 확정.

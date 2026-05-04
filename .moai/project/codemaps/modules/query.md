# query 패키지 — QueryEngine State Machine

**위치**: internal/query/  
**파일**: 12개 (.go + _test.go)  
**상태**: ✅ Active (SPEC-GOOSE-QUERY-001)

---

## 목적

단일 세션 = 단일 QueryEngine 인스턴스 (1:1 대응). 대화 상태 관리, Tool dispatch, Permission Ask/resolve.

---

## 공개 API

### QueryEngine
```go
type QueryEngine struct {
    cfg QueryEngineConfig
    mu sync.Mutex                    // SubmitMessage 직렬화
    stateMu sync.Mutex               // state 읽기/쓰기
    state loop.State                 // 대화 상태 (messages, budget)
    permInbox chan loop.PermissionDecision  // Ask permission 채널
    pendingPerms map[string]struct{} // 현재 pending toolUseID
}

// @MX:ANCHOR [AUTO] Main session entry point
// @MX:REASON: Fan-in ≥3 (CLI, Transport, Subagent)
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan message.SDKMessage, error)
// REQ-QUERY-005: 10ms 내 반환
// Returns unbuffered channel (non-blocking)

// internal/query/engine.go:293
// behavior is the permission decision (approval/denial encoded as int per
// PermissionBehavior constants); reason is operator-facing rationale.
func (e *QueryEngine) ResolvePermission(toolUseID string, behavior int, reason string) error
// REQ-QUERY-013: Unknown ID silent drop 방지
```

---

## 내부 상태

### loop.State (loop/state.go)
```go
type State struct {
    Messages             []message.Message    // 누적 메시지
    TaskBudgetRemaining  int64               // 토큰 예산
    MessageBudgetUsed    int64               // 누적 사용량
    ToolUsesPending      []string            // 현재 Ask 대기 중
    LastCompletionAt     time.Time
}
```

### QueryEngineConfig
```go
type QueryEngineConfig struct {
    TaskBudget          Budget              // 토큰 예산
    ContextAdapter      ContextAdapter      // Budget tracking
    LLMRouter           llm.Router          // Provider routing
    MemoryProvider      memory.Provider     // Recall
    PermissionRequester PermissionRequester // Permission checks
    Logger              *zap.Logger
}
```

---

## 생명주기

### 생성 (New)
```go
engine, err := query.New(cfg)
// cfg 유효성 검증
// permInbox := make(chan PermissionDecision, 4)
// pendingPerms := make(map[string]struct{})
```

### SubmitMessage Flow (단계 1-5)

```
Step 1: Acquire mu lock (serialization)
  └─ REQ-QUERY-004: Prevent concurrent SubmitMessage

Step 2: Snapshot state
  ├─ Copy currentState
  └─ Append userMessage

Step 3: Create output channel
  └─ out := make(chan SDKMessage)  // unbuffered

Step 4: Spawn Agent goroutine
  ├─ go agent.Execute(snapshot)
  ├─ Agent sends responses to out
  └─ Agent calls OnComplete(finalState)

Step 5: Return immediately
  ├─ mu.Unlock()
  └─ return out  // REQ-QUERY-005: <10ms
```

### Permission Ask/Resolve (단계 6-8)

```
Step 6: Agent encounters Ask tool
  └─ loop.askPermission(toolUseID, request)
      └─ Add toolUseID to pendingPerms
      └─ Emit PermissionRequest message

Step 7: External goroutine (CLI/Bridge)
  └─ ResolvePermission(toolUseID, approved)
      ├─ Check pendingPerms (pendingPermsMu)
      ├─ Send decision to permInbox (cap 4)
      └─ REQ-QUERY-013: Unknown ID → error

Step 8: Loop receives decision
  └─ <-permInbox
      └─ Continue with decision
```

### OnComplete (단계 9-10)

```
Step 9: Agent completes
  └─ loop.OnComplete(finalState, error)
      ├─ stateMu.Lock()
      ├─ e.state = finalState (merge)
      └─ stateMu.Unlock()

Step 10: Close output channel
  └─ close(out)
```

---

## @MX 주석

### @MX:ANCHOR
```go
// @MX:ANCHOR [AUTO] 모든 상위 레이어의 단일 진입점
// @MX:REASON: REQ-QUERY-001 - 대화 세션과 1:1 대응
type QueryEngine struct { ... }

// @MX:ANCHOR [AUTO] 모든 상위 레이어의 단일 스트리밍 진입점
// @MX:REASON: REQ-QUERY-001/005 - 10ms 마감 + 세션 1:1
// @MX:WARN [AUTO] goroutine spawn 지점 - State 단독 소유
// @MX:REASON: REQ-QUERY-015 - spawned goroutine이 State 단독 소유
func (e *QueryEngine) SubmitMessage(...) (<-chan message.SDKMessage, error)
```

### @MX:WARN
```go
// @MX:WARN [AUTO] 여러 goroutine에서 send, loop만 receive
// @MX:REASON: REQ-QUERY-013 - Ask 분기 재개 단일 경로
permInbox chan loop.PermissionDecision

// @MX:WARN [AUTO] pendingPerms는 loop(등록) + 외부(조회) 공유
// @MX:REASON: REQ-QUERY-013 - unknown ID 감지
pendingPermsMu sync.Mutex
pendingPerms   map[string]struct{}
```

---

## 동시성 패턴

### SubmitMessage 직렬화 (mu)
```
mu.Lock() [acquire]
  ├─ snapshot state
  ├─ spawn agent
  └─ mu.Unlock() [release]

↑ Each SubmitMessage is serialized (only one active)
```

### State 갱신 (stateMu)
```
Agent goroutine (async)
  └─ loop runs
      └─ OnComplete calls stateMu.Lock()
          ├─ e.state = finalState
          └─ stateMu.Unlock()

External read (ResolvePermission)
  └─ stateMu.RLock()
      ├─ check pendingPerms
      └─ stateMu.RUnlock()
```

### Permission Inbox (buffered chan)
```
Sender: External goroutine (CLI/WebSocket handler)
  └─ permInbox <- decision  (cap 4, won't block)

Receiver: Loop goroutine (single)
  └─ decision := <-permInbox
```

---

## 에러 처리

### Config Validation
```go
if err := validateConfig(cfg); err != nil {
    return nil, fmt.Errorf("invalid QueryEngineConfig: %w", err)
}
```

### SubmitMessage Errors
```
ErrAlreadyRunning
  └─ SubmitMessage already active (mu locked)

ErrConfigInvalid
  └─ Config validation failed

ErrContextDeadlineExceeded
  └─ Budget exhausted or timeout
```

### ResolvePermission Errors
```
ErrUnknownPermissionRequest
  └─ toolUseID not in pendingPerms (REQ-QUERY-013)

ErrPermissionDenied
  └─ User rejected permission
```

---

## 테스트

| 파일 | 커버리지 |
|------|---------|
| engine_test.go | Main flow, error cases |
| lifecycle_test.go | State transitions |
| types_test.go | Type marshaling |

**중요 테스트**:
- `TestSubmitMessage` — 10ms deadline ✓
- `TestPermissionAsk` — pendingPerms tracking ✓
- `TestStateSnapshot` — race detection ✓

---

## SPEC 참조

| REQ | 내용 |
|-----|------|
| REQ-QUERY-001 | 1:1 session:QueryEngine correspondence |
| REQ-QUERY-002 | Unbuffered channel guarantee |
| REQ-QUERY-004 | SubmitMessage serialization (mu) |
| REQ-QUERY-005 | 10ms return deadline |
| REQ-QUERY-013 | Unknown permission request handling |
| REQ-QUERY-015 | Loop goroutine State ownership |

---

**Version**: query v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~300  
**@MX:ANCHOR Candidates**: 2 (SubmitMessage, loop)

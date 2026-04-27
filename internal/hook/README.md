# hook

Goose 데몬의 hook 시스템을 구현합니다. 생애주기 이벤트에 등록된 hook을 실행하고 권한 관리를 수행합니다.

## 개요

`hook` 패키지는 goosed 데몬의 플러그인 가능한 확장 시스템인 hook을 관리합니다. 사용자 정의 스크립트를 데몬 생애주기 이벤트(부트스트랩, shutdown, 명령 실행 등)에 연결하고 권한을 제어합니다.

## 주요 컴포넌트

### Dispatcher

Dispatch* 함수군의 설정을 담는 구조체입니다.

```go
type Dispatcher struct {
    Registry    *HookRegistry           // hook 레지스트리
    Logger      *zap.Logger             // 로거
    PermQueue   PermissionQueueOps       // 권한 큐 연산
    Interactive InteractiveHandler      // CLI 대화형 핸들러
    Coordinator CoordinatorHandler      // SUBAGENT 조정자
    SwarmWorker SwarmWorkerHandler      // SUBAGENT 스웜 워커
    IsTTY       func() bool             // TTY 여부 확인 함수
}
```

### HookRegistry

등록된 hook을 관리하는 컨테이너입니다.

```go
type HookRegistry struct {
    // 생략 (내부 구현)
}
```

### Hook 타입

```go
type Hook struct {
    Name     string            // hook 이름
    Command  string            // 실행 명령
    Args     []string          // 명령 인자
    Env      []string          // 환경변수
    When     TriggerWhen       // 실행 시점 (Before/After)
    Stage    TriggerStage      // 트리거 스테이지
}
```

## 트리거 이벤트

### TriggerWhen

```go
type TriggerWhen string

const (
    Before TriggerWhen = "before" // 이벤트 전 실행
    After  TriggerWhen = "after"  // 이벤트 후 실행
)
```

### TriggerStage

```go
type TriggerStage string

const (
    StageBootstrap TriggerStage = "bootstrap" // 부트스트랩
    StageShutdown  TriggerStage = "shutdown"   // shutdown
    StageCommand   TriggerStage = "command"    // 명령 실행
)
```

## 주요 함수

### DispatchBootstrap

부트스트랩 hook을 실행합니다.

```go
func DispatchBootstrap(ctx context.Context, d *Dispatcher) error
```

**REQ-HK-004**: fan_in >= 5 (각 Dispatch 함수)

### DispatchShutdown

Shutdown hook을 실행합니다.

```go
func DispatchShutdown(ctx context.Context, d *Dispatcher) error
```

### DispatchCommand

명령 실행 hook을 실행합니다.

```go
func DispatchCommand(ctx context.Context, d *Dispatcher, cmd string, args ...string) error
```

## 권한 관리

### PermissionQueueOps

tool 사용 권한 큐 연산 인터페이스입니다.

```go
type PermissionQueueOps interface {
    Enqueue(toolUseID string) error
    Dequeue(toolUseID string) error
    IsPending(toolUseID string) bool
}
```

### 권한 흐름

```
1. LLM이 tool call 요청
2. PermissionQueueOps.Enqueue(toolUseID)
3. 사용자 승인 대기 (InteractiveHandler 또는 자동 승인)
4. 승인 완료 후 PermissionQueueOps.Dequeue(toolUseID)
5. tool 실행
```

## 사용 예시

### Hook 등록

```go
registry := hook.NewHookRegistry()

// 부트스트랩 후 실행될 hook 등록
registry.Register(hook.Hook{
    Name:    "post-bootstrap",
    Command: "/usr/local/bin/setup.sh",
    When:    hook.After,
    Stage:   hook.StageBootstrap,
})
```

### Dispatcher 사용

```go
import (
    "context"
    "go.uber.org/zap"
    "github.com/modu-ai/goose/internal/hook"
)

func main() {
    logger := zap.NewExample()
    registry := hook.NewHookRegistry()

    dispatcher := &hook.Dispatcher{
        Registry:  registry,
        Logger:    logger,
        PermQueue: NewPermissionQueue(),
        // ... 기타 의존성
    }

    // 부트스트랩 hook 실행
    if err := hook.DispatchBootstrap(context.Background(), dispatcher); err != nil {
        logger.Fatal("bootstrap hook failed", zap.Error(err))
    }

    // ... 메인 루프 ...

    // Shutdown hook 실행
    if err := hook.DispatchShutdown(context.Background(), dispatcher); err != nil {
        logger.Error("shutdown hook failed", zap.Error(err))
    }
}
```

### 사용자 권한 요청

```go
func requestPermission(toolUseID string) error {
    // 권한 큐에 등록
    if err := dispatcher.PermQueue.Enqueue(toolUseID); err != nil {
        return err
    }

    // 대화형 사용자 승인 (TTY인 경우)
    if dispatcher.IsTTY() {
        return dispatcher.Interactive.RequestPermission(toolUseID)
    }

    // 비대화형 자동 거부
    return errors.New("permission denied (non-TTY)")
}
```

## @MX 태그

Dispatcher는 단일 진입점으로 fan_in >= 5이므로 `@MX:ANCHOR` 태그가 지정됩니다:

```go
// @MX:ANCHOR: [AUTO] 모든 Dispatch* 함수의 공통 의존성 컨테이너
// @MX:REASON: SPEC-GOOSE-HOOK-001 REQ-HK-004 — fan_in >= 5 (각 Dispatch 함수)
type Dispatcher struct { ... }
```

## 크로스 패키지 인터페이스

### CORE-001 통합

CORE-001의 `Runtime`이 Shutdown hook을 호출:

```go
// core/runtime.go
func (sm *ShutdownManager) Execute(ctx context.Context) error {
    return hook.DispatchShutdown(ctx, sm.dispatcher)
}
```

### COMMAND-001 통합

COMMAND-001의 builtin 명령이 hook을 호출:

```go
// command/builtin/status.go
func (c *StatusCommand) Execute(ctx context.Context) error {
    // 명령 전 hook 실행
    if err := hook.DispatchCommand(ctx, dispatcher, "status"); err != nil {
        return err
    }

    // ... 명령 로직 ...
}
```

## 격리 및 보안

### Process 격리

hook 실행은 프로세스 격리 환경에서 수행됩니다:

- **Unix**: `rlimit`으로 자원 제한
- **Darwin**: `process_darwin.go` 플랫폼별 구현
- **Linux**: `process_linux.go` 플랫폼별 구현

### 권한 검증

- tool 사용 시 명시적 사용자 승인 필요 (REQ-HK-021(b))
- silent drop 금지 (QUERY-001과 연동)
- 권한 큐로 중복 요청 방지

## SPEC 참조

본 패키지는 **SPEC-GOOSE-HOOK-001**에 의해 정의됩니다.

- REQ-HK-001 ~ REQ-HK-027: 모든 요구사항 충족
- 테스트 커버리지: 85.5%
- COMMAND-001 PR #50에서 HOOK-001 언급

## 의존성

- `context`: 생애주기 및 취소
- `path/filepath`: 경로 조작
- `time`: 타임아웃
- `go.uber.org/zap`: 로깅

## 테스트

```bash
go test ./internal/hook/...
go test -race ./internal/hook/...
go test -cover ./internal/hook/...
```

## 보안 고려사항

### Hook 스크립트 검증

- hook 실행 전 스크립트 경로 검증
- 상대 경로만 허용 (`../` 차단)
- 실행 가능 권한 확인

### 환경변수 주입

민감한 환경변수만 hook에 노출:

```go
safeEnv := []string{
    "PATH=/usr/bin:/bin",
    "HOME=" + os.Getenv("HOME"),
    "GOOSE_SESSION_ID=" + sessionID,
    // API 키, 토큰은 제외
}
```

## 성능 최적화

### 비동기 Hook 실행

긴 실행 시간이 예상되는 hook는 비동기로 실행:

```go
go func() {
    if err := hook.ExecuteAsync(ctx); err != nil {
        logger.Warn("async hook failed", zap.Error(err))
    }
}()
```

### Hook 타임아웃

hook 실행 시 타임아웃 강제:

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

if err := hook.Execute(ctx); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        logger.Warn("hook timeout")
    }
}
```

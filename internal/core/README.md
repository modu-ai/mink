# core

Goose 데몬의 핵심 런타임 컴포넌트를 제공합니다.

## 개요

`core` 패키지는 goosed 데몬의 부트스트랩 및 Graceful Shutdown을 담당하는 핵심 런타임 시스템입니다. 모든 상위 레이어(CLI, TRANSPORT, SUBAGENT)가 이 패키지를 통해 데몬 생애주기에 참여합니다.

## 주요 컴포넌트

### Runtime

`Runtime` 구조체는 goosed 프로세스의 핵심 컴포넌트를 묶는 컨테이너입니다:

```go
type Runtime struct {
    State     *StateHolder       // 프로세스 생애주기 상태 홀더
    Logger    *zap.Logger        // 구조화 JSON 로거
    Shutdown  *ShutdownManager   // cleanup hook 관리자
    RootCtx   context.Context    // 데몬 생애주기 컨텍스트 (SIGINT/SIGTERM 시 cancel)
    Sessions  SessionRegistry    // sessionID → workspace root 매핑
    Drain     *DrainCoordinator  // in-flight 작업 마감 관리자
}
```

### 생애주기 상태

```go
const (
    StateInit = iota // 초기화 상태
    StateReady       // 부트스트랩 완료, 요청 수신 가능
    StateDraining    // Graceful shutdown 진행 중 (새 요청 거부)
    StateShutdown    // 모든 cleanup 완료, 프로세스 종료
)
```

### 주요 함수

#### `NewRuntime(logger *zap.Logger, rootCtx context.Context) *Runtime`

기본값으로 초기화된 Runtime을 반환합니다.

- `logger`가 nil이면 nop 로거 사용
- `rootCtx`가 nil이면 `context.Background()` 사용

#### `Bootstrap(runtime *Runtime) error`

데몬 부트스트랩을 수행합니다. (GREEN 단계)

1. 환경 설정 로드
2. 로거 초기화
3. LLM provider 초기화
4. Hook 시스템 초기화
5. 상태를 `StateReady`로 전환

#### `RequestShutdown(runtime *Runtime)`

Graceful shutdown을 요청합니다. (SIGINT/SIGTERM 핸들러)

1. 상태를 `StateDraining`으로 전환
2. `DrainCoordinator`를 통해 in-flight 작업 마감
3. `ShutdownManager`의 cleanup hook 실행
4. 상태를 `StateShutdown`으로 전환

## 사용 예시

### 데몬 부트스트랩

```go
import (
    "context"
    "go.uber.org/zap"
    "github.com/modu-ai/goose/internal/core"
)

func main() {
    logger := zap.NewExample()
    ctx := context.Background()

    runtime := core.NewRuntime(logger, ctx)
    
    // 부트스트랩
    if err := core.Bootstrap(runtime); err != nil {
        logger.Fatal("bootstrap failed", zap.Error(err))
    }

    // ... 메인 루프 ...

    // Graceful shutdown
    core.RequestShutdown(runtime)
}
```

### 생애주기 상태 확인

```go
state := runtime.State.Load()
if state == core.StateReady {
    // 요청 처리 가능
} else if state == core.StateDraining {
    // 새 요청 거부
}
```

### Session 레지스트리

```go
workspaceRoot := runtime.Sessions.WorkspaceRoot(sessionID)
```

## SPEC 참조

본 패키지는 **SPEC-GOOSE-CORE-001** (goosed 데몬 부트스트랩 및 Graceful Shutdown)에 의해 정의됩니다.

- REQ-CORE-001 ~ REQ-CORE-014: 모든 요구사항 충족
- AC-CORE-001 ~ AC-CORE-009: 모든 인수조건 충족
- 테스트 커버리지: 88.5%

## 크로스 패키지 인터페이스

### `WorkspaceRoot(sessionID string) string`

HOOK-001 dispatcher가 호출하는 패키지 헬퍼 함수입니다. sessionID를 workspace root 경로로 변환합니다.

```go
// @MX:ANCHOR: HOOK-001 cross-package surface
func WorkspaceRoot(sessionID string) string
```

## 의존성

- `go.uber.org/zap`: 구조화 로깅
- `context`: 생애주기 관리

## 테스트

```bash
go test ./internal/core/...
go test -race ./internal/core/...
go test -cover ./internal/core/...
```

커버리지: 88.5%

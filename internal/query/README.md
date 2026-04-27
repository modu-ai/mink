# query

Agentic Query Engine을 제공합니다. 단일 대화 세션 = 단일 QueryEngine 인스턴스의 1:1 대응 런타임입니다.

## 개요

`query` 패키지는 LLM 쿼리 인터페이스 및 스트리밍 응답 처리를 담당합니다. 모든 상위 레이어(CLI, TRANSPORT, SUBAGENT)가 QueryEngine을 통해 LLM과 상호작용합니다.

## 주요 컴포넌트

### QueryEngine

`QueryEngine`은 단일 대화 세션의 agentic core 런타임입니다.

```go
type QueryEngine struct {
    cfg          QueryEngineConfig     // 엔진 설정
    mu           sync.Mutex            // SubmitMessage 직렬화 뮤텍스
    client       LLMClient             // LLM provider 클라이언트
    permResolver  PermissionResolver    // tool 사용 권한 해석기
    orchestrator StreamingOrchestrator // 응답 스트리밍 관리자
}
```

**핵심 설계 원칙**:
- **REQ-QUERY-001**: 인스턴스 하나가 대화 세션 하나에 1:1 대응
- **REQ-QUERY-002**: SubmitMessage는 동시 호출 방지 (mu 직렬화)
- **REQ-QUERY-013**: 알 수 없는 toolUseID로 ResolvePermission 시 `ErrUnknownPermissionRequest` 반환

### 주요 메서드

#### `NewQueryEngine(cfg QueryEngineConfig) (*QueryEngine, error)`

새 QueryEngine 인스턴스를 생성합니다.

```go
engine, err := query.NewQueryEngine(query.QueryEngineConfig{
    Client:      llmClient,
    PermResolver: permResolver,
})
```

#### `SubmitMessage(ctx context.Context, req LLMCallReq) (<-chan StreamingChunk, error)`

LLM 쿼리를 제출하고 스트리밍 응답 채널을 반환합니다.

- **입력**: `LLMCallReq` (메시지 역할 배열, 도구 목록, max_tokens 등)
- **출력**: `StreamingChunk` 채널 (delta 텍스트, tool call, 상태)
- **직렬화**: 동일 인스턴스에서 동시 SubmitMessage 호출 방지

```go
chunks, err := engine.SubmitMessage(ctx, llmCallReq)
if err != nil {
    return err
}

for chunk := range chunks {
    if chunk.Error != nil {
        return chunk.Error
    }
    if chunk.ContentDelta != "" {
        fmt.Print(chunk.ContentDelta)
    }
    if chunk.ToolCall != nil {
        // tool call 처리
    }
}
```

#### `ResolvePermission(toolUseID string) (PermissionDecision, error)`

tool 사용 권한을 해석합니다.

- **입력**: toolUseID (고유 식별자)
- **출력**: `PermissionDecision` (permit/deny)
- **에러**: 알 수 없는 toolUseID 시 `ErrUnknownPermissionRequest`

```go
decision, err := engine.ResolvePermission(toolUseID)
if err != nil {
    if errors.Is(err, query.ErrUnknownPermissionRequest) {
        // silent drop 금지 (REQ-QUERY-013)
        log.Warn("unknown permission request", zap.String("toolUseID", toolUseID))
        return PermissionDecision{Deny: true}, nil
    }
    return PermissionDecision{}, err
}
```

## 스트리밍 응답 처리

### StreamingChunk

```go
type StreamingChunk struct {
    ContentDelta string           // 증분 텍스트
    ToolCall     *ToolCallChunk   // tool call 청크
    Usage        *UsageChunk      // 토큰 사용량
    Error        error            // 스트리밍 에러
    Done         bool             // 완료 플래그
}
```

### 사용 예시

```go
func handleQuery(ctx context.Context, engine *query.QueryEngine, userMessage string) error {
    req := query.LLMCallReq{
        Messages: []query.Message{
            {Role: "user", Content: userMessage},
        },
        Tools: toolRegistry,
    }

    chunks, err := engine.SubmitMessage(ctx, req)
    if err != nil {
        return err
    }

    var fullResponse strings.Builder
    for chunk := range chunks {
        if chunk.Error != nil {
            return chunk.Error
        }
        if chunk.ContentDelta != "" {
            fullResponse.WriteString(chunk.ContentDelta)
            fmt.Print(chunk.ContentDelta) // 실시간 출력
        }
        if chunk.ToolCall != nil {
            // tool 실행
            if err := executeTool(chunk.ToolCall); err != nil {
                return err
            }
        }
    }

    return nil
}
```

## 크로스 패키지 인터페이스

### LLMClient 인터페이스

```go
type LLMClient interface {
    Call(ctx context.Context, req LLMCallReq) (<-chan StreamingChunk, error)
}
```

### PermissionResolver 인터페이스

```go
type PermissionResolver interface {
    ResolvePermission(toolUseID string) (PermissionDecision, error)
}
```

## @MX 태그

QueryEngine은 단일 진입점으로 fan_in >= 3이므로 `@MX:ANCHOR` 태그가 지정됩니다:

```go
// @MX:ANCHOR: [AUTO] 모든 상위 레이어의 단일 진입점 (CLI, TRANSPORT, SUBAGENT)
// @MX:REASON: REQ-QUERY-001 - 대화 세션과 1:1 대응. fan_in >= 3
type QueryEngine struct { ... }
```

## SPEC 참조

본 패키지는 **SPEC-GOOSE-QUERY-001**에 의해 정의됩니다.

- 테스트 커버리지: 95.9% (cmdctrl만)
- S0 ~ S3+까지 구현 완료

## 의존성

- `context`: 생애주기 및 취소 관리
- `sync`: 동시성 제어
- `go.uber.org/zap`: 로깅

## 테스트

```bash
# cmdctrl 패키지 테스트 (cmdctrl만 95.9%)
go test ./internal/query/cmdctrl/...
go test -race ./internal/query/cmdctrl/...
go test -cover ./internal/query/cmdctrl/...

# loop 패키지는 별도 관리
go test ./internal/query/loop/...
```

## 후속 SPEC

- **TOOLS-001**: Tool Registry 및 실행
- **RATELIMIT-001**: LLM API 속도 제한
- **PROMPT-CACHE-001**: 프롬프트 캐싱

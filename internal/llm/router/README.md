# router

LLM 요청 라우팅 결정을 내리는 라우터 패키지입니다. 대화 문맥을 분석하여 적절한 모델/파라미터를 선택합니다.

## 개요

`router` 패키지는 다중 LLM provider 환경에서 요청을 적절한 대상으로 라우팅하는 의사결정 엔진입니다. 대화 길이, 도구 사용 여부, 메타데이터를 기반으로 라우팅 결정을 내립니다.

## 주요 컴포넌트

### Router

라우팅 결정을 수행하는 핵심 인터페이스입니다.

```go
type Router interface {
    // Route는 라우팅 결정을 내립니다.
    Route(ctx context.Context, req RoutingRequest) (Route, error)
}
```

### RoutingRequest

라우팅 결정을 위한 입력입니다.

```go
type RoutingRequest struct {
    Messages            []Message    // 대화 메시지 배열
    ConversationLength  int          // 대화 턴 수 (observability용)
    HasPriorToolUse    bool         // 이전 턴에서 tool call 여부
    Meta                map[string]any // 추가 컨텍스트 정보
}
```

**REQ-ROUTER-004**: Router는 `RoutingRequest`를 읽기만 하고 변경하지 않습니다 (불변).

### Route

라우팅 결정 결과입니다.

```go
type Route struct {
    ModelID    string            // 타겟 모델 식별자 (예: "claude-3-5-sonnet-20241022")
    Parameters map[string]any   // 모델 파라미터 (max_tokens, temperature 등)
    Reason     string            // 라우팅 결정 이유 (observability용)
}
```

## 사용 예시

### 기본 라우팅

```go
import (
    "context"
    "github.com/modu-ai/goose/internal/llm/router"
)

func routeRequest(messages []router.Message) (router.Route, error) {
    r := router.NewRouter() // 기본 구현

    req := router.RoutingRequest{
        Messages:           messages,
        ConversationLength: len(messages),
        HasPriorToolUse:    false,
        Meta:               make(map[string]any),
    }

    return r.Route(context.Background(), req)
}
```

### 커스텀 라우팅 로직

```go
type CustomRouter struct {
    // 의존성 주입
}

func (r *CustomRouter) Route(ctx context.Context, req router.RoutingRequest) (router.Route, error) {
    // 1. 대화 길이 기반 라우팅
    if req.ConversationLength > 50 {
        return router.Route{
            ModelID: "claude-3-5-sonnet-20241022", // 장기 대화용
            Reason: "long_conversation",
        }, nil
    }

    // 2. tool 사용 기반 라우팅
    if req.HasPriorToolUse {
        return router.Route{
            ModelID: "claude-3-5-sonnet-20241022", // tool 호출 용이
            Reason: "tool_use_required",
        }, nil
    }

    // 3. 기본값
    return router.Route{
        ModelID: "claude-3-5-haiku-20241022", // 짧은 질의응답
        Reason: "default_short_query",
    }, nil
}
```

## 라우팅 결정 요소

### 1. 대화 길이 (ConversationLength)

- **짧은 대회** (< 10 턴): Haiku (빠른 응답)
- **중간 대화** (10-50 턴): Sonnet (균형)
- **긴 대화** (> 50 턴): Sonnet + max_tokens 증가

### 2. Tool 사용 (HasPriorToolUse)

- **tool 사용 있음**: Sonnet (도구 호환성)
- **tool 사용 없음**: Haiku 가능 (단순 질의응답)

### 3. 메타데이터 (Meta)

사용자 정의 라우팅 힌트:
```go
req.Meta["priority"] = "speed"       // 빠른 응답 우선
req.Meta["requires_reasoning"] = true // 복잡한 추론 필요
req.Meta["cost_budget"] = 1000        // 비용 제한 (토큰)
```

## Message 타입

```go
type Message struct {
    Role    string // "user" | "assistant" | "system"
    Content string // 메시지 내용
}
```

## SPEC 참조

본 패키지는 **SPEC-GOOSE-ROUTER-001**에 의해 정의됩니다.

- REQ-ROUTER-001 ~ REQ-ROUTER-004: 모든 요구사항 충족
- COMMAND-001 PR #50에서 ROUTER-001 언급
- CONTEXT-001 및 SUBAGENT-001와 연동

## 의존성

- `context`: 생애주기 관리
- `go.uber.org/zap`: 로깅

## 테스트

```bash
go test ./internal/llm/router/...
go test -race ./internal/llm/router/...
go test -cover ./internal/llm/router/...
```

## 후속 SPEC

- **PROMPT-CACHE-001**: 프롬프트 캐싱 연동
- **RATELIMIT-001**: 속도 제한 고려 라우팅

## 크로스 패키지 통합

### COMMAND-001 통합

CLI의 `/model` 명령어를 통한 모델 선택:
```go
// user가 `/model sonnet` 실행 시
req.Meta["user_override"] = "claude-3-5-sonnet-20241022"
```

### CONTEXT-001 통합

대화 컨텍스트 압축 시 라우팅 재평가:
```go
if compactionNeeded {
    req.Meta["compaction_pending"] = true
}
```

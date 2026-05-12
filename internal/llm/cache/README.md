# internal/llm/cache

**Prompt Cache 패키지** — Anthropic prompt caching 전략 수립 및 marker 적용

## 개요

본 패키지는 MINK의 **Prompt Cache** 시스템을 구현합니다. Anthropic API의 prompt caching 기능을 활용하여 반복되는 system prompt, tool 정의, 이전 대화의 토큰 사용량을 최소화합니다.

## 핵심 기능

### CachePlanner

Cache marker 배치 전략 수립:

```go
type Planner interface {
    Plan(messages []message.Message, strategy CacheStrategy, ttl time.Duration) (*CachePlan, error)
}

type CachePlan struct {
    Markers []CacheMarker // cache_control marker 위치
    TTL     time.Duration // cache 유효 기간
    Strategy CacheStrategy // 적용 전략
}
```

### Cache Strategy

```go
type CacheStrategy int

const (
    StrategySystemAnd3 CacheStrategy = iota // system + 최근 3개 메시지
    StrategySystem                          // system prompt만
    StrategyLast3                           // 최근 3개 메시지만
    StrategyAll                             // 전체
)
```

### CacheMarker

```go
type CacheMarker struct {
    MessageIndex int       // 대상 메시지 인덱스
    Type         string    // "ephemeral" (현재 Anthropic 유일 지원)
    TTL          time.Duration
}
```

## Anthropic Adapter 연계

Anthropic adapter가 `CachePlan`을 소비하여 `cache_control` 필드 적용:

```go
func applyCachePlan(messages []anthropic.Message, plan *cache.CachePlan) []anthropic.Message {
    for _, marker := range plan.Markers {
        if marker.MessageIndex < len(messages) {
            messages[marker.MessageIndex].CacheControl = &anthropic.CacheControl{
                Type: marker.Type,
            }
        }
    }
    return messages
}
```

## 파일 구조

```
internal/llm/cache/
├── planner.go         # CachePlanner 구현
├── strategy.go        # Cache 전략 정의
├── marker.go          # CacheMarker 타입
└── *_test.go          # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-PROMPT-CACHE-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-ADAPTER-001**: Anthropic adapter에서 cache plan 소비

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-PROMPT-CACHE-001

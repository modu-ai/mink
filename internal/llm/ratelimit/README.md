# internal/llm/ratelimit

**Rate Limit Tracker 패키지** — LLM Provider 응답 헤더 기반 rate limit 추적

## 개요

본 패키지는 AI.GOOSE의 **Rate Limit 추적**을 구현합니다. LLM provider 응답 헤더에서 rate limit 정보를 파싱하여, 요청 빈도 조절, 대기 시간 계산, backoff 전략을 수행합니다.

## 핵심 기능

### Tracker

Provider별 rate limit 상태 추적:

```go
type Tracker struct {
    mu      sync.RWMutex
    limits  map[string]*ProviderLimit  // provider → limit state
}

func (t *Tracker) Parse(provider string, headers http.Header, now time.Time) error
func (t *Tracker) WaitTime(provider string) time.Duration
func (t *Tracker) Remaining(provider string) int
func (t *Tracker) ResetAt(provider string) time.Time
```

### ProviderLimit

```go
type ProviderLimit struct {
    Provider      string        // provider 식별자
    RequestsLimit int           // 총 요청 한도
    Remaining     int           // 남은 요청 수
    TokensLimit   int           // 총 토큰 한도
    TokensUsed    int           // 사용된 토큰
    ResetAt       time.Time     // 한도 초기화 시간
    RetryAfter    time.Duration // 재시도 대기 시간 (429 시)
}
```

### Backoff

```go
type BackoffStrategy int

const (
    BackoffExponential BackoffStrategy = iota
    BackoffLinear
    BackoffFixed
)

func CalculateBackoff(base time.Duration, attempt int, strategy BackoffStrategy) time.Duration
```

## Provider별 헤더 파싱

| Provider | Rate Limit Header | 형식 |
|----------|-------------------|------|
| Anthropic | `x-ratelimit-requests-remaining` | 정수 |
| OpenAI | `x-ratelimit-remaining-requests` | 정수 |
| Google | N/A (SDK 내장) | - |
| Ollama | N/A (로컬) | - |

## 파일 구조

```
internal/llm/ratelimit/
├── tracker.go         # Tracker 구현
├── limit.go           # ProviderLimit 정의
├── backoff.go         # Backoff 전략
├── parser.go          # Provider별 헤더 파서
└── *_test.go          # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-RATELIMIT-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-ADAPTER-001**: Provider에서 응답 헤더 전달

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-RATELIMIT-001

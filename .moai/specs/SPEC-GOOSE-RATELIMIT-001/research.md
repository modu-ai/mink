# SPEC-GOOSE-RATELIMIT-001 — Research

> Hermes `rate_limit_tracker.py` 분석 + provider 헤더 fixture + 파서 테스트 설계.

## 1. Hermes 원형 분석 (hermes-llm.md §5 인용)

### 1.1 Python 구조 인용

```python
@dataclass
class RateLimitBucket:
    limit: int = 0
    remaining: int = 0
    reset_seconds: float = 0.0
    captured_at: float = 0.0
    
    @property
    def used(self) -> int: return max(0, self.limit - self.remaining)
    @property
    def usage_pct(self) -> float: return (self.used / self.limit) * 100.0
    @property
    def remaining_seconds_now(self) -> float:
        elapsed = time.time() - self.captured_at
        return max(0.0, self.reset_seconds - elapsed)

@dataclass
class RateLimitState:
    requests_min: RateLimitBucket
    requests_hour: RateLimitBucket
    tokens_min: RateLimitBucket
    tokens_hour: RateLimitBucket
    captured_at: float
    provider: str
```

### 1.2 수집 헤더 인용 (§5)

> **수집**: `x-ratelimit-limit-{requests,tokens}`, `x-ratelimit-remaining-*`, `x-ratelimit-reset-*` 헤더 (OpenAI, Anthropic, OpenRouter, Nous Portal 호환).

### 1.3 경고 정책 (§5)

> **경고**: 80% 사용률 도달 시 리셋 시간과 함께 표시.

## 2. Provider 헤더 fixture

### 2.1 OpenAI 예시

```
HTTP/1.1 200 OK
x-ratelimit-limit-requests: 10000
x-ratelimit-remaining-requests: 9999
x-ratelimit-reset-requests: 6s
x-ratelimit-limit-tokens: 2000000
x-ratelimit-remaining-tokens: 1999000
x-ratelimit-reset-tokens: 1s
```

Reset 포맷: Go duration ("6s", "1m30s"). 파싱:
```go
func parseDurationSeconds(s string) (float64, error) {
    d, err := time.ParseDuration(s)
    if err != nil { return 0, err }
    return d.Seconds(), nil
}
```

### 2.2 Anthropic 예시

```
HTTP/1.1 200 OK
anthropic-ratelimit-requests-limit: 50
anthropic-ratelimit-requests-remaining: 40
anthropic-ratelimit-requests-reset: 2026-04-21T12:00:34Z
anthropic-ratelimit-tokens-limit: 2000000
anthropic-ratelimit-tokens-remaining: 1500000
anthropic-ratelimit-tokens-reset: 2026-04-21T12:01:00Z
```

Reset 포맷: ISO 8601 timestamp. 파싱:
```go
t, err := time.Parse(time.RFC3339, v)
if err != nil { return 0, err }
seconds := t.Sub(now).Seconds()
if seconds < 0 { seconds = 0 }
```

### 2.3 OpenRouter 예시

OpenAI와 동일한 `x-ratelimit-*` prefix. 동일 parser 재사용 가능.

### 2.4 xAI/DeepSeek/Groq

OpenAI-compat 이므로 `OpenAIParser`로 공용. 단, Groq는 `x-ratelimit-limit-requests`를 RPM 기준으로 반환(OpenAI와 동일 단위 가정).

## 3. Bucket 유도 속성 상세

### 3.1 UsagePct

```go
func (b RateLimitBucket) UsagePct() float64 {
    if b.Limit <= 0 { return 0.0 }
    used := b.Limit - b.Remaining
    if used < 0 { used = 0 }
    return float64(used) / float64(b.Limit) * 100.0
}
```

경계:
- Limit=0 → 0.0 (zero-division 방지)
- Remaining > Limit (이론상 불가능) → used=0 → 0%

### 3.2 RemainingSecondsNow

```go
func (b RateLimitBucket) RemainingSecondsNow(now time.Time) float64 {
    elapsed := now.Sub(b.CapturedAt).Seconds()
    remaining := b.ResetSeconds - elapsed
    if remaining < 0 { remaining = 0 }
    return remaining
}
```

경계:
- CapturedAt.IsZero() → elapsed = now.Unix() (매우 큰 값) → remaining=0
- 시계 되감김 → elapsed 음수 → remaining > ResetSeconds 가능 (OK, 의도된 동작)

### 3.3 IsStale

```go
func (b RateLimitBucket) IsStale(now time.Time) bool {
    return now.Sub(b.CapturedAt).Seconds() > b.ResetSeconds
}
```

## 4. 테스트 전략

### 4.1 파서 테이블 테스트

```go
parserTests := []struct {
    name     string
    parser   Parser
    headers  map[string]string
    want     RateLimitState
    wantErr  bool
}{
    {
        name:   "openai_happy_path",
        parser: &OpenAIParser{},
        headers: map[string]string{
            "x-ratelimit-limit-requests": "10000",
            "x-ratelimit-remaining-requests": "9500",
            "x-ratelimit-reset-requests": "60s",
            "x-ratelimit-limit-tokens": "2000000",
            "x-ratelimit-remaining-tokens": "1900000",
            "x-ratelimit-reset-tokens": "120s",
        },
        want: RateLimitState{
            RequestsMin: RateLimitBucket{Limit: 10000, Remaining: 9500, ResetSeconds: 60},
            TokensMin:   RateLimitBucket{Limit: 2000000, Remaining: 1900000, ResetSeconds: 120},
        },
    },
    // Anthropic ISO 8601
    // Malformed int
    // Missing headers
    // Case insensitive lookup
    // ...
}
```

### 4.2 동시성 테스트

```go
func TestTracker_ConcurrentParse_RaceDetectorPasses(t *testing.T) {
    tracker := New(TrackerOptions{
        Parsers: map[string]Parser{"openai": &OpenAIParser{}},
    })
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            headers := map[string]string{
                "x-ratelimit-limit-requests": "1000",
                "x-ratelimit-remaining-requests": strconv.Itoa(1000 - i),
                "x-ratelimit-reset-requests": "60s",
            }
            tracker.Parse("openai", headers, time.Now())
        }(i)
    }
    wg.Wait()
    st := tracker.State("openai")
    assert.Greater(t, st.RequestsMin.Limit, 0)
}
```

`go test -race` 통과 필수.

### 4.3 이벤트 발화 스파이

```go
type observerSpy struct {
    mu     sync.Mutex
    events []Event
}

func (s *observerSpy) OnRateLimitEvent(e Event) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.events = append(s.events, e)
}
```

### 4.4 쿨다운 테스트

```go
func TestTracker_CooldownSuppressesDuplicate(t *testing.T) {
    now := time.Now()
    clock := func() time.Time { return now }
    tracker := New(TrackerOptions{
        Clock: clock,
        Parsers: ..., Observers: []Observer{spy},
        WarnCooldown: 30 * time.Second,
    })

    // 1차: 85% 사용 → event 1회
    tracker.Parse("openai", headersAt85Pct, now)
    require.Len(t, spy.events, 1)

    // 2차: 10초 후 동일 상태 → 쿨다운으로 억제
    now = now.Add(10 * time.Second)
    tracker.Parse("openai", headersAt85Pct, now)
    require.Len(t, spy.events, 1)

    // 3차: 31초 후 → 새 event
    now = now.Add(31 * time.Second)
    tracker.Parse("openai", headersAt85Pct, now)
    require.Len(t, spy.events, 2)
}
```

## 5. Display 포맷 샘플

```
openai rate limit (captured at 12:34:56):
  requests/min: 9500/10000 (95.0%)  reset in 42s  [WARN: >= 80%]
  tokens/min:   1900000/2000000 (95.0%)  reset in 108s  [WARN: >= 80%]
  requests/hr:  (not reported)
  tokens/hr:    (not reported)
```

Stale bucket은 `[STALE]` 접미사.

## 6. Go 라이브러리 결정

| 역할 | 라이브러리 | 근거 |
|-----|----------|------|
| 시간 파싱 | stdlib `time` | RFC3339 + Duration |
| 로깅 | `go.uber.org/zap` | CORE-001 상속 |
| 테스트 | `stretchr/testify` | |

**외부 라이브러리 추가 없음**. stdlib + zap만.

## 7. 오픈 이슈

- **Q1**: Hour bucket은 OpenAI가 미제공. Anthropic도 `-daily-*`는 있지만 hourly 없음. 현 설계는 4 bucket 모두 정의하되 데이터 없는 bucket은 zero-value 유지.
- **Q2**: 로깅 fmt에 색상 적용? CLI-001의 관심사. 본 SPEC은 plain string.
- **Q3**: tracker state를 디스크에 persist? Phase 1 미포함. goosed 재기동 시 첫 API 호출로 재채움.

## 8. 구현 순서 (TDD)

| # | Test | Focus |
|---|---|---|
| 1 | TestBucket_UsagePct_ZeroLimit | 경계 |
| 2 | TestOpenAIParser_HappyPath | 기본 |
| 3 | TestTracker_ThresholdEventEmittedOnce | 이벤트 |
| 4 | TestTracker_CooldownSuppressesDuplicate | 쿨다운 |
| 5 | TestAnthropicParser_ISO8601ResetNormalized | ISO 8601 |
| 6 | TestParser_MalformedHeader_ZeroValue | graceful |
| 7 | TestBucket_StaleDetection | stale |
| 8 | TestTracker_ConcurrentParse_RaceDetectorPasses | 동시성 |

---

**End of Research (SPEC-GOOSE-RATELIMIT-001)**

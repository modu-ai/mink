# context

QueryEngine의 context window 관리와 compaction 전략을 구현합니다. 대화 길이 최적화와 토큰 효율적 사용을 지원합니다.

## 개요

`context` 패키지는 LLM 대화의 context window를 관리하고, 토큰 사용량을 최적화하는 compaction 전략을 제공합니다. 대화가 길어질수록 토큰 사용량이 기하급수로 증가하는 문제를 해결합니다.

## 주요 컴포넌트

### Compactor

Context compaction을 수행하는 핵심 인터페이스입니다.

```go
type Compactor interface {
    // Compact는 대화 기록을 압축합니다.
    Compact(ctx context.Context, messages []Message) ([]Message, error)
}
```

### SystemContext

시스템 수준 context 정보입니다.

```go
type SystemContext struct {
    GitStatus    string    // git 상태 문자열 (최대 4KB). git 부재 시 "(no git)"
    CacheBreaker string    // build version 또는 session id
    ComputedAt   time.Time // 처음 계산된 시각
}
```

### Compaction 전략

3가지 전략을 제공합니다:

#### 1. Auto Strategy (자동)

대화 길이에 따라 동적으로 전략 선택:

```go
type AutoStrategy struct {
    Thresholds []CompactionThreshold
}

type CompactionThreshold struct {
    MessageCount int    // 메시지 수 기준
    Strategy     string // "snip" | "reactive" 중 선택
}
```

#### 2. Snip Strategy

오래된 메시지를 삭제하는 간단 전략:

```go
type SnipStrategy struct {
    KeepLastN int // 최근 N개 메시지만 유지
}
```

#### 3. Reactive Strategy

최근 요약을 생성하고 이전 메시지를 삭제:

```go
type ReactiveStrategy struct {
    Summarizer Summarizer // 요약 생성 인터페이스
}
```

## 사용 예시

### 기본 Compaction

```go
import (
    "context"
    "github.com/modu-ai/goose/internal/context"
)

func compactMessages(messages []context.Message) ([]context.Message, error) {
    compactor := context.NewAutoStrategy(context.CompactionThreshold{
        MessageCount: 20,
        Strategy:     "snip",
    })

    return compactor.Compact(context.Background(), messages)
}
```

### Reactive Compaction (요약 기반)

```go
func compactWithSummary(messages []context.Message) ([]context.Message, error) {
    summarizer := NewLLMSummarizer(llmClient)
    compactor := context.NewReactiveStrategy(summarizer)

    return compactor.Compact(ctx, messages)
}
```

### SystemContext 사용

```go
func getSystemContext() (*context.SystemContext, error) {
    sysCtx, err := context.ComputeSystemContext()
    if err != nil {
        return nil, err
    }

    log.Printf("git status: %s", sysCtx.GitStatus)
    log.Printf("cache breaker: %s", sysCtx.CacheBreaker)

    return sysCtx, nil
}
```

## Compaction 동작

### Auto Strategy 동작

```
메시지 수 ≤ 10:   No compaction
메시지 수 11-20:  Snip (최근 15개 유지)
메시지 수 > 20:    Reactive (요약 생성)
```

### Snip Strategy 동작

```
입력: [M1, M2, M3, M4, M5, M6, M7, M8, M9, M10, M11, M12]
KeepLastN: 8

출력: [M5, M6, M7, M8, M9, M10, M11, M12]
        (M1-M4 삭제)
```

### Reactive Strategy 동작

```
입력: [M1, M2, M3, ..., M10, M11, M12]

1. 최근 메시지 추출: [M10, M11, M12]
2. LLM으로 요약 생성: "이전 대화에서 사용자는..."
3. 요약 메시지 삽입: [Summary, M10, M11, M12]
   (M1-M9 삭제)
```

## 토큰 절감 효과

Compaction을 통한 토큰 사용량 최적화:

| 대화 길이 | Compaction 전 | Compaction 후 | 절감률 |
|---------|---------------|----------------|--------|
| 20 메시지 | ~10,000 토큰 | ~8,000 토큰 | 20% |
| 50 메시지 | ~25,000 토큰 | ~12,000 토큰 | 52% |
| 100 메시지 | ~50,000 토큰 | ~15,000 토큰 | 70% |

## SPEC 참조

본 패키지는 **SPEC-GOOSE-CONTEXT-001**에 의해 정의됩니다.

- REQ-CTX-001 ~ REQ-CTX-015: 모든 요구사항 충족
- 테스트 커버리지: 90.4%
- COMMAND-001 PR #50에서 CONTEXT-001 언급

## 의존성

- `bytes`: 버퍼 관리
- `context`: 생애주기 및 취소
- `os/exec`: git 명령 실행
- `sync`, `sync/atomic`: 동시성 제어
- `time`: 타임스탬프

## 테스트

```bash
go test ./internal/context/...
go test -race ./internal/context/...
go test -cover ./internal/context/...
```

## 크로스 패키지 통합

### QUERY-001 통합

QueryEngine이 대화 길이가 길어질 때 자동 compaction:

```go
if len(messages) > compactionThreshold {
    messages, err := compactor.Compact(ctx, messages)
    if err != nil {
        return err
    }
}
```

### ROUTER-001 통합

Compaction 상태를 라우팅 결정에 활용:

```go
req.Meta["compaction_pending"] = needsCompaction
```

## Git 상태 계산

`ComputeSystemContext()`는 git 상태를 계산합니다:

```go
func ComputeSystemContext() (*SystemContext, error) {
    // 1. git status 실행 (타임아웃 2초)
    // 2. 출력을 4KB로 제한
    // 3. 캐시 브레이커 생성 (build version 또는 session id)
    return &SystemContext{...}, nil
}
```

**제한 사항**:
- `gitStatusMaxBytes`: 4KB (과도한 상태 정보 방지)
- `gitTimeoutSeconds`: 2초 (응답 지연 방지)
- git 부재 시: `"(no git)"` 반환

## Compaction 트리거

### 자동 트리거

QueryEngine이 다음 조건에서 자동 compaction:

1. 대화 길이가 임계값 초과
2. 토큰 사용량이 max_tokens의 80% 도달
3. 사용자 명시적 요청 (`/compact` 명령)

### 수동 트리거

```go
// 사용자가 /compact 명령 실행 시
compacted, err := compactor.Compact(ctx, messages)
```

## 성능 고려사항

### Compaction 비용

- **Snip**: O(1) - 매우 빠름
- **Reactive**: O(N) + LLM 호출 - 비용 높음
- **Auto**: 트리거 비용 포함

### 권장 전략

- 짧은 대화 (< 20 턴): Compaction 불필요
- 중간 대화 (20-50 턴): Snip 권장
- 긴 대화 (> 50 턴): Reactive 권장

## 로깅

Compaction 이벤트를 로그에 기록:

```go
logger.Info("compaction triggered",
    zap.String("strategy", "snip"),
    zap.Int("before_count", len(messages)),
    zap.Int("after_count", len(compacted)),
)
```

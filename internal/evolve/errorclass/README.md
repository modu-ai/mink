# internal/evolve/errorclass

**Error Classification 패키지** — 구조화된 에러 분류 및 계층화

## 개요

본 패키지는 AI.GOOSE의 **에러 분류 시스템**을 구현합니다. 런타임 에러를 체계적으로 분류하여, retry 가능 여부, 사용자 노출 여부, 로깅 레벨을 자동 결정합니다.

## 핵심 기능

### ErrorClass

에러 분류 열거형:

```go
type ErrorClass int

const (
    ClassTransient    ErrorClass = iota // 일시적 (retry 가능)
    ClassPermanent                      // 영구적 (retry 불가)
    ClassAuth                           // 인증 오류
    ClassRateLimit                      // Rate limit 초과
    ClassValidation                     // 입력 검증 실패
    ClassTimeout                        // 타임아웃
    ClassNetwork                        // 네트워크 오류
    ClassInternal                       // 내부 로직 오류
)
```

### ClassifiedError

분류된 에러:

```go
type ClassifiedError struct {
    Class       ErrorClass
    Original    error
    Retryable   bool
    UserMessage string    // 사용자에게 노출할 메시지
    StatusCode  int       // HTTP status code (해당 시)
    Provider    string    // 관련 provider
    Metadata    map[string]string
}
```

### Classifier

에러 자동 분류:

```go
type Classifier struct {
    rules []ClassificationRule
}

func (c *Classifier) Classify(err error) *ClassifiedError
```

분류 규칙 예시:
- HTTP 429 → ClassRateLimit, Retryable=true
- HTTP 401 → ClassAuth, Retryable=false
- `context.DeadlineExceeded` → ClassTimeout, Retryable=true
- `net.OpError` → ClassNetwork, Retryable=true

## 파일 구조

```
internal/evolve/errorclass/
├── class.go           # ErrorClass 열거형
├── error.go           # ClassifiedError 구조체
├── classifier.go      # Classifier 구현
├── rules.go           # 분류 규칙
└── *_test.go          # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-ERROR-CLASS-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-ADAPTER-001**: Provider 에러 분류
- **SPEC-GOOSE-QUERY-001**: Query loop 에러 처리

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-ERROR-CLASS-001

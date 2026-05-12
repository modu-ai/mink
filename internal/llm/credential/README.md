# internal/llm/credential

**Credential Pool 패키지** — LLM Provider 자격 증명 풀 관리 및 자동 갱신

## 개요

본 패키지는 MINK의 **Credential Pool**을 구현합니다. 다중 API key/OAuth token을 풀로 관리하며, rate limit 도달 시 자동 rotate, 만료 임박 시 자동 refresh, credential 전환 시 무중단 처리를 제공합니다.

## 핵심 기능

### CredentialPool

```go
type Pool struct {
    mu         sync.RWMutex
    entries    map[string]*Entry  // id → credential entry
    refresher  Refresher          // OAuth refresh 구현
    strategy   SelectionStrategy  // 선택 전략
}

func (p *Pool) Select(ctx context.Context, strategy Strategy) (*Credential, error)
func (p *Pool) MarkExhaustedAndRotate(id string, statusCode int, retryAfter time.Duration) error
func (p *Pool) Refresh(ctx context.Context, id string) (*RefreshResult, error)
```

### Selection Strategy

```go
type Strategy int

const (
    StrategyRoundRobin Strategy = iota // 순차 선택
    StrategyRandom                      // 무작위 선택
    StrategyLeastUsed                   // 가장 적게 사용된 것
)
```

### Credential Entry

```go
type Entry struct {
    ID           string          // 고유 식별자
    Provider     string          // provider 이름
    Type         CredentialType  // api_key, oauth, bearer
    Token        string          // access token / API key
    RefreshToken string          // OAuth refresh token (optional)
    ExpiresAt    time.Time       // 만료 시간
    Metadata     map[string]string
}
```

### Refresher Interface

OAuth 자동 갱신:

```go
type Refresher interface {
    Refresh(ctx context.Context, cred *Entry) (*RefreshResult, error)
}

type RefreshResult struct {
    AccessToken  string    // 새 access token
    RefreshToken string    // rotated refresh token
    ExpiresAt    time.Time // 새 만료 시간
}
```

## 자격 증명 소스

| 소스 | 경로 | 설명 |
|------|------|------|
| API Key | 환경변수, `.env` | 정적 키 |
| OAuth | `~/.goose/credentials/` | PKCE 기반 자동 갱신 |
| Claude Sync | `~/.claude/.credentials.json` | Anthropic 토큰 동기화 |

## 파일 구조

```
internal/llm/credential/
├── pool.go            # CredentialPool 구현
├── source.go          # 자격 증명 소스 (환경변수, 파일)
├── entry.go           # Credential Entry 정의
├── strategy.go        # 선택 전략
├── refresh.go         # 자동 갱신 로직
└── *_test.go          # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-CREDPOOL-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-ADAPTER-001**: Provider에서 credential pool 사용
- **SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001**: OnModelChange 후 credential pool swap wiring

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-CREDPOOL-001

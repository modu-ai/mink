---
id: SPEC-GOOSE-CREDPOOL-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-CREDPOOL-001 — Credential Pool (OAuth/API 통합, 4 Strategy, Rotation)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §1-3 + ROADMAP v2.0 Phase 1 기반) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT Phase 1의 **Multi-LLM Infrastructure 진입점**을 정의한다. Hermes Agent의 `credential_pool.py`(~1.3K LoC) 원형을 Go로 포팅하여, OAuth 토큰과 API key 자격을 하나의 풀에 통합하고, 선택 전략(4종)·자동 갱신·고갈 쿨다운·소프트 임대·외부 디스크 동기화를 제공하는 `internal/llm/credential` 패키지를 구현한다.

본 SPEC이 Plan·Run을 통과한 시점에서 `CredentialPool`은:

- `Select(ctx, strategy)`를 호출하면 즉시 사용 가능한 `PooledCredential` 하나를 반환(또는 풀 전체 소진 시 `ErrExhausted`)하고,
- OAuth 엔트리가 `expires_at` 임박 시 자동 `Refresh()`를 수행(PKCE single-use refresh token rotation 포함)하고,
- HTTP 429/402 등 고갈 응답을 받은 호출자가 `MarkExhaustedAndRotate(id, statusCode)`를 호출하면 해당 엔트리에 1시간 쿨다운을 설정하고 다음 가용 엔트리로 전환하며,
- 동시성 제어를 위한 `AcquireLease(id) / ReleaseLease(id)`(soft lease)를 노출하고,
- `~/.claude/.credentials.json`, `~/.codex/auth.json`, `~/.hermes/auth.json` 파일을 감지하면 해당 엔트리를 풀에 자동 주입하며 변경 시 반영한다.

본 SPEC은 위 동작의 **인터페이스 계약과 관찰 가능 행동**을 규정한다. Provider별 HTTP 호출은 ADAPTER-001, Rate limit 헤더 해석은 RATELIMIT-001, Smart routing은 ROUTER-001로 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 06은 CREDPOOL-001을 `CONFIG-001` 이후의 **Multi-LLM 인프라 최초 SPEC**으로 명시한다. 후속 SPEC(ROUTER-001, RATELIMIT-001, ADAPTER-001)이 모두 "풀에서 자격을 1건 취득한다"는 계약 위에 세워져 있다.
- `.moai/project/research/hermes-llm.md` §1-3은 Hermes `credential_pool.py`의 구조(4 strategy + availability filter + refresh + exhaustion cooldown + soft lease + persistent JSON state)를 Go 포팅 매핑(§9)과 함께 제시한다.
- v4.0 포지셔닝 원칙("모든 LLM을 API 또는 OAuth로 연결")의 **유일한 자격 단일 진입점**을 제공한다. Phase 2~6의 모든 LLM 의존 모듈이 본 풀만 바라보게 만드는 것이 본 SPEC의 책임이다.

### 2.2 상속 자산 (패턴 계승)

- **Hermes Agent Python** (`./hermes-agent-main/agent/credential_pool.py` 1.3K LoC): `CredentialPool`, `PooledCredential`, `SelectionStrategy`, `_refresh_entry`, `_available_entries`, `acquire_lease`. Python asyncio → Go `context.Context`+mutex 재해석.
- **Anthropic Claude Code credential storage**: `~/.claude/.credentials.json` 스키마 (OAuth PKCE 2.1 + single-use refresh token).
- **OpenAI Codex storage**: `~/.codex/auth.json` (OAuth with refresh rotation).
- **Nous Portal**: `~/.hermes/auth.json` (Agent Key TTL).
- **MoAI-ADK-Go**: 본 레포에 미러 없음. 패턴 참고 전무.

### 2.3 범위 경계

- **IN**: `PooledCredential` 구조체 + 상태 머신(OK/EXHAUSTED/PENDING), `CredentialPool` 선택·갱신·회전·임대 API, 4 `SelectionStrategy`, OAuth 자동 갱신 훅(실제 HTTP는 ADAPTER-001), 외부 디스크 파일 동기화(`CredentialSource` interface), 영속 JSON 상태 저장소, 동시성 안전성, 고갈 쿨다운 정책.
- **OUT**: 실제 OAuth HTTP 요청(토큰 엔드포인트 POST — ADAPTER-001/Anthropic/OpenAI 구현), PKCE 로그인 플로우 UI(CLI-001), 429 헤더 파싱(RATELIMIT-001), 모델 선택(ROUTER-001), 프롬프트 캐시 마커(PROMPT-CACHE-001), 사용량 가격 계산(후속 메트릭 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/llm/credential/` 패키지: `PooledCredential`, `CredentialPool`, `Lease`, `SelectionStrategy`, `ExhaustionPolicy`, `Storage`, `CredentialSource`.
2. 선택 전략 4종 — `FillFirst`(우선순위), `RoundRobin`(공평), `Random`(부하 분산), `LeastUsed`(사용 빈도).
3. 가용성 필터링: `status == OK` AND `last_error_reset_at <= now` AND (OAuth의 경우) `expires_at > now + refresh_margin`.
4. OAuth 자동 갱신 훅: `Refresher` 인터페이스를 통해 provider별 HTTP refresh 구현을 주입받고, `expires_at - refreshMargin < now`일 때 호출.
5. 고갈 처리: HTTP 429 → 서버 제공 `Retry-After` 또는 기본 1시간 쿨다운 기록. HTTP 402 → 영구 고갈(운영자 개입 대기).
6. 회전: `MarkExhaustedAndRotate(id, statusCode, ctx)` 호출 시 (a) 해당 엔트리 상태를 `EXHAUSTED` 전환, (b) 다음 가용 엔트리 선택, (c) 반환.
7. 소프트 임대(`AcquireLease/ReleaseLease`): 동일 엔트리가 동시에 N개 요청에 쓰여도 API 호출은 허용하되, 엔트리 단위 사용량(`request_count`, `active_leases`)을 추적한다. 하드 락이 아니므로 복수 임대가 허용됨(하지만 `LeastUsed` 전략이 이를 참조).
8. 영속 상태: `~/.goose/credentials/<provider>.json`에 `last_status`, `last_status_at`, `last_error_code`, `last_error_reset_at`, `request_count`, `expires_at_ms` 저장. `Load()` 시 복원, `MarkExhausted/Refresh` 시 atomic write(tmp + rename).
9. 외부 소스 동기화: `CredentialSource` 인터페이스 구현 3종 — `AnthropicClaudeSource`(`~/.claude/.credentials.json`), `OpenAICodexSource`(`~/.codex/auth.json`), `NousSource`(`~/.hermes/auth.json`). 파일 mtime 감지를 통해 load-only(폴링 없음, 명시적 `Refresh()` 호출 시점에만 재검사).
10. 관측성: `PoolStats{Total, Available, Exhausted, Pending, RefreshesTotal, RotationsTotal}` 노출.
11. 에러 타입: `ErrExhausted`(풀 전체 소진), `ErrRefreshFailed`, `ErrLeaseUnavailable`, `ErrInvalidEntry`.
12. `CONFIG-001`의 `LLMConfig.Providers[*].Credentials[]`를 읽어 초기 풀 구성.

### 3.2 OUT OF SCOPE (명시적 제외)

- **OAuth HTTP refresh 실 구현**: `Refresher.Refresh(ctx, entry)` 인터페이스만 정의. Anthropic PKCE·OpenAI token rotation 구현은 ADAPTER-001.
- **PKCE 로그인 UI/브라우저 리다이렉트**: CLI-001 + ADAPTER-001 조합.
- **Rate limit 헤더 파싱**: 호출자가 RATELIMIT-001 결과(statusCode + resetSeconds)를 `MarkExhaustedAndRotate`에 전달. 본 풀은 헤더를 직접 읽지 않음.
- **모델 라우팅/비용 기반 선택**: ROUTER-001 결정을 풀은 소비만 한다(선택 후보 필터링은 provider 단위까지).
- **GitHub/Google Workspace OAuth**: 본 Phase는 LLM provider 3종(Anthropic, OpenAI Codex, Nous)만. 다른 provider는 후속 SPEC.
- **키체인/1Password/Vault 연동**: 본 SPEC은 평문 JSON 파일만. 보안 저장소 통합은 후속.
- **크로스 프로세스 락**: 단일 `goosed` 프로세스 가정. 다중 프로세스 락(flock)은 후속.
- **Hot reload/fsnotify**: `CONFIG-001`의 방침과 동일하게 load-once + 명시적 Refresh.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CREDPOOL-001 [Ubiquitous]** — The `CredentialPool` **shall** guarantee that every call to `Select()` returns either (a) a `*PooledCredential` whose `status == OK` and (for OAuth) `expires_at > now`, or (b) `ErrExhausted` if no such entry exists; **shall not** return an entry in `EXHAUSTED` or `PENDING` state.

**REQ-CREDPOOL-002 [Ubiquitous]** — All mutations to `PooledCredential.status`, `last_error_reset_at`, and `request_count` **shall** go through `CredentialPool` public methods; external callers **shall not** mutate credential fields directly.

**REQ-CREDPOOL-003 [Ubiquitous]** — The `CredentialPool` **shall** be safe for concurrent `Select`/`MarkExhaustedAndRotate`/`AcquireLease` calls from multiple goroutines; internal synchronization uses `sync.Mutex` scoped to the pool instance.

**REQ-CREDPOOL-004 [Ubiquitous]** — Every mutation **shall** be immediately persisted to `~/.goose/credentials/<provider>.json` via atomic write (temp file + rename) before the public method returns.

### 4.2 Event-Driven (이벤트 기반)

**REQ-CREDPOOL-005 [Event-Driven]** — **When** `Select(ctx, strategy)` is invoked, the pool **shall** (a) snapshot its entries, (b) compute the available subset per §3.1 rule 3, (c) trigger `Refresher.Refresh()` on entries whose `expires_at - refreshMargin < now`, (d) apply the requested `SelectionStrategy` to the available subset, and (e) return the chosen entry within 100ms (excluding refresh RTT).

**REQ-CREDPOOL-006 [Event-Driven]** — **When** `MarkExhaustedAndRotate(id, statusCode, retryAfterSeconds)` is invoked with `statusCode == 429`, the pool **shall** (a) set that entry's `status = EXHAUSTED`, (b) set `last_error_reset_at = now + max(retryAfterSeconds, defaultCooldown)` where `defaultCooldown = 1h`, (c) persist, and (d) select the next available entry per the pool's default strategy or return `ErrExhausted`.

**REQ-CREDPOOL-007 [Event-Driven]** — **When** `MarkExhaustedAndRotate` is invoked with `statusCode == 402`, the pool **shall** set the entry to a permanent exhausted state (`last_error_reset_at = far-future sentinel`) and log at ERROR level; subsequent `Refresh()` on this entry is a no-op until operator intervention.

**REQ-CREDPOOL-008 [Event-Driven]** — **When** an external credential source file (e.g., `~/.claude/.credentials.json`) is modified and `Pool.RefreshSources(ctx)` is called, the pool **shall** reconcile its entries with the file: new entries are appended, deleted entries are removed, mutated entries update their tokens without resetting `request_count`.

**REQ-CREDPOOL-009 [Event-Driven]** — **When** `AcquireLease(id)` is called, the pool **shall** increment `entry.active_leases` and return a `*Lease` whose `Release()` decrements the counter; if `id` is not found, returns `ErrInvalidEntry`.

### 4.3 State-Driven (상태 기반)

**REQ-CREDPOOL-010 [State-Driven]** — **While** an OAuth entry's `expires_at - refreshMargin < now AND expires_at > now`, the next `Select()` that would return this entry **shall** first invoke `Refresher.Refresh(ctx, entry)`; upon refresh success, update `access_token`, `refresh_token`(if rotated), and `expires_at`; upon refresh failure, mark the entry as `EXHAUSTED` with a 5-minute cooldown and try the next entry.

**REQ-CREDPOOL-011 [State-Driven]** — **While** the pool's `defaultStrategy == LeastUsed`, the `Select()` ordering **shall** prefer entries with the lowest `active_leases` first, then the lowest `request_count` as tiebreaker.

**REQ-CREDPOOL-012 [State-Driven]** — **While** all entries are in `EXHAUSTED` state with future `last_error_reset_at`, `Select()` **shall** return `ErrExhausted` with `RetryAfter` populated to the earliest reset time.

### 4.4 Unwanted Behavior (방지)

**REQ-CREDPOOL-013 [Unwanted]** — **If** `Refresher.Refresh` returns an error for the same entry **3 times consecutively**, the pool **shall** mark the entry as permanently exhausted (`last_error_code = -1`) and emit a WARN log; **shall not** retry this entry in subsequent `Select()` calls until operator intervention (e.g., `Pool.Reset(id)`).

**REQ-CREDPOOL-014 [Unwanted]** — The pool **shall not** write credential fields to any log line; `access_token`, `refresh_token`, `api_key` must be redacted (`"sk-***"`) in every log emission. `PooledCredential.String()` **shall** return a redacted form.

**REQ-CREDPOOL-015 [Unwanted]** — The pool **shall not** block on refresh for entries other than the one currently being considered; specifically, `Select()` **shall not** preemptively refresh all entries in the background.

**REQ-CREDPOOL-016 [Unwanted]** — **If** two concurrent `Select()` calls race and both observe the same entry as the best candidate, both **shall** succeed (returning the same entry pointer); hard-lease enforcement is out of scope — callers rely on provider rate limit feedback via `MarkExhaustedAndRotate`.

### 4.5 Optional (선택적)

**REQ-CREDPOOL-017 [Optional]** — **Where** `CredentialSource.Sync(ctx)` is invoked with a non-nil `context.Context`, the source **shall** respect `ctx.Done()` and abort partial file reads within 500ms.

**REQ-CREDPOOL-018 [Optional]** — **Where** `PoolOptions.RefreshMargin` is configured, OAuth entries **shall** be refreshed when `expires_at - RefreshMargin < now`; default is 5 minutes.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-CREDPOOL-001 — 기본 선택(FillFirst) 및 우선순위**
- **Given** 3개 엔트리 `[{id:"a", priority:10}, {id:"b", priority:5}, {id:"c", priority:1}]`, 모두 `OK` 상태
- **When** `Select(ctx, FillFirst)` 호출
- **Then** 반환된 엔트리의 `id == "a"` (최고 priority), `entry.request_count == 1`

**AC-CREDPOOL-002 — RoundRobin 전략**
- **Given** 3개 엔트리 모두 OK, 전략 `RoundRobin`
- **When** 연속 4회 `Select` 호출
- **Then** 반환 순서는 `[a, b, c, a]` (wrap-around)

**AC-CREDPOOL-003 — OAuth 자동 갱신**
- **Given** 엔트리 `e1`의 `expires_at == now + 2분`, `refreshMargin == 5분`. 스텁 `Refresher.Refresh`가 새 토큰 `"new_token"`과 `expires_at = now + 1h`를 반환
- **When** `Select(ctx, FillFirst)`
- **Then** 반환된 엔트리의 `access_token == "new_token"`, `expires_at > now + 30분`, `Refresher.Refresh`는 1회만 호출됨

**AC-CREDPOOL-004 — 429 고갈 후 회전**
- **Given** 2개 엔트리 `[a, b]` 모두 OK, `a`가 선택된 후 외부 호출이 HTTP 429(retry_after=60)를 반환
- **When** `MarkExhaustedAndRotate("a", 429, 60)` 호출
- **Then** (1) `a.status == EXHAUSTED`, `a.last_error_reset_at == now + 60s`, (2) 반환값은 `b`이고 `b.status == OK`, (3) `~/.goose/credentials/<provider>.json`에 `a`의 상태가 persisted됨

**AC-CREDPOOL-005 — 풀 전체 소진**
- **Given** 모든 엔트리가 `EXHAUSTED`, `last_error_reset_at = now + 30s`
- **When** `Select`
- **Then** 반환은 `nil, ErrExhausted`이고, `ErrExhausted.RetryAfter == 30s`

**AC-CREDPOOL-006 — 재기동 시 영속 상태 복원**
- **Given** 엔트리 `a`가 `EXHAUSTED`, `last_error_reset_at = now + 5분`으로 저장된 상태에서 풀을 파괴 후 새 풀 생성
- **When** 새 풀이 `Load()`
- **Then** `a.status == EXHAUSTED`, `last_error_reset_at` 복원, `Select()` 시 `a`는 필터 아웃됨

**AC-CREDPOOL-007 — 외부 파일 소스 동기화**
- **Given** `~/.claude/.credentials.json`이 엔트리 `claude-1`을 포함. 풀 초기화 후 파일 수정으로 `claude-2` 추가
- **When** `Pool.RefreshSources(ctx)`
- **Then** 풀의 `Stats().Total`이 1 증가하고 `claude-2`가 `Select` 후보에 포함됨. 기존 `claude-1`의 `request_count`는 보존

**AC-CREDPOOL-008 — Soft lease 집계**
- **Given** 엔트리 `a` OK, 전략 `LeastUsed`, 두 번째 엔트리 `b`도 OK
- **When** `AcquireLease("a")` 3회 후 `Select(LeastUsed)`
- **Then** 반환은 `b` (active_leases 0 vs a의 3)

**AC-CREDPOOL-009 — Refresh 3회 연속 실패 → 영구 고갈**
- **Given** 엔트리 `e1`의 `expires_at`이 임박, `Refresher.Refresh`가 항상 에러 반환
- **When** 3회 `Select` 시도
- **Then** 3회 모두 내부 refresh 시도가 발생하고, 3회 이후 `e1.status == EXHAUSTED`, `last_error_code == -1`, WARN 로그 1건. 이후 `Select`는 다른 엔트리 또는 `ErrExhausted`.

**AC-CREDPOOL-010 — 비밀값 로그 마스킹**
- **Given** zap logger에 debug 레벨 훅
- **When** 모든 공개 메서드 호출 후 로그 수집
- **Then** `access_token`, `refresh_token`, `api_key` 원문이 로그에 등장하지 않음. `String()` 결과에 `"sk-***"` 형태.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/credential/
├── pool.go                  # CredentialPool + Select + MarkExhaustedAndRotate
├── pool_test.go             # Unit tests (20+ cases)
├── credential.go            # PooledCredential + Status + String(redacted)
├── strategy.go              # SelectionStrategy enum + 4 구현
├── lease.go                 # Lease + AcquireLease/ReleaseLease
├── oauth.go                 # Refresher interface + refresh margin policy
├── storage.go               # Storage interface + JSON file backend (atomic write)
├── source.go                # CredentialSource interface + 3 구현
├── source_anthropic.go      # ~/.claude/.credentials.json reader
├── source_openai.go         # ~/.codex/auth.json reader
├── source_nous.go           # ~/.hermes/auth.json reader
├── errors.go                # ErrExhausted + ErrRefreshFailed + ErrInvalidEntry
└── stats.go                 # PoolStats
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/llm/credential/credential.go

// Status는 풀 엔트리의 현재 상태.
type Status string

const (
    StatusOK        Status = "ok"        // 즉시 사용 가능
    StatusExhausted Status = "exhausted" // 고갈 (쿨다운 중)
    StatusPending   Status = "pending"   // 갱신 진행 중
)

// AuthType은 엔트리의 인증 방식.
type AuthType string

const (
    AuthOAuth  AuthType = "oauth"
    AuthAPIKey AuthType = "api_key"
)

// PooledCredential은 단일 자격 엔트리.
// 필드는 Pool의 공개 메서드를 통해서만 mutate됨(REQ-CREDPOOL-002).
type PooledCredential struct {
    Provider          string
    ID                string // UUID hex[:6]
    Label             string
    AuthType          AuthType
    Priority          int
    Source            string // "config" | "anthropic_claude" | "openai_codex" | "nous"

    // 비밀값 — String() / MarshalJSON에서 redact.
    accessToken  string
    refreshToken string
    apiKey       string
    agentKey     string // Nous-specific

    // 상태.
    Status            Status
    LastStatusAt      time.Time
    LastErrorCode     int
    LastErrorResetAt  time.Time
    ExpiresAt         time.Time
    BaseURL           string
    RequestCount      int64
    ActiveLeases      int32

    // Refresh 연속 실패 카운터.
    refreshFailCount  int
}

// RuntimeAPIKey는 호출자가 실제 HTTP 헤더에 쓸 토큰.
// agentKey 우선, 없으면 accessToken, 없으면 apiKey.
func (c *PooledCredential) RuntimeAPIKey() string

// String은 비밀값을 redact한 표현을 반환(REQ-CREDPOOL-014).
func (c *PooledCredential) String() string
```

```go
// internal/llm/credential/strategy.go

type SelectionStrategy int

const (
    FillFirst SelectionStrategy = iota // 최고 priority 엔트리 우선
    RoundRobin                         // 마지막 선택 인덱스 + 1
    Random                             // crypto/rand
    LeastUsed                          // min(active_leases), tiebreaker min(request_count)
)
```

```go
// internal/llm/credential/oauth.go

// Refresher는 provider별 OAuth refresh 구현 주입점.
// ADAPTER-001이 provider별 구현체 제공.
type Refresher interface {
    // Refresh는 refreshToken을 사용해 새 토큰을 얻는다.
    // 반환값의 AccessToken/RefreshToken/ExpiresAt은 기존 entry에 덮어쓰여진다.
    Refresh(ctx context.Context, entry *PooledCredential) (RefreshResult, error)
}

type RefreshResult struct {
    AccessToken  string
    RefreshToken string // rotated or same
    ExpiresAt    time.Time
    Extra        map[string]any
}

// RefreshMargin: expires_at - margin < now → refresh 필요.
// 기본값 5분 (REQ-CREDPOOL-018).
```

```go
// internal/llm/credential/pool.go

type PoolOptions struct {
    Provider         string
    DefaultStrategy  SelectionStrategy
    Refresher        Refresher         // nullable: API key-only 풀은 nil
    Storage          Storage           // nullable: 영속 비활성화
    Sources          []CredentialSource // 외부 파일 소스
    RefreshMargin    time.Duration     // 기본 5분
    DefaultCooldown  time.Duration     // 기본 1시간
    Logger           *zap.Logger
    Clock            func() time.Time  // testability; 기본 time.Now
}

type CredentialPool struct {
    opts    PoolOptions
    mu      sync.Mutex
    entries []*PooledCredential
    rrIdx   int // RoundRobin 인덱스
    stats   PoolStats
}

func New(ctx context.Context, opts PoolOptions, initial []*PooledCredential) (*CredentialPool, error)

// Select는 전략에 따라 가용 엔트리 1건을 반환.
func (p *CredentialPool) Select(
    ctx context.Context,
    strategy SelectionStrategy, // 전략 override; FillFirst 기본
) (*PooledCredential, error)

// MarkExhaustedAndRotate은 호출자가 HTTP 에러를 수신한 뒤 호출.
func (p *CredentialPool) MarkExhaustedAndRotate(
    ctx context.Context,
    id string,
    statusCode int,
    retryAfter time.Duration,
) (*PooledCredential, error) // 다음 가용 엔트리

// AcquireLease는 soft lease 발급.
func (p *CredentialPool) AcquireLease(id string) (*Lease, error)

// RefreshSources는 CredentialSource들을 재호출하여 풀을 동기화.
func (p *CredentialPool) RefreshSources(ctx context.Context) error

// Stats는 현재 통계 스냅샷.
func (p *CredentialPool) Stats() PoolStats

// Reset은 특정 엔트리의 영구 고갈을 해제(운영자 수동).
func (p *CredentialPool) Reset(id string) error
```

```go
// internal/llm/credential/source.go

// CredentialSource는 외부 파일/키체인에서 엔트리를 읽어오는 인터페이스.
type CredentialSource interface {
    Provider() string
    // Load는 현재 파일 상태를 반환. mtime 변경 시 새 엔트리 목록.
    Load(ctx context.Context) ([]*PooledCredential, error)
    // 파일 mtime을 반환하여 변경 감지.
    ModTime(ctx context.Context) (time.Time, error)
}
```

```go
// internal/llm/credential/errors.go

var (
    ErrExhausted       = errors.New("credential pool exhausted")
    ErrRefreshFailed   = errors.New("credential refresh failed")
    ErrInvalidEntry    = errors.New("invalid credential entry id")
    ErrLeaseUnavailable = errors.New("lease unavailable")
)

type ExhaustedError struct {
    Provider   string
    RetryAfter time.Duration // 가장 빠른 reset
}

func (e *ExhaustedError) Error() string
func (e *ExhaustedError) Is(target error) bool // match ErrExhausted
```

### 6.3 선택 알고리즘 의사코드

```
Select(strategy):
  Lock()
  defer Unlock()

  // 1. 가용성 필터
  available = entries.filter(e =>
    e.status == OK AND
    (e.LastErrorResetAt <= now OR e.LastErrorResetAt.IsZero()) AND
    (e.ExpiresAt.IsZero() OR e.ExpiresAt > now))

  // 2. OAuth 엔트리 중 refresh 필요한 것 처리
  for e in available.filter(AuthOAuth):
    if e.ExpiresAt - RefreshMargin < now AND refresher != nil:
      Unlock()
      result, err = refresher.Refresh(ctx, e)
      Lock()
      if err:
        e.refreshFailCount++
        if e.refreshFailCount >= 3:
          e.Status = EXHAUSTED; e.LastErrorCode = -1  # 영구 고갈
          logWarn("credential permanently exhausted")
        else:
          e.Status = EXHAUSTED; e.LastErrorResetAt = now + 5분
        persistOne(e)
        continue
      e.accessToken = result.AccessToken
      if result.RefreshToken != "":
        e.refreshToken = result.RefreshToken
      e.ExpiresAt = result.ExpiresAt
      e.refreshFailCount = 0
      persistOne(e)

  // 3. 다시 가용성 재평가
  available = entries.filter(...)
  if available.empty():
    return nil, ExhaustedError{RetryAfter: minResetGap}

  // 4. 전략 적용
  chosen = applyStrategy(strategy, available)
  chosen.RequestCount++
  persistOne(chosen)
  return chosen, nil
```

### 6.4 영속 레이아웃

`~/.goose/credentials/<provider>.json` 예시:

```json
{
  "provider": "anthropic",
  "version": 1,
  "entries": [
    {
      "id": "a1b2c3",
      "label": "work-claude",
      "auth_type": "oauth",
      "priority": 10,
      "source": "anthropic_claude",
      "last_status": "ok",
      "last_status_at_ms": 1746000000000,
      "last_error_code": 0,
      "last_error_reset_at_ms": 0,
      "expires_at_ms": 1746003600000,
      "request_count": 42
    }
  ]
}
```

**비밀값은 저장하지 않는다**. `access_token`/`refresh_token`은 `~/.claude/.credentials.json` 등 **원본 소스 파일에만** 저장되며, 풀은 매 프로세스 시작 시 원본에서 읽는다. `api_key`는 `CONFIG-001`의 env vars 또는 YAML (redacted log).

### 6.5 외부 소스 스키마

**AnthropicClaudeSource** — `~/.claude/.credentials.json`:
```json
{
  "claudeAiOauth": {
    "accessToken": "...",
    "refreshToken": "...",
    "expiresAt": 1746003600000
  }
}
```

**OpenAICodexSource** — `~/.codex/auth.json`:
```json
{
  "OPENAI_API_KEY": null,
  "tokens": {
    "access_token": "...",
    "refresh_token": "...",
    "id_token": "..."
  },
  "last_refresh": "2026-04-20T..."
}
```

**NousSource** — `~/.hermes/auth.json`:
```json
{
  "agent_key": "...",
  "expires_at": "2026-05-01T..."
}
```

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestPool_Select_FillFirst_ReturnsHighestPriority` — AC-CREDPOOL-001.
2. **RED #2**: `TestPool_Select_RoundRobin_WrapsAround` — AC-CREDPOOL-002.
3. **RED #3**: `TestPool_OAuthRefreshAutoOnExpiring` — AC-CREDPOOL-003.
4. **RED #4**: `TestPool_MarkExhaustedAndRotate_OnHTTP429` — AC-CREDPOOL-004.
5. **RED #5**: `TestPool_AllExhausted_ReturnsErrExhausted` — AC-CREDPOOL-005.
6. **RED #6**: `TestPool_PersistsStateAcrossRecreation` — AC-CREDPOOL-006.
7. **RED #7**: `TestSources_AnthropicClaudeFileSync` — AC-CREDPOOL-007.
8. **RED #8**: `TestPool_LeaseCounterAffectsLeastUsed` — AC-CREDPOOL-008.
9. **RED #9**: `TestPool_Refresh3FailuresMarksPermanentExhausted` — AC-CREDPOOL-009.
10. **RED #10**: `TestPool_LogsRedactSecrets` — AC-CREDPOOL-010.
11. **GREEN**: 최소 구현.
12. **REFACTOR**: strategy를 별도 파일로 분리, Storage atomic write 추상화 정리.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, race detector 통과, 4 strategy × 3 상태 × 3 refresh 결과 = 36 cross-product의 주요 16 조합 테스트 |
| **R**eadable | 패키지 단일 책임, `Status`/`AuthType` enum, 공개 메서드 10 이하 |
| **U**nified | `go fmt` + `golangci-lint`, 모든 mutation은 Pool 메서드 경유, atomic write 패턴 통일 |
| **S**ecured | 비밀값 redaction 의무(REQ-CREDPOOL-014), 영속 파일은 비밀 미포함, 0600 권한 강제 |
| **T**rackable | 모든 상태 전환에 구조화 로그(`{entry_id, from_status, to_status, reason}`), `PoolStats`로 관측성 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, `GOOSE_HOME` 의미 상속 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `LLMConfig.Providers[*]`로부터 초기 엔트리 구성 |
| 후속 SPEC | SPEC-GOOSE-ROUTER-001 | 풀에서 provider 단위 자격 선택 |
| 후속 SPEC | SPEC-GOOSE-RATELIMIT-001 | 429 status + Retry-After 헤더를 `MarkExhaustedAndRotate`로 전달 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | `Refresher` 인터페이스의 provider별 구현 주입 |
| 외부 | Go 1.22+ | generics, `time.Time` 등 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |
| 외부 | stdlib `encoding/json` | 영속 JSON |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

**라이브러리 결정 (본 SPEC에서 채택만, 구현은 후속)**:
- 현 SPEC은 외부 OAuth 라이브러리 **미사용**(Refresher interface가 ADAPTER-001로 위임).
- ADAPTER-001이 `anthropic-sdk-go`(공식), `go-openai`(OpenAI-compat, xAI/DeepSeek/Groq 재사용) 등을 도입 예정.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 동시 `Select`/`MarkExhausted` race | 고 | 고 | `sync.Mutex` 스코프를 공개 메서드 전체로 확장, `go test -race` 필수 |
| R2 | Refresh 중 Unlock 구간에서 엔트리 mutate된 상태 불일치 | 중 | 중 | Refresh 호출 전 엔트리 ID 기준으로 post-refresh lookup, 발견되지 않으면 결과 drop |
| R3 | `~/.claude/.credentials.json` 스키마가 Anthropic 측에서 변경 | 중 | 중 | 스키마 버전 필드 검사, 미지 버전은 WARN 로그 + skip(기존 풀 유지) |
| R4 | 영속 JSON이 disk full에서 실패 | 낮 | 중 | atomic write 실패 시 풀 상태는 메모리에 남고, 다음 mutation에서 재시도 |
| R5 | OAuth token rotation에서 이전 refresh_token이 이미 무효화된 상태 | 고 | 고 | 3회 연속 실패 시 permanent exhausted (REQ-CREDPOOL-013), 운영자 `Pool.Reset` 또는 재로그인 요구 |
| R6 | 다중 `goosed` 인스턴스가 동일 영속 파일에 쓰면 충돌 | 중 | 중 | Phase 1 범위 외(단일 인스턴스 가정). 후속 SPEC에서 flock 또는 SQLite로 이관 |
| R7 | provider별 Retry-After 헤더 부재 시 기본 1시간이 과도 | 중 | 낮 | `PoolOptions.DefaultCooldown` 조정 가능, 호출자(RATELIMIT-001)가 더 정확한 값 제공 가능 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-llm.md` §1 Credential Pool 아키텍처, §2 Provider 매트릭스, §3 Python 인터페이스, §9 Go 포팅 매핑, §10 SPEC 도출, §12 고리스크
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 06, §13 핵심 설계 원칙 3
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` §6.2 `LLMConfig.Providers` 스키마
- `.moai/project/tech.md` §3.2 Go LLM 스택

### 9.2 외부 참조

- **Hermes Agent Python**: `./hermes-agent-main/agent/credential_pool.py` — 원형 참고
- **OAuth 2.1 PKCE RFC**: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1
- **Anthropic OAuth (Claude Code)**: https://docs.anthropic.com/claude-code
- **Go sync package**: https://pkg.go.dev/sync

### 9.3 부속 문서

- `./research.md` — Hermes `credential_pool.py` 구조 상세 분석 + 테스트 전략 + Python→Go 의미 보존 이슈

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **OAuth HTTP refresh의 실 구현을 포함하지 않는다**. `Refresher` interface만. ADAPTER-001이 각 provider별 HTTP 엔드포인트 호출을 구현.
- 본 SPEC은 **PKCE 로그인 브라우저 플로우를 구현하지 않는다**. CLI-001 + ADAPTER-001.
- 본 SPEC은 **Rate limit 헤더(x-ratelimit-*) 파싱을 포함하지 않는다**. 호출자가 RATELIMIT-001에서 파싱한 결과를 `MarkExhaustedAndRotate(retryAfter)`로 전달.
- 본 SPEC은 **모델 선택/비용 최적화를 포함하지 않는다**. ROUTER-001.
- 본 SPEC은 **프롬프트 캐시 마커 주입을 포함하지 않는다**. PROMPT-CACHE-001.
- 본 SPEC은 **사용량 가격 계산(토큰 × $/Mtoken)을 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **크로스 프로세스 flock을 구현하지 않는다**. 단일 `goosed` 프로세스 가정.
- 본 SPEC은 **키체인/1Password/Vault 통합을 포함하지 않는다**. 평문 JSON 파일 + env vars만.
- 본 SPEC은 **GitHub/Google Workspace OAuth를 지원하지 않는다**. LLM provider 3종만(Anthropic, OpenAI Codex, Nous).
- 본 SPEC은 **hot reload / fsnotify 감시를 포함하지 않는다**. 명시적 `Pool.RefreshSources(ctx)` 호출만.

---

**End of SPEC-GOOSE-CREDPOOL-001**

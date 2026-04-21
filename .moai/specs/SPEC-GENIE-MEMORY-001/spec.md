---
id: SPEC-GENIE-MEMORY-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GENIE-MEMORY-001 — Pluggable Memory Provider (Builtin + 외부 1개 Plugin)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §6 + Hermes `memory_provider.py` / `memory_manager.py` 368 LoC 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE-AGENT **자기진화 파이프라인의 Layer 4**를 정의한다. QueryEngine의 세션 경계를 넘어 **사용자 맥락·선호도·사실·과거 delegation 결과**를 유지하는 **Pluggable Memory Provider 인터페이스**를 정립하고, 항상 1순위로 동작하는 **Builtin Provider**(SQLite FTS5 + 파일 MEMORY.md/USER.md)와 **최대 1개의 외부 Plugin Provider**(Honcho / Hindsight / Mem0 등)를 `MemoryManager`가 조정한다.

본 SPEC이 통과한 시점에서:

- `MemoryProvider` 인터페이스 11개 메서드(필수 4 + 선택 7)가 정의되고,
- `BuiltinProvider`가 SQLite FTS5 전문 검색 + 파일 기반 system prompt block을 제공하며,
- `MemoryManager`가 Builtin을 항상 첫 번째로 등록 + 선언된 외부 plugin 1개를 그 뒤에 등록하되, 이름 충돌 시 충돌 감지 오류를 반환하고,
- QueryEngine의 lifecycle 훅(`on_turn_start` / `on_session_end` / `on_pre_compress` / `on_delegation`)이 각 Provider에 순차 dispatch되며 개별 Provider 실패가 QueryEngine 흐름을 차단하지 않고,
- 각 Provider의 `get_tool_schemas()`가 반환한 도구들이 QueryEngine의 Tool Registry에 이름 중복 검출과 함께 등록된다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- Identity Graph(Phase 6 IDENTITY-001), Preference Vector(VECTOR-001), Learning Engine의 graduated learning 결과는 **어딘가에 지속되어야** 다음 세션에 로드된다. Memory Provider가 그 지속 레이어.
- `.moai/project/research/hermes-learning.md` §6이 Hermes의 swappable memory backend(Builtin + Honcho/Hindsight/Mem0) 아키텍처를 이식 대상으로 명시.
- 한 플러그인 벤더(예: Honcho)에 종속되면 사용자가 backend를 교체할 수 없다. **인터페이스 분리**가 핵심.
- `SessionID` 격리: TRAJECTORY-001이 할당한 session_id를 키로 memory를 조회/저장. 여러 세션이 동시 실행되어도 memory가 섞이지 않음.
- 로드맵 v2.0 §4 Phase 4 #23. v1.0에서는 MEMORY SPEC이 없었기에 **신규 작성**(기존 SPEC-GENIE-MEMORY-001이 레포에 존재하지 않음 확인 필요).

### 2.2 상속 자산

- **Hermes Agent Python** (`./hermes-agent-main/agent/memory_provider.py` + `memory_manager.py` 368 LoC): `MemoryProvider` ABC, `MemoryManager` 등록 로직, Builtin SQLite + 파일 혼합, 외부 plugin 1개 제한, lifecycle 훅 7종, tool schema 수집, 이름 충돌 검출, 실패 격리.
- **Claude Code TypeScript**: 자체 memory 레이어 없음(외부 MCP에 위임). 계승 대상 아님.
- **MoAI-ADK-Go auto-memory** (`~/.claude/projects/{hash}/memory/`): MoAI의 lessons.md 스토리지. 개념 참고(외부 저장소 격리 패턴).

### 2.3 범위 경계

- **IN**: `MemoryProvider` 인터페이스(11 메서드), `MemoryManager` + 등록/조정 로직, `BuiltinProvider`(SQLite FTS5 + 파일 MEMORY.md/USER.md), session_id 격리, lifecycle 훅 dispatch, tool schema 수집 + 충돌 검출, 실패 격리, 설정 통합(config.memory.{provider, plugin}).
- **OUT**: 외부 plugin 구현체(Honcho / Hindsight / Mem0 각각의 HTTP 클라이언트 — 각 별도 SPEC 또는 외부 모듈), MCP gateway(MCP-001), Identity Graph 스키마 자체(IDENTITY-001), vector embedding 검색(VECTOR-001), REFLECT-001 5단계 승격(REFLECT-001), 오래된 memory pruning 정책(초기값만 제공, 고도화는 별도 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/memory/` 패키지: `MemoryProvider` 인터페이스, `MemoryManager`, `SessionContext`, `RecallResult`, `ToolSchema` 구조체.
2. `internal/memory/builtin/` 서브패키지: `BuiltinProvider` (SQLite FTS5 + 파일 기반).
3. `internal/memory/plugin/` 서브패키지: 외부 plugin loader의 공통 인터페이스(`PluginProvider` 어댑터). 실제 구현체는 별도 모듈.
4. `MemoryProvider` 인터페이스 11 메서드:
   - **필수 4**: `Name()`, `IsAvailable()`, `Initialize(sessionID, ...)`, `GetToolSchemas()`
   - **선택 7**: `SystemPromptBlock()`, `Prefetch(query)`, `QueuePrefetch(query)`, `SyncTurn(userMsg, assistantMsg)`, `HandleToolCall(toolName, args)`, `OnTurnStart(turn, msg)`, `OnSessionEnd(messages)`, `OnPreCompress(messages)`, `OnDelegation(task, result)`
5. `MemoryManager` 책임: provider 등록 검증(Builtin 필수 1순위, 외부 최대 1개), 이름 충돌 검출, tool schema 수집(충돌 시 에러), lifecycle 훅 순차 dispatch, 개별 provider 실패 격리.
6. `BuiltinProvider` 기능: SQLite FTS5 풀텍스트 인덱스, `MEMORY.md` 파일 read/write, `USER.md` 파일 read(읽기 전용), `session_id` 컬럼 기반 격리, `recall(query, limit, session_id)` 구현.
7. 설정 통합: `config.memory.builtin.db_path`, `config.memory.plugin.name`, `config.memory.plugin.config` (플러그인별 임의 YAML).
8. 동시성: 동일 session_id의 메서드는 직렬화(Provider별 mutex), 다른 session_id는 병렬 OK.

### 3.2 OUT OF SCOPE (명시적 제외)

- **외부 plugin 구현체 본체**: `HonchoProvider`, `HindsightProvider`, `Mem0Provider` 각각의 HTTP 클라이언트와 인증 — 별도 모듈(`internal/memory/plugin/honcho/`, 등). 본 SPEC은 어댑터 인터페이스만.
- **MCP gateway 통합**: MCP-001이 MCP 서버로서 memory tool을 외부에 노출. 본 SPEC은 내부 provider 레이어만.
- **Identity Graph**: POLE+O 스키마, Kuzu backend는 IDENTITY-001(Phase 6).
- **Vector Search**: 768-dim embedding + cosine similarity는 VECTOR-001(Phase 6).
- **5단계 승격 파이프라인**: observation → heuristic → rule → high_conf → graduated는 REFLECT-001(Phase 5).
- **Federated Learning / Secure Aggregation**: 로컬 저장 only. 원격 동기화는 별도 SPEC.
- **암호화 at-rest**: SQLite 파일 권한(0600) + 디렉토리(0700) 만. DB 암호화는 선택적 향후 확장.
- **Backup / Restore**: 사용자가 `~/.genie/memory/` 복사로 해결. 자동 백업 기능 없음.
- **Pruning / Retention 정책**: `BuiltinProvider`에 `max_rows` 기본값만(10,000 행), LRU 자동 제거는 향후 확장.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-MEMORY-001 [Ubiquitous]** — The `MemoryManager` **shall** always register the `BuiltinProvider` as the first provider in its internal list; attempts to register without Builtin **shall** return `ErrBuiltinRequired`.

**REQ-MEMORY-002 [Ubiquitous]** — The `MemoryManager` **shall** accept at most one external plugin provider beyond Builtin; a second `RegisterPlugin` call **shall** return `ErrOnlyOnePluginAllowed`.

**REQ-MEMORY-003 [Ubiquitous]** — Each `MemoryProvider.Name()` **shall** return a non-empty unique ASCII identifier (`[a-z][a-z0-9_-]{0,31}`); the `MemoryManager` **shall** reject registration of providers whose names collide (case-insensitive) with `ErrNameCollision`.

**REQ-MEMORY-004 [Ubiquitous]** — The `MemoryProvider.IsAvailable()` method **shall** return a boolean without performing network I/O (config + credential presence only); side effects prohibited.

**REQ-MEMORY-005 [Ubiquitous]** — Every memory operation that accepts a `session_id` **shall** restrict reads and writes to that session_id scope; cross-session data **shall not** leak via `Prefetch` / `HandleToolCall` responses.

### 4.2 Event-Driven (이벤트 기반)

**REQ-MEMORY-006 [Event-Driven]** — **When** a session starts, the `MemoryManager.Initialize(sessionID)` **shall** invoke `Provider.Initialize(sessionID, context)` on every registered provider in registration order, and **shall** collect returned errors into a multi-error without aborting.

**REQ-MEMORY-007 [Event-Driven]** — **When** a QueryEngine turn begins, the `MemoryManager.OnTurnStart(sessionID, turn, userMsg)` **shall** dispatch to every provider; individual provider panics **shall** be recovered and logged without propagation.

**REQ-MEMORY-008 [Event-Driven]** — **When** a QueryEngine session terminates (Terminal event), the `MemoryManager.OnSessionEnd(sessionID, messages)` **shall** dispatch to every provider in reverse registration order (LIFO), enabling external plugins to flush before Builtin's final commit.

**REQ-MEMORY-009 [Event-Driven]** — **When** a QueryEngine tool call matches a tool name owned by a provider (via `GetToolSchemas`), the `MemoryManager` **shall** route the call to that provider's `HandleToolCall(toolName, args, ctx)` and return its JSON string response verbatim.

**REQ-MEMORY-010 [Event-Driven]** — **When** `MemoryManager.SystemPromptBlock(sessionID)` is invoked, the manager **shall** concatenate each provider's `SystemPromptBlock()` output in registration order, separated by a blank line, and return the aggregate.

**REQ-MEMORY-011 [Event-Driven]** — **When** the QueryEngine CompactBoundary is about to fire, the `MemoryManager.OnPreCompress(sessionID, messages)` **shall** dispatch and collect each provider's pre-compression hint string; aggregate hints are appended to the compaction prompt.

### 4.3 State-Driven (상태 기반)

**REQ-MEMORY-012 [State-Driven]** — **While** a provider's `IsAvailable() == false`, the `MemoryManager` **shall** skip dispatching any lifecycle hook or tool call to that provider but **shall** keep it in the registration list (re-availability check on next session).

**REQ-MEMORY-013 [State-Driven]** — **While** a provider returned a non-nil error from `Initialize()`, subsequent dispatches to that provider **shall** be suppressed for the current session, but **shall** retry `Initialize()` on the next session.

### 4.4 Unwanted Behavior (방지)

**REQ-MEMORY-014 [Unwanted]** — The `MemoryManager` **shall not** block the QueryEngine goroutine for more than 50ms on `OnTurnStart` dispatch; individual provider timeout (default 40ms) **shall** cause that provider's hook to be cancelled, not the whole dispatch.

**REQ-MEMORY-015 [Unwanted]** — The `MemoryManager.GetToolSchemas()` **shall not** return duplicate tool names; if two providers declare the same tool name, `RegisterPlugin` time **shall** return `ErrToolNameCollision` — tool collisions are detected at registration, not at dispatch.

**REQ-MEMORY-016 [Unwanted]** — The `BuiltinProvider` **shall not** write to `USER.md`; that file is **read-only** user-supplied static context.

**REQ-MEMORY-017 [Unwanted]** — The `MemoryManager` **shall not** retain references to `messages[]` passed to `OnSessionEnd` or `OnPreCompress` after the hook returns; providers that need persistence must copy the data.

### 4.5 Optional (선택적)

**REQ-MEMORY-018 [Optional]** — **Where** a provider implements `QueuePrefetch`, the `MemoryManager` **shall** dispatch the query asynchronously on a background goroutine and **shall not** wait for the result (best-effort cache warming).

**REQ-MEMORY-019 [Optional]** — **Where** `config.memory.plugin.name` is a known Go import path (e.g. `github.com/genie/memory-honcho`), the `MemoryManager` **shall** attempt to instantiate the plugin via the registered factory function in `internal/memory/plugin/registry.go`.

**REQ-MEMORY-020 [Optional]** — **Where** a provider returns a non-empty string from `OnPreCompress`, that string **shall** be wrapped in a `## Memory Context` section and prepended to the compaction summary prompt.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then.

**AC-MEMORY-001 — Builtin 필수**
- **Given** 새 `MemoryManager` 인스턴스
- **When** Builtin 등록 없이 `manager.Initialize(sessionID="s1", ctx)` 호출
- **Then** 반환 error가 `errors.Is(err, ErrBuiltinRequired)`, 어떤 provider도 dispatch되지 않음

**AC-MEMORY-002 — 최대 1개 plugin**
- **Given** Builtin + PluginA 등록됨
- **When** PluginB 추가 등록 시도
- **Then** `errors.Is(err, ErrOnlyOnePluginAllowed)`, PluginB는 내부 list에 추가되지 않음

**AC-MEMORY-003 — Name 충돌 검출**
- **Given** Builtin (name="builtin")
- **When** PluginX (name="builtin") 등록 시도 (case-insensitive "Builtin"도 포함)
- **Then** `errors.Is(err, ErrNameCollision)`

**AC-MEMORY-004 — Tool schema 충돌 검출**
- **Given** Builtin이 `memory_recall` tool 노출, PluginX가 `memory_recall` tool 노출
- **When** PluginX 등록
- **Then** `errors.Is(err, ErrToolNameCollision)`, PluginX 등록 실패, Builtin 상태 변경 없음

**AC-MEMORY-005 — Session 격리**
- **Given** Builtin + `BuiltinProvider.SaveFact(sessionID="s1", "fact A")` 선행 호출, 그리고 `SaveFact(sessionID="s2", "fact B")`
- **When** `manager.Prefetch("fact", sessionID="s1")`
- **Then** 반환 결과에 "fact A" 포함, "fact B" 미포함

**AC-MEMORY-006 — Lifecycle dispatch 순서 (Initialize forward, SessionEnd reverse)**
- **Given** Builtin + PluginA 등록, 각 hook 호출 순서 log 기록
- **When** `manager.Initialize`, 후 `manager.OnSessionEnd`
- **Then** Initialize log: `[Builtin, PluginA]`, SessionEnd log: `[PluginA, Builtin]` (REQ-MEMORY-008 LIFO)

**AC-MEMORY-007 — 개별 provider 실패 격리**
- **Given** Builtin + PluginA(panic 주입), 세션 진행 중
- **When** `manager.OnTurnStart(turn=1, msg="hi")`
- **Then** PluginA는 panic 후 skip(zap error 로그 1건), Builtin은 정상 dispatch, QueryEngine 계속 진행

**AC-MEMORY-008 — IsAvailable=false 스킵**
- **Given** PluginA `IsAvailable()=false` (credential 없음)
- **When** `manager.OnTurnStart` dispatch
- **Then** PluginA hook 0회 호출, Builtin만 호출됨

**AC-MEMORY-009 — SystemPromptBlock 집계**
- **Given** Builtin prompt="Remember A", PluginA prompt="Remember B"
- **When** `manager.SystemPromptBlock(sessionID)`
- **Then** 반환 문자열에 "Remember A\n\nRemember B" 순서 포함(구분자 빈 줄 2개 = \n\n)

**AC-MEMORY-010 — Tool 라우팅**
- **Given** Builtin이 `memory_recall` tool, PluginA가 `memory_save` tool 노출
- **When** `manager.HandleToolCall("memory_save", args={"key":"K"}, ctx)`
- **Then** PluginA의 `HandleToolCall`이 호출되고, Builtin은 호출되지 않음. 반환값이 PluginA의 JSON 응답 그대로

**AC-MEMORY-011 — 50ms dispatch budget**
- **Given** PluginA가 `OnTurnStart`에서 100ms 블로킹
- **When** `manager.OnTurnStart`
- **Then** 40ms 시점에 PluginA context cancel, 전체 dispatch 시간 ≤ 50ms, PluginA 미완료 오류 로그 1건

**AC-MEMORY-012 — QueuePrefetch async**
- **Given** PluginA.QueuePrefetch 구현 (비동기 100ms 소요)
- **When** `manager.QueuePrefetch(sessionID, "query")`
- **Then** 호출 return이 10ms 이내, 내부 goroutine이 100ms 후 fetch 완료(테스트에서 가시적 effect 확인)

**AC-MEMORY-013 — BuiltinProvider FTS5 검색**
- **Given** Builtin DB에 `SaveFact("user_loves_go", "User prefers Go over Python for systems work", sessionID="s1")` 선행
- **When** `Prefetch("Go systems", sessionID="s1", limit=5)`
- **Then** 결과에 "user_loves_go" fact 포함, FTS5 랭크 순으로 정렬

**AC-MEMORY-014 — USER.md 읽기 전용**
- **Given** `~/.genie/memory/USER.md` 파일에 "I prefer async over sync" 기록
- **When** `BuiltinProvider.SystemPromptBlock()` 호출
- **Then** 반환 문자열에 "I prefer async over sync" 포함. Builtin이 USER.md에 쓰기 시도하면 `ErrUserMdReadOnly` 반환

**AC-MEMORY-015 — MEMORY.md 쓰기 (Builtin만)**
- **Given** Builtin.MEMORY.md 경로 `~/.genie/memory/MEMORY.md`
- **When** `BuiltinProvider.SyncTurn(sessionID="s1", userMsg="I live in Seoul", assistantMsg="Noted: Seoul")` 호출
- **Then** MEMORY.md 파일에 `- [s1] I live in Seoul` 형태의 한 줄 append(형식은 구현 결정)

**AC-MEMORY-016 — Plugin 부재 시 Builtin-only 동작**
- **Given** `config.memory.plugin.name == ""` (plugin 없음)
- **When** manager 생성 + 모든 hook 호출
- **Then** 오직 Builtin이 dispatch됨, 에러 없음, 시스템은 정상 작동

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/
└── memory/
    ├── provider.go                 # MemoryProvider 인터페이스
    ├── manager.go                  # MemoryManager + 등록/dispatch
    ├── manager_test.go
    ├── types.go                    # SessionContext, RecallResult, ToolSchema
    ├── errors.go                   # Sentinel errors
    ├── config.go                   # MemoryConfig
    ├── builtin/
    │   ├── builtin.go              # BuiltinProvider
    │   ├── sqlite.go               # FTS5 인덱스 스키마 + 쿼리
    │   ├── sqlite_test.go
    │   ├── files.go                # MEMORY.md / USER.md 관리
    │   └── tools.go                # memory_recall, memory_save tool schema
    └── plugin/
        ├── adapter.go              # PluginProvider 공통 어댑터
        ├── registry.go             # factory map (plugin 이름 → constructor)
        └── README.md               # 외부 plugin 작성 가이드
```

### 6.2 핵심 타입 (Go 시그니처)

```go
// internal/memory/provider.go

// MemoryProvider는 swappable memory backend 인터페이스.
// 필수 4 + 선택 7. 선택 메서드는 기본 구현(no-op) 제공 가능.
type MemoryProvider interface {
    // ===== 필수 =====
    // Name은 고유 식별자. ^[a-z][a-z0-9_-]{0,31}$
    Name() string

    // IsAvailable은 네트워크 I/O 없이 사용 가능 여부 판단.
    IsAvailable() bool

    // Initialize는 세션당 1회. 네트워크 연결, 캐시 워밍 등.
    Initialize(sessionID string, ctx SessionContext) error

    // GetToolSchemas는 이 provider가 노출할 도구 목록.
    GetToolSchemas() []ToolSchema

    // ===== 선택 (default no-op 구현 제공) =====
    SystemPromptBlock() string
    Prefetch(query string, sessionID string) (RecallResult, error)
    QueuePrefetch(query string, sessionID string)
    SyncTurn(sessionID, userContent, assistantContent string) error
    HandleToolCall(toolName string, args json.RawMessage, ctx ToolContext) (string, error)

    OnTurnStart(sessionID string, turnNumber int, message Message)
    OnSessionEnd(sessionID string, messages []Message)
    OnPreCompress(sessionID string, messages []Message) string
    OnDelegation(sessionID, task, result string)
}

// BaseProvider는 선택 메서드에 대한 no-op 기본 구현.
// 임베드해서 사용.
type BaseProvider struct{}
func (BaseProvider) SystemPromptBlock() string                             { return "" }
func (BaseProvider) Prefetch(q, s string) (RecallResult, error)            { return RecallResult{}, nil }
func (BaseProvider) QueuePrefetch(q, s string)                             {}
// ... (모든 선택 메서드)


// internal/memory/types.go

type SessionContext struct {
    HermesHome    string   // legacy: Hermes에서 GENIE_HOME과 동의어
    Platform      string   // "darwin" | "linux" | "windows"
    AgentContext  map[string]string   // QueryEngine에서 주입 (user_id, preferences 등)
    AgentIdentity string   // teammate 모드 식별자
}

type ToolSchema struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
    Owner       string          // 내부 field: provider name
}

type RecallResult struct {
    Items       []RecallItem
    TotalTokens int
}

type RecallItem struct {
    Content    string
    Source     string     // "builtin:sqlite" | "honcho:endpoint" 등
    Score      float64    // 관련도 [0,1]
    SessionID  string
    Timestamp  time.Time
}


// internal/memory/manager.go

type MemoryManager struct {
    cfg         MemoryConfig
    providers   []MemoryProvider       // [0]=Builtin, [1]=external (최대)
    toolIndex   map[string]int         // tool_name -> provider index
    mu          sync.RWMutex
    dispatcher  *dispatcher             // lifecycle hook 라우팅
    logger      *zap.Logger
}

func New(cfg MemoryConfig, logger *zap.Logger) (*MemoryManager, error)

// RegisterBuiltin은 반드시 첫 번째 호출. 이후 다른 등록 가능.
func (m *MemoryManager) RegisterBuiltin(p MemoryProvider) error

// RegisterPlugin은 Builtin 다음에 최대 1회.
func (m *MemoryManager) RegisterPlugin(p MemoryProvider) error

// Initialize는 세션 시작.
func (m *MemoryManager) Initialize(ctx context.Context, sessionID string, sctx SessionContext) error

// OnTurnStart는 매 턴. 50ms 전체 예산 + 40ms provider 개별 budget.
func (m *MemoryManager) OnTurnStart(ctx context.Context, sessionID string, turn int, msg Message)

func (m *MemoryManager) OnSessionEnd(ctx context.Context, sessionID string, messages []Message)
func (m *MemoryManager) OnPreCompress(ctx context.Context, sessionID string, messages []Message) string

func (m *MemoryManager) SystemPromptBlock(sessionID string) string
func (m *MemoryManager) Prefetch(ctx context.Context, query, sessionID string) (RecallResult, error)
func (m *MemoryManager) QueuePrefetch(query, sessionID string)

func (m *MemoryManager) GetAllToolSchemas() []ToolSchema
func (m *MemoryManager) HandleToolCall(ctx context.Context, toolName string, args json.RawMessage, tctx ToolContext) (string, error)


// internal/memory/errors.go

var (
    ErrBuiltinRequired      = errors.New("memory: BuiltinProvider must be registered first")
    ErrOnlyOnePluginAllowed = errors.New("memory: at most one external plugin allowed")
    ErrNameCollision        = errors.New("memory: provider name collides (case-insensitive)")
    ErrToolNameCollision    = errors.New("memory: tool name collides between providers")
    ErrInvalidProviderName  = errors.New("memory: provider name must match ^[a-z][a-z0-9_-]{0,31}$")
    ErrUserMdReadOnly       = errors.New("memory: USER.md is read-only")
    ErrProviderNotInit      = errors.New("memory: provider not initialized for this session")
)


// internal/memory/builtin/builtin.go

type BuiltinProvider struct {
    memory.BaseProvider    // 선택 메서드 no-op 기본
    dbPath     string
    memoryMd   string       // ~/.genie/memory/MEMORY.md
    userMd     string       // ~/.genie/memory/USER.md
    db         *sql.DB
    mu         sync.Mutex
    logger     *zap.Logger
}

func NewBuiltin(cfg BuiltinConfig, logger *zap.Logger) (*BuiltinProvider, error)

func (b *BuiltinProvider) Name() string                           { return "builtin" }
func (b *BuiltinProvider) IsAvailable() bool                       { return b.db != nil }
func (b *BuiltinProvider) Initialize(sessionID string, sctx SessionContext) error
func (b *BuiltinProvider) GetToolSchemas() []ToolSchema {
    return []ToolSchema{
        {Name: "memory_recall", Description: "Recall facts by keyword", ...},
        {Name: "memory_save",   Description: "Save a fact for future recall", ...},
    }
}
func (b *BuiltinProvider) Prefetch(query, sessionID string) (RecallResult, error)
func (b *BuiltinProvider) SyncTurn(sessionID, userContent, assistantContent string) error
func (b *BuiltinProvider) SystemPromptBlock() string   // USER.md + 최근 MEMORY.md 일부
func (b *BuiltinProvider) OnSessionEnd(sessionID string, messages []Message)
```

### 6.3 BuiltinProvider SQLite FTS5 스키마

```sql
-- facts 테이블 + FTS5 인덱스
CREATE TABLE IF NOT EXISTS facts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL,
    key         TEXT NOT NULL,
    content     TEXT NOT NULL,
    source      TEXT NOT NULL,          -- "user" | "inferred" | "tool_result"
    confidence  REAL NOT NULL DEFAULT 1.0,
    created_at  INTEGER NOT NULL,        -- unix epoch
    updated_at  INTEGER NOT NULL,
    UNIQUE(session_id, key)
);

CREATE VIRTUAL TABLE IF NOT EXISTS facts_fts USING fts5(
    content,
    content=facts,
    content_rowid=id,
    tokenize='porter unicode61'
);

-- FTS5 sync triggers
CREATE TRIGGER IF NOT EXISTS facts_ai AFTER INSERT ON facts
BEGIN
    INSERT INTO facts_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER IF NOT EXISTS facts_ad AFTER DELETE ON facts
BEGIN
    INSERT INTO facts_fts(facts_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;
CREATE TRIGGER IF NOT EXISTS facts_au AFTER UPDATE ON facts
BEGIN
    INSERT INTO facts_fts(facts_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO facts_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE INDEX IF NOT EXISTS idx_facts_session ON facts(session_id);
CREATE INDEX IF NOT EXISTS idx_facts_updated ON facts(updated_at DESC);
```

Prefetch 쿼리 예:
```sql
SELECT f.id, f.content, f.source, f.confidence, f.created_at
FROM facts f
JOIN facts_fts fts ON f.id = fts.rowid
WHERE f.session_id = ?
  AND facts_fts MATCH ?          -- FTS5 query syntax
ORDER BY rank
LIMIT ?;
```

### 6.4 Dispatch 전략 (50ms 전체 예산)

```go
func (m *MemoryManager) OnTurnStart(ctx context.Context, sessionID string, turn int, msg Message) {
    ctx, cancel := context.WithTimeout(ctx, 50 * time.Millisecond)
    defer cancel()

    for _, p := range m.providers {
        if !p.IsAvailable() { continue }

        pctx, pcancel := context.WithTimeout(ctx, 40 * time.Millisecond)
        done := make(chan struct{})
        go func(prov MemoryProvider) {
            defer func() {
                if r := recover(); r != nil {
                    m.logger.Error("provider panic", zap.String("provider", prov.Name()), zap.Any("panic", r))
                }
                close(done)
            }()
            prov.OnTurnStart(sessionID, turn, msg)
        }(p)

        select {
        case <-done:
            // OK
        case <-pctx.Done():
            m.logger.Warn("provider hook timeout", zap.String("provider", p.Name()))
        }
        pcancel()
        if ctx.Err() != nil { return }  // 전체 예산 소진
    }
}
```

### 6.5 Tool 충돌 검출 알고리즘

```go
func (m *MemoryManager) RegisterPlugin(p MemoryProvider) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if len(m.providers) < 1 || m.providers[0].Name() != "builtin" {
        return ErrBuiltinRequired
    }
    if len(m.providers) >= 2 {
        return ErrOnlyOnePluginAllowed
    }
    if !isValidProviderName(p.Name()) {
        return ErrInvalidProviderName
    }
    if strings.EqualFold(p.Name(), "builtin") {
        return ErrNameCollision
    }

    // Tool name 충돌 검출
    existing := m.GetAllToolSchemas()
    existingNames := make(map[string]bool, len(existing))
    for _, s := range existing {
        existingNames[s.Name] = true
    }
    for _, newSchema := range p.GetToolSchemas() {
        if existingNames[newSchema.Name] {
            return fmt.Errorf("%w: %q already owned by another provider",
                ErrToolNameCollision, newSchema.Name)
        }
    }

    m.providers = append(m.providers, p)
    // toolIndex 업데이트
    pluginIdx := len(m.providers) - 1
    for _, schema := range p.GetToolSchemas() {
        m.toolIndex[schema.Name] = pluginIdx
    }
    return nil
}
```

### 6.6 TDD 진입 순서

1. **RED #1**: `TestManager_BuiltinRequired` — AC-MEMORY-001.
2. **RED #2**: `TestManager_OnlyOnePlugin` — AC-MEMORY-002.
3. **RED #3**: `TestManager_NameCollision_CaseInsensitive` — AC-MEMORY-003.
4. **RED #4**: `TestManager_ToolNameCollision_AtRegistration` — AC-MEMORY-004.
5. **RED #5**: `TestManager_DispatchOrder_InitForward_EndReverse` — AC-MEMORY-006.
6. **RED #6**: `TestManager_ProviderPanicIsolated` — AC-MEMORY-007.
7. **RED #7**: `TestManager_IsAvailableFalseSkips` — AC-MEMORY-008.
8. **RED #8**: `TestManager_SystemPromptAggregation` — AC-MEMORY-009.
9. **RED #9**: `TestManager_ToolRoutingByName` — AC-MEMORY-010.
10. **RED #10**: `TestManager_DispatchBudget50ms` — AC-MEMORY-011.
11. **RED #11**: `TestManager_QueuePrefetchIsAsync` — AC-MEMORY-012.
12. **RED #12**: `TestBuiltin_FTS5_RecallByKeyword` — AC-MEMORY-013.
13. **RED #13**: `TestBuiltin_SessionIsolation` — AC-MEMORY-005.
14. **RED #14**: `TestBuiltin_UserMdReadOnly` — AC-MEMORY-014.
15. **RED #15**: `TestBuiltin_MemoryMdAppend` — AC-MEMORY-015.
16. **RED #16**: `TestManager_BuiltinOnlyFlow` — AC-MEMORY-016.
17. **GREEN**: Manager 등록 / dispatcher / BuiltinProvider SQLite 구현.
18. **REFACTOR**: `dispatcher.go` 추출, `builtin/sqlite.go`의 쿼리를 constant 분리, BaseProvider 임베드 패턴 문서화.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 16 AC 전부 단위 테스트, `-race` 통과, BuiltinProvider의 SQLite는 `t.TempDir()`로 격리 |
| **R**eadable | 필수/선택 메서드 구분 명시(§6.2 주석), BaseProvider 임베드 패턴으로 선택 메서드 no-op 자동화 |
| **U**nified | `golangci-lint` 통과, tool 이름 규약 `^[a-z][a-z0-9_-]{0,31}$` (provider 이름과 동일) |
| **S**ecured | 파일 mode 0600 + 디렉토리 0700, USER.md 읽기 전용 강제, session_id 격리 강제, panic guard |
| **T**rackable | 모든 dispatch에 zap 로그 `{provider, hook, duration}`, tool 라우팅 기록 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GENIE-CORE-001 | `GENIE_HOME`, zap 로거, context 루트 |
| 선행 SPEC | SPEC-GENIE-CONFIG-001 | `config.memory.*` 로드 |
| 선행 SPEC | SPEC-GENIE-QUERY-001 | `Message` 타입, lifecycle 훅 진입점(`PostSamplingHooks` 등) |
| 협력 SPEC | SPEC-GENIE-TRAJECTORY-001 | `session_id` 공유 |
| 후속 SPEC | SPEC-GENIE-TOOLS-001 | `GetAllToolSchemas()`를 Tool Registry가 수집 |
| 후속 SPEC | SPEC-GENIE-MCP-001 | Memory tool을 MCP 서버로 외부 노출 가능 |
| 후속 SPEC | SPEC-GENIE-IDENTITY-001 (Phase 6) | Identity Graph를 Builtin 확장으로 주입 |
| 후속 SPEC | SPEC-GENIE-VECTOR-001 (Phase 6) | Vector search provider로 추가 등록 가능 |
| 후속 SPEC | SPEC-GENIE-REFLECT-001 (Phase 5) | graduated learning을 `facts` 테이블에 영속 |
| 외부 | Go 1.22+ | generics, `sync.Mutex`, `context` |
| 외부 | `modernc.org/sqlite` v1.29+ 또는 `mattn/go-sqlite3` v1.14+ | SQLite FTS5 |
| 외부 | `go.uber.org/zap` v1.27+ | CORE-001 계승 |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `mattn/go-sqlite3` CGO 의존성이 cross-compile 부담 | 고 | 중 | 대안 `modernc.org/sqlite` (pure Go) 제공. 본 SPEC은 `Tokenizer`처럼 SQLite 인터페이스 추상화는 하지 않지만, 빌드 태그로 선택 가능하게 config |
| R2 | FTS5 비활성 빌드에서 실패 | 중 | 중 | `sqlite3_fts5=1` 빌드 태그 강제. init 시 `SELECT * FROM pragma_compile_options()` 검사 후 `ErrFTS5Missing` 반환 |
| R3 | 50ms dispatch 예산이 plugin HTTP 호출에 부족 | 고 | 중 | `OnTurnStart`는 동기 — plugin은 `QueuePrefetch`로 비동기 처리 권장. 문서화 명시. 필요 시 budget을 config로 노출 |
| R4 | Tool schema 충돌 검출이 runtime register 시점 → 외부 plugin version up 시 silent 충돌 | 중 | 중 | Plugin 버전 업 시 manager가 재검증. `GetAllToolSchemas()` 호출 시마다 이름 unique 검증 assertion |
| R5 | USER.md가 수십 MB로 비대해지면 SystemPromptBlock 호출 비용 폭증 | 낮 | 중 | `SystemPromptBlock()`에 first N KB 제한(기본 8KB). truncation warning |
| R6 | Provider panic 시 state 변화가 rollback 안 됨 (예: SQLite transaction 열림) | 중 | 고 | BuiltinProvider의 모든 SQLite 쓰기는 transactional, `defer tx.Rollback()` 패턴. Plugin 구현체는 문서로 가이드 |
| R7 | 세션 종료 시 OnSessionEnd가 큰 messages[] 처리 중 long-running → 다음 세션 시작 지연 | 중 | 중 | OnSessionEnd를 goroutine으로 spawn(단, Builtin만 sync). Plugin은 fire-and-forget 정책 문서화 |
| R8 | Multi-goroutine session이 동일 session_id에 경쟁 쓰기 | 낮 | 중 | BuiltinProvider.mu로 session_id 범위 직렬화. 성능 영향 → future optimization with per-session mutex map |
| R9 | Plugin registry에 없는 이름이 config에 있으면 | 중 | 낮 | Manager.New가 `ErrUnknownPlugin` 반환, 부트스트랩 실패 |
| R10 | `OnPreCompress` 결과가 토큰 많아서 compaction prompt 팽창 | 낮 | 낮 | provider당 반환 문자열 cap(기본 1KB). CONTEXT-001과 협의 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-learning.md` §6 Memory Provider 인터페이스, §7 Skill 카탈로그 (tool 연동 참고)
- `.moai/project/learning-engine.md` §2 MoAI-ADK-Go SPEC-REFLECT-001 계승, §9.1 `internal/memory/` 구조
- `.moai/project/adaptation.md` §10 Identity Graph 저장 통합 포인트
- `.moai/specs/ROADMAP.md` §4 Phase 4 #23, §11 오픈 이슈 (Kuzu 위치 결정은 Phase 6 IDENTITY-001)
- `.moai/specs/SPEC-GENIE-TRAJECTORY-001/spec.md` — session_id 공유 공급자
- `.moai/specs/SPEC-GENIE-QUERY-001/spec.md` — lifecycle hook 진입점

### 9.2 외부 참조

- **Hermes `memory_provider.py` + `memory_manager.py`** (368 LoC): ABC + Manager 원본
- **SQLite FTS5**: https://www.sqlite.org/fts5.html — 전문 검색 인덱스 문법
- **Go sql driver 비교**: `modernc.org/sqlite` (pure Go, FTS5 포함) vs `mattn/go-sqlite3` (CGO, 빠름)
- **Honcho API**: https://honcho.dev — 후속 plugin 대상 (참고만)

### 9.3 부속 문서

- `./research.md` — Hermes 368 LoC → Go 800 LoC 이식 매핑, SQLite 라이브러리 결정, plugin registry 설계
- `../SPEC-GENIE-TRAJECTORY-001/spec.md` — session_id 공급자
- `../SPEC-GENIE-INSIGHTS-001/spec.md` — MEMORY 통계 소비자

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **Honcho / Hindsight / Mem0 각 plugin의 HTTP 클라이언트 구현을 포함하지 않는다**. `internal/memory/plugin/<name>/` 별도 모듈.
- 본 SPEC은 **MCP 서버로 memory tool을 외부에 노출하지 않는다**. MCP-001.
- 본 SPEC은 **Identity Graph 스키마 / Kuzu backend를 구현하지 않는다**. IDENTITY-001 (Phase 6).
- 본 SPEC은 **Vector search / 768-dim embedding을 구현하지 않는다**. VECTOR-001 (Phase 6).
- 본 SPEC은 **5단계 승격 파이프라인을 구현하지 않는다**. REFLECT-001 (Phase 5).
- 본 SPEC은 **Federated Learning / Secure Aggregation을 포함하지 않는다**.
- 본 SPEC은 **DB 암호화 at-rest를 강제하지 않는다**(파일 권한 0600만).
- 본 SPEC은 **Backup / Restore 기능을 포함하지 않는다**.
- 본 SPEC은 **LRU / retention 기반 pruning을 자동화하지 않는다**(max_rows 기본값만).
- 본 SPEC은 **SystemPromptBlock 캐싱을 구현하지 않는다**(매 호출 계산).
- 본 SPEC은 **USER.md에 쓰기를 허용하지 않는다**(read-only).

---

**End of SPEC-GENIE-MEMORY-001**

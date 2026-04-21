# SPEC-GENIE-MEMORY-001 — Research & Porting Analysis

> **목적**: Hermes `memory_provider.py` + `memory_manager.py` 368 LoC의 Pluggable Memory 아키텍처를 Go로 이식할 때의 결정점, SQLite FTS5 라이브러리 선택, plugin registry 설계를 정리한다.
> **작성일**: 2026-04-21
> **범위**: `internal/memory/` 패키지 + `builtin/` + `plugin/` 서브패키지.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/hermes-agent-main/agent/
memory_provider.py       # ABC + BuiltinProvider
memory_manager.py        # 368 LoC manager
```

- `internal/memory/` → **전부 부재**. Phase 4에서 신규 작성.
- 기존 SPEC 디렉토리 `SPEC-GENIE-MEMORY-001/`이 없음을 `ls .moai/specs/`로 확인 → **신규 SPEC 디렉토리 생성**(v1.0에 없었음).
- 선행 CORE-001의 `GENIE_HOME` 해석, zap 로거, TRAJECTORY-001의 session_id 전제.

---

## 2. hermes-learning.md §6 원문 → SPEC 매핑

hermes-learning.md §6 원문 인터페이스:

```python
class MemoryProvider(ABC):
    """Swappable 메모리 백엔드"""
    
    # 필수
    @abstractmethod
    def name(self) -> str:                    # "builtin", "honcho", "hindsight", "mem0"
    @abstractmethod
    def is_available(self) -> bool:           # 설정 + 자격증명 (네트워크 X)
    @abstractmethod
    def initialize(self, session_id, **kwargs):
        # kwargs: hermes_home, platform, agent_context, agent_identity
    @abstractmethod
    def get_tool_schemas(self) -> List[Dict]:
    
    # 선택
    def system_prompt_block(self) -> str
    def prefetch(self, query, *, session_id) -> str        # 회상 (빠름)
    def queue_prefetch(self, query, *, session_id)         # 백그라운드
    def sync_turn(self, user_content, assistant_content, *, session_id)
    def handle_tool_call(self, tool_name, args, **kwargs) -> str  # JSON
    
    # Lifecycle hooks
    def on_turn_start(self, turn_number, message, **kwargs)
    def on_session_end(self, messages)
    def on_pre_compress(self, messages) -> str
    def on_delegation(self, task, result, **kwargs)
```

**MemoryManager**:
- Builtin 항상 첫 번째 (제거 불가)
- 최대 1개 외부 plugin provider
- Tool schema 수집 (이름 충돌 검출)
- 실패 격리 (한 provider 오류 ≠ 차단)

매핑 표:

| Hermes 요소 | 본 SPEC | REQ/AC |
|---|---|---|
| `MemoryProvider` ABC | `MemoryProvider` interface | REQ-003, REQ-005 |
| `name()` | `Name()` | REQ-003 |
| `is_available()` | `IsAvailable()` | REQ-004 |
| `initialize(session_id, **kwargs)` | `Initialize(sessionID, SessionContext) error` | REQ-006 |
| `get_tool_schemas() -> List[Dict]` | `GetToolSchemas() []ToolSchema` | REQ-015 |
| `system_prompt_block()` | `SystemPromptBlock() string` | REQ-010 |
| `prefetch(query, session_id)` | `Prefetch(query, sessionID) (RecallResult, error)` | AC-013 |
| `queue_prefetch(...)` | `QueuePrefetch(query, sessionID)` | REQ-018 |
| `sync_turn(user, assistant, session_id)` | `SyncTurn(sessionID, user, assistant) error` | AC-015 |
| `handle_tool_call(name, args) -> str` | `HandleToolCall(name, args, ToolContext) (string, error)` | REQ-009 |
| `on_turn_start(turn, message)` | `OnTurnStart(sessionID, turn, msg)` | REQ-007 |
| `on_session_end(messages)` | `OnSessionEnd(sessionID, messages)` | REQ-008 |
| `on_pre_compress(messages) -> str` | `OnPreCompress(sessionID, messages) string` | REQ-011 |
| `on_delegation(task, result)` | `OnDelegation(sessionID, task, result)` | (선택) |
| Builtin 필수 1순위 | REQ-001 | AC-001 |
| 최대 1 plugin | REQ-002 | AC-002 |
| Tool name 충돌 검출 | REQ-015 | AC-004 |
| 실패 격리 | REQ-007 (panic recover) | AC-007 |

Hermes와의 주요 차이:
- **session_id가 모든 메서드 파라미터로 승격**: Go는 Python `**kwargs`보다 명시성 선호. 메서드 서명이 길어지지만 호출 시 누락 불가.
- **`ctx context.Context` 추가**: 타임아웃 / cancellation이 Go 이디엄.
- **`SessionContext` struct 분리**: Python의 `**kwargs`를 type-safe struct로.
- **`BaseProvider` 임베드 패턴**: Python의 "선택 메서드에 default 구현"을 Go 임베딩으로 구현.

---

## 3. SQLite 라이브러리 결정

### 3.1 후보 비교

| 라이브러리 | CGO | FTS5 | 순수 Go | cross-compile | 속도 |
|---|:-:|:-:|:-:|:-:|---|
| **`modernc.org/sqlite`** | ✗ | ✓ (built-in) | ✓ | 쉬움 | 중간 |
| `mattn/go-sqlite3` | ✓ | 빌드 태그 필요 | ✗ | 복잡 | 빠름 |
| `zombiezen.com/go/sqlite` | ✗ | ✓ | ✓ (modernc 기반) | 쉬움 | 중간 |

### 3.2 결정

**`modernc.org/sqlite`** 채택 근거:
- CGO 없음 → Docker/크로스플랫폼 빌드 단순.
- FTS5가 기본 빌드에 포함 (별도 빌드 태그 불필요).
- GENIE의 memory 부하는 I/O bound가 아닌 쿼리 선택성 bound (세션당 facts 수천 개 이하) — 중간 속도로 충분.

**대안 경로**:
- 프로덕션에서 성능 이슈 발생 시 `mattn/go-sqlite3 + fts5` 빌드 태그로 스왑.
- DB 인터페이스는 `database/sql` 표준이므로 이식 부담 낮음.

### 3.3 FTS5 쿼리 디자인

FTS5 기본 쿼리 패턴(AC-MEMORY-013):
```sql
SELECT f.id, f.content, rank
FROM facts f
JOIN facts_fts fts ON f.id = fts.rowid
WHERE f.session_id = ?
  AND facts_fts MATCH ?
ORDER BY rank
LIMIT ?;
```

MATCH 표현:
- `'Go'` → 단일 단어
- `'Go OR Rust'` → OR
- `'Go systems'` → 양쪽 포함
- `'"exact phrase"'` → 완전 매칭

tokenize 옵션: `porter unicode61`
- `porter`: 영어 stemming (learning / learned → learn)
- `unicode61`: 유니코드 정규화 (한국어/일본어/중국어 지원)

---

## 4. Plugin Registry 설계

### 4.1 Registry 패턴

```go
// internal/memory/plugin/registry.go

type Factory func(cfg json.RawMessage, logger *zap.Logger) (memory.MemoryProvider, error)

var factories = map[string]Factory{}

// Register는 init()에서 plugin이 호출.
func Register(name string, f Factory) {
    if _, exists := factories[name]; exists {
        panic("duplicate plugin registration: " + name)
    }
    factories[name] = f
}

// Instantiate는 manager.New가 config.memory.plugin.name으로 호출.
func Instantiate(name string, cfg json.RawMessage, logger *zap.Logger) (memory.MemoryProvider, error) {
    f, ok := factories[name]
    if !ok {
        return nil, fmt.Errorf("%w: %q", memory.ErrUnknownPlugin, name)
    }
    return f(cfg, logger)
}
```

### 4.2 외부 Plugin 추가 방법

```go
// internal/memory/plugin/honcho/honcho.go (별도 모듈)
package honcho

func init() {
    plugin.Register("honcho", func(cfg json.RawMessage, logger *zap.Logger) (memory.MemoryProvider, error) {
        var c HonchoConfig
        if err := json.Unmarshal(cfg, &c); err != nil {
            return nil, err
        }
        return &HonchoProvider{cfg: c, logger: logger}, nil
    })
}
```

Bootstrap에서 `_ "github.com/genie/memory-honcho"` import로 등록.

### 4.3 config.yaml 예

```yaml
memory:
  builtin:
    db_path: ~/.genie/memory/facts.db
    memory_md: ~/.genie/memory/MEMORY.md
    user_md: ~/.genie/memory/USER.md
  plugin:
    name: honcho              # registry에 등록된 이름
    config:                   # plugin별 자유 포맷
      endpoint: https://api.honcho.dev
      api_key_env: HONCHO_API_KEY
      timeout_ms: 500
```

---

## 5. Python → Go 이식 결정

### 5.1 `**kwargs` 대체

Python `initialize(session_id, **kwargs)`의 유연함 → Go는 **struct**:

```go
type SessionContext struct {
    HermesHome    string
    Platform      string
    AgentContext  map[string]string
    AgentIdentity string
}
```

이유:
- 컴파일 타임 field 검증.
- 자동 문서화 (godoc에 필드 설명).
- 테스트에서 `SessionContext{Platform: "darwin"}` 명시적 초기화.

### 5.2 선택 메서드 default 구현

Python ABC는 선택 메서드에 default no-op 제공 가능. Go는 interface에 default 구현 불가 — **임베드 패턴**으로 해결:

```go
type BaseProvider struct{}
func (BaseProvider) SystemPromptBlock() string { return "" }
// ... 모든 선택 메서드 no-op

type HonchoProvider struct {
    memory.BaseProvider   // 선택 메서드 기본 no-op 상속
    // 필수 메서드만 override
}
func (h *HonchoProvider) Name() string                  { return "honcho" }
func (h *HonchoProvider) IsAvailable() bool             { /* ... */ }
func (h *HonchoProvider) Initialize(...) error          { /* ... */ }
func (h *HonchoProvider) GetToolSchemas() []ToolSchema  { /* ... */ }
// SystemPromptBlock 등은 자동으로 Base의 ""
```

### 5.3 Panic 격리

Python은 `try/except`로 단순. Go는 `recover()` + goroutine:

```go
go func(prov MemoryProvider) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("panic", zap.Any("recovered", r), zap.String("provider", prov.Name()))
        }
    }()
    prov.OnTurnStart(sessionID, turn, msg)
}(p)
```

---

## 6. Go 라이브러리 결정

| 용도 | 채택 | 대안 | 근거 |
|---|---|---|---|
| SQLite | `modernc.org/sqlite` v1.29+ | `mattn/go-sqlite3` | CGO 없음, FTS5 기본 포함 |
| JSON | 표준 `encoding/json` | `goccy/go-json` | tool_call args 파싱 간단 |
| 설정 파싱 | CONFIG-001의 YAML | — | plugin.config는 raw JSON으로 전달 |
| Mutex | 표준 `sync.Mutex` | `spf13/sync` | 표준 충분 |
| 테스트 | `testify` + `t.TempDir()` | — | 병렬 테스트 안전 |

---

## 7. 테스트 전략

### 7.1 격리 테스트 환경

- 각 테스트마다 `t.TempDir()`로 격리된 `GENIE_HOME`.
- BuiltinProvider는 `./test.db`(tempdir 하위) 생성.
- Plugin은 stub으로 대체 (`stubProvider`).

### 7.2 Fixture 구성

```go
type stubProvider struct {
    memory.BaseProvider
    name           string
    toolSchemas    []memory.ToolSchema
    onTurnStart    func(sessionID string, turn int, msg memory.Message)
    systemPrompt   string
    isAvail        bool
}
func (s *stubProvider) Name() string                       { return s.name }
func (s *stubProvider) IsAvailable() bool                  { return s.isAvail }
func (s *stubProvider) Initialize(string, memory.SessionContext) error { return nil }
func (s *stubProvider) GetToolSchemas() []memory.ToolSchema { return s.toolSchemas }
func (s *stubProvider) OnTurnStart(sessionID string, turn int, msg memory.Message) {
    if s.onTurnStart != nil { s.onTurnStart(sessionID, turn, msg) }
}
func (s *stubProvider) SystemPromptBlock() string { return s.systemPrompt }
```

### 7.3 SQLite FTS5 테스트

```go
func TestBuiltin_FTS5RecallRanking(t *testing.T) {
    b, _ := NewBuiltin(cfg, logger)
    b.SaveFact(ctx, "s1", "user_lang", "User prefers Rust and Go", ...)
    b.SaveFact(ctx, "s1", "user_os",   "User runs macOS Sonoma",   ...)

    result, _ := b.Prefetch("Rust systems", "s1")
    assert.Len(t, result.Items, 1)
    assert.Equal(t, "user_lang", result.Items[0].Key)
    assert.Greater(t, result.Items[0].Score, 0.0)
}
```

### 7.4 Race 검증

AC-MEMORY-011 (50ms budget)은 타이밍 민감 → `-race` + 반복 실행:

```
go test -race -count=20 -timeout 30s ./internal/memory/...
```

---

## 8. Hermes 재사용 평가

| Hermes 구성요소 | 재사용 가능성 | 재작성 필요 이유 |
|---|---|---|
| `MemoryProvider` ABC 시그니처 | **95% 재사용** | Go interface로 번역, session_id 승격 |
| `MemoryManager` 등록 로직 | **85% 재사용** | Builtin 1순위 + 외부 1개 정책 동일 |
| Builtin SQLite 스키마 | **90% 재사용** | FTS5 triggers 문법 거의 동일 |
| Lifecycle hook 7종 | **100% 재사용** | 호출 시점 동일 |
| Tool schema 수집 | **100% 재사용** | 충돌 검출 정책 동일 |
| 실패 격리 (Python try/except) | **0% 재사용** | Go goroutine + recover로 재작성 |
| Python asyncio prefetch | **0% 재사용** | Go goroutine + channel |
| Plugin registry | **신규 추가** | Python은 동적 import, Go는 init() 등록 패턴 |
| BaseProvider 임베드 | **신규 추가** | Go는 default method 없어서 우회 필요 |

**추정 Go LoC** (hermes-learning.md §10: `memory/{provider,manager,sqlite}: 800 LoC`):
- provider.go (interface + BaseProvider): 100
- manager.go: 280
- types.go: 80
- errors.go: 30
- config.go: 50
- builtin/builtin.go: 180
- builtin/sqlite.go: 220 (스키마 + FTS5)
- builtin/files.go: 120 (MEMORY.md / USER.md)
- builtin/tools.go: 80
- plugin/adapter.go: 60
- plugin/registry.go: 50
- 테스트: 900+
- **합계**: ~1,250 production + 900 test ≈ 2,150 LoC (Hermes 800 대비 56% 증가, 주된 이유: BuiltinProvider 분화 + plugin registry + 테스트 깊이)

---

## 9. 통합 시나리오

### 9.1 부트스트랩 흐름

```go
// cmd/genied/main.go (본 SPEC scope 외)

memCfg := config.Memory
logger := bootstrap.Logger()

manager, err := memory.New(memCfg, logger)
if err != nil { log.Fatal(err) }

// Builtin은 필수
builtin, err := builtin.NewBuiltin(memCfg.Builtin, logger)
if err != nil { log.Fatal(err) }
manager.RegisterBuiltin(builtin)

// Plugin은 optional
if memCfg.Plugin.Name != "" {
    plug, err := plugin.Instantiate(memCfg.Plugin.Name, memCfg.Plugin.Config, logger)
    if err != nil { log.Fatal(err) }
    if err := manager.RegisterPlugin(plug); err != nil { log.Fatal(err) }
}

// QueryEngine에 hook 주입 (QUERY-001 책임)
queryCfg.PostSamplingHooks = append(queryCfg.PostSamplingHooks, manager.QueryHook())
```

### 9.2 tool_call 라우팅

```go
// QueryEngine queryLoop 내부 (QUERY-001 scope, 참고만)

toolName := req.ToolUse.Name
if manager.OwnsTool(toolName) {
    result, err := manager.HandleToolCall(ctx, toolName, req.ToolUse.Input, toolCtx)
    return synthesizeResult(result, err)
}
// 일반 tools.Registry로
```

---

## 10. hermes-learning.md §12 Layer 매핑

```
Layer 1: Trajectory 수집      ← TRAJECTORY-001
Layer 2: 압축                 ← COMPRESSOR-001
Layer 3: Insights 추출        ← INSIGHTS-001
Layer 4: Memory 저장          ← 본 SPEC
Layer 5: Skill/Prompt 자동 진화 ← REFLECT-001 (Phase 5)
```

본 SPEC의 `MemoryManager.SyncTurn`, `OnSessionEnd`가 Layer 4의 주된 진입점이다. TRAJECTORY-001(Layer 1)과 INSIGHTS-001(Layer 3)은 파일 기반 교환이지만, MEMORY-001(Layer 4)은 QueryEngine과 직접 결합된 in-process 레이어다.

---

## 11. 향후 SPEC 연계

- **SPEC-GENIE-IDENTITY-001 (Phase 6)**: Builtin을 확장하여 Kuzu graph + POLE+O 스키마 추가. `memory_graph_query` tool 노출.
- **SPEC-GENIE-VECTOR-001 (Phase 6)**: 별도 VectorProvider로 등록하는 옵션 고려 (단, Phase 4에서는 최대 1 plugin 제약). Phase 6에서 "최대 N plugin" 제약 완화 가능성 검토.
- **SPEC-GENIE-REFLECT-001 (Phase 5)**: graduated learning을 Builtin.facts 테이블에 confidence 컬럼으로 영속. status 컬럼 추가(observation/heuristic/rule/graduated/rejected).
- **SPEC-GENIE-MCP-001**: Memory tool들을 MCP 서버로 외부 노출. MCP-001이 `manager.GetAllToolSchemas()`를 import.
- **SPEC-GENIE-INSIGHTS-001**: `facts.created_at` / `source` 컬럼 집계로 "최근 30일 사실 추가 추이" 리포트.

---

**End of Research**

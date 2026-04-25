---
id: SPEC-GOOSE-AGENT-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-AGENT-001 — Agent Runtime 최소 생애주기 + Persona

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0 row 05) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT 대화 루프의 **최소 Agent Runtime과 Persona 모델**을 정의한다. 본 SPEC은 Tool Registry(Tool-001), Memory(MEMORY-001), Learning Engine(REFLECT-001) 등 후속 SPEC의 consumer가 접속할 **인터페이스 계약**과 **단일 LLM 호출 루프**를 확정한다.

수락 조건 통과 시점에서:

- YAML로 정의된 `Agent` persona를 로드한다 (`agents/*.yaml`, CORE-001 `GOOSE_HOME` 하위 또는 프로젝트 로컬).
- 사용자 메시지를 받아 `LLMProvider`(LLM-001)로 단일 completion 호출 후 응답을 반환하는 최소 대화 루프가 동작한다.
- Conversation history가 in-memory slice로 유지되며, context window에 맞추어 슬라이딩 트림(FIFO) 된다.
- Tool calling은 **본 SPEC에서 구현하지 않는다** — 인터페이스 훅(`ToolInvoker`)만 제공하고 TOOL-001이 구현한다.
- Learning hook은 **본 SPEC에서 구현하지 않는다** — `InteractionObserver` 인터페이스 훅만 제공.

---

## 2. 배경 (Background)

### 2.1 왜 Agent Runtime이 Phase 0에

- ROADMAP §4 Phase 0 row 05는 "Agent Runtime 최소 생애주기 + Persona"를 P0로 지정. CLI-001이 "대화 요청 → 응답"의 end-to-end를 성립시키려면 Agent Runtime이 필수.
- `.moai/project/structure.md` §1 `internal/agent/`가 `manifest.go`, `lifecycle.go`, `conversation.go`, `persona.go`, `budget.go`, `message.go`, `executor.go` 로 설계됨. 본 SPEC은 그중 **manifest + lifecycle + conversation + persona + message**만 다룬다(`budget.go`, `executor.go`는 후속 SPEC).
- `.moai/project/product.md` §2는 "single digital twin" 비전을 제시. 여러 agent가 협업하는 그림은 Phase 3+. Phase 0은 **단일 agent + 단일 세션**.

### 2.2 Hermes `cli.py` 에이전트 루프 참조

- `hermes-agent-main/cli.py` (409KB)가 agent 루프를 단일 파일에 포함. 본 SPEC은 그 구조를 **작은 모듈로 분해 재작성**.
- Hermes의 `trajectory.py`(2KB)가 상호작용 기록 패턴을 정의 → 본 SPEC의 `Interaction` 구조에 영감.

### 2.3 범위 경계

- **IN**: `Agent` 인터페이스 + `AgentSpec` YAML 파싱, `Conversation`(in-memory history), `Persona`(system prompt 소스), 단일 turn `Ask()` 메서드 구현, context 트림, Learning/Tool hook 훅(빈 구현).
- **OUT**: Tool 실제 실행(TOOL-001), Memory persistence(MEMORY-001), Subagent spawn, Multi-agent coordination, Streaming UI(CLI-001이 Stream을 소비), 토큰 예산(TOKEN economy), Budget overdraft protection, File/shell tool, Vision.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/agent/agent.go`: `Agent` 인터페이스 + 기본 구현 `*defaultAgent`.
2. `internal/agent/manifest.go`: `AgentSpec` 구조 + YAML 파싱 (`agents/<name>.yaml`).
3. `internal/agent/persona.go`: Persona(system prompt) 빌드 로직 (정적 템플릿).
4. `internal/agent/conversation.go`: `Conversation` 구조 (in-memory history slice + thread-safe append/trim).
5. `internal/agent/message.go`: `Message` 타입(LLM-001의 `Message`와 구분: agent-level에는 metadata 포함).
6. `internal/agent/lifecycle.go`: `Create → Load → Ask → Close` 4-단계 상태 machine.
7. `internal/agent/registry.go`: 로드된 agent 이름 → 인스턴스 매핑 (단일 프로세스 scope).
8. Context window 관리: FIFO 트림 (가장 오래된 user/assistant 쌍부터 제거), reserved tokens 계산.
9. Hook interfaces (구현 없이 인터페이스만):
   - `ToolInvoker` — `Invoke(ctx, name, args) (string, error)` — 기본 구현 `NoopToolInvoker`.
   - `InteractionObserver` — `OnInteraction(Interaction)` — 기본 구현 `NoopObserver`.
10. `Agent.Ask(ctx, userMsg) (string, error)`: 단일 turn 동기 호출. Stream 버전 `AskStream(ctx, userMsg) (StreamReader, error)`도 제공.

### 3.2 OUT OF SCOPE

- Tool calling 실제 실행 (TOOL-001).
- Memory persistence, Context recall (MEMORY-001).
- Learning engine integration (REFLECT-001, FEEDBACK-001).
- Subagent spawn (`agent-spawn` tool, Phase 3+).
- Multi-agent team orchestration.
- 토큰 budget 추적 (`internal/agent/budget.go`).
- Cost tracking ($/request).
- Safety guardrails (content moderation).
- Retry with summarization (context overflow 자동 요약).
- User session multiplexing(여러 사용자).
- Persona evolution / runtime adjustment.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-AG-001 [Ubiquitous]** — Every `Agent` instance **shall** hold an immutable `AgentSpec` reference; runtime mutation of persona fields is prohibited (re-create the agent instead).

**REQ-AG-002 [Ubiquitous]** — The `Conversation` structure **shall** be safe for concurrent append from a single user goroutine and concurrent read from observer goroutines, using a `sync.RWMutex`.

**REQ-AG-003 [Ubiquitous]** — The agent **shall** prepend the persona's `SystemPrompt` as the first `Message{Role:"system"}` on every LLM call; this system message is not stored in the conversation history (avoid duplication).

### 4.2 Event-Driven

**REQ-AG-004 [Event-Driven]** — **When** `Agent.Ask(ctx, userMsg)` is invoked, the runtime **shall** (a) append `{role:user, content:userMsg}` to history, (b) build the LLM message list (system + trimmed history), (c) call `LLMProvider.Complete`, (d) append `{role:assistant, content:resp.Text}` to history, (e) invoke `ToolInvoker.Invoke` **only if** `resp` indicates a tool call (Phase 0: never, because Phase 0 LLM has no tool support in request), and (f) return `resp.Text`.

**REQ-AG-005 [Event-Driven]** — **When** the combined token count of (persona + history + new user message) exceeds `Capabilities.MaxContextTokens - ReservedCompletionTokens`, the runtime **shall** drop the oldest user/assistant pair from history and retry the count; repeat until it fits or only the most recent exchange remains.

**REQ-AG-006 [Event-Driven]** — **When** the LLM returns a non-retryable error (e.g., `ErrModelNotFound`), `Ask()` **shall** return the error wrapped in `AgentError{AgentName, Cause}` without mutating history (the user message is rolled back).

### 4.3 State-Driven

**REQ-AG-007 [State-Driven]** — **While** an `Agent` is in `Loading` state, calls to `Ask()` **shall** return `ErrAgentNotReady` immediately.

**REQ-AG-008 [State-Driven]** — **While** an `Agent`'s `Capabilities` for its model have not been queried, the first `Ask()` **shall** block to fetch capabilities via `LLMProvider.Capabilities()`; subsequent calls use the cached value.

### 4.4 Unwanted Behavior

**REQ-AG-009 [Unwanted]** — **If** `AgentSpec.SystemPrompt` is empty, **then** `Load()` **shall** return `ErrInvalidSpec{Field: "system_prompt", Reason: "must not be empty"}`.

**REQ-AG-010 [Unwanted]** — **If** `AgentSpec.Model` references a provider not registered in the LLM registry, **then** `Load()` **shall** return `ErrInvalidSpec{Field: "model", Reason: "provider X not registered"}`.

**REQ-AG-011 [Unwanted]** — The agent **shall not** persist conversation history to disk; all history is in-memory and discarded on process exit. Persistence is MEMORY-001's responsibility.

**REQ-AG-012 [Unwanted]** — The agent **shall not** call any tool or external side effect inside `Ask()` unless the injected `ToolInvoker` is a non-noop implementation explicitly provided by the caller.

### 4.5 Optional

**REQ-AG-013 [Optional]** — **Where** `AgentSpec.Examples []Example` is non-empty, the runtime **shall** append them as alternating user/assistant messages immediately after the system prompt (few-shot prompting).

**REQ-AG-014 [Optional]** — **Where** an `InteractionObserver` is registered, `Ask()` **shall** invoke `observer.OnInteraction(Interaction{...})` after the LLM call completes (both success and failure paths); observer errors **shall not** fail `Ask()`.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-AG-001 — 정상 단일 turn**
- **Given** `AgentSpec{Name:"default", Model:"ollama/qwen2.5:3b", SystemPrompt:"You are helpful."}` 로드됨, mock `LLMProvider`가 고정 응답 `"Hi!"` 반환
- **When** `agent.Ask(ctx, "Hello")`
- **Then** 반환값 `"Hi!"`, conversation history 길이 2(`user:"Hello"`, `assistant:"Hi!"`), mock LLM에 전달된 messages는 `[system, user]` 순서

**AC-AG-002 — System prompt 중복 없음**
- **Given** AC-AG-001 후
- **When** `agent.Ask(ctx, "Again")`
- **Then** LLM에 전달된 messages는 `[system, user("Hello"), assistant("Hi!"), user("Again")]` — system은 여전히 1개만

**AC-AG-003 — Context 트림**
- **Given** `MaxContextTokens=100`, `ReservedCompletionTokens=50`, 현재 history가 system+사용자/어시스턴트 쌍 10개로 80 tokens 차지 중 (합계 80+50=130, 100 초과)
- **When** 새 user 메시지(10 tokens)로 `Ask()`
- **Then** 가장 오래된 user/assistant 쌍부터 제거하여 90 - trim + 10 = 100 이하가 될 때까지 trim. LLM 호출은 1회.

**AC-AG-004 — Empty system prompt 거부**
- **Given** `AgentSpec{SystemPrompt:""}`
- **When** `Load(spec)`
- **Then** `ErrInvalidSpec{Field:"system_prompt"}` 반환, agent 인스턴스 생성 안 됨

**AC-AG-005 — Unknown provider 거부**
- **Given** registry에 `ollama`만 등록. spec `Model:"anthropic/claude-3"`
- **When** `Load(spec)`
- **Then** `ErrInvalidSpec{Field:"model"}`

**AC-AG-006 — LLM 오류 시 history 롤백**
- **Given** mock LLM이 `ErrModelNotFound`를 반환
- **When** `agent.Ask(ctx, "Hello")`
- **Then** 반환 에러는 `AgentError`로 래핑되어 있고 `errors.Is(err, ErrModelNotFound)==true`, history 길이 0 (user 메시지 롤백)

**AC-AG-007 — Observer 호출**
- **Given** 테스트 observer가 `OnInteraction` 호출을 기록
- **When** `Ask(ctx, "X")` 1회
- **Then** observer에 정확히 1번 호출, `Interaction{UserMsg:"X", AssistantMsg:"Hi!", Err:nil}`

**AC-AG-008 — Observer 오류는 Ask 실패로 전파 안 됨**
- **Given** observer가 `OnInteraction`에서 panic 발생
- **When** `Ask(ctx, "X")`
- **Then** `Ask` 반환값은 정상 `"Hi!"`, panic은 내부에서 recover되어 WARN 로그 1건

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/agent/
├── agent.go             # Agent interface + defaultAgent
├── manifest.go          # AgentSpec + YAML loader
├── persona.go           # Persona 빌드 (system prompt, examples)
├── conversation.go      # Conversation struct + Trim()
├── message.go           # Message type (agent-level)
├── lifecycle.go         # state machine: Created|Loading|Ready|Closed
├── registry.go          # name → Agent
├── observer.go          # InteractionObserver + noop
├── tool.go              # ToolInvoker + noop
├── errors.go            # AgentError, ErrInvalidSpec, ErrAgentNotReady
└── *_test.go

agents/                   # $GOOSE_HOME/agents/ (런타임 탐색)
└── default.yaml          # 기본 agent 예시

agents/builtin/           # 레포 내 builtin (빌드 시 embed)
└── default.yaml
```

### 6.2 AgentSpec 스키마 (YAML)

```yaml
# agents/default.yaml
name: default
version: 1
model: ollama/qwen2.5:3b
system_prompt: |
  You are GOOSE, a helpful personal assistant.
  Be concise and factual. Do not invent URLs.
examples:
  - user: "What's the capital of France?"
    assistant: "Paris."
  - user: "Thanks"
    assistant: "Happy to help."
# Tool 목록은 TOOL-001에서 추가
```

`model` 필드는 `provider/model` 문법. `/` 분리하여 registry lookup.

### 6.3 Agent 인터페이스

```go
type Agent interface {
    Name() string
    Spec() AgentSpec
    Ask(ctx context.Context, userMsg string) (string, error)
    AskStream(ctx context.Context, userMsg string) (llm.StreamReader, error)
    History() []Message // 복사본 반환 (mutation 방지)
    Close() error
}
```

### 6.4 Context Window 관리

- Token 카운팅 방법: 첫 호출 시 `LLMProvider.Capabilities(model)`로 `MaxContextTokens` 조회.
- **근사 카운팅**: Phase 0은 서버 측 tokenizer 없이 `len(string)/4` 같은 rough 근사. Ollama가 응답에서 실제 카운트를 알려주므로, 다음 turn에서 **이전 turn의 실제 카운트**를 기반으로 보정.
- `ReservedCompletionTokens`는 `AgentSpec.Generation.MaxTokens` 또는 기본 512.
- Trim 알고리즘: FIFO — system/examples는 유지, history 리스트의 가장 오래된 `[user, assistant]` 쌍부터 제거.

### 6.5 AskStream 구현

```go
func (a *defaultAgent) AskStream(ctx context.Context, userMsg string) (llm.StreamReader, error) {
    msgs := a.buildMessages(userMsg)
    req := llm.CompletionRequest{Model: a.modelName, Messages: msgs}
    reader, err := a.provider.Stream(ctx, req)
    if err != nil { return nil, a.wrapError(err) }
    return &recordingReader{inner: reader, onClose: func(finalText string, usage llm.Usage) {
        a.appendHistory("user", userMsg)
        a.appendHistory("assistant", finalText)
        a.observer.OnInteraction(Interaction{...})
    }}, nil
}
```

`recordingReader`는 inner의 Next 호출을 passthrough하면서 최종 close 시 history를 한번에 append. **중도 취소 시 history는 업데이트 안 함**(user 메시지도 포함 안 됨).

### 6.6 Observer panic 보호

```go
func (a *defaultAgent) notifyObserver(ix Interaction) {
    defer func() {
        if r := recover(); r != nil {
            a.logger.Warn("observer panic recovered", zap.Any("panic", r))
        }
    }()
    a.observer.OnInteraction(ix)
}
```

### 6.7 Registry + Factory

```go
func LoadFromDir(dir string, provReg *llm.Registry, logger *zap.Logger) (*Registry, error)
```

- `$GOOSE_HOME/agents/*.yaml` + 레포 builtin `embed.FS`를 모두 스캔, 중복 name 시 $GOOSE_HOME 우선.

### 6.8 TDD 진입

1. **RED**: `TestLoad_ValidSpec_Success`.
2. **RED**: `TestLoad_EmptySystemPrompt_Rejected` → AC-AG-004.
3. **RED**: `TestLoad_UnknownProvider_Rejected` → AC-AG-005.
4. **RED**: `TestAsk_SingleTurn` → AC-AG-001.
5. **RED**: `TestAsk_SystemPromptNotDuplicated` → AC-AG-002.
6. **RED**: `TestAsk_TrimsContextWindow` → AC-AG-003.
7. **RED**: `TestAsk_LLMError_RollsBackHistory` → AC-AG-006.
8. **RED**: `TestAsk_InvokesObserver` → AC-AG-007.
9. **RED**: `TestAsk_ObserverPanicRecovered` → AC-AG-008.
10. **GREEN** → **REFACTOR**.

### 6.9 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| Tested | mock `LLMProvider` 테스트 + in-memory conversation 검증, observer table-driven |
| Readable | `Agent` interface에 각 메서드 계약 주석, `manifest.go`는 YAML 스키마 링크 포함 |
| Unified | `golangci-lint` + spec 태그 통일(`yaml:"snake_case"`) |
| Secured | conversation 디스크 저장 금지(REQ-AG-011), persona 불변(REQ-AG-001) |
| Trackable | `AgentError`가 agent name 포함, observer hook으로 외부 관측 경로 확보 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-CORE-001** | zap logger, context 전파 |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | `Config` 일부 — agents dir 경로 등 |
| 선행 SPEC | **SPEC-GOOSE-LLM-001** | `LLMProvider`, `StreamReader`, `Usage` 소비 |
| 후속 SPEC | SPEC-GOOSE-TOOL-001 | `ToolInvoker` 구현체 |
| 후속 SPEC | SPEC-GOOSE-MEMORY-001 | Conversation persistence |
| 후속 SPEC | SPEC-GOOSE-TELEM-001 | `InteractionObserver` 구현체 |
| 후속 SPEC | SPEC-GOOSE-FEEDBACK-001 | observer 체인 연결 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | `AskStream` 소비자 |
| 외부 | `gopkg.in/yaml.v3` | manifest 파싱 |
| 외부 | 표준 `embed` | builtin agents 번들 |
| 외부 | `github.com/stretchr/testify` | 테스트 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 근사 token count 오차로 context overflow 발생 | 중 | 중 | 다음 turn에서 실제 eval_count로 보정, 안전 마진 20% 추가 |
| R2 | conversation slice 무제한 성장 시 메모리 증가 | 중 | 중 | Trim 알고리즘이 매 turn 실행, MaxHistory 하드 캡(100) 추가 |
| R3 | Tool/Observer hook 인터페이스가 후속 SPEC 요구와 불일치 | 중 | 높 | 본 SPEC v0.1을 "임시"로 간주, TOOL-001 구현 시 인터페이스 v0.2 허용 |
| R4 | Agent 상태 머신의 race condition (동시 Ask) | 중 | 중 | Phase 0은 단일 사용자 가정, `sync.RWMutex` + 테스트로 방어 |
| R5 | YAML spec에 신뢰할 수 없는 내용 주입(prompt injection 공격) | 낮 | 중 | Phase 0 agents는 사용자 로컬 제어, 주의 문구만 문서화 |
| R6 | Observer가 블로킹하여 Ask 레이턴시 증가 | 중 | 중 | Observer 호출은 goroutine(`go a.notifyObserver(ix)`)으로 비동기 기본, 동기 옵션도 제공 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/structure.md` §1 `internal/agent/`, §3 (Tier 1 Personal agents)
- `.moai/project/product.md` §2 (single digital twin 비전)
- `.moai/project/tech.md` §3.1 (zap), §3.2 (`anthropic-sdk-go` 등은 후속)
- `.moai/specs/SPEC-GOOSE-LLM-001/spec.md` §3.1 IN SCOPE 항목 1-6

### 9.2 외부 참조

- Hermes `cli.py` agent 루프 구조 (파일 크기 409KB — 본 SPEC은 이를 모듈화된 4-5개 파일로 분해)
- Google ADK-Go `Agent` 인터페이스 참고 (사용 안 함, 참조만)
- Anthropic prompt caching 설계 (Phase 0 제외)

### 9.3 부속 문서

- `./research.md`
- `../ROADMAP.md` §4 Phase 0 row 05

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Tool 실제 실행(파일 read/write, shell, web fetch)을 구현하지 않는다**. TOOL-001.
- 본 SPEC은 **Memory persistence / recall을 구현하지 않는다**. MEMORY-001.
- 본 SPEC은 **Subagent spawn / inter-agent messaging을 구현하지 않는다**.
- 본 SPEC은 **Learning engine 통합(feedback detector, pattern miner)을 포함하지 않는다**. 인터페이스 hook만.
- 본 SPEC은 **토큰 비용 추적 / 예산 차감을 구현하지 않는다**.
- 본 SPEC은 **Content moderation / safety guardrails를 구현하지 않는다**.
- 본 SPEC은 **Automatic summarization on overflow를 구현하지 않는다**. 오래된 history는 단순 drop.
- 본 SPEC은 **Vision / multi-modal input을 지원하지 않는다**.
- 본 SPEC은 **Persistent session / resume across process restarts를 지원하지 않는다**.
- 본 SPEC은 **여러 사용자를 구분하지 않는다**. Single-tenant in-memory.
- 본 SPEC은 **스트리밍 UI rendering을 정의하지 않는다**. CLI-001이 담당.

---

**End of SPEC-GOOSE-AGENT-001**

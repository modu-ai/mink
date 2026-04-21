# SPEC-GOOSE-AGENT-001 — Research & Inheritance Analysis

> **목적**: Agent Runtime과 Persona 모델 구현을 위한 자산 조사.
> **작성일**: 2026-04-21

---

## 1. 레포 상태

`internal/agent/` 부재. `agents/*.yaml` 부재. Go 소스 0. **신규 작성**.

---

## 2. 참조 자산별 분석

### 2.1 Hermes Agent (`./hermes-agent-main/`) — 핵심 참조

Hermes는 가장 유사한 에이전트 루프 구조를 가진 자산이다.

```
hermes-agent-main/
├── cli.py                         # 409KB (!!)
├── agent/
│   ├── trajectory.py              # 2KB — interaction 기록
│   ├── memory_manager.py          # 다른 SPEC(MEMORY-001)에서 참조
│   ├── memory_provider.py         # interface 참조
│   ├── context_compressor.py      # Phase 0 OUT OF SCOPE(자동 요약)
│   ├── skill_utils.py             # Phase 3+
│   └── ...
└── tools/                         # 58 sub-dirs — TOOL-001에서 참조
```

`cli.py` 부분 탐색:

```
$ grep -n "def.*agent\|def.*chat\|class.*Agent\|class.*Conversation" hermes-agent-main/cli.py | head -30
(결과: 단일 거대 함수에 agent 루프 interleaved, 디자인 패턴 추출 어려움)
```

**핵심 교훈**:
- Hermes `cli.py`가 저지른 **실수**: 단일 파일 409KB에 모든 로직 혼재. 본 SPEC은 4-5개 모듈(`agent.go`, `manifest.go`, `persona.go`, `conversation.go`, `lifecycle.go`)로 **처음부터** 분해.
- `trajectory.py`의 `Interaction` 스키마(`user_msg, assistant_msg, timestamp, duration`)는 본 SPEC의 `Interaction` 구조로 그대로 차용 가능.

**직접 포트**: 없음 (Python → Go). 설계 원칙만.

### 2.2 Claude Code TypeScript (`./claude-code-source-map/`)

```
$ ls claude-code-source-map | head
bootstrap/  entrypoints/  bridge/  sdk/  services/  tools/  ...
```

Claude Code의 대화 루프는 `services/` 또는 `sdk/query.ts`에 분산:

- Agent 메타데이터는 `.claude/agents/*.md` frontmatter에서 로드 → YAML 스펙 패턴과 유사.
- Message history는 session 기반(DB persist) → Phase 0 OUT OF SCOPE와 대조.
- Tool registry는 동적 ("AllowedTools") → TOOL-001 참조.

**직접 포트**: 없음. frontmatter 파싱 패턴만 참고.

### 2.3 Google ADK-Go (`google/adk-go`, 외부, tech.md §3.2)

외부 레포. 본 AgentOS 내 미러 없음. tech.md가 Go AI 생태계 참조로 명시.

- ADK의 `Agent` 인터페이스는 `Run(ctx, input) (output, error)` 단일 메서드 중심.
- 본 SPEC `Ask(ctx, userMsg) (string, error)`는 동일 모티브.

**직접 포트**: 없음. 인터페이스 모티브만.

### 2.4 MoAI-ADK `.claude/agents/*.md`

현 레포 `.claude/agents/` 하 실제 agent 정의 다수 존재:

```
$ ls /Users/goos/MoAI/AgentOS/.claude/agents/ | head
builder-agent/  builder-plugin/  builder-skill/  evaluator-active/  ...
```

각 agent는 `frontmatter + body` 구조의 Markdown. 본 SPEC의 YAML 변형 대신 Markdown+frontmatter를 쓸 수도 있으나:

- **YAML 선택 이유**: structure.md §1 `agents/*.yaml`로 이미 명문화. MoAI의 Markdown agent는 Claude Code 전용 개념이므로 GOOSE의 런타임 agent와 용어 구분.
- 향후 MoAI agents를 GOOSE가 읽을 필요가 생기면 adapter를 별도 SPEC에서 추가.

---

## 3. Go 이디엄

### 3.1 Agent state machine

```go
type agentState int32  // atomic.Int32로 관리

const (
    stateCreated agentState = iota
    stateLoading
    stateReady
    stateClosed
)
```

- Ask()는 `stateReady`만 허용. 다른 상태에서는 `ErrAgentNotReady` 즉시 반환.
- Load는 `Created → Loading → Ready`. 실패 시 `Created`로 복귀.

### 3.2 Conversation 구조

```go
type Conversation struct {
    mu       sync.RWMutex
    messages []Message
    cap      int // MaxHistory hard cap
}

func (c *Conversation) Append(m Message)
func (c *Conversation) Snapshot() []Message // deep copy
func (c *Conversation) Trim(maxTokens int, tokenCount func(m Message) int) int // returns dropped count
```

- `Snapshot`은 반드시 deep copy (외부에서 mutation하지 못하도록).
- Trim은 system/examples를 제외한 user/assistant 쌍 단위로.

### 3.3 YAML spec 로딩

```go
type AgentSpec struct {
    Name         string    `yaml:"name"`
    Version      int       `yaml:"version"`
    Model        string    `yaml:"model"`          // "ollama/qwen2.5:3b"
    SystemPrompt string    `yaml:"system_prompt"`
    Examples     []Example `yaml:"examples,omitempty"`
    Generation   *GenOpts  `yaml:"generation,omitempty"`
}
type Example struct {
    User      string `yaml:"user"`
    Assistant string `yaml:"assistant"`
}
```

`yaml.v3` `decoder.KnownFields(true)`로 스키마 drift 차단.

### 3.4 Model routing

`spec.Model == "ollama/qwen2.5:3b"` → split by `/` → `provider="ollama"`, `modelName="qwen2.5:3b"`.

- Agent 객체 생성 시 `provider := llmRegistry.Get("ollama")` 한 번만.
- Reflection 없는 정적 routing.

### 3.5 Observer 비동기 호출

```go
func (a *defaultAgent) notifyObserverAsync(ix Interaction) {
    go func() {
        defer a.recoverObserverPanic()
        a.observer.OnInteraction(ix)
    }()
}
```

- 기본값 비동기 (R6 완화).
- 테스트용 동기 옵션(`AgentConfig.SyncObserver=true`) 제공해 AC-AG-007 검증 가능.

### 3.6 Builtin agents (embed.FS)

```go
//go:embed agents/builtin/*.yaml
var builtinAgents embed.FS
```

`default.yaml`은 embed → 유저가 `$GOOSE_HOME/agents/default.yaml`을 override해도 작동.

---

## 4. 외부 의존성 합계

| 모듈 | 용도 | 채택 |
|------|------|-----|
| `gopkg.in/yaml.v3` | spec 파싱 | ✅ |
| 표준 `embed` | builtin agents | ✅ |
| `go.uber.org/zap` | logging | ✅ (공유) |
| `github.com/stretchr/testify` | 테스트 | ✅ |
| `github.com/google/uuid` | Interaction ID | ⚠️ 선택 (나중에 추가 가능) |
| `github.com/pkoukk/tiktoken-go` | token 카운팅 | ❌ (Phase 0은 근사, Ollama가 실제 count 제공) |
| `github.com/google/adk-go` | Google ADK | ❌ (참조만) |

---

## 5. Persona 디자인 결정

### 5.1 Static vs Dynamic

- **Static**: YAML에 고정 system_prompt. 본 SPEC 채택.
- **Dynamic**: 사용자 프로필, 시간, 상태 등을 바인딩한 템플릿. REFLECT-001/MEMORY-001 완성 후.

### 5.2 Examples (few-shot)

`AgentSpec.Examples` 옵션으로 시작. 많은 모델이 system 이후 user/assistant 쌍 몇 개를 fictive history로 주면 톤 정확도 향상. Phase 0에서 필수 아님.

### 5.3 Tool 목록

AgentSpec.Tools는 **포함하지 않음**. Tool 지원은 TOOL-001에서 인터페이스 추가. Phase 0 agents는 도구 없이 순수 대화.

---

## 6. 테스트 전략

### 6.1 Unit 테스트

- `TestSpec_LoadValidYaml_Success`
- `TestSpec_LoadEmptySystemPrompt_Rejected`
- `TestSpec_LoadMalformedYaml_Error`
- `TestSpec_LoadUnknownField_Rejected` (KnownFields strict)
- `TestConversation_AppendAndSnapshot`
- `TestConversation_Trim_DropsOldestPair`
- `TestConversation_Trim_PreservesSystem`
- `TestLifecycle_StateTransitions`
- `TestLifecycle_AskBeforeLoad_NotReady`
- `TestRouting_ModelStringSplit`
- `TestRouting_UnknownProvider_Error`

### 6.2 Integration with mock LLMProvider

```go
type fakeLLM struct {
    nextResp llm.CompletionResponse
    nextErr  error
    calls    []llm.CompletionRequest
}
```

- `TestAsk_SingleTurn_E2E` → AC-AG-001.
- `TestAsk_SystemNotDuplicated_E2E` → AC-AG-002.
- `TestAsk_ContextTrim_E2E` → AC-AG-003.
- `TestAsk_LLMError_HistoryRolledBack_E2E` → AC-AG-006.
- `TestAsk_InvokesObserver_Sync_E2E` → AC-AG-007.
- `TestAsk_ObserverPanicRecovered_E2E` → AC-AG-008.
- `TestAskStream_PassThrough_E2E`.
- `TestAskStream_CancelMidStream_NoHistoryAppend` (stream 취소 시 history append 금지).

### 6.3 Race

`go test -race` 필수. Conversation은 RWMutex.

### 6.4 커버리지 목표

- `internal/agent/`: 90%+
- `conversation.go`: 95%+ (trim 알고리즘)
- `lifecycle.go`: 100%

---

## 7. 오픈 이슈

1. **근사 token 카운팅의 정확도**: `len(string)/4` 스타일은 중국어/한국어에서 10~30% 과소추정. Phase 0 수용, 추후 실제 tokenizer 통합 SPEC.
2. **Example 메시지의 언어**: builtin `default.yaml`의 examples를 영어/한국어 중 무엇으로 줄지. 본 SPEC은 영어(글로벌 기본)로 결정, locale별 variant는 후속 작업.
3. **ToolInvoker 인터페이스 수준**: `Invoke(ctx, name, args)` 외 `List()`, `Schema(name)` 필요 여부. Phase 0은 최소 `Invoke`만, TOOL-001에서 확장.
4. **Observer call order**: 여러 observer를 chain할지 단일 observer만 허용할지. 본 SPEC은 단일 slot, `MultiObserver` wrapper를 유틸로 제공하여 체인은 caller가 구성.
5. **Stream path에서 history 롤백**: 취소 시 user 메시지 미append 방침을 유지할지, 아니면 부분 응답을 저장할지. Phase 0은 미append. 향후 UX 판단.

---

## 8. 결론

- **이식 자산**: 없음. Agent Runtime 전부 신규.
- **참조 자산**: Hermes `cli.py`(분해 대상), Hermes `trajectory.py` Interaction 스키마, Claude Code frontmatter 패턴.
- **기술 스택**: stdlib + yaml.v3 + embed + zap. 외부 AI SDK 미사용.
- **구현 규모 예상**: 600~1,000 LoC (테스트 포함 1,400~2,000 LoC).
- **주요 리스크**: 근사 token count(R1), conversation race(R4). 전부 mitigation 명시.

GREEN 완료 시 CLI-001이 `agent.Ask(ctx, userInput)`로 단일 turn 응답을 수신할 수 있고, TOOL-001/MEMORY-001/REFLECT-001 등 후속 SPEC이 각자 hook 인터페이스를 구현해 플러그인될 수 있는 **안정 면(stable surface)**이 확보된다.

---

**End of research.md**

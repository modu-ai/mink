# SPEC-GENIE-ADAPTER-001 — Research

> Anthropic 58KB Python adapter 분해 + 6 provider 통합 전략 + 테스트 설계.

## 1. Hermes Anthropic Adapter 분해 (hermes-llm.md §8 인용)

### 1.1 왜 58KB인가 (원문 인용)

> **큰 이유**:
> 1. OAuth 관리 (PKCE 2.1 + single-use refresh token)
> 2. Token Sync (`~/.claude/.credentials.json`, `~/.hermes/.../hermes-oauth.json`)
> 3. Model Normalization (점, 버전, 별칭)
> 4. Tool Conversion (OpenAI → Anthropic schema)
> 5. Content Conversion (이미지, 코드, thinking 블록)
> 6. Thinking Mode (Adaptive thinking, o1-style)

본 SPEC은 6가지 모두 Go 패키지로 분할:
- `oauth.go` + `token_sync.go` → (1), (2)
- `models.go` → (3)
- `tools.go` → (4)
- `content.go` → (5)
- `thinking.go` → (6)

Go로 재작성 시 SDK(`anthropic-sdk-go`)가 HTTP 레이어를 커버하므로 예상 LoC **2,000** (Hermes Python의 40%).

### 1.2 핵심 함수 인용 (§8)

```python
def read_claude_code_credentials() -> Optional[Dict]
def refresh_anthropic_oauth_pure(refresh_token, *, use_json) -> Dict
def resolve_anthropic_token() -> Optional[str]
def run_hermes_oauth_login_pure() -> Optional[Dict]
def build_anthropic_client(api_key, base_url=None)
def convert_messages_to_anthropic(messages) -> List
def convert_tools_to_anthropic(tools) -> List
def normalize_anthropic_response(response) -> Dict
def _supports_adaptive_thinking(model) -> bool
def _get_anthropic_max_output(model) -> int
```

Go 매핑:
- `read_claude_code_credentials` → `token_sync.ReadClaudeCredentials()`
- `refresh_anthropic_oauth_pure` → `AnthropicAdapter.Refresh()`
- `resolve_anthropic_token` → (풀에 위임, 본 SPEC 불필요)
- `run_hermes_oauth_login_pure` → CLI-001
- `build_anthropic_client` → `anthropic.NewClient(apiKey, baseURL)`
- `convert_messages_to_anthropic` → `content.ConvertMessages()`
- `convert_tools_to_anthropic` → `tools.ConvertOpenAIToAnthropic()`
- `normalize_anthropic_response` → `stream.ToStreamEvents()`
- `_supports_adaptive_thinking` → `thinking.IsAdaptive(model)`
- `_get_anthropic_max_output` → `models.MaxOutputTokens(model)`

## 2. OAuth PKCE 2.1 상세

### 2.1 ~/.claude/.credentials.json 스키마

```json
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oat01-...",
    "refreshToken": "sk-ant-ort01-...",
    "expiresAt": 1746003600000,
    "scopes": ["user:inference", "user:profile"]
  }
}
```

### 2.2 Refresh 엔드포인트

```
POST https://claude.ai/api/oauth/token
Content-Type: application/json

{
  "grant_type": "refresh_token",
  "refresh_token": "sk-ant-ort01-...",
  "client_id": "9d1c250a-e61b-44d9-88ed-5944d1962f5e"  // Claude Code client_id
}
```

응답:
```json
{
  "access_token": "sk-ant-oat01-NEW...",
  "refresh_token": "sk-ant-ort01-NEW...", // rotated (single-use)
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

Single-use refresh token 주의: 이전 refresh_token은 즉시 무효. 쓰기 실패 시 재로그인 필요.

### 2.3 Token sync write-back

```go
func WriteClaudeCredentials(creds *ClaudeCredentials) error {
    path := os.ExpandEnv("$HOME/.claude/.credentials.json")
    tmp := path + ".tmp"
    data, _ := json.MarshalIndent(creds, "", "  ")
    if err := os.WriteFile(tmp, data, 0600); err != nil {
        return err
    }
    return os.Rename(tmp, path) // atomic
}
```

권한: 0600 (사용자 전용).

## 3. Tool Schema 변환

### 3.1 OpenAI function → Anthropic tool

OpenAI:
```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "description": "Get weather",
    "parameters": {
      "type": "object",
      "properties": {"location": {"type": "string"}},
      "required": ["location"]
    }
  }
}
```

Anthropic:
```json
{
  "name": "get_weather",
  "description": "Get weather",
  "input_schema": {
    "type": "object",
    "properties": {"location": {"type": "string"}},
    "required": ["location"]
  }
}
```

변환 규칙:
- `function.name` → top-level `name`
- `function.description` → `description`
- `function.parameters` → `input_schema`

### 3.2 tool_choice 변환

| OpenAI | Anthropic |
|---|---|
| `"none"` | `{"type": "none"}` (지원 안 하는 버전은 tool 자체 제거) |
| `"auto"` | `{"type": "auto"}` |
| `"required"` | `{"type": "any"}` |
| `{"type":"function","function":{"name":"X"}}` | `{"type":"tool","name":"X"}` |

### 3.3 tool_result 변환

OpenAI:
```json
{
  "role": "tool",
  "tool_call_id": "call_abc",
  "content": "..."
}
```

Anthropic:
```json
{
  "role": "user",
  "content": [
    {"type": "tool_result", "tool_use_id": "toolu_abc", "content": "..."}
  ]
}
```

Anthropic은 tool_result를 user message 안에 감쌈.

## 4. Content Block 변환

### 4.1 Text block
그대로 `{"type":"text","text":"..."}`.

### 4.2 Image block

OpenAI:
```json
{"type":"image_url","image_url":{"url":"data:image/png;base64,...."}}
```

Anthropic:
```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/png",
    "data": "..."
  }
}
```

`data:` URL 파싱 → media_type + base64 데이터 분리.

### 4.3 Thinking block

Anthropic 응답에 `thinking` block이 올 수 있음:
```json
{"type":"thinking","thinking":"reasoning..."}
```

본 SPEC의 `StreamEvent{Type:"thinking_delta"}`로 매핑. 최종 assistant 메시지에는 thinking 제외(observational).

## 5. Adaptive Thinking 모델 판정

### 5.1 목록

```go
var adaptiveThinkingModels = map[string]struct{}{
    "claude-opus-4-7":            {},
    "claude-opus-4-7-20260320":   {},
    "claude-opus-4-7[1m]":        {},
    // 추가 모델은 Anthropic 공개 시 업데이트
}
```

### 5.2 Effort level

Opus 4.7: `low | medium | high | xhigh | max`. `budget_tokens`를 설정하면 HTTP 400 반환(CLAUDE.md 인용: "Opus 4.7 rejects fixed budgets").

이전 모델(3.7 Sonnet 등): `budget_tokens: N` 설정, effort 미지원.

```go
func buildThinking(cfg *ThinkingConfig, model string) anthropic.ThinkingParam {
    if cfg == nil || !cfg.Enabled {
        return anthropic.ThinkingParam{Type: "disabled"}
    }
    if _, adaptive := adaptiveThinkingModels[model]; adaptive {
        return anthropic.ThinkingParam{
            Type:   "enabled",
            Effort: cfg.Effort,
        }
    }
    return anthropic.ThinkingParam{
        Type:         "enabled",
        BudgetTokens: cfg.BudgetTokens,
    }
}
```

## 6. Model Alias Normalization

### 6.1 예시 매핑

```go
var anthropicAliases = map[string]string{
    "claude-3-5-sonnet":        "claude-3-5-sonnet-20241022",
    "claude-3-5-haiku":         "claude-3-5-haiku-20241022",
    "claude-3-opus":            "claude-3-opus-20240229",
    "claude-opus-4-7":          "claude-opus-4-7-20260320",
    "claude-opus-4-7-1m":       "claude-opus-4-7[1m]",
}

func Normalize(model string) string {
    if concrete, ok := anthropicAliases[model]; ok {
        return concrete
    }
    return model
}
```

Alias map은 주기적 업데이트 필요. `models.go`에 집중.

## 7. OpenAI-compat 재사용 설계

### 7.1 공통 `openai.Adapter` 구조

```go
// internal/llm/provider/openai/adapter.go
type Options struct {
    Name    string // "openai" | "xai" | "deepseek" | "groq"
    BaseURL string // nil 시 기본 OpenAI
    Pool    *credential.CredentialPool
    Tracker *ratelimit.Tracker
    Logger  *zap.Logger
}

type Adapter struct {
    opts Options
    client *openai.Client // go-openai
}

func NewWithBase(opts Options) *Adapter {
    clientOpts := []openai.Option{}
    if opts.BaseURL != "" {
        clientOpts = append(clientOpts, openai.WithBaseURL(opts.BaseURL))
    }
    // credential은 호출마다 pool.Select 사용
    return &Adapter{opts: opts}
}
```

### 7.2 xAI wrapper

```go
// xai/grok.go (단순 팩토리)
func New(pool *credential.CredentialPool, tracker *ratelimit.Tracker, logger *zap.Logger) *openai.Adapter {
    return openai.NewWithBase(openai.Options{
        Name:    "xai",
        BaseURL: "https://api.x.ai/v1",
        Pool:    pool,
        Tracker: tracker,
        Logger:  logger,
    })
}
```

### 7.3 Capability 차이

```go
// openai: {Streaming:true, Tools:true, Vision:true, Embed:true}
// xai:    {Streaming:true, Tools:true, Vision:true, Embed:false}
// deepseek: {Streaming:true, Tools:true, Vision:false, Embed:false}
// groq:    {Streaming:true, Tools:true, Vision:false, Embed:false}
```

Adapter 인스턴스마다 `Capabilities()` override.

## 8. Ollama 특이사항

### 8.1 API 차이

- 엔드포인트: `POST /api/chat` (OpenAI 호환 `/v1/chat/completions`도 제공하지만 제한 있음)
- Streaming: JSON-L (줄 단위 JSON)
- Tool: v0.1.25+ 지원, 하위 버전은 미지원

### 8.2 인증

- 없음 (localhost:11434)
- CREDPOOL에서 "ollama" provider는 empty entry (authType: "none")

### 8.3 Stream event 변환

```
{"model":"llama3.2","created_at":"...","message":{"role":"assistant","content":"Hi"},"done":false}
→ StreamEvent{Type:"text_delta", Delta:"Hi"}

{"done":true, "total_duration":...}
→ StreamEvent{Type:"message_stop"}
```

## 9. Google Gemini 특이사항

### 9.1 SDK 선택

`google.golang.org/genai` v0.x 공식 SDK. `google.golang.org/api/generativelanguage/v1beta`는 low-level, genai SDK를 권장.

### 9.2 Tool 변환

Gemini function declarations:
```json
{
  "name": "get_weather",
  "description": "...",
  "parameters": {"type":"OBJECT", "properties":...}
}
```

OpenAI와 유사하지만 type 대문자. 별도 변환 함수.

### 9.3 Streaming

gRPC streaming (internal). SDK가 `<-chan Response`로 변환. 어댑터는 각 Response를 StreamEvent로 변환.

## 10. 테스트 전략

### 10.1 httptest stub 기반

```go
func TestAnthropic_Stream_HappyPath(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("anthropic-ratelimit-requests-limit", "50")
        w.Header().Set("anthropic-ratelimit-requests-remaining", "48")
        w.Header().Set("anthropic-ratelimit-requests-reset", "2026-04-21T13:00:00Z")
        w.WriteHeader(200)

        // SSE events
        fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", messageStartJSON)
        fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", cbStartJSON)
        fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", textDelta("Hi"))
        fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", textDelta(" there"))
        fmt.Fprintf(w, "event: message_stop\ndata: {}\n\n")
    }))
    defer server.Close()

    // stub pool with fixed credential, stub tracker, stub cachePlanner
    ...

    ch, err := adapter.Stream(ctx, req)
    require.NoError(t, err)

    events := drainChannel(ctx, ch, 10)
    assert.Contains(t, eventTypes(events), "text_delta")
    assert.Equal(t, "Hi there", concatText(events))
}
```

### 10.2 OAuth refresh stub

```go
func TestAnthropic_OAuthRefresh_Success(t *testing.T) {
    oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req map[string]string
        json.NewDecoder(r.Body).Decode(&req)
        assert.Equal(t, "refresh_token", req["grant_type"])
        assert.Equal(t, "old-refresh-token", req["refresh_token"])

        json.NewEncoder(w).Encode(map[string]any{
            "access_token":  "new-access-token",
            "refresh_token": "new-refresh-token",
            "expires_in":    3600,
        })
    }))
    // ... adapter.Refresh 호출 검증
}
```

### 10.3 goleak 고루틴 누수 검증

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

Stream 고루틴이 ctx 취소 시 반드시 종료됨을 검증.

### 10.4 Race detector

모든 integration test는 `go test -race` 필수.

## 11. 성능 목표

| 메트릭 | 목표 |
|-----|------|
| `LLMCall` setup p99 (credential + cache plan + schema 변환) | < 10ms (네트워크 제외) |
| Stream chunk 변환 p99 | < 100μs per event |
| OAuth refresh RTT (stub) | < 50ms |
| Fallback retry 지연 | < 200ms (primary 실패 → fallback 시작) |

## 12. 오픈 이슈

- **Q1**: `anthropic-sdk-go`가 OAuth bearer를 native 지원하는가? 현재 확인 필요(본 SPEC은 수동 HTTP 호출 fallback 준비).
- **Q2**: Gemini `genai` SDK가 Adaptive Thinking 동등 기능을 지원하는가? Phase 1 미적용 가정.
- **Q3**: xAI Grok이 OpenAI-compat `/v1/chat/completions`를 정확히 지원하는가? 필요 시 per-provider parser 분리.
- **Q4**: DeepSeek의 tool 지원 범위? 현재 기본 지원 가정.
- **Q5**: Anthropic 1h TTL은 account/region별 제한? ADAPTER-001이 HTTP 400 시 5m fallback 처리.
- **Q6**: OpenAI O1 계열 reasoning 모델의 streaming 변경 여부? Phase 1은 GPT-4o 기본, o1은 후속.

## 13. 구현 순서 (TDD)

순서는 SPEC §6.9와 일치:

| # | Test | Provider |
|---|---|---|
| 1 | TestRegistry_LookupByName | — |
| 2 | TestNewLLMCall_WithStubProvider | — |
| 3 | TestAnthropic_Stream_HappyPath | Anthropic |
| 4 | TestAnthropic_ToolCall_RoundTrip | Anthropic |
| 5 | TestAnthropic_OAuthRefresh_Success | Anthropic |
| 6 | TestAnthropic_ThinkingMode | Anthropic |
| 7 | TestAnthropic_CacheMarkersApplied | Anthropic |
| 8 | TestOpenAI_Stream_HappyPath | OpenAI |
| 9 | TestXAI_UsesCustomBaseURL | xAI |
| 10 | TestGoogle_GeminiStream_HappyPath | Google |
| 11 | TestOllama_Stream_HappyPath | Ollama |
| 12 | TestAdapter_429Rotation | 전체 |
| 13 | TestAdapter_FallbackModelChain | 전체 |
| 14 | TestAdapter_ContextCancellation | 전체 |
| 15 | TestAdapter_CapabilityUnsupported | 전체 |

## 14. 재사용 vs 재작성 (hermes-llm.md §11 인용)

| 모듈 | Python | 판정 | Go LoC 목표 |
|---|---|---|---|
| Credential Pool | 1.3K | ✅ 재사용 | (CREDPOOL-001에서) |
| Rate Limit Tracker | 243 | ✅ 재사용 | (RATELIMIT-001에서) |
| Prompt Caching | 73 | ✅ 재사용 | (PROMPT-CACHE-001에서) |
| Anthropic Adapter | 1.45K | 🔶 부분 | **800** (본 SPEC) |
| Auxiliary Client (OpenAI-compat) | 2.25K | ❌ 재작성 | **1200** (본 SPEC) |
| Context Compressor | 33KB | ❌ 재작성 | (CONTEXT-001에서) |
| Prompt Builder | 42KB | ❌ 재작성 | (CONTEXT-001에서) |

본 SPEC의 예상 LoC: **2,000** (Anthropic 800 + OpenAI-compat 1200 + 각 래퍼 수백).

---

**End of Research (SPEC-GENIE-ADAPTER-001)**

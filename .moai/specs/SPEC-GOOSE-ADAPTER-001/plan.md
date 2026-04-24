# SPEC-GOOSE-ADAPTER-001 — Implementation Plan (Phase 1 Output)

Author: manager-strategy (Phase 1) · Approved by user 2026-04-24
Target worktree: `/Users/goos/.moai/worktrees/goose/SPEC-GOOSE-ADAPTER-001`
Branch: `feature/SPEC-GOOSE-ADAPTER-001`

## 1. Executive Summary

본 SPEC은 QUERY-001의 `LLMCallFunc` 수신자로서 6개 provider 어댑터(Anthropic / OpenAI / Google / xAI / DeepSeek / Ollama)를 `internal/llm/provider/` 하위에 구현한다. 접근은 **타입 스펙 선제 정의 + Anthropic 우선 TDD + OpenAI-compat 공유**. 최대 리스크는 의존 SPEC 4종(QUERY / RATELIMIT / PROMPT-CACHE / CREDPOOL 확장)이 아직 interface 수준으로도 존재하지 않아 본 SPEC이 공용 타입(`message.StreamEvent`, `tool.Definition`)을 선제 정의해야 한다는 점. 성공 기준: AC-ADAPTER-001~012 전수 GREEN, 85%+ 커버리지, `go test -race` 클린, httptest stub 기반 (실 API 無).

## 2. Dependency Status Matrix

| 의존 SPEC | 필요 Interface | 현재 상태 | 블로커 | 대안 |
|---|---|---|---|---|
| QUERY-001 | `query.LLMCallFunc`, `query.LLMCallReq` | 스펙에 이름만 언급, 타입 미정의. `internal/query/` **없음** | 부분 | 본 SPEC이 `internal/query/types.go`에 최소 구조체 선제 정의. QUERY-001 구현 시 흡수 |
| QUERY-001 | `message.Message`, `message.StreamEvent`, `message.ContentBlock` | 패키지 **없음** | 블로커 | `internal/message/` 선제 생성. 최소 필드. 추후 QUERY-001이 확장 |
| CREDPOOL-001 | `Select`, `MarkExhaustedAndRotate`, `AcquireLease`, `Refresher.Refresh`, `RuntimeSecret` | 부분 구현: `Select(ctx)` only, `MarkExhausted(cred, dur)` only, `AcquireLease`·`MarkExhaustedAndRotate` **부재** | 부분 | 본 SPEC에서 `MarkExhaustedAndRotate`, `AcquireLease` 선행 구현 (CREDPOOL-001 §3.1 rule 6/7 약속). Secret resolver는 `SecretStore` interface + `FileSecretStore` MVP |
| ROUTER-001 | `router.Route`, `router.ProviderRegistry` (메타) | 완전 구현 (registry.go 310 LOC). 6 provider 모두 `AdapterReady=true` | 없음 | 재사용. 메타 vs 인스턴스 역할 분리 |
| RATELIMIT-001 | `ratelimit.Tracker.Parse(provider, headers, now)` | 패키지 **없음** | 부분 | `internal/llm/ratelimit/tracker.go` 최소 stub (noop). RATELIMIT-001 실 구현 시 교체 |
| PROMPT-CACHE-001 | `cache.BreakpointPlanner`, `Plan`, `CachePlan`, `CacheStrategy`, `TTL` | 패키지 **없음** | 부분 | `internal/llm/cache/planner.go` stub (`CachePlan{Markers: nil}` 반환). REQ-ADAPTER-015 자동 만족 |
| TOOLS-001 | `tool.Definition` | 패키지 **없음** | 없음 | `internal/tool/definition.go` 선제 (Name/Description/Parameters) |
| CORE-001 | `zap.Logger` | 완전 구현 | 없음 | 재사용 |
| CONFIG-001 | `LLMConfig` | 최소 구현 | 없음 | 어댑터는 Options 구조체 직접 수용 |

**핵심 판단**: 본 SPEC의 책임 범위를 확장하여 `internal/message/`, `internal/tool/`, `internal/llm/ratelimit/`, `internal/llm/cache/`, `internal/query/types.go` 5개 타입 패키지를 lightweight skeleton으로 선제 정의. 각 <100 LOC. 타입 호환만 보장하고 로직은 stub. 후속 SPEC에서 로직이 채워질 때 adapter 코드는 무변경.

## 3. File Layout (확정)

```
internal/
├── message/                                [NEW skeleton]
│   └── types.go                            ~80 LOC  Message, StreamEvent, ContentBlock
├── tool/                                   [NEW skeleton]
│   └── definition.go                       ~30 LOC  Definition struct
├── query/
│   └── types.go                            ~40 LOC  [NEW skeleton] LLMCallReq, LLMCallFunc
├── llm/
│   ├── ratelimit/
│   │   └── tracker.go                      ~50 LOC  [NEW stub] Tracker.Parse noop
│   ├── cache/
│   │   └── planner.go                      ~40 LOC  [NEW stub] BreakpointPlanner returning empty plan
│   └── provider/
│       ├── provider.go                     ~90 LOC  Provider interface, CompletionRequest/Response, ThinkingConfig, UsageStats, Capabilities (capabilities.go 통합)
│       ├── registry.go                     ~70 LOC  ProviderRegistry (instance) + Register/Get/NewLLMCall helper
│       ├── errors.go                       ~30 LOC  ErrProviderNotFound, ErrCapabilityUnsupported
│       ├── llm_call.go                     ~80 LOC  NewLLMCall() → query.LLMCallFunc
│       ├── secret.go                       ~40 LOC  [NEW] SecretStore interface + FileSecretStore MVP
│       ├── fallback.go                     ~80 LOC  (M5) provider-agnostic model chain helper
│       │
│       ├── anthropic/
│       │   ├── adapter.go                  ~220 LOC Stream/Complete orchestration, retry-on-429 once
│       │   ├── oauth.go                    ~110 LOC PKCE refresh POST (Refresher 구현)
│       │   ├── token_sync.go               ~80 LOC  ~/.claude/.credentials.json atomic R/W
│       │   ├── models.go                   ~40 LOC  alias map + normalize + MaxOutputTokens
│       │   ├── tools.go                    ~90 LOC  OpenAI fn → Anthropic tool schema + tool_choice
│       │   ├── content.go                  ~130 LOC Message/ContentBlock 변환 (image base64, tool_result)
│       │   ├── thinking.go                 ~50 LOC  adaptive model set + buildThinkingParam
│       │   ├── stream.go                   ~200 LOC SSE parser → StreamEvent fan-out
│       │   └── cache_apply.go              ~60 LOC  Apply plan markers to request payload
│       │
│       ├── openai/
│       │   ├── adapter.go                  ~200 LOC generic OpenAI-compat client (base_url-swappable)
│       │   ├── stream.go                   ~140 LOC SSE → StreamEvent, tool_calls aggregation
│       │   └── tools.go                    ~50 LOC  passthrough tool schema + tool_call id propagation
│       │
│       ├── google/
│       │   └── gemini.go                   ~240 LOC genai SDK wrap + streaming + tool + capabilities (stream.go 병합)
│       │
│       ├── xai/
│       │   └── grok.go                     ~30 LOC  openai.New + BaseURL=https://api.x.ai/v1
│       │
│       ├── deepseek/
│       │   └── client.go                   ~30 LOC  openai.New + BaseURL=https://api.deepseek.com/v1 + Vision=false
│       │
│       └── ollama/
│           └── local.go                    ~180 LOC /api/chat JSON-L streaming (stream.go 병합)
```

**§6.1 대비 축소/통합 결정**:
- `capabilities.go` → `provider.go` 통합 (YAGNI)
- `google/stream.go` → `gemini.go` 통합 (SDK가 stream 내부 추상화)
- `ollama/stream.go` → `local.go` 통합 (endpoint 하나)
- `secret.go` 추가: CREDPOOL Zero-Knowledge + goose-proxy 부재 MVP gap 해소

**총 예상 LoC**: Production ~2,600 + Test ~2,400 ≈ **5,000 LOC**.

## 4. Implementation Order (Atomic Tasks)

단일 run 세션 내 5개 마일스톤으로 TDD RED-GREEN-REFACTOR 사이클. 각 task는 한 사이클 내 완료. 상세 테이블은 `tasks.md` 참조 (29개 항목).

### M0 Dependency Skeleton (T-001~T-007)
- T-001 `internal/message/types.go`: `Message{Role, Content []ContentBlock, ToolUseID}`, `ContentBlock{Type, Text, Image, ToolUseID, ToolResultJSON, Thinking}`, `StreamEvent{Type, Delta, BlockType, ToolUseID, StopReason, Error, Raw}`
- T-002 `internal/tool/definition.go`: `Definition{Name, Description, Parameters map[string]any}`
- T-003 `internal/query/types.go`: `LLMCallReq{Route, Messages, Tools, MaxOutputTokens, Temperature, Thinking, FallbackModels}`, `LLMCallFunc = func(ctx, LLMCallReq) (<-chan message.StreamEvent, error)`
- T-004 `internal/llm/ratelimit/tracker.go`: `Tracker{}.Parse(provider string, headers http.Header, now time.Time)` noop
- T-005 `internal/llm/cache/planner.go`: `BreakpointPlanner.Plan(msgs []message.Message, strategy CacheStrategy, ttl TTL) CachePlan`, always empty Markers
- T-006 `internal/llm/provider/secret.go`: `type SecretStore interface { Resolve(ctx, keyringID) (string, error); WriteBack(ctx, keyringID, secret) error }` + `FileSecretStore` MVP
- T-007 CREDPOOL 확장:
  - `internal/llm/credential/lease.go`: `AcquireLease(id string) *Lease`, `Lease.Release()`
  - `internal/llm/credential/pool.go`: `MarkExhaustedAndRotate(ctx, id string, statusCode int, retryAfter time.Duration) (*PooledCredential, error)` — atomic MarkExhausted + Select
  - unit tests: `pool_test.go` 확장, `lease_test.go` 확장 (기존 테스트 깨뜨리지 않고 확장)

### M1 Provider Core + Anthropic (T-010~T-021)
- T-010 `provider.go`:
  ```go
  type Capabilities struct { Streaming, Tools, Vision, Embed, AdaptiveThinking bool; MaxContextTokens, MaxOutputTokens int }
  type CompletionRequest struct { Route router.Route; Messages []message.Message; Tools []tool.Definition; MaxOutputTokens int; Temperature float64; Thinking *ThinkingConfig; Vision *VisionConfig; ResponseFormat string; FallbackModels []string; Metadata RequestMetadata }
  type ThinkingConfig struct { Enabled bool; Effort string; BudgetTokens int }
  type CompletionResponse struct { Message message.Message; StopReason string; Usage UsageStats; ResponseID string; RawHeaders http.Header }
  type UsageStats struct { InputTokens, OutputTokens, CacheReadTokens, CacheCreateTokens int }
  type Provider interface { Name() string; Capabilities() Capabilities; Complete(ctx, req CompletionRequest) (*CompletionResponse, error); Stream(ctx, req CompletionRequest) (<-chan message.StreamEvent, error) }
  ```
- T-011 `registry.go`: `ProviderRegistry` (instance), `Register(p Provider) error`, `Get(name) (Provider, bool)`, `Names() []string`
- T-012 `llm_call.go`: `NewLLMCall(registry, pool, tracker, cachePlanner, cacheStrategy, cacheTTL, logger) query.LLMCallFunc`; 구현: Route → registry.Get → Provider.Stream 전달
- T-013 `anthropic/models.go`: alias map e.g. `"claude-3.5-sonnet" → "claude-3-5-sonnet-20241022"`, `"claude-opus-4" → "claude-opus-4-7"`, `MaxOutputTokens` per model
- T-014 `anthropic/thinking.go`: `adaptiveModels = {"claude-opus-4-7", ...}`; `buildThinkingParam(cfg *ThinkingConfig, model string) *AnthropicThinkingParam`
- T-015 `anthropic/tools.go`: OpenAI function spec → Anthropic `Tool{Name, Description, InputSchema}`; tool_choice 매핑 (auto/any/tool{name} → Anthropic 각 필드)
- T-016 `anthropic/content.go`: message.ContentBlock → Anthropic native blocks. Image base64 변환, tool_result tool_use_id 보존
- T-017 `anthropic/stream.go`: SSE event 10종 → StreamEvent 매핑 (§6.5 테이블). text_delta / thinking_delta / input_json_delta 구분 핵심. content_block_start tool_use의 ToolUseID 추출
- T-018 `anthropic/cache_apply.go`: plan.Markers를 messages에 cache_control 필드로 apply. empty markers 시 무변경
- T-019 `anthropic/oauth.go`: `Refresh(ctx, cred) error` 구현. POST `https://console.anthropic.com/v1/oauth/token` with grant_type=refresh_token. Rotated refresh_token 대응. expires_at 갱신
- T-020 `anthropic/token_sync.go`: `ReadCredentialsFile()` + `AtomicWriteCredentialsFile()` (temp + rename + chmod 0600). 경로: `~/.claude/.credentials.json`
- T-021 `anthropic/adapter.go`: `New(opts AnthropicOptions) (*AnthropicAdapter, error)`; `Stream()` 구현: credential → plan → convert → HTTP → stream conversion goroutine. 429/402 시 MarkExhaustedAndRotate 후 1회 retry

## 5. Test Strategy

**Framework**: 표준 `testing` + `net/http/httptest` + `github.com/stretchr/testify` + `go.uber.org/goleak`.

**go.mod 추가**:
```
github.com/stretchr/testify v1.9.0
go.uber.org/goleak v1.3.0
```

**Per-test discipline**:
- 각 test 파일 상단에 `func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }`
- AC 기준 spec test 이름: `Test<Provider>_<Scenario>`
- `httptest.Server` fake response; 라이브 API 호출 금지
- `t.Parallel()` 허용, race detector 필수 (`go test -race`)
- Table-driven tests for conversion logic (tools.go, content.go, models.go, thinking.go)

**AC별 test 설계** (M0+M1 범위):
- AC-ADAPTER-001 TestAnthropic_Stream_HappyPath: SSE `message_start` → 2× `text_delta` → `message_stop`. Tracker.Parse 호출 수신 검증
- AC-ADAPTER-002 TestAnthropic_ToolCall_RoundTrip: `content_block_start{tool_use}` + 2× `input_json_delta` + `content_block_stop`. messages에 prior tool_result 포함
- AC-ADAPTER-003 TestAnthropic_OAuthRefresh_Success: 2대 httptest (token endpoint + API endpoint). expires_at=now+2분, refreshMargin=5분. tempdir `~/.claude/.credentials.json` 쓰기 검증
- AC-ADAPTER-008 TestAnthropic_429Rotation: 첫 호출 429 → MarkExhaustedAndRotate → 두 번째 cred 재시도 성공. pool에 a/b 2개
- AC-ADAPTER-010 TestAnthropic_ContextCancellation: httptest 10s sleep, ctx=WithTimeout(500ms). 채널 500±50ms close + 마지막 이벤트 error
- AC-ADAPTER-012 TestAnthropic_ThinkingMode_AdaptiveVsBudget: claude-opus-4-7 + Effort=high → body thinking{type,effort}, budget_tokens 부재. claude-3.7-sonnet + BudgetTokens=8000 → budget_tokens, effort 부재

**Shared test helpers** (`internal/llm/provider/testhelper/`):
- `FakePool(creds []string) *credential.CredentialPool`
- `NewSSEServer(events []string) *httptest.Server`
- `DrainStream(ctx, ch, max int) []message.StreamEvent`
- `CapturingHandler()` — records all requests

**Coverage target**: 85% (google 제외 평균). google/ 60-70% 허용 (SDK 내부 훅 한계).

## 6. Interface Design Decisions

1. `CompletionRequest.Route`: `router.Route` 직접 임베드. 재사용 유지.
2. `SecretStore` 도입: CREDPOOL은 KeyringID만 보유. `secretStore.Resolve(cred.KeyringID)`로 토큰 획득. MVP: `FileSecretStore`는 `~/.goose/credentials/{keyringID}.json` 평문 파일 읽기 (CREDPOOL이 로드한 같은 파일 구조 재사용).
3. `Refresher` 시그니처 유지: 현 `Refresh(ctx, cred) error` (in-place mutation). Anthropic adapter가 구현, `cred.ExpiresAt`, `cred.RefreshToken`, `cred.AccessToken` 업데이트 + `secretStore.WriteBack`.
4. ProviderRegistry 이중화: `router.ProviderRegistry` (메타, provider metadata including AdapterReady flag) + `provider.ProviderRegistry` (instance). NewLLMCall은 인스턴스 레지스트리만 참조.
5. `MarkExhaustedAndRotate` 추가는 CREDPOOL-001 §3.1 rule 6의 선행 구현 — 범위 확장 승인됨.
6. `StreamEvent` Type 10종: `stream_request_start`, `message_start`, `text_delta`, `thinking_delta`, `input_json_delta`, `content_block_start`, `content_block_stop`, `message_delta`, `message_stop`, `error`.

## 7. Risks & Mitigations (주요 항목)

- R1 anthropic-sdk-go OAuth Bearer 미지원 가능 → SDK는 HTTP client primitive로만, SSE parsing 직접
- R3 Streaming goroutine 누수 → `defer close(out)`, `select ctx.Done()` 패턴 통일. `goleak` 전 테스트 적용
- R4 tool_result/tool_use_id 순서 오매핑 → content.go table-driven test 10+
- R5 PROMPT-CACHE 미구현 nil 패닉 → stub이 항상 empty plan. `if len(plan.Markers) == 0: skip`
- R6 credentials.json write 중 crash → temp + rename atomic, 권한 0600
- R9 Adaptive vs budget 오분기 HTTP 400 → thinking.go unit test AC-012에서 강제
- R10 goose-proxy 부재 시 secret 평문 노출 → SecretStore interface, log redaction REQ-014 엄수
- R12 single-session token budget → 마일스톤 체크포인트, 중간 context compaction

## 8. Effort Estimate

| 영역 | Prod LOC | Test LOC | 총 LOC |
|------|----------|----------|--------|
| skeleton (message/tool/query/ratelimit/cache/secret) | 240 | 120 | 360 |
| provider core (provider, registry, errors, llm_call, fallback) | 310 | 240 | 550 |
| CREDPOOL 확장 (MarkExhaustedAndRotate, AcquireLease) | 80 | 150 | 230 |
| Anthropic (9 files) | 980 | 900 | 1,880 |
| OpenAI + tools + stream | 390 | 450 | 840 |
| xAI + DeepSeek | 60 | 120 | 180 |
| Google Gemini | 240 | 260 | 500 |
| Ollama | 180 | 160 | 340 |
| testhelper | 120 | — | 120 |
| **합계** | **2,600** | **2,400** | **~5,000** |

## 9. Proportionality / Simplicity / Reuse Checklist

- 6 provider 전체를 단일 SPEC: 적정 (xAI/DeepSeek가 OpenAI-compat 재사용)
- Skeleton 5 선제 정의: 적정 (각 <100 LOC, 타입 호환만)
- SecretStore: 적정 (gap 해소 유일 방법)
- CREDPOOL 확장: 경계선 → 승인됨 (CREDPOOL-001이 약속한 API)
- Anthropic 9-file 분해: 적정 (파일당 70-220 LOC, 단일 책임)
- google/stream.go, ollama/stream.go 병합: 적정
- fallback.go 별도 파일: 적정 (3개 provider 공유)
- testify + goleak 의존성: 적정 (수십 test에서 가독성)
- Tests 2,400 LOC: 적정 (85% coverage + race + goleak)

---

READY_FOR_APPROVAL: yes (user approved 2026-04-24)
BLOCKERS: none — CREDPOOL 확장은 본 SPEC 선행 구현으로 처리

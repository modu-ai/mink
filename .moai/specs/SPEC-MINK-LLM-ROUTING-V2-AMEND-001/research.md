# Research — SPEC-MINK-LLM-ROUTING-V2-AMEND-001

연구 목적: 5 curated provider 어댑터 + 2 인증 패턴 (key paste / OAuth PKCE) + fallback chain + MEMORY-QMD export hook 의 외부 자료 & 선행 SPEC 학습 정리. plan / tasks / acceptance 작성의 *근거 문서* 로 활용.

조사 범위: 5 provider API 명세, OAuth 2.1 PKCE + device-code RFC, 기존 SPEC-GOOSE-LLM-ROUTING-V2-001 의 패턴 / 한계, ChatGPT OAuth 8일 idle 관찰값, SSE streaming 호환성.

---

## 1. Provider API 명세

### 1.1 Anthropic Claude — Messages API

- Endpoint: `POST https://api.anthropic.com/v1/messages`
- 인증: `x-api-key: sk-ant-...` 헤더. key prefix `sk-ant-` (regex 검증 가능)
- 발급 페이지: `https://console.anthropic.com/settings/keys`
- 모델: `claude-opus-4-7`, `claude-sonnet-4-7`, `claude-haiku-4-7`
- Streaming: `"stream": true` → SSE `event: message_start | content_block_delta | message_stop`
- Rate limit header: `anthropic-ratelimit-tokens-limit`, `anthropic-ratelimit-tokens-remaining`, `anthropic-ratelimit-tokens-reset` (TPM 우선)
- Prompt caching: `cache_control: {"type": "ephemeral"}` per content block. Cache hit/miss 는 `usage.cache_read_input_tokens` 로 reporting

### 1.2 DeepSeek — OpenAI-compat

- Endpoint: `POST https://api.deepseek.com/v1/chat/completions` (OpenAI-compat)
- 인증: `Authorization: Bearer sk-...` 헤더. key prefix `sk-` (regex 검증 시 OpenAI 와 구분 어려움 → provider 명시 필요)
- 발급 페이지: `https://platform.deepseek.com/api_keys`
- 모델: `deepseek-reasoner` (≈ o1-mini 급 reasoning), `deepseek-chat` (general)
- 가격: $0.27 / M input (cached $0.07), $1.10 / M output (2026-05 기준)
- Rate limit header: OpenAI 와 동일 `x-ratelimit-*-requests`, `x-ratelimit-*-tokens` (RPM/TPM 모두)
- Streaming: OpenAI 와 동일 SSE `data: {...}\n\n` + terminator `data: [DONE]\n\n`

### 1.3 OpenAI GPT — Chat Completions API

- Endpoint: `POST https://api.openai.com/v1/chat/completions`
- 인증: `Authorization: Bearer sk-...` 또는 `sk-proj-...`
- 발급 페이지: `https://platform.openai.com/api-keys`
- 모델: `gpt-5.5`, `gpt-5.5-mini`, `gpt-5.5-nano` (2026-05 기준 product page)
- Function calling: `tools: [{type: "function", function: {...}}]`
- Vision: 멀티모달 input `{type: "image_url", image_url: {url}}`
- Streaming: SSE 동일

### 1.4 Codex (ChatGPT OAuth) — 비공식

- Endpoint: `POST https://api.openai.com/v1/responses` (Codex 전용 backend) — *비공식 정보*, 정책 변경 위험 (R3)
- 인증: OAuth 2.1 PKCE → access_token (Bearer)
- OAuth authorize URL: `https://chatgpt.com/codex/authorize?response_type=code&client_id=...&redirect_uri=http://127.0.0.1:PORT&scope=codex.chat+codex.streaming&code_challenge=...&code_challenge_method=S256`
- Token endpoint: `https://chatgpt.com/codex/token` — code + code_verifier → access_token + refresh_token
- **8일 idle 만료**: refresh_token 의 마지막 사용 시점 기준 8일 미사용 시 만료. ChatGPT Plus / Pro 정책 (외부 관측 reverse-engineering 기준, 정확 만료 = 응답 401 + body `"error": "invalid_grant"` 시 확정)
- Device-code flow fallback: `POST https://chatgpt.com/codex/device/code` → `device_code` + `user_code` + `verification_uri`. 사용자가 verification_uri 에 user_code 입력 → polling

### 1.5 z.ai GLM — OpenAI-compat

- Endpoint: `POST https://api.z.ai/api/paas/v4/chat/completions` (또는 OpenAI-compat 별칭)
- 인증: `Authorization: Bearer <key>` 헤더
- 발급 페이지: `https://bigmodel.cn/usercenter/apikeys` (한국에서 접근 시 우회 필요할 수 있음 — 사용자 책임, §3 A3)
- 모델: `glm-5-turbo`, `glm-5-coding`, `glm-5-air`, `glm-5-flash`
- 가격: `glm-5-coding` plan ~월 9 USD 정액 (별도 코딩 plan)
- Streaming: OpenAI-compat SSE

---

## 2. OAuth 2.1 PKCE — RFC 7636

### 2.1 Flow 요약

1. `code_verifier` = 43~128자 base64url random
2. `code_challenge` = base64url( SHA256(code_verifier) )
3. Authorize URL 에 `code_challenge=<>&code_challenge_method=S256` 포함
4. Callback 으로 `code` 수신
5. Token endpoint 에 `code` + `code_verifier` POST → access_token + refresh_token

### 2.2 Localhost callback 패턴

- `redirect_uri=http://127.0.0.1:0/callback` → OS 가 free port 할당
- Go: `net.Listen("tcp", "127.0.0.1:0")` → `addr.Port` 추출 → authorize URL 동적 생성
- callback handler: `http.HandleFunc("/callback", ...)` → `r.URL.Query().Get("code")` → channel 발행
- 보안: `state` 파라미터 CSRF 방지, expected ↔ received 검증

### 2.3 Device Authorization Grant — RFC 8628 (Fallback)

- 환경: headless / sandbox / port-bind 불가
- Flow:
  1. `POST /device/code` → `{device_code, user_code, verification_uri, expires_in, interval}`
  2. 사용자가 verification_uri 에 user_code 입력 (별도 단말에서)
  3. Client 는 `POST /token` polling (interval 초마다) → pending / approved / denied
  4. approved 시 access_token + refresh_token 수신

---

## 3. 기존 SPEC-GOOSE-LLM-ROUTING-V2-001 의 패턴 / 한계

### 3.1 차용 가능한 자산

- `RoutingPolicy` struct + YAML loader (`~/.goose/routing-policy.yaml`)
- `CapabilityMatrix` (15 provider × 4 capability)
- `RateLimitView` 인터페이스 (RATELIMIT-001 reader)
- `FallbackChain` 실행 + 14 FailoverReason 분기

### 3.2 본 amendment 에서 변경할 부분

| v0.2.1 (15-direct) | v0.2.0-amend (5 curated) |
|---|---|
| 15 provider 1차 시민 | 5 provider 1차 시민 + custom endpoint 무한 |
| 4 PolicyMode (`prefer_local`/`prefer_cheap`/`prefer_quality`/`always_specific`) | 3 카테고리 (`cost` / `quality` / `coding`) |
| 사용자가 fallback chain 수동 선언 | 카테고리별 chain 자동 (사용자 override 가능 via `--provider`) |
| credential 위임 SPEC 미명시 | AUTH-CREDENTIAL-001 위임 (A1) |
| MEMORY hook 없음 | MEMORY-QMD-001 export hook 인터페이스 (A2) |

### 3.3 코드 재활용 가능성

- `internal/llm/router/v2/policy.go` → 카테고리 enum 만 변경 (`PolicyMode` → `RoutingCategory`)
- `internal/llm/router/v2/capability.go` → 15 → 5 row 단축
- `internal/llm/router/v2/ratelimit_filter.go` → 그대로
- `internal/llm/router/v2/chain.go` → 카테고리 기반 자동 chain 으로 수정
- 15 adapter 코드는 *유지* (백워드 호환), 활성 default 만 5 로 전환

---

## 4. ChatGPT OAuth 8일 idle — 관찰 기반

### 4.1 외부 관찰값

- 커뮤니티 reverse-engineering 보고 (Hacker News / Reddit /r/ChatGPTPro 2026-Q1) → refresh_token 의 마지막 사용 시점 기준 7~8일 미사용 시 401
- 정확한 정책 = 미공개 (사용자 OpenAI ToS 변경 위험)
- 본 SPEC 의 R3 명시: 정책 변경 감지 시 user-facing warning

### 4.2 본 SPEC 의 대응

- REQ-RV2A-025 (State-Driven): 7일 경과 시 warning surface
- `~/.config/mink/codex-last-use` (또는 keyring meta) 에 timestamp 기록
- 7일 경과 + 첫 `mink ask` → stdout "7d since last Codex use — refresh recommended"
- 8일+ 실 만료 → `invalid_grant` 401 → user-facing "Codex login expired, run `mink login codex`"

---

## 5. SSE Streaming 호환성

### 5.1 OpenAI 표준

```
data: {"id":"...","object":"chat.completion.chunk","choices":[{"delta":{"content":"..."}}]}\n\n
data: [DONE]\n\n
```

### 5.2 Anthropic 변종

```
event: message_start
data: {"type":"message_start","message":{...}}\n\n

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}\n\n

event: message_stop
data: {"type":"message_stop"}\n\n
```

→ 본 SPEC 의 `internal/llm/provider/anthropic/stream.go` 가 변환 (Anthropic event → 통일된 `ChatChunk{Content: string}`)

### 5.3 Custom endpoint 호환성 위험

- vLLM: OpenAI-compat 완전 호환 (검증됨)
- Ollama OpenAI-compat (`/v1/chat/completions` 11434 port): SSE 호환, but `usage` 필드 누락 케이스 있음
- lm-studio: OpenAI-compat, but error response 가 OpenAI 와 미세 차이
- OpenRouter: OpenAI-compat + custom header `x-or-...`
- Together: OpenAI-compat 완전 호환
- → 본 SPEC 의 `internal/llm/provider/custom/` 에 5 golden fixture 단위 테스트 (R4)

---

## 6. AUTH-CREDENTIAL-001 인터페이스 가정

본 SPEC plan 종료 시점에 AUTH-CREDENTIAL-001 이 노출해야 하는 stable 인터페이스 (A1):

```go
package authcredential

type CredentialStore interface {
    Store(ctx context.Context, provider string, secret string, meta map[string]string) error
    Load(ctx context.Context, provider string) (secret string, meta map[string]string, err error)
    Delete(ctx context.Context, provider string) error
    List(ctx context.Context) ([]string, error)
}

type Backend int
const (
    BackendKeyring Backend = iota
    BackendPlaintext
)

type Config struct {
    PreferredBackend Backend
    FallbackAllowed  bool
}
```

본 SPEC 의 `internal/llm/auth/` 가 이 인터페이스의 *consumer* 로만 동작. 본 SPEC 어떤 코드도 keyring / file IO 직접 호출 안 함.

---

## 7. MEMORY-QMD-001 SessionExporter 인터페이스 가정

본 SPEC 이 정의 (host) + MEMORY-QMD-001 이 구현 (provide):

```go
package llmexport

type SessionMeta struct {
    Provider  string
    Model     string
    Timestamp time.Time
    Category  string  // "cost" / "quality" / "coding"
    Hash      string  // chunks hash for dedup
}

type SessionExporter interface {
    OnStreamChunk(chunk string) error          // 동기 fast-path, error 시 warning log only
    OnStreamComplete(meta SessionMeta) error   // 비동기 fan-out, error 는 stream 차단 안 함
}

// 기본 no-op
type NoopExporter struct{}
func (NoopExporter) OnStreamChunk(string) error          { return nil }
func (NoopExporter) OnStreamComplete(SessionMeta) error  { return nil }
```

MEMORY-QMD-001 가 plan/run 시점에 미구현이면 `NoopExporter` 사용. 후속 SPEC 머지 시 wire-up.

---

## 8. Threat Model — 5 Provider × 2 인증 패턴

| 위협 | 영향 | 본 SPEC 의 완화 |
|---|----|---|
| API key 평문 디스크 저장 → leak | High | REQ-RV2A-026, AUTH-CREDENTIAL-001 위임 |
| OAuth callback port hijack (다른 process listen) | Med | 127.0.0.1:0 auto-port + state CSRF |
| OAuth code interception (man-in-the-middle) | Low | localhost callback 만 허용, PKCE |
| refresh_token leak (디스크) | Med | AUTH-CREDENTIAL-001 keyring 위임 |
| Custom endpoint 가 악의적 server (key 탈취 시도) | Med | 사용자 책임, base_url 명시 입력 |
| MEMORY-QMD export 가 민감 응답 색인 → 누출 | Med | opt-in, 사용자가 `--no-export` 또는 config 로 비활성 가능 |

---

## 9. 외부 자료 (Sources)

> URL 은 WebFetch 미실행 — 본 plan 단계에서는 가정 기반. run 단계 진입 시 어댑터 구현 시점에 각 provider 의 latest API 문서 재확인 필수.

1. Anthropic Messages API — `https://docs.anthropic.com/en/api/messages`
2. OpenAI Chat Completions — `https://platform.openai.com/docs/api-reference/chat`
3. DeepSeek API — `https://api-docs.deepseek.com/`
4. z.ai BigModel API — `https://bigmodel.cn/dev/api`
5. OAuth 2.1 PKCE — RFC 7636
6. OAuth Device Authorization Grant — RFC 8628
7. SPEC-GOOSE-LLM-ROUTING-V2-001 v0.2.1 (`.moai/specs/SPEC-GOOSE-LLM-ROUTING-V2-001/spec.md`)
8. SPEC-GOOSE-ERROR-CLASS-001 (14 FailoverReason 정의)
9. SPEC-GOOSE-RATELIMIT-001 (4-bucket tracker)
10. ADR-001 (no self-host LLM, 외부 호출만)
11. ADR-002 (AGPL-3.0-only)

---

## 10. 결론 (Conclusion for Plan Authoring)

- 5 provider 어댑터는 단일 `Provider` 인터페이스로 추상화 → 구현은 어댑터 5종 + custom 1종 = 6 패키지
- 인증은 2 패턴 명확 분리: `internal/llm/auth/keypaste.go` (4 provider) + `internal/llm/auth/oauth_pkce.go` + `internal/llm/auth/device_code.go` (Codex)
- 라우팅은 3 카테고리 (`cost` / `quality` / `coding`) 단순화 — v0.2.1 의 4 PolicyMode 보다 사용자 인지 부담 낮음
- Fallback chain 자동 (카테고리 기반), 사용자 override 는 `--provider` flag 만
- AUTH-CREDENTIAL-001 / MEMORY-QMD-001 위임은 stable 인터페이스 freeze 합의가 plan 단계의 *외부 dependency 1*

---

Version: 1.0.0
Last Updated: 2026-05-16

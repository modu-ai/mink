---
id: SPEC-GOOSE-ADAPTER-001-AMEND-001
version: 0.1.0
status: completed
created_at: 2026-04-30
updated_at: 2026-05-04
author: manager-spec
priority: P2
issue_number: null
phase: 1
size: 소(S)
lifecycle: spec-anchored
labels: ["area/llm-provider", "type/feature", "amendment", "json-mode", "user-id"]
parent_spec: SPEC-GOOSE-ADAPTER-001
parent_version: 1.0.0
---

# SPEC-GOOSE-ADAPTER-001-AMEND-001 — JSON mode + UserID forwarding (deferred AC 처리)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-30 | 초안 작성 — 부모 SPEC v1.0.0의 deferred AC-016/017 처리, REQ-019/020 실구현 | manager-spec |

---

## 1. 개요 (Overview)

본 amendment SPEC은 부모 `SPEC-GOOSE-ADAPTER-001` v1.0.0(FROZEN, completed 2026-04-27)에서 **deferred 처리된 두 개 Optional 요구사항**을 처리한다.

- **REQ-ADAPTER-019 [Optional]**: JSON mode (`CompletionRequest.ResponseFormat == "json"` → provider별 structured output 강제)
- **REQ-ADAPTER-020 [Optional]**: UserID forwarding (`CompletionRequest.Metadata.UserID` → provider별 abuse-tracking 식별자 전달)

부모 SPEC v1.0.0은 위 두 필드를 `provider.go`에 선언만 두고 6개 어댑터 어디에서도 소비하지 않는 상태로 마감되었다(`spec.md` §5 AC-016/017 "DEFERRED" 표기). 본 amendment는 6 어댑터(anthropic / openai / xai / deepseek / google / ollama)에 두 기능을 capability-gated 방식으로 도입하면서 부모 `Provider` interface 시그니처를 보존한다.

**성공 조건**: 부모 SPEC의 24개 기존 단위 테스트가 0건 수정으로 통과하면서, 24 케이스(provider × ResponseFormat × UserID)의 신규 매트릭스 테스트가 전수 GREEN.

---

## 2. 배경 (Background)

### 2.1 부모 SPEC 상태

부모 SPEC `SPEC-GOOSE-ADAPTER-001`은 다음 상태로 동결되었다:

- v1.0.0 status: `completed` (2026-04-27)
- 마일스톤: M0~M5 전수 GREEN, AC-001~012 검증 완료, AC-013~015 직간접 검증 완료
- v1.0 신설 AC 중 **AC-016/017은 명시적으로 deferred**로 마감 (`spec.md` §5):
  - AC-016: "Status: DEFERRED to SPEC-GOOSE-ADAPTER-003"
  - AC-017: "Status: DEFERRED to SPEC-GOOSE-ADAPTER-003"

부모 `progress.md`(2026-04-30 메타 정합 회복 commit)는 "잔여 개선 작업: SPEC-GOOSE-ADAPTER-001-AMEND-001 (별도 신설 — deferred AC-016 JSON mode + AC-017 UserID forwarding, 2026-04-30 사용자 결정)"으로 본 amendment를 명시한다. 즉 본 SPEC은 부모 표기상의 `SPEC-GOOSE-ADAPTER-003` 자리를 amendment 형태로 직접 처리한다.

### 2.2 왜 amendment인가 (vs 신규 SPEC)

- 부모 surface 변경 없음 (`Provider` interface, `CompletionRequest` 시그니처 보존)
- 신규 추가는 (a) `Capabilities` 필드 2개, (b) 6 어댑터의 request body 직렬화 분기, (c) `llm_call.go` capability gate 1건. 단일 도메인, 단일 패키지, 부모 결정의 직접 후속.
- 새로운 도메인이나 패러다임 도입 없음 → 새 SPEC ID 채번보다 amendment 추적성이 우수.

### 2.3 부모 surface 보존 원칙 (HARD)

[HARD] 부모 `provider.Provider` interface 시그니처는 변경하지 않는다.
[HARD] `provider.CompletionRequest`의 기존 필드(`ResponseFormat`, `Metadata.UserID`)는 의미 변경 없이 그대로 사용한다.
[HARD] `provider.Capabilities`에 신규 필드 2개(`JSONMode bool`, `UserID bool`)를 추가하되 zero value(`false`)가 미지원을 의미하므로 backward compatible.
[HARD] 부모 SPEC의 24개 기존 단위 테스트는 본 amendment 구현 후 **0건 수정**으로 통과해야 한다.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **JSON mode 활성 매트릭스** (REQ-019 실구현):
   - OpenAI / xAI / DeepSeek: `response_format: {"type":"json_object"}` (request body top-level)
   - Google Gemini: `generationConfig.responseMimeType: "application/json"`
   - Ollama: `format: "json"` (request body top-level)
   - Anthropic: 미지원 — `Capabilities.JSONMode = false`, capability gate가 `ErrCapabilityUnsupported{feature:"json_mode"}` 반환
2. **UserID forwarding 매트릭스** (REQ-020 실구현):
   - OpenAI / xAI: top-level `"user"` 필드
   - Anthropic: `metadata.user_id` (nested) 필드
   - DeepSeek: 미지원(공식 문서 미문서) — silent drop + DEBUG 로그
   - Google Gemini / Ollama: 미지원(스키마에 필드 부재) — silent drop + DEBUG 로그
3. **`Capabilities` 확장**: `JSONMode bool` / `UserID bool` 두 boolean 필드 추가
4. **`NewLLMCall` capability gate 확장**:
   - `req.ResponseFormat == "json" && Capabilities.JSONMode == false` → `ErrCapabilityUnsupported{feature:"json_mode"}` 반환 (HTTP 호출 전 차단)
   - `req.Metadata.UserID != "" && Capabilities.UserID == false` → silent drop + `Logger.Debug("user_id_dropped")`
5. **테스트 매트릭스**: 6 provider × 2 (JSON on/off) × 2 (UserID present/empty) = 24 케이스 + 통합 테스트 2건
6. **부모 SPEC 정합**: 부모 `spec.md` §5 AC-016/017 본문에 본 amendment 참조 추가는 본 SPEC Run phase 마감 시 별도 commit으로 분리 처리(부모 FROZEN 정신 유지)

### 3.2 OUT OF SCOPE (Exclusions — 명시 제외)

- **Anthropic structured output via `output_config.format = json_schema`**: 별도 후속 SPEC 처리. 본 amendment는 Anthropic JSON mode를 명시적으로 unsupported로 분류하고 그 이상 진행하지 않는다.
- **System prompt 가이드를 통한 JSON 우회 처리**: prompt-engineering 영역으로 분류, 본 SPEC 범위 외.
- **DeepSeek `user` 필드 forwarding 시도**: 공식 문서 미문서 필드 → 일부 모델 400 응답 가능성으로 silent drop 채택. forwarding 시도 자체를 OUT.
- **UserID 형식 검증**(uuid/hex/길이 제한): 부모 SPEC "opaque identifier" 정신 유지, 호출자 책임.
- **JSON schema 강제 검증**(응답이 valid JSON인지 어댑터 단에서 파싱 확인): 응답 후처리 영역. 어댑터는 요청 forwarding만 책임진다.
- **`Provider` interface 시그니처 변경**: HARD 금지.
- **`CompletionRequest` 신규 필드 추가**: 기존 두 필드(`ResponseFormat`, `Metadata.UserID`)만 사용. 새 필드 도입 시 부모 SPEC 재개봉 필요.
- **GLM / Cerebras / Mistral / Groq / Kimi / OpenRouter / Together / Qwen / Fireworks 등 metadata-only provider**: 부모 SPEC OUT OF SCOPE 범위 그대로 유지(본 amendment에서도 OUT).

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-AMEND-001 [Ubiquitous]** — The `provider.Capabilities` struct **shall** expose two additional boolean fields, `JSONMode` and `UserID`, both defaulting to `false`; existing fields and their semantics **shall not** change.

**REQ-AMEND-002 [Ubiquitous]** — Every adapter **shall** declare its `Capabilities.JSONMode` and `Capabilities.UserID` values according to the provider matrix in §5; declarations **shall** be expressed in the adapter's `Capabilities()` method return value, not in external configuration.

### 4.2 Event-Driven

**REQ-AMEND-003 [Event-Driven]** — **When** `NewLLMCall(ctx, req)` is invoked and `req.ResponseFormat == "json"` and the resolved provider's `Capabilities.JSONMode == false`, the call **shall** return `ErrCapabilityUnsupported{Feature:"json_mode", Provider:<name>}` before issuing any HTTP request.

**REQ-AMEND-004 [Event-Driven]** — **When** `NewLLMCall(ctx, req)` is invoked and `req.Metadata.UserID != ""` and the resolved provider's `Capabilities.UserID == false`, the call **shall** drop the UserID value silently (no error), forward the request without the identifier, and emit a single `logger.Debug("user_id_dropped", provider=<name>)` log line.

**REQ-AMEND-005 [Event-Driven]** — **When** the OpenAI-compat adapter (covering OpenAI, xAI, OpenAI-compat-but-DeepSeek-with-JSONMode-only) issues a request with `req.ResponseFormat == "json"`, the request body **shall** include `"response_format": {"type": "json_object"}` as a top-level field; **when** `req.Metadata.UserID != ""` and `Capabilities.UserID == true`, the request body **shall** include `"user": "<value>"` as a top-level field.

**REQ-AMEND-006 [Event-Driven]** — **When** the Anthropic adapter issues a request with `req.Metadata.UserID != ""`, the request body **shall** include `"metadata": {"user_id": "<value>"}` as a nested field; the `metadata` object **shall not** be emitted when `UserID == ""`.

**REQ-AMEND-007 [Event-Driven]** — **When** the Google Gemini adapter issues a request with `req.ResponseFormat == "json"`, the underlying `genai.GenerateContentConfig` **shall** include `ResponseMIMEType: "application/json"`; the configuration **shall not** include the field when `ResponseFormat == ""`.

**REQ-AMEND-008 [Event-Driven]** — **When** the Ollama adapter issues a request with `req.ResponseFormat == "json"`, the request body **shall** include `"format": "json"` as a top-level field; the field **shall not** be emitted when `ResponseFormat == ""`.

### 4.3 State-Driven

**REQ-AMEND-009 [State-Driven]** — **While** `req.ResponseFormat` is the empty string and `req.Metadata.UserID` is the empty string, every adapter's serialized request body **shall** be byte-identical to the parent SPEC v1.0.0 baseline (regression invariant).

### 4.4 Unwanted

**REQ-AMEND-010 [Unwanted]** — **If** `req.Metadata.UserID` contains personally identifying data (this SPEC does not validate the format), the adapter **shall not** log the UserID value at INFO level or higher; UserID **may** appear in DEBUG-level logs only when explicitly redacted (first 4 chars + `...`).

**REQ-AMEND-011 [Unwanted]** — The capability gate in `NewLLMCall` **shall not** mutate `req`; it **shall** return errors or perform silent drops via a copy/wrapper rather than altering the caller-owned struct.

### 4.5 Optional

**REQ-AMEND-012 [Optional]** — **Where** the OpenAI-compat adapter is reused for xAI via BaseURL override, the adapter **shall** inherit `Capabilities.JSONMode = true` and `Capabilities.UserID = true` from the OpenAI default; DeepSeek **shall** override to `JSONMode = true` and `UserID = false` via its own `Capabilities()` method.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC 포맷 선언**: §4 REQ는 EARS 패턴, §5 AC는 Given/When/Then 시나리오로 작성된다. 각 AC는 실행 가능한 Go 테스트 1개 이상과 1:1 매핑되며, REQ↔AC 매핑은 본 절 말미 표에 기록한다. 부모 SPEC §5와 동일 형식.

**Provider 매트릭스 (capability declarations)**:

| Provider | JSONMode | UserID | JSON 페이로드 위치 | UserID 페이로드 위치 |
|----------|----------|--------|-------------------|---------------------|
| anthropic | false | true | n/a (capability gate가 차단) | request body `metadata.user_id` |
| openai | true | true | request body `response_format` | request body `user` |
| xai | true | true | request body `response_format` (OpenAI 어댑터 재사용) | request body `user` (OpenAI 어댑터 재사용) |
| deepseek | true | false | request body `response_format` (OpenAI 어댑터 재사용) | n/a (silent drop + DEBUG 로그) |
| google | true | false | `genai.GenerateContentConfig.ResponseMIMEType` | n/a (silent drop + DEBUG 로그) |
| ollama | true | false | request body `format: "json"` | n/a (silent drop + DEBUG 로그) |

---

**AC-AMEND-001 — Capabilities 신규 필드 expose** *(주 REQ: REQ-AMEND-001, REQ-AMEND-002)*
- **Given** 6개 어댑터가 각각 `New()`로 인스턴스화됨
- **When** 각 어댑터의 `Capabilities()`를 호출
- **Then** 반환된 `Capabilities` 구조체가 §5 매트릭스의 `JSONMode` / `UserID` 값을 정확히 반환. 매트릭스 외 필드(`Streaming`, `Tools`, `Vision`, `Embed`, `AdaptiveThinking`, `MaxContextTokens`, `MaxOutputTokens`)는 부모 SPEC v1.0.0 baseline과 동일

**AC-AMEND-002 — JSON mode unsupported fail-fast (Anthropic)** *(주 REQ: REQ-AMEND-003)*
- **Given** ProviderRegistry에 Anthropic 어댑터 등록, `req.ResponseFormat = "json"`, `req.Route = {provider:"anthropic", model:"claude-opus-4-7"}`
- **When** `NewLLMCall(ctx, req)` 호출
- **Then** (1) 반환 에러가 `ErrCapabilityUnsupported{Feature:"json_mode", Provider:"anthropic"}`, (2) httptest 서버에 도달한 HTTP 요청 수는 0(완전 차단), (3) credential pool의 `Select`는 호출되지 않거나 호출 후 lease 즉시 해제

**AC-AMEND-003 — JSON mode 활성 (OpenAI compat)** *(주 REQ: REQ-AMEND-005)*
- **Given** OpenAI 어댑터, `req.ResponseFormat = "json"`, 모델 `gpt-4o`
- **When** `NewLLMCall` 호출 후 httptest 서버가 받은 request body capture
- **Then** body JSON에 `"response_format": {"type": "json_object"}` 포함. 다른 표준 필드(`model`, `messages`, `stream`)는 부모 SPEC 형식 유지. xAI/DeepSeek 어댑터로 동일 케이스 반복(BaseURL override만 차이) 시 동일 결과

**AC-AMEND-004 — JSON mode 활성 (Gemini)** *(주 REQ: REQ-AMEND-007)*
- **Given** Google 어댑터, fake `ClientFactory`가 `GeminiRequest`를 capture하도록 주입, `req.ResponseFormat = "json"`
- **When** `NewLLMCall` 호출
- **Then** capture된 `GeminiRequest`의 `ResponseFormat` 필드 값이 `"json"`이고, 실제 SDK 호출 분기에서 `genai.GenerateContentConfig.ResponseMIMEType`이 `"application/json"`으로 설정됨

**AC-AMEND-005 — JSON mode 활성 (Ollama)** *(주 REQ: REQ-AMEND-008)*
- **Given** Ollama 어댑터, httptest 서버 `/api/chat`, `req.ResponseFormat = "json"`
- **When** `NewLLMCall` 호출 후 request body capture
- **Then** body JSON에 `"format": "json"` 포함. `req.ResponseFormat = ""`로 동일 호출 시 body에 `format` 필드 부재(omitempty 검증)

**AC-AMEND-006 — UserID forwarding (OpenAI top-level)** *(주 REQ: REQ-AMEND-005)*
- **Given** OpenAI 어댑터, `req.Metadata.UserID = "u-abc-123"`, 모델 `gpt-4o`
- **When** `NewLLMCall` 호출
- **Then** request body에 `"user": "u-abc-123"` 포함. `UserID = ""`인 경우 `user` 필드 부재(omitempty 검증). xAI 어댑터에서도 동일

**AC-AMEND-007 — UserID forwarding (Anthropic nested)** *(주 REQ: REQ-AMEND-006)*
- **Given** Anthropic 어댑터, `req.Metadata.UserID = "u-xyz-789"`, 모델 `claude-opus-4-7`
- **When** `NewLLMCall` 호출 후 request body capture
- **Then** body JSON에 `"metadata": {"user_id": "u-xyz-789"}` 포함. `UserID = ""`인 경우 `metadata` 객체 자체 부재(`omitempty`)

**AC-AMEND-008 — UserID silent drop (DeepSeek/Google/Ollama)** *(주 REQ: REQ-AMEND-004, REQ-AMEND-010)*
- **Given** DeepSeek 어댑터(또는 Google/Ollama) `Capabilities.UserID = false`, `req.Metadata.UserID = "u-test"`, zaptest observed-logs 활성
- **When** `NewLLMCall` 호출 후 request body capture + 로그 capture
- **Then** (1) request body에 `user` 필드 부재(silent drop), (2) HTTP 호출은 정상 진행되고 stream 정상 수신, (3) 에러 반환 없음, (4) DEBUG 레벨 로그에 `"user_id_dropped"` 메시지 + `provider` 필드 포함, INFO 이상 레벨에는 UserID 미기록. 동일 검증을 Google/Ollama 어댑터에 반복

**AC-AMEND-009 — Backward compatibility (zero-value 회귀)** *(주 REQ: REQ-AMEND-009)*
- **Given** 6개 어댑터 각각, `req.ResponseFormat = ""` 그리고 `req.Metadata.UserID = ""`
- **When** `NewLLMCall` 호출 후 request body capture
- **Then** capture된 body가 부모 SPEC v1.0.0 baseline의 직렬화 결과와 byte-identical(또는 정규화 후 동등). 신규 필드(`response_format`, `user`, `metadata`, `format`, `responseMimeType`)가 부재함을 명시 검증

**AC-AMEND-010 — 기존 부모 SPEC 회귀 zero modification** *(주 REQ: REQ-AMEND-009)*
- **Given** 부모 SPEC v1.0.0의 24개 단위 테스트 (anthropic/adapter_test.go, openai/adapter_test.go, ollama/local_test.go, google/gemini_test.go 등)
- **When** 본 amendment 구현 commit이 적용된 후 `go test ./...` 실행
- **Then** 모든 부모 테스트가 0건 수정 상태로 PASS. 본 amendment에서 추가 가능한 변경은 (a) `Capabilities()` 반환값 확장(zero-value 부분만 채움), (b) 신규 테스트 파일 추가뿐. 기존 테스트 파일 어셔션 변경 0건

**AC-AMEND-011 — Capability gate request 비변경** *(주 REQ: REQ-AMEND-011)*
- **Given** caller-owned `req` 인스턴스 (`req.ResponseFormat = "json"`, `req.Metadata.UserID = "u-1"`)
- **When** `NewLLMCall(ctx, req)` 호출 — provider가 `JSONMode = false`라서 에러 반환되거나, `UserID = false`라서 silent drop 경로 진입
- **Then** 호출 후 caller가 보유한 `req` 인스턴스의 모든 필드가 호출 전과 동일 (`reflect.DeepEqual` 검증). 어댑터 내부 복사본이 변형되더라도 caller 측 변형은 0건

---

### REQ → AC 매핑

| REQ | 매핑된 AC | 검증 방식 |
|-----|----------|----------|
| REQ-AMEND-001 | AC-AMEND-001 | 직접 (Capabilities() 반환값 검증) |
| REQ-AMEND-002 | AC-AMEND-001 | 직접 (매트릭스 검증) |
| REQ-AMEND-003 | AC-AMEND-002 | 직접 (httptest 호출 차단 검증) |
| REQ-AMEND-004 | AC-AMEND-008 | 직접 (silent drop + 로그 검증) |
| REQ-AMEND-005 | AC-AMEND-003, AC-AMEND-006 | 직접 (request body capture) |
| REQ-AMEND-006 | AC-AMEND-007 | 직접 (request body capture) |
| REQ-AMEND-007 | AC-AMEND-004 | 직접 (fake client capture) |
| REQ-AMEND-008 | AC-AMEND-005 | 직접 (request body capture) |
| REQ-AMEND-009 | AC-AMEND-009, AC-AMEND-010 | 직접 (회귀 검증) |
| REQ-AMEND-010 | AC-AMEND-008 | 직접 (zaptest log allowlist) |
| REQ-AMEND-011 | AC-AMEND-011 | 직접 (DeepEqual 검증) |
| REQ-AMEND-012 | AC-AMEND-001 (xAI/DeepSeek 행) | 직접 |

---

## 6. 기술적 접근 (Technical Approach, 요약)

### 6.1 변경 파일 (research.md §4.1 상세)

- `internal/llm/provider/provider.go`: `Capabilities` 필드 2개 추가
- `internal/llm/provider/llm_call.go`: capability gate 확장 (JSON mode fail-fast, UserID silent drop)
- `internal/llm/provider/anthropic/adapter.go`: `anthropicAPIRequest.Metadata` 필드 + `Capabilities()` 갱신
- `internal/llm/provider/openai/adapter.go`: `openAIRequest.{ResponseFormat,User}` 필드 + `Capabilities()` 갱신
- `internal/llm/provider/deepseek/client.go`: `Capabilities()` override (UserID:false)
- `internal/llm/provider/google/gemini.go` + `gemini_real.go`: `GeminiRequest.ResponseFormat` 필드 + ClientFactory 분기 + `Capabilities()` 갱신
- `internal/llm/provider/ollama/local.go`: `ollamaRequest.Format` 필드 + `Capabilities()` 갱신

### 6.2 페이로드 직렬화 결정 사항

- **OpenAI compat 어댑터**: `openAIRequest`에 `ResponseFormat *openAIResponseFormat \`json:"response_format,omitempty"\`` 추가 (포인터 + omitempty로 zero-value 시 직렬화 회피, research.md §4.3 R4 mitigation). `User string \`json:"user,omitempty"\`` 추가.
- **Anthropic 어댑터**: `anthropicAPIRequest`에 `Metadata *anthropicMetadata \`json:"metadata,omitempty"\`` 추가. `anthropicMetadata`는 `{UserID string \`json:"user_id,omitempty"\`}` 단일 필드.
- **Ollama 어댑터**: `ollamaRequest`에 `Format string \`json:"format,omitempty"\`` 추가.
- **Google 어댑터**: `GeminiRequest`에 `ResponseFormat string` 추가. `gemini_real.go`의 SDK 호출 시 `ResponseFormat == "json"`이면 `genai.GenerateContentConfig{ResponseMIMEType: "application/json"}` 주입.
- **xAI / DeepSeek**: OpenAI 어댑터 재사용 — 페이로드 코드 변경 없음. `Capabilities()` 메서드만 provider별 override.

### 6.3 Capability gate (llm_call.go)

```
NewLLMCall(...).LLMCallFunc 내부:
  p, ok = registry.Get(req.Route.Provider)
  if !ok: return ErrProviderNotFound

  caps = p.Capabilities()

  // 신규: JSON mode fail-fast
  if req.ResponseFormat == "json" && !caps.JSONMode {
    return nil, ErrCapabilityUnsupported{Feature: "json_mode", Provider: p.Name()}
  }

  // 신규: UserID silent drop (req 비변경, 내부 복사본만 변경)
  reqCopy := req
  if reqCopy.Metadata.UserID != "" && !caps.UserID {
    if logger != nil {
      logger.Debug("user_id_dropped",
        zap.String("provider", p.Name()),
        zap.String("user_id_redacted", redact(reqCopy.Metadata.UserID)),
      )
    }
    reqCopy.Metadata.UserID = ""
  }

  // 기존: vision capability gate (부모 SPEC T-061) 유지
  if reqCopy.Vision != nil && !caps.Vision {
    return nil, ErrCapabilityUnsupported{Feature: "vision", Provider: p.Name()}
  }

  return p.Stream(ctx, reqCopy)
```

`redact(s)`은 처음 4글자 + `...` 패턴(REQ-AMEND-010 참조).

### 6.4 테스트 전략 (research.md §6 참조)

- 각 어댑터 패키지에 `*_jsonmode_test.go` + `*_userid_test.go` 추가 (또는 통합 테이블 테스트)
- `internal/llm/provider/llm_call_test.go`에 capability gate 통합 테스트 추가:
  - `TestNewLLMCall_JSONModeUnsupportedFails` — Anthropic 차단
  - `TestNewLLMCall_UserIDSilentDrop` — DeepSeek/Google/Ollama silent drop
  - `TestNewLLMCall_RequestImmutability` — caller req 비변경 검증

---

## 7. Dependencies & Cross-references

- **부모 SPEC**: `SPEC-GOOSE-ADAPTER-001` v1.0.0 (FROZEN)
  - REQ-019 / REQ-020 본 amendment에서 직접 구현
  - AC-016 / AC-017 본 amendment에서 처리(부모 SPEC 본문은 FROZEN, 처리 결과는 본 SPEC 합격으로 마킹)
- **Provider 공식 문서** (research.md §5 verified 2026-04-30):
  - OpenAI Chat Completions, xAI API, DeepSeek API, Anthropic Messages API, Google Gemini API, Ollama API
- **부모 코드**: `internal/llm/provider/{provider.go,llm_call.go,llm_call_test.go,anthropic,openai,xai,deepseek,google,ollama}/...`
- **후속 SPEC 예상**: 없음(본 amendment로 deferred 항목 모두 해소). Anthropic structured output(`output_config.format=json_schema`)은 별도 SPEC 채번 시점 결정.

---

## 8. Exclusions (What NOT to Build)

부모 SPEC §3.2와 §3.1 OUT OF SCOPE 항목을 그대로 승계하면서, 본 amendment의 추가 명시 제외 항목:

1. **부모 `Provider` interface 시그니처 변경** — HARD 금지
2. **Anthropic `output_config.format = json_schema` 지원** — 후속 SPEC 처리
3. **Anthropic system prompt를 통한 JSON 우회 가이드** — prompt-engineering 스코프 외
4. **DeepSeek `user` field forwarding 시도** — 공식 문서 미문서, silent drop 채택
5. **UserID 형식 검증 (uuid/hex/길이)** — opaque identifier 정신 유지
6. **JSON 응답 valid JSON 후처리 검증** — 어댑터 책임 외
7. **부모 SPEC 24개 기존 단위 테스트 어서션 수정** — 회귀 기준선
8. **`CompletionRequest`에 신규 필드 추가** — 기존 `ResponseFormat` / `Metadata.UserID` 두 필드만 사용
9. **GLM / Cerebras / Mistral / Groq / Kimi / OpenRouter / Together / Qwen / Fireworks 등 metadata-only provider** — 부모 OUT 그대로

---

## 9. Quality Gates

- **TRUST-Tested**: 24 매트릭스 케이스 + 통합 테스트 3건, line coverage 본 amendment 변경분 90% 이상
- **TRUST-Readable**: 신규 테스트 파일은 테이블 드리븐, capability matrix는 단일 표로 표현
- **TRUST-Unified**: gofmt, go vet 0 warnings, golangci-lint 0 warnings
- **TRUST-Secured**: REQ-AMEND-010(UserID redaction in logs) 정적 검증 — `grep` 또는 zaptest observed-logs로 INFO+ 레벨 UserID 노출 0건 검증
- **TRUST-Trackable**: commit conventional + parent SPEC trailer (`SPEC: SPEC-GOOSE-ADAPTER-001-AMEND-001`, `Parent: SPEC-GOOSE-ADAPTER-001 v1.0.0`)

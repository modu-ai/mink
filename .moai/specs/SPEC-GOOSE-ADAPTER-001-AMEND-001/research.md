# Research — SPEC-GOOSE-ADAPTER-001-AMEND-001

- 작성일: 2026-04-30
- 작성자: manager-spec
- 부모 SPEC: SPEC-GOOSE-ADAPTER-001 v1.0.0 (FROZEN, completed 2026-04-27)
- 대상 deferred AC: AC-ADAPTER-016 (JSON mode, REQ-019), AC-ADAPTER-017 (UserID forwarding, REQ-020)
- Harness 후보: standard (단일 도메인, 6 어댑터 × 2 기능 매트릭스, 신규 surface 추가 없음)

---

## 1. 배경 (Why this amendment)

부모 SPEC `SPEC-GOOSE-ADAPTER-001` v1.0.0은 M0~M5 전 마일스톤이 GREEN 상태로 완료되었으나, REQ-019(JSON mode)와 REQ-020(UserID forwarding) 두 Optional 요구사항은 v1.0 합격 기준에서 의도적으로 deferred 처리되었다 (`tasks.md` "v1.0 신설 AC" 표 참조). 두 필드 자체는 `provider.go`의 `CompletionRequest`에 이미 선언되어 있으나(`ResponseFormat string`, `Metadata.UserID string`) 6개 어댑터 중 어느 것도 해당 필드를 실제 API 요청 페이로드에 forwarding하지 않는다. 본 amendment는 이 gap을 처리하는 후속 SPEC이다.

### 1.1 부모 SPEC 컨텍스트

- 부모 surface: `provider.Provider` interface (`Complete`, `Stream`, `Capabilities`, `Name`)
- 부모 status: `completed` (2026-04-27)
- 부모 v1.0.0 history 마지막 항목: "AC-013~017 신설 · status: planned→implemented"
- 부모 deferred AC 표기 (`spec.md` §5):
  - AC-ADAPTER-016 — "**Status**: **DEFERRED** to SPEC-GOOSE-ADAPTER-003"
  - AC-ADAPTER-017 — "**Status**: **DEFERRED** to SPEC-GOOSE-ADAPTER-003"
- 본 amendment는 위 표기상의 후속 SPEC을 `SPEC-GOOSE-ADAPTER-003` 대신 `SPEC-GOOSE-ADAPTER-001-AMEND-001`로 직접 처리한다(2026-04-30 사용자 결정, parent `progress.md` "잔여 개선 작업" 표기 일치).

### 1.2 Surface 보존 원칙

부모 `Provider` interface 시그니처는 변경하지 않는다. `CompletionRequest`에 신규 필드를 추가하지 않으며(이미 `ResponseFormat`, `Metadata.UserID` 존재), `Capabilities`에 두 개의 신규 boolean(`JSONMode`, `UserID`)을 추가한다. 이는 zero-value(false)가 backward compatible이므로 기존 6 어댑터 호출 측 코드 변경 없이 점진적 도입이 가능하다.

---

## 2. Provider 매트릭스 — JSON mode 지원

각 provider의 공식 문서를 확인한 결과(2026-04-30 기준) 다음과 같은 차이를 가진다.

| Provider | JSON mode 메커니즘 | 페이로드 필드 | 어댑터 위치 | 비고 |
|----------|--------------------|--------------|-------------|------|
| Anthropic | response_format 미지원 | (없음) | n/a | 공식 메시지 API에 `response_format` 필드 부재. 대안은 `output_config.format = json_schema` (schema-based)이지만 본 SPEC 스코프 외. JSON mode 요청 시 system prompt 가이드로 우회 또는 `ErrCapabilityUnsupported` 반환 |
| OpenAI | `response_format: {"type":"json_object"}` | request body top-level | `internal/llm/provider/openai/adapter.go` (openAIRequest struct 확장) | Chat Completions API 표준 |
| xAI Grok | OpenAI 호환 — `response_format` 지원 | (OpenAI 어댑터 재사용) | `internal/llm/provider/xai/grok.go` (별도 구현 불필요) | xAI 공식 문서: response_format 파라미터 listed |
| DeepSeek | OpenAI 호환 — `response_format: {"type":"json_object"}` 명시 지원 | (OpenAI 어댑터 재사용) | `internal/llm/provider/deepseek/client.go` (별도 구현 불필요) | DeepSeek 공식 문서에 명시 |
| Google Gemini | `generationConfig.responseMimeType: "application/json"` | nested in generationConfig | `internal/llm/provider/google/gemini.go` (genai SDK 경유) | OpenAI와 다른 위치 — generation_config 내부 |
| Ollama | `format: "json"` (또는 JSON schema) | request body top-level | `internal/llm/provider/ollama/local.go` (ollamaRequest struct 확장) | OpenAI와 다른 필드명 (`format` vs `response_format`) |

### 2.1 결정 매트릭스 (JSON mode)

- **OpenAI compat 통합 처리**: OpenAI/xAI/DeepSeek는 단일 `internal/llm/provider/openai/adapter.go`에서 `req.ResponseFormat == "json"`일 때 `openAIRequest`에 `ResponseFormat *openAIResponseFormat` 필드를 채워 직렬화한다. xAI/DeepSeek는 해당 어댑터를 BaseURL override로 재사용하므로 자동 상속.
- **Google 분기 처리**: `gemini.go`의 `GenerateStream` 호출 직전 `genai.GenerateContentConfig`에 `ResponseMIMEType: "application/json"`을 주입한다. 단, 본 SPEC은 `GeminiClientIface`의 인자 시그니처를 확장하지 않고, `GeminiRequest` 구조체에 `ResponseFormat string` 필드를 추가하여 `ClientFactory`가 적절히 해석하도록 위임한다.
- **Ollama 별도 처리**: `ollamaRequest`에 `Format string \`json:"format,omitempty"\`` 필드 추가. `req.ResponseFormat == "json"`이면 `"json"` 직접 매핑.
- **Anthropic 명시적 unsupported**: `Capabilities.JSONMode = false` 선언. `req.ResponseFormat == "json"`이고 provider가 Anthropic이면 `NewLLMCall`의 capability gate가 `ErrCapabilityUnsupported{feature:"json_mode", provider:"anthropic"}` 반환.

### 2.2 capability gate 위치

기존 vision capability gate(`llm_call.go` 내부 capability pre-check, T-061 참조)와 동일 패턴으로 처리한다. 즉 어댑터 내부가 아닌 `NewLLMCall` 진입 직후 단일 위치에서 `Capabilities.JSONMode == false && req.ResponseFormat == "json"` 조합을 검출하여 HTTP 호출 전에 차단한다. 이는 6 어댑터 각자에 unsupported 분기를 중복 작성하는 비용을 회피하고, 부모 AC-011(capability unsupported) 패턴과 일관된다.

---

## 3. Provider 매트릭스 — UserID forwarding

| Provider | UserID 메커니즘 | 페이로드 필드 | 어댑터 위치 | 비고 |
|----------|-----------------|--------------|-------------|------|
| Anthropic | `metadata.user_id` (nested) | request body `metadata.user_id` | `internal/llm/provider/anthropic/adapter.go` (`anthropicAPIRequest` 구조체 확장) | 공식 문서: "external identifier ... uuid, hash value, or other opaque identifier. Anthropic may use this id to help detect abuse" |
| OpenAI | `user` (top-level) | request body top-level `"user"` | `internal/llm/provider/openai/adapter.go` (`openAIRequest` 구조체 확장) | Chat Completions 표준 |
| xAI Grok | OpenAI 호환 — `user` field 명시 지원 | (OpenAI 어댑터 재사용) | n/a | xAI 공식 문서: "unique identifier ... help xAI to monitor and detect abuse" |
| DeepSeek | top-level `user` 미문서화 | (지원 불가 또는 silent skip) | `internal/llm/provider/deepseek/client.go` 또는 OpenAI 어댑터 분기 | 공식 문서에 명시되지 않음 — 미지원으로 간주, `Capabilities.UserID = false` |
| Google Gemini | 미지원 | (없음) | n/a | 공식 GenerateContentRequest 스키마에 user identifier 필드 부재 |
| Ollama | 미지원 (로컬 모델) | (없음) | n/a | 로컬 LLM 특성상 abuse tracking 불필요. 명시적 skip + DEBUG 로그 |

### 3.1 결정 매트릭스 (UserID)

- **OpenAI/xAI 직접 forwarding**: `openAIRequest`에 `User string \`json:"user,omitempty"\`` 필드 추가. `req.Metadata.UserID != ""`이면 그대로 매핑.
- **Anthropic nested forwarding**: `anthropicAPIRequest`에 `Metadata *anthropicMetadata \`json:"metadata,omitempty"\`` 필드 추가. `anthropicMetadata`는 `{UserID string \`json:"user_id,omitempty"\`}` 단일 필드 구조체.
- **DeepSeek**: 공식 문서 미문서 → `Capabilities.UserID = false`. capability gate가 단순 skip(에러 반환 X — 보안 식별자는 옵션 정보이므로 silent drop이 안전). 단 `Logger`에 "userID drop: provider=deepseek (not supported)" DEBUG 로그.
- **Google Gemini, Ollama**: 동일하게 `Capabilities.UserID = false` + silent drop + DEBUG 로그.

### 3.2 silent drop vs error 의사결정

- vision/JSON mode와 달리 UserID는 **남용 추적 부가 기능**이므로 미지원 시 호출 자체를 차단하면 가용성 저하가 크다. 따라서 UserID는 silent drop 정책을 채택한다.
- 대조적으로 JSON mode는 **출력 형식 계약**이므로 미지원 시 사용자가 unstructured 응답을 받게 되어 downstream 파싱 실패를 유발한다 → fail-fast 정책 채택(`ErrCapabilityUnsupported` 반환).
- 부모 SPEC AC-011(vision unsupported)은 **fail-fast** 패턴이며, 본 amendment는 이를 JSON mode에만 동일 적용하고 UserID는 별도 silent drop 패턴을 도입한다. AC 본문에 명시.

---

## 4. 기존 코드 영향 분석

### 4.1 변경 대상 파일

| 파일 | 변경 종류 | 변경 내용 요약 | LoC 추정 |
|------|-----------|--------------|---------|
| `internal/llm/provider/provider.go` | EXTEND | `Capabilities`에 `JSONMode bool`, `UserID bool` 필드 2개 추가 | +6 |
| `internal/llm/provider/llm_call.go` | EXTEND | capability gate에 JSON mode unsupported 검사 추가, UserID silent-drop 로그 | +20 |
| `internal/llm/provider/anthropic/adapter.go` | EXTEND | `anthropicAPIRequest.Metadata` 필드 추가 + `Capabilities()`에 `JSONMode:false, UserID:true` | +25 |
| `internal/llm/provider/openai/adapter.go` | EXTEND | `openAIRequest.{ResponseFormat,User}` 필드 추가 + `Capabilities()`에 `JSONMode:true, UserID:true` | +30 |
| `internal/llm/provider/xai/grok.go` | NO-CHANGE | OpenAI adapter 재사용 — capability만 자동 상속 | 0 (또는 capability override만) |
| `internal/llm/provider/deepseek/client.go` | EXTEND | `Capabilities` override: `JSONMode:true, UserID:false` (DeepSeek 미문서) | +5 |
| `internal/llm/provider/google/gemini.go` | EXTEND | `GeminiRequest.ResponseFormat` 필드 + ClientFactory 분기 + `Capabilities()`에 `JSONMode:true, UserID:false` | +20 |
| `internal/llm/provider/ollama/local.go` | EXTEND | `ollamaRequest.Format` 필드 + `Capabilities()`에 `JSONMode:true, UserID:false` | +15 |
| `internal/llm/provider/gemini_real.go` | EXTEND | 실제 SDK 어댑터에 `ResponseMIMEType` 주입 분기 | +10 |
| **신규 테스트 파일** | NEW | `*_jsonmode_test.go` × 6, `*_userid_test.go` × 6 (또는 통합 테이블 테스트) | +400 |

총 추정 production LoC: ~+130, 테스트 LoC: ~+400. 부모 SPEC 6,450 LoC 대비 약 8% 증가, **size: 소(S)** 분류 적합.

### 4.2 Backward compatibility 검증

- `CompletionRequest.ResponseFormat`이 빈 문자열(zero value)이면 모든 어댑터는 기존 동작 유지(JSON mode 비활성).
- `RequestMetadata.UserID`가 빈 문자열이면 모든 어댑터는 기존 동작 유지(헤더/필드 미주입).
- `Capabilities`의 신규 두 필드는 zero value(`false`)가 "미지원"을 의미하므로 capability gate를 활성화하지 않으면 호출은 그대로 통과한다. → **단계적 활성화 가능**.

### 4.3 Risk 요소

- **R1**: Anthropic의 system prompt 가이드를 통한 JSON 요청을 본 SPEC 스코프에 포함하면 prompt-engineering 영역으로 확장된다 → **명시적 OUT OF SCOPE**. Anthropic은 capability false로만 처리.
- **R2**: Google `genai` SDK의 `GenerateContentConfig.ResponseMIMEType` 필드는 SDK 버전에 따라 가용성이 다를 수 있음 → 본 amendment 진행 시 `go.mod`의 `google.golang.org/genai` 버전 확인 필요(현재 코드에 import 존재).
- **R3**: capability gate의 위치를 `llm_call.go`로 단일화하면 어댑터 단위 테스트에서는 직접 호출 시 gate를 우회한다 → 테스트 매트릭스에 `NewLLMCall` 경유 통합 테스트 1건 이상 포함 필수.
- **R4**: `openAIRequest.ResponseFormat`을 `*openAIResponseFormat` 포인터로 두지 않고 inline struct로 두면 `omitempty`가 zero-value struct에 대해 작동하지 않아 미요청 시에도 빈 객체가 직렬화되어 일부 호환 provider(예: Ollama 호환을 시도하는 사용자 정의 endpoint)가 거부할 가능성 → 포인터 + omitempty 필수.

---

## 5. 참고 문서 링크 (verified 2026-04-30)

- Anthropic Messages API: https://platform.claude.com/docs/en/api/messages
  - `metadata.user_id`: "An external identifier for the user who is associated with the request. This should be a uuid, hash value, or other opaque identifier."
  - `response_format`: Not available in Messages API. Structured output via `output_config.format = json_schema` (별도 SPEC 처리 권장).
- OpenAI Chat Completions: https://platform.openai.com/docs/api-reference/chat/create
  - `response_format: {"type": "json_object"}`: 표준 지원
  - `user`: "A unique identifier representing your end-user, which can help OpenAI to monitor and detect abuse."
- xAI Grok API: https://docs.x.ai/docs/api-reference
  - `response_format`: listed (OpenAI 호환)
  - `user`: 지원 ("help xAI to monitor and detect abuse")
- DeepSeek API: https://api-docs.deepseek.com/api/create-chat-completion
  - `response_format: {"type": "json_object"}`: 명시 지원
  - top-level `user`: **문서에 명시되지 않음** → 본 SPEC은 미지원으로 간주
- Google Gemini API: https://ai.google.dev/api/generate-content
  - `generationConfig.responseMimeType: "application/json"`: 지원 (responseSchema와 함께)
  - user identification: **GenerateContentRequest 스키마에 부재**
- Ollama API: https://github.com/ollama/ollama/blob/main/docs/api.md
  - `format: "json"` 또는 JSON schema: 지원
  - top-level user identifier: **부재** (로컬 LLM)

---

## 6. 테스트 전략 권고

### 6.1 테이블 매트릭스 (6 × 2 × 2 = 24 케이스)

| 차원 | 값 |
|------|-----|
| Provider | anthropic / openai / xai / deepseek / google / ollama |
| JSON mode | enabled (`ResponseFormat:"json"`) / disabled (`""`) |
| UserID | provided (`"u-test-123"`) / empty (`""`) |

각 케이스에 대해:
- request body assertion (httptest 또는 fake client capture)
- capability gate 통과/차단 검증
- silent drop 시 logger.Debug 호출 검증 (zaptest observed-logs)

### 6.2 통합 테스트

- `llm_call_test.go`에 `TestNewLLMCall_JSONModeUnsupportedFails` 추가 → Anthropic + ResponseFormat="json" 조합이 `ErrCapabilityUnsupported{feature:"json_mode"}` 반환 검증.
- `TestNewLLMCall_UserIDSilentDrop` → Ollama + UserID="u-1" 조합이 silent drop + 로그 기록 검증.

### 6.3 부모 SPEC 회귀 방지

- 기존 24개 어댑터 단위 테스트(adapter_test.go × 6 + 보조)는 zero-value `ResponseFormat` / `UserID`로 호출되므로 변경 없이 통과해야 한다. **부모 테스트 0건 수정**이 amendment 합격 조건.

---

## 7. Open Questions (Plan phase 결정 사항)

1. capability gate 위치 — `llm_call.go` 단일 처리 vs 각 어댑터 분산 처리
   → **권고**: `llm_call.go` 단일 처리 (R3 mitigation은 통합 테스트로). plan 단계 확정.
2. Anthropic `output_config.format=json_schema` 지원 포함 여부
   → **권고**: 명시적 OUT OF SCOPE. 후속 SPEC(`SPEC-GOOSE-ADAPTER-002` 또는 별도 schema SPEC)에서 처리.
3. DeepSeek `user` 필드 — silent drop vs OpenAI 호환 가정으로 forwarding
   → **권고**: silent drop. 공식 문서 미문서 필드는 forwarding 시 일부 deepseek-v4 모델에서 400 응답 가능성. 보수적 처리.
4. UserID 검증 — uuid/hex 형식 검사 추가 여부
   → **권고**: 본 SPEC 스코프 외. 부모 SPEC 정신("opaque identifier") 유지, 호출자 책임.

---

## 8. 종속성 / Cross-reference

- 부모 SPEC: `SPEC-GOOSE-ADAPTER-001` v1.0.0 (`spec.md` REQ-019, REQ-020 / AC-016, AC-017 deferred 표기)
- 부모 tasks.md: §"v1.0 신설 AC" 표 — DEFERRED 매핑 행
- 부모 progress.md: "잔여 개선 작업: SPEC-GOOSE-ADAPTER-001-AMEND-001 (별도 신설 — deferred AC-016 JSON mode + AC-017 UserID forwarding, 2026-04-30 사용자 결정)"
- 관련 후속 SPEC 후보: 없음(본 amendment로 deferred 항목 모두 해소 예정)

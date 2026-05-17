# Plan — SPEC-MINK-LLM-ROUTING-V2-AMEND-001

5 마일스톤 구성. 각 마일스톤은 priority (P0/P1/P2) 로만 정렬 — 시간 추정 금지.

---

## M1 — Provider 어댑터 5종 + 단일 인터페이스 (Priority P0)

### 목적

`Provider` 단일 인터페이스로 5 curated provider (Anthropic / DeepSeek / OpenAI / Codex / z.ai GLM) + custom OpenAI-compat 1 종을 추상화한다. Stream-first 응답, OpenAI-compat / Anthropic-native SSE 정상 변환.

### 산출물

- `internal/llm/provider/types.go` — `Provider`, `ChatRequest`, `ChatResponse`, `ChatStream`, `ChatChunk`, `ProviderCapabilities` 타입
- `internal/llm/provider/anthropic/` (Messages API + SSE event 변환)
- `internal/llm/provider/deepseek/` (OpenAI-compat)
- `internal/llm/provider/openai/` (Chat Completions)
- `internal/llm/provider/codex/client.go` (Codex 백엔드 호출; OAuth 흐름은 M2)
- `internal/llm/provider/zai/` (GLM, OpenAI-compat)
- `internal/llm/provider/custom/` (사용자 정의 OpenAI-compat endpoint, health-check)

### 매핑

- REQ: REQ-RV2A-001, -002, -006, -008, -010
- AC: AC-001, -002, -003, -004, -005, -006

### 의존 / 가정

- A1 (AUTH 인터페이스 stable) 는 M2 진입 전까지만 freeze 되면 됨. M1 의 어댑터는 키 인자를 함수 인자로 받으므로 AUTH 측 미구현 무관.
- 5 provider 의 API key 사전 발급은 사용자 책임 (A3).

### 위험

- R3 (Codex 비공식 API 변동): 어댑터를 `internal/llm/provider/codex/` 로 격리, 정책 변경 시 본 패키지만 수정.
- R4 (custom endpoint SSE 미세 차이): 5 golden fixture 단위 테스트로 검증.

---

## M2 — 인증 흐름 2 패턴 + AUTH-CREDENTIAL-001 위임 (Priority P0)

### 목적

4 provider (Anthropic / DeepSeek / OpenAI / GLM) = browser 새 탭 → key paste UI → AUTH 위임 저장. Codex = OAuth 2.1 PKCE + 127.0.0.1:auto-port + device-code 폴백.

### 산출물

- `internal/llm/auth/keypaste.go` — 4 provider key paste flow (browser open + stdin / POST `/install/provider/save` 양쪽 경로)
- `internal/llm/auth/oauth_pkce.go` — code_verifier / code_challenge / authorize URL 생성 + callback handler
- `internal/llm/auth/oauth_callback.go` — 127.0.0.1:0 listen + state CSRF 검증 + token endpoint 교환
- `internal/llm/auth/device_code.go` — Codex device-code flow (headless 환경 폴백)
- `internal/llm/auth/store_adapter.go` — AUTH-CREDENTIAL-001 `CredentialStore` consumer wrapper
- `internal/server/install/handlers.go` 갱신 — `/install/provider/save` 엔드포인트 (web onboarding 경로) — POST key + provider name → store_adapter 위임
- `internal/cli/commands/login.go` — `mink login <provider>` cobra 명령

### 매핑

- REQ: REQ-RV2A-004, -012, -013, -014, -015, -021, -025, -026
- AC: AC-007 ~ AC-016

### 의존 / 가정

- A1 (AUTH-CREDENTIAL-001 인터페이스 freeze) 필수. M2 진입 전 AUTH 측 SPEC plan 완료 확인.
- A4 (localhost port-bind 허용) → 실패 시 device-code flow 자동 폴백.
- A5 (8일 idle refresh-token 만료) → REQ-RV2A-025 의 7일 warning surface.

### 위험

- R1 (callback port 충돌) → auto-port + device-code 폴백 (REQ-RV2A-013)
- R6 (keyring 미가용 silent 평문 fallback) → 본 SPEC 의 login flow 가 fallback 사용 시 stdout warning surface (협조 책임)
- R8 (AUTH 인터페이스 drift) → plan 단계 freeze 합의

---

## M3 — 라우팅 정책 + Fallback Chain (Priority P0)

### 목적

3 카테고리 (cost / quality / coding) 활성 선택 + 카테고리별 자동 chain + 14 FailoverReason 분기 + RATELIMIT-001 80% 임계 회피.

### 산출물

- `internal/llm/router/v2/policy.go` — `RoutingCategory` enum + `~/.config/mink/routing.yaml` loader (atomic write)
- `internal/llm/router/v2/chain.go` — 카테고리별 priority list + 14 FailoverReason 매핑
- `internal/llm/router/v2/ratelimit_filter.go` — RATELIMIT-001 `RateLimitView` consumer (v0.2.1 재활용)
- `internal/llm/fallback/executor.go` — chain 순차 시도 + 실패 누적 + 모든 후보 실패 시 상세 에러 표면화
- `internal/cli/commands/routing.go` — `mink routing set {cost|quality|coding}` + `mink routing show`

### 매핑

- REQ: REQ-RV2A-003, -005, -016, -017, -019, -022, -023, -024, -027, -032
- AC: AC-017 ~ AC-026

### 의존 / 가정

- SPEC-GOOSE-ERROR-CLASS-001 v0.2.x (14 FailoverReason, completed)
- SPEC-GOOSE-RATELIMIT-001 (4-bucket tracker, completed)

### 위험

- R2 (provider rate-limit 정책 비대칭) → 4-bucket 중 *하나라도* 80% 도달 시 후보 제외 (v0.2.1 정책 재사용)
- Auto category 전환 금지 (REQ-RV2A-027) → 사용자 의도와 다른 모델 사용 방지

---

## M4 — MEMORY-QMD-001 Export Hook 인터페이스 (Priority P1)

### 목적

LLM 응답 stream → sanitized markdown chunk → MEMORY-QMD-001 `sessions/` collection 으로 *opt-in* 비동기 export. 본 SPEC 은 인터페이스 정의 only — 실 색인은 MEMORY-QMD-001 책임.

### 산출물

- `internal/llm/export/exporter.go` — `SessionExporter` 인터페이스 + `SessionMeta` struct
- `internal/llm/export/noop.go` — `NoopExporter` (default)
- `internal/llm/export/registry.go` — runtime 에 단일 active exporter 등록 (MEMORY-QMD-001 init 시점에 swap)
- `internal/llm/router/v2/stream_tap.go` — `Provider.Chat` 의 ChatStream 을 wrap 하여 chunk 별로 `OnStreamChunk` 비동기 발행 (errgroup 분리)
- `internal/cli/commands/ask.go` 갱신 — `--no-export` 옵션 (opt-out)

### 매핑

- REQ: REQ-RV2A-009, -020, -028
- AC: AC-027, -028, -029, -030

### 의존 / 가정

- A2 (MEMORY-QMD-001 의 sessions collection 가 정의된 payload 수용) — 본 SPEC plan 단계에서는 인터페이스만 정의, MEMORY-QMD-001 미구현이어도 `NoopExporter` 로 빌드 / 테스트 가능
- A2 가 위반되면 본 마일스톤은 인터페이스 freeze 만 하고 wire-up 후속 amendment 로 분리

### 위험

- R5 (hook 동기 차단) → `errgroup` + buffered channel 으로 응답 stream 차단 금지 (REQ-RV2A-020, -028)

---

## M5 — CLI/TUI 통합 + Custom Endpoint + E2E (Priority P1)

### 목적

`mink login` / `mink model {set|list|add-custom}` / `mink routing {set|show}` / `mink ask` 통합. Custom endpoint health-check + 5 사전 template (openrouter / vllm / lm-studio / ollama / together) provisioning. E2E 시나리오 4종.

### 산출물

- `internal/cli/commands/model.go` — `set` / `list` / `add-custom`
- `internal/llm/provider/custom/template.go` — 5 사전 template 정의
- `internal/cli/commands/ask.go` — routing 결정 → Provider 호출 → stdout stream 출력 + `--provider` override + `--no-stream` + `--no-export`
- `internal/server/install/handlers.go` 확장 — web onboarding 경로의 provider 선택 UI 와 통합 (ONBOARDING-001 v0.3.1 wiring 활용)
- E2E 테스트 시나리오:
  1. 새 사용자 → `mink login anthropic` → key paste → `mink ask "hello"` → claude-opus-4-7 응답 stream
  2. 사용자 → `mink login codex` → OAuth → `mink ask` → Codex 응답
  3. 사용자 → `mink routing set cost` → `mink ask` → DeepSeek 1순위 응답
  4. 사용자 → `mink ask` (모든 provider quota 초과 시뮬) → 14 FailoverReason + 시도 목록 표면화

### 매핑

- REQ: REQ-RV2A-007, -011, -018, -029, -030, -031
- AC: AC-031 ~ AC-038

### 의존 / 가정

- M1~M4 모두 GREEN
- ONBOARDING-001 v0.3.1 의 `/install/provider/save` 엔드포인트 wiring 활용
- AGPL-3.0-only 라이선스 헤더 모든 신규 파일 포함

### 위험

- E2E 시나리오 2 (Codex OAuth) 는 ChatGPT 실 계정 필요 → CI 에서는 mock token endpoint 사용
- 5 template 의 base_url 이 사용자 환경별 가변 → 사용자가 명시 입력 옵션 항상 제공

---

## 마일스톤 의존 그래프

```
M1 (Provider 어댑터) ────────┐
                             ▼
M2 (Auth) ◀── A1 freeze ────►  M3 (Routing + Fallback)
                             ▼
                          M4 (Export hook)
                             ▼
                          M5 (CLI/TUI + E2E)
```

M1 / M2 / M3 / M4 는 인터페이스 freeze 후 *부분 병렬 가능* (어댑터 5종 병렬, auth 2 패턴 병렬). M5 는 전체 wire-up.

---

## 기술 접근

### Go 패키지 분해 원칙

- `internal/llm/provider/<name>/` — 1 provider = 1 패키지, `client.go` + `stream.go` + `client_test.go`
- `internal/llm/auth/` — 인증 패턴별 파일 분리 (keypaste / oauth_pkce / oauth_callback / device_code / store_adapter)
- `internal/llm/router/v2/` — 기존 v0.2.1 코드 재활용 (policy / capability / ratelimit_filter / chain)
- `internal/llm/fallback/` — chain executor 단일 패키지
- `internal/llm/export/` — 인터페이스 only + noop default

### 단위 테스트 전략

- 각 provider 어댑터: httptest mock server + golden SSE fixture
- OAuth flow: state CSRF / code_verifier roundtrip / device-code polling timeout
- Routing chain: 14 FailoverReason 별 1 케이스 + 모든 후보 실패 케이스
- Export hook: NoopExporter 기본 + mock exporter 비동기 발행 검증 + 발행 실패 시 stream 차단 안 됨 검증

### 통합 테스트 전략

- 5 provider 각 1 실 호출 (CI secret 사용, 일일 1회만 실행)
- Codex OAuth: mock token endpoint 로 PKCE roundtrip 검증
- Web onboarding `/install/provider/save` ↔ login.go ↔ store_adapter chain 통합

---

## TRUST 5 정합 (Plan 단계)

| 항목 | Plan 단계 약속 |
|---|---|
| Tested | M1~M5 각 마일스톤별 unit + integration + e2e 테스트 단계 계획 (38 AC 1:1 mapping) |
| Readable | 패키지 분해 원칙 명시, 모듈별 단일 책임 |
| Unified | 단일 `Provider` 인터페이스 + 단일 fallback chain + 단일 credential 위임 |
| Secured | M2 의 평문 저장 금지 (REQ-RV2A-026), OAuth PKCE, regex 검증 |
| Trackable | 5 마일스톤 × 38 AC 매트릭스 (tasks.md), progress.md milestone 추적 |

---

Version: 1.0.0
Last Updated: 2026-05-16

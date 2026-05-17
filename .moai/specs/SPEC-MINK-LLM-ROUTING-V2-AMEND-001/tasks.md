# Tasks — SPEC-MINK-LLM-ROUTING-V2-AMEND-001

총 26 atomic task. 마일스톤별로 그룹화. 각 task 는 `T-XXX` 식별자 + priority + 산출 패키지 / 파일 + 검증 방법 + 매핑 REQ/AC.

---

## M1 — Provider 어댑터 (8 tasks)

### T-001 (P0) — Provider 타입 정의

- 산출: `internal/llm/provider/types.go`
- 내용: `Provider` interface, `ChatRequest`, `ChatResponse`, `ChatChunk`, `ChatStream`, `ProviderCapabilities` struct
- 검증: `go vet ./internal/llm/provider/...` PASS, godoc lint 0 violation
- REQ: REQ-RV2A-001, -002 / AC: AC-001

### T-002 (P0) — Anthropic 어댑터

- 산출: `internal/llm/provider/anthropic/client.go`, `stream.go`, `client_test.go`
- 내용: Messages API + Anthropic SSE event (`message_start` / `content_block_delta` / `message_stop`) 를 통일 `ChatChunk` 로 변환
- 검증: httptest mock server + golden SSE fixture 3종 (success / partial chunks / error 401)
- REQ: REQ-RV2A-001, -002 / AC: AC-002

### T-003 (P0) — DeepSeek 어댑터

- 산출: `internal/llm/provider/deepseek/client.go`, `client_test.go`
- 내용: OpenAI-compat `POST /v1/chat/completions` + SSE `data: [DONE]` terminator
- 검증: golden fixture 2종 (success / rate-limit 429)
- REQ: REQ-RV2A-001, -002, -008 / AC: AC-003

### T-004 (P0) — OpenAI 어댑터

- 산출: `internal/llm/provider/openai/client.go`, `client_test.go`
- 내용: Chat Completions + function calling 옵션 통과 (요청 통과만, function 처리는 본 SPEC 범위 외)
- 검증: golden fixture 3종 (success / function_call request / context_window_exceeded 400)
- REQ: REQ-RV2A-001, -002, -008 / AC: AC-004

### T-005 (P0) — Codex 어댑터 (client only)

- 산출: `internal/llm/provider/codex/client.go`, `client_test.go`
- 내용: Codex backend endpoint 호출 (OAuth 흐름은 T-011). 토큰은 `CredentialStore.Load` 로 인자 수신
- 검증: mock token + golden fixture 2종
- REQ: REQ-RV2A-001, -002 / AC: AC-005

### T-006 (P0) — z.ai GLM 어댑터

- 산출: `internal/llm/provider/zai/client.go`, `client_test.go`
- 내용: OpenAI-compat 호환, glm-5-turbo / glm-5-coding 모델 지원
- 검증: golden fixture 2종
- REQ: REQ-RV2A-001, -002 / AC: AC-005a

### T-007 (P1) — Custom OpenAI-compat 어댑터

- 산출: `internal/llm/provider/custom/client.go`, `health.go`, `client_test.go`
- 내용: 임의 base_url + `/v1/chat/completions` 호출. health-check 는 `GET /v1/models` 또는 `OPTIONS /v1/chat/completions` 시도
- 검증: 5 golden fixture (openrouter / vllm / ollama-openai-compat / lm-studio / together)
- REQ: REQ-RV2A-008, -018 / AC: AC-006

### T-008 (P1) — ProviderRegistry 일원화

- 산출: `internal/llm/provider/registry.go`, `registry_test.go`
- 내용: 5 curated provider 등록 + custom 동적 추가 / 제거 API
- 검증: 단위 테스트 — 5 curated 기본 등록, custom add 후 lookup, 중복 등록 거부
- REQ: REQ-RV2A-001, -018 / AC: AC-001a

---

## M2 — 인증 흐름 (8 tasks)

### T-009 (P0) — AUTH consumer adapter

- 산출: `internal/llm/auth/store_adapter.go`, `store_adapter_test.go`
- 내용: AUTH-CREDENTIAL-001 `CredentialStore` 인터페이스 consumer wrapper. provider 명 정규화 (anthropic / deepseek / openai / codex / glm / custom:<name>)
- 검증: mock CredentialStore + Store/Load/Delete roundtrip + 정규화 케이스 5종
- REQ: REQ-RV2A-004, -026 / AC: AC-007

### T-010 (P0) — Key paste flow (4 provider)

- 산출: `internal/llm/auth/keypaste.go`, `keypaste_test.go`
- 내용: 4 provider 각각의 발급 페이지 URL 매핑 + browser open + stdin 입력 + regex 검증 + store_adapter 위임
- 검증: regex 검증 케이스 (anthropic `^sk-ant-`, deepseek `^sk-`, openai `^sk-(proj-)?`, glm 임의 32자+)
- REQ: REQ-RV2A-012, -015, -026 / AC: AC-008, AC-009

### T-011 (P0) — OAuth PKCE 생성

- 산출: `internal/llm/auth/oauth_pkce.go`, `pkce_test.go`
- 내용: code_verifier (43~128 byte) + code_challenge (SHA256 base64url) + state CSRF token 생성. authorize URL 빌드
- 검증: roundtrip 단위 테스트 + state 무작위성
- REQ: REQ-RV2A-013 / AC: AC-010

### T-012 (P0) — OAuth callback server

- 산출: `internal/llm/auth/oauth_callback.go`, `callback_test.go`
- 내용: 127.0.0.1:0 listen → free port allocation → callback handler → state 검증 → code 추출 → channel 발행 → token endpoint 교환
- 검증: httptest mock token endpoint + 정상 case + state mismatch reject + port conflict 시 device-code 폴백
- REQ: REQ-RV2A-013, -014 / AC: AC-011, AC-012

### T-013 (P0) — Device-code flow 폴백

- 산출: `internal/llm/auth/device_code.go`, `device_code_test.go`
- 내용: RFC 8628 device-code flow — `POST /device/code` → user_code stdout 표시 → polling (interval) → token
- 검증: mock device endpoint + polling timeout + approved/denied 분기
- REQ: REQ-RV2A-013 / AC: AC-013

### T-014 (P0) — `mink login` 명령

- 산출: `internal/cli/commands/login.go`, `login_test.go`
- 내용: cobra subcommand `mink login <provider>` — provider 검증 → keypaste.go 또는 oauth_pkce.go 분기 → 성공/실패 stdout 안내
- 검증: 5 provider 분기 + 미지원 provider 거부 + Codex OAuth 단계 stdout 안내 (REQ-RV2A-021)
- REQ: REQ-RV2A-012, -013, -021 / AC: AC-014, AC-015

### T-015 (P1) — Web onboarding `/install/provider/save` 통합

- 산출: `internal/server/install/handlers.go` 갱신 + `handler_test.go` 갱신
- 내용: 기존 ONBOARDING-001 v0.3.1 의 `/install/provider/save` 엔드포인트가 본 SPEC 의 `keypaste.go` 로 라우팅. CSRF double-submit + Origin allowlist 재사용
- 검증: 기존 ONBOARDING-001 handler_test.go 의 server test 패턴 재사용 (Session per-session sync.Mutex 보존)
- REQ: REQ-RV2A-015 / AC: AC-016

### T-016 (P1) — Codex refresh-token 7일 warning

- 산출: `internal/llm/provider/codex/idle_warning.go`, `idle_warning_test.go`
- 내용: `~/.config/mink/codex-last-use` (또는 AUTH `meta` 필드) 에 timestamp 기록 → 7일 경과 시 첫 ask 호출에 stdout warning surface
- 검증: 시계 mock (`clock.Clock` 의존성 주입) + 7일 경계 / 8일 만료 분기
- REQ: REQ-RV2A-025 / AC: AC-016a

---

## M3 — 라우팅 + Fallback Chain (5 tasks)

### T-017 (P0) — RoutingCategory + YAML loader

- 산출: `internal/llm/router/v2/policy.go`, `policy_test.go`
- 내용: `RoutingCategory` enum (`Cost` / `Quality` / `Coding`) + `~/.config/mink/routing.yaml` atomic write/read
- 검증: 단위 테스트 — default `Quality`, atomic write race-test, 알 수 없는 카테고리 reject
- REQ: REQ-RV2A-003, -019 / AC: AC-017, AC-018

### T-018 (P0) — Category-based chain 정의

- 산출: `internal/llm/router/v2/chain.go`, `chain_test.go`
- 내용: 3 카테고리별 priority list (cost / quality / coding) 정적 정의 — REQ-RV2A-022, -023, -024 의 순서 그대로
- 검증: 단위 테스트 — 카테고리별 head 후보 일치, 길이 일치 (5 provider 모두 포함)
- REQ: REQ-RV2A-022, -023, -024 / AC: AC-019, AC-020, AC-021

### T-019 (P0) — Fallback chain executor

- 산출: `internal/llm/fallback/executor.go`, `executor_test.go`
- 내용: chain 순차 호출 + 14 FailoverReason 분기 + 모든 후보 실패 시 종합 에러 (시도 목록 + 마지막 에러)
- 검증: 14 FailoverReason 각 1 케이스 + 모든 후보 실패 케이스 + 첫 후보 성공 케이스
- REQ: REQ-RV2A-005, -016, -017, -027 / AC: AC-022, AC-023, AC-024

### T-020 (P0) — Rate-limit filter 재활용

- 산출: `internal/llm/router/v2/ratelimit_filter.go` (v0.2.1 코드 재활용 + 5 provider 만 처리)
- 내용: RATELIMIT-001 `RateLimitView` consumer, 80% 임계 회피
- 검증: 기존 v0.2.1 ratelimit_filter_test.go 의 케이스 그대로 + 5 provider 한정
- REQ: REQ-RV2A-005 / AC: AC-025

### T-021 (P1) — `mink routing` 명령

- 산출: `internal/cli/commands/routing.go`, `routing_test.go`
- 내용: `mink routing set {cost|quality|coding}` + `mink routing show`
- 검증: 단위 테스트 + golden output
- REQ: REQ-RV2A-019 / AC: AC-026

---

## M4 — MEMORY-QMD Export Hook (3 tasks)

### T-022 (P1) — SessionExporter 인터페이스

- 산출: `internal/llm/export/exporter.go`, `noop.go`, `registry.go`, `exporter_test.go`
- 내용: 인터페이스 정의 + `NoopExporter` 기본 + runtime 단일 active exporter 등록 API (MEMORY-QMD-001 측이 init 시점에 swap)
- 검증: 단위 테스트 — default Noop, swap, double-register 거부
- REQ: REQ-RV2A-009 / AC: AC-027

### T-023 (P1) — Stream tap 비동기 발행

- 산출: `internal/llm/router/v2/stream_tap.go`, `stream_tap_test.go`
- 내용: `Provider.Chat` 의 ChatStream 을 wrapping → chunk 별로 `OnStreamChunk` 비동기 발행 (errgroup + buffered channel). 발행 실패 시 warning log only, stream 차단 안 함
- 검증: mock exporter 호출 횟수 검증 + 발행 실패 시 응답 stream 정상 완료 + race-test
- REQ: REQ-RV2A-020, -028 / AC: AC-028, AC-029

### T-024 (P2) — `--no-export` opt-out 플래그

- 산출: `internal/cli/commands/ask.go` 갱신
- 내용: `--no-export` 플래그 시 stream_tap 등록 skip
- 검증: 단위 테스트 — 플래그 ON 시 exporter 호출 0회
- REQ: REQ-RV2A-009 / AC: AC-030

---

## M5 — CLI/TUI 통합 + E2E (2 tasks)

### T-025 (P1) — `mink model` + custom endpoint template

- 산출: `internal/cli/commands/model.go`, `model_test.go`, `internal/llm/provider/custom/template.go`
- 내용: `mink model {set|list|add-custom}` + 5 사전 template (openrouter / vllm / lm-studio / ollama / together) + health-check + `--no-stream` / `--provider` override 옵션
- 검증: 5 template 단위 + custom add health-check 성공/실패 + `--provider` override
- REQ: REQ-RV2A-018, -030, -031, -032 / AC: AC-031, AC-032, AC-033, AC-034

### T-026 (P1) — E2E 시나리오 4종

- 산출: `e2e/llm-routing-v2-amend/{anthropic,codex,cost-routing,fallback-all-fail}_test.go`
- 내용:
  1. Anthropic 키 paste → ask → claude-opus-4-7 응답
  2. Codex OAuth (mock token endpoint) → ask → Codex 응답
  3. routing set cost → ask → DeepSeek 1순위 응답
  4. 모든 provider quota 초과 시뮬 → 14 FailoverReason + 시도 목록 surface
- 검증: 각 시나리오 PASS, brand-lint 0 violation, AGPL-3.0 헤더 모든 신규 파일
- REQ: REQ-RV2A-007, -011, -017, -029 / AC: AC-035, AC-036, AC-037, AC-038

---

## Task ↔ REQ ↔ AC 요약 매트릭스

| Task | REQs | ACs |
|------|------|-----|
| T-001 | 001, 002 | 001 |
| T-002 | 001, 002 | 002 |
| T-003 | 001, 002, 008 | 003 |
| T-004 | 001, 002, 008 | 004 |
| T-005 | 001, 002 | 005 |
| T-006 | 001, 002 | 005a |
| T-007 | 008, 018 | 006 |
| T-008 | 001, 018 | 001a |
| T-009 | 004, 026 | 007 |
| T-010 | 012, 015, 026 | 008, 009 |
| T-011 | 013 | 010 |
| T-012 | 013, 014 | 011, 012 |
| T-013 | 013 | 013 |
| T-014 | 012, 013, 021 | 014, 015 |
| T-015 | 015 | 016 |
| T-016 | 025 | 016a |
| T-017 | 003, 019 | 017, 018 |
| T-018 | 022, 023, 024 | 019, 020, 021 |
| T-019 | 005, 016, 017, 027 | 022, 023, 024 |
| T-020 | 005 | 025 |
| T-021 | 019 | 026 |
| T-022 | 009 | 027 |
| T-023 | 020, 028 | 028, 029 |
| T-024 | 009 | 030 |
| T-025 | 018, 030, 031, 032 | 031, 032, 033, 034 |
| T-026 | 007, 011, 017, 029 | 035, 036, 037, 038 |

---

Version: 1.0.0
Last Updated: 2026-05-16

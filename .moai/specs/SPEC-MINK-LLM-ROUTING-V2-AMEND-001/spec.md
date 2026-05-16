---
id: SPEC-MINK-LLM-ROUTING-V2-AMEND-001
version: 0.2.0
status: planned
amends: SPEC-GOOSE-LLM-ROUTING-V2-001
created_at: 2026-05-16
updated_at: 2026-05-16
author: manager-spec
priority: high
phase: 3
size: 대(L)
lifecycle: spec-first
labels:
  - amendment
  - llm-routing
  - provider-5-curated
  - auth-key-paste
  - oauth-codex
  - fallback-chain
  - memory-qmd-hook
  - agpl-3.0
  - mink-prefix
---

# SPEC-MINK-LLM-ROUTING-V2-AMEND-001 — 5-Provider Curated Routing + Key-Paste/OAuth Auth + MEMORY-QMD Session Export

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub 생성. SPEC-GOOSE-LLM-ROUTING-V2-001 (v0.2.1 completed, 15-direct provider) 의 amendment 스켈레톤. Provider pool 15 → 5 curated 정선, 인증 2 패턴 (key paste × 4 + OAuth × 1), MEMORY-QMD-001 export hook 의 골격만 기록. 본격 EARS / plan / tasks / acceptance 산출은 v0.2.0 로 이월. | manager-spec |
| 0.2.0 | 2026-05-16 | 본격 plan 산출 — 5 산출물 (research.md / plan.md / tasks.md / acceptance.md / progress.md) + 본 spec.md frontmatter 갱신 (draft→planned, 0.1.0→0.2.0, 중(M)→대(L)). §6 EARS 섹션 정식화: 32 REQ (Ubiquitous 11 / Event-Driven 9 / State-Driven 5 / Unwanted 4 / Optional 3). 인증 흐름 — Anthropic / DeepSeek / OpenAI / GLM = browser 새 탭 key paste, Codex = OAuth 2.1 PKCE + 127.0.0.1:auto-port + device-code fallback. Credential 저장은 SPEC-MINK-AUTH-CREDENTIAL-001 에 완전 위임 (본 SPEC 은 consumer). 라우팅 카테고리 3 (cost / quality / coding) 사용자 활성 선택. Codex 8일 idle refresh-token 만료 정책 명시. MEMORY-QMD-001 export hook 인터페이스 정의 (실 색인 = MEMORY-QMD 책임). | manager-spec |

---

## 1. 개요 (Overview)

### 1.1 한 줄 요약

SPEC-GOOSE-LLM-ROUTING-V2-001 (v0.2.1 completed) 의 15-direct provider pool 을 **5 curated provider + 임의 custom endpoint 무한 확장** 으로 재편하고, 인증 흐름을 **2 패턴 (browser 새 탭 key paste × 4 / OAuth 2.1 PKCE × 1 Codex)** 으로 표준화한다. Credential 저장 자체는 SPEC-MINK-AUTH-CREDENTIAL-001 에 완전 위임하며, LLM 응답 스트림 → 정제된 마크다운 → SPEC-MINK-MEMORY-QMD-001 session collection 으로 흘려보내는 *opt-in* export hook 의 인터페이스를 본 SPEC 에서 규정한다.

### 1.2 본 SPEC 통과 시 동작 가능 시나리오

본 SPEC 의 plan/run 결과로 다음 3 가지 시나리오가 동작해야 한다.

1. **로그인**: 사용자가 `mink login` 을 실행 → TUI 가 5 provider (Anthropic Claude / DeepSeek / OpenAI GPT / Codex ChatGPT / z.ai GLM) + "custom endpoint" 옵션을 제시 → 사용자가 Anthropic 을 선택하면 시스템이 `https://console.anthropic.com/settings/keys` 를 브라우저 새 탭으로 열고 paste 입력 UI 를 표시 → 사용자가 `sk-ant-...` 를 paste → POST `/install/provider/save` (web onboarding 경로) 또는 stdin (CLI 경로) → AUTH-CREDENTIAL-001 의 `Store(provider, key)` 호출 → 성공 응답. Codex 만 예외로 OAuth flow.
2. **모델 지정**: 사용자가 `mink model set claude-opus-4-7` 또는 `mink model set deepseek-reasoner | gpt-5.5 | codex-gpt-5.5 | glm-5-turbo | <custom-endpoint>:<model>` 을 실행 → router 가 활성 provider pool 에 등록된 모델인지 검증 후 `~/.config/mink/active-model` (또는 keyring meta) 에 저장.
3. **질의**: 사용자가 `mink ask "..."` 를 실행 → router 가 활성 routing category (cost / quality / coding) 에 따라 1순위 provider 호출 → 첫 실패 시 fallback chain 의 다음 후보로 자동 전환 → 모든 provider 실패 시 14 FailoverReason 분류 + 마지막 에러 상세를 사용자 stdout 으로 표면화.

### 1.3 본 SPEC 이 다루지 않는 것

- **Credential 저장 구현**: SPEC-MINK-AUTH-CREDENTIAL-001 책임. 본 SPEC 은 `CredentialStore` 인터페이스 *consumer* 일 뿐.
- **MEMORY-QMD session 색인 / 검색 / 임베딩**: SPEC-MINK-MEMORY-QMD-001 책임. 본 SPEC 은 *export hook 인터페이스* 만 정의.
- **자체 모델 호스팅 / weight 학습 / QLoRA / RL**: ADR-001 정합, 본 amendment 도 동일하게 외부 호출만 다룬다.
- **15-direct provider 풀 유지**: v0.2.1 의 OpenRouter / Together / Anyscale / Groq / Fireworks / Cerebras / Mistral / Qwen / Kimi 등 12 잔여 provider 는 모두 *custom OpenAI-compatible endpoint* 로 흡수 (사용자가 직접 추가 가능). 본 SPEC 은 curated 5 만 1차 시민으로 둔다.

---

## 2. 배경 (Background)

### 2.1 v0.2.1 (15-direct) 의 한계 — 사용자 결정 2026-05-16

| 사유 | 상세 |
|-----|------|
| **인증 UX 일관성 손실** | 15 provider 각각의 OAuth/API key/CLI tool 인증 흐름이 모두 달라 onboarding TUI 가 분기 폭증 |
| **유지보수 비용** | 15 provider × (rate-limit header 포맷 / capability 매트릭스 / pricing 표) = 60+ 변동 포인트, Sprint 1~3 동안 평균 월 3건 drift 발생 |
| **품질 편차** | 15 중 3~4 개 (OpenAI / Anthropic / DeepSeek) 가 사용자 query 의 95% 흡수, 나머지 11 개는 long-tail. 라우팅 결정 복잡도 대비 ROI 낮음 |
| **curated 정선 의도** | "잘 검증된 5 + 사용자가 직접 추가하는 custom endpoint 무한" 모델로 전환 → 1차 시민 5 의 품질을 끌어올리고, custom 은 OpenAI-compat 단일 인터페이스로 흡수 |

### 2.2 5 Curated Provider — 선정 근거

| Provider | 모델 | 인증 | 강점 |
|---------|------|-----|-----|
| Anthropic Claude | claude-opus-4-7, claude-sonnet-4-7 | key paste (`sk-ant-...`) | 최상위 reasoning, 1M context, prompt caching |
| DeepSeek | deepseek-reasoner, deepseek-chat | key paste (`sk-...`) | 가성비 최강 (~$0.27/M), 한국어 양호 |
| OpenAI GPT (API) | gpt-5.5, gpt-5.5-mini | key paste (`sk-...`) | function calling 표준, vision, realtime |
| Codex (ChatGPT) | codex-gpt-5.5 | OAuth 2.1 PKCE | 코딩 특화, ChatGPT Plus/Pro 정액 |
| z.ai GLM | glm-5-turbo, glm-5-coding | key paste | 코딩 plan 저가, 중국어 1급 |

### 2.3 Custom Endpoint 흡수 전략

OpenAI-compatible API 를 노출하는 임의 endpoint (`https://api.openrouter.ai/v1`, `http://localhost:11434/v1` Ollama, `https://api.together.xyz/v1`, self-host vLLM, lm-studio 등) 를 `mink model add-custom <name> <base_url> <model>` 로 무한 추가 가능. 인증은 동일하게 key paste. 본 SPEC 은 OpenAI-compat 스키마 호환성만 보장한다.

### 2.4 폐기 사항 (도입 금지)

- 자체 모델 호스팅 / vLLM 내장 / Ollama 의존 (Ollama 는 MEMORY-QMD-001 의 임베딩 sidecar 용도만 — LLM 호출 안 함)
- 15-direct adapter 풀 잔여 12 provider 의 1차 시민 지위
- QLoRA / RL / weight fine-tuning (ADR-001)
- 클라우드 keyring (AUTH-CREDENTIAL-001 의 평문 fallback 만 폴백)

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **Provider 추상화 통일**: 단일 `Provider` 인터페이스로 5 curated + custom 모두 추상화 (`Chat(ctx, req) (stream, error)` + `Capabilities() ProviderCapabilities`).
2. **5 어댑터 구현**: Anthropic-native (Messages API) 1 종 + OpenAI-compat 3 종 (DeepSeek / OpenAI / GLM) + Codex (ChatGPT OAuth backend) 1 종.
3. **2 인증 패턴**:
   - **Key paste** (4 provider): 브라우저 새 탭 자동 open → 사용자 paste → POST `/install/provider/save` 또는 stdin → AUTH-CREDENTIAL-001 위임 저장.
   - **OAuth PKCE** (Codex): 127.0.0.1:auto-port browser callback + device-code flow fallback (headless 환경).
4. **라우팅 카테고리 3**:
   - **cost-first**: DeepSeek (1st) → GLM (2nd) → ...
   - **quality-first**: Claude Opus (1st) → GPT-5.5 (2nd) → ...
   - **coding-first**: Codex (1st) → GLM-Coding (2nd) → Claude Sonnet (3rd) → ...
5. **Fallback chain**: 활성 카테고리의 우선순위 → 첫 실패 → 다음 후보 → 모두 실패 → 사용자 상세 에러 표면화. 14 FailoverReason 분류 (v0.2.1 ERROR-CLASS-001 재사용).
6. **MEMORY-QMD-001 export hook 인터페이스**: `SessionExporter` 인터페이스 정의 (`OnStreamChunk(chunk) error` + `OnStreamComplete(meta) error`). 실 색인은 MEMORY-QMD-001 책임.
7. **CLI/TUI 통합**: `mink login` / `mink model {set|list|add-custom}` / `mink ask` / `mink routing {set|show}` 명령.

### 3.2 OUT OF SCOPE

- Credential 저장 구현 (AUTH-CREDENTIAL-001 위임)
- MEMORY-QMD session 색인 / 검색 (MEMORY-QMD-001 위임)
- Provider 자체 호스팅 / weight 학습
- 학습 기반 (RL-from-routing-feedback) 라우팅
- multi-region routing (region 은 custom endpoint 옵션으로만)
- 클라우드 keyring 동기화

---

## 4. 가정 (Assumptions, Surface Assumptions Box)

> [HARD] Surface Assumptions — 다음 가정은 plan 단계에서 명시적으로 surface 한다. plan-auditor 가 §4 ↔ §5 cross-read 시 충돌 0 이어야 한다.

| # | 가정 | 위반 시 위험 |
|---|------|----------|
| A1 | AUTH-CREDENTIAL-001 이 본 SPEC plan 종료 시점에 `Store(provider, key) error` + `Load(provider) (key, error)` + `Delete(provider) error` 3 메서드를 노출하는 stable 인터페이스로 존재 | 위반 시 본 SPEC 은 credential mock 으로 plan 검증 → run 단계에서 AUTH 미구현이면 stub 사용 |
| A2 | SPEC-MINK-MEMORY-QMD-001 의 `sessions/` collection 이 markdown chunk + metadata `{provider, model, timestamp, hash}` 페이로드를 수용 | 위반 시 본 SPEC 의 `SessionExporter` 인터페이스를 no-op 으로 두고 후속 amendment 로 재배선 |
| A3 | 5 curated provider 모두 API key 또는 OAuth token 으로 인증 가능 (별도 region account / 결제 등록 등 외부 사전 작업은 사용자 책임) | 위반 시 onboarding flow 의 paste 단계가 "유효성 검증 실패" 만 표면화, 사용자가 manual 해결 |
| A4 | OAuth 2.1 PKCE callback 을 127.0.0.1:auto-port 으로 받을 수 있는 환경 (localhost 바인딩 허용). headless / sandboxed 환경은 device-code flow 폴백 | 위반 시 device-code flow 만 사용, UX 다소 저하 |
| A5 | Codex (ChatGPT) refresh-token 만료 정책 = 8일 idle (관찰 기반). 만료 시 명시적 `mink login codex` 재실행 | 만료 정책이 변경되면 사용자 재로그인 요청 메시지만 갱신 |
| A6 | 14 FailoverReason 분류 (ERROR-CLASS-001) 가 5 curated provider 의 모든 실패 케이스를 표현 가능 | 위반 시 `unknown` 분류로 처리 + warning log + 후속 amendment |
| A7 | OpenAI-compat 스키마 (Chat Completions /v1/chat/completions) 가 custom endpoint 의 *최소* 인터페이스로 충분 | 위반 시 해당 custom endpoint 비호환 안내 + 추가는 사용자 책임 |
| A8 | conversation_language=ko / code_comments=en 정책 정합. 본 SPEC 문서 본문은 한국어, 모든 Go 식별자·주석은 영어 | — |

---

## 5. 위험 (Risks)

| # | 위험 | 등급 | 완화 |
|---|------|----|----|
| R1 | OAuth callback port 충돌 (다른 프로세스가 127.0.0.1:8765 사용 중) | M | auto-port (0-bind → OS allocate) + 실패 시 device-code 폴백 |
| R2 | 5 provider rate-limit 정책 비대칭 (예: Anthropic = TPM 우선, DeepSeek = RPM 우선) | M | RATELIMIT-001 4-bucket tracker 재사용, 정책 카테고리별 80% 임계 |
| R3 | Codex API 가 비공식 ChatGPT backend → 정책 변경 위험 | H | hard dependency 표기 + 정책 변경 감지 시 user-facing warning |
| R4 | Custom endpoint 의 streaming SSE 포맷이 OpenAI 와 미세 차이 → 응답 깨짐 | M | 단위 테스트 with golden fixture 5종 (OpenRouter / Together / Ollama / vLLM / lm-studio) |
| R5 | MEMORY-QMD export hook 이 stream 동기 차단 → 응답 지연 | M | hook 은 비동기 channel 발행 only, MEMORY-QMD 측이 consume |
| R6 | AUTH-CREDENTIAL-001 plaintext fallback 시 사용자가 의식하지 못함 | M | onboarding TUI / web UI 에서 keyring unavailable 경고 명시 (위임처 책임, 본 SPEC consumer 측 표면화 협조) |
| R7 | 8일 idle refresh-token 만료를 사용자가 인지 못 한 채 `mink ask` 호출 → 갑작스러운 실패 | M | 만료 임박 (예: 7일 경과) 시점에서 첫 호출에 warning surface |
| R8 | Surface Assumptions §4 의 A1 (AUTH 인터페이스) drift — AUTH-CREDENTIAL-001 가 인터페이스 시그니처 변경 | H | 본 SPEC plan 단계에서 AUTH 측 인터페이스 freeze 합의 → SPEC contract 로 명시 |

> **A vs R cross-read pass** — A1 ↔ R8, A2 ↔ R5, A4 ↔ R1, A5 ↔ R7. 충돌 0.

---

## 6. EARS 요구사항 (32 REQ)

EARS 패턴 분포: Ubiquitous 11 / Event-Driven 9 / State-Driven 5 / Unwanted 4 / Optional 3. 각 REQ 에 priority (P0/P1/P2) 라벨.

### 6.1 Ubiquitous (시스템 상시 작동) — 11 REQ

**REQ-RV2A-001 (P0)** — `internal/llm/provider/` 시스템은 5 curated provider (Anthropic / DeepSeek / OpenAI / Codex / z.ai GLM) 와 사용자 정의 OpenAI-compatible custom endpoint 를 단일 `Provider` 인터페이스로 추상화 **shall**.

**REQ-RV2A-002 (P0)** — `Provider` 인터페이스는 `Chat(ctx context.Context, req ChatRequest) (ChatStream, error)` 와 `Capabilities() ProviderCapabilities` 두 메서드를 노출 **shall**.

**REQ-RV2A-003 (P0)** — `internal/llm/router/v2/` 의 라우팅 결정은 **활성 routing category** (`cost-first` / `quality-first` / `coding-first`) 단일 카테고리에만 근거 **shall**.

**REQ-RV2A-004 (P0)** — Credential 저장·로드·삭제는 `internal/llm/auth/` 가 SPEC-MINK-AUTH-CREDENTIAL-001 의 `CredentialStore` 인터페이스에만 위임 **shall** — 본 SPEC 의 구현체는 어떤 credential 도 자체 저장하지 **shall not**.

**REQ-RV2A-005 (P0)** — `internal/llm/fallback/` 의 chain 실행은 SPEC-GOOSE-ERROR-CLASS-001 의 14 FailoverReason 열거값에만 분기 근거 **shall**.

**REQ-RV2A-006 (P1)** — `mink ask` 의 모든 응답은 stream-first 로 제공 **shall** (`Provider.Chat` 결과 `ChatStream` 을 stdout 으로 직접 forward).

**REQ-RV2A-007 (P1)** — 본 SPEC 의 모든 코드 산출물은 AGPL-3.0-only 라이선스 헌장 (ADR-002) 을 준수 **shall**.

**REQ-RV2A-008 (P1)** — Custom endpoint 는 OpenAI Chat Completions 스키마 (`POST /v1/chat/completions` + SSE streaming) 와 호환 **shall**.

**REQ-RV2A-009 (P1)** — `internal/llm/router/v2/` 패키지는 `MEMORY-QMD-001` 의 `SessionExporter` 인터페이스를 *optional consumer* 로 호출 **shall** — 인터페이스 미구현 시 no-op.

**REQ-RV2A-010 (P2)** — 본 SPEC 의 모든 사용자 대면 에러 메시지는 한국어, 모든 코드 주석·로그·메트릭 라벨은 영어 (language.yaml 정합) **shall**.

**REQ-RV2A-011 (P0)** — TRUST 5 (Tested / Readable / Unified / Secured / Trackable) 정합. 패키지 단위 coverage ≥ 85%, lint 0 violation, security scan PASS **shall**.

### 6.2 Event-Driven (이벤트 트리거) — 9 REQ

**REQ-RV2A-012 (P0)** — **When** 사용자가 `mink login <provider>` 를 실행 (provider ∈ {anthropic, deepseek, openai, glm}), `internal/cli/commands/login.go` 는 해당 provider 의 API key 발급 페이지를 브라우저 새 탭으로 열고 stdin paste 입력을 대기 **shall**.

**REQ-RV2A-013 (P0)** — **When** 사용자가 `mink login codex` 를 실행, `internal/llm/provider/codex/oauth.go` 는 OAuth 2.1 PKCE 흐름을 시작 **shall** — code_verifier 생성 + 127.0.0.1:0 callback server 바인딩 + ChatGPT authorize URL 을 브라우저 새 탭으로 open.

**REQ-RV2A-014 (P0)** — **When** OAuth callback 이 127.0.0.1:auto-port 으로 도착, callback handler 는 `code` 파라미터를 token endpoint 로 교환하여 access_token + refresh_token 을 AUTH-CREDENTIAL-001 위임 저장 **shall**.

**REQ-RV2A-015 (P1)** — **When** 사용자가 web onboarding 의 `/install/provider/save` 엔드포인트로 paste 한 API key 를 POST, server 는 key 의 형식 검증 (regex prefix 확인) 후 AUTH-CREDENTIAL-001 위임 저장 **shall**.

**REQ-RV2A-016 (P0)** — **When** `Provider.Chat` 호출이 14 FailoverReason 중 하나 (rate_limit / auth / network / server_5xx / model_not_found / context_window_exceeded 등) 로 실패, `internal/llm/fallback/` 은 활성 카테고리 chain 의 다음 후보로 자동 전환 **shall**.

**REQ-RV2A-017 (P0)** — **When** 활성 카테고리 chain 의 *모든* 후보가 실패, `internal/llm/router/v2/` 는 마지막 실패의 FailoverReason + 상세 에러 메시지 + 시도한 provider 목록을 사용자 stdout 으로 표면화 **shall**.

**REQ-RV2A-018 (P1)** — **When** 사용자가 `mink model add-custom <name> <base_url> <model>` 을 실행, `internal/llm/provider/custom/` 은 base_url 의 `/v1/chat/completions` 가 OpenAI-compat 스키마를 반환하는지 health-check 후 등록 **shall**.

**REQ-RV2A-019 (P1)** — **When** 사용자가 `mink routing set {cost|quality|coding}` 을 실행, `internal/llm/router/v2/policy.go` 는 활성 카테고리를 `~/.config/mink/routing.yaml` 의 `active_category` 필드에 atomic write **shall**.

**REQ-RV2A-020 (P1)** — **When** Provider 의 응답 stream 이 `OnStreamComplete` 시점에 도달, `internal/llm/router/v2/` 는 등록된 `SessionExporter` 에 `(chunks []string, meta SessionMeta)` 페이로드를 *비동기* 발행 **shall** — 발행 실패가 응답 stream 을 차단해서는 **shall not**.

### 6.3 State-Driven (상태 조건) — 5 REQ

**REQ-RV2A-021 (P1)** — **While** OAuth flow 가 pending 상태 (callback 대기 중) 인 동안, `mink login codex` 는 stdout 에 현재 단계 (browser open / await callback / token exchange) 를 표시 **shall**.

**REQ-RV2A-022 (P0)** — **While** 활성 routing category 가 `cost-first` 인 동안, router 는 DeepSeek → z.ai GLM → Anthropic Sonnet → OpenAI GPT-mini → Codex 순으로 후보 정렬 **shall**.

**REQ-RV2A-023 (P0)** — **While** 활성 routing category 가 `quality-first` 인 동안, router 는 Claude Opus → GPT-5.5 → DeepSeek Reasoner → GLM-5-Turbo → Codex 순으로 후보 정렬 **shall**.

**REQ-RV2A-024 (P0)** — **While** 활성 routing category 가 `coding-first` 인 동안, router 는 Codex → GLM-Coding → Claude Sonnet → DeepSeek → GPT-5.5 순으로 후보 정렬 **shall**.

**REQ-RV2A-025 (P1)** — **While** Codex refresh-token 의 마지막 사용 시각이 7일 초과 (8일 만료 임박), 첫 `mink ask` 호출은 stdout 에 "7d since last Codex use — refresh recommended" warning 을 표면화 **shall**.

### 6.4 Unwanted (금지) — 4 REQ

**REQ-RV2A-026 (P0)** — `internal/llm/` 의 어떤 구현체도 사용자 API key / OAuth token 을 평문 디스크 파일로 저장 **shall not** — 모든 저장은 AUTH-CREDENTIAL-001 위임 (keyring default, 명시적 fallback 만 평문).

**REQ-RV2A-027 (P0)** — Fallback chain 은 활성 카테고리의 상위 후보 실패 시 *다른 카테고리* 의 후보로 자동 전환 **shall not** — 카테고리 전환은 명시적 `mink routing set` 만.

**REQ-RV2A-028 (P0)** — `SessionExporter` hook 의 발행 실패 / MEMORY-QMD-001 의 수신 거절은 사용자 응답 stream 을 silent drop **shall not** — warning log + 다음 응답 정상 계속.

**REQ-RV2A-029 (P1)** — 본 SPEC 의 어떤 산출물도 자체 모델 weight 호스팅 / fine-tuning / RL 코드를 포함 **shall not** (ADR-001 정합).

### 6.5 Optional (선택 기능) — 3 REQ

**REQ-RV2A-030 (P2)** — **Where** custom endpoint 가 사용자 환경에 사전 정의된 template (`openrouter`, `vllm`, `lm-studio`, `ollama-openai-compat`) 으로 추가 가능한 경우, `mink model add-custom --template <name>` 은 base_url + 기본 인증 패턴을 자동 채움 **shall**.

**REQ-RV2A-031 (P2)** — **Where** 사용자가 비용 절감을 위해 streaming 을 비활성화하길 원하는 경우, `mink ask --no-stream` 옵션은 batch response 모드로 전환 **shall**.

**REQ-RV2A-032 (P2)** — **Where** 사용자가 routing 결정을 명시적으로 override 하길 원하는 경우, `mink ask --provider <name> --model <model>` 은 활성 카테고리를 무시하고 단일 provider 호출 **shall**.

---

## 7. 데이터 흐름 (Data Flow)

```
[user]
  │ mink login anthropic
  ▼
[CLI / TUI] ── browser open ──▶ console.anthropic.com/settings/keys
  │                                       │
  │ ◀── user paste sk-ant-... ────────────┘
  │ stdin
  ▼
[internal/llm/auth/keypaste.go]
  │ validate regex
  ▼
[AUTH-CREDENTIAL-001 ::CredentialStore.Store("anthropic", key)]
  │ keyring or plaintext fallback
  ▼
[user]
  │ mink ask "summarize this PR"
  ▼
[internal/llm/router/v2/route.go]
  │ active_category = "quality-first"
  │ candidates = [claude-opus-4-7, gpt-5.5, ...]
  ▼
[internal/llm/provider/anthropic/client.go]
  │ Chat(req) → ChatStream
  ▼
[stdout] ◀── SSE chunks ──┐
                          │
                          └──▶ [SessionExporter.OnStreamChunk] (async)
                                       │
                                       ▼
                               [MEMORY-QMD-001 sessions/]
```

---

## 8. 변경 영향 (Impact)

### 8.1 신규 패키지

- `internal/llm/provider/anthropic/`
- `internal/llm/provider/deepseek/`
- `internal/llm/provider/openai/`
- `internal/llm/provider/codex/` (OAuth flow 포함)
- `internal/llm/provider/zai/` (GLM)
- `internal/llm/provider/custom/` (OpenAI-compat 임의 endpoint)
- `internal/llm/router/v2/` (5 curated 한정 라우팅)
- `internal/llm/auth/` (key paste + OAuth client, AUTH-CREDENTIAL-001 consumer)
- `internal/llm/fallback/` (14 FailoverReason chain)
- `internal/llm/export/` (SessionExporter 인터페이스 + no-op default)

### 8.2 갱신 / supersede 대상

- SPEC-GOOSE-LLM-ROUTING-V2-001 (v0.2.1 completed) — 본 amendment 머지 시 `completed` 유지하되 본 SPEC 으로 활성 라우팅 위임 (구 패키지 `internal/llm/router/v2/` 는 본 SPEC 의 `internal/llm/router/v2/` 와 동일 경로지만 *재작성* — 단일 active path 정책).
- 기존 15-direct adapter 코드는 *유지* (백워드 호환 path) 하되, default 활성 pool 은 5 로 전환.

### 8.3 의존 SPEC

- SPEC-MINK-AUTH-CREDENTIAL-001 (credential 저장 위임처, A1)
- SPEC-MINK-MEMORY-QMD-001 (session export hook 수신처, A2)
- SPEC-GOOSE-ERROR-CLASS-001 v0.2.x (14 FailoverReason, completed)
- SPEC-GOOSE-RATELIMIT-001 (4-bucket tracker, completed)

---

## 9. 마일스톤 매핑 (Milestone ↔ REQ)

| Milestone | 범위 | REQ |
|----|---|-----|
| M1 | Provider 어댑터 5종 | REQ-001, -002, -006, -008, -010 |
| M2 | 인증 흐름 (key paste + OAuth) | REQ-004, -012, -013, -014, -015, -021, -025, -026 |
| M3 | 라우팅 정책 + fallback chain | REQ-003, -005, -016, -017, -019, -022, -023, -024, -027, -032 |
| M4 | MEMORY-QMD export hook 인터페이스 | REQ-009, -020, -028 |
| M5 | CLI/TUI 통합 + E2E + custom endpoint | REQ-018, -030, -031, -007, -011, -029 |

상세 task 분해는 `tasks.md`, AC 매핑은 `acceptance.md` 참조.

---

## 10. 라이선스 (License)

본 SPEC 의 모든 산출물은 **AGPL-3.0-only** 헌장 (ADR-002) 을 준수한다. 외부 사용자가 본 SPEC 의 구현을 fork / SaaS 형태로 변형 배포 시 source 공개 의무가 동일 적용된다.

---

## 11. TRUST 5 정합

| 항목 | 본 SPEC 정합 |
|---|---|
| Tested | acceptance.md 38 AC + tasks.md unit/integration/e2e test scaffold, coverage ≥ 85% |
| Readable | EARS 명세, naming convention 정합, code_comments=en |
| Unified | 단일 `Provider` 인터페이스, 단일 fallback chain, 단일 credential 위임 경로 |
| Secured | API key 평문 저장 금지 (REQ-RV2A-026), OAuth PKCE, regex 검증 |
| Trackable | 32 REQ + 41 AC traceable mapping (1:N), progress.md 마일스톤 추적. AC-039/040 audit B2 fix |

---

## 12. 참조

- 이전 SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001 v0.2.1 (completed)
- 의존 SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (planned), SPEC-MINK-MEMORY-QMD-001 (planned)
- 외부: Anthropic Messages API / OpenAI Chat Completions / DeepSeek API / z.ai GLM API / OAuth 2.1 PKCE (RFC 7636) / Device Authorization Grant (RFC 8628)
- ADR-001 (no self-host LLM), ADR-002 (AGPL-3.0-only)

---

Version: 0.2.0
Status: planned
Last Updated: 2026-05-16

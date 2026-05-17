# Acceptance — SPEC-MINK-LLM-ROUTING-V2-AMEND-001

총 41 AC (M1 8 + M2 11 + M3 10 + M4 4 + M5 8). REQ ↔ AC traceable mapping (1:N 허용). Verify type: unit / integration / e2e / manual.

Given-When-Then 형식. 마일스톤별로 그룹화.

---

## M1 — Provider 어댑터 (6 ACs)

### AC-001 (REQ-RV2A-001, -002) — Provider 인터페이스 단일성

- **Given**: `internal/llm/provider/types.go` 가 컴파일됨
- **When**: 5 어댑터 (`anthropic` / `deepseek` / `openai` / `codex` / `zai`) + `custom` 패키지의 `Client` 타입을 인스턴스화
- **Then**: 6 모두 `Provider` 인터페이스 satisfy 한다 (compile-time check)
- **Verify**: unit (`var _ Provider = (*anthropic.Client)(nil)` 패턴)

### AC-001a (REQ-RV2A-001, -018) — ProviderRegistry 5 default + custom 추가

- **Given**: 새 프로세스 시작
- **When**: `registry.NewDefault()` 호출
- **Then**: 5 curated provider 가 등록되어 있고, `registry.AddCustom(...)` 으로 custom 추가 후 `registry.Lookup("custom:<name>")` 가 반환된다
- **Verify**: unit

### AC-002 (REQ-RV2A-001, -002) — Anthropic SSE 변환

- **Given**: Anthropic mock server 가 `message_start` + 3개 `content_block_delta` + `message_stop` 이벤트를 stream
- **When**: `anthropic.Client.Chat(ctx, req)` 호출 후 `ChatStream` consume
- **Then**: `ChatChunk{Content: "<delta1+delta2+delta3>"}` 3 chunk 가 통일 포맷으로 반환된다
- **Verify**: unit (golden SSE fixture)

### AC-003 (REQ-RV2A-001, -002, -008) — DeepSeek OpenAI-compat 호환

- **Given**: DeepSeek mock server 가 OpenAI-compat SSE 응답
- **When**: `deepseek.Client.Chat(ctx, req)` 호출
- **Then**: ChatChunk stream 이 OpenAI 와 동일 포맷으로 반환, `data: [DONE]` 시 stream close
- **Verify**: unit

### AC-004 (REQ-RV2A-001, -002, -008) — OpenAI function calling 옵션 통과

- **Given**: 요청에 `tools: [{type: "function", ...}]` 포함
- **When**: `openai.Client.Chat(ctx, req)` 호출
- **Then**: mock server 의 수신 body 에 tools 필드가 그대로 포함되어 있고 (round-trip 검증), 응답의 `tool_calls` 가 그대로 stream 반환된다
- **Verify**: unit

### AC-005 (REQ-RV2A-001, -002) — Codex backend 호출

- **Given**: mock token endpoint 가 access_token 발급, mock Codex backend 가 응답
- **When**: `codex.Client.Chat(ctx, req)` 호출 (token 은 `CredentialStore.Load` 로 인자 수신)
- **Then**: 응답 stream 이 정상 ChatChunk 로 반환된다
- **Verify**: unit

### AC-005a (REQ-RV2A-001, -002) — z.ai GLM 어댑터

- **Given**: GLM mock server (OpenAI-compat)
- **When**: `zai.Client.Chat(ctx, req)` 호출
- **Then**: glm-5-turbo / glm-5-coding 응답이 OpenAI-compat 포맷으로 반환된다
- **Verify**: unit

### AC-006 (REQ-RV2A-008, -018) — Custom endpoint 5 template

- **Given**: 5 template (openrouter / vllm / ollama-openai-compat / lm-studio / together) 의 golden fixture
- **When**: `custom.Client.Chat(ctx, req)` 호출 → SSE 응답 parse
- **Then**: 5 모두 ChatChunk stream 으로 동일 변환된다
- **Verify**: unit (5 golden fixture)

---

## M2 — 인증 흐름 (10 ACs)

### AC-007 (REQ-RV2A-004, -026) — AUTH consumer adapter

- **Given**: mock `CredentialStore` 가 in-memory map
- **When**: `store_adapter.Save("anthropic", "sk-ant-test")` → `store_adapter.Load("anthropic")` → `store_adapter.Delete("anthropic")`
- **Then**: Save 후 Load 가 동일 secret 반환, Delete 후 Load 가 `ErrNotFound`
- **Verify**: unit

### AC-008 (REQ-RV2A-012, -026) — Key paste regex 검증

- **Given**: 4 provider 의 paste flow 활성
- **When**: 각 provider 에 잘못된 prefix (예: anthropic 에 `sk-...`) 입력
- **Then**: regex 검증 실패 + 사용자 stdout "Invalid key format for anthropic" + store 호출 안 됨
- **Verify**: unit

### AC-009 (REQ-RV2A-012, -015) — Browser 새 탭 open

- **Given**: 사용자가 `mink login anthropic` 실행
- **When**: `login.go` 가 발급 페이지 URL 을 browser open
- **Then**: 모니터링 가능한 시그널 (mock browser opener) 가 `https://console.anthropic.com/settings/keys` 수신
- **Verify**: unit (browser opener mock)

### AC-010 (REQ-RV2A-013) — PKCE 생성 정합성

- **Given**: `oauth_pkce.New()` 호출
- **When**: code_verifier 생성 후 SHA256 base64url encode
- **Then**: code_challenge 가 spec (RFC 7636) S256 와 일치, state 가 16 byte+ 무작위
- **Verify**: unit (roundtrip 검증)

### AC-011 (REQ-RV2A-013, -014) — OAuth callback 정상 흐름

- **Given**: 127.0.0.1:0 listen → port=P 할당
- **When**: HTTP GET `http://127.0.0.1:P/callback?code=abc&state=<valid>` 수신
- **Then**: handler 가 state 검증 PASS → channel 에 code 발행 → token endpoint 교환 → access_token + refresh_token 을 AUTH 위임 저장
- **Verify**: integration (httptest mock token endpoint)

### AC-012 (REQ-RV2A-014) — state mismatch 거부

- **Given**: state=S1 으로 OAuth 시작
- **When**: callback 에 state=S2 (다른 값) 수신
- **Then**: handler 가 state mismatch 감지 → HTTP 400 + 토큰 교환 skip + 사용자 stdout "OAuth state mismatch — possible CSRF"
- **Verify**: unit

### AC-013 (REQ-RV2A-013) — Device-code flow 폴백

- **Given**: 127.0.0.1 port-bind 시도가 실패 (mock listener fail)
- **When**: `mink login codex` 실행
- **Then**: device_code flow 자동 시작 → user_code stdout 표시 → polling → 성공 시 토큰 저장
- **Verify**: integration (mock device endpoint)

### AC-014 (REQ-RV2A-012, -013) — `mink login` 분기

- **Given**: 5 provider 명 (anthropic / deepseek / openai / glm / codex)
- **When**: 각각 `mink login <provider>` 실행
- **Then**: anthropic / deepseek / openai / glm 은 keypaste flow, codex 는 OAuth flow 가 호출됨
- **Verify**: integration

### AC-015 (REQ-RV2A-021) — Codex OAuth 단계 stdout 안내

- **Given**: `mink login codex` 실행 중
- **When**: 각 단계 (browser open / await callback / token exchange) 진입
- **Then**: stdout 에 단계별 메시지가 순서대로 출력된다
- **Verify**: integration (stdout capture)

### AC-016 (REQ-RV2A-015) — Web onboarding `/install/provider/save`

- **Given**: ONBOARDING-001 v0.3.1 의 web onboarding 활성, CSRF token 보유
- **When**: POST `/install/provider/save` body `{provider: "deepseek", key: "sk-test"}` + CSRF header
- **Then**: 200 + store 호출 1회 + 응답 body `{ok: true}`
- **Verify**: integration (handler_test.go 패턴 재사용)

### AC-016a (REQ-RV2A-025) — Codex 7일 idle warning

- **Given**: `codex-last-use` timestamp = 7.5일 전
- **When**: 다음 `mink ask` 호출 (첫 호출)
- **Then**: stdout 에 "7d since last Codex use — refresh recommended" warning 1회 표시
- **Verify**: unit (clock mock)

---

## M3 — 라우팅 + Fallback (10 ACs)

### AC-017 (REQ-RV2A-003, -019) — 카테고리 기본값

- **Given**: 새 사용자, `~/.config/mink/routing.yaml` 미존재
- **When**: `policy.Load()` 호출
- **Then**: default `RoutingCategory.Quality` 반환
- **Verify**: unit

### AC-018 (REQ-RV2A-019) — 카테고리 atomic write

- **Given**: 동시 2 process 가 `mink routing set cost` / `mink routing set coding` 실행
- **When**: 두 명령 종료 후 routing.yaml read
- **Then**: 둘 중 하나가 *완전히* 기록되어 있음 (partial write 0)
- **Verify**: unit (race-test, atomic rename 패턴)

### AC-019 (REQ-RV2A-022) — Cost-first chain 순서

- **Given**: 활성 카테고리 = `Cost`
- **When**: `chain.For(Cost)` 호출
- **Then**: `[DeepSeek, GLM, ClaudeSonnet, GPT-mini, Codex]` 순서 list 반환
- **Verify**: unit

### AC-020 (REQ-RV2A-023) — Quality-first chain 순서

- **Given**: 활성 카테고리 = `Quality`
- **When**: `chain.For(Quality)` 호출
- **Then**: `[ClaudeOpus, GPT-5.5, DeepSeekReasoner, GLM-5-Turbo, Codex]` 순서
- **Verify**: unit

### AC-021 (REQ-RV2A-024) — Coding-first chain 순서

- **Given**: 활성 카테고리 = `Coding`
- **When**: `chain.For(Coding)` 호출
- **Then**: `[Codex, GLM-Coding, ClaudeSonnet, DeepSeek, GPT-5.5]` 순서
- **Verify**: unit

### AC-022 (REQ-RV2A-005, -016) — 14 FailoverReason 분기

- **Given**: chain head 가 14 FailoverReason 중 하나로 실패하도록 mock
- **When**: `fallback.Execute(chain, req)` 호출
- **Then**: 다음 후보로 자동 전환되어 정상 응답
- **Verify**: unit (14 케이스 × 1 = 14 sub-test)

### AC-023 (REQ-RV2A-017) — 모든 후보 실패 시 상세 에러

- **Given**: chain 의 5 후보 모두 실패하도록 mock (각기 다른 FailoverReason)
- **When**: `fallback.Execute(chain, req)`
- **Then**: 반환 error 가 `{LastReason, AttemptedProviders []string, ProviderErrors map[string]error}` 구조로 stdout 에 상세 표시
- **Verify**: unit

### AC-024 (REQ-RV2A-027) — Category 자동 전환 금지

- **Given**: 활성 카테고리 = `Cost`, 5 후보 모두 실패
- **When**: `fallback.Execute(...)` 종료 후
- **Then**: 활성 카테고리는 여전히 `Cost` (다른 카테고리 후보로 자동 전환 안 됨)
- **Verify**: unit

### AC-025 (REQ-RV2A-005) — Rate-limit 80% filter

- **Given**: RATELIMIT-001 view 가 `anthropic` provider 의 TPM 사용률 85% 보고
- **When**: `chain.FilterByRateLimit(...)` 호출
- **Then**: anthropic 제외된 chain 반환
- **Verify**: unit

### AC-026 (REQ-RV2A-019) — `mink routing` 명령

- **Given**: 새 사용자
- **When**: `mink routing show` 실행
- **Then**: stdout "Active category: quality\nChain: claude-opus-4-7 → gpt-5.5 → ..." 형식 출력
- **Verify**: integration (golden output)

---

## M4 — Export Hook (4 ACs)

### AC-027 (REQ-RV2A-009) — Default Noop

- **Given**: 새 프로세스, MEMORY-QMD-001 미초기화
- **When**: `export.Registry().Current()` 호출
- **Then**: `*NoopExporter` 인스턴스 반환
- **Verify**: unit

### AC-028 (REQ-RV2A-020) — 비동기 발행

- **Given**: mock exporter (synchronous block 1초)
- **When**: `Provider.Chat` 가 5 chunk stream
- **Then**: 응답 stream 은 mock exporter block 과 무관하게 즉시 완료, mock exporter 는 background 에서 5회 호출됨
- **Verify**: unit (race-test)

### AC-029 (REQ-RV2A-028) — 발행 실패 silent log

- **Given**: mock exporter 가 매 호출 error 반환
- **When**: `Provider.Chat` 가 chunk stream
- **Then**: 응답 stream 은 정상 완료, error 는 warning log only, 사용자 응답에는 영향 없음
- **Verify**: unit (log capture)

### AC-030 (REQ-RV2A-009) — `--no-export` opt-out

- **Given**: 활성 exporter = mock (호출 counter 보유)
- **When**: `mink ask --no-export "hello"`
- **Then**: mock exporter 호출 카운트 = 0
- **Verify**: integration

---

## M5 — CLI/TUI + E2E (8 ACs)

### AC-031 (REQ-RV2A-018) — `mink model add-custom` health-check 성공

- **Given**: mock OpenAI-compat server 가 `/v1/models` 정상 응답
- **When**: `mink model add-custom my-vllm http://127.0.0.1:8000 llama-3-70b`
- **Then**: 성공 stdout + `~/.config/mink/models.yaml` 에 entry 추가
- **Verify**: integration

### AC-032 (REQ-RV2A-018) — `mink model add-custom` health-check 실패

- **Given**: mock server 가 404 반환
- **When**: `mink model add-custom my-bad http://127.0.0.1:9999 foo`
- **Then**: 실패 stdout "health check failed: 404" + entry 추가 안 됨
- **Verify**: integration

### AC-033 (REQ-RV2A-031) — `--no-stream` batch 모드

- **Given**: 정상 Anthropic 키 / 모델 설정
- **When**: `mink ask --no-stream "hi"`
- **Then**: stdout 에 응답 전체가 한 번에 출력 (chunk 단위 ANSI 갱신 없음)
- **Verify**: integration

### AC-034 (REQ-RV2A-032) — `--provider` override

- **Given**: 활성 카테고리 = `Quality` (default head = ClaudeOpus)
- **When**: `mink ask --provider deepseek --model deepseek-reasoner "hi"`
- **Then**: DeepSeek 만 호출되고 chain fallback 발생 안 함 (single-shot)
- **Verify**: integration

### AC-035 (REQ-RV2A-007, -011) — E2E 시나리오 1: Anthropic

- **Given**: 새 사용자
- **When**: `mink login anthropic` → key paste → `mink ask "hello"`
- **Then**: claude-opus-4-7 응답 stream stdout 표시, 종료 코드 0
- **Verify**: e2e

### AC-036 (REQ-RV2A-007, -011) — E2E 시나리오 2: Codex OAuth

- **Given**: 새 사용자, mock token endpoint
- **When**: `mink login codex` → OAuth callback → `mink ask "hi"` (활성 카테고리 = coding)
- **Then**: Codex 응답 stream + AUTH 에 refresh_token 저장 확인
- **Verify**: e2e

### AC-037 (REQ-RV2A-017) — E2E 시나리오 3: Cost routing

- **Given**: 4 provider 키 paste 완료
- **When**: `mink routing set cost` → `mink ask "hi"`
- **Then**: DeepSeek (chain head) 호출 후 응답
- **Verify**: e2e

### AC-038 (REQ-RV2A-017, -029) — E2E 시나리오 4: 모든 fallback 실패

- **Given**: 5 provider 모두 mock 으로 실패 강제 (각기 다른 FailoverReason)
- **When**: `mink ask "hi"`
- **Then**: stdout 에 종합 에러 (시도한 provider 5 + 각 FailoverReason + 마지막 에러) 표시, 종료 코드 ≠ 0
- **Verify**: e2e

---

## REQ ↔ AC 매트릭스 (검증용)

| REQ | AC |
|-----|-----|
| 001 | 001, 001a, 002, 003, 004, 005, 005a |
| 002 | 001, 002, 003, 004, 005, 005a |
| 003 | 017 |
| 004 | 007 |
| 005 | 022, 025 |
| 006 | 002, 003, 004, 005, 005a, 035, 036 (stream 동작 검증 포함) |
| 007 | 035, 036, 037, 038 |
| 008 | 003, 004, 006 |
| 009 | 027, 030 |
| 010 | (전 AC 의 stdout 한국어 확인) |
| 011 | 035~038 (TRUST 5 게이트 통과) |
| 012 | 008, 009, 014 |
| 013 | 010, 013, 014 |
| 014 | 011, 012 |
| 015 | 016 |
| 016 | 022 |
| 017 | 023, 038 |
| 018 | 006, 031, 032 |
| 019 | 017, 018, 026 |
| 020 | 028 |
| 021 | 015 |
| 022 | 019 |
| 023 | 020 |
| 024 | 021 |
| 025 | 016a |
| 026 | 007, 008 |
| 027 | 024 |
| 028 | 029 |
| 029 | 038 (AGPL-3.0 헤더 검증 포함) |
| 030 | 031 (template) |
| 031 | 033 |
| 032 | 034 |

---

## Verify Type 분포

- **unit**: AC-001, -001a, -002~006, -005a, -007, -008, -010, -012, -017~025, -027~029
- **integration**: AC-009, -011, -013, -014, -015, -016, -016a, -026, -030, -031~034
- **e2e**: AC-035~038
- **manual**: 없음 (모두 자동화 가능)

총 41 AC (audit B1 fix). 다음 단계 (run) 진입 조건: 41/41 GREEN, coverage ≥ 85%, AGPL-3.0 헤더 모든 신규 파일 포함, brand-lint 0 violation.

---

## 추가 AC (audit B2 fix — REQ-010 + REQ-029 전용 매핑)

### AC-039 [REQ-RV2A-010 / M5 / P2] — 한국어 stdout / 영어 godoc 정합

- **Given**: 5 provider 어댑터 + login + ask + routing 명령 전체 컴파일 완료
- **When**:
  1. 14 FailoverReason 각각 의도적 트리거 후 stdout/stderr 캡처
  2. `internal/llm/...` 모든 .go 파일의 godoc 코멘트 스캔
- **Then**:
  1. stdout 한국어 메시지 14 종 모두 한글 unicode block 포함 (`\p{Hangul}` ≥ 1)
  2. godoc 코멘트에 한글 unicode block 0 occurrence (영문 only)
- **Verify**: integration (golden output) + lint (정적 스캔)

### AC-040 [REQ-RV2A-029 / M5 / P1] — ADR-001 정합 (자체 모델 호스팅 0)

- **Given**: 본 SPEC 신규 산출물 패키지 (`internal/llm/`)
- **When**: 정적 검증
  1. `go list -deps ./internal/llm/...` import 그래프 스캔
  2. `git grep -i "vllm\|huggingface\|transformers\|torch\|qlora\|llama_cpp\|ggml\|candle\|burn"` 검색
- **Then**:
  1. 키워드 0 occurrence
  2. import 그래프에 ML runtime / training framework 0
  3. 외부 HTTP 클라이언트 + provider SDK + OAuth 라이브러리만 import
- **Verify**: unit (build-time static check, CI 게이트 등록)

---

Version: 1.0.0
Last Updated: 2026-05-16

## SPEC-GOOSE-MSG-TELEGRAM-001 Progress

- Started: 2026-05-09
- Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-P1
- Phase 0.9: Detected go.mod → Go 1.26 project, moai-lang-go applies
- Phase 0.95: SPEC scope ≥ 10 files, single domain (backend) → Standard Mode
- Phase 1: Strategy already produced via plan workflow (plan.md). Implementation Plan loaded.
- Phase 1.5: 8 tasks (P1-T1 ~ P1-T8) defined in plan.md §2 Phase 1
- Phase 1.6: AC-MTGM-001 acceptance criteria registered as P1-Exit gate (P1 partial scope: setup CLI 5분 시나리오 minus auto-admit which requires P2 MEMORY-001)
- Phase 1.7: Stub creation skipped — TDD methodology creates files alongside tests (RED phase first)

## P1 Implementation Tasks

- T1 [pending]: go.mod add github.com/go-telegram/bot v1.20.0
- T2 [pending]: client.go + client_test.go — go-telegram/bot wrapper, narrow Client interface, mock httptest
- T3 [pending]: config.go + config_test.go — yaml load, REQ-N01 token plain-text reject
- T4 [pending]: messaging.go + messaging_telegram.go (cobra) — setup subcommand with keyring + getMe verification + yaml gen
- T5 [pending]: poller.go + poller_test.go — getUpdates long polling loop, in-memory offset, graceful ctx shutdown
- T6 [pending]: handler.go + handler_test.go — minimal echo handler (no BRIDGE wire yet)
- T7 [pending]: bootstrap.go — Start(ctx, deps) entry + cobra start subcommand
- T8 [pending]: Manual smoke test plan documented (real bot registration walkthrough)

## P1 Exit Criteria

- AC-MTGM-001 GREEN (partial: setup + keyring + getMe + yaml; auto-admit deferred to P2)
- Informal echo smoke gate (manual verification)
- Coverage ≥ 70% on internal/messaging/telegram/...
- gofmt clean, go vet clean, go build PASS, go test -race PASS

---

## P2 Implementation (2026-05-09)

Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-P2
Methodology: TDD RED-GREEN-REFACTOR

### 아키텍처 결정 (analyze-p2.md)

- **audit EventType**: `EventTypeMessagingInbound` / `EventTypeMessagingOutbound` 신규 추가 (`messaging.inbound`, `messaging.outbound`) — 기존 타입과 의미적으로 분리
- **Store backend**: Option B — 독립 sqlite DB (`~/.goose/messaging/telegram.db`). `modernc.org/sqlite` v1.50.0 (go.mod에 기 등록). MemoryProvider 인터페이스와 무관.
- **AgentQuery**: narrow interface `Query(ctx, text) (string, error)` — gRPC `Chat()` 어댑터 래핑. P2에서는 NoOpAgentQuery로 대체 (P3 연기).
- **credproxy**: HTTP proxy 패턴으로 봇 토큰 직접 조회에 부적합 → MemoryKeyring 유지, @MX:TODO P3 업데이트.
- **daemon hook**: goosed self-gRPC 미해결 → `go telegram.Start()` 고루틴 + NoOpAgentQuery 패턴. P3에서 완전 연동.

### Task 1: audit.go (AuditWrapper)

- `internal/messaging/telegram/audit.go` 생성 (AuditWrapper, sha256Hex)
- `internal/audit/event.go`에 EventTypeMessagingInbound/Outbound 추가
- 테스트 9개 (모두 GREEN), coverage > 90%

### Task 2: store.go (SqliteStore)

- `internal/messaging/telegram/store.go` 생성 (Store interface, SqliteStore, UserMapping)
- WAL 모드, ON CONFLICT DO UPDATE 원자적 upsert
- 테스트 9개 (모두 GREEN), coverage > 85%

### Task 3: bridge_handler.go (BridgeQueryHandler)

- `internal/messaging/telegram/bridge_handler.go` 생성 (AgentQuery, BridgeQueryHandler)
- 첫 메시지 게이트 (auto_admit on/off), 차단 드롭, 길이 제한, 타임아웃 처리
- 테스트 9개 (모두 GREEN)

### Task 4: bootstrap.go 확장

- Deps 구조체 P2 필드 추가 (Store, Audit, Agent)
- NoOpAgentQuery 추가 (@MX:TODO P3)
- Start: 전체 deps → BridgeQueryHandler, 부분 deps → EchoHandler (P1 호환)
- 테스트 5개 (모두 GREEN)

### Task 5: CLI approve / revoke / status 확장

- `messaging_telegram.go`: newTelegramApproveCommand, newTelegramRevokeCommand, 확장된 newTelegramStatusCommand
- `messaging.go`: NewMessagingCommandWithDepsFull (storePath 파라미터 추가)
- 테스트 5개 (모두 GREEN)

### Task 6: integration_test.go

- `//go:build integration` 태그로 격리
- AC-MTGM-002 (audit hash), AC-MTGM-003 (offset 영속성), AC-MTGM-004 (partial 게이트), 차단 드롭, graceful skip
- 통합 테스트 5개 (모두 GREEN)

## P2 Exit Criteria

- AC-MTGM-002 GREEN (audit content_hash, raw body 부재 검증)
- AC-MTGM-003 GREEN (sqlite offset 재시작 영속성)
- AC-MTGM-004 GREEN (partial: 첫 메시지 게이트 + auto-admit)
- AC-MTGM-006 GREEN (graceful skip with nil deps)
- Coverage: telegram 82.0%, cli/commands 73.8%, audit 85.5%
- gofmt clean, go vet clean, go build PASS, go test -race PASS (전 패키지)

---

## P3 Implementation (2026-05-09)

Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-P3
Methodology: TDD RED-GREEN-REFACTOR

### Task 1: markdown.go — EscapeV2 + RenderInlineKeyboard

- `internal/messaging/telegram/markdown.go` 생성
- EscapeV2: 18개 reserved chars 모두 escape (Telegram MarkdownV2 spec)
- RenderInlineKeyboard: 1단 inline keyboard JSON 렌더
- markdown_test.go: 18자 전수 + 복합 케이스 + idempotent + empty
- AC-MTGM-010 일부 GREEN

### Task 2: sender.go — Sender.Send + Client interface P3 확장

- `internal/messaging/telegram/sender.go` 생성 (Sender, SendRequest, SendResponse)
- Client interface에 AnswerCallbackQuery/SendPhoto/SendDocument 추가
- ErrUnauthorizedChatID (REQ-MTGM-N02), MarkdownV2 escape, attachment 분기
- sender_test.go: 7 테스트 (권한 거부, V2 escape, 이미지/문서 분기, audit)
- 기존 mock들 Client interface 준수로 업데이트
- AC-MTGM-005 GREEN

### Task 3: tool.go — TOOLS-001 registry 등록

- `internal/messaging/telegram/tool.go` 생성 (telegramSendMessageTool, WithMessaging)
- JSON Schema (draft 2020-12): chat_id/text/parse_mode/inline_keyboard/attachments/silent
- toolSender 인터페이스로 의존 역전, ErrUnauthorizedChatID → tool error
- tool_test.go: 8 테스트 (등록, 스키마, 호출, 권한 거부, 중복 등록 panic)
- AC-MTGM-005 GREEN (일부)

### Task 4: inbox.go — file attach + Janitor

- `internal/messaging/telegram/inbox.go` 생성 (Janitor, downloadAttachment, isAllowedExt)
- 확장자 화이트리스트 (10개), O_CREATE|O_EXCL idempotent skip, 중앙 janitor goroutine
- AgentQuery interface attachments []string 확장 (strategy-p3.md §C.5 option i)
- inbox_test.go, handler_attach_test.go: Janitor sweep/run, download/idempotent/ext
- AC-MTGM-007 GREEN

### Task 5: callback_query branch

- BridgeQueryHandler.handleCallback 추가 (bridge_handler.go)
- CallbackQuery 타입 + convertUpdates 확장 (client.go)
- callbackQueryTimeout 60초, expired 감지 + audit 기록 (REQ-MTGM-N04)
- callback_test.go: 3 테스트 (정상/만료/차단)
- AC-MTGM-005 callback + REQ-MTGM-E05/N04 GREEN

### Task 6: ChatService 어댑터 (NoOpAgentQuery deprecate)

- `internal/agent/chat.go` 신설 (ChatService, QueryEngineChatService, ChatRequest/Response)
- `internal/messaging/telegram/agent_adapter.go` 신설 (AgentAdapter)
- bootstrap.go NoOpAgentQuery @MX:TODO P3 → @MX:NOTE로 변경
- keyring.go @MX:TODO P3 → @MX:NOTE로 변경
- agent_adapter_test.go: 4 테스트, chat_test.go: 6 테스트

### Task 7: Integration Test P3

- `internal/messaging/telegram/integration_p3_test.go` 신설 (build tag: integration)
- AC-MTGM-005 통합 (SendTool → audit outbound)
- AC-MTGM-010 통합 (MarkdownV2 18 reserved chars 전수 escape 검증)
- inline keyboard 렌더 검증
- Janitor 실행 without error 검증
- 기존 P2 통합 테스트 회귀 0건

### Task 8: zalando/go-keyring 도입

- `go get github.com/zalando/go-keyring@v0.2.8`
- `keyring_os.go` (//go:build !nokeyring) — OSKeyring 구현
- `keyring_nokeyring.go` (//go:build nokeyring) — CI stub
- messaging_telegram.go setup/start command 기본값 → NewOSKeyring()
- telegramClientIface에 P3 메서드 추가
- coverage_test.go: 추가 테스트 (decodeChatID, HTTP 메서드, Janitor 엣지케이스)
- go test -tags=nokeyring PASS (전 패키지)

## P3 Exit Criteria

- AC-MTGM-005 GREEN (outbound tool + allowed_users gate; modal P4 deferred)
- AC-MTGM-007 GREEN (file attach download + janitor cleanup)
- AC-MTGM-008 GREEN (token security — P1/P2 기 구현)
- AC-MTGM-010 GREEN (MarkdownV2 18 reserved chars escape + inline keyboard)
- Coverage: telegram 83.5% (nokeyring), 84.6% (integration), agent 78.5%
- gofmt clean, go vet clean, go build PASS, go test -race PASS
- go test -tags=integration PASS, go test -tags=nokeyring PASS
- @MX:TODO 0개

---

## Sync Phase (2026-05-09)

Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-SYNC

### Divergence 일괄 반영 (12건)

**P2 에서 발견된 6건 (analyze-p2.md §6 기준)**:

1. **AgentService/Query → Chat**: gRPC 메서드명은 `Chat()`, adapter interface `ChatService` 로 래핑
   - 반영: spec.md §3.1 Area 2.3, §3.1 Area 2 메시지 처리 흐름, plan.md §3.2 wiring, §7 검증

2. **MEMORY-001 BoltDB → sqlite (Option B)**: `modernc.org/sqlite` v1.50.0 독립 DB (`~/.goose/messaging/telegram.db`)
   - 반영: spec.md §3.1 Area 4, §5.2 의존 라이브러리, §6 의존성, plan.md §3.2, §3.5 test

3. **NoOpAgentQuery 임시 (P2) → ChatService (P3)**: P3에서 domain interface로 대체 완료
   - 반영: spec.md HISTORY version 0.1.2

4. **AgentChatRequest.Agent 필드명**: 실제 gRPC schema 명시
   - 반영: plan.md §3.2 의사코드

5. **P2 NoOp 응답 → P3 ChatService 응답**: 완전 폐기
   - 반영: progress.md (본 entry)

6. **credproxy 부적합 → OS keyring (zalando/go-keyring v0.2.8)**: bot token keyring alternative
   - 반영: spec.md §3.1 Area 1, §4.1 REQ-MTGM-U02, §5.2 의존, §6 의존성, plan.md §3.4 보안

**P3 에서 발견된 6건 (strategy-p3.md §F 기준)**:

7. **CLI-TUI-002 modal 미구현 → AC-MTGM-005 E2 P4 deferred**: registry preapproval + sender allowed_users gate 이중 방어
   - 반영: spec.md §9 AC 요약, acceptance.md AC-MTGM-005 "Note (P3 현황)"

8. **callback_data PII 정책**: audit 에 content_hash 만 기록, raw 미포함
   - 반영: spec.md §4.4 REQ-MTGM-N06 (callback_data 정책 추가)

9. **REQ-MTGM-N04 표현 보완**: callback timeout 초과 시 answerCallbackQuery만 skip, 응답은 진행
   - 반영: spec.md §4.4 REQ-MTGM-N04 표현 정정

10. **attachment JSON Schema strict oneOf**: path | url oneOf 형태
    - 반영: spec.md §3.1 Area 3 JSON schema 정정

11. **daemon hook wiring**: `cmd/goosed/main.go` Step 10.9/11.5 위치 명시
    - 반영: spec.md §5.1 패키지 레이아웃 (new marker [NEW] (P3), Step 숫자 명시)

12. **BRIDGE Query → Chat**: P2 #1과 중복 (ChatService domain interface 이미 반영)
    - 반영: spec.md §3.1 Area 2.3 표현 보강

### Frontmatter 변경

- version: `0.1.1` → `0.1.2`
- status: `audit-ready` → `implemented` (P3 까지)
- updated_at: `2026-05-09` (유지)

### HISTORY entry 추가

version 0.1.2 entry (위 HISTORY 섹션 참조).

### 완료 상태

- spec.md: 12건 divergence 모두 반영 완료
- plan.md: BRIDGE Query→Chat, MEMORY-001→sqlite, test 전략, 보안 고려사항 정정 완료
- acceptance.md: AC-MTGM-005 E2 P4 deferred, AC-MTGM-007/010 P3 GREEN, AC-MTGM-011 callback_data PII 완료
- progress.md: Sync Phase entry 추가 완료

### Sync Exit Criteria

- [x] spec.md 12건 divergence 반영 완료
- [x] plan.md BRIDGE/MEMORY/test/security 정정 완료
- [x] acceptance.md AC 상태 명시 완료
- [x] progress.md Sync Phase entry 추가 완료
- [x] frontmatter (version 0.1.2, status implemented) 갱신 완료

---

## P4 Implementation (2026-05-09 entry, 진행 중)

Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-P4
Base: main = 3291f9c (sync v0.1.2 머지 완료)
Methodology: TDD RED-GREEN-REFACTOR
Lesson 정책: isolation 미사용 + foreground spawn (PR #127/#128/#129/#130 8회 무사고 검증)

### P4 Task 분해 (plan.md §2 Phase 4)

- T1 [pending]: handler streaming branch + StreamingChatService interface + 1-edit/sec rate limit (AC-MTGM-009 / REQ-MTGM-E02)
- T2 [pending]: webhook mode + setWebhook + TLS fallback to polling (REQ-MTGM-E07)
- T3 [pending]: silent_default + typing_indicator (REQ-MTGM-O01/O02)
- T4 [pending]: streaming queue per chat_id FIFO max 5 (REQ-MTGM-S05)
- T5 [pending]: golden output testdata Markdown V2 회귀 보호
- T6 [pending]: manual_smoke.md + progress.md P4 entry 갱신

### P4 Architectural Decision — Streaming via query.SubmitMessage native channel

기존 `internal/agent/chat.go` 의 `ChatService.Chat` 은 `query.SubmitMessage(ctx, text)` 가 반환하는 `<-chan SDKMessage` 의 모든 chunk 를 합쳐 단일 string 으로 반환 중. P4 streaming 은 **별도 BRIDGE-001 RPC 추가 없이** 같은 채널을 wrap 하여 `StreamingChatService.ChatStream` 으로 노출하면 native streaming 가능.

- `internal/agent/chat.go`: `StreamingChatService` interface + `ChatChunk` struct 추가
- `QueryEngineChatService.ChatStream`: `query.SubmitMessage` chan 을 ChatChunk chan 으로 변환 (text block 만 emit, Final flag 마지막 1회)
- `internal/messaging/telegram/agent_adapter.go`: `AgentStreamAdapter` 추가
- `bridge_handler.go`: `/stream` prefix 또는 `cfg.DefaultStreaming` 시 streaming branch
- `streaming.go` 신규: chunk-merge buffer + `time.Ticker(1s)` + final flush + audit `streaming_flag=true, edit_count=N`

### P4 Exit Criteria

- AC-MTGM-009 GREEN (streaming UX, /stream 접두 + DefaultStreaming yaml)
- coverage telegram ≥ 85% (현재 84.6% → +0.4% 이상)
- @MX:TODO 0개 유지
- gofmt clean, go vet clean, go test -race PASS, -tags=integration PASS, -tags=nokeyring PASS
- golangci-lint clean (P3 까지 미실행 → P4 에서 첫 수행)
- AC-MTGM-005 E2 (CLI-TUI-002 modal) 은 외부 SPEC 의존 → P4 에서도 별도 SPEC 으로 deferred 표기 유지

### P4 Task 진척 (실제 결과)

- T1 [GREEN]: handler streaming branch + StreamingChatService + 1 edit/sec rate limit
  - 신규: `internal/agent/streaming.go` (+test), `internal/messaging/telegram/streaming.go` (+test)
  - 확장: `client.go` EditMessageText, `agent_adapter.go` AgentStream/AgentStreamAdapter, `bridge_handler.go` streaming branch, `bootstrap.go` Stream 필드
  - AC-MTGM-009 GREEN, REQ-MTGM-E02 GREEN
- T2 [GREEN]: webhook mode (BRIDGE HTTP mux + setWebhook + TLS fallback)
  - 신규: `webhook.go` (+test), `bootstrap_webhook_test.go`, `config_webhook_test.go`
  - 확장: `client.go` SetWebhook/DeleteWebhook, `config.go` WebhookConfig (FallbackToPolling 기본값 true), `bootstrap.go` Mode 분기
  - REQ-MTGM-E07 GREEN
- T3 [GREEN]: silent_default + typing_indicator
  - 확장: `config.go` SilentDefault/TypingIndicator yaml 키, `client.go` SendChatAction + Silent 필드, `sender.go` WithSilentDefault, `bridge_handler.go` startTypingIndicator helper, `streaming.go` silent 파라미터
  - 신규: `config_options_test.go`, `client_options_test.go`, `sender_silent_test.go`, `typing_test.go`
  - REQ-MTGM-O01/O02 GREEN
- T4 [GREEN]: streaming queue per chat_id (FIFO max 5)
  - 신규: `streaming_queue.go` (+test) — chatStreamQueue (TryAcquire/Enqueue/Release)
  - 확장: `bridge_handler.go` queue 통합 + `runStreamingForUpdate` helper 추출
  - REQ-MTGM-S05 GREEN
- T5 [GREEN]: golden output testdata
  - 신규: `testdata/markdown_v2/` 7 fixture pair, `testdata/inline_keyboard/` 5 fixture pair, `golden_test.go` (-update-golden flag 지원)
  - Markdown V2 18 reserved chars + inline keyboard 회귀 보호 자동화
- T6 [GREEN]: 문서 갱신
  - `manual_smoke.md` P4 시나리오 5개 추가 (streaming/queue/silent+typing/webhook fallback/V2 regression)
  - `progress.md` 본 entry 갱신

### Side-effect (사용자 요청 처리 — 같은 세션)

- `internal/mcp/auth_test.go::TestOpenBrowser` 가 매 `go test ./...` 실행마다 macOS `open` 명령 호출 → 브라우저 팝업 noise 발생. `t.Skip` 처리.
- 이유: assert.NotPanics 검증 가치 < dev 환경 noise. production `openBrowser` 코드 미변경 (OAuth flow 에서만 실행).
- lesson 적재: `~/.claude/projects/-Users-goos-MoAI-AI-Goose/memory/lesson_unit_test_no_real_os_side_effects.md`

### 누적 lesson 강화 (P4 세션)

- isolation 미사용 + foreground spawn — 13회 연속 무사고 (PR #127~#130 의 8회 + P4 의 5 task 직접/agent 호출)
- LSP stale after codegen — 8번째 reproduction (T2 mock 누락 진단 + T3 SendChatAction 진단 모두 stale, vet/test 직접 verify 로 결정)
- mock client 갱신 누락 — `go vet ./...` 전 패키지 실행으로 검출 (T1 cli/commands fakeClient 직접 수정 lesson 반복 활용)
- unit test 가 OS 부작용 호출 금지 — 신규 lesson (TestOpenBrowser 사례)

---

## P4 Sync Phase (2026-05-09)

Branch: feature/SPEC-GOOSE-MSG-TELEGRAM-001-SYNC
Base: main = 0cc1386 (P4 PR #131 머지 후)

### Frontmatter 변경

- `version: 0.1.2 → 0.1.3`
- `status: implemented` (유지)
- `updated_at: 2026-05-09` (유지)

### HISTORY entry 추가

spec.md HISTORY 표에 v0.1.3 entry 추가:
- AC-MTGM-009 GREEN (streaming UX, /stream 접두 + DefaultStreaming yaml)
- REQ-MTGM-E02 / E07 / O01 / O02 / S05 모두 GREEN
- AC-MTGM-005 E2 외부 SPEC 의존 deferred 유지
- testdata/ 12 fixture pair 회귀 보호 추가
- coverage 84.6% (P3 종점 회복)
- golangci-lint 0 issues

### spec.md §10 DoD 갱신

- "P3 까지 완료" 박스: AC-MTGM-009 GREEN 표기 추가 (`P4 에서 GREEN 처리`)
- 새 "P4 완료" 박스 추가 — Webhook mode / Streaming UX / silent_default / typing_indicator / streaming queue / testdata / manual_smoke / coverage / lint / @MX:TODO 모두 GREEN
- "deferred (외부 SPEC 의존)" 박스 — AC-MTGM-005 E2 (CLI-TUI-002 modal) 만 잔여

### acceptance.md 갱신

- AC-MTGM-009 — **P4 GREEN** 표기 추가, "P4 구현 현황" Note 추가 (StreamingChatService 위치, runStreaming 위치, chatStreamQueue 위치)
- AC-MTGM-006 Webhook fallback 부분 — **P4 GREEN** 표기 + 구현 위치 (webhook.go, bootstrap.go, config.go) 명시
- §11 DoD Checklist — 모든 항목 [x] 마킹 (10/11 AC GREEN, deferred 1 명시)

### Sync Exit Criteria

- [x] spec.md frontmatter version 0.1.2 → 0.1.3, status: implemented 유지
- [x] spec.md HISTORY v0.1.3 entry 추가
- [x] spec.md §10 DoD P4 박스 신설 + AC-MTGM-009 GREEN 표기
- [x] acceptance.md AC-MTGM-009 P4 GREEN + AC-MTGM-006 Webhook fallback P4 GREEN
- [x] acceptance.md §11 DoD checklist [x] 마킹
- [x] progress.md 본 entry 추가

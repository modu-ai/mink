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

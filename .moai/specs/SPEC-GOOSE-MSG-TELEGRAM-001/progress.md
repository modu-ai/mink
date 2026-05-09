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

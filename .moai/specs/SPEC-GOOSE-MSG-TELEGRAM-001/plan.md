---
id: SPEC-GOOSE-MSG-TELEGRAM-001
artifact: plan
version: "0.1.0"
created_at: 2026-05-05
updated_at: 2026-05-05
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — Implementation Plan

## 1. 전략적 위치 (Strategic Position)

본 SPEC 은 GOOSE 8주 로드맵 Phase 4 (channel rollout) 의 **첫 채널** 이며, 향후 모든 messaging 채널 (Kakao/Slack/Discord) 의 **reference 구현**이 된다. 따라서 patterns 가 후속 채널에 재사용 가능한 형태로 정제되어야 한다 — `internal/messaging/<channel>/` sibling 구조, `bootstrap.Start(ctx, deps)` entry pattern, audit/cred/memory/tool wiring sequence.

채택 우선순위:

- **P0** (본 SPEC priority) — GOOSE 사용자 향 1차 접점, 다른 모든 user-facing 기능의 전제. 매일 사용자가 만나는 채널.
- 사용자 마찰 0.05 (5분 setup) — KakaoTalk Channel (5분 + 사업자 인증), Slack (10분 + workspace install) 보다 낮음.

---

## 2. 우선순위 기반 마일스톤 (No Time Estimates — CLAUDE.local.md §2.5 준수)

CLAUDE.local.md 규칙 + agent-common-protocol "Time Estimation" HARD 규칙: 시간 추정 금지, **우선순위 + phase 순서**만 사용.

### Phase 1 (P-Highest) — Foundation: Setup CLI + Polling Skeleton + First Echo

**목표**: 사용자가 `goose messaging telegram setup` 으로 등록 + Telegram 에서 메시지 전송 시 **echo bot** 형태로 응답 (BRIDGE-001 query 미연결, 단순 echo). 첫 GREEN.

P1-T1: `internal/messaging/telegram/` 패키지 생성 + go.mod 에 `github.com/go-telegram/bot` 추가.
P1-T2: `client.go` — go-telegram/bot wrapper (interface 추출하여 mock 가능). `client_test.go` characterization 테스트 (mock httptest server).
P1-T3: `config.go` — yaml 로드 + REQ-MTGM-N01 검증 (token 평문 reject).
P1-T4: `cmd/goose/cmd/telegram.go` — cobra `setup` subcommand. token 입력 → CREDENTIAL-PROXY-001 keyring 저장 → `getMe` 검증 → yaml 생성. unit test (cobra-test fixture).
P1-T5: `poller.go` — `Poller` struct, `getUpdates` long polling loop, offset 메모리 보관 (영속은 P3). graceful shutdown via context.
P1-T6: `handler.go` 최소 — 들어온 메시지 본문을 그대로 echo 로 응답 (BRIDGE-001 미연결). `handler_test.go` (mock client).
P1-T7: `bootstrap.go` `Start(ctx, deps)` — daemon 외부 standalone 기동 가능. `cmd/goose/cmd/telegram.go` `start` subcommand.
P1-T8: 수동 검증 시나리오 — 실제 Telegram 봇 등록 후 `Hello` 전송 → echo 응답 수신.

**Phase 1 Exit Criteria**: AC-MTGM-001 (setup 5분 시나리오) GREEN + Phase 1 informal echo smoke gate (numbered AC 미할당, 수동 검증만 — BRIDGE-001 미연결 echo bot 형태로 inbound text 가 그대로 outbound 로 회신). 정식 round-trip AC (AC-MTGM-002) 와 streaming AC (AC-MTGM-009) 는 본 Phase 의 exit 와 무관하며 각각 Phase 2/Phase 4 에서 GREEN 한다. coverage ≥ 70%.

### Phase 2 (P-High) — BRIDGE 연동 + Audit + Mapping

**목표**: echo 를 BRIDGE-001 query 호출로 교체 + AUDIT-001 + MEMORY-001 chat_id mapping 통합.

P2-T1: `audit.go` — AUDIT-001 wrapper. inbound/outbound 모두 `direction`/`content_hash`/`user_profile_id` 기록.
P2-T2: `store.go` — MEMORY-001 wrapper. bucket `messaging.telegram.users` + `messaging.telegram.last_offset` 영속. offset 매 update 직후 즉시 write.
P2-T3: `handler.go` 갱신 — chat_id mapping 조회 → 미매핑 시 first-message flow (REQ-MTGM-S01) → BRIDGE-001 `AgentService/Query` 호출 → 응답 본문 sendMessage (Markdown V2 미적용, P3 에서).
P2-T4: `bootstrap.go` 갱신 — daemon bootstrap hook (`internal/daemon/bootstrap.go`) 추가. token 부재 시 graceful skip (REQ-MTGM-S02).
P2-T5: `cmd/goose/cmd/telegram.go` `approve|revoke|status` subcommands.
P2-T6: integration test — mock Telegram API + mock BRIDGE-001 + 실제 MEMORY-001 (테스트 BoltDB) → setup → 첫 메시지 → query → 응답 → audit 검증.

**Phase 2 Exit Criteria**: AC-MTGM-002 (round-trip), AC-MTGM-003 (chat_id mapping), AC-MTGM-004 (first-message gate), AC-MTGM-006 (daemon graceful skip when not configured). coverage ≥ 80%.

### Phase 3 (P-High) — Outbound Tool + Markdown V2 + File Attach

**목표**: GOOSE agent 가 `telegram_send_message` tool 호출 가능. Markdown V2 정상 렌더. 파일 attach 송수신.

P3-T1: `markdown.go` — V2 escape (18 reserved chars `_*[]()~\`>#+-=|{}.!`) + inline keyboard (1단) 렌더. `markdown_test.go` 18자 전수 + 복합 시나리오.
P3-T2: `sender.go` — outbound `Send(ctx, req)`. allowed_users 검증 (REQ-MTGM-N02), Markdown V2 escape, sendMessage/sendPhoto/sendDocument 분기.
P3-T3: `tool.go` (또는 `internal/tools/telegram_send.go`) — TOOLS-001 registry 등록. JSON schema 정의. permission gate 통과 후 `sender.Send` 호출.
P3-T4: `handler.go` 갱신 — `getFile` API 로 inbound attachment 다운로드 (`~/.goose/messaging/telegram/inbox/`). 30분 후 cleanup goroutine. attachment_paths 를 BRIDGE query 본문에 포함.
P3-T5: `handler.go` 갱신 — callback_query 분기 (`answerCallbackQuery` + callback_data → BRIDGE query).
P3-T6: integration test — agent 가 mock TOOLS-001 통해 `telegram_send_message` 호출 → 사용자에게 메시지 + audit 기록 → 사용자 inline keyboard 클릭 → callback 처리.

**Phase 3 Exit Criteria**: AC-MTGM-005 (outbound tool), AC-MTGM-007 (file attach round-trip), AC-MTGM-008 (token security), AC-MTGM-010 (Markdown V2 + inline keyboard). coverage ≥ 85%.

### Phase 4 (P-Medium) — Streaming + Webhook + Polish

**목표**: streaming UX (editMessageText), webhook mode (옵션), nice-to-have (typing indicator).

P4-T1: `handler.go` streaming branch — `/stream` 접두 또는 `default_streaming: true` 시 BRIDGE-001 streaming RPC + placeholder + chunk-merge buffer + final edit. rate limit 1 edit/sec 윈도우.
P4-T2: `webhook.go` — BRIDGE-001 HTTP mux 에 `/webhook/telegram/<secret>` 등록 + `setWebhook` 호출. fallback to polling on TLS 부재 (REQ-MTGM-E07).
P4-T3: REQ-MTGM-O01 / O02 — silent_default, typing_indicator (옵션 yaml 키).
P4-T4: streaming queue (REQ-MTGM-S05) — 같은 chat_id streaming 중 inbound 큐 (max 5). 가득 시 안내 메시지.
P4-T5: golden output 파일 (`testdata/`) — Markdown V2 렌더 결과 회귀 보호.
P4-T6: 수동 시나리오 검증 — 실제 봇 사용 시 streaming 자연스러운지, webhook 등록 후 ngrok TLS 환경에서 동작.

**Phase 4 Exit Criteria**: 모든 AC GREEN, coverage ≥ 85%, `@MX:TODO` 0개, golangci-lint clean.

---

## 3. 기술적 접근 (Technical Approach)

### 3.1 Layered Architecture

```
┌────────────────────────────────────────────────────────┐
│ cmd/goose/cmd/telegram.go (cobra: setup/start/...)    │  ← user CLI
├────────────────────────────────────────────────────────┤
│ internal/messaging/telegram/bootstrap.go               │  ← daemon entry
│   Start(ctx, Deps{Bridge, Tools, Memory, CredProxy,   │
│                    Audit, Logger})                     │
├────────────────────────────────────────────────────────┤
│  poller.go      handler.go     sender.go    webhook.go │  ← runtime layers
│  (long poll)    (inbound)      (outbound)   (옵션)     │
├────────────────────────────────────────────────────────┤
│  client.go (go-telegram/bot wrapper, mockable)         │  ← API client
├────────────────────────────────────────────────────────┤
│  audit.go     store.go     markdown.go     config.go   │  ← cross-cutting
│  (AUDIT-001)  (MEMORY-001) (V2 escape)     (yaml load) │
└────────────────────────────────────────────────────────┘
```

핵심 원칙:

- `bootstrap.Start(ctx, deps)` 는 모든 의존을 인터페이스로 받음 (testable, future channel 들이 같은 패턴 재사용).
- `client.go` 는 `Client` 인터페이스로 추출 → 모든 테스트는 mock client 로 격리.
- `handler.go` 는 inbound 분기만 (text / callback / file), 비즈니스 로직 (mapping, audit, query) 은 협력자 호출.

### 3.2 Dependency Wiring 순서 (bootstrap.Start)

```
Start(ctx, deps):
  1. config.Load() → yaml + keyring token
  2. client.New(token, deps.Logger) → Telegram API client
  3. store.New(deps.Memory) → mapping/offset wrapper
  4. audit.New(deps.Audit) → audit wrapper
  5. handler.New(client, store, audit, deps.Bridge) → inbound dispatcher
  6. sender.New(client, store, audit) → outbound sender
  7. tool.Register(deps.Tools, sender) → TOOLS-001 등록
  8. mode == polling: poller.Run(ctx, client, handler, store) (goroutine)
     mode == webhook: webhook.Register(deps.HTTPMux, handler, secret_path)
  9. block until ctx.Done()
```

### 3.3 Streaming Edit Buffer

editMessageText 1초 윈도우 rate limit:

- chunk 누적 buffer (size 64KB cap)
- 1초 timer (time.Ticker)
- ticker tick 시 buffer 비어있지 않으면 editMessageText (현재 누적 본문)
- streaming 종료 시 final flush + audit `streaming_completed`

### 3.4 Error Recovery

- network error (Telegram API): exponential backoff 2→4→8→cap 30초.
- BRIDGE-001 query timeout (default 30초): 사용자에게 "처리 시간 초과, 다시 시도" 응답.
- AUDIT-001 write fail: warning log + 메시지 처리는 계속 (audit 실패가 사용자 채널을 막지 않음).
- MEMORY-001 read fail: 미매핑 처리 fallback (REQ-MTGM-S01).
- TOOLS-001 permission denied: tool error `permission_denied` 반환, 사용자에게 미전송.

### 3.5 Test 전략

- **unit**: client_test.go (mock httptest), markdown_test.go (16 special chars), config_test.go (yaml + token reject).
- **handler integration**: mock client + mock BRIDGE + 실 MEMORY (테스트 BoltDB 임시 디렉토리).
- **bootstrap E2E**: full Start(ctx, deps) 시나리오, mock 모든 deps 통과.
- **manual smoke**: 실제 Telegram bot 등록 후 setup→send→receive 5분 walkthrough — phase 별 1회.

---

## 4. 보안 고려 (Security Considerations)

| 항목 | 위협 | 통제 |
|-----|------|-----|
| bot token 누출 | yaml 평문 저장 | REQ-MTGM-N01 (setup reject) + CREDENTIAL-PROXY-001 keyring 강제 |
| unauthorized chat_id outbound | agent 가 임의 chat 으로 메시지 | REQ-MTGM-N02 (allowed_users 검증) + TOOLS-001 permission gate |
| webhook URL 탈취 | secret_path 약함 | 32자 hex 무작위 + TLS 필수 |
| PII leak via audit log | 다른 user 정보 outbound 본문 | REQ-MTGM-N06 + audit hash 만 기록 |
| message replay | 같은 update 재전송 | offset 영속 (REQ-MTGM-U05) |
| inbound malicious link | phishing | Telegram 측 spam 필터 + GOOSE 측 별도 검증 없음 (사용자 신뢰 모델) |

---

## 5. Rollout 계획

- **Stage 1 — Internal dogfood**: 본 개발자만 (allowed_users = [own chat_id]). 1주.
- **Stage 2 — Closed beta**: 5명 사용자, auto_admit_first_user=false, 명시적 approve 만. 1주.
- **Stage 3 — Open**: yaml 기본값 변경 없음, 사용자가 setup CLI 로 admin 수동 결정.

향후 채널 (Slack/Discord) SPEC 작성 시 본 SPEC 의 bootstrap 패턴 + audit/mem/cred wiring sequence 를 그대로 차용.

---

## 6. 후속 별도 SPEC 후보 (Out-of-Scope but Tracked)

- `SPEC-GOOSE-MSG-TELEGRAM-002` — Group chat ingress.
- `SPEC-GOOSE-MSG-TELEGRAM-003` — Inline mode (`@bot search`).
- `SPEC-GOOSE-MSG-TELEGRAM-004` — Telegram Web App / Mini App.
- `SPEC-GOOSE-MSG-KAKAO-001` — KakaoTalk 채널 (본 SPEC 패턴 재사용).
- `SPEC-GOOSE-MSG-SLACK-001` — Slack 채널.
- `SPEC-GOOSE-MSG-DISCORD-001` — Discord 채널.

---

## 7. 검증 체크포인트 (Pre-Run Gate)

- [ ] BRIDGE-001 의 `AgentService/Query` 가 outbound caller 친화 시그니처인지 (channel agnostic 입력) — research.md §2.1 검증.
- [ ] TOOLS-001 의 register API 가 in-process 등록 (network roundtrip 없음) 인지 — research.md §2.2.
- [ ] MEMORY-001 의 BoltDB provider 가 process 시작 시 default initialize 되는지 — research.md §2.3.
- [ ] CREDENTIAL-PROXY-001 의 keyring API 가 cross-platform (macOS Keychain / Windows Credential Manager / Linux Secret Service) 동작 검증.
- [ ] AUDIT-001 의 Append 가 sync write 인지 (장애 시 손실 없는지).

위 5개 모두 PASS 시 P1 Phase 진입.

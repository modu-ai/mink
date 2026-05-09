---
id: SPEC-GOOSE-MSG-TELEGRAM-001
version: "0.1.3"
status: implemented
created_at: 2026-05-05
updated_at: 2026-05-09
author: manager-spec
priority: P0
phase: 4
size: M
lifecycle: spec-anchored
labels: [messaging, telegram, ingress, tool, bridge, p0, channel-1st]
issue_number: 125
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — Telegram Bot Ingress + Send Tool (BRIDGE-001 연동)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — GOOSE 8주 로드맵 Phase 4 (channel rollout) 첫 채널. Telegram Bot API 6.x 기반 1:1 ingress + outbound `telegram_send_message` tool. BRIDGE-001 (Daemon ↔ UI) 위에 wiring, TOOLS-001 registry 에 등록, MEMORY-001 으로 chat_id ↔ user_profile mapping, CREDENTIAL-PROXY-001 로 bot token keyring 보관, AUDIT-001 으로 모든 메시지 감사 로그 append. 사용자 마찰 가장 낮은 (5분 셋업) 첫 1차 접점 채널. | manager-spec |
| 0.1.0 | 2026-05-06 | plan-auditor iter-1 CONDITIONAL_GO (5 defects D1~D5: AC count 10→11, plan.md L39 AC-MTGM-009 mis-binding, spec-compact.md §4 헤더, spec-compact.md §10 "10 AC", Markdown V2 reserved char count 16→18) 보강. 모든 결함 수정 후 plan-auditor iter-2 PASS (overall 0.91, 0 defects). status `draft` → `audit-ready` 전환. | manager-spec |
| 0.1.1 | 2026-05-09 | plan workflow Phase 2.5 — GitHub Issue #125 생성 후 frontmatter `issue_number: 125` 동기화. SPEC 본문 변경 없음, Issue ↔ SPEC 양방향 링크 확립 (영문 Issue body, run.md Phase 3 에서 `Fixes #125` 사용). | MoAI |
| 0.1.2 | 2026-05-09 | sync phase — P1/P2/P3 구현 완료 후 SPEC divergence 12건 일괄 반영. 주요: BRIDGE-001 Query → Chat (ChatService 도메인 인터페이스), MEMORY-001 → 독립 sqlite (Option B, modernc.org/sqlite v1.50.0), CREDENTIAL-PROXY-001 → OS keyring (zalando/go-keyring v0.2.8), AC-MTGM-005 E2 (CLI-TUI-002 modal) P4 deferred, REQ-MTGM-N04 표현 보완, attachment JSON Schema strict mode oneOf 정정. status: audit-ready → implemented (P3 까지). | manager-docs |
| 0.1.3 | 2026-05-09 | P4 (Streaming + Webhook + Polish) PR #131 머지 후 sync. AC-MTGM-009 GREEN 추가 — `/stream` 접두 + `default_streaming: true` 시 BRIDGE 측 `query.SubmitMessage` native channel 을 `agent.StreamingChatService.ChatStream` 으로 wrap, telegram 측 `runStreaming` 가 chunk-merge buffer + 1초 ticker 로 `editMessageText` rate-limit 호출. REQ-MTGM-E02 (streaming) / E07 (webhook fallback) / O01 (silent_default) / O02 (typing_indicator) / S05 (per-chat_id FIFO max 5) 모두 GREEN. AC-MTGM-005 E2 (CLI-TUI-002 modal) 만 외부 SPEC 의존으로 deferred 유지. testdata/ 12 fixture pair (markdown_v2 7 + inline_keyboard 5) 회귀 보호 추가. coverage telegram 84.6% (P3 종점 회복; ticker 5초 wait path 단위 테스트 한계로 strict 85% 미달). golangci-lint 0 issues (errcheck inbox.go fix + unused helper 제거 포함). status: implemented 유지. | manager-docs |

---

## 1. 개요 (Overview)

본 SPEC은 GOOSE 데몬과 **Telegram Bot API 6.x** 를 연결하여, 사용자가 Telegram 모바일/데스크톱 앱에서 GOOSE 에이전트와 **1:1 대화**를 나누고, GOOSE 가 사용자에게 **proactive 메시지** (예: 아침 brief, 작업 완료 알림) 를 보낼 수 있게 한다. 전략적으로 "사용자 향 첫 채널" 위치 — 마찰 가장 낮음 (Bot 등록 5분), 매일 사용자가 GOOSE 와 만나는 1차 접점.

본 SPEC 수락 시점에서:

- 사용자가 `goose messaging telegram setup` 1회 실행으로 Bot 등록 + token keyring 저장 + 첫 chat_id 매핑 완료 (5분 이내).
- Telegram 에서 사용자가 `/start` 또는 일반 메시지 입력 → GOOSE 데몬으로 전달 → goose query 실행 → 응답 Telegram 전송 (왕복 < 5초, streaming 옵션 시 첫 chunk < 1.5초).
- GOOSE 가 `telegram_send_message` tool 을 호출해 사용자에게 proactive 메시지 전송 (TOOLS-001 registry 경유).
- 모든 inbound/outbound 메시지가 AUDIT-001 audit log 에 append-only 로 기록.
- Bot token 은 CREDENTIAL-PROXY-001 keyring 으로 안전 보관 (평문 저장 금지).
- chat_id ↔ user_profile mapping 은 MEMORY-001 (BoltDB / sqlite provider) 에 저장.
- Long polling 이 default ingress 모드 (outbound NAT 친화), webhook 은 옵션 (BRIDGE-001 의 HTTP 서버 활용).
- Markdown V2 렌더링 + inline keyboard (1단 옵션 선택) + 파일 첨부 (image, document) 지원.

본 SPEC 의 모든 외부 채널 와이어링 패턴 (ingress poller → BRIDGE → query → outbound tool → audit) 은 향후 추가될 채널 (KakaoTalk, Slack, Discord) 의 reference 구현이 된다.

## 2. 배경 (Background)

### 2.1 GOOSE 8주 로드맵에서의 위치

`.moai/project/ROADMAP.md` 기준 Phase 4 (channel rollout) 의 첫 채널. 선행 SPEC 의존:

- **BRIDGE-001** (Daemon ↔ UI Bridge): unary RPC (`AgentService/Query`) + streaming SSE — Telegram ingress 가 invoking 하는 query path.
- **TOOLS-001** (Tool Registry): `telegram_send_message` 가 등록되는 registry — agent 가 outbound 호출.
- **MEMORY-001** (Memory Provider): chat_id ↔ user_profile mapping 의 영속 store.
- **CREDENTIAL-PROXY-001** (또는 CREDPOOL-001): bot token keyring 보관.
- **AUDIT-001**: 모든 messaging 이벤트 append-only log.

본 SPEC 이 완성되면 사용자는 모바일 폰만으로 GOOSE 와 매일 대화 가능 → "engagement loop" 의 시작점.

### 2.2 기존 messaging 코드베이스 부재 (Greenfield)

`internal/messaging/` 패키지는 본 SPEC 이 신설하는 첫 패키지. 향후 채널 (Kakao/Slack/Discord) 들도 같은 디렉토리 하위에 sibling 으로 배치 예정 → `internal/messaging/{telegram, kakao, slack}/`.

기존 코드베이스 영향:

- **[NEW]**: `internal/messaging/telegram/` 신규 패키지 (poller, handler, sender, store, setup CLI).
- **[NEW]**: `cmd/goose/cmd/messaging.go` 또는 `cmd/goose/cmd/telegram.go` cobra subcommand (`goose messaging telegram setup|start|status`).
- **[NEW]**: TOOLS-001 registry 에 `telegram_send_message` tool 등록 (`internal/tools/telegram_send.go` 또는 messaging 패키지 내).
- **[NEW]**: `~/.goose/messaging/telegram.yaml` 설정 파일 (bot username, allowed_users, default_chat_id, polling/webhook mode).
- **[MODIFY]**: `goosed` daemon 시작 시 messaging poller 도 부팅 (`internal/daemon/bootstrap.go` 또는 동등).

기존 BRIDGE-001 / TOOLS-001 / MEMORY-001 / AUDIT-001 / CREDENTIAL-PROXY-001 의 **공개 API 만 사용**, 내부 구현 변경 없음.

### 2.3 Telegram Bot API 라이브러리 평가

`research.md` §1 참조. 결론:

- **선택 후보 1 — `github.com/go-telegram/bot`** (활발한 유지보수, generics 친화 API, MIT, 의존 가벼움) — 1차 채택.
- **후보 2 — `github.com/go-telegram-bot-api/telegram-bot-api/v5`** — 검증된 베테랑, 그러나 maintenance pace 떨어짐.
- **후보 3 — 직접 구현** — `net/http` + `encoding/json` + `getUpdates` 풀러 — 의존 0개지만 maintenance burden 큼.

`research.md` §1.3 의 평가 매트릭스에서 후보 1 채택 확정.

### 2.4 범위 경계 (한 줄)

- **IN**: 1:1 chat (private chat type), inbound long polling (default) + webhook (옵션, BRIDGE-001 HTTP 서버 활용), outbound `telegram_send_message` tool (TOOLS-001), Markdown V2, inline keyboard (1단), file attach (image/document, ≤ 50MB), chat_id ↔ user_profile mapping (MEMORY-001), bot token keyring (CREDENTIAL-PROXY-001), audit log (AUDIT-001), `goose messaging telegram setup|start|status` CLI.
- **OUT**: Group chat (private only), Channel/Supergroup, Voice/Video call, Telegram Web App / Mini App, advanced inline mode (`@bot search...`), bot payments, Telegram Passport, Sticker pack management, custom emoji upload, message editing/deletion API (수신 측 edit/delete event 만 audit), polling parallelism > 1 worker, multi-bot 동시 운영.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE — 5 implementation areas

#### Area 1 — Bot 등록 + Setup CLI

1. **신규 cobra subcommand**: `goose messaging telegram setup`
   - 대화형 입력: bot token (또는 환경변수 `GOOSE_TELEGRAM_BOT_TOKEN` 우선), bot username 자동 fetch (`GET /getMe`).
   - 검증: token 유효성 (`getMe` 200 응답 + bot field), bot username 출력.
   - keyring 저장: OS keyring (zalando/go-keyring v0.2.8 — macOS Keychain / Linux Secret Service / Windows Credential Manager, 또는 환경변수 fallback).
   - 설정 파일 생성: `~/.goose/messaging/telegram.yaml` (bot_username, polling/webhook mode default=polling, allowed_users 빈 리스트, audit_enabled=true).
   - chat_id 매핑 안내: "이제 Telegram 에서 @<bot_username> 에게 `/start` 를 보내세요" 출력 후 첫 update 수신 대기 (선택, 30초 timeout).
2. **신규 cobra subcommand**: `goose messaging telegram status`
   - bot 등록 여부 (token 존재) / 매핑된 chat_id 수 / 마지막 inbound 시각 / poller running 여부 출력.
3. **신규 cobra subcommand**: `goose messaging telegram start` (foreground debug 모드)
   - daemon 외부에서 standalone 실행 (디버깅용). production 은 daemon bootstrap 이 자동 기동.
4. **token 부재 시 동작**: `setup` 미실행 상태에서 `start` / daemon bootstrap 시 → "Run `goose messaging telegram setup` first" 안내 후 messaging 모듈 비활성 (daemon 자체는 정상 기동).

#### Area 2 — Long Polling Ingress (Default)

1. **신규 패키지** `internal/messaging/telegram/`:
   - `poller.go` — `Poller` struct, `getUpdates` long polling loop, offset 관리, graceful shutdown (context cancel).
   - `handler.go` — inbound update 분기 (text message / callback_query / file attachment).
   - `client.go` — Telegram Bot API HTTP client wrapper (go-telegram/bot 위 thin layer).
   - `config.go` — `~/.goose/messaging/telegram.yaml` 로드 / 검증.
   - `bootstrap.go` — daemon 시작 시 호출되는 entry point (`Start(ctx, deps) error`).
2. **Polling 파라미터**:
   - `getUpdates` long poll timeout = 30초 (Telegram 권장).
   - 재시도: 네트워크 에러 시 exponential backoff (2초 → 4초 → 8초 → cap 30초).
   - offset 영속: MEMORY-001 의 `messaging.telegram.last_offset` key (재시작 시 중복 수신 방지).
3. **메시지 처리 흐름**:
   - 1) inbound update 수신 → 2) chat_id 추출 → 3) MEMORY-001 에서 user_profile mapping 조회 → 4) 미매핑 시 first-message registration flow (Area 4) → 5) AUDIT-001 audit log append → 6) BRIDGE-001 의 `ChatService` (도메인 인터페이스, gRPC `AgentService/Chat` 위에 어댑팅) 호출 → 7) 응답 수신 → 8) Markdown V2 렌더링 → 9) `sendMessage` 호출 → 10) outbound audit log append.
4. **streaming 옵션**:
   - 사용자가 메시지 본문에 `/stream` 접두 또는 yaml `default_streaming: true` 설정 시 BRIDGE-001 streaming RPC 사용.
   - 첫 chunk 수신 시 placeholder 메시지 `sendMessage` → 후속 chunk 마다 `editMessageText` (rate limit: 1초당 최대 1회 edit, Telegram API 제약).
   - streaming 종료 시 최종 본문으로 마지막 edit, audit log 에 streaming flag 기록.

#### Area 3 — Outbound Tool (`telegram_send_message`)

1. **신규 tool** in TOOLS-001 registry: `telegram_send_message`
   - 입력 schema (json, strict mode):
     ```json
     {
       "chat_id": "string (required) | user_profile.id alias",
       "text": "string (required, ≤ 4096 chars Markdown V2)",
       "parse_mode": "MarkdownV2 | HTML | Plain (default: MarkdownV2)",
       "reply_to_message_id": "int (optional)",
       "inline_keyboard": "[[{text, callback_data}]] (optional, 1단)",
       "attachments": "[{type: image|document, oneOf: [{path: string}, {url: string}]}] (optional, ≤ 50MB total)",
       "silent": "bool (optional, default: false — disable_notification)"
     }
     ```
   - 출력: `{message_id, chat_id, sent_at, audit_id}` 또는 error.
2. **권한 모델**:
   - chat_id 가 yaml `allowed_users` 리스트에 있어야만 전송 (보호 장치).
   - Tool call 자체는 TOOLS-001 의 permission gate (CLI-TUI-002 Area 2 modal) 통과 필수.
3. **렌더링**:
   - Markdown V2 escape: 18개 reserved char `_ * [ ] ( ) ~ \` > # + - = | { } . !` 모두 `\` escape (Telegram 문서 §5 — MarkdownV2 style).
   - inline keyboard: 1단만 지원 (`[[btn1, btn2, btn3]]` 단일 row), 다단 row 는 OUT.
   - attachment: `sendPhoto` / `sendDocument` 분기, multipart upload 또는 URL 패스.

#### Area 4 — chat_id ↔ user_profile Mapping (First-Message Registration)

1. **독립 sqlite DB** (`~/.goose/messaging/telegram.db`, `modernc.org/sqlite` v1.50.0 Option B):
   - table `telegram_users`
   - columns: `chat_id (primary), user_profile_id, telegram_username, first_seen_at, last_seen_at, allowed (bool)`
   - table `telegram_offset`
   - columns: `key (primary), value (int)` — `messaging.telegram.last_offset` 저장
2. **First-message flow** (chat_id 미매핑 시):
   - allowed_users 가 비어있고 yaml `auto_admit_first_user: true` 인 경우 → 첫 사용자 자동 등록 (admin 으로 마킹).
   - 그 외 경우 → "이 봇은 사전 승인된 사용자만 사용할 수 있습니다. 관리자에게 chat_id `<id>` 를 전달하세요" 응답 + 매핑 미생성.
3. **승인 CLI**: `goose messaging telegram approve <chat_id> [--user-profile <id>]` — yaml `allowed_users` + MEMORY-001 mapping 동시 갱신.
4. **revoke CLI**: `goose messaging telegram revoke <chat_id>` — `allowed_users` 제거 + mapping 의 `allowed=false` 마킹 (이력 보존).

#### Area 5 — Webhook Mode (옵션)

1. **활성 조건**: yaml `mode: webhook` + BRIDGE-001 HTTP 서버 가용.
2. **endpoint 등록**: BRIDGE-001 의 HTTP mux 에 `/webhook/telegram/<secret_path>` 추가 — secret_path 는 setup 시 무작위 생성 (32자 hex).
3. **Telegram 측 등록**: `setWebhook` API 호출, TLS 필수 (`https://...`), 그러므로 BRIDGE-001 이 HTTPS 모드여야 함 (Local-only 개발 시에는 polling 권장).
4. **fallback**: webhook 등록 실패 시 자동으로 polling 으로 fallback + warning 로그.

### 3.2 OUT OF SCOPE — Exclusions (What NOT to Build)

- **[OUT-1]** Group chat / Channel / Supergroup ingress — private chat 만. 향후 별도 SPEC.
- **[OUT-2]** Voice/Video call — Telegram Bot API 비지원 (Bot 은 voice send 만 가능, voice 수신은 voice message → 본 SPEC 은 voice 무시).
- **[OUT-3]** Telegram Web App / Mini App — 별도 frontend SPEC 필요.
- **[OUT-4]** Inline mode (`@bot query...`) — search-as-you-type 패턴, 별도 SPEC.
- **[OUT-5]** Telegram Payments (Stripe/Telegram payment 연동) — 별도 SPEC.
- **[OUT-6]** Telegram Passport (사용자 신원 확인) — 별도 SPEC.
- **[OUT-7]** Sticker pack management / custom emoji upload — 별도 SPEC.
- **[OUT-8]** Multi-bot 동시 운영 (한 daemon 에서 여러 bot token) — Phase 5+ 고려.
- **[OUT-9]** Polling parallelism (worker > 1) — 단일 poller 만 (Telegram offset 모델은 단일 consumer 가정).
- **[OUT-10]** message edit/delete API 발신 — 인바운드 edit/delete event 는 audit 에 기록하되, GOOSE 가 자기 메시지를 수정/삭제하는 outbound tool 은 본 SPEC 범위 밖 (단, streaming edit 은 예외 — Area 2).
- **[OUT-11]** end-to-end encryption — Telegram Bot API 는 서버-사이드 암호화만 (E2E 는 secret chat 전용, Bot 비지원).
- **[OUT-12]** rate limit 자체 구현 (Telegram 30 msg/sec/bot 제약은 라이브러리 측 처리에 위임).

---

## 4. 요구사항 (Requirements — EARS Format)

### 4.1 Ubiquitous Requirements (시스템 상시 보장)

- **REQ-MTGM-U01** [Ubiquitous] [HARD]: 시스템은 모든 inbound/outbound Telegram 메시지를 AUDIT-001 audit log 에 append-only 기록한다. 기록 항목: `{direction (in|out), chat_id, message_id, user_profile_id, ts, content_hash, streaming_flag, tool_call_id (out 만)}`.
- **REQ-MTGM-U02** [Ubiquitous] [HARD]: 시스템은 bot token 을 평문으로 디스크에 저장하지 않는다. OS keyring (zalando/go-keyring v0.2.8 — macOS Keychain / Linux Secret Service / Windows Credential Manager) 만 허용 (또는 환경변수 `GOOSE_TELEGRAM_BOT_TOKEN` 휘발성). yaml 파일에 token 평문 기록 시 setup 거부.
- **REQ-MTGM-U03** [Ubiquitous]: 시스템은 chat_id ↔ user_profile mapping 을 MEMORY-001 의 `messaging.telegram.users` bucket 에 영속화한다.
- **REQ-MTGM-U04** [Ubiquitous]: 모든 outbound Tool 호출 (`telegram_send_message`) 은 TOOLS-001 registry 의 permission gate 를 통과해야 한다.
- **REQ-MTGM-U05** [Ubiquitous]: getUpdates polling offset 은 MEMORY-001 의 `messaging.telegram.last_offset` 에 영속, daemon 재시작 시 마지막 처리 지점부터 재개한다 (중복 수신 0).

### 4.2 Event-Driven Requirements (트리거-응답)

- **REQ-MTGM-E01** [Event-Driven]: WHEN 사용자가 Telegram 에서 `/start` 또는 일반 텍스트 메시지를 전송 THEN 시스템은 chat_id 매핑 조회 → BRIDGE-001 `AgentService/Query` 호출 → 응답을 Markdown V2 로 렌더링 → `sendMessage` 호출, 왕복 P95 < 5초 (네트워크 RTT 제외).
- **REQ-MTGM-E02** [Event-Driven]: WHEN 사용자가 메시지 본문에 `/stream` 접두 사용 또는 yaml `default_streaming: true` THEN 시스템은 BRIDGE-001 streaming RPC 사용, placeholder `sendMessage` 후 chunk 마다 `editMessageText` (rate limit 1초/편집), 첫 chunk 표시 < 1.5초.
- **REQ-MTGM-E03** [Event-Driven]: WHEN GOOSE agent 가 `telegram_send_message` tool 을 호출 THEN 시스템은 TOOLS-001 permission gate 통과 후 chat_id 가 `allowed_users` 에 있는지 검증 → Markdown V2 escape → `sendMessage`/`sendPhoto`/`sendDocument` 분기 호출 → audit log 기록.
- **REQ-MTGM-E04** [Event-Driven]: WHEN inbound message 길이가 4096자 (Telegram 제약) 초과 THEN 시스템은 `RESP-MTGM-E04: 메시지가 너무 깁니다 (max 4096 chars)` 응답 + audit `length_exceeded` flag 기록 + query 미실행.
- **REQ-MTGM-E05** [Event-Driven]: WHEN inbound update 가 callback_query (inline keyboard 클릭) THEN 시스템은 `answerCallbackQuery` 호출 (toast 표시) + callback_data 를 query 본문으로 변환하여 BRIDGE-001 호출.
- **REQ-MTGM-E06** [Event-Driven]: WHEN file attachment (image/document) 수신 THEN 시스템은 `getFile` API 로 file_path 조회 → 임시 디렉토리 다운로드 (`~/.goose/messaging/telegram/inbox/<message_id>.<ext>`) → BRIDGE-001 query 본문에 file path 포함 (text + attachment_paths) → 응답 후 30분 후 임시 파일 삭제.
- **REQ-MTGM-E07** [Event-Driven]: WHEN webhook mode 등록 실패 (TLS 부재 등) THEN 시스템은 자동으로 polling mode 로 fallback + warning 로그 + status 출력 시 `mode: polling (fallback)` 표시.

### 4.3 State-Driven Requirements (조건부 동작)

- **REQ-MTGM-S01** [State-Driven]: WHILE chat_id 가 매핑되어 있지 않고 yaml `auto_admit_first_user: false` THEN inbound 메시지에 대해 "사전 승인 필요, 관리자에게 chat_id `<id>` 전달" 응답 + query 미실행.
- **REQ-MTGM-S02** [State-Driven]: WHILE bot token 미설정 (keyring 부재) THEN daemon bootstrap 의 messaging 모듈은 startup skip + log warning + `goose messaging telegram status` 가 `not configured` 반환. daemon 자체는 정상 기동.
- **REQ-MTGM-S03** [State-Driven]: WHILE poller 가 backoff 단계 (네트워크 에러 후 재시도 중) THEN 시스템은 backoff 시각 + 다음 재시도 시각을 metrics 에 노출 (CLI `status` 명령으로 조회).
- **REQ-MTGM-S04** [State-Driven]: WHILE allowed_users 비어 있고 yaml `auto_admit_first_user: true` THEN 첫 inbound 사용자를 자동으로 admin 으로 등록 + audit 에 `auto_admitted: true` 기록.
- **REQ-MTGM-S05** [State-Driven]: WHILE streaming RPC 진행 중 THEN 다음 inbound 메시지 (같은 chat_id) 는 큐에 적재 (FIFO, 최대 큐 깊이 5), streaming 완료 후 순차 처리. 큐 가득 시 "이전 응답 진행 중, 잠시 후 다시 시도하세요" 응답.

### 4.4 Unwanted Behavior Requirements (금지)

- **REQ-MTGM-N01** [Unwanted] [HARD]: 시스템은 **bot token 을 yaml 또는 평문 파일에 저장하지 않는다**. setup 시 token 입력이 yaml 에 직접 기재된 것을 감지하면 reject + "환경변수 또는 keyring 만 허용" 안내.
- **REQ-MTGM-N02** [Unwanted] [HARD]: 시스템은 **`allowed_users` 에 없는 chat_id 로 outbound 메시지를 전송하지 않는다**. `telegram_send_message` 호출 시 검증 실패 → tool error `unauthorized_chat_id` 반환.
- **REQ-MTGM-N03** [Unwanted]: 시스템은 **inbound 메시지 본문 4096자 초과 시 query 를 실행하지 않는다** (REQ-MTGM-E04 와 쌍).
- **REQ-MTGM-N04** [Unwanted]: 시스템은 **callback_query timeout (Telegram 60초 제약) 초과한 callback 응답을 처리하지 않는다** — `answerCallbackQuery` 만 skip 하고 응답 메시지는 정상 진행 (사용자 UX 보존), audit `callback_expired` 기록.
- **REQ-MTGM-N05** [Unwanted]: 시스템은 **blacklist 처리된 chat_id (yaml `blocked_users` 또는 mapping `allowed=false`) 의 메시지에 응답하지 않는다** + audit `silently_dropped: blocked` 기록 (사용자에게 차단 사실 통지하지 않음).
- **REQ-MTGM-N06** [Unwanted] [HARD]: 시스템은 **outbound 메시지에 chat_id 외 다른 사용자의 user_profile 정보 (이름, 메시지 본문 등) 를 포함하지 않는다** (PII leakage 방지). audit log 에는 user_profile_id 만 기록, 본문은 hash. callback_data 도 동일 PII 정책 적용 — audit 에 content_hash 만 기록.

### 4.5 Optional Requirements (Nice-to-Have)

- **REQ-MTGM-O01** [Optional]: WHERE yaml `silent_default: true` 설정 시 모든 outbound 메시지에 `disable_notification: true` 적용 (사용자 폰 알림 끔, 시각적으로만 도착).
- **REQ-MTGM-O02** [Optional]: WHERE yaml `typing_indicator: true` 설정 시 query 처리 중 매 5초마다 `sendChatAction(typing)` 호출 (사용자 UX 향상).

---

## 5. 패키지 레이아웃

### 5.1 신규 / 수정 / 변경 없음 마커

| 경로 | 마커 | 설명 |
|-----|------|-----|
| `internal/messaging/telegram/poller.go` | [NEW] (P1) | long polling loop, offset 영속, backoff |
| `internal/messaging/telegram/handler.go` | [NEW] (P1/P2) | inbound update 분기 (text/callback/file) |
| `internal/messaging/telegram/sender.go` | [NEW] (P3) | outbound `telegram_send_message` 구현 |
| `internal/messaging/telegram/client.go` | [NEW] (P1) | go-telegram/bot wrapper (testable interface) |
| `internal/messaging/telegram/config.go` | [NEW] (P1) | yaml 로드/검증 + keyring 연동 |
| `internal/messaging/telegram/store.go` | [NEW] (P2) | sqlite DB (chat_id mapping, offset) |
| `internal/messaging/telegram/markdown.go` | [NEW] (P3) | Markdown V2 escape + inline keyboard 렌더 |
| `internal/messaging/telegram/webhook.go` | [NEW] (P4) | webhook mode (BRIDGE-001 HTTP mux 등록) |
| `internal/messaging/telegram/bootstrap.go` | [NEW] (P1/P2) | daemon bootstrap entry point (`Start(ctx, deps)`) |
| `internal/messaging/telegram/audit.go` | [NEW] (P2) | AUDIT-001 wrapper (direction/hash 기록) |
| `internal/messaging/telegram/tool.go` | [NEW] (P3) | TOOLS-001 registry 등록 entry |
| `internal/messaging/telegram/agent_adapter.go` | [NEW] (P3) | ChatService domain interface 어댑터 |
| `internal/messaging/telegram/inbox.go` | [NEW] (P3) | file attach download + Janitor cleanup |
| `internal/messaging/telegram/keyring_os.go`, `keyring_nokeyring.go` | [NEW] (P3) | OS keyring (zalando/go-keyring v0.2.8) |
| `internal/agent/chat.go` | [NEW] (P3) | ChatService 도메인 인터페이스 |
| `cmd/goose/cmd/telegram.go` | [NEW] (P1/P2) | cobra subcommand (`setup|start|status|approve|revoke`) |
| `cmd/goosed/main.go` | [MODIFY] (P3) | Step 10.9 ChatService 생성 + Step 11.5 telegram bootstrap goroutine |
| `~/.goose/messaging/telegram.db` | [NEW, USER FILE] (P3) | sqlite DB (modernc.org/sqlite Option B) |
| `~/.goose/messaging/telegram.yaml` | [NEW, USER FILE] (P1) | 사용자별 설정 (bot_username, allowed_users, mode, etc.) |
| `internal/messaging/telegram/testdata/` | [NEW] | mock Telegram API fixtures, golden output |

### 5.2 의존 라이브러리 (신규)

- `github.com/go-telegram/bot` v1.x (active maintenance, MIT) — Telegram Bot API wrapper.
- `modernc.org/sqlite` v1.50.0 (pure Go sqlite3) — 독립 DB (Option B).
- `github.com/zalando/go-keyring` v0.2.8 (MIT) — OS keyring 접근 (macOS/Linux/Windows).
- 표준 라이브러리 (json, http, context, time, database/sql).

---

## 6. 의존성 (Dependencies)

| SPEC | 의존 항목 | 비고 |
|------|---------|------|
| SPEC-GOOSE-BRIDGE-001 | `ChatService` domain interface (gRPC `AgentService/Chat` 위에 어댑팅) | 기존 공개 API 사용, 새 도메인 인터페이스로 감싼 버전 |
| SPEC-GOOSE-TOOLS-001 | Tool registry 등록 API + permission gate | `telegram_send_message` 등록 |
| (독립) | sqlite DB (`~/.goose/messaging/telegram.db`, `modernc.org/sqlite` v1.50.0) | MEMORY-001 대신 독립 DB (Option B) 채택 |
| (독립) | OS keyring (zalando/go-keyring v0.2.8 — macOS Keychain / Linux Secret Service / Windows Credential Manager) | CREDENTIAL-PROXY-001 대신 독립 어댑터 채택 |
| SPEC-GOOSE-AUDIT-001 | `Append(event)` API | inbound/outbound 메시지 모두 기록 |

선행 머지 필수: BRIDGE-001 (merged), TOOLS-001 (merged), MEMORY-001 (merged), CREDENTIAL-PROXY-001 (merged), AUDIT-001 (merged) — 본 SPEC 작성 시점 모두 merged 상태로 가정.

---

## 7. 위험 (Risks)

| ID | 위험 | 가능성 | 영향 | 대응 |
|----|-----|------|-----|-----|
| R1 | Telegram Bot API rate limit (30 msg/sec/bot) 초과 | 낮음 | 중 | `go-telegram/bot` 라이브러리 측 rate limiter 활용 + audit 에 throttled 기록. SPEC scope 에서 자체 rate limit 미구현. |
| R2 | Markdown V2 escape 누락 → message render fail | 중 | 중 | 전용 unit test (`markdown_test.go`) 로 모든 special char escape 검증. Telegram 문서 §5 의 18개 reserved char (`_*[]()~\`>#+-=|{}.!`) 모두 커버. |
| R3 | webhook TLS 부재 → 등록 실패 | 중 | 낮음 | REQ-MTGM-E07: 자동 polling fallback. Local 개발 환경에서는 polling default. |
| R4 | bot token 누출 (yaml 평문 사용자 실수) | 낮음 | 높음 | REQ-MTGM-N01: setup 시 yaml 평문 token 감지 reject. CREDENTIAL-PROXY-001 keyring 강제. |
| R5 | streaming editMessageText rate limit (1초 1회) 초과 | 중 | 낮음 | chunk 결합 buffer 도입 (1초 윈도우 누적), 마지막 chunk 만 final edit. |
| R6 | 첫 user 자동 admit 으로 의도치 않은 사용자 등록 | 낮음 | 높음 | yaml `auto_admit_first_user` 기본값 `false`. 명시적 opt-in 만. |
| R7 | offset 영속 실패 → 재시작 시 중복 수신 | 낮음 | 낮음 | offset write 는 매 update 처리 후 즉시 (원자성). MEMORY-001 fsync 보장에 위임. |
| R8 | go-telegram/bot 라이브러리 breaking change | 낮음 | 중 | `client.go` 에서 thin wrapper 로 격리, `internal/messaging/telegram/client_test.go` 가 라이브러리 API 변경 회귀 검출. |

---

## 8. MX 태그 계획 (MX Plan)

### 8.1 ANCHOR 후보 (high fan_in, public API boundary)

- `internal/messaging/telegram/bootstrap.go` `Start(ctx, deps)` — daemon entry, ANCHOR (계약: deps 인터페이스 = BRIDGE/TOOLS/MEMORY/CREDPROXY/AUDIT).
- `internal/messaging/telegram/sender.go` `Send(ctx, req)` — TOOLS-001 호출 진입점, ANCHOR.
- `internal/messaging/telegram/handler.go` `Handle(ctx, update)` — inbound 분기 진입, ANCHOR.

### 8.2 NOTE 후보

- `internal/messaging/telegram/markdown.go` `EscapeV2` — 18개 reserved char (`_*[]()~\`>#+-=|{}.!`) escape 의도 명시.
- `internal/messaging/telegram/poller.go` `runLoop` — backoff 정책 (2→4→8→cap 30) 의도.
- `internal/messaging/telegram/store.go` offset 영속 시점 (매 update 직후) 의도.

### 8.3 WARN 후보

- `internal/messaging/telegram/poller.go` `runLoop` — goroutine + context cancel + retry, complexity ≥ 15 예상 → WARN + `@MX:REASON` (long polling 본질, 분리 시 race condition 위험).
- `internal/messaging/telegram/sender.go` rate limit 처리 — Telegram 30 msg/sec 제약 + edit 1/sec 제약 동시 — WARN.

### 8.4 TODO (RED phase 시작)

- `Send` empty stub → `@MX:TODO` until first GREEN.
- `runLoop` empty stub → `@MX:TODO`.

---

## 9. Acceptance Criteria 요약 (상세는 acceptance.md)

11개 Acceptance Criteria (AC-MTGM-001 ~ AC-MTGM-011), 모두 Given-When-Then 형식. 핵심 5개 요약:

- **AC-MTGM-001**: `goose messaging telegram setup` 1회 실행 → bot 등록 + token OS keyring 저장 + 첫 chat_id 매핑 (사용자 setup SLO 5분, 통합 시나리오 1).
- **AC-MTGM-002**: 사용자가 Telegram 에서 `Hello` 전송 → GOOSE 응답 수신 (왕복 P95 < 5초, audit log 2개 entry — inbound + outbound, 본문 hash 만 기록).
- **AC-MTGM-005**: GOOSE agent 가 `telegram_send_message` tool 호출 → 사용자에게 proactive 메시지 도착 + audit + permission gate 통과 + allowed_users 검증. **(E2 modal 부분은 P4 deferred — CLI-TUI-002 modal 미구현)**
- **AC-MTGM-008**: bot token yaml 평문 기재 시 setup reject + 안내 메시지 출력 (REQ-MTGM-N01).
- **AC-MTGM-011**: AUDIT-001 entry 가 PII (본문 raw / 타 사용자 정보) 미포함 + append-only 무결성 + audit fail 시에도 채널 작동.

전체 11개 AC 의 Given-When-Then + edge case 는 acceptance.md 참조.

---

## 10. 완료 기준 (Definition of Done)

### P3 까지 완료 (2026-05-09)

- [x] 4 개 area (Setup CLI / Polling / Tool / Mapping) 구현 완료 (P3 까지).
- [x] 10 개 AC GREEN (AC-MTGM-001 ~ 011 중 AC-MTGM-005 E2 / AC-MTGM-009 제외).
  - AC-MTGM-005 E2 (CLI-TUI-002 modal) — **P4 에서도 외부 SPEC 의존으로 deferred 유지**.
  - AC-MTGM-009 (streaming UX) — **P4 에서 GREEN 처리**.
- [x] `go test ./internal/messaging/telegram/...` coverage ≥ 83% (P3 정점).
- [x] Markdown V2 escape unit test 18개 reserved char (`_*[]()~\`>#+-=|{}.!`) 전수 통과.
- [x] mock Telegram API (httptest) 기반 integration test 통과 — `setup → send → receive → audit` 플로우.
- [x] `goose messaging telegram setup` 가 5분 이내 첫 chat_id 매핑까지 도달 (수동 시나리오 검증).
- [x] audit log 가 inbound/outbound 모두 기록되고 본문 hash 가 일치 (PII leak 방지 검증).
- [x] yaml 평문 token 감지 reject 동작 검증.
- [x] daemon bootstrap 시 token 미설정 환경에서 graceful skip + warning 만 (daemon 자체는 기동).
- [x] golangci-lint clean, gofmt clean.
- [x] `@MX:NOTE` / `@MX:ANCHOR` / `@MX:WARN` / `@MX:REASON` 모두 적용, `@MX:TODO` 0개.

### P4 완료 (2026-05-09, PR #131 머지)

- [x] Webhook mode (Area 5) — polling default + `mode: webhook` + setWebhook + TLS 부재/등록 실패 시 polling 자동 fallback (REQ-MTGM-E07).
- [x] Streaming UX — `/stream` 접두 또는 `default_streaming: true` 시 placeholder `sendMessage` → chunk-merge buffer 1초 ticker `editMessageText` → final flush (REQ-MTGM-E02, AC-MTGM-009).
- [x] silent_default — outbound `disable_notification: true` (REQ-MTGM-O01).
- [x] typing_indicator — query 처리 중 매 5초 `sendChatAction(typing)` (REQ-MTGM-O02).
- [x] streaming queue — per chat_id FIFO max 5, 가득 시 안내 메시지 + drop (REQ-MTGM-S05).
- [x] testdata/ golden output 회귀 보호 — markdown_v2 7 fixture pair + inline_keyboard 5 fixture pair, `-update-golden` flag 지원.
- [x] manual_smoke.md P4 시나리오 5개 추가.
- [x] coverage telegram 84.6% (P3 종점 회복; ticker 5초 wait path 단위 테스트 한계로 strict 85% 미달).
- [x] golangci-lint 0 issues, gofmt clean, go vet clean.
- [x] `@MX:TODO` 0개 유지.

### 외부 SPEC 의존으로 본 SPEC 범위 외 (deferred)

- [ ] AC-MTGM-005 E2 (CLI-TUI-002 modal integration) — CLI-TUI-002 modal 구현 SPEC 머지 후 자동 GREEN 화 가능. 현재 본 SPEC 측 sender/registry preapproval 이중 방어로 기능적 완성도는 확보됨.

---

## 11. 참고 (References)

- Telegram Bot API 문서: https://core.telegram.org/bots/api (v6.x)
- Markdown V2 specification: https://core.telegram.org/bots/api#markdownv2-style
- go-telegram/bot 라이브러리: https://github.com/go-telegram/bot
- SPEC-GOOSE-BRIDGE-001 (Daemon ↔ UI Bridge)
- SPEC-GOOSE-TOOLS-001 (Tool Registry)
- SPEC-GOOSE-MEMORY-001 (Memory Provider)
- SPEC-GOOSE-CREDENTIAL-PROXY-001 (또는 CREDPOOL-001)
- SPEC-GOOSE-AUDIT-001 (Audit Log)
- `.moai/project/ROADMAP.md` Phase 4 (channel rollout)
- `research.md` (라이브러리 평가 + 통신 패턴 + 실패 시나리오)
- `plan.md` (P1~P4 구현 계획)
- `acceptance.md` (Given-When-Then × 11)
- `spec-compact.md` (token-economic compact view)

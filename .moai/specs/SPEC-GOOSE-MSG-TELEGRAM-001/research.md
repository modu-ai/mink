---
id: SPEC-GOOSE-MSG-TELEGRAM-001
artifact: research
version: "0.1.0"
created_at: 2026-05-05
updated_at: 2026-05-05
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — Research

본 문서는 Telegram Bot API 6.x 의 핵심 사양 + Go 라이브러리 평가 + GOOSE 의존 SPEC 들과의 통신 패턴 + 실패 시나리오 분석을 다룬다.

---

## 1. Telegram Bot API 6.x 핵심 사양 분석

### 1.1 인증 모델

- **Bot token**: BotFather 발급, 형식 `<bot_id>:<35자 random>` (예: `1234567890:ABCDEFghijklmnopqrstuvwxyz12345678`).
- token 자체가 인증 (별도 OAuth flow 없음). 모든 API 호출 URL: `https://api.telegram.org/bot<TOKEN>/<METHOD>`.
- token 유출 = bot 완전 탈취. 따라서 keyring 보관 필수 (REQ-MTGM-N01 의 근거).

### 1.2 Update 수신 두 가지 모드

#### 1.2.1 Long Polling (default 채택)

- `getUpdates(offset, timeout=30, allowed_updates=[...])` 호출.
- 서버는 새 update 가 있으면 즉시, 없으면 timeout 까지 holding 후 빈 배열 반환.
- 응답 받은 update 의 `update_id` + 1 을 다음 호출의 offset 으로 사용 → 자동 ack.
- 장점: outbound NAT 친화 (방화벽 / 사설망 OK), TLS 인증서 불필요, 개발 환경 친화.
- 단점: 항상 1 connection 유지, 파일럿 단계 적합.

GOOSE 의 default 채택 이유:

- 첫 채널 → 사용자 마찰 최소. 개발자가 webhook 등록 + TLS 인증서 + 도메인 준비 강요받지 않음.
- BRIDGE-001 의 HTTP 서버를 webhook 으로 노출하려면 외부 접근 (ngrok 등) 필요 → 첫 사용자 setup 5분 목표 달성 불가.

#### 1.2.2 Webhook (옵션)

- `setWebhook(url=<https URL>, secret_token=<...>, allowed_updates=[...])`.
- 서버가 update 발생 시 해당 URL 로 POST.
- 장점: connection 효율, 즉시 push, 대량 트래픽 친화.
- 단점: 도메인 + TLS 인증서 + 외부 도달 가능성 필요. 자가호스팅 환경 부담.

GOOSE 의 옵션 채택 이유: BRIDGE-001 이 이미 HTTP 서버를 가지고 있으므로 같은 mux 에 `/webhook/telegram/<secret>` 등록 가능. ngrok / Cloudflare tunnel 환경에서는 더 효율적.

### 1.3 Go 라이브러리 평가

#### 후보 1: `github.com/go-telegram/bot` ★ 채택

- **Stars**: ~1.2k (2026-04 기준), 활발.
- **maintenance**: 2주 단위 commit, 최신 Bot API 7.x 까지 follow.
- **License**: MIT.
- **API style**: generic-friendly, struct 기반 (no global state).
- **dependencies**: `golang.org/x/sync` 만, 의존 최소.
- **mock 친화성**: 인터페이스 추출이 자연스러움.
- **rate limiter**: 내장 (30 msg/sec/bot 기본).

#### 후보 2: `github.com/go-telegram-bot-api/telegram-bot-api/v5`

- **Stars**: ~5.5k, 인지도 높음.
- **maintenance**: 6개월 휴면 (2025년 후반 commit 적음, 우려).
- **API style**: legacy, struct 기반.
- **rate limiter**: 외부 미들웨어 필요.
- **rejection 사유**: maintenance pace, 후보 1 의 generics 친화성 우위.

#### 후보 3: 직접 구현 (`net/http` + `encoding/json` + `getUpdates`)

- **장점**: 의존 0개, 정확한 제어.
- **단점**: maintenance burden 큼, Telegram Bot API 7.x → 8.x 마이그레이션 시 자체 부담, error handling 코드 중복.
- **rejection 사유**: 첫 채널 SPEC 에서 시간 비용 과다. 후보 1 wrapper 가 격리 비용을 충분히 낮춤.

#### 채택 결과

**`github.com/go-telegram/bot` v1.x** 채택. `client.go` 에서 thin wrapper interface 추출하여 라이브러리 의존을 격리 (R8 위험 대응).

```go
// internal/messaging/telegram/client.go (스케치)
type Client interface {
    GetMe(ctx context.Context) (*User, error)
    GetUpdates(ctx context.Context, offset int, timeout time.Duration) ([]Update, error)
    SendMessage(ctx context.Context, req SendMessageRequest) (*Message, error)
    EditMessageText(ctx context.Context, req EditMessageTextRequest) (*Message, error)
    AnswerCallbackQuery(ctx context.Context, req AnswerCallbackQueryRequest) error
    GetFile(ctx context.Context, fileID string) (*File, error)
    DownloadFile(ctx context.Context, filePath string, dst io.Writer) error
    SendPhoto(ctx context.Context, req SendPhotoRequest) (*Message, error)
    SendDocument(ctx context.Context, req SendDocumentRequest) (*Message, error)
    SetWebhook(ctx context.Context, req SetWebhookRequest) error
    DeleteWebhook(ctx context.Context) error
}
```

### 1.4 메시지 제약 (Telegram 측)

| 항목 | 제약 |
|-----|-----|
| 단일 메시지 최대 길이 | 4096 chars (UTF-16 code units) |
| 캡션 길이 | 1024 chars |
| inline keyboard callback_data | 64 bytes |
| sendPhoto file size | 10 MB |
| sendDocument file size | 50 MB |
| sendMessage rate limit | 30 msg/sec/bot (글로벌) |
| editMessageText | 권장 1회/sec (rate limit 보호) |
| getUpdates timeout | 0~50초 (권장 30초) |
| chat 별 메시지 발송 | 1 msg/sec/chat (대화방당) |

REQ-MTGM-E04 (4096자 거부) 와 sender의 split 로직 근거.

### 1.5 Markdown V2 Escape

Telegram Bot API 문서 §5 Formatting:

> All `_`, `*`, `[`, `]`, `(`, `)`, `~`, `` ` ``, `>`, `#`, `+`, `-`, `=`, `|`, `{`, `}`, `.`, `!` characters must be escaped with the preceding character `\`.

총 18개 reserved char (Telegram MarkdownV2 spec §5). markdown_test.go 가 18개 전수 검증.

추가 주의: code block (`)에서는 `\` 미사용, ` 자체만 닫음. URL 의 `()` 는 별도 처리.

### 1.6 callback_query 제약

- timeout: 사용자 클릭 후 60초 안에 `answerCallbackQuery` 응답 안 하면 expire.
- expire 후 응답 시 Telegram API 400. → REQ-MTGM-N04 + AC-MTGM-010 E3.

---

## 2. GOOSE 의존 SPEC 통신 패턴

### 2.1 BRIDGE-001 (`AgentService/Query`)

- **호출 시그니처**: `Query(ctx, QueryRequest{session_id, prompt, attachment_paths, metadata}) → QueryResponse{message_id, text, tool_calls, ...}`.
- **streaming 변형**: `QueryStream(ctx, req) → stream<ChatChunk>` (SSE over HTTP).
- **session_id 매핑**: chat_id → session_id (1:1, 또는 chat_id 별 새 session). 첫 채택: 1 chat_id = 1 persistent session (`messaging.telegram.<chat_id>` prefix).
- **검증 필요**: BRIDGE-001 의 `Query` 가 `attachment_paths` field 를 이미 보유하는지 — research 시점에서 spec.md 참조 필요. **TODO during P1**: spec.md 확인 후 미보유 시 BRIDGE-001 minor amendment 또는 metadata field 활용.

### 2.2 TOOLS-001 (Tool Registry)

- **등록 API**: `Register(name string, schema JSONSchema, handler func(ctx, input) → output) error`.
- **Permission gate**: TOOLS-001 이 호출 시 CLI-TUI-002 의 permission modal 또는 stored decision 적용.
- **in-process**: same daemon process, network roundtrip 없음.
- **호출 흐름**: agent 가 LLM 응답에서 tool call → TOOLS-001 dispatcher → permission gate → handler 실행 → 결과 LLM 컨텍스트로.

본 SPEC 의 `tool.go` (또는 `internal/tools/telegram_send.go`) 가 init 시점에 `Register("telegram_send_message", schema, sender.Send)` 호출.

### 2.3 MEMORY-001 (Memory Provider)

- **provider 추상화**: BoltDB (default) / sqlite / inmem.
- **API**: `Get(bucket, key) → value`, `Set(bucket, key, value) error`, `Iterate(bucket, callback)`.
- **본 SPEC 사용 bucket**:
  - `messaging.telegram.users` — chat_id → user_profile mapping
  - `messaging.telegram.last_offset` — single key, value=int64 (last processed update_id)
  - `messaging.telegram.config_cache` (옵션) — yaml hot-reload 지원 시
- **fsync**: BoltDB 기본 fsync on commit → REQ-MTGM-U05 의 영속성 근거.

### 2.4 CREDENTIAL-PROXY-001 (또는 CREDPOOL-001)

- **API**: `Set(key, value, scope) error`, `Get(key, scope) → value`.
- **scope**: user (per-OS keyring) / process (memory only) / file (encrypted file fallback).
- **본 SPEC 사용**: `Set("telegram.bot.token", <token>, scope=user)`.
- **fallback chain**: macOS Keychain → Windows Credential Manager → Linux Secret Service → AES-encrypted file (`~/.goose/credentials.enc`).
- **TODO during P1**: CREDENTIAL-PROXY-001 vs CREDPOOL-001 중 어느 것이 keyring 책임 보유인지 spec.md 확인. 본 research 작성 시점은 CREDENTIAL-PROXY-001 가 keyring proxy, CREDPOOL-001 은 LLM provider credential pool 이라는 가정.

### 2.5 AUDIT-001 (Audit Log)

- **API**: `Append(event AuditEvent) error`.
- **AuditEvent struct**: `{ts, source, direction, hash, metadata{...}}`.
- **본 SPEC 의 source 값**: `messaging.telegram`.
- **storage**: append-only file (`~/.goose/audit/<date>.jsonl`) 또는 BoltDB bucket — 기존 SPEC 결정 따름.
- **fsync**: append 시 즉시 flush (감사 추적 무결성).

---

## 3. 통신 시퀀스 다이어그램

### 3.1 Inbound (사용자 → GOOSE)

```
User ─[Telegram]─▶ Telegram Server
                       │
                       │ getUpdates (long poll)
                       ▼
              ┌─────────────────┐
              │ telegram.Poller │
              └────────┬────────┘
                       │ Update
                       ▼
              ┌─────────────────┐
              │ telegram.Handler│
              └────────┬────────┘
                       │ chat_id 매핑 조회
                       ▼
                ┌─────────────┐
                │ MEMORY-001  │
                └─────────────┘
                       │ user_profile_id
                       ▼
              ┌─────────────────┐
              │ telegram.Audit  │ ─▶ AUDIT-001 (in)
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ BRIDGE-001 Query│
              └────────┬────────┘
                       │ response text
                       ▼
              ┌─────────────────┐
              │ markdown V2     │
              │ escape          │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ Client.SendMsg  │ ─▶ Telegram Server ─▶ User
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ telegram.Audit  │ ─▶ AUDIT-001 (out)
              └─────────────────┘
```

### 3.2 Outbound (Agent → User via Tool)

```
Agent ─▶ LLM tool_call: telegram_send_message(...)
            │
            ▼
       ┌──────────────┐
       │ TOOLS-001    │ permission gate
       │ dispatcher   │
       └──────┬───────┘
              │ allow
              ▼
       ┌──────────────┐
       │ telegram.Send│ allowed_users 검증
       │              │ Markdown V2 escape
       └──────┬───────┘
              │
              ▼
       ┌──────────────┐
       │ Client.SendMsg─▶ Telegram Server ─▶ User
       └──────┬───────┘
              │
              ▼
       ┌──────────────┐
       │ telegram.Audit ─▶ AUDIT-001 (out, tool_call_id)
       └──────────────┘
```

### 3.3 Streaming (사용자 → GOOSE → 사용자, BRIDGE streaming RPC)

```
User msg "/stream ..." ─▶ Handler
                              │
                              ▼
                      placeholder sendMessage("...")
                              │
                              ▼ (BRIDGE-001 streaming open)
                       chunk_1 ─┐
                       chunk_2  │ ─▶ buffer 누적
                       ...      │
                                │
                       (1초 ticker)
                                ▼
                       editMessageText(buffer)
                                │
                                ▼
                       chunk_n (final)
                                ▼
                       buffer flush + editMessageText(final)
```

핵심: `editMessageText` rate limit 1/sec. buffer 가 1초 단위로 누적되어 한 번에 update.

---

## 4. 실패 시나리오 분석

### 4.1 Network 실패

| 시나리오 | 원인 | 대응 |
|---------|------|-----|
| Telegram API 타임아웃 | 인터넷 단절 | exponential backoff 2→4→8→cap 30초, audit warning 추가 |
| getUpdates 401 | token 무효 (revoke) | poller 정지 + status `token_revoked` + setup 재실행 안내 |
| sendMessage 5xx | Telegram 측 문제 | 3회 재시도, 모두 실패 시 audit `send_failed` |
| sendMessage 429 (rate limit) | 30 msg/sec 초과 | retry-after header 따라 sleep + 재시도 |

### 4.2 Data 손상

| 시나리오 | 원인 | 대응 |
|---------|------|-----|
| BoltDB corruption | 디스크 오류 | daemon 시작 시 fail-fast, recovery script 안내 |
| keyring access denied | OS keyring 잠김 | fallback 환경변수 안내 |
| yaml syntax error | 사용자 수동 수정 실수 | startup reject + ERROR 로그 |

### 4.3 동시성 / 경합

| 시나리오 | 원인 | 대응 |
|---------|------|-----|
| 두 daemon instance 동시 실행 | 사용자 실수 | offset 경합 → 한 쪽이 항상 lag. Lock file (`~/.goose/messaging/telegram.lock`) 도입 — 본 SPEC 범위는 단일 instance 가정. |
| streaming 중 daemon kill | 비정상 종료 | 사용자 입장에서 placeholder 메시지가 "..." 로 멈춤. 재시작 시 streaming 재개 안 함 (idempotency 부족, 사용자가 재요청 필요). |

---

## 5. 변경 영향 분석

### 5.1 [EXISTING] 변경 없음 (회귀 보호 대상)

- BRIDGE-001 의 모든 공개 API.
- TOOLS-001 의 register / dispatch API.
- MEMORY-001 의 Get/Set API.
- CREDENTIAL-PROXY-001 의 keyring API.
- AUDIT-001 의 Append API.

### 5.2 [MODIFY] 최소 변경

- `internal/daemon/bootstrap.go` — messaging.telegram.Start 호출 추가 (5~10 LOC).

### 5.3 [NEW] 신규 패키지

- `internal/messaging/telegram/` 전체.
- `cmd/goose/cmd/telegram.go`.
- `~/.goose/messaging/` 디렉토리 (사용자 측, 자동 생성).

---

## 6. 성능 / 자원 추정

- **메모리**: poller goroutine + handler + 큐 = ~5MB resident.
- **네트워크**: long polling 1 connection 유지, 30초마다 재연결, 평균 < 1KB/s idle.
- **디스크**: audit log 평균 1KB/메시지 (본문 hash + 메타). 사용자당 일평균 50메시지 가정 시 50KB/일.
- **CPU**: idle < 1%, 메시지 처리 시 spike < 10ms.

---

## 7. 검증 체크리스트 (research → plan 진입 게이트)

- [x] Telegram Bot API 6.x 핵심 사양 파악 (long polling, webhook, Markdown V2, file size).
- [x] Go 라이브러리 3개 후보 평가 → `go-telegram/bot` 채택.
- [x] BRIDGE-001 / TOOLS-001 / MEMORY-001 / CREDENTIAL-PROXY-001 / AUDIT-001 통신 패턴 정의.
- [x] 통신 시퀀스 다이어그램 (inbound / outbound / streaming).
- [x] 실패 시나리오 + 대응 정의.
- [x] 변경 영향 (EXISTING / MODIFY / NEW) 마커.
- [ ] **TODO P1 진입 시**: BRIDGE-001 spec.md 의 `Query` 시그니처에서 `attachment_paths` field 존재 여부 확인 (없으면 metadata 활용).
- [ ] **TODO P1 진입 시**: CREDENTIAL-PROXY-001 spec.md 에서 keyring 책임 확정 (vs CREDPOOL-001).

---

## 8. 참고 자료

- Telegram Bot API: https://core.telegram.org/bots/api
- Markdown V2 syntax: https://core.telegram.org/bots/api#markdownv2-style
- BotFather: https://t.me/BotFather
- go-telegram/bot: https://github.com/go-telegram/bot
- go-telegram-bot-api/telegram-bot-api: https://github.com/go-telegram-bot-api/telegram-bot-api
- Telegram rate limits: https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
- BoltDB fsync semantics: https://github.com/etcd-io/bbolt#caveats--limitations

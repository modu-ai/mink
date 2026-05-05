---
id: SPEC-GOOSE-MSG-TELEGRAM-001
artifact: acceptance
version: "0.1.0"
created_at: 2026-05-05
updated_at: 2026-05-05
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — Acceptance Criteria

11 개 Acceptance Criteria, 모두 Given-When-Then 형식. REQ 매핑 명시.

---

## AC-MTGM-001 — Setup CLI 5분 시나리오 (REQ-MTGM-U02, REQ-MTGM-N01)

**Given**:
- 사용자가 BotFather 에서 새 bot 을 생성하여 token 을 발급받음.
- `goosed` daemon 이 실행 중 (또는 미실행 — setup 자체는 daemon 무관).
- `~/.goose/messaging/telegram.yaml` 파일이 존재하지 않음.
- CREDENTIAL-PROXY-001 keyring 이 정상 동작 (macOS Keychain / Linux Secret Service / Windows Credential Manager).

**When**:
1. 사용자가 `goose messaging telegram setup` 실행.
2. 프롬프트에서 token 입력 (또는 `GOOSE_TELEGRAM_BOT_TOKEN` 환경변수 활용).
3. CLI 가 `getMe` API 호출하여 token 검증 + bot username 출력.
4. CLI 가 keyring 에 token 저장 (`telegram.bot.token`, scope=user).
5. CLI 가 `~/.goose/messaging/telegram.yaml` 생성 (bot_username 포함, allowed_users 빈 배열, mode=polling, audit_enabled=true).
6. CLI 가 안내 출력: "Telegram 에서 @<bot_username> 에게 `/start` 보내세요" + 30초 대기 (선택).

**Then**:
- yaml 파일이 생성되고 token 평문이 **포함되지 않음** (REQ-MTGM-N01).
- keyring 에 token 이 저장됨 (`security find-generic-password ...` 또는 동등 검증).
- 사용자가 30초 안에 `/start` 보내면 첫 chat_id 가 자동 매핑되며 yaml `allowed_users` + MEMORY-001 mapping 동시 갱신 (yaml `auto_admit_first_user: true` 기본값일 때).
- 전체 시나리오 완료 시간 ≤ 5분 (사용자 BotFather 작업 + token 입력 + 첫 메시지 송신까지).

**Edge Cases**:
- E1) 잘못된 token → `getMe` 401 → CLI "Invalid token" 출력 + keyring 미저장 + yaml 미생성.
- E2) 네트워크 단절 → 3회 재시도 후 "Telegram API unreachable" 에러.
- E3) yaml 이미 존재 → "Already configured. Use `goose messaging telegram status` or delete yaml first" 안내 + 작업 중단.
- E4) keyring 비활성 (Linux secret service 부재) → 환경변수 fallback 안내.

---

## AC-MTGM-002 — 첫 텍스트 메시지 round-trip (REQ-MTGM-E01, REQ-MTGM-U01, REQ-MTGM-U03, REQ-MTGM-E04, REQ-MTGM-N03, REQ-MTGM-N06)

**Given**:
- AC-MTGM-001 완료, chat_id 1개 매핑됨.
- `goosed` daemon 실행 중.
- BRIDGE-001 `AgentService/Query` 가 텍스트 입력 받아 텍스트 응답 반환.
- AUDIT-001 가 정상 동작.

**When**:
1. 사용자가 Telegram 에서 `Hello, GOOSE` 텍스트 전송.
2. poller 가 update 수신 → handler 가 chat_id 매핑 조회 → BRIDGE-001 `Query("Hello, GOOSE")` 호출.
3. BRIDGE-001 가 응답 반환 (예: "Hello! How can I help?").
4. handler 가 응답을 sendMessage 로 사용자에게 전송.

**Then**:
- 사용자 Telegram 클라이언트에 "Hello! How can I help?" 메시지 도착.
- 왕복 시간 P95 < 5초 (Telegram 서버 RTT 제외, 사용자 모바일 네트워크 환경 무시 시).
- AUDIT-001 에 2개 entry append:
  - entry 1: `direction=in, chat_id=<id>, message_id=<msg>, content_hash=<sha256(Hello, GOOSE)>, ts=<>`.
  - entry 2: `direction=out, chat_id=<id>, message_id=<msg>, content_hash=<sha256(Hello! How can I help?)>, ts=<>`.
- MEMORY-001 의 `messaging.telegram.users.<chat_id>.last_seen_at` 갱신.
- MEMORY-001 의 `messaging.telegram.last_offset` 가 update_id 보다 큰 값으로 갱신.

**4096자 inbound 거부 검증 (REQ-MTGM-E04, REQ-MTGM-N03)**:
- 사용자가 4097자 (또는 그 이상) 본문을 전송 시 → BRIDGE Query 미호출 → 사용자에게 `RESP-MTGM-E04: 메시지가 너무 깁니다 (max 4096 chars)` 응답 → audit entry 의 metadata 에 `length_exceeded: true, length: 4097` 기록.

**PII hash 검증 (REQ-MTGM-N06)**:
- AUDIT-001 entry 의 `content_hash` field 가 SHA-256 hex string (64자) 인지 확인.
- AUDIT-001 entry 에 본문 평문 (raw text) 이 **포함되지 않음** 검증 — 다른 사용자 정보 leak 방지.
- audit entry inspection test (`audit_test.go`) 가 entry 직렬화 결과에 본문 raw 가 없음을 assert.

**Edge Cases**:
- E1) BRIDGE-001 timeout (30초 초과) → 사용자에게 "처리 시간 초과" 응답 + audit `query_timeout` flag.
- E2) 응답 본문 4096자 초과 → split 으로 여러 메시지 전송 (Telegram 단일 메시지 outbound 제약, 문서상 split rule 명시).
- E3) Telegram API 5xx → backoff 후 재시도, 최대 3회.

---

## AC-MTGM-003 — chat_id ↔ user_profile mapping 영속 (REQ-MTGM-U03, REQ-MTGM-U05)

**Given**:
- 사용자 1명이 등록되어 chat_id `<C>` ↔ user_profile_id `<P>` 매핑됨.
- `goose messaging telegram status` 가 mapping 1개를 반환.

**When**:
1. `goosed` daemon 재시작 (graceful stop + start).
2. 사용자가 다시 메시지 전송.

**Then**:
- daemon 재시작 후에도 mapping 이 보존됨 (BoltDB 영속).
- offset 도 보존되어 재시작 직전 마지막 update 의 offset+1 부터 polling 재개 → **중복 수신 0**.
- 사용자가 보낸 메시지가 "이미 처리된 것" 으로 간주되지 않고 정상 query 트리거.

**Edge Cases**:
- E1) BoltDB 손상 → daemon 시작 시 fail-fast + "Memory store corrupt, recovery needed" 로그 + messaging 모듈 비활성.
- E2) offset 영속 직후 daemon kill -9 → 마지막 update 1개 중복 처리 가능 (idempotency 는 BRIDGE-001 query 측에서 보장 — 본 SPEC 범위 밖).

---

## AC-MTGM-004 — First-message gate (REQ-MTGM-S01, REQ-MTGM-S04, REQ-MTGM-N05)

**Given**:
- yaml `auto_admit_first_user: false`, `allowed_users: []`.
- 사용자 chat_id `<X>` 가 mapping 에 없음.

**When**:
- 사용자 `<X>` 가 메시지 전송.

**Then**:
- BRIDGE-001 query 미실행.
- 사용자에게 응답: "이 봇은 사전 승인된 사용자만 사용할 수 있습니다. 관리자에게 chat_id `<X>` 를 전달하세요".
- audit log 에 `direction=in` entry 만 (outbound 안내 메시지도 audit 됨, 총 2개 entry, 단 query 는 미호출).
- MEMORY-001 mapping 에 `<X>` 가 미등록 (또는 `allowed=false` 로 placeholder).

**Then (auto_admit_first_user: true 일 때)**:
- 같은 시나리오 재실행 시 `<X>` 가 자동으로 admin 등록 + yaml `allowed_users` 에 추가 + MEMORY-001 mapping 생성.
- audit 에 `auto_admitted: true` 기록.
- 통상 round-trip (AC-MTGM-002) 으로 진행.

**Blacklist drop 검증 (REQ-MTGM-N05)**:
- `goose messaging telegram revoke <X>` 으로 차단된 chat_id `<X>` 에서 메시지 전송 시:
  - 사용자에게 **응답 없음** (silently dropped — 차단 사실 누설 금지).
  - audit entry 에 `direction=in, dropped: blocked, chat_id=<X>` 기록 (outbound 없음).
  - BRIDGE Query 미호출.
- yaml `blocked_users: [<X>]` 직접 설정 시도 동일 동작 확인.

**Edge Cases**:
- E1) `goose messaging telegram approve <X>` 로 명시 승인 후 다음 메시지 → 정상 round-trip.
- E2) revoke 후 audit log 조회 시 mapping 에 `allowed=false` 가 보존되어 이력 추적 가능.

---

## AC-MTGM-005 — Outbound `telegram_send_message` tool (REQ-MTGM-E03, REQ-MTGM-N02, REQ-MTGM-U04)

**Given**:
- AC-MTGM-001 + AC-MTGM-002 완료, chat_id `<C>` 매핑됨.
- TOOLS-001 registry 가 `telegram_send_message` tool 을 보유.
- TOOLS-001 permission gate 가 정상 (CLI-TUI-002 modal).
- GOOSE agent 가 query 응답에서 tool call 트리거.

**When**:
1. agent 가 `telegram_send_message({"chat_id": "<C>", "text": "*Daily Brief*\n\n오늘의 일정 3건"})` 호출.
2. TOOLS-001 permission gate 가 사용자에게 modal 표시 (또는 stored permission 적용).
3. 사용자가 Allow → tool 실행.
4. sender 가 allowed_users 검증 → `<C>` 통과 → Markdown V2 escape → sendMessage 호출.

**Then**:
- 사용자 Telegram 클라이언트에 "**Daily Brief**\n\n오늘의 일정 3건" 메시지 도착 (bold 렌더).
- tool 응답: `{message_id: <m>, chat_id: "<C>", sent_at: <iso>, audit_id: <a>}`.
- AUDIT-001 에 `direction=out, tool_call_id=<id>, content_hash=<>` entry append.

**Edge Cases**:
- E1) chat_id 가 allowed_users 미포함 → tool error `unauthorized_chat_id` + audit `denied: not_allowed`.
- E2) Permission gate 에서 사용자 Deny → tool error `permission_denied` + 사용자에게 미전송.
- E3) text > 4096자 → split 으로 여러 메시지 전송 (Markdown 컨텍스트 보존 best-effort).
- E4) Markdown V2 syntax error (escape 누락) → tool error `markdown_invalid` + 미전송.

---

## AC-MTGM-006 — Daemon graceful skip + status visibility (REQ-MTGM-S02, REQ-MTGM-S03, REQ-MTGM-E07)

**Given**:
- keyring 에 telegram bot token 미저장.
- `~/.goose/messaging/telegram.yaml` 미존재.

**When**:
- `goosed` daemon 시작.

**Then**:
- daemon 자체는 정상 기동 (다른 모듈 영향 없음).
- log 에 warning: "Telegram messaging not configured. Run `goose messaging telegram setup` to enable.".
- `goose messaging telegram status` 가 `not configured` 반환.
- inbound polling goroutine 미시작.
- TOOLS-001 registry 에 `telegram_send_message` 미등록 (호출 시 unknown tool error).

**Backoff metrics 검증 (REQ-MTGM-S03)**:
- token 정상 + 네트워크 일시 단절 시뮬레이션 (mock client 가 3회 연속 timeout 반환):
  - poller 가 backoff 모드 진입 → status 명령 시 `poller: backoff_step=2, next_retry_at=<iso>` 출력.
  - 네트워크 회복 후 status 명령 시 `poller: running` 으로 복귀.

**Webhook fallback 검증 (REQ-MTGM-E07)**:
- yaml `mode: webhook` 으로 설정했으나 BRIDGE-001 HTTP 서버가 TLS 미활성 (또는 setWebhook 401 반환):
  - bootstrap.Start 가 자동으로 polling mode 로 fallback.
  - log 에 `WARN webhook registration failed (reason: tls_required), falling back to polling`.
  - status 출력 시 `mode: polling (fallback from webhook)` 표시.
- 명시적 fallback 비활성화 옵션 (`webhook_no_fallback: true`) 시 → bootstrap fail-fast (별도 옵션 미구현 시 OUT-of-scope, 본 AC 는 fallback 동작만 검증).

**Edge Cases**:
- E1) yaml 은 존재하나 keyring 비어 있음 → "Token missing in keyring. Re-run setup" warning.
- E2) yaml 손상 (invalid yaml syntax) → fail-fast warning + messaging 모듈 비활성.

---

## AC-MTGM-007 — File attach round-trip (REQ-MTGM-E06)

**Given**:
- AC-MTGM-002 완료.
- 사용자가 Telegram 에서 GOOSE 봇으로 이미지 (jpg, 500KB) 전송.

**When**:
1. poller 가 update 수신 (message 에 photo array 포함).
2. handler 가 `getFile` API 호출 → file_path 획득.
3. handler 가 file 다운로드 → `~/.goose/messaging/telegram/inbox/<msg_id>.jpg` 저장.
4. handler 가 BRIDGE-001 query 호출 시 본문에 `attachment_paths: ["~/.goose/messaging/telegram/inbox/<msg_id>.jpg"]` 포함.
5. BRIDGE-001 응답 수신.
6. handler 가 응답 sendMessage.
7. 30분 후 cleanup goroutine 이 임시 파일 삭제.

**Then**:
- inbox 디렉토리에 다운로드된 파일 존재 (30분간).
- BRIDGE-001 query 본문이 attachment 경로 포함.
- 응답이 사용자에게 도착.
- 30분 후 임시 파일이 자동 삭제 (filesystem 검증).
- AUDIT-001 entry 에 `attachment_count: 1, attachment_size_bytes: 500000` 기록 (본문 hash 와 별도).

**Outbound 측**:
- agent 가 `telegram_send_message({chat_id: "<C>", attachments: [{type: "image", path: "/tmp/chart.png"}]})` 호출 시 sendPhoto multipart upload.
- 사용자 Telegram 에 이미지 도착.

**Edge Cases**:
- E1) 파일 크기 > 50MB → tool error `attachment_too_large` (Telegram bot 제약).
- E2) inbox 다운로드 실패 → query 본문에 `attachment_error: <msg>` 포함, query 는 진행.
- E3) cleanup goroutine 이 daemon 재시작 후에도 30분 미만 파일 보존 (재시작 시 mtime 검사).

---

## AC-MTGM-008 — yaml 평문 token reject (REQ-MTGM-N01, REQ-MTGM-U02)

**Given**:
- 사용자가 실수로 `~/.goose/messaging/telegram.yaml` 에 `bot_token: "1234:ABC..."` 키를 직접 추가.
- `goosed` daemon 시작 또는 `goose messaging telegram start`.

**When**:
- config.Load() 실행.

**Then**:
- config.Load() 가 reject 하며 다음 에러 반환:
  - "yaml 에 bot_token 평문 기재 금지. 환경변수 GOOSE_TELEGRAM_BOT_TOKEN 또는 `goose messaging telegram setup` 으로 keyring 등록하세요."
- messaging 모듈 비활성, daemon 자체는 기동.
- log 에 ERROR 레벨 기록.

**Edge Cases**:
- E1) yaml 의 다른 키 (`bot_username`, `allowed_users` 등) 만 있고 `bot_token` 없음 → 정상 통과.
- E2) `bot_token: ""` (빈 문자열) → reject (의도 불명확).

---

## AC-MTGM-009 — Streaming UX (REQ-MTGM-E02, REQ-MTGM-S05)

**Given**:
- AC-MTGM-002 완료.
- yaml `default_streaming: true` 또는 사용자 메시지가 `/stream Hello` 형식.
- BRIDGE-001 streaming RPC 가 chunk 단위 응답 반환 (SSE).

**When**:
1. 사용자 `/stream 긴 답변 부탁` 전송.
2. handler 가 streaming branch 진입 → placeholder `sendMessage("...")` 호출.
3. BRIDGE-001 streaming chunk 수신마다 buffer 누적.
4. 1초 ticker tick 시 buffer 가 비어있지 않으면 editMessageText 호출.
5. streaming 종료 시 final flush.

**Then**:
- 첫 chunk 표시 (placeholder 첫 edit) < 1.5초.
- 사용자가 답변이 점진적으로 채워지는 것을 봄 (1초 간격).
- 최종 메시지가 완성된 답변.
- AUDIT-001 entry 에 `streaming: true, edit_count: <n>, total_duration_ms: <>` 기록.

**Streaming 중 다른 inbound 큐 (REQ-MTGM-S05)**:
- 같은 chat_id 로 streaming 중 추가 메시지 도착 → FIFO 큐 적재 (max 5).
- streaming 완료 후 큐의 다음 메시지 처리.
- 큐 가득 시 사용자에게 "이전 응답 진행 중, 잠시 후 다시 시도" 응답 + 큐 미적재.

**Edge Cases**:
- E1) editMessageText rate limit 초과 (Telegram 측 429) → backoff + 다음 ticker 까지 대기.
- E2) streaming 중간에 BRIDGE-001 disconnect → buffer flush + audit `streaming_aborted`.

---

## AC-MTGM-010 — Markdown V2 + Inline Keyboard 렌더 (REQ-MTGM-E03, REQ-MTGM-E05)

**Given**:
- AC-MTGM-005 완료.
- agent 가 inline keyboard 포함 outbound tool 호출.

**When**:
1. agent 가 `telegram_send_message({chat_id: "<C>", text: "선택하세요", inline_keyboard: [[{text: "옵션 A", callback_data: "opt_a"}, {text: "옵션 B", callback_data: "opt_b"}]]})` 호출.
2. sender 가 sendMessage with reply_markup (inline keyboard) 호출.
3. 사용자가 "옵션 A" 버튼 클릭.
4. poller 가 callback_query update 수신.
5. handler 가 `answerCallbackQuery` (toast 표시) + callback_data="opt_a" 를 BRIDGE query 본문으로 변환 → query 호출.

**Then**:
- 사용자 Telegram 에 "선택하세요" 메시지 + 두 버튼 표시.
- 사용자 클릭 시 toast (또는 alert) 표시.
- BRIDGE-001 가 `selected: opt_a` 컨텍스트로 query 호출됨.
- AUDIT-001 에 callback_query entry 기록 (`callback_data: opt_a`).

**Markdown V2 escape 검증** (markdown_test.go):
- 18개 reserved chars (`_ * [ ] ( ) ~ \` > # + - = | { } . !`) 모두 `\` escape 적용 (Telegram MarkdownV2 spec §5).
- 의도된 markdown (`*bold*`) 은 보존.
- 사용자 입력 본문 (raw text) 이 markdown 으로 잘못 해석되지 않음.

**Edge Cases**:
- E1) 다단 inline keyboard (2단 이상) → 1단으로 flatten 또는 tool error `multi_row_keyboard_unsupported` (OUT-9 정합).
- E2) callback_data > 64 bytes (Telegram 제약) → tool error `callback_data_too_long`.
- E3) callback_query 가 60초 후 도착 → REQ-MTGM-N04 에 따라 audit `callback_expired` + 무시.

---

## AC-MTGM-011 — Audit log 무결성 + PII protection 통합 검증 (REQ-MTGM-U01, REQ-MTGM-N06)

**Given**:
- AC-MTGM-002 + AC-MTGM-005 완료 (inbound + outbound 메시지 모두 흐른 후).
- AUDIT-001 가 `~/.goose/audit/<date>.jsonl` (또는 동등 store) 에 기록 완료.

**When**:
1. `goose audit query --source messaging.telegram --since <today>` 실행 (또는 동등 audit 조회 메커니즘).
2. 결과 JSONL entry 들을 `audit_test.go` 가 파싱.

**Then**:
- 모든 entry 는 다음 필드를 보유:
  - `ts`, `source: "messaging.telegram"`, `direction (in|out)`, `chat_id`, `message_id`, `user_profile_id`, `content_hash (64자 hex)`, `streaming_flag (bool)`, `tool_call_id (out 일 때만)`.
- `content_hash` 가 SHA-256 hex (64자) 이며 같은 본문에 대해 결정적 (다른 entry 와 충돌 시 본문 동일성 추정 가능).
- entry 직렬화에 본문 raw text **포함되지 않음** (PII protection).
- entry 에 다른 사용자의 user_profile_id 또는 식별정보 **포함되지 않음** — `chat_id` + `user_profile_id` 는 본 메시지 송신자 본인 것만.
- 메시지 처리 (inbound query) 가 AUDIT-001 write fail 후에도 **계속 진행** (audit 실패가 채널 막지 않음 — plan.md §3.4 정책).

**append-only 무결성 검증**:
- `audit_test.go` 가 두 시점 사이 entry count 가 monotonically increasing 인지 확인.
- entry 의 ts 가 monotonic (시간 역행 없음).
- entry 직렬화 후 file size 가 항상 증가 (overwrite 없음).

**Edge Cases**:
- E1) AUDIT-001 disk full → poller 는 메시지 처리 계속 (audit fail 로깅), 회복 후 audit 재개.
- E2) 본문 hash 충돌 (이론상 SHA-256 충돌) → audit 자체는 정상, 별도 처리 없음 (이력 추적 시 본문 동일성 불확실).

---

## 11. Definition of Done (DoD) Checklist

- [ ] 11 개 AC 모두 GREEN.
- [ ] coverage ≥ 85% (`go test ./internal/messaging/telegram/...`).
- [ ] Markdown V2 escape unit test 18자 (`_ * [ ] ( ) ~ \` > # + - = | { } . !`) 전수 통과.
- [ ] mock httptest 기반 integration test (setup → polling → query → response → audit) 통과.
- [ ] keyring atomic write/read 검증 (3개 OS 또는 fallback 환경변수).
- [ ] `@MX:TODO` 0개, ANCHOR/NOTE/WARN/REASON 모두 적용.
- [ ] golangci-lint clean, gofmt clean (CLAUDE.local.md §1.3 required status check 통과).
- [ ] CLAUDE.local.md §2.5 영문 주석 준수 (신규 코드).
- [ ] CHANGELOG.md entry (Phase 별 PR squash merge 시 release-drafter 자동 분류 — `type/feature` + `area/messaging` label).
- [ ] plan-auditor 1라운드 PASS → status 자동 audit-ready.
- [ ] 수동 smoke test — 실제 Telegram bot 등록 후 5분 setup → echo → query 검증 (Phase 1, 3 종료 시 각 1회).

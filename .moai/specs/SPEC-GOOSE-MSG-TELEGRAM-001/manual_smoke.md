# SPEC-GOOSE-MSG-TELEGRAM-001 P1 — 수동 검증 가이드 (Manual Smoke Test)

P1 Foundation 완료 후 5분 SLO 내에 아래 절차로 수동 검증을 수행합니다.

---

## 사전 준비

1. Telegram 앱에서 **@BotFather** 에게 `/newbot` 명령 전송
2. 안내에 따라 봇 이름 및 username 입력 → HTTP API 토큰 발급 (`123456:ABC-DEF1234...` 형식)
3. 토큰을 클립보드에 복사해 두기

---

## Step 1 — 봇 토큰 설정 (setup)

```bash
goose messaging telegram setup --token "<발급받은_토큰>"
```

**기대 결과**:
- `bot username: @<봇_username>` 출력
- `Telegram 에서 @<봇_username> 에게 /start 보내세요` 안내 출력
- `~/.goose/messaging/telegram.yaml` 파일 생성 확인:
  ```bash
  cat ~/.goose/messaging/telegram.yaml
  # bot_username 이 출력되어야 하며, bot_token 필드는 없어야 함
  ```
- 키링(macOS Keychain)에 `goose-messaging / telegram.bot.token` 항목 저장 확인 (선택)

**SLO 측정 시작 시각**: ___________

---

## Step 2 — 상태 확인 (status)

```bash
goose messaging telegram status
```

**기대 결과**: `configured` 출력

---

## Step 3 — 포어그라운드 시작 (start)

```bash
goose messaging telegram start
```

**기대 결과**: 프로세스가 foreground에서 실행되며 polling 루프 진입. 로그 출력 없으면 정상.

---

## Step 4 — Echo 검증

1. Telegram 앱에서 위 봇(@봇_username)에게 메시지 전송: `Hello`
2. 봇이 `Hello` 를 그대로 응답하면 P1 echo 동작 확인 완료

**기대 결과**: 봇이 동일 텍스트 `Hello` 를 즉시 (수초 내) 응답

---

## Step 5 — 종료

`Ctrl+C` 로 foreground 프로세스 종료. 정상 종료 확인.

---

## Step 6 — 재설정 후 중복 방지 확인

```bash
goose messaging telegram setup --token "<동일_토큰>"
```

**기대 결과**: `already configured` 에러 메시지 출력 (E3 거부)

---

## SLO 측정 결과

| 항목 | 시각 | 소요 |
|------|------|------|
| 시작 | ___ | — |
| setup 완료 | ___ | ___ |
| start + echo 수신 | ___ | ___ |
| **총 소요** | — | **≤ 5분** |

5분 초과 시 UX 개선 이슈로 등록 (AC-MTGM-001 SLO 위반).

---

## 검증 체크리스트

- [ ] `setup --token <T>` 성공 → keyring 저장 + yaml 생성
- [ ] yaml 에 `bot_token` 필드 없음 (REQ-MTGM-N01 준수)
- [ ] `start` 포어그라운드 실행 후 Telegram echo 응답 수신
- [ ] `setup` 재실행 시 "already configured" 에러 반환 (E3)
- [ ] 총 소요 시간 5분 이내 (AC-MTGM-001 SLO)

---

# P4 — Streaming + Webhook + Polish 수동 검증 (2026-05-09 추가)

P4 6 task (T1 streaming / T2 webhook / T3 silent+typing / T4 streaming queue / T5 golden output / T6 manual smoke) 완료 후 실제 봇 환경에서 다음 시나리오를 수동 검증한다. 코드 단위 검증은 자동 테스트로 끝났으므로, 이 절은 사용자 UX 와 외부 의존(TLS / ngrok / Telegram client) 동작 확인만 다룬다.

## P4 시나리오 1 — Streaming UX (AC-MTGM-009)

### 사전 조건
- P3 까지 setup + first chat_id 매핑 완료
- `~/.goose/messaging/telegram.yaml` 의 `default_streaming` 가 `false` (default)
- `goosed` daemon 실행 중 + ChatService streaming 가용

### 절차
1. Telegram 에서 봇에게 `/stream 짧은 시 한 편 써주세요` 전송
2. 봇이 `...` placeholder 즉시(< 1.5초) 표시 확인
3. 시간 흐름에 따라 placeholder 본문이 점진 업데이트되는 것 관찰 (1초 윈도우)
4. 응답 종료 시 최종 본문이 placeholder 자리에 보존
5. `goose audit query --source messaging.telegram --since 5m` 으로 outbound entry 의 `streaming_flag=true` + `edit_count>=1` + `total_duration_ms` 확인

### `default_streaming: true` 변형
- yaml `default_streaming: true` 로 수정 후 daemon 재기동
- `/stream` 접두 없이 평소 메시지 → 자동 streaming 적용 확인

### 기대 결과
- 첫 placeholder 표시 P95 < 1.5초 (REQ-MTGM-E02)
- 응답 중간 4096자 초과 시 split (Telegram 제약) — best-effort
- audit 에 raw 본문 포함되지 않음 (content_hash 만, REQ-MTGM-N06)

---

## P4 시나리오 2 — Streaming Queue 가득(REQ-MTGM-S05)

### 절차
1. yaml `default_streaming: true` 활성
2. 봇에게 메시지 6개 빠르게 연속 전송 (각 메시지 사이 < 0.5초)
3. 첫 메시지가 streaming 진행 중인 동안 2~5번째는 큐 적재
4. 6번째 메시지 도착 시 `이전 응답 진행 중, 잠시 후 다시 시도하세요.` 안내 응답 수신 확인

### 기대 결과
- 1번째 streaming 종료 후 2~5번째가 FIFO 순으로 처리
- 6번째는 응답 없음 (drop) + audit 에 `stream_queue_dropped: true` 기록

---

## P4 시나리오 3 — Silent default + Typing indicator

### Silent default (REQ-MTGM-O01)
1. yaml `silent_default: true` 추가, daemon 재기동
2. 봇이 보내는 모든 메시지 (echo / agent 응답 / 안내)가 사용자 폰 알림 없이 도착하는지 확인 (vibration + sound 없음, 시각적으로만 표시)

### Typing indicator (REQ-MTGM-O02)
1. yaml `typing_indicator: true` 추가, daemon 재기동
2. 봇에게 일반 메시지 전송
3. agent 응답 도착 전까지 Telegram client 상단에 `typing...` 표시 5초 간격으로 갱신되는지 확인
4. 응답 도착 또는 ctx 종료 시 indicator 사라짐

---

## P4 시나리오 4 — Webhook mode + TLS fallback (REQ-MTGM-E07)

### 사전 조건
- ngrok 또는 동등한 TLS 터널 도구 설치
- BRIDGE-001 HTTP mux 가 webhook endpoint 받을 수 있는 상태

### Happy path (TLS 있음)
1. ngrok 으로 로컬 BRIDGE 포트를 HTTPS 로 expose: `ngrok http 8080`
2. yaml 에 `mode: webhook` + `webhook.public_url: "https://<ngrok-id>.ngrok-free.app"` + `webhook.secret: "<32 hex>"` 작성
   - secret 미작성 시 daemon 가 자동 생성
3. daemon 재기동
4. log 에 `telegram webhook registered path=/webhook/telegram/<secret> public_url=https://...` 출력 확인
5. 봇에게 메시지 전송 → POST 가 ngrok 경유로 BRIDGE mux 로 전달, 봇이 응답
6. `goose messaging telegram status` 출력에 `mode: webhook` 표시

### Fallback path (TLS 없음 또는 setWebhook 실패)
1. yaml `mode: webhook` 유지하되 `public_url` 을 의도적으로 invalid (`http://localhost:9999`) 로 설정
2. daemon 기동
3. log 에 `webhook registration failed, falling back to polling` warning + polling 루프 정상 동작 확인
4. `goose messaging telegram status` 출력에 `mode: polling (fallback from webhook)` 표시 (구현 시점에 따라 메시지 wording 변동 허용)

### Fail-fast 옵션
1. yaml `webhook.fallback_to_polling: false` 명시
2. invalid URL 로 daemon 기동 → daemon 자체가 messaging 모듈만 비활성 + ERROR 로그 + daemon 본체는 정상 동작
3. 회복: yaml 수정 후 daemon 재기동

---

## P4 시나리오 5 — Markdown V2 + Inline Keyboard regression

### 절차
- agent 가 outbound 로 `**bold**` (escape 필요) + 한국어 + 이모지 + 1단 inline keyboard 포함 메시지 호출
- Telegram client 에서 bold/이탤릭 정상 렌더 + 버튼 클릭 가능 + 클릭 시 callback_query 처리 확인
- testdata/inline_keyboard 의 5 fixture 와 실제 봇 표현이 일치하는지 시각 검증

### 기대 결과
- 18 reserved chars 모두 escape 적용되어 평문 표현 (raw chars 가 markdown syntax 로 잘못 해석되지 않음)
- 다단(2 row 이상) inline keyboard 호출 시 1단으로 flatten 또는 tool error `multi_row_keyboard_unsupported` (OUT-9 정합)

---

## P4 검증 체크리스트

- [ ] AC-MTGM-009 streaming UX — `/stream` 접두 + `default_streaming: true` 양쪽 PASS
- [ ] REQ-MTGM-S05 streaming queue — FIFO max 5 + apology drop 확인
- [ ] REQ-MTGM-O01 silent_default — outbound 알림 무음
- [ ] REQ-MTGM-O02 typing_indicator — 5초 간격 indicator 갱신
- [ ] REQ-MTGM-E07 webhook fallback — TLS 없음/setWebhook 실패 시 polling 자동 전환
- [ ] webhook happy path — ngrok TLS 환경에서 정상 동작
- [ ] Markdown V2 18 reserved chars 시각 회귀 — testdata fixture 와 실제 봇 일치
- [ ] inline keyboard 1단 클릭 → callback_query 처리

위 체크 모두 PASS 시 P4 sync phase 진입 가능.

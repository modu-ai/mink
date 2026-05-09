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

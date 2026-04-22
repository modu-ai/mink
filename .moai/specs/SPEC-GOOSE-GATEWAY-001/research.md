# SPEC-GOOSE-GATEWAY-001 — Research Notes

> Hermes gateway/ 매핑 상세, 플랫폼별 제약, 한국/중국 특화, BRIDGE와의 차별점.

---

## 1. Hermes gateway/ → GOOSE 포팅 매핑 (확정)

| Hermes (Python) | GOOSE (Go) | 비고 |
|----------------|-----------|-----|
| `gateway/platform.py` (base class) | `internal/gateway/platform.go` (interface) | duck typing → interface |
| `gateway/telegram.py` | `internal/gateway/telegram/bot.go` | telegram-bot-api v5 |
| `gateway/discord.py` | `internal/gateway/discord/bot.go` | discordgo |
| `gateway/slack.py` | `internal/gateway/slack/bot.go` | slack-go |
| `gateway/matrix.py` | `internal/gateway/matrix/bot.go` | mautrix-go |
| `gateway/kakao.py` | `internal/gateway/kakao/client.go` | Kakao 비즈니스 API |
| `gateway/wechat.py` | `internal/gateway/wechat/client.go` | silenceper/wechat v2 |
| `gateway/webhook.py` | `internal/gateway/webhook/handler.go` | stdlib http |
| `gateway/router.py` | `internal/gateway/router.go` | user mapping + dispatch |
| `gateway/ratelimit.py` | `internal/gateway/ratelimit/limiter.go` | 공통 token bucket |
| `gateway/redactor.py` | `internal/gateway/redactor/pii.go` | 공통 PII masking |

---

## 2. 플랫폼별 제약 & 설계 요점

### 2.1 Telegram

- **인증**: Bot token (BotFather 생성)
- **수신**: Long polling (getUpdates) 또는 Webhook (HTTPS 필요)
- **제한**: Bot당 초당 30 메시지, 그룹당 20 (2026 기준)
- **특수**: Inline keyboard 버튼, 이미지 첨부 → 1차 out
- **개인정보**: user_id, 이름, username 수신

### 2.2 Discord

- **인증**: Bot token + Gateway intent
- **수신**: WebSocket Gateway 상시 연결
- **제한**: 초당 50 message, 채널당 제약
- **특수**: 서버 membership 필요, slash commands + application commands

### 2.3 Slack

- **인증**: OAuth + Bot user token, Signing secret
- **수신**: Events API (HTTPS webhook) 또는 Socket Mode (WebSocket)
- **제한**: 초당 1 메시지 per channel (Tier)
- **특수**: **3초 acknowledgement 의무**. 답변이 길면 deferred response 사용.
- **HMAC**: `x-slack-signature` 헤더 검증 필수.

### 2.4 Matrix

- **인증**: 사용자별 access token
- **수신**: Long polling `/sync` API
- **제한**: 서버별 상이
- **특수**: E2EE 방(Megolm) 참여 시 추가 키 관리. mautrix-go가 처리.
- **장점**: 자체 호스팅 가능, 프라이버시 강함

### 2.5 카카오톡 알림톡 (Kakao Business)

- **인증**: 카카오 비즈니스 계정 + API key + 발신프로필 ID
- **수신**: **없음** (단방향 발신 전용)
- **제한**: 초당 30-300 (계약 등급)
- **특수**:
  - **승인된 템플릿만 발송 가능** (카카오 사전 심사)
  - 템플릿 파라미터 치환 (`#{name}`, `#{time}` 등)
  - 실패 시 SMS 대체 발송 옵션
- **API endpoint**: `https://api.solapi.com/messages/v4/send` (aligo/솔라피 등 벤더 경유 일반적)

### 2.6 WeChat

- **인증**: AppID + AppSecret (개발자 계정)
- **수신**: XML webhook
- **계정 종류**:
  - 구독계정(订阅号): 1일 1회 발송 제한
  - 서비스계정(服务号): 월 4회 템플릿 메시지
  - 미니프로그램: 별도
- **1차 스코프**: 서비스 계정 템플릿 메시지만

### 2.7 Generic Webhook

- 임의 시스템(Zapier, n8n, 자체 서버)이 HTTP POST로 GOOSE 호출
- 양방향: inbound(POST /gateway/webhook/{id}) + outbound(사용자 지정 URL로 POST)
- HMAC-SHA256 서명으로 인증

---

## 3. 한국 시장 — 카카오톡 상세 고려

### 3.1 왜 카카오톡인가

- 국민 MAU ~4500만
- 문자/알림 대체 1순위
- 공공기관·은행·병원 알림 표준

### 3.2 알림톡 vs 친구톡 vs 상담톡

| 종류 | 용도 | 제약 |
|-----|-----|-----|
| 알림톡 | 정보성 알림 | 승인 템플릿, 80자 제한 |
| 친구톡 | 마케팅 | 채널 친구 추가 필수, 광고 표시 의무 |
| 상담톡 | 양방향 상담 | 운영자 필수, 인력 비용 |

**1차 스코프**: 알림톡만. GOOSE는 정보성 알림(브리핑, 리추얼 리마인더)이 주용도라 맞음.

### 3.3 발송 벤더

- 직접 카카오 비즈니스 계약 (사업자 필요)
- 솔라피(Solapi), 알리고(Aligo) 등 메시지 API 업체 경유 (개인도 가능)
- 1차는 **Solapi 경유** 지원. 사용자가 Solapi API key를 설정에 입력.

### 3.4 템플릿 예시

```
[GOOSE] 오늘의 브리핑
#{name}님, 좋은 아침이에요.
오늘 일정: #{schedule}
먼저 할 일: #{priority_task}
```

사전에 카카오 승인 받은 템플릿 ID를 설정에 등록 → params만 런타임 치환.

---

## 4. 중국 시장 — WeChat 고려

- 중국 사용자 접근 필수
- 단, 중국 내 클라우드 호스팅 및 ICP 신고 필요 → 1차는 **옵션 스텁만** 구현, 실제 배포는 중국 현지 파트너 필요
- 서비스 계정 자격 요건: 중국 법인 필수

**결정**: 1차는 인터페이스 + 스텁만. 실제 배포는 v2+.

---

## 5. User Mapping 스키마

```sql
CREATE TABLE user_mappings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  goose_user_id TEXT NOT NULL,
  platform TEXT NOT NULL,         -- 'telegram', 'slack', ...
  platform_user_id TEXT NOT NULL, -- 플랫폼 user/member ID
  display_name TEXT,              -- 사용자 친화 표시
  scopes TEXT NOT NULL,           -- JSON: ["read", "notify"]
  linked_at INTEGER NOT NULL,
  last_active_at INTEGER,
  UNIQUE(platform, platform_user_id)
);

CREATE INDEX idx_goose_user ON user_mappings(goose_user_id);
```

링크 플로우:

1. 사용자가 Desktop/Mobile 설정에서 "Telegram 연결" 버튼
2. Gateway가 일회용 deep link 발급 (`t.me/GooseBot?start=<link_token>`)
3. Telegram에서 해당 링크 클릭 → bot이 link_token 수신
4. link_token으로 goose_user_id 확인 → 매핑 등록

---

## 6. BRIDGE vs GATEWAY 차별점 재정리

| 항목 | BRIDGE-001 (Mobile 퍼스트파티) | GATEWAY-001 (메신저 서드파티) |
|-----|-------------------------------|------------------------------|
| 세션 개념 | 정식 QueryEngine 세션 | 메시지 단위 request-response |
| 스트리밍 | 청크 단위 실시간 | 완성된 메시지만 |
| 첨부 | 이미지·파일 지원 | 1차는 텍스트만 |
| 권한 콜백 | Mobile에서 승인 UI | 불가 (escalate to Desktop/Mobile) |
| 암호화 | Noise E2EE | 플랫폼 고유 (대부분 TLS만, Matrix는 E2EE 옵션) |
| 전송 | WS/SSE/Polling | 각 플랫폼 API |
| 사용자 수 | 1 user, 1 device | 그룹·팀 채널 지원 |

---

## 7. PII Redactor 정책

### 7.1 자동 마스킹 대상

- 이메일 주소 (`[email redacted]`)
- 전화번호 (`[phone redacted]`)
- 신용카드 번호 형식 (`[card redacted]`)
- 주민등록번호 (한국)
- 다른 user_mapping에 등록된 display_name (타 사용자 정보 유출 방지)

### 7.2 ML 기반 NER은 out

- 1차는 정규표현식만
- 향후 로컬 NER 모델 (전각 이름 탐지 등) 검토

---

## 8. Rate Limiting 설계

```go
// internal/gateway/ratelimit/limiter.go
type Limiter interface {
    Allow(platform PlatformKind, channelID string) bool
    Wait(ctx context.Context, platform PlatformKind, channelID string) error
    Report(platform PlatformKind) Budget
}

type Budget struct {
    Remaining  int
    ResetAt    time.Time
    Queued     int
}
```

Token bucket per (platform, channel). 플랫폼별 기본값 사전 등록.

---

## 9. 보안 고려

- OAuth 토큰은 OS 시크릿 스토어 (go-keyring, darwinkit 경유)
- Webhook signing secret은 환경변수 or config file
- 플랫폼별 webhook URL은 HTTPS 필수 (dev는 ngrok 권장)
- 외부 메시지는 **절대 tool 실행 권한 없음** (REQ-GW-004)

---

## 10. TDD 전략 상세

### 10.1 단위

- `platform/contract_test.go`: 모든 Platform 구현체가 공통 계약 충족 (사용: reflection + table-driven)
- `telegram/bot_test.go`: httptest mock server
- `slack/hmac_test.go`: 벡터 기반 검증
- `kakao/template_test.go`: 승인 템플릿 검증
- `ratelimit/bucket_test.go`: burst + sustained rate
- `redactor/pii_test.go`: 패턴 매칭 edge cases

### 10.2 통합

- `integration/telegram_live_test.go` (skip unless TELEGRAM_TOKEN set)
- Slack sandbox workspace + bot token (옵션)
- 카카오는 mock (실제 API는 비용 발생)

### 10.3 퍼지

- `fuzz_webhook_test.go`: 임의 payload
- `fuzz_redactor_test.go`: PII 탐지 robustness

### 10.4 커버리지 목표

- 공통 레이어 (platform, router, ratelimit, redactor): 90%+
- 플랫폼 어댑터: 75%+ (외부 API mock 한계)

---

## 11. 오픈 이슈

1. **카카오 Solapi vs 직접 계약**: 1차는 Solapi. 기업 전환 시 직접 계약 검토.
2. **Slack Enterprise Grid**: 대기업 워크스페이스는 별도 manifest 필요. 1차는 standard workspace만.
3. **Discord Verified Bot**: 100+ 서버 참여 시 verification 필요. 처음엔 unverified.
4. **Matrix homeserver**: 공식 matrix.org vs self-hosted(dendrite/synapse). 사용자 자유 선택.
5. **WeChat 심사**: 중국 법인 필수. 오픈소스 패키지만 제공하고 실제 배포는 사용자 책임.
6. **Multi-account**: Telegram 여러 계정(개인+업무) 동시 연결 시 세션 격리. REQ-GW-012로 명시.
7. **팀 공유 봇 vs 개인 봇**: 1차는 개인 1:1. 팀 공유는 v2+.
8. **알림톡 비용**: 건당 ~10원 (2026 기준). 예산 한도 설정 UI 필요.

---

## 12. 참조

- Hermes `gateway/` (Python) 구현 분석
- Telegram Bot API: <https://core.telegram.org/bots/api>
- Slack API: <https://api.slack.com/>
- Discord Developer Portal: <https://discord.com/developers/docs>
- Matrix Spec: <https://spec.matrix.org/>
- Kakao 알림톡 개발자 가이드 (SOLAPI)
- WeChat Official Account Platform API
- structure.md §1 `internal/gateway/`, §4 모듈 책임 매트릭스

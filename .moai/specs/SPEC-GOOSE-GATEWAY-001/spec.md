---
id: SPEC-GOOSE-GATEWAY-001
version: 0.2.0
status: Planned (재정의)
created: 2026-04-21
updated: 2026-04-22
author: manager-spec
priority: P1
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-GATEWAY-001 — **Self-hosted Messenger Bridge (umbrella)** ★ v6.2 재정의

> ⚠️ **v6.2 재정의 알림 (2026-04-22)**
>
> 본 SPEC은 ROADMAP v6.2에 따라 **전면 재정의**되었습니다. 본문은 M6 진입 시 manager-spec subagent가 완전 재작성합니다.
>
> **스코프 변경**:
> - **포함 (v1.0~1.2)**: Tier A **Self-hosted long-polling** 5종
>   - `SPEC-GOOSE-GATEWAY-TG-001` (Telegram, v1.0)
>   - `SPEC-GOOSE-GATEWAY-DC-001` (Discord, v1.1)
>   - `SPEC-GOOSE-GATEWAY-SL-001` (Slack, v1.1)
>   - `SPEC-GOOSE-GATEWAY-MX-001` (Matrix, v1.2)
>   - `SPEC-GOOSE-GATEWAY-SG-001` (Signal, v1.2)
> - **분리 (v2.0+, 사업자 전제)**: Tier B **Business Registration 필수**
>   - KakaoTalk (한국 사업자), WeChat (중국 법인), LINE (일본 법인), SMS (Twilio) → 별도 SPEC
> - **제외**: 범용 Webhook은 GATEWAY-001 범위 외, 별도 사용자 가이드로 이관
>
> **핵심 재정의**:
> - 본 SPEC은 `MessengerAdapter` 인터페이스 정의 + 공통 보안 계층 + 관리 UI만 담당 (umbrella)
> - 플랫폼별 구체 구현은 위 5개 하위 SPEC에 분리
> - **클라우드 0, 계정 0**: PC에서 outbound long-polling, NAT 뒤에서도 동작
> - **Channel HARD rule** 준수: Journal/Health/Identity Graph 본문 송출 차단 (Signal/Matrix E2EE opt-in 제외)
>
> **왜 변경되었는가**:
> 1. Hermes 모델이 Daily Companion 카테고리와 충돌 (2차 분석) → 폐기 고려
> 2. 재평가 결과 "메신저를 PC GOOSE의 원격 리모컨으로 쓰는 패턴"은 여전히 가치 있음 → 재도입
> 3. 단, 사업자 등록 필요 플랫폼(Kakao/WeChat/LINE/SMS)은 v2.0+로 분리하여 v1.0 범위 확정

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.2.0 | 2026-04-22 | ROADMAP v6.2 재정의. umbrella SPEC으로 전환. Tier A 5종(TG/DC/SL/MX/SG)만 포함, Tier B(Kakao/WeChat/LINE/SMS)는 v2.0+ 별도 SPEC 분리. Self-hosted long-polling 강조, 클라우드 0 원칙. Channel HARD rule 명시. | 세션 결정 |
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 신규 SPEC. Hermes `gateway/` 패턴을 Go로 이식. Telegram / Discord / Slack / Matrix / 카카오톡(알림톡) / WeChat / 일반 Webhook 봇을 통해 메신저에서 GOOSE 호출. 사용자 옵트인, 한국 시장은 카카오톡 우선. | manager-spec |

---

## 1. 개요 (Overview)

Gateway는 주요 메신저 플랫폼을 **GOOSE의 부가 인터페이스**로 열어주는 게이트웨이 서비스다. 사용자는 자신의 Telegram, Discord, Slack 등 계정에서 봇을 친구로 추가하거나 워크스페이스에 초대하면, 메신저에서 텍스트로 GOOSE에게 질문할 수 있다. GOOSE는 응답을 같은 메신저 채널로 돌려준다.

본 SPEC은 Hermes `gateway/` (Python) 패턴을 Go로 재구현한다:
- Telegram Bot API
- Discord Bot API
- Slack Bot API (Slash commands + Events)
- Matrix Protocol
- **카카오톡 알림톡** (한국 시장)
- WeChat Official Account (중국 시장, 옵션)
- Generic Webhook (임의 시스템 통합)

모든 플랫폼 통합은 **opt-in**이다. 기본 설치에서는 비활성. 사용자가 명시적으로 OAuth + 채널 연결 수행해야 활성화된다. 개인정보 수집 최소 원칙을 지킨다.

---

## 2. 배경 (Background)

### 2.1 왜 Gateway가 필요한가

- 사용자는 일상에서 메신저를 가장 많이 쓴다. GOOSE Desktop/Mobile을 열지 않고도 "카톡으로 알림 받기" 요구 존재.
- 팀 협업 시나리오: Slack 워크스페이스에서 "`@goose 이슈 #123 정리해줘`" 호출.
- 한국 시장: 카카오톡 알림톡은 일일 국민 ~4500만 사용자에게 도달 가능.
- 중국 시장: WeChat 없이는 실질적 접근 불가.
- Hermes가 동일 패턴 구현 완료 → 검증된 설계 재사용.

### 2.2 왜 BRIDGE-001과 별도인가

- **BRIDGE-001**: 퍼스트파티 Mobile 앱 전용, 전체 세션 제어(권한·첨부·스트리밍)
- **GATEWAY-001**: 서드파티 메신저, 단순 request-response 패턴(메신저 UX 제약)

예컨대 Telegram에서는 첨부파일 업로드 & 권한 콜백 UI가 복잡하므로, Gateway는 **간단한 Q&A·알림 수신**으로 스코프 축소.

### 2.3 Hermes gateway/ 매핑

| Hermes gateway/ | GOOSE Go 포팅 |
|----------------|---------------|
| `telegram.py` | `internal/gateway/telegram/bot.go` |
| `discord.py` | `internal/gateway/discord/bot.go` |
| `slack.py` | `internal/gateway/slack/bot.go` |
| `matrix.py` | `internal/gateway/matrix/bot.go` |
| `kakao.py` | `internal/gateway/kakao/client.go` (알림톡) |
| `wechat.py` | `internal/gateway/wechat/client.go` |
| `webhook.py` | `internal/gateway/webhook/handler.go` |
| `platform.py` (abstract) | `internal/gateway/platform.go` |

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/gateway/` 패키지 + platform별 서브패키지.
2. **공통 `Platform` 인터페이스**: 플랫폼 독립적 메시지 수신/발신 추상.
3. **플랫폼 어댑터 7종**:
   - Telegram Bot (long polling + webhook 모드)
   - Discord Bot (discordgo)
   - Slack Bot (slack-go, socket mode + events API)
   - Matrix (matrix-go-sdk)
   - 카카오톡 알림톡 (비즈니스 API, 서버→사용자 알림만)
   - WeChat Official Account (구독/서비스 계정 단계적)
   - Generic Webhook (임의 시스템이 inbound/outbound HTTP)
4. **OAuth 2.0 연결 플로우**: 플랫폼별, Desktop/Mobile 설정에서 시작.
5. **메시지 라우팅**: Gateway가 수신한 메시지 → `goosed` QueryEngine 세션에 주입 → 응답을 플랫폼으로 전송.
6. **사용자 계정 매핑**: Platform user ID ↔ GOOSE user ID 매핑(SQLite).
7. **권한·스코프**:
   - 기본적으로 Gateway로 들어온 요청은 **읽기·간단 답변**만
   - Tool 실행(파일 쓰기 등)은 거부(메신저에 권한 승인 UX 없음)
8. **Rate limit per platform**: 플랫폼별 API 한도 준수.
9. **프라이버시**:
   - 플랫폼 토큰은 Keystore/Secret Service에 저장
   - 메시지 로그 기본 OFF (debug만 opt-in)
   - 개인정보 필드 마스킹
10. **관찰성**: 플랫폼별 in/out 메트릭, 에러율, 지연.

### 3.2 OUT OF SCOPE

- **음성/비디오 통화 (Telegram Voice, Discord Voice)**: v2+.
- **파일 첨부 업·다운로드**: 1차는 텍스트만.
- **봇 커스터마이즈(페르소나)**: 1차는 표준 GOOSE. 커스텀은 이후.
- **멀티 테넌트 배포**: 각 사용자 self-hosted 혹은 단일 공식 인스턴스. SaaS 상용 모델은 별도.
- **카카오톡 일반 챗봇 (채널)**: 알림톡만. 양방향 챗봇은 카카오 i 오픈빌더 연동 별도.
- **LINE, Viber, Signal**: v2+.
- **SMS/MMS**: 전송 비용 문제로 out.

---

## 4. 의존성 (Dependencies)

- **상위 의존**: SPEC-GOOSE-BRIDGE-001(공통 QueryEngine 세션 접근 패턴 참조), SPEC-GOOSE-MCP-001(Gateway가 MCP tool로 등록되어 호출될 수도).
- **비의존(부모 아님)**: Gateway는 Bridge 없이도 독립 동작 가능. 하지만 관찰·권한 정책은 동일 원칙 준수.
- **라이브러리**:
  - `github.com/go-telegram-bot-api/telegram-bot-api/v5` 5.x
  - `github.com/bwmarrin/discordgo` 0.28+
  - `github.com/slack-go/slack` 0.14+
  - `maunium.net/go/mautrix` 0.20+ (Matrix)
  - 카카오 알림톡: 비즈니스 API HTTP 호출(표준 http.Client)
  - WeChat: `github.com/silenceper/wechat/v2`
  - `github.com/golang-jwt/jwt/v5` (webhook 인증)

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-GW-001**: The Gateway **shall** register each platform adapter as an implementation of a shared `Platform` interface.
- **REQ-GW-002**: The Gateway **shall** store platform OAuth tokens only in the OS secret store (Keychain / Secret Service / Credential Manager).
- **REQ-GW-003**: The Gateway **shall** record platform user IDs in a SQLite table mapped to GOOSE user IDs, and all message routing **shall** require a valid mapping.
- **REQ-GW-004**: The Gateway **shall** never execute tools that modify local filesystem or external state based solely on a platform message — such requests must be escalated to Desktop/Mobile approval.

### 5.2 Event-Driven

- **REQ-GW-005**: **When** a Telegram message arrives from a mapped user, the Gateway **shall** forward the text to the associated `goosed` session and stream the response back as text messages.
- **REQ-GW-006**: **When** a Slack slash command `/goose` is invoked, the Gateway **shall** respond within 3 seconds with either the answer or a deferred-response placeholder, then follow up with the full answer.
- **REQ-GW-007**: **When** the Discord bot is mentioned in a channel where it is a member, the Gateway **shall** treat the mention as a request and respond in the same channel.
- **REQ-GW-008**: **When** `goosed` emits a notification targeted at a user with a configured KakaoTalk notification channel, the Gateway **shall** send the approved 알림톡 template to the user.

### 5.3 State-Driven

- **REQ-GW-009**: **While** a platform adapter is disconnected (e.g., WebSocket closed, token expired), the Gateway **shall** attempt reconnection with exponential backoff and refresh the OAuth token if expired.
- **REQ-GW-010**: **While** the per-platform rate limit budget is exhausted, the Gateway **shall** queue outgoing messages in memory (capped at 1000 per platform) and release them as budget becomes available.

### 5.4 Optional

- **REQ-GW-011**: **Where** the user enables debug mode, the Gateway **shall** log full inbound/outbound message text to a local file with automatic 7-day rotation — off by default.
- **REQ-GW-012**: **Where** the user configures multiple platform accounts (e.g., personal Telegram + work Slack), the Gateway **shall** maintain separate session threads per account and never cross-post replies between them.

### 5.5 Unwanted Behavior

- **REQ-GW-013**: **If** a platform message originates from a user ID that is not mapped to a GOOSE user, **then** the Gateway **shall not** forward the message to `goosed` and **shall** reply with an onboarding link.
- **REQ-GW-014**: **If** an outgoing message contains personally identifying information from another mapped user (email, phone, home address), **then** the Gateway **shall** redact the identifier before sending.
- **REQ-GW-015**: **If** a KakaoTalk 알림톡 template does not match an approved template ID, **then** the Gateway **shall not** attempt delivery and **shall** log a template violation error.

### 5.6 Complex

- **REQ-GW-016**: **While** the user has enabled Slack integration, **when** a Slack event arrives for a channel the bot is a member of, the Gateway **shall** (a) verify the HMAC signature of the event payload, (b) check that the user is mapped, (c) forward the message to `goosed`, and (d) respond within the 3-second Slack acknowledgement window using a deferred response if needed.

---

## 6. 핵심 Go 타입 시그니처

```go
// internal/gateway/platform.go
// 모든 메신저 플랫폼 어댑터가 구현하는 공통 인터페이스.

// Platform은 외부 메신저 플랫폼의 단일 어댑터.
type Platform interface {
    Name() string                          // "telegram" | "discord" | ...
    Start(ctx context.Context) error       // long polling / webhook 시작
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutboundMessage) error
    OnInbound(handler InboundHandler)      // 수신 핸들러 등록
    HealthCheck() HealthStatus
}

type InboundHandler func(ctx context.Context, msg InboundMessage) error

// 플랫폼 식별자
type PlatformKind string

const (
    PlatformTelegram PlatformKind = "telegram"
    PlatformDiscord  PlatformKind = "discord"
    PlatformSlack    PlatformKind = "slack"
    PlatformMatrix   PlatformKind = "matrix"
    PlatformKakao    PlatformKind = "kakao"
    PlatformWeChat   PlatformKind = "wechat"
    PlatformWebhook  PlatformKind = "webhook"
)

// InboundMessage: 외부 플랫폼 → GOOSE
type InboundMessage struct {
    Platform       PlatformKind
    PlatformUserID string              // 예: Telegram user_id
    ChannelID      string              // 채널 ID (DM이면 user ID와 동일)
    Text           string
    ReceivedAt     time.Time
    ReplyContext   ReplyContext        // 답장 스레드 지원용
}

// OutboundMessage: GOOSE → 외부 플랫폼
type OutboundMessage struct {
    Platform       PlatformKind
    PlatformUserID string
    ChannelID      string
    Text           string
    TemplateID     string              // 카카오 알림톡용 승인된 템플릿 ID
    Attachments    []Attachment        // 1차 사용 안함
}

type ReplyContext struct {
    ThreadID      string               // Slack thread_ts, Discord message reference
    OriginalID    string
}

// UserMapping: platform user ↔ GOOSE user
type UserMapping struct {
    GooseUserID    string
    Platform       PlatformKind
    PlatformUserID string
    LinkedAt       time.Time
    Scopes         []string            // ["read", "notify"]
}

// KakaoTalkClient: 알림톡 전용
type KakaoTalkClient interface {
    SendNotification(ctx context.Context, phone string, templateID string, params map[string]string) error
    ListApprovedTemplates(ctx context.Context) ([]KakaoTemplate, error)
}

type KakaoTemplate struct {
    ID           string
    Name         string
    Body         string   // 승인된 문구 (placeholder 포함)
    Category     string
    ApprovedAt   time.Time
}

// WebhookMessage: Generic Webhook용
type WebhookMessage struct {
    Source   string            // 발신 시스템 식별자
    Kind     string            // 이벤트 종류
    Payload  map[string]any
    Signature []byte           // HMAC-SHA256
}
```

---

## 7. 수락 기준 (Acceptance Criteria)

### 7.1 AC-GW-001 — Telegram 연결

**Given** 사용자가 Telegram bot 생성 + token 입력 **When** Gateway Telegram adapter 시작 **Then** long polling으로 메시지 수신 가능, `/start` 명령에 greeting 응답.

### 7.2 AC-GW-002 — Telegram 메시지 라우팅

**Given** 매핑된 Telegram user **When** "오늘 일정 뭐야?" 전송 **Then** 3초 내 `goosed` 응답이 Telegram 메시지로 표시.

### 7.3 AC-GW-003 — 매핑되지 않은 사용자 거부

**Given** Gateway가 실행 중 **When** 매핑 없는 random user가 메시지 전송 **Then** bot이 onboarding 링크 반환, goosed에 메시지 전달 안 함.

### 7.4 AC-GW-004 — Slack slash command

**Given** Slack 워크스페이스에 bot 설치 **When** 사용자가 `/goose 오늘 TODO` 입력 **Then** 3초 내 ack 응답, 5초 내 deferred response로 실제 답변 표시.

### 7.5 AC-GW-005 — Slack HMAC 검증

**Given** Slack bot 설치 **When** 조작된 signature의 Slack event 수신 **Then** 서명 실패 로그 기록, goosed에 전달 안 함.

### 7.6 AC-GW-006 — Discord mention

**Given** Discord 서버에 bot 초대 **When** 채널에서 "@GooseBot 질문" **Then** 동일 채널에 응답 메시지 도착.

### 7.7 AC-GW-007 — Matrix

**Given** Matrix homeserver에 bot 계정 생성 **When** 사용자가 DM 전송 **Then** 암호화된 방(Megolm)에서도 정상 수신·응답.

### 7.8 AC-GW-008 — 카카오 알림톡 전송

**Given** 승인된 알림톡 템플릿 ID + 매핑된 사용자 phone **When** `goosed`가 알림 이벤트 발행 **Then** 카카오 비즈니스 API 호출 성공, 사용자 카톡에 도착.

### 7.9 AC-GW-009 — 알림톡 템플릿 위반 거부

**Given** 비승인 template ID 요청 **When** SendNotification 호출 **Then** 즉시 `template_violation` 에러 반환, 카카오 API 호출 안 함.

### 7.10 AC-GW-010 — Tool 실행 거부

**Given** 사용자가 Telegram에서 "로컬 파일 지워줘" 요청 **When** Gateway가 `goosed`에 전달 **Then** goosed가 tool 실행을 permission-required로 표기, Gateway가 "Desktop/Mobile에서 승인 필요" 응답.

### 7.11 AC-GW-011 — 플랫폼 rate limit

**Given** Telegram 초당 30 메시지 한도 **When** Gateway가 50 메시지 발송 시도 **Then** 처음 30은 전송, 나머지는 큐에서 대기하며 429 없이 완료.

### 7.12 AC-GW-012 — PII redaction

**Given** outbound 메시지에 다른 사용자 이메일 포함 **When** 발송 직전 **Then** 이메일이 `[email redacted]`로 치환되어 전송.

### 7.13 AC-GW-013 — OAuth 토큰 갱신

**Given** 플랫폼 OAuth access token 만료 **When** 메시지 발송 시도 **Then** Gateway가 refresh token으로 자동 갱신, 메시지가 투명하게 전송됨.

### 7.14 AC-GW-014 — Webhook HMAC 인증

**Given** Generic Webhook 엔드포인트 + 공유 secret **When** 서명된 요청 수신 **Then** HMAC 일치 확인 후 InboundMessage로 변환, 서명 불일치 시 401 반환.

---

## 8. TDD 전략

- **RED**:
  - `platform_test.go`: 공통 Platform 인터페이스 contract 테스트(모든 어댑터가 동일 규약 충족)
  - `telegram/bot_test.go`: mock Telegram API server로 long polling + reply
  - `slack/hmac_test.go`: HMAC signature verify 성공/실패 케이스
  - `kakao/template_test.go`: 승인되지 않은 template 거부
- **GREEN**:
  - 각 어댑터는 최소 send + receive 구현 후 단위 테스트 통과
  - User mapping은 SQLite 단일 테이블
- **REFACTOR**:
  - Rate limiter를 공통 util(`gateway/ratelimit/`)로 분리
  - PII redactor를 shared middleware로 추출
- **통합 테스트**:
  - Telegram: `t.me/BotFather`로 테스트 봇 생성 + 수동 실행 가능한 integration 테스트
  - Slack/Discord는 sandbox workspace/server 권장
  - 카카오·WeChat은 mock 서버 기반 (실 API는 E2E only)
- **커버리지**: 80%+ (외부 API mocking 한계)

---

## 9. 제외 항목 (Exclusions)

- **음성/비디오 통화**: v2+.
- **파일 업·다운로드**: 1차 텍스트만.
- **봇 페르소나 커스터마이즈**: 표준 GOOSE만.
- **SaaS 멀티테넌트**: self-hosted 또는 단일 사용자 인스턴스.
- **카카오톡 i 오픈빌더 양방향 챗봇**: 알림톡 단방향만.
- **LINE / Viber / Signal**: 이후 별도 SPEC.
- **SMS/MMS**: 비용 문제 out.
- **기업용 Microsoft Teams / Zoom Chat**: v2+.
- **Proactive outreach(스팸 성격)**: 사용자가 명시 구독한 이벤트만 발송.
- **Billing / usage metering**: 플랫폼 API 비용은 사용자 부담, Gateway는 호출 카운트만 제공.

# Research — SPEC-MINK-MSG-SLACK-001

AGPL-3.0 헌장. MSG-DISCORD-001 / MSG-TELEGRAM-001 패턴 재사용.

## 1. Slack API matrix
- **Events API** (HTTP webhook): app_mention, message.im, etc. signing secret HMAC-SHA256
- **Slash command**: `/mink ...`, response_url 또는 chat.postMessage
- **Interactive components**: Block Kit (Buttons / Select / Modal), interaction payload
- **OAuth v2**: workspace 단위 installation, bot_token + signing_secret 발급

## 2. 3초 응답 SLA
- Events API 3초 내 200 OK ack 후 chat.postMessage 비동기
- Slash command 3초 ack 후 response_url POST 또는 chat.postMessage

## 3. HMAC 검증
- Slack signing secret + `X-Slack-Signature` + `X-Slack-Request-Timestamp` 헤더
- `v0:{timestamp}:{body}` HMAC-SHA256 = 서명 비교
- replay attack 방지 — timestamp ±5분 검증

## 4. Go 라이브러리
- `slack-go/slack` (BSD-2, AGPL 호환) Block Kit + Events API 우수
- 자체 HTTP + crypto/hmac (zero-dep) 도 가능
- 결정: `slack-go/slack` 채택 (Block Kit 보일러플레이트 절감)

## 5. BRIDGE-001 정합
- inbound → HMAC verify → normalize (canonical: user_id, channel_id, team_id, text, ts, thread_ts) → BRIDGE router → LLM → reply

## 6. 참조
- Slack API docs / Events API / Block Kit / OAuth v2 / signing secret 검증
- slack-go: https://github.com/slack-go/slack
- BRIDGE-001 / MSG-TELEGRAM-001 / MSG-DISCORD-001
- AUTH-CREDENTIAL-001 / LLM-ROUTING-V2-AMEND-001 / MEMORY-QMD-001
- ADR-002 (AGPL)

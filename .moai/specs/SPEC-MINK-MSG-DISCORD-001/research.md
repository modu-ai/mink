# Research — SPEC-MINK-MSG-DISCORD-001

AGPL-3.0-only 헌장 (ADR-002) 위 작성.

## 1. Discord API 매트릭스

- **Interactions endpoint (HTTP)**: Discord 가 PING / APPLICATION_COMMAND / MESSAGE_COMPONENT / APPLICATION_COMMAND_AUTOCOMPLETE / MODAL_SUBMIT 등을 POST 로 전송. 3초 SLA.
- **Gateway (WebSocket)**: 실시간 presence / typing — 본 SPEC OUT (post-launch).
- **Slash command API**: `applications.{app_id}.commands` POST/PATCH 로 등록. 글로벌 vs 길드별.
- **Bot invite scopes**: `bot` + `applications.commands`. 권한 비트 마스크.

## 2. Ed25519 서명 검증

- 모든 Interactions 요청은 `X-Signature-Ed25519` + `X-Signature-Timestamp` 헤더 보유
- 서명 검증 실패 시 HTTP 401 (Discord 가 자동 retry 안 함)
- Go 표준 `crypto/ed25519.Verify(publicKey, msg, sig)` 사용. libsodium 의존 0
- 공개키는 Discord Developer Portal 에서 application 단위 발급, AUTH-CREDENTIAL-001 에 저장

## 3. 3초 SLA + Deferred Response

- Interaction 응답 type 5 (`DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE`) = "Bot is thinking..." 즉시 표시
- 그 후 webhook follow-up endpoint (`/webhooks/{app_id}/{token}/messages/@original` 또는 신규 메시지) 로 final response 전송
- LLM 응답이 3초 이상 걸려도 OK

## 4. BRIDGE-001 정합

- inbound webhook → ed25519 verify → normalize (canonical schema: user_id, channel_id, guild_id, text, timestamp, thread_id) → BRIDGE-001 router → LLM-ROUTING-V2-AMEND → reply formatter → outbound webhook

## 5. MSG-TELEGRAM-001 / MSG-SLACK-001 패턴 재사용

- HMAC 검증 패턴 (Slack signing secret) vs Ed25519 패턴 (Discord) — 검증 helper 분리
- 3초 SLA — Slack ack pattern 과 동등
- BRIDGE router / message normalizer / LLM 위임 / outbound — 동일

## 6. Go 라이브러리

- `bwmarrin/discordgo` (BSD-3-Clause, AGPL 호환): Gateway + REST. 본 SPEC 은 REST + HTTP Interactions 만 사용
- 또는 자체 HTTP 클라이언트 + crypto/ed25519 (zero-dep 원칙)
- 결정: 자체 HTTP + crypto/ed25519 (zero-dep 우위)

## 7. AGPL-3.0 정합

- 신규 .go 모두 SPDX 헤더
- discordgo 라이선스 (BSD-3) 호환 ✓ (사용 안 함, 자체 구현)
- crypto/ed25519 = Go 표준 (BSD-3) 호환

## 8. 참조

- Discord Developer Docs: https://discord.com/developers/docs/interactions/receiving-and-responding
- Ed25519 verification: https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
- discordgo: https://github.com/bwmarrin/discordgo (참고용)
- BRIDGE-001 / MSG-TELEGRAM-001 / MSG-SLACK-001 (의존)
- AUTH-CREDENTIAL-001 (credential 저장)
- LLM-ROUTING-V2-AMEND-001 (LLM 호출)
- MEMORY-QMD-001 (옵션 session 색인)
- ADR-002 (AGPL-3.0)

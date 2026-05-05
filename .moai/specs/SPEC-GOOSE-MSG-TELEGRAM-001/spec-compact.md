---
id: SPEC-GOOSE-MSG-TELEGRAM-001
artifact: spec-compact
version: "0.1.0"
created_at: 2026-05-05
updated_at: 2026-05-05
---

# SPEC-GOOSE-MSG-TELEGRAM-001 — Compact View (Run-Phase Reference)

이 문서는 `/moai run` 단계에서 implementation agent 가 SPEC 전체 (~30KB) 로드 없이 핵심만 빠르게 참조하는 token-economic compact view 다. 모든 결정은 `spec.md` / `plan.md` / `acceptance.md` / `research.md` 가 근거.

---

## 1. 한 줄 정의

GOOSE 데몬과 Telegram Bot API 6.x 를 연결하는 첫 사용자-향 채널. 1:1 chat ingress (long polling default) + outbound `telegram_send_message` tool (TOOLS-001 등록). BRIDGE-001 / TOOLS-001 / MEMORY-001 / CREDENTIAL-PROXY-001 / AUDIT-001 위에 wiring.

## 2. 의존 SPEC (모두 merged 가정)

- BRIDGE-001 — `AgentService/Query` (inbound 처리), streaming RPC (Phase 4)
- TOOLS-001 — `telegram_send_message` 등록, permission gate
- MEMORY-001 — `messaging.telegram.users` + `messaging.telegram.last_offset` bucket
- CREDENTIAL-PROXY-001 — bot token keyring 보관
- AUDIT-001 — inbound/outbound 모두 append-only

## 3. 패키지 구조 (NEW)

```
internal/messaging/telegram/
├── bootstrap.go      # Start(ctx, deps) entry — daemon hook
├── client.go         # go-telegram/bot wrapper (Client interface)
├── poller.go         # getUpdates long polling loop + offset
├── handler.go        # inbound (text / callback / file) 분기
├── sender.go         # outbound Send(ctx, req) + allowed_users 검증
├── tool.go           # TOOLS-001 register("telegram_send_message")
├── store.go          # MEMORY-001 wrapper (mapping, offset)
├── audit.go          # AUDIT-001 wrapper (direction, hash)
├── markdown.go       # Markdown V2 escape (18 reserved chars), inline keyboard
├── webhook.go        # 옵션 mode (BRIDGE HTTP mux)
├── config.go         # yaml load + token reject
└── testdata/         # mock fixtures, golden output

cmd/goose/cmd/telegram.go   # cobra: setup|start|status|approve|revoke
internal/daemon/bootstrap.go  # MODIFY: messaging.Start 호출 추가
```

## 4. 핵심 EARS 요구사항 (25개 전체 — Ubiquitous 5 / Event-Driven 7 / State-Driven 5 / Unwanted 6 / Optional 2)

| ID | type | 한 줄 |
|----|------|-----|
| U01 | Ubiquitous [HARD] | 모든 메시지 AUDIT-001 append |
| U02 | Ubiquitous [HARD] | bot token 평문 저장 금지, keyring/env 만 |
| U03 | Ubiquitous | chat_id mapping MEMORY-001 영속 |
| U04 | Ubiquitous | outbound tool TOOLS-001 permission gate 통과 |
| U05 | Ubiquitous | offset 영속, 재시작 중복 수신 0 |
| E01 | Event-Driven | 사용자 메시지 → BRIDGE Query → response, P95 < 5초 |
| E02 | Event-Driven | `/stream` 또는 default_streaming → streaming + editMessageText |
| E03 | Event-Driven | `telegram_send_message` tool → permission → allowed_users → MD V2 → send |
| E04 | Event-Driven | inbound > 4096자 → reject + audit |
| E05 | Event-Driven | callback_query → answerCallbackQuery + BRIDGE Query |
| E06 | Event-Driven | file attachment → getFile → 다운로드 → query 본문 포함 |
| E07 | Event-Driven | webhook 등록 실패 → polling fallback |
| S01 | State-Driven | 미매핑 chat_id + auto_admit=false → "사전 승인 필요" 응답 |
| S02 | State-Driven | token 부재 → daemon 정상 기동 + messaging skip |
| S03 | State-Driven | poller backoff 단계 metrics 노출 |
| S04 | State-Driven | auto_admit=true + 첫 user → 자동 admin 등록 |
| S05 | State-Driven | streaming 중 같은 chat 후속 메시지 → FIFO 큐 (max 5) |
| N01 | Unwanted [HARD] | yaml 평문 token reject |
| N02 | Unwanted [HARD] | allowed_users 외 chat_id outbound 금지 |
| N03 | Unwanted | inbound > 4096자 query 미실행 |
| N04 | Unwanted | callback 60초 expire 처리 거부 |
| N05 | Unwanted | blacklist chat_id silently drop |
| N06 | Unwanted [HARD] | outbound 본문에 PII (다른 user 정보) 포함 금지 |
| O01 | Optional | silent_default → disable_notification |
| O02 | Optional | typing_indicator → sendChatAction(typing) 5초 주기 |

## 5. Phase 요약 (no time estimates)

- **P1 — Foundation**: setup CLI + polling skeleton + echo. AC-MTGM-001 GREEN.
- **P2 — BRIDGE 연동**: query + audit + mapping. AC-MTGM-002/003/004/006 GREEN.
- **P3 — Outbound + Markdown V2**: tool + escape + file attach. AC-MTGM-005/007/008/010 GREEN.
- **P4 — Streaming + Webhook + Polish**: streaming + webhook + nice-to-have. AC-MTGM-009 GREEN. 모든 AC GREEN.

## 6. AC 매트릭스 (11 → REQ traceability 완전)

| AC | REQ 매핑 | Phase |
|----|--------|-------|
| AC-001 setup SLO 5분 | U02, N01 | P1 |
| AC-002 round-trip + 4096 reject + audit hash | E01, U01, U03, E04, N03, N06 | P2 |
| AC-003 mapping 영속 | U03, U05 | P2 |
| AC-004 first-msg gate + blacklist | S01, S04, N05 | P2 |
| AC-005 outbound tool | E03, N02, U04 | P3 |
| AC-006 daemon skip + backoff metrics + webhook fallback | S02, S03, E07 | P2/P4 |
| AC-007 file attach | E06 | P3 |
| AC-008 yaml token reject | N01, U02 | P3 (또는 P1 일찍) |
| AC-009 streaming | E02, S05 | P4 |
| AC-010 MD V2 + keyboard + callback expire | E03, E05, N04 | P3 |
| AC-011 audit append-only + PII protection | U01, N06 | P2~P4 |

**REQ traceability 완전 매핑** — 모든 REQ (U01~U05, E01~E07, S01~S05, N01~N06, O01~O02) 가 AC 또는 Optional 표기. Optional (O01, O02) 은 AC 없음 (nice-to-have).

## 7. 의존 라이브러리 (신규 1개)

`github.com/go-telegram/bot` v1.x — MIT, 활발 maintenance. `client.go` thin wrapper 로 격리 (R8 회피).

## 8. MX 태그 핵심

- **ANCHOR**: `bootstrap.Start`, `sender.Send`, `handler.Handle`
- **NOTE**: `markdown.EscapeV2`, `poller.runLoop` backoff, `store` offset 영속 시점
- **WARN**: `poller.runLoop` (goroutine + retry, complexity ≥ 15) → REASON 필수
- **TODO**: P1 RED 시작 시 stub 들

## 9. 보안 핵심 4

1. token: keyring only (REQ-N01).
2. outbound: allowed_users gate (REQ-N02).
3. webhook: 32자 hex secret_path + TLS.
4. audit hash: 본문 평문 미저장 (REQ-N06).

## 10. 완료 기준 (1줄)

11 AC GREEN + coverage ≥ 85% + Markdown V2 escape 18자 전수 (`_*[]()~\`>#+-=|{}.!`) + integration mock test + 수동 5분 setup 검증 + golangci-lint clean + `@MX:TODO` 0개 + plan-auditor PASS.

---

전체 컨텍스트가 필요하면 spec.md (~31KB), plan.md (~9KB), acceptance.md (~12KB), research.md (~12KB) 참조.

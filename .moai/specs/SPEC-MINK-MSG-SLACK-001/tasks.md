# Tasks — SPEC-MINK-MSG-SLACK-001

20 tasks, 4 마일스톤.

## §0 패키지 매핑
- `internal/channel/slack/handler.go`: T-001, T-002, T-005, T-006
- `internal/channel/slack/verify.go`: T-003
- `internal/channel/slack/normalize.go`: T-004
- `internal/channel/slack/oauth.go`: T-009, T-010
- `internal/channel/slack/blockkit.go`: T-011, T-012
- `internal/channel/slack/client.go`: T-007, T-008
- 통합: T-013~T-020

## §1 M1 (3 task)
- T-001: POST handler 골격 (AC SLK-002 3초 SLA)
- T-002: url_verification challenge (AC SLK-008)
- T-003: HMAC verify (AC SLK-001)

## §2 M2 (5 task)
- T-004: normalize → BRIDGE canonical (AC SLK-006)
- T-005: app_mention handler (AC SLK-009)
- T-006: message.im DM handler (AC SLK-010, 015)
- T-007: slash command + response_url / chat.postMessage (AC SLK-011)
- T-008: BRIDGE router 위임 + LLM-ROUTING (AC SLK-003, 017)

## §3 M3 (2 task)
- T-009: OAuth v2 flow + workspace_id (AC SLK-012, 016)
- T-010: AUTH-CREDENTIAL Store/Load (AC SLK-004)

## §4 M4 (10 task)
- T-011: Block Kit message 생성 (AC SLK-013)
- T-012: Interactive component (Buttons/Select/Modal) parsing (AC SLK-013 보강)
- T-013: thread_ts threading (AC SLK-018)
- T-014: chat.update placeholder → final (AC SLK-015 보강)
- T-015: rate limit HTTP 429 Retry-After (AC SLK-014, 022)
- T-016: chat.postMessage retry max 3 + backoff (AC SLK-022 보강)
- T-017: PII 마스킹 (MEMORY-QMD redact) (AC SLK-020)
- T-018: AGPL SPDX 헤더 신규 .go (AC SLK-005, 026)
- T-019: 사용자 trigger 없이 메시지 0 (AC SLK-019)
- T-020: MINK_SLACK_ALLOWED_CHANNELS env + memory share OPT + go vet/golangci-lint CI gate (AC SLK-007, 021, 023, 024, 025)

## §5 task↔AC↔REQ (orphan 0, milestone 경계 0)

| task | M | 핵심 AC | 핵심 REQ |
|---|---|---|---|
| T-001 | M1 | SLK-002 | REQ-SLK-002 |
| T-002 | M1 | SLK-008 | REQ-SLK-008 |
| T-003 | M1 | SLK-001 | REQ-SLK-001 |
| T-004 | M2 | SLK-006 | REQ-SLK-006 |
| T-005 | M2 | SLK-009 | REQ-SLK-009 |
| T-006 | M2 | SLK-010, 015 | REQ-SLK-010, 016 |
| T-007 | M2 | SLK-011 | REQ-SLK-011 |
| T-008 | M2 | SLK-003, 017 | REQ-SLK-003, 017 |
| T-009 | M3 | SLK-012, 016 | REQ-SLK-012 |
| T-010 | M3 | SLK-004 | REQ-SLK-004 |
| T-011 | M4 | SLK-013 | REQ-SLK-013 |
| T-012 | M4 | SLK-013 보강 | REQ-SLK-013 |
| T-013 | M4 | SLK-018 | REQ-SLK-018 |
| T-014 | M4 | SLK-015 보강 | REQ-SLK-015 |
| T-015 | M4 | SLK-014 | REQ-SLK-014 |
| T-016 | M4 | SLK-022 | REQ-SLK-022 |
| T-017 | M4 | SLK-020 | REQ-SLK-020 |
| T-018 | M4 | SLK-005, 026 | REQ-SLK-005 |
| T-019 | M4 | SLK-019 | REQ-SLK-019 |
| T-020 | M4 | SLK-007, 021, 023, 024, 025 | REQ-SLK-007, 021, 023, 024 |

각 AC ≥1 task, 각 REQ ≥1 task. orphan 0.

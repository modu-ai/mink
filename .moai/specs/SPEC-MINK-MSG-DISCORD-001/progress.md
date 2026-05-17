# Progress — SPEC-MINK-MSG-DISCORD-001

## 1. Overall

- **Status**: planned
- **Version**: 0.2.0
- **Overall Progress**: 0% (0/24 AC GREEN)
- **plan-auditor pass**: 대기
- **Last Updated**: 2026-05-16

## 2. Milestone Progress

| 마일스톤 | priority | tasks | AC (메인 책임 unique) | 진행률 | status |
|---|---|---|---|---|---|
| M1 — Ed25519 verify + PING/PONG | High | 4 (T-001~T-004) | 4 (AC-DCD-001, 002, 007, 011) | 0% | not-started |
| M2 — APPLICATION_COMMAND + slash + deferred | High | 5 (T-005~T-009) | 5 (AC-DCD-003, 006, 008, 010, 013) | 0% | not-started |
| M3 — MESSAGE_COMPONENT + Buttons/Select | Medium | 2 (T-010, T-011) | 3 (AC-DCD-009, 014, 022) | 0% | not-started |
| M4 — Bot invite + AUTH + 3초 SLA + PII | Medium | 7 (T-012~T-018) | 12 (AC-DCD-004, 005, 012, 015, 016, 017, 018, 019, 020, 021, 023, 024) | 0% | not-started |

> 카운팅: 메인 책임 unique. 합 = 4+5+3+12 = 24 (overall AC 24 정합).

## 3. Task Status

T-001~T-018 = 0% / not-started.

## 4. External Dependencies (freeze 시점)

| SPEC | freeze 시점 | 인터페이스 |
|---|---|---|
| AUTH-CREDENTIAL-001 | M4 진입 전 | AuthCredentialService (Store/Load/Delete/List) |
| LLM-ROUTING-V2-AMEND-001 | M2 진입 전 | Router.Route |
| BRIDGE-001 | M2 진입 전 | canonical schema (user_id/channel_id/guild_id/text/timestamp/thread_id) |
| MEMORY-QMD-001 | M4 진입 전 (옵션) | redact pipeline interface |

# Progress — SPEC-MINK-WEB-CONFIG-001

- Status planned / Version 0.2.0 / 0% (0/24 AC GREEN) / plan-auditor 대기 / Last 2026-05-16

## Milestone

| 마일스톤 | priority | tasks | AC (unique) | 진행 |
|---|---|---|---|---|
| M1 — Server + CSRF + Origin + auto-port + shutdown | High | 8 (T-001~T-007, T-019) | 8 (WCF-001, 002, 003, 004, 005, 008, 009, 014) | 0% |
| M2 — Provider/Channel/Memory | High | 4 (T-008~T-011) | 4 (WCF-007, 010, 011, 012) | 0% |
| M3 — Ritual/Scheduler/Locale | Medium | 3 (T-012~T-014) | 3 (WCF-006, 015, 016) | 0% |
| M4 — Privacy + Optional + E2E | Medium | 5 (T-015~T-018, T-020) | 9 (WCF-013, 017, 018, 019, 020, 021, 022, 023, 024) | 0% |

합 = 8+4+3+9 = 24 (overall 24 정합).

## External Deps freeze
- ONBOARDING-001 v0.3.1 (source codebase, 변경 0)
- AUTH-CREDENTIAL-001 (M2 전): Store/Load/Delete/List
- LLM-ROUTING-V2-AMEND-001 (M2 전): Router.Route + provider test
- MEMORY-QMD-001 (M2 전): vault 경로 설정 API
- CLI-TUI-003 amend (M2~M3): 동일 backend service layer
- MSG-SLACK/DISCORD/TELEGRAM (M2 전): channel revoke API

# Progress — SPEC-MINK-MSG-SLACK-001

- **Status**: planned / Version 0.2.0 / 0% (0/26 AC GREEN) / plan-auditor 대기 / Last 2026-05-16

## Milestone

| 마일스톤 | priority | tasks | AC (unique) | 진행 |
|---|---|---|---|---|
| M1 — HMAC + Events API | High | 3 (T-001~T-003) | 3 (SLK-001, 002, 008) | 0% |
| M2 — handler + BRIDGE | High | 5 (T-004~T-008) | 7 (SLK-003, 006, 009, 010, 011, 015, 017) | 0% |
| M3 — OAuth + AUTH | High | 2 (T-009~T-010) | 3 (SLK-004, 012, 016) | 0% |
| M4 — Interactive + rate + PII + AGPL | Medium | 10 (T-011~T-020) | 13 (SLK-005, 007, 013, 014, 018, 019, 020, 021, 022, 023, 024, 025, 026) | 0% |

합 = 3+7+3+13 = 26 (overall 26 정합).

## External Deps freeze
- AUTH-CREDENTIAL-001 (M3 전): Store/Load/Delete/List
- LLM-ROUTING-V2-AMEND-001 (M2 전): Router.Route
- BRIDGE-001 (M2 전): canonical schema
- MEMORY-QMD-001 (M4 전 옵션): redact pipeline

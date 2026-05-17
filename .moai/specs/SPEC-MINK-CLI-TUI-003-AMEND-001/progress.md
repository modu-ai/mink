# Progress — SPEC-MINK-CLI-TUI-003-AMEND-001

## 1. Overall

- **Status**: planned (plan 종결, run 미개시)
- **Version**: 0.2.0
- **Overall Progress**: 0% (0/24 AC GREEN)
- **plan-auditor pass**: 대기
- **Last Updated**: 2026-05-16

## 2. Milestone Progress

| 마일스톤 | priority | tasks | AC (메인 책임) | 진행률 | status |
|---|---|---|---|---|---|
| M1 — 피처 패리티 audit | High | 1 (T-001) | 4 (AC-CTA-001, 002, 003, 004) | 0% | not-started |
| M2 — 누락 명령 wiring | High | 6 (T-002~T-006 + T-008-M2 분리) | 8 (AC-CTA-007, 008, 009, 010, 011, 012, 013, 014) | 0% | not-started |
| M3 — 용어 통일 + Optional + CI gate | Medium | 7 (T-007~T-012 + T-013 신규) | 12 (AC-CTA-005, 006, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024) | 0% | not-started |

> 카운팅 방법론: 메인 GREEN 책임 task 기준 unique 카운트 (audit B1 학습). 합 = 4+8+12 = 24 (overall AC 24 정합).

## 3. Task Status

| Task | 마일스톤 | priority | status | 핵심 AC |
|---|---|---|---|---|
| T-001 | M1 | High | not-started | AC-CTA-001, 002, 003, 004 |
| T-002 | M2 | High | not-started | AC-CTA-007 |
| T-003 | M2 | High | not-started | AC-CTA-008 |
| T-004 | M2 | High | not-started | AC-CTA-009 |
| T-005 | M2 | High | not-started | AC-CTA-010 |
| T-006 | M2 | High | not-started | AC-CTA-013, 014 |
| T-007 | M3 | Medium | not-started | AC-CTA-005, 019 |
| T-008 | M3 | Medium | not-started | AC-CTA-005, 011, 012 |
| T-009 | M3 | Medium | not-started | AC-CTA-005 |
| T-010 | M3 | Medium | not-started | AC-CTA-021 |
| T-011 | M3 | Medium | not-started | AC-CTA-022 |
| T-012 | M3 | Medium | not-started | AC-CTA-006, 015, 020 |

## 4. External Dependencies (freeze 시점)

| SPEC | freeze 시점 | 인터페이스 |
|---|---|---|
| MEMORY-QMD-001 | M2 진입 전 | MemoryQMDService (Add/Search/Reindex/Export/Import/Stats/Prune — 7 메서드) |
| LLM-ROUTING-V2-AMEND-001 | M2 진입 전 | LLMRoutingAuth (StartKeyPaste/StartOAuth/RefreshOAuth/ValidateCredential — 4 메서드, audit D4 fix) |
| AUTH-CREDENTIAL-001 | M2 진입 전 | AuthCredentialService (Store/Load/Delete/List — 4 메서드, audit D4 fix). config store selector 는 별도 SetAuthStore/GetAuthStore |

# Plan — SPEC-MINK-WEB-CONFIG-001

AGPL-3.0. ONBOARDING Phase 5 Web shell 재사용. audit 학습 사전 적용.

## 1. Go 패키지
- `internal/server/config/` 신규 (handler.go / server.go / embed.go)
- `web/config/` 신규 (React + Vite, install 패턴 복사)

## 2. 4 마일스톤

### M1 — Server skeleton + CSRF + Origin + auto-port + graceful shutdown
- 127.0.0.1:0 bind, CSRF double-submit, Origin allowlist, SIGINT/SIGTERM 5s
- **AC**: WCF-001, 002, 003, 004, 008, 009, 014, 017, 019

### M2 — Provider / Channel / Memory 카테고리
- Provider UI (5 cards) → AUTH-CREDENTIAL Store/Load/Delete + LLM-ROUTING validation
- Channel UI (3 cards) → MSG-* OAuth + revoke
- Memory UI → MEMORY-QMD vault 경로 / retention / 모델 변경
- **AC**: WCF-005, 010, 011, 012, 015

### M3 — Ritual / Scheduler / Locale 카테고리
- Ritual UI → JOURNAL/BRIEFING/WEATHER/RITUAL 빈도·tone
- Scheduler UI → cron 패턴 편집
- Locale UI → LOCALE-001/I18N-001 변경
- **AC**: WCF-007, 015 보강

### M4 — Privacy / Optional + E2E
- Privacy: AGPL consent 갱신, audit log retention
- Playwright E2E (configuration round-trip)
- Optional: MINK_CONFIG_WEB_PORT env, audit log JSON export, dry-run
- **AC**: WCF-006, 013, 016, 018, 020, 021, 022

## 3. 의존 SPEC freeze
- ONBOARDING-001 v0.3.1 (코드베이스 source) — 변경 없음
- AUTH-CREDENTIAL-001: M2 진입 전 Store/Load/Delete/List
- LLM-ROUTING-V2-AMEND-001: M2 진입 전 Router.Route + provider test
- MEMORY-QMD-001: M2 진입 전 vault 경로 설정 API
- CLI-TUI-003 amend: M2~M3 동일 backend service layer
- MSG-SLACK/DISCORD/TELEGRAM: M2 진입 전 (channel revoke API)

## 4. 위험
| R | 완화 |
|---|---|
| R1: 단일 사용자 동시 mutation | mutex per key |
| R2: 평문 credential 표시 | mask `****`, REQ-WCF-018 |
| R3: 0.0.0.0 bind 실수 | REQ-WCF-001 강제 127.0.0.1 |
| R4: ONBOARDING 코드베이스 drift | shared 패턴 + 회귀 테스트 |

checklist: 22 REQ + 24 AC + 4 milestones + 18 tasks. orphan 0, milestone 경계 0, REQ-WCF-NNN 통일.

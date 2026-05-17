# Tasks — SPEC-MINK-WEB-CONFIG-001

18 tasks, 4 마일스톤.

## §0 패키지 매핑
- `internal/server/config/handler.go`: T-001, T-002, T-003, T-004
- `internal/server/config/server.go`: T-005, T-006
- `internal/server/config/embed.go`: T-007
- `web/config/`: T-008~T-014 (React UI)
- 통합: T-015~T-018

## §1 M1 (6 task)
- T-001: POST handler 골격 + CSRF double-submit (AC WCF-003)
- T-002: Origin header 검증 (AC WCF-004)
- T-003: atomic config write (temp + rename) (AC WCF-005)
- T-004: mutex per key (AC WCF-019)
- T-005: RunServer 127.0.0.1:auto-port (AC WCF-001, 002)
- T-006: SIGINT/SIGTERM 5s graceful shutdown (AC WCF-009)
- T-007: //go:embed web/config/dist (AC WCF-008 보강)
- T-019: 단일 활성 session 강제 (AC WCF-014)

## §2 M2 (4 task)
- T-008: Provider UI 5 cards + validation spinner (AC WCF-010, 015)
- T-009: Channel UI 3 cards + revoke (AC WCF-011)
- T-010: Memory UI + MEMORY-QMD prune queue (AC WCF-012)
- T-011: AUTH-CREDENTIAL Store/Load/Delete 위임 backend (AC WCF-007)

## §3 M3 (3 task)
- T-012: Ritual UI (JOURNAL/BRIEFING/WEATHER/RITUAL 빈도) (AC WCF-007 보강)
- T-013: Scheduler UI cron (AC WCF-007 보강)
- T-014: Locale/I18N UI (AC WCF-007 보강)

## §4 M4 (5 task)
- T-015: Privacy AGPL consent UI + audit log retention (AC WCF-013, 021)
- T-016: Playwright E2E config round-trip (AC WCF-006, 016)
- T-017: brand theme 적용 (AC WCF-016 보강)
- T-018: 옵션 features — MINK_CONFIG_WEB_PORT + dry-run + AGPL SPDX 헤더 + lint CI gate (AC WCF-006 보강, 020, 022, 023, 024)
- T-020: bind 0.0.0.0 negative test (AC WCF-017, 018)

## §5 task↔AC↔REQ (orphan 0)

| task | M | 핵심 AC | 핵심 REQ |
|---|---|---|---|
| T-001 | M1 | WCF-003 | REQ-WCF-003 |
| T-002 | M1 | WCF-004 | REQ-WCF-004 |
| T-003 | M1 | WCF-005 | REQ-WCF-005 |
| T-004 | M1 | WCF-019 | REQ-WCF-019 |
| T-005 | M1 | WCF-001, 002 | REQ-WCF-001, 002 |
| T-006 | M1 | WCF-009 | REQ-WCF-009 |
| T-007 | M1 | WCF-008 보강 | REQ-WCF-008 |
| T-019 | M1 | WCF-014 | REQ-WCF-014 |
| T-008 | M2 | WCF-010, 015 | REQ-WCF-010, 015 |
| T-009 | M2 | WCF-011 | REQ-WCF-011 |
| T-010 | M2 | WCF-012 | REQ-WCF-012 |
| T-011 | M2 | WCF-007 | REQ-WCF-007 |
| T-012 | M3 | WCF-007 보강 (ritual) | REQ-WCF-007 |
| T-013 | M3 | WCF-007 보강 (scheduler) | REQ-WCF-007 |
| T-014 | M3 | WCF-007 보강 (locale) | REQ-WCF-007 |
| T-015 | M4 | WCF-013, 021 | REQ-WCF-013 |
| T-016 | M4 | WCF-006, 016 | REQ-WCF-006, 016 |
| T-017 | M4 | WCF-016 보강 | REQ-WCF-016 |
| T-018 | M4 | WCF-006 보강, 020, 022, 023, 024 | REQ-WCF-006, 020, 022 |
| T-020 | M4 | WCF-017, 018 | REQ-WCF-017, 018 |

각 AC ≥1 task, 각 REQ ≥1 task. orphan 0.

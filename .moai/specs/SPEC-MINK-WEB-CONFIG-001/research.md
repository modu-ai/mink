# Research — SPEC-MINK-WEB-CONFIG-001

AGPL-3.0 헌장. ONBOARDING-001 Phase 5 Web shell 재사용.

## 1. ONBOARDING-001 v0.3.1 Web shell 코드베이스
- Vite 6 + React 19 + TS strict + Tailwind 3 + shadcn/ui Button/Card/Progress
- internal/server/install/handler.go: CSRF double-submit + Origin allowlist + SessionStore (per-session sync.Mutex 30분 TTL 60초 sweep) + Go 1.22 enhanced ServeMux PathValue + JSON error envelope + sentinel→HTTP 매핑
- internal/server/install/server.go: RunServer 127.0.0.1:0 + 자동 브라우저 + SIGINT/SIGTERM 5s graceful shutdown
- internal/server/install/embed.go: //go:embed all:dist + dev mode 분기

## 2. 본 SPEC 의 재사용
- `internal/server/config/` 신규 (handler/server/embed) — install 패턴 mirror
- 동일 CSRF + Origin 정책
- 동일 SessionStore 패턴 (단일 사용자, 30분 TTL)
- `mink config web` 명령 추가

## 3. 7 설정 카테고리 매핑
- Provider → AUTH-CREDENTIAL + LLM-ROUTING-V2-AMEND
- Channel → MSG-TELEGRAM/SLACK/DISCORD
- Ritual → JOURNAL/BRIEFING/WEATHER/RITUAL
- Scheduler → SCHEDULER-001
- Locale/I18N → LOCALE-001/I18N-001
- Memory → MEMORY-QMD-001
- Privacy → AGPL consent + audit log retention

## 4. CSRF + Origin (ONBOARDING 패턴)
- double-submit cookie + Origin header 검증
- 단일 사용자 / multi-user 0

## 5. 참조
- ONBOARDING-001 v0.3.1 (PR #211/#212/#213/#217)
- LLM-ROUTING-V2-AMEND-001 / AUTH-CREDENTIAL-001 / MEMORY-QMD-001 / MSG-* / SCHEDULER-001 / LOCALE-001 / I18N-001
- CLI-TUI-003 amend (동일 backend service layer)
- ADR-002 (AGPL)

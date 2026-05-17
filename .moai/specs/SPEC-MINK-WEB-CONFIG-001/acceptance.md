# Acceptance — SPEC-MINK-WEB-CONFIG-001

총 24 AC (M1 8 + M2 4 + M3 3 + M4 9). REQ ↔ AC traceable (1:N).

## §1 M1 (8 AC)
- **WCF-001 [REQ-WCF-001/P0]**: 127.0.0.1 only bind, external 0, Verify: integration (netstat)
- **WCF-002 [REQ-WCF-002/P0]**: auto-port — unused port 선택, Verify: integration
- **WCF-003 [REQ-WCF-003/P0]**: CSRF double-submit — 없거나 위조 시 403, Verify: integration
- **WCF-004 [REQ-WCF-004/P0]**: Origin allowlist — 비허용 Origin 403, Verify: integration
- **WCF-005 [REQ-WCF-005/P0]**: atomic config write — kill -9 시 partial state 0, Verify: chaos test
- **WCF-008 [REQ-WCF-008/P0]**: `mink config web` 시작 + 브라우저 자동 오픈, Verify: integration
- **WCF-009 [REQ-WCF-009/P0]**: graceful shutdown ≤ 5s — SIGTERM/Ctrl+C, Verify: integration
- **WCF-014 [REQ-WCF-014/P0]**: 단일 active session — 두 번째 client 거부, Verify: integration

## §2 M2 (4 AC)
- **WCF-007 [REQ-WCF-007/P1]**: `mink config set <key> <value>` 와 동등 — CLI vs Web 결과 일치, Verify: integration (CLI-TUI parity)
- **WCF-010 [REQ-WCF-010/P0]**: provider credential 업데이트 — AUTH Store + test API, Verify: integration
- **WCF-011 [REQ-WCF-011/P1]**: channel revoke — OAuth revoke endpoint, Verify: integration
- **WCF-012 [REQ-WCF-012/P1]**: memory retention 변경 — MEMORY-QMD prune queue, non-blocking, Verify: integration

## §3 M3 (3 AC)
- **WCF-015 [REQ-WCF-015/P1]**: credential validation 중 spinner — UI 상태, Verify: e2e (Playwright)
- **WCF-016 [REQ-WCF-016/P2]**: brand theme onboarding 정합 — 동일 색상/폰트, Verify: e2e visual
- **WCF-006 [REQ-WCF-006/P1]**: AGPL SPDX 헤더 신규 .go grep 0 missing, Verify: CI

## §4 M4 (9 AC)
- **WCF-013 [REQ-WCF-013/P2]**: AGPL consent 갱신 timestamp audit log 기록, Verify: integration
- **WCF-017 [REQ-WCF-017/P0]**: 0.0.0.0 bind 시도 거부 — REQ-WCF-001 강제, Verify: unit (negative test)
- **WCF-018 [REQ-WCF-018/P0]**: credential 로그 mask — `sk-****`, Verify: unit
- **WCF-019 [REQ-WCF-019/P1]**: 동시 mutation 거부 — mutex per key, Verify: integration (race detect)
- **WCF-020 [REQ-WCF-020/P2,OPT]**: MINK_CONFIG_WEB_PORT env — 지정 포트 사용, Verify: integration
- **WCF-021 [REQ-WCF-021/P2,OPT]**: audit log JSON export, Verify: integration
- **WCF-022 [REQ-WCF-022/P2,OPT]**: dry-run — mutations 미persist, Verify: unit
- **WCF-023 [REQ-WCF-006 보강/P0]**: go vet+golangci-lint CI gate clean, Verify: CI
- **WCF-024 [REQ-WCF-006 보강/P1]**: gofmt+lint 신규 .go clean, Verify: CI

## §5 DoD
24/24 GREEN, coverage ≥ 85% (internal/server/config/), Playwright E2E PASS, plan-auditor pass.

## SPEC-GOOSE-BRIDGE-001 Progress

- **Status**: 🟢 COMPLETE — daemon ↔ localhost Web UI bridge 16 AC 모두 implemented + main 머지 완료
- **Completion date**: 2026-05-04
- **Last commit**: `dff9648` (PR #91 M5 follow-up)
- **Implementation timeline**: 2026-05-04 단일 day, 8 PRs 순차 머지

## Milestone & PR 매핑

| Milestone | 범위 | PR | merge commit |
|-----------|------|----|----|
| M0 — Foundation | types + bind + registry + server skeleton | [#82](https://github.com/modu-ai/goose/pull/82) | `110059e` |
| M0 follow-up | error wraps + Stop race + Sessions contract (4 CodeRabbit findings) | [#90](https://github.com/modu-ai/goose/pull/90) | `815ac57` |
| M1 — Auth | cookie + CSRF + login/logout + revocation | [#84](https://github.com/modu-ai/goose/pull/84) | `c8590a5` |
| M2 — Mux | single listener (WebSocket + SSE + POST inbound) | [#85](https://github.com/modu-ai/goose/pull/85) | `e65132d` |
| M3 — Outbound | dispatcher + permission roundtrip | [#87](https://github.com/modu-ai/goose/pull/87) | `eadb9a4` |
| M4 — Buffer + Gate | offline buffer + flush-gate + reconnect/replay | [#88](https://github.com/modu-ai/goose/pull/88) | `8c7f673` |
| M5 — Rate-limit + Metrics | reconnect rate-limit + OTel metrics | [#89](https://github.com/modu-ai/goose/pull/89) | `ffa6315` |
| M5 follow-up | transport wire-in (resumer.Resume + sender bracket + login helper) + 5 CodeRabbit findings | [#91](https://github.com/modu-ai/goose/pull/91) | `dff9648` |

## AC Coverage (16/16)

| AC | REQ | 구현 PR |
|----|-----|---------|
| AC-BR-001 | REQ-BR-001 | #82 |
| AC-BR-002 | REQ-BR-002 | #84 |
| AC-BR-003 | REQ-BR-003 | #85 |
| AC-BR-004 | REQ-BR-004 | #89 |
| AC-BR-005 | REQ-BR-005 | #82 |
| AC-BR-006 | REQ-BR-006 | #85 |
| AC-BR-007 | REQ-BR-007 | #87 |
| AC-BR-008 | REQ-BR-008 | #87 |
| AC-BR-009 | REQ-BR-009 | #88 |
| AC-BR-010 | REQ-BR-010 | #88 (#91 transport wire-in) |
| AC-BR-011 | REQ-BR-011 | #85 (#91 SSE Resume wire-in) |
| AC-BR-012 | REQ-BR-014 | #84 |
| AC-BR-013 | REQ-BR-015 | #85 |
| AC-BR-014 | REQ-BR-016 (v0.2.0) | #84 |
| AC-BR-015 | REQ-BR-017 | #88 (#91 WS Resume wire-in) |
| AC-BR-016 | REQ-BR-018 (v0.2.0) | #89 (#91 RecordFailure wire-in) |

## 품질 지표 (final)

- `go test -race -count=1 ./internal/bridge/...`: pass
- `internal/bridge` coverage: **84.2 %** (M5 follow-up 측정), 모든 milestone ≥ 80 %
- `gofmt -l internal/bridge/`: clean
- `go vet ./internal/bridge/...`: clean
- TRUST 5 충족 (Tested / Readable / Unified / Secured / Trackable)

## Out-of-scope (다음 amendment 후보)

handoff prompt §"WebSocket connID = sid + randSuffix" 함정 그대로:

- 매 upgrade 마다 새 `connID` 가 발급되므로 buffer 키도 새것. `resumer.Resume(connID, headers)` 가 같은 connID 의 buffer 만 검색하므로 cross-connection replay 는 동작하지 않음 (현재는 시그니처 wire-in 까지만)
- 진짜 reconnect/replay 흐름은 cookie hash 기반 logical session 매핑이 필요. 별도 SPEC amendment (BRIDGE-001 v0.3) 또는 신규 SPEC 후보로 분리
- M5 follow-up 의 통합 테스트 (`followup_integration_test.go`) 는 "같은 sessionID 가 buffer 와 일치하는 단순 시나리오" 로 검증

## 진입 조건 (logical session 매핑 amendment 작업 시)

1. `git fetch && git log -3 main` — `dff9648` (PR #91) 머지 확인
2. `WebUISession` 에 `LogicalID` 필드 (= `cookieHash + transport`) 추가 명세
3. `dispatcher` / `outboundBuffer` / `resumer` 가 `LogicalID` 단위로 buffer 매핑
4. multi-tab 시나리오 (같은 cookie 로 동시 두 tab 접속) 명세 추가
5. handoff prompt 함정 #11 의 완전 해소

## 후속 학습 (BRIDGE-001 누적)

- gofmt 차이: push 전 `gofmt -w internal/bridge/` 필수
- `clockwork.FakeClock`: `Advance` 직전 `BlockUntil(N)` 으로 goroutine park 보장
- byte slice deep copy: registry sensitive 필드는 모든 경로 copy
- LSP stale: codegen / 신규 파일 직후 false positive — `go build` 로 즉시 verify
- M3 contract: ghost session → `ErrSessionUnknown`
- 병렬 `-race` 부하 시 `internal/hook` + `internal/mcp/transport` flake — 격리 실행 시 통과 (pre-existing)
- `AuthError.CookieHash` 노출이 RecordFailure 의 전제 (hash 없으면 streak 영원히 0)
- spec.md "분당 60회" 해석: 60th 까지 OK, 61st 차단 (`>` not `>=`)
- ObserveWrite/Drain owner = sender (transport 단). dispatcher 는 Stalled 검사 + metrics 만
- PR merge BLOCKED 의 흔한 원인: `required_conversation_resolution` + CodeRabbit unresolved actionable. `enforce_admins=false` 면 admin bypass 가능

---
Last Updated: 2026-05-04 (PR #91 머지 후 종결 메타 갱신)
Status Source: chore/bridge-001-completion-meta

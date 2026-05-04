---
spec: SPEC-GOOSE-BRIDGE-001-AMEND-001
parent_spec: SPEC-GOOSE-BRIDGE-001
parent_version: 0.2.1
version: 0.1.2
status: completed
created_at: 2026-05-04
updated_at: 2026-05-04
---

# Progress — SPEC-GOOSE-BRIDGE-001-AMEND-001

## 🟢 COMPLETE — Cookie-Hash Logical Session Mapping for Cross-Connection Replay

부모 SPEC-GOOSE-BRIDGE-001 v0.2.1 의 cross-connection replay 결손을 amendment 로 분리해 완전 해소. `connID = sid + randSuffix()` 함정으로 인해 v0.2.1 의 `outboundBuffer` / `resumer` 가 같은 connID 의 buffer 만 검색해 진짜 reconnect 시나리오에서 동작하지 않던 문제를 LogicalID 도입으로 해결.

## Milestone × PR 매핑

| Milestone | Scope | PR  | 머지 일자  |
| --------- | ----- | --- | ---------- |
| Spec      | v0.1.1 SPEC bundle (audit Iteration 1 → 2 GO 후) — 7 REQ × 8 AC, NIST SP 800-108 도메인 분리, REQ↔AC bijection, logout eager-drop 정책 | #93 | 2026-05-04 |
| M1        | `Authenticator.DeriveLogicalID` 메서드 + `WebUISession.LogicalID` 필드. HMAC input: `"bridge-logical-id-v1\x00" \|\| uvarint(len(cookieHash)) \|\| cookieHash \|\| transport`. 5 unit tests, 100% coverage. | #94 | 2026-05-04 |
| M2        | `Registry.LogicalID(connID) (string, bool)` lookup + ws.go/sse.go 의 WebUISession 생성 시점에 LogicalID 채움 (`h.cfg.Auth.DeriveLogicalID`). 5 unit tests, 100% coverage. | #95 | 2026-05-04 |
| M3        | dispatcher buffer rekey by LogicalID + sequence counter 단일 atomic.Uint64 per LogicalID + logout eager-drop hook (`Registry.SetLogoutHook`, ordering: hook BEFORE closers). `outboundDispatcher.bufferKey` / `nextSequenceByKey` / `dropLogicalBuffer` 추가. wire envelope §7.1 invariant lockdown test 포함. 11 tests (6 buffer_logical + 5 logout_drop). 85.1% coverage. | #96 | 2026-05-04 |
| M4        | resumer LogicalID lookup with fallback + cross-connection replay 통합 테스트 + multi-tab integration tests. `newResumer(buf, reg)` 시그니처 변경 (D1 audit carve-out, package-private additive). 8 integration tests (5 cross-conn + 3 multi-tab). | #97 | 2026-05-04 |

총 5 PR / 4 milestone / 8 AC / 7 REQ.

## AC × PR 매핑 (1:1 bijective + 1:N for REQ-004)

| AC                  | REQ                  | Milestone-Task                                          | PR    |
| ------------------- | -------------------- | ------------------------------------------------------- | ----- |
| AC-BR-AMEND-001     | REQ-BR-AMEND-001     | M1-T1 (LogicalID derivation + 도메인 분리 prefix)        | #94   |
| AC-BR-AMEND-002     | REQ-BR-AMEND-002     | M2-T1 (Registry.LogicalID lookup)                       | #95   |
| AC-BR-AMEND-003     | REQ-BR-AMEND-003     | M3-T2 (dispatcher SendOutbound buffer keying)           | #96   |
| AC-BR-AMEND-004     | REQ-BR-AMEND-004     | M4-T1 (cross-conn replay full)                          | #97   |
| AC-BR-AMEND-005     | REQ-BR-AMEND-004     | M4-T1 partial 변형 (X-Last-Sequence > 0)                | #97   |
| AC-BR-AMEND-006     | REQ-BR-AMEND-005     | M4-T2 (multi-tab buffer share + emit single)            | #97   |
| AC-BR-AMEND-007     | REQ-BR-AMEND-006     | M3-T4 (sequence monotonic per LogicalID, race test)     | #96   |
| AC-BR-AMEND-008     | REQ-BR-AMEND-007     | M3-T5 (logout eager-drop hook + integration test)       | #96   |

## Audit Trail

| Iteration | Verdict          | Major | Minor | 보고서 |
| --------- | ---------------- | ----- | ----- | ------ |
| 1         | CONDITIONAL GO   | 5 (D1~D5) | 5 (D6~D10) | `.moai/reports/plan-audit/SPEC-GOOSE-BRIDGE-001-AMEND-001-review-1.md` |
| 2         | **GO**           | 0     | 1 cosmetic E1 (resolved in spec PR) | `.moai/reports/plan-audit/SPEC-GOOSE-BRIDGE-001-AMEND-001-review-2.md` |

D1~D10 결함 모두 RESOLVED (D1 newResumer carve-out, D2 1:1 bijection, D3 wire envelope invariant, D4 logout drop policy, D5 NIST SP 800-108 도메인 분리, D6~D10 cosmetic + line citation 정정).

## 품질 지표 (Final)

- `go test -race -count=10 ./internal/bridge/...` PASS — 부모 v0.2.1 16 AC + amendment 22 tests 모두 race-clean
- `internal/bridge` coverage **85.1%** (parent baseline 84.2% 초과, 목표 ≥ 85% 달성)
- `go build` / `go vet` / `gofmt` 모두 clean
- `golangci-lint` 신규 코드 0 issues
- TRUST 5 충족

## Public API 보존 (HARD)

- `Bridge` interface 시그니처 변경 0건
- `dispatcher.SendOutbound(sessionID, t, payload)` 시그니처 변경 0건
- `resumer.Resume(sessionID, http.Header)` public method 시그니처 변경 0건
- `WebUISession` 추가 필드 1 개 (LogicalID), named-field literal 호환
- `newResumer` package-private 생성자만 additive arg (spec §7 D1 carve-out)
- `Registry.LogicalID(connID)` + `Registry.SetLogoutHook(fn)` 신규 method 만 additive

## 핵심 행동 변화

amendment 머지 후:
- WS / SSE 재접속 시 같은 cookie + transport 라면 이전 connID 의 buffered messages 가 자동 회수됨 (진짜 cross-connection replay 동작)
- multi-tab 의미: emit 은 명시된 connID 만 wire 로 받고, sibling tab 은 reconnect 시점에야 sibling 의 history 회수
- 로그아웃 시 buffer + sequence 즉시 drop → 후속 재접속이 이전 messages 회수 불가 (security invariant)

## Out-of-scope (다음 amendment 후보)

본 amendment 가 명시적으로 **다루지 않은** 항목 (spec §10 참조):
- dispatcher 의 broadcast 정책 (multi-tab live broadcast UX) — Open Question 1, 별도 SPEC `BRIDGE-001-AMEND-002` 또는 `BRIDGE-002` 후보
- Cross-transport replay (WS ↔ SSE) — §5.3 Alternative B 거부, transport 변경 시 명시적 fresh session 정책
- logical-id 비밀키 rotation / KMS 통합 — 기존 cookie HMAC 비밀키 lifecycle 그대로
- buffer evict 정책의 multi-tab 인지화 — 4MB / 500 msg 한도가 LogicalID 단위, 후속 SPEC 검토

## 누적 학습 (BRIDGE-001 + AMEND-001)

- gofmt 차이: push 전 `gofmt -w internal/bridge/` 필수 (CI Go check 통과 게이트)
- LSP stale: codegen / 신규 파일 직후 false positive — `go build` 즉시 verify (M1, M2 두 번 재확인)
- byte slice deep copy: registry sensitive 필드 (CookieHash, CSRFHash) 는 모든 read 경로에서 deep copy. LogicalID (string) 는 immutable 이라 자동 안전.
- HMAC 도메인 분리: 같은 secret 을 두 용도로 재사용 시 NIST SP 800-108 prefix prepend 필수 (audit D5 fix)
- Wire-vs-buffer SessionID swap 안전성: outboundEnvelope JSON 이 SessionID 직렬화하지 않음 (outbound.go:147~150) — schema 변경 시 본 패턴 깨짐, `@MX:WARN` lockdown
- Logout ordering: hook (eager drop buffer) → closers (terminate transport) → unregister sessions. atomic counter 로 ordering invariant 검증.
- AC↔REQ bijection: AC → REQ injective (각 AC 가 정확히 하나의 REQ verify) + REQ → AC at-least-one. Strict 1:1 은 변형 시나리오 (full vs partial) 가 분리될 때 1:N 허용.
- amendment SPEC 의 backwards compat 주장은 항상 production 코드 시그니처 verify (newResumer 가 public 표면 외 package-private additive 임을 §7 명시)

---
spec: SPEC-GOOSE-BRIDGE-001-AMEND-001
parent_spec: SPEC-GOOSE-BRIDGE-001
parent_version: 0.2.1
version: 0.1.2
methodology: TDD (RED-GREEN-REFACTOR)
harness: standard
created_at: 2026-05-04
updated_at: 2026-05-04
status: completed
---

# Task Decomposition — SPEC-GOOSE-BRIDGE-001-AMEND-001

> Cookie-hash logical session mapping amendment to BRIDGE-001 v0.2.1. 본 문서는 spec.md §9 TDD 전략과 §6 AC 8 개를 milestone 단위 atomic task 로 분해한다. 부모 v0.2.1 의 16 AC 회귀 0건이 HARD 게이트. v0.1.1 (audit Iteration 1 반영) 에서 AC 6→8, REQ 6→7, 1:1 bijective mapping 복원, logout eager-drop hook (M3-T5) 추가.

## Acceptance Criteria → Task Mapping (1:1 bijective)

| AC                  | REQ                  | Milestone-Task                                          | 상태   | PR    |
| ------------------- | -------------------- | ------------------------------------------------------- | ------ | ----- |
| AC-BR-AMEND-001     | REQ-BR-AMEND-001     | M1-T1 (LogicalID derivation + 도메인 분리 prefix)        | DONE   | #94   |
| AC-BR-AMEND-002     | REQ-BR-AMEND-002     | M2-T1 (Registry.LogicalID)                              | DONE   | #95   |
| AC-BR-AMEND-003     | REQ-BR-AMEND-003     | M3-T2 (dispatcher SendOutbound buffer keying)           | DONE   | #96   |
| AC-BR-AMEND-004     | REQ-BR-AMEND-004     | M4-T1 (cross-conn replay full)                          | DONE   | #97   |
| AC-BR-AMEND-005     | REQ-BR-AMEND-004     | M4-T1 partial 변형 (X-Last-Sequence > 0)                | DONE   | #97   |
| AC-BR-AMEND-006     | REQ-BR-AMEND-005     | M4-T2 (multi-tab buffer share + emit single)            | DONE   | #97   |
| AC-BR-AMEND-007     | REQ-BR-AMEND-006     | M3-T4 (sequence monotonic per LogicalID, race test)     | DONE   | #96   |
| AC-BR-AMEND-008     | REQ-BR-AMEND-007     | M3-T5 (logout eager drop hook + integration test)       | DONE   | #96   |

총 8 AC × 4 Milestone × 13 atomic task. **Bijection**: 각 REQ ↔ 정확히 하나의 dedicated AC (REQ-004 의 두 변형 AC-004 + AC-005 는 동일 REQ 의 full/partial 행동을 분담).

## Milestones

### M1 — LogicalID derivation + WebUISession field

목표: `LogicalID` 결정성 헬퍼와 `WebUISession.LogicalID` 필드 추가. **부모 회귀 0건** (struct named-field literal 호환 검증 포함).

**Files**:
- `internal/bridge/logical_id.go` (신규, ~60 LoC) — `DeriveLogicalID(cookieHash []byte, transport Transport) string`. HMAC-SHA256 with cookie HMAC secret. Input format: `"bridge-logical-id-v1\x00" || uvarint(len(cookieHash)) || cookieHash || transport_string` (spec.md §3.1 + REQ-BR-AMEND-001, NIST SP 800-108 도메인 분리). RawURLEncoding output 32 chars.
- `internal/bridge/types.go` (수정, +1 field, ~5 LoC delta) — `WebUISession` 에 `LogicalID string` 필드 추가. godoc 영어 (CLAUDE.local.md §2.5).
- `internal/bridge/logical_id_test.go` (신규, ~100 LoC) — `TestDeriveLogicalID_Deterministic`, `TestDeriveLogicalID_TransportSeparated`, `TestDeriveLogicalID_CookieHashSeparated`, `TestDeriveLogicalID_EmptyCookieHashRejected`, `TestDeriveLogicalID_DomainPrefixPresent` (D5 검증 — prefix 변경 시 결과 달라짐).

**Tasks (atomic)**:

- M1-T1: `DeriveLogicalID` 함수 구현 + RED 테스트 5개 (결정성 100회, transport WS≠SSE, cookieHash 1byte 변경 시 다른 결과, 빈 cookieHash 시 빈 문자열 또는 panic 정책 결정, 도메인 prefix `"bridge-logical-id-v1\x00"` 가 input 에 실제로 포함됨 검증 — 다른 prefix 로 재산출 시 결과 다름). REQ-BR-AMEND-001.
- M1-T2: `WebUISession.LogicalID` 필드 추가. 부모 spec §6.3 의 7 필드 보존. `cloneSession` (registry.go) 도 LogicalID 복사 — string 은 immutable 이므로 단순 대입.

**Exit criteria**:
- `go build ./internal/bridge/...` clean.
- `go test -race ./internal/bridge/...` PASS — 부모 v0.2.1 16 AC 의 모든 테스트 통과 (HARD).
- `go vet ./internal/bridge/...` clean.
- `gofmt -l internal/bridge/` empty.
- M1 신규 파일 coverage ≥ 90%.

### M2 — Registry.LogicalID lookup

목표: `Registry` 에 `LogicalID(connID)` method 추가. dispatcher / resumer 가 사용할 단일 진실 출처 lookup.

**Files**:
- `internal/bridge/registry.go` (수정, +20 LoC) — `LogicalID(connID string) (string, bool)` method.
- `internal/bridge/registry_logical_test.go` (신규, ~120 LoC) — `TestRegistry_LogicalID_Hit`, `TestRegistry_LogicalID_Miss`, `TestRegistry_LogicalID_EmptyValue`, `TestRegistry_LogicalID_Concurrent` (race test).

**Tasks**:

- M2-T1: `LogicalID(connID)` lookup 구현. `Get(connID)` 와 동일 mutex 사용. 미등록 또는 LogicalID 빈 문자열 시 `("", false)`. RED → GREEN. REQ-BR-AMEND-002.
- M2-T2: ws.go / sse.go 의 `WebUISession{...}` literal 에 `LogicalID: DeriveLogicalID(cookieHash, transport)` 채움. struct literal named-field 형식 보존.

**Exit criteria**:
- 통합 테스트: 같은 cookie 의 두 connID 등록 후 둘 다 같은 LogicalID 반환 검증.
- coverage ≥ 90% (registry_logical_test.go).
- 부모 회귀 0건.

### M3 — Buffer + Dispatcher LogicalID keying + Logout hook

목표: outbound buffer 의 키 의미를 LogicalID 로 전환. dispatcher.SendOutbound 의 public 시그니처 보존하면서 내부 buffer Append 가 LogicalID 단위. dispatcher.sequences 도 LogicalID 단위로 일치. **추가 (v0.1.1)**: logout (`CloseSessionsByCookieHash`) 시 LogicalID buffer + sequence eager drop hook 구현 (REQ-BR-AMEND-007).

**Files**:
- `internal/bridge/buffer.go` (수정, ~20 LoC delta) — `Append(msg)` / `Replay(key, lastSeq)` / `Drop(key)` / `Len(key)` / `Bytes(key)` 의 키 인자 의미가 LogicalID 임을 godoc 으로 명시. 시그니처 자체는 같은 string — package-private 이므로 의미만 변경.
- `internal/bridge/outbound.go` (수정, ~40 LoC delta) — `outboundDispatcher.SendOutbound(sessionID, ...)`:
  1. `registry.LogicalID(sessionID)` lookup.
  2. lookup miss 시 fallback: sessionID 를 직접 키로 사용 (REQ-BR-AMEND-003 fallback 분기).
  3. lookup hit 시 `msg.SessionID = LogicalID` 로 buffer 만 keying (메시지의 wire envelope 의 SessionID 필드는 connID 보존을 위해 별도 분리 — implementation detail, 본 amendment 의 dispatcher 가 buffer 호출 직전에 msg 의 SessionID 만 LogicalID 로 swap 해서 buffer.Append, sender 호출은 원래 msg with connID). 안전성 근거: spec §7.1 (outboundEnvelope 가 SessionID 를 직렬화하지 않음, outbound.go:147~150).
  4. `nextSequence` 의 키도 LogicalID 로 전환 — `sequences map[string]*atomic.Uint64` 가 LogicalID 단위.
  5. **신규 (v0.1.1)**: `dropLogicalBuffer(LogicalID)` package-private method 추가 — buffer.Drop(LogicalID) + delete(sequences, LogicalID) + gate.Drop 보조. logout hook 에서 호출.
- `internal/bridge/registry.go` (수정, ~15 LoC delta, v0.1.1) — `CloseSessionsByCookieHash` 에 dispatcher logout hook 추가. closer invoke **이전에** 매칭된 connID 들의 LogicalID 를 수집하고 dispatcher.dropLogicalBuffer 호출. 의존성 방향이 registry → dispatcher 이므로, dispatcher 가 자신을 registry 에 logout-listener 로 등록하는 패턴 또는 Bridge struct 가 두 컴포넌트의 wiring 을 담당하는 패턴 중 implementation detail 로 결정.
- `internal/bridge/buffer_logical_test.go` (신규, ~150 LoC) — `TestBuffer_AppendByLogicalID`, `TestBuffer_ReplayByLogicalID`, `TestBuffer_FallbackToSessionIDWhenLogicalIDEmpty`, `TestDispatcher_SequenceMonotonicPerLogicalID` (race + parallel 200 calls), `TestDispatcher_WireEnvelopeIgnoresSessionIDSwap` (D3 invariant — outbound.go:147~150 의 envelope JSON 이 SessionID 를 포함하지 않음을 byte-equal 검증).
- `internal/bridge/logout_drop_test.go` (신규, ~120 LoC, v0.1.1) — `TestLogout_DropsLogicalBufferEagerly`, `TestLogout_ResetsSequenceCounter`, `TestLogout_PreventsCrossSessionReplay` (AC-BR-AMEND-008 검증).
- `internal/bridge/outbound_test.go` (수정) — 기존 dispatcher 테스트가 registry-less 로 동작하는 케이스 보존 (fallback 분기 검증).

**Tasks**:

- M3-T1: buffer.go 의 godoc 갱신 — Append(msg) 의 keying 의미가 LogicalID 임을 명시. (의미적 변경이므로 RED 테스트는 새 로직과 함께 작성)
- M3-T2: dispatcher.SendOutbound 의 buffer Append 호출 직전 LogicalID lookup. fallback 분기 명시. wire envelope invariant 검증 테스트 추가 (D3). REQ-BR-AMEND-003.
- M3-T3: dispatcher.nextSequence 의 키 전환 (sessionID → LogicalID). `sequences map` 의 GC (`dropSequence`) 의미는 transient disconnect 의 경우 lazy TTL 에 위임 (research §6.1). registry.Snapshot 또는 LogicalID 별 connID count 헬퍼 추가 검토. REQ-BR-AMEND-006.
- M3-T4: dispatcher 단조성 race 테스트 — 두 connID (같은 LogicalID) 에 100회씩 parallel SendOutbound, sequence 집합 정확히 {1..200} 검증. `go test -race -count=10` 로 검증. REQ-BR-AMEND-006.
- M3-T5 (v0.1.1 신규): Logout eager-drop hook 구현 + 통합 테스트. `Registry.CloseSessionsByCookieHash` 가 closer invoke 이전에 dispatcher.dropLogicalBuffer 를 호출하도록 wiring. RED 테스트는 buffer Len(L1) > 0 → CloseSessionsByCookieHash 호출 → buffer Len(L1) == 0 + sequence reset 검증. AC-BR-AMEND-008. REQ-BR-AMEND-007.

**Exit criteria**:
- coverage ≥ 85% on outbound.go + buffer.go + registry.go.
- 부모 v0.2.1 의 outbound_test.go / buffer_test.go / registry_test.go 회귀 0건.
- `go test -race -count=10 ./internal/bridge/...` clean (HARD — race 조건 검증 의무).
- AC-BR-AMEND-008 통합 테스트 PASS — logout 후 buffer 즉시 비워짐 + sequence 1 재시작.

### M4 — Cross-connection replay integration tests

목표: 진짜 reconnect 시나리오 통합 테스트. multi-tab 의미론 검증. 부모 v0.2.1 의 `m4_integration_test.go` / `followup_integration_test.go` 확장 (별도 신규 파일 권장 — 부모 테스트 보존을 위해).

**Files**:
- `internal/bridge/cross_conn_replay_test.go` (신규, ~250 LoC) — AC-BR-AMEND-003, AC-BR-AMEND-004.
- `internal/bridge/multi_tab_integration_test.go` (신규, ~200 LoC) — AC-BR-AMEND-005.

**Tasks**:

- M4-T1: cross-connection replay 통합 테스트 — `httptest.Server` + `coder/websocket` client. 시나리오:
  1. Tab-A 연결 (cookie c1, connID 자동 = sid+rand1).
  2. dispatcher 가 Tab-A 에 5 청크 emit (sequence 1~5).
  3. Tab-A close (client side).
  4. 같은 cookie c1 으로 Tab-B 연결 (connID = sid+rand2, LogicalID 동일).
  5. Tab-B 의 upgrade 헤더에 `X-Last-Sequence: 0` (전체 replay → AC-BR-AMEND-004) 또는 `X-Last-Sequence: 3` (partial → AC-BR-AMEND-005) 포함.
  6. Tab-B 가 frame 수신 — sequence 1~5 (전체) 또는 4,5 (partial). 누락 0, 중복 0, 순서 보존.
  AC-BR-AMEND-004, AC-BR-AMEND-005. REQ-BR-AMEND-004.
- M4-T2: multi-tab buffer 공유 통합 테스트:
  1. 같은 cookie c1 으로 Tab-A, Tab-B 동시 active.
  2. dispatcher.SendOutbound(connA, payloadA) + dispatcher.SendOutbound(connB, payloadB).
  3. Tab-A 가 wire 로 받은 메시지: payloadA only.
  4. Tab-B 가 wire 로 받은 메시지: payloadB only.
  5. Tab-A close. 같은 cookie c1 으로 Tab-C 재접속 (`X-Last-Sequence: 0`).
  6. Tab-C 가 wire 로 받은 메시지: payloadA + payloadB 모두.
  AC-BR-AMEND-006. REQ-BR-AMEND-005.
- M4-T3: SSE 측 동등 테스트 — `Last-Event-ID` 헤더로 partial replay. SSE 는 transport 가 다르므로 LogicalID 가 WS 와 분리됨을 검증. (동일 cookie 의 WS Tab + SSE Tab 은 buffer 공유하지 **않음** — REQ-BR-AMEND-001 의 transport 분리.)

**Exit criteria**:
- 통합 테스트 5+ 시나리오 PASS.
- 부모 v0.2.1 의 통합 테스트 회귀 0건 (특히 `m4_integration_test.go` 의 single-tab buffer scenario).
- 전체 `internal/bridge/` coverage ≥ 85% (부모 baseline 84.2% 향상).
- `go test -race -count=10 ./internal/bridge/...` clean (HARD — multi-tab + race 시나리오의 핵심 검증 게이트).

## TDD Entry Order (전체)

1. M1-T1: `DeriveLogicalID` (REQ-BR-AMEND-001) — 가장 단순한 RED 진입 (pure function, no I/O). 도메인 분리 prefix + length-prefix concat.
2. M1-T2: `WebUISession.LogicalID` 필드 + clone 처리.
3. M2-T1: `Registry.LogicalID(connID)` (REQ-BR-AMEND-002).
4. M2-T2: ws.go / sse.go 의 LogicalID 채움.
5. M3-T1: buffer.go godoc 갱신 (의미만 명시 — 시그니처 그대로).
6. M3-T2: dispatcher.SendOutbound 의 LogicalID 매핑 + fallback 분기 + wire envelope invariant 테스트 (REQ-BR-AMEND-003).
7. M3-T3: dispatcher.sequences 의 키 전환.
8. M3-T4: 단조성 race 테스트 (REQ-BR-AMEND-006, AC-BR-AMEND-007).
9. M3-T5 (v0.1.1 신규): Logout eager-drop hook (REQ-BR-AMEND-007, AC-BR-AMEND-008).
10. M4-T1: cross-connection replay 통합 테스트 — full + partial (REQ-BR-AMEND-004, AC-BR-AMEND-004 + AC-BR-AMEND-005).
11. M4-T2: multi-tab buffer 공유 (REQ-BR-AMEND-005, AC-BR-AMEND-006).
12. M4-T3: SSE transport 분리 검증 (REQ-BR-AMEND-001 cross-check).

## Concurrent Implementation Strategy

- M1 / M2 는 병렬 가능 (다른 파일, 다른 책임 — types.go vs registry.go).
- M3 는 M2 의존 (Registry.LogicalID 가 dispatcher 의 prerequisite).
- M4 는 M3 의존 (buffer keying 이 통합 테스트의 prerequisite).
- 단일 PR 단위 권장: M1 단독 (1 PR), M2 단독 (1 PR), M3 묶음 (1 PR), M4 묶음 (1 PR).
- 총 4 PR 예상 (부모 v0.2.1 의 8 PR 패턴 대비 절반 — scope 가 더 좁음).

## External Dependencies (go.mod 추가 후보)

본 amendment 는 **신규 외부 의존성 0건**. `crypto/hmac`, `crypto/sha256`, `encoding/base64` 만 표준 라이브러리에서 사용. 부모 v0.2.1 이 이미 가져온 `coder/websocket`, `clockwork`, `otel` 그대로.

## Out of Scope (본 amendment 내에서 다루지 않음)

- dispatcher 의 broadcast (`BroadcastOutbound(LogicalID, ...)`) — spec.md §5.4 Open Question 1, 후속 SPEC.
- cross-transport replay (WS ↔ SSE) — spec.md §5.3 alternative B 거부.
- buffer 한도의 multi-tab 인지화 (LogicalID 별 4MB → 2 탭 합계 4MB) — spec.md §10 항목 8, 후속 SPEC.
- **transient disconnect** 의 dispatcher.sequences GC 정책 fine-tuning — spec.md §10 항목 5, M3-T3 의 implementation detail 로 흡수 (lazy TTL).
- **단, logout (의도적 invalidation) 의 eager drop 은 v0.1.1 에서 in-scope 로 승격** — REQ-BR-AMEND-007 + AC-BR-AMEND-008 + M3-T5 가 다룬다. 이는 transient disconnect 와 명확히 구분된다 (research §6.1 vs §6.2).

## TRUST 5 Gate Plan

- **T**ested: 각 milestone 별 coverage ≥ 85% (M1~M4 평균 90%+ 목표). M3-M4 race 시나리오는 `-race -count=10` 로 검증.
- **R**eadable: godoc 영어 + spec.md cross-reference (CLAUDE.local.md §2.5 준수).
- **U**nified: gofmt + golangci-lint clean.
- **S**ecured: HMAC-SHA256 + 기존 cookie HMAC secret 재사용 + **NIST SP 800-108 도메인 분리 prefix** `"bridge-logical-id-v1\x00"` (D5 fix) — 신규 키 관리 표면 없으면서 키 분리 원칙 준수. cookieHash 의 length-prefix concat 으로 boundary attack 방어 (research §3.2). Logout 시 LogicalID buffer eager drop (REQ-BR-AMEND-007) 으로 invalidation oracle 위험 차단.
- **T**rackable: 각 PR 의 conventional commit + SPEC reference (`SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001`) + `@MX:ANCHOR` (DeriveLogicalID, Registry.LogicalID, dispatcher LogicalID 매핑, dropLogicalBuffer hook).

## @MX Tag Plan

| 위치                                         | tag                            | 이유                                           |
| -------------------------------------------- | ------------------------------ | ---------------------------------------------- |
| `DeriveLogicalID`                            | `@MX:ANCHOR` + `@MX:REASON`    | LogicalID 결정성 invariant (REQ-BR-AMEND-001) + 도메인 분리 prefix |
| `Registry.LogicalID`                         | `@MX:ANCHOR`                   | 단일 진실 출처 lookup, fan_in ≥ 3 (dispatcher, resumer, tests) |
| `outboundDispatcher.SendOutbound` (buffer 매핑 분기) | `@MX:NOTE`                     | LogicalID lookup 의 fallback 분기 명시         |
| `outboundDispatcher` (buffer SessionID swap)  | `@MX:WARN` + `@MX:REASON`      | wire envelope 가 SessionID 직렬화하지 않는 invariant 의존 (spec §7.1, outbound.go:147~150). envelope schema 변경 시 본 swap 패턴이 안전성 보장하지 못함 |
| `outboundBuffer.Append` (godoc)              | `@MX:NOTE`                     | 키 의미가 connID 에서 LogicalID 로 변경됨 명시 |
| `dispatcher.sequences` 키 전환               | `@MX:WARN` + `@MX:REASON`      | sequence 단조성 invariant 가 LogicalID 단위로 옮겨감 — multi-tab race 위험 |
| `outboundDispatcher.dropLogicalBuffer` (v0.1.1) | `@MX:ANCHOR` + `@MX:REASON` | logout eager-drop 보안 invariant (REQ-BR-AMEND-007) — registry.CloseSessionsByCookieHash hook |
| `Registry.CloseSessionsByCookieHash` (v0.1.1, hook 추가) | `@MX:WARN` + `@MX:REASON` | dispatcher 와의 ordering 의존: closer invoke **이전** 에 dropLogicalBuffer 가 호출되어야 함 |

## 진입 조건 (구현 세션)

1. `git fetch && git log -3 main` — 부모 v0.2.1 의 PR #91 (`dff9648`) 머지 확인.
2. `git checkout -b feature/SPEC-GOOSE-BRIDGE-001-AMEND-001-m1-derive`.
3. `/moai run SPEC-GOOSE-BRIDGE-001-AMEND-001` 또는 `manager-tdd` 직접 위임 (M1 milestone scope guard).
4. 진입 prompt: "Implement M1 only (`logical_id.go` + `WebUISession.LogicalID` field + tests). RED-GREEN-REFACTOR per spec.md §9. **Parent v0.2.1 regression 0건 HARD**. Stop at end of M1."

## Total

- **Atomic tasks**: 12 (v0.1.1: M3-T5 logout hook 신규 추가 — M1: 2 + M2: 2 + M3: 5 + M4: 3 = 12)
- **Production files**: 6 modified
  - 신규 1: `logical_id.go`
  - 수정 5: `types.go`, `registry.go`, `outbound.go`, `buffer.go` (godoc + dropLogicalBuffer 추가), `ws.go` + `sse.go` (LogicalID 채움)
- **Test files**: 6 new
  - `logical_id_test.go`
  - `registry_logical_test.go`
  - `buffer_logical_test.go`
  - `logout_drop_test.go` (v0.1.1)
  - `cross_conn_replay_test.go`
  - `multi_tab_integration_test.go`
- **LoC 예상**: production ~180 (v0.1.1: +30 for logout hook) / test ~820
- **PR 예상**: 4 (M1, M2, M3 (T1-T5 묶음), M4)
- **Milestone 수**: 4 (M1~M4)
- **REQ / AC**: 7 REQ × 8 AC (v0.1.1: REQ-BR-AMEND-007 + AC-BR-AMEND-008 추가)

---

## 부모 회귀 검증 체크리스트 (HARD — 모든 PR 머지 게이트)

- [ ] `go test -race -count=10 ./internal/bridge/...` PASS
- [ ] 부모 v0.2.1 의 16 AC 통합 테스트 (`m4_integration_test.go`, `followup_integration_test.go`) 변경 없이 PASS
- [ ] `gofmt -l internal/bridge/` empty
- [ ] `go vet ./internal/bridge/...` clean
- [ ] `golangci-lint run ./internal/bridge/...` 0 issues
- [ ] coverage ≥ 84.2% (parent baseline) — 목표 85%+
- [ ] `Bridge` interface 시그니처 변경 0건 (정적 검증: `go doc ./internal/bridge` baseline diff)
- [ ] `dispatcher.SendOutbound`, `resumer.Resume` public 시그니처 변경 0건
- [ ] `WebUISession` struct 의 named-field literal 사용처 0건 회귀 (positional literal 미사용 grep 검증)

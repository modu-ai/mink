# SPEC-GOOSE-CMDLOOP-WIRE-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd, run phase 결정 시 변경 가능)
- Harness: standard (file_count<10 예상, 단일 Go domain — `internal/query/cmdctrl/`, security/payment 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/query/cmdctrl/` 신규, 의존 SPEC 인터페이스/구조는 implemented FROZEN 유지
- Branch base: main (의존 SPEC 모두 main 머지됨, PR #50 / #52 / SPEC-GOOSE-CONTEXT-001 / SPEC-GOOSE-QUERY-001)
- Parent SPEC: SPEC-GOOSE-CMDCTX-001 (PR #52, c018ec5 / 6593705) — `LoopController` 인터페이스 정의 자산을 본 SPEC 이 구현

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 자산 확인:
    - `internal/command/adapter/controller.go:19-51` — LoopController interface (4 method) FROZEN
    - `internal/command/adapter/adapter.go:107-176` — ContextAdapter 위임 경로 read-only 사용
  - 의존 SPEC 3종 (CMDCTX-001 v0.1.1, CONTEXT-001, QUERY-001 v0.1.3) 모두 implemented status, FROZEN — 변경하지 않는다
  - 본 SPEC 산출물: research.md, spec.md, progress.md (본 파일)
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 분기 결정.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~280 | wiring surface analysis, 메서드 매트릭스, 위험 영역 식별, 옵션 A/B/C 비교 |
| spec.md | ~500 | EARS 20 REQ + 19 AC, 기술 접근, 의존성, Exclusions 12개 |
| progress.md | ~30 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: 20
  - Ubiquitous: 7 (REQ-001 ~ 007)
  - Event-Driven: 5 (REQ-008 ~ 012)
  - State-Driven: 3 (REQ-013 ~ 015)
  - Unwanted: 3 (REQ-016 ~ 018)
  - Optional: 2 (REQ-019, 020)
- 총 AC: 19 (각 REQ 최소 1개 매핑, 일부 REQ는 다중 AC)
- 커버리지 매트릭스: spec.md §5 참고

### 사용자 결정 보류 항목 (run phase 진입 전 확인 필요)

| ID | 결정 사항 | 본 SPEC 의 권장안 | 영향 |
|----|--------|----------------|------|
| D-001 | 구현 패키지 위치 (옵션 A: `internal/query/loop/cmdctrl/` / B: `internal/command/adapter/loopctrl/` / C: `internal/query/cmdctrl/`) | **옵션 C** (query 패키지와 동일 모듈 트리, cycle 없음) | 코드 위치 |
| D-002 | loop iteration drain 훅 주입 방식 (C-i: `LoopConfig.PreIteration` 신규 필드 / C-ii: engine.SubmitMessage 진입 시 drain) | **옵션 C-i** (loop iteration 시점이 정확) | QUERY-001 의 loop.go 1줄 추가, frontmatter version bump 가능 |
| D-003 | Snapshot 의 TokenCount source | **0 fix** (후속 SPEC-GOOSE-CMDLOOP-TOKEN-WIRE-001 가칭으로 위임) | LoopSnapshot.TokenCount 가 본 SPEC 에서는 항상 0 |
| D-004 | RequestReactiveCompact(target) 의 target 파라미터 처리 | **무시** (compactor 기본값 사용, 후속 SPEC 으로 위임) | Exclusions §10 #2 |

### 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, 의존 SPEC FROZEN 변경 부재 확인, REQ-AC 매트릭스 완전성, R-001 ~ R-007 mitigation 적정성 점검
- [ ] 사용자 ratify: AskUserQuestion 통해 D-001 ~ D-004 결정 + 진행 여부
- [ ] /moai run SPEC-GOOSE-CMDLOOP-WIRE-001 (사용자 승인 시)
  - Phase 2B TDD 예상 task: T-001(타입/sentinel error 정의) → T-002(Snapshot) → T-003(RequestModelChange + atomic.Pointer) → T-004(RequestClear + atomic.Bool) → T-005(RequestReactiveCompact) → T-006(applyPendingRequests + PreIteration 주입) → T-007(race + nil paths)
  - 예상 산출물: `internal/query/cmdctrl/` 패키지 (3-4 파일, ~500 LOC)
  - 부수 산출물 (옵션 C-i 채택 시): `internal/query/loop/loop.go` PreIteration 훅 1줄 추가 + `internal/query/engine.go` controller 주입 ~20 LOC
  - 커버리지 ≥ 90%, race detector pass

### 주의 사항

- 본 SPEC 은 `LoopController` 인터페이스를 **변경하지 않는다**. CMDCTX-001 의 controller.go 는 FROZEN.
- 본 SPEC 은 `ContextAdapter` (CMDCTX-001 의 adapter.go) 도 **변경하지 않는다**.
- 본 SPEC 은 dispatcher (PR #50 SPEC-GOOSE-COMMAND-001) 도 **변경하지 않는다**.
- CLI 진입점 wiring (adapter instantiate + dispatcher 주입 + LoopControllerImpl 인스턴스화) 은 본 SPEC 범위 외. 후속 SPEC (CLI-001 / DAEMON-WIRE-001) 가 담당.
- 옵션 C-i 채택 시 `internal/query/loop/loop.go` 의 `LoopConfig` 에 `PreIteration func(state *State)` nil-tolerant 필드 1개 추가. 후방 호환 변경이지만 QUERY-001 SPEC 의 frontmatter / HISTORY 갱신 필요 여부는 사용자 결정.
- 다른 동시 작성 중인 SPEC (CLI / DAEMON / ALIAS-CONFIG 등) 의 디렉토리는 건드리지 않는다.

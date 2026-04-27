# SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd, run phase 결정 시 변경 가능)
- Harness: standard (file_count<10 예상, 단일 Go domain — `internal/query/cmdctrl/` 의 옵션/본문 확장, security/payment 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/query/cmdctrl/credresolver.go` 신규, `controller.go` / `errors.go` 후방 호환 확장. 의존 SPEC 인터페이스/구조는 implemented FROZEN (CMDCTX-001 / CREDPOOL-001) 또는 planned FROZEN-on-implement (CMDLOOP-WIRE-001) 유지.
- Branch base: `feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan` (현재 작업 브랜치, batch plan 단계)
- Parent SPECs:
  - SPEC-GOOSE-CMDCTX-001 (implemented, v0.1.1) — `OnModelChange` 위임 경로의 OUT-SCOPE #4 명시 호출자
  - SPEC-GOOSE-CMDLOOP-WIRE-001 (planned, batch A, v0.1.0) — `LoopControllerImpl` 본체의 후방 호환 확장 대상
  - SPEC-GOOSE-CREDPOOL-001 (implemented, v0.3.0) — credential pool API surface read-only 의존

### Phase Log

- 2026-04-27 plan phase 시작 (batch B 신규 SPEC 1건)
  - 부모 자산 확인 (read-only):
    - `internal/llm/credential/pool.go:60-76, 237-273, 279-320, 324-337, 372-379` — New / Select / triggerRefreshLocked / Release / Size
    - `internal/llm/credential/refresher.go:12-16` — Refresher 인터페이스
    - `internal/llm/credential/strategy.go:25-44, 50-70, 76-119, 125-166` — 4 strategy
    - `internal/llm/credential/factory.go:46-82` — NewPoolsFromConfig (provider→pool map)
    - `internal/command/adapter/adapter.go:140-168` — ContextAdapter.OnModelChange 위임 경로 (FROZEN)
    - `internal/command/adapter/controller.go:19-51` — LoopController interface (FROZEN)
    - `.moai/specs/SPEC-GOOSE-CMDLOOP-WIRE-001/spec.md` §6.3 — RequestModelChange 알고리즘 (planned)
  - 의존 SPEC 정합성 (research.md §9):
    - CREDPOOL-001 v0.3.0 implemented — API surface 검증 완료 (line 단위)
    - CMDCTX-001 v0.1.1 implemented — controller.go / adapter.go 변경 금지 확인
    - CMDLOOP-WIRE-001 v0.1.0 planned — 본 SPEC 의 변경 대상이 그 SPEC 의 `LoopControllerImpl` 의 `New` 옵션 + RequestModelChange 본문에 한정. CMDLOOP-WIRE-001 implemented 후 본 SPEC run phase 가능.
  - 본 SPEC 산출물: research.md, spec.md, progress.md (본 파일)
  - 다른 동시 작성 SPEC 디렉토리 (CMDCTX-PERMISSIVE-ALIAS-001 / ALIAS-CONFIG-001 / CMDLOOP-WIRE-001 등) 미접근.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~340 | wiring surface analysis, API matrix, hook point options (A/B/C/D), refresh timing decision (옵션 A/B/C), failure modes (F-1~F-8), risk surface (R-001~R-008), 결정 후보 (D-001~D-007) |
| spec.md | ~480 | EARS 23 REQ + 20 AC, 기술 접근 (패키지 레이아웃, RequestModelChange 본문 확장, lease 정책, race 보장), 의존성 5종, Exclusions 12개 |
| progress.md | ~85 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: 23
  - Ubiquitous: 7 (REQ-CCWIRE-001 ~ 007)
  - Event-Driven: 6 (REQ-CCWIRE-008 ~ 013)
  - State-Driven: 3 (REQ-CCWIRE-014 ~ 016)
  - Unwanted: 4 (REQ-CCWIRE-017 ~ 020)
  - Optional: 3 (REQ-CCWIRE-021 ~ 023)
- 총 AC: 20 (각 REQ 최소 1개 매핑, 일부 REQ 는 다중 AC, 일부 AC 는 다중 REQ 검증)
- 커버리지 매트릭스: spec.md §5 참고

### 사용자 결정 보류 항목 (run phase 진입 전 확인 필요)

| ID | 결정 사항 | 본 SPEC 의 권장안 | 영향 |
|----|--------|----------------|------|
| D-001 | Hook 주입 방식 (옵션 A: `CredentialPoolResolver` dependency / B: adapter 변경 / C: dispatcher 변경 / D: middleware) | **옵션 A** (LoopControllerImpl 옵션, 후방 호환) | adapter.go / dispatcher 변경 없음. CMDLOOP-WIRE-001 의 `New` 시그니처에 옵션 추가만. |
| D-002 | Refresh trigger 시점 (A: 첫 Select 자동 / B: swap 동기 / C: preWarm 비동기) | **A 기본 + C optional** | swap latency 보존, preWarm 은 옵션 enable 시만 |
| D-003 | 신규 pool 의 `available == 0` 시 fallback | **swap 거부 + ErrCredentialUnavailable** | activeModel 변경 안 됨. 사용자가 인지 후 OAuth 재로그인 또는 다른 provider 선택 |
| D-004 | preWarm goroutine 의 lifecycle (Close hook 대기) | **sync.WaitGroup 또는 atomic counter 추가, 명시적 Close 도입 시까지 Wait 호출 없음** | Optional REQ-CCWIRE-023 |
| D-005 | `extractProvider` 알고리즘 | `strings.SplitN(info.ID, "/", 2)[0]`, 빈 provider 는 `""` | 기존 CMDCTX-001 ResolveModelAlias 와 일관 |
| D-006 | CMDLOOP-WIRE-001 의 `New` 시그니처에 옵션 추가가 그 SPEC 의 FROZEN 정책 위반인가 | **후방 호환 변경 (0.1.x → 0.2.0 frontmatter version bump 합의 필요)** | 본 SPEC 머지 시 동시 patch. CMDLOOP-WIRE-001 의 ratify 단계에서 합의. |
| D-007 | `ErrCredentialUnavailable` 의 위치 | `internal/query/cmdctrl/errors.go` (CMDLOOP-WIRE-001 sentinel 그룹과 응집) | sentinel error 단일 위치 정책 |

### 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, FROZEN SPEC 변경 부재 확인 (CMDCTX-001 / CREDPOOL-001 패키지 diff = 0), REQ-AC 매트릭스 완전성, R-001 ~ R-009 mitigation 적정성 점검
- [ ] 사용자 ratify: AskUserQuestion 통해 D-001 ~ D-007 결정 + 진행 여부
- [ ] CMDLOOP-WIRE-001 의 implemented 대기 (본 SPEC 의 run phase 진입 전제 조건)
- [ ] /moai run SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 (사용자 승인 + CMDLOOP-WIRE-001 implemented 시)
  - Phase 2B TDD 예상 task: T-001 (인터페이스/sentinel/extractProvider) → T-002 (옵션 추가) → T-003 (RequestModelChange 본문 확장) → T-004 (preWarmRefreshAsync helper) → T-005 (logger 통합) → T-006 (race + nil paths) → T-007 (정적 분석 검증)
  - 예상 산출물:
    - 신규 파일: `internal/query/cmdctrl/credresolver.go` (~80 LOC), `internal/query/cmdctrl/credresolver_test.go` (~200 LOC)
    - 수정 파일: `internal/query/cmdctrl/controller.go` (+~25 LOC), `internal/query/cmdctrl/errors.go` (+1 sentinel)
  - 커버리지 ≥ 90%, race detector pass, golangci-lint clean

### 주의 사항

- 본 SPEC 은 CMDCTX-001 의 `ContextAdapter` (adapter.go) 또는 `LoopController` (controller.go) 인터페이스를 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 COMMAND-001 의 dispatcher (PR #50) 도 **변경하지 않는다**. FROZEN.
- 본 SPEC 은 CREDPOOL-001 의 `internal/llm/credential/` 패키지 어떤 파일도 **변경하지 않는다**. FROZEN.
- 본 SPEC 의 변경 범위는 `internal/query/cmdctrl/` 에 한정 (CMDLOOP-WIRE-001 가 도입한 패키지). 신규 파일 1개 (credresolver.go) + 본 SPEC 의 옵션/본문 추가 2개 파일 (controller.go / errors.go).
- 본 SPEC 의 Optional REQ (REQ-CCWIRE-021/022/023) 는 run phase 에서 사용자 결정에 따라 분기 가능. 기본 구현은 핵심 EARS (REQ-CCWIRE-001 ~ 020) 완료 후 결정.
- 다른 동시 작성 중인 SPEC (CMDCTX-PERMISSIVE-ALIAS-001 / ALIAS-CONFIG-001 / CMDLOOP-WIRE-001) 의 디렉토리는 본 plan 단계에서 read-only 참조 외 접근 금지.
- CMDLOOP-WIRE-001 의 D-001 (패키지 위치) / D-002 (PreIteration 훅) 결정에 따라 본 SPEC 의 import path 가 달라질 가능성 — `internal/query/cmdctrl/` (옵션 C 권장) 가정. 다른 옵션 채택 시 본 SPEC 0.1.1 patch 필요.

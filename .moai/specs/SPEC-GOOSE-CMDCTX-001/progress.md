## SPEC-GOOSE-CMDCTX-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd, 후속 run phase 결정 시 변경 가능)
- Harness: standard (file_count<10 예상, 단일 Go domain, security/payment 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/command/adapter/` 신규, 의존 SPEC 인터페이스는 implemented 상태 유지
- Branch base: main (의존 SPEC 모두 main에 머지됨)

### Phase Log

- 2026-04-27 run phase 완료 (TDD RED-GREEN-REFACTOR, manager-tdd)
  - Branch: feature/SPEC-GOOSE-CMDCTX-001-impl
  - 산출물: `internal/command/adapter/` 패키지 (7 파일)
  - 구현 순서: T-001(타입 정의) → T-002(ResolveModelAlias) → T-003(SessionSnapshot) → T-004(PlanModeActive) → T-005(OnClear/OnCompactRequest) → T-006(OnModelChange) → T-007(race+nil paths)
  - Coverage: 100.0% (statements)
  - Race test: -count=10 PASS (100 goroutines × 1000 iter)
  - golangci-lint: 0 issues
  - gofmt: clean
  - AC-CMDCTX-019 정적 분석: 0건 (loop.State 직접 할당 없음)
  - 19 AC 전체 검증 완료
  - SPEC frontmatter status: planned → implemented (별도 커밋)

- 2026-04-27 plan phase 시작
  - 부모 SPEC: SPEC-GOOSE-COMMAND-001 (PR #50, c018ec5/6593705) 머지로 `SlashCommandContext` 인터페이스 노출
  - 위임 대상 SPEC 3종 (ROUTER-001, CONTEXT-001, SUBAGENT-001) 모두 implemented status, FROZEN — 변경하지 않음
  - 본 SPEC 산출물: research.md, spec.md, progress.md (본 파일)
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 분기 결정.

- 2026-04-27 plan-auditor iter1 결과 (FAIL) → spec.md v0.1.0 → v0.1.1 결함 수정
  - **M1 적용**: §3.1 IN SCOPE, §6.2 패키지 레이아웃 주석, §7 의존성 절 — `command.ErrUnknownModel` stale claim 정정. PR #50 (SPEC-GOOSE-COMMAND-001) 에서 `internal/command/errors.go:23-25` 에 이미 정의됨이 사실. 본 SPEC은 **재사용** (추가/수정 없음).
  - **M2 적용**: §6.2 ContextAdapter struct, `New(...)` / `SetPlanMode` / `WithContext` godoc, §6.5 PlanModeActive 알고리즘, §6.6 race 안전성 — `planMode atomic.Bool` (값 타입) → `planMode *atomic.Bool` (포인터 indirection) 으로 변경. 이유: `sync/atomic.Bool` 의 `noCopy` 가드 위반을 막기 위함. shallow-copy 기반 `WithContext` 가 `go vet copylocks` 경고를 유발하지 않으면서 부모/자식 adapter 간 plan-mode 상태 공유(single source of truth) 보장.
  - **M3 적용**: AC-CMDCTX-019 신설 — adapter 비-mutation invariant 의 정적 분석 검증(`grep -rE 'loop\.State\.[A-Z][A-Za-z]*\s*=' internal/command/adapter/`). REQ-CMDCTX-016 매핑에 추가. AC 18 → 19.
  - **M4 적용**: REQ-CMDCTX-016 을 §4.4 Unwanted Behavior 에서 §4.1 Ubiquitous 로 재배치. EARS 분류상 "shall not mutate" 형태의 시스템 상시 불변은 Ubiquitous 가 적절. REQ ID 는 016 유지.
  - **M5 적용**: AC-CMDCTX-016 본문 확장 — `fakeWarnLogger` 주입 후 `WarnCount >= 1` 검증 + 원본 에러 포함 확인. REQ-CMDCTX-018 의 logger 절을 단일 AC 로 통합 검증.
  - **N2 적용**: Exclusions #7-9 (Permissive alias mode / Hot-reload / Multi-session) 에 "TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요" 형식 추가. #1-6 와 형식 통일.
  - frontmatter: version 0.1.0 → 0.1.1, updated_at 유지(2026-04-27), HISTORY 항목 1줄 추가.
  - **REQ / AC 통계 갱신**: 총 REQ 18 (Ubiquitous 6, Event-Driven 5, State-Driven 3, Unwanted 2, Optional 2) / 총 AC 19. 모든 REQ 가 최소 1개의 AC로 검증.
  - 다음 단계: plan-auditor iter2 호출은 본 위임 범위 밖. 사용자 ratify 또는 manager-spec 후속 호출 시 진행.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~250 | Wiring surface analysis, 메서드 매트릭스, 위험 영역 식별 |
| spec.md | ~450 | EARS 18 REQ + 18 AC, 기술적 접근, 의존성, 제외 항목 |
| progress.md | ~30 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.1 기준)

- 총 REQ: 18 (Ubiquitous 6, Event-Driven 5, State-Driven 3, Unwanted 2, Optional 2)
  - v0.1.0 → v0.1.1: REQ-CMDCTX-016 재배치 (Unwanted 3 → 2, Ubiquitous 5 → 6)
- 총 AC: 19 (각 REQ 최소 1개 매핑, 일부 REQ는 다중 AC)
  - v0.1.0 → v0.1.1: AC-CMDCTX-019 신설 (REQ-CMDCTX-016 정적 분석)
- 커버리지 매트릭스: spec.md §5 참고

### 다음 단계 (제안)

- [ ] plan-auditor iter 1 (선택): EARS 형식 검증, 의존 SPEC 변경 부재 확인, REQ-AC 매트릭스 완전성 점검
- [ ] 사용자 ratify: AskUserQuestion 통해 진행 여부 결정 (TDD vs DDD, 즉시 run vs 추후)
- [ ] /moai run SPEC-GOOSE-CMDCTX-001 (사용자 승인 시)
  - Phase 2B TDD: T-001 ~ T-007 순차 구현
  - 예상 산출물: `internal/command/adapter/` 패키지 (8 파일 추정), coverage ≥ 90%, race detector pass
  - 부수 산출물: `internal/command/errors.go` 에 `ErrUnknownModel` sentinel 추가 (없는 경우)

### 주의 사항

- 본 SPEC은 `LoopController` 의 실제 구현체를 만들지 않는다. 인터페이스만 정의.
- CLI 진입점 wiring(adapter instantiate + dispatcher 주입)은 본 SPEC 범위 외. 후속 SPEC (CLI-001 / DAEMON-WIRE-001) 가 담당.
- 의존 SPEC (COMMAND-001 / ROUTER-001 / CONTEXT-001 / SUBAGENT-001) 의 spec.md / 코드 변경 금지.

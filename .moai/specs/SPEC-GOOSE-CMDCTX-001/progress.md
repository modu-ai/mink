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

- 2026-04-27 plan phase 시작
  - 부모 SPEC: SPEC-GOOSE-COMMAND-001 (PR #50, c018ec5/6593705) 머지로 `SlashCommandContext` 인터페이스 노출
  - 위임 대상 SPEC 3종 (ROUTER-001, CONTEXT-001, SUBAGENT-001) 모두 implemented status, FROZEN — 변경하지 않음
  - 본 SPEC 산출물: research.md, spec.md, progress.md (본 파일)
  - 다음 단계: plan-auditor 사이클은 본 위임 범위 외. 사용자 검토 후 /moai run 분기 결정.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~250 | Wiring surface analysis, 메서드 매트릭스, 위험 영역 식별 |
| spec.md | ~450 | EARS 18 REQ + 18 AC, 기술적 접근, 의존성, 제외 항목 |
| progress.md | ~30 | phase log (본 파일) |

### REQ / AC 통계

- 총 REQ: 18 (Ubiquitous 5, Event-Driven 5, State-Driven 3, Unwanted 3, Optional 2)
- 총 AC: 18 (각 REQ 최소 1개 매핑, 일부 REQ는 다중 AC)
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

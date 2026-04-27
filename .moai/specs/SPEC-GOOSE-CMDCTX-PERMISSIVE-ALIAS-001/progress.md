## SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd)
- Harness: minimal (file_count<5 예상, single-file enum addition + 분기 추가, security/payment 아님)
- Scale-Based Mode: XS (극소)
- Language: Go (moai-lang-go)
- Greenfield 여부: NO — `internal/command/adapter/` 기존 패키지 amendment. CMDCTX-001 v0.1.1 (implemented, FROZEN) 의 v0.2.0 amendment.
- Branch base: main (CMDCTX-001 implemented + main 머지 가정)
- Parent SPEC: SPEC-GOOSE-CMDCTX-001 v0.1.1 (implemented)
- Amendment target version: CMDCTX-001 v0.1.1 → v0.2.0

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 SPEC: SPEC-GOOSE-CMDCTX-001 v0.1.1 (`internal/command/adapter/` 기 구현됨, status implemented)
  - 본 SPEC 의 출처: CMDCTX-001 §9 R2 + §Exclusions #7 ("Permissive alias mode — TBD-SPEC-ID, 본 SPEC 머지 후 별도 plan 필요")
  - 변경 surface: ContextAdapter `Options` 에 `AliasResolveMode` enum 필드 추가 + `ResolveModelAlias` step 7 분기 추가 + warn-log + Optional `provider/*` wildcard alias key.
  - 변경 strategy: CMDCTX-001 SPEC 본문 변경은 본 SPEC 의 implementation phase 에만 (지금 plan 단계에서 변경 금지). spec.md §3.1 #8 / §6.9 / §7 에 amendment governance 명시.
  - 산출물: research.md, spec.md, progress.md (본 파일).
  - 다음 단계: 사용자 검토 / plan-auditor 사이클 (선택) / `/moai run` 분기 결정.

### 산출물 요약

| 파일 | 라인 수 추정 | 목적 |
|------|----------|------|
| research.md | ~210 | OpenRouter 사례 분석, 결정 옵션 3종(A/B/C) 비교, 옵션 A 채택 근거, CMDCTX-001 amendment 형태 정리 |
| spec.md | ~340 | EARS 12 REQ + 8 AC, 기술적 접근(§6.1~§6.12), CMDCTX-001 v0.2.0 amendment governance, Exclusions 8종 |
| progress.md | ~50 | phase log (본 파일) |

### REQ / AC 통계 (v0.1.0 기준)

- 총 REQ: 12 (Ubiquitous 3, Event-Driven 3, State-Driven 2, Unwanted 2, Optional 2)
- 총 AC: 8 (REQ-CMDCTX-PA-012 는 hook stub 만 정의 — 별도 AC 없음, 후속 TELEMETRY-001 책임)
- 검증 매핑: 11 REQ 가 최소 1개의 AC 로 검증, 1 REQ 는 hook 정의로 충족.
- 커버리지 매트릭스: spec.md §5 참고
- CMDCTX-001 의 19 AC 회귀 검증: spec.md §8.5 (backward compat 회귀 의무).

### 제약 / 주의

- **CMDCTX-001 SPEC 본문은 plan 단계에서 변경 금지**. 본 SPEC 의 spec.md 가 amendment 형태만 사전 정의. 실제 본문 갱신은 run phase 에서 수행 (§6.9).
- **CMDCTX-001 의 19 AC 보존 의무**. 신규 분기 추가 후에도 strict default 동작이 v0.1.1 와 동일해야 함 (REQ-CMDCTX-PA-001, REQ-CMDCTX-PA-007).
- **신규 외부 의존성 금지**. stdlib (`errors`, `strings`) + 기존 의존성 재사용.
- **mode 폭주 방지**. Strict / PermissiveProvider / Permissive 3종 한정 (REQ-CMDCTX-PA-002). 신규 mode 는 별도 SPEC amendment 필요.

### 다음 단계 (제안)

- (옵션) plan-auditor iter1 호출 — EARS 분류 적정성, AC 커버리지 완전성, amendment governance 명료성 검증.
- 사용자 ratify 후 `/moai run SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001` 진입:
  - T-001 ~ T-011 순차 TDD 진행 (spec.md §6.10 참조).
  - 동일 PR 에 CMDCTX-001 v0.2.0 amendment 본문 갱신 포함.
  - PR base: main, type=feature, priority=p4-low, area=router.
  - merge: squash (feature branch 단일 commit 원칙, CLAUDE.local.md §1.4).
- run phase 진입 시 branch 이름 권장: `feature/SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001-impl` (현재 작업 브랜치 `feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan` 와는 별도).

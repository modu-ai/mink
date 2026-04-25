---
id: SPEC-AGENCY-CLEANUP-002
version: 0.1.0
status: planned
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
priority: P2
format: compact
source: spec.md, plan.md, acceptance.md
---

# SPEC-AGENCY-CLEANUP-002 (Compact)

## Purpose

ABSORB-001 (completed) Open Items(D3/D4/D5)로 이관된 legacy agency 파일 19개 정리. 본 SPEC은 **계획만** 수립, 실제 삭제는 후속 `/moai run`.

## Target (19 artifacts)

- `.agency/` (8 파일): config.yaml, fork-manifest.yaml, context/5, templates/1
- `.claude/skills/agency-{client-interview,copywriting,design-system,evaluation-criteria,frontend-patterns}/` (5 dir)
- `.claude/agents/agency/{builder,copywriter,designer,evaluator,learner,planner}.md` (6 파일)

## EARS Requirements (5 patterns × 1)

| ID | Pattern | Summary |
|----|---------|---------|
| REQ-CL-001 | Ubiquitous | legacy-manifest.yaml로 19 artifact 추적 |
| REQ-CL-002 | Event-Driven | user 승인 시 `.archive/agency-legacy-{DATE}/` 백업 후 git rm |
| REQ-CL-003 | Unwanted | import violation 발견 시 halt + blocker |
| REQ-CL-004 | State-Driven | 단일 commit 유지로 rollback 보장 |
| REQ-CL-005 | Optional | `--archive-only` 모드 지원 (git mv rename) |

## Acceptance (5 AC, 각 3 scenario)

| AC | 핵심 검증 |
|----|----------|
| AC-CL-001 | manifest 생성, 19 entry, 필드 완전성 |
| AC-CL-002 | `.archive/` 존재 + sha256 일치 + 원자성 |
| AC-CL-003 | violation 주입 시 halt, 허용 예외 미트리거 |
| AC-CL-004 | single commit, `git reset --hard HEAD~1` 복원 |
| AC-CL-005 | `--archive-only` + `git log --follow` rename 추적 |

## Milestones (Priority 기반, 시간추정 없음)

1. CL-M1 (High): Manifest + import check
2. CL-M2 (High): Archive creation + sha256 verify
3. CL-M3 (High): `git rm -r` (standard) or `git mv` (archive-only)
4. CL-M4 (Medium): Single commit + rollback dry-run
5. CL-M5 (Medium): `/moai design` smoke test
6. CL-M6 (Low): HISTORY + 완료 리포트

## Dependencies

- 선행: SPEC-AGENCY-ABSORB-001 (v1.0.1, completed) Open Items D3/D4/D5
- 후속 (예상): SPEC-AGENCY-STUB-REMOVAL-003 (2 minor cycle 후)
- Out of scope: `.claude/rules/agency/constitution.md` REDIRECT stub, `/agency` 서브커맨드 stub 8개, v3.3 신규 artifact

## Risks

| ID | Risk | Severity | Mitigation |
|----|------|----------|------------|
| R-CL-01 | import 파괴 | High | REQ-CL-003 halt |
| R-CL-02 | accidental deletion | Critical | archive-first + single commit |
| R-CL-03 | `.gitignore .archive/` | Medium | `!.archive/agency-legacy-*` 예외 |
| R-CL-04 | sha256 불일치 | Low | atomic rollback |
| R-CL-05 | rename 미감지 | Low | `diff.renames=true` |
| R-CL-06 | legacy 복원 불가 | Medium | archive-only 권장 |

## Exclusions (HARD)

- 본 SPEC 실행 중 실제 파일 삭제 금지 (plan 전용)
- 코드(.go) 변경 금지
- `.claude/rules/agency/constitution.md` REDIRECT stub 수정 금지
- `/agency` 서브커맨드 stub 제거 금지
- v3.3 흡수 후 신규 artifact 수정 금지
- research.md 미생성 (소량 SPEC)
- ABSORB-001 REQ 번호 재배치 금지

## Commit Strategy

```
chore(cleanup): SPEC-AGENCY-CLEANUP-002 legacy agency 파일 19개 정리

...한국어 본문...

SPEC: SPEC-AGENCY-CLEANUP-002
REQ: REQ-CL-001, REQ-CL-002, REQ-CL-003, REQ-CL-004, REQ-CL-005
AC: AC-CL-001, AC-CL-002, AC-CL-003, AC-CL-004, AC-CL-005
```

Branch: `feature/SPEC-AGENCY-CLEANUP-002-cleanup` (CLAUDE.local.md §1.2) — 단일 squash merge

## HISTORY

- 2026-04-25 (v0.1.0): 초안. ABSORB-001 v1.0.1 audit patch의 D3/D4/D5 결함을 공식 cleanup SPEC으로 분리. research.md 미생성 (SPEC 소량 원칙).

---

Version: 0.1.0
Full detail: spec.md (detailed), plan.md (execution), acceptance.md (verification)

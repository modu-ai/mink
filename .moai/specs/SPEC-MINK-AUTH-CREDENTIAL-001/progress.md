---
spec_id: SPEC-MINK-AUTH-CREDENTIAL-001
artifact: progress.md
version: 0.2.0
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
---

# Progress — SPEC-MINK-AUTH-CREDENTIAL-001

본 SPEC 의 마일스톤 / task / AC 완료 추적. plan.md §3 의 4 마일스톤과 tasks.md 의 18 tasks, acceptance.md 의 32 AC 가 입력. status 전이는 spec.md frontmatter 와 일관 유지한다.

---

## 1. Overall Status

| 항목 | 값 |
|------|-----|
| spec.md status | `planned` (v0.2.0) |
| REQ 총 | 30 |
| AC 총 | 32 (UB 12 / ED 8 / SD 5 / UN 5 / OP 2) |
| 마일스톤 총 | 4 |
| Tasks 총 | 18 (M1: 5, M2: 4, M3: 5, M4: 4) |
| 완료된 AC | 0 / 32 |
| 완료된 Tasks | 0 / 18 |
| 완료된 마일스톤 | 0 / 4 |
| Overall 진행률 | 0% (plan 완료, run 미진입) |
| plan-auditor pass | 대기 (본 PR 머지 후 또는 후속 PR 에서) |

## 2. Milestone Progress

| 마일스톤 | priority | tasks | AC | 진행률 | status | 비고 |
|---------|----------|-------|------|--------|--------|------|
| M1 — keyring abstraction | High | 5 (T-001~T-005) | 10 (AC-CR-001, 002, 005, 007, 008, 009, 013, 014, 015, 020, 024) | 0% | not-started | audit B1 fix: 메인 책임 GREEN AC 만 unique 카운트. AC-CR-024 (UN, T-001 partial) 포함 |
| M2 — 평문 fallback + auto-detect + CLI config | High | 4 (T-006~T-009) | 8 (AC-CR-004, 006, 021, 022, 026, 027, 029, 030) | 0% | not-started | AC-CR-006 의 M2 부분 GREEN (T-009). OP placeholder (AC-CR-029, 030) T-008 GREEN |
| M3 — 8 credential schema + Codex OAuth + LLM-ROUTING-V2 wiring | High | 5 (T-010~T-014) | 10 (AC-CR-003, 010, 011, 016, 017, 018, 023, 025, 028, 031) | 0% | not-started | AC-CR-006 의 M3 최종 GREEN (T-013), 카운트는 메인 M2 에 귀속. LLM-ROUTING-V2 회귀 PASS 필수 |
| M4 — USERDATA-MIGRATE 통합 + CLI 완성 + OP placeholder | Medium | 4 (T-015~T-018) | 4 (AC-CR-012, 019, 032 + meta T-018) | 0% | not-started | USERDATA-MIGRATE-001 회귀 PASS, plan-auditor pass 포함 |

> **카운팅 방법론** (audit B1 fix): 마일스톤별 AC 카운트는 *메인 GREEN 책임 task* 기준 unique 카운트만 포함. 다중 task GREEN AC (예: AC-CR-006 = T-009 partial M2 + T-013 final M3) 는 메인 책임 마일스톤 (M2) 에만 카운트. 합 = 10 + 8 + 10 + 4 = 32 (overall AC 32 정합).

## 3. Task Status Table

| Task | 마일스톤 | priority | status | 핵심 AC | 의존 task | 비고 |
|------|----------|----------|--------|---------|-----------|------|
| T-001 | M1 | High | not-started | AC-CR-001, AC-CR-024 | (없음) | M1 진입 task |
| T-002 | M1 | High | not-started | AC-CR-002, AC-CR-005, AC-CR-009, AC-CR-013, AC-CR-014 | T-001 | go-keyring wrapper |
| T-003 | M1 | High | not-started | AC-CR-020 | T-002 | probe + KeyringUnavailable sentinel |
| T-004 | M1 | High | not-started | AC-CR-008, AC-CR-031 | T-002, T-003 | Health API + doctor subcommand |
| T-005 | M1 | High | not-started | AC-CR-001, AC-CR-007, AC-CR-015 | T-001, T-002, T-003 | M1 통합 테스트 |
| T-006 | M2 | High | not-started | AC-CR-004, AC-CR-026, AC-CR-027 | T-001 | FileBackend + atomic write |
| T-007 | M2 | High | not-started | AC-CR-021, AC-CR-022 | T-002, T-006 | Dispatcher |
| T-008 | M2 | High | not-started | AC-CR-021, AC-CR-029, AC-CR-030 | T-007 | CLI config + OP placeholder |
| T-009 | M2 | High | not-started | AC-CR-006, AC-CR-022 | T-006, T-007 | M2 통합 테스트 + cloud folder 경고 |
| T-010 | M3 | High | not-started | AC-CR-016 (initial) | T-007 | Codex PKCE flow |
| T-011 | M3 | High | not-started | AC-CR-016, AC-CR-018 | T-010 | auto-refresh |
| T-012 | M3 | High | not-started | AC-CR-017, AC-CR-025 | T-011 | invalid_grant detection |
| T-013 | M3 | High | not-started | AC-CR-006, AC-CR-011, AC-CR-028 | T-001, T-006 | 8 credential schema |
| T-014 | M3 | High | not-started | AC-CR-003, AC-CR-010, AC-CR-023 | T-013 | LLM-ROUTING-V2 wiring + 헌장 정합 회귀 |
| T-015 | M4 | Medium | not-started | AC-CR-019 | T-013, T-014 | USERDATA-MIGRATE 통합 |
| T-016 | M4 | Medium | not-started | AC-CR-012, AC-CR-015, AC-CR-032 | T-013, T-015 | CLI 완성 (8 login + logout) |
| T-017 | M4 | Medium | not-started | AC-CR-032 (문서) | T-001~T-016 | 사용자 문서 `.moai/docs/auth-credential.md` |
| T-018 | M4 | Medium | not-started | (meta) | T-001~T-017 | progress.md sync + plan-auditor pass |

## 4. AC Status Table

| AC | 분류 | REQ | task(s) | status | 비고 |
|----|------|-----|---------|--------|------|
| AC-CR-001 | UB | UB-1 | T-001, T-005 | not-started | |
| AC-CR-002 | UB | UB-2 | T-002 | not-started | |
| AC-CR-003 | UB | UB-3 | T-014 | not-started | static analysis |
| AC-CR-004 | UB | UB-2 | T-006 | not-started | |
| AC-CR-005 | UB | UB-7 | T-002 | not-started | |
| AC-CR-006 | UB | UB-6 | T-009, T-013 | not-started | M3 에서 완전 검증 |
| AC-CR-007 | UB | UB-7 | T-001, T-005 | not-started | |
| AC-CR-008 | UB | UB-8 | T-004 | not-started | |
| AC-CR-009 | UB | UB-9 | T-002 | not-started | CI matrix |
| AC-CR-010 | UB | UB-5, UN-5 | T-014 | not-started | static + I |
| AC-CR-011 | UB | UB-4 | T-013 | not-started | |
| AC-CR-012 | UB | UB-4 | T-016 | not-started | |
| AC-CR-013 | ED | ED-1 | T-002, T-016 | not-started | |
| AC-CR-014 | ED | ED-2 | T-002 | not-started | |
| AC-CR-015 | ED | ED-3 | T-005, T-016 | not-started | |
| AC-CR-016 | ED | ED-4 | T-010, T-011 | not-started | |
| AC-CR-017 | ED | ED-5 | T-012 | not-started | |
| AC-CR-018 | ED | SD-4 | T-011 | not-started | silent refresh (ED 영역 관찰) |
| AC-CR-019 | ED | ED-6, ED-7 | T-015 | not-started | export+import round-trip |
| AC-CR-031 | ED | UB-8, UB-9 | T-004 | not-started | doctor subcommand |
| AC-CR-020 | SD | SD-1 | T-003 | not-started | |
| AC-CR-021 | SD | SD-2 | T-007, T-008 | not-started | |
| AC-CR-022 | SD | SD-1 | T-007, T-009 | not-started | |
| AC-CR-023 | SD | SD-3 | T-014 | not-started | LLM-ROUTING-V2 graceful degrade |
| AC-CR-024 | SD | UN-1 | T-001 | not-started | log state observation |
| AC-CR-025 | UN | UN-4 | T-012 | not-started | |
| AC-CR-026 | UN | UN-2 | T-006 | not-started | |
| AC-CR-027 | UN | UN-6 | T-006 | not-started | |
| AC-CR-028 | UN | UN-3 | T-013 | not-started | |
| AC-CR-032 | UN | UN-2 (보조) + OP-3 | T-016, T-017 | not-started | doc + e2e |
| AC-CR-029 | OP | OP-1 | T-008 | not-started | placeholder |
| AC-CR-030 | OP | OP-2 | T-008 | not-started | placeholder |

> AC unique count = 32. 분포: UB 12 + ED 8 + SD 5 + UN 5 + OP 2 = 32. spec.md frontmatter `acceptance_total: 32` 와 acceptance.md §6 매트릭스 및 plan.md §4 매트릭스와 정합.

## 5. Status Legend

- `not-started`: 작업 미진입 (현재 모든 task 상태)
- `in-progress`: RED phase 진입 (실패 테스트 작성됨)
- `green`: GREEN phase (테스트 PASS)
- `refactored`: REFACTOR phase 완료 + LSP clean
- `blocked`: 외부 의존 (예: USERDATA-MIGRATE schema amendment 대기)

## 6. Stagnation / Re-planning Triggers (workflow-modes §Re-planning Gate)

- 3+ iteration 에서 새 AC GREEN 0건 → 사용자에게 gap analysis 보고
- 테스트 coverage 가 iteration 간 하락 → 재계획 트리거
- 사이클 내 신규 error > 수정 error → 재계획 트리거
- 외부 SPEC 회귀 (LLM-ROUTING-V2 / USERDATA-MIGRATE-001) 발생 → blocked status 전환

각 iteration 종료 시 본 표 (§3, §4) 갱신 + 누적 AC 완료 수 + error count 기록.

## 7. plan-auditor Pre-flight Checklist

본 PR (v0.2.0 plan 완료) 시 plan-auditor agent invocation 입력. 모두 PASS 시 spec.md status → planned 유지 + run phase 진입 준비.

- [ ] 산출 6 파일 모두 존재 (research / spec / plan / tasks / acceptance / progress)
- [ ] 카운트 정합: REQ 30 / AC 32 / 마일스톤 4 / Tasks 18 (전 파일 동일)
- [ ] AC 분포 정합: UB 12 / ED 8 / SD 5 / UN 5 / OP 2 (acceptance.md §6 + progress.md §1 + spec.md §6 동일)
- [ ] REQ ↔ AC 매핑: 30 REQ 중 29 가 1 이상 AC 에서 GREEN 대상, OP-4 placeholder 명시
- [ ] tasks ↔ AC 매핑: 18 tasks 가 모두 1 이상 AC 와 cross-reference (T-018 meta task 제외)
- [ ] OUT OF SCOPE (spec.md §3.2) 항목이 plan.md / tasks.md / acceptance.md 의 구현 작업으로 등장하지 않음
- [ ] spec.md §8 Surface Assumptions 6 가정이 acceptance AC 또는 후속 검증 task 로 매핑
- [ ] AGPL 헌장 §2 + ADR-001 정합 검증 task (T-014) 명시
- [ ] CROSSPLAT-001 §5.1 가드 정합 task (T-004) 명시
- [ ] USERDATA-MIGRATE-001 schema 확장 대상 task (T-015) 명시

---

## 8. v0.2.0 → 차기 버전 전이 트리거

| 전이 | 트리거 | spec.md `version` | spec.md `status` |
|------|--------|------------------|------------------|
| v0.2.0 → v0.2.1 | plan-auditor PASS (본 PR 머지 후) | 0.2.0 | planned (유지) |
| v0.2.1 → v0.3.0 | M1 진입 (T-001 첫 commit) | 0.3.0 | in-progress |
| v0.3.0 → ... | M2, M3, M4 진행 | 0.3.x | in-progress |
| v0.x.x → v1.0.0 | M4 종결 (32 AC 모두 GREEN, 외부 SPEC 회귀 PASS) | 1.0.0 | implemented |
| v1.0.0 → completed | sync phase 완료 (`/moai sync SPEC-MINK-AUTH-CREDENTIAL-001`) | 1.0.x | completed |

---

Version: 0.2.0
Last Updated: 2026-05-16
spec.md status: planned
REQ: 30 (UB 9 / ED 7 / SD 4 / UN 6 / OP 4)
AC: 32 (UB 12 / ED 8 / SD 5 / UN 5 / OP 2)
Milestones: 4 (all not-started)
Tasks: 18 (all not-started)
Overall progress: 0% (plan 완료, run 미진입)

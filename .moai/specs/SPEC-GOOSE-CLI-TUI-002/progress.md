# SPEC-GOOSE-CLI-TUI-002 Progress

> Initial scaffolding. /moai run 진입 후 phase 별 기록은 본 파일에 append. plan-auditor PASS 후 §0 plan completion signal 채움.

## Header

- **SPEC ID**: SPEC-GOOSE-CLI-TUI-002
- **Title**: goose CLI TUI 보강 (teatest harness + permission UI + streaming UX + session UX)
- **Started**: 2026-05-05 (plan phase)
- **Status**: 🟢 PLANNED — awaiting `/moai run`
- **Mode**: TDD (brownfield: tui/{model,view,update,client}.go 는 기존 characterization tests 위 += new RED tests; greenfield: permission/, snapshots/, sessionmenu/, editor/ 는 신규 RED 우선)
- **Harness**: standard (4 area, 17 task, 17 RED test, 8 snapshot — 복잡하지만 thorough 까지 가지 않음. plan-auditor 결정에 따라 재조정 가능)
- **Predecessor SPEC**: SPEC-GOOSE-CLI-001 v0.2.0 (completed) — FROZEN base
- **Author**: manager-spec
- **Priority**: P1
- **Labels**: tui, cli, bubbletea, permission, ux

---

## 0. Plan Completion Signal Block

plan-auditor iteration 1 verdict PASS (보고서: `.moai/reports/plan-audit/SPEC-GOOSE-CLI-TUI-002-review-1.md`). 이후 D1(AC-CLITUI-018 추가, REQ-005 traceability) + D4(AC-006 cite REQ-012 추가) + D5(시나리오 수 14+→18) 직접 정정 적용. D2/D3/D6 는 known minor defects 로 PR description + run 단계로 이월.

```yaml
plan_complete_at: 2026-05-05T12:00:00+09:00
plan_status: audit-ready
plan_audit:
  verdict: PASS
  iteration: 1
  report: .moai/reports/plan-audit/SPEC-GOOSE-CLI-TUI-002-review-1.md
  post_audit_fixes:
    - D1: AC-CLITUI-018 신규 추가 (REQ-005 testability)
    - D4: AC-006 REQ cite 에 REQ-CLITUI-012 추가
    - D5: AC 수 표기 14+ → 18
  deferred_minor:
    - D2: REQ-013 compound (ANSI sanitization + permission persist) → run 단계 split 검토
    - D3: REQ-004 라벨 [Ubiquitous] (실제 State-Driven syntax) → label-only, behavior 영향 없음
    - D6: P4-T2/T3 LoC 250 가드 — RED phase 측정 후 분리 결정
```

---

## 1. Phase Mapping (plan.md §1 align)

| Phase | Area | RED tests | Files | Status |
|-------|------|----------|-------|--------|
| **P1** | Area 1 — bubbletea teatest 하네스 + visual snapshot | 5 (P1-T1~T5) | snapshots/ NEW + 2 golden + go.mod MODIFY | 🟡 PENDING |
| **P2** | Area 3 — Streaming UX + statusbar 고도화 + multi-line editor | 5 (P2-T1~T5) | editor/ NEW + model/view/update.go MODIFY + 3 golden | 🟡 PENDING |
| **P3** | Area 2 — Tool call permission modal UI | 5 (P3-T1~T5) | permission/ NEW + agent.proto MODIFY + client/model/view/update.go MODIFY + 2 golden | 🟡 PENDING |
| **P4** | Area 4 — Multi-turn polish + session UX | 4 (P4-T1~T4) | sessionmenu/ NEW + model/update/dispatch(or slash).go MODIFY + 1 golden | 🟡 PENDING |

**Phase 진행 순서**: P1 → P2 (parallel 가능) → P3 → P4 (plan.md §2 ordering rationale)

**Total**: 17 tasks, 17 RED tests, 8 snapshot golden, 4 신규 패키지, 4 MODIFY 파일, 1 proto 변경.

---

## 2. AC 매핑 요약 (acceptance.md verbatim)

| AC | REQ | Phase | RED Test |
|----|-----|-------|---------|
| AC-CLITUI-001 | REQ-CLITUI-001 | P1 | TestSnapshot_Helper_Determinism_AcrossOSes |
| AC-CLITUI-002 | REQ-CLITUI-001 | P1 | TestSnapshot_ChatREPL_InitialRender |
| AC-CLITUI-003 | REQ-CLITUI-006 | P3 | TestPermission_Modal_OpensOnRequest |
| AC-CLITUI-004 | REQ-CLITUI-002+013 | P3 | TestPermission_AllowAlways_PersistsToDisk |
| AC-CLITUI-005 | REQ-CLITUI-013 | P3 | TestPermission_AllowOnce_DoesNotPersist |
| AC-CLITUI-006 | REQ-CLITUI-004+012 | P3 | TestPermission_StreamPaused_WhileModalOpen |
| AC-CLITUI-007 | REQ-CLITUI-011 | P2 | TestStatusbar_Streaming_Throughput |
| AC-CLITUI-008 | REQ-CLITUI-011 + CLI-001 REQ-CLI-009 | P2 | (snapshot regression) |
| AC-CLITUI-009 | REQ-CLITUI-008 | P2 | TestEditor_CtrlN_TogglesMode |
| AC-CLITUI-010 | REQ-CLITUI-008 | P2 | TestEditor_MultiLine_CtrlJ_NewlineEnter_Send |
| AC-CLITUI-011 | REQ-CLITUI-013 | P2 | TestRender_MarkdownCodeBlock_GlamourEscapes |
| AC-CLITUI-012 | REQ-CLITUI-010 | P4 | TestSession_Save_WritesJsonl |
| AC-CLITUI-013 | REQ-CLITUI-010 | P4 | TestSession_Load_RestoresWithInitialMessages |
| AC-CLITUI-014 | REQ-CLITUI-007 | P4 | TestSessionMenu_CtrlR_OpensList |
| AC-CLITUI-015 | REQ-CLITUI-009 | P4 | TestEdit_CtrlUp_ReplacesLastTurn |
| AC-CLITUI-016 | REQ-CLITUI-014 | P2 | TestStatusbar_CostEstimate_FromUsage |
| AC-CLITUI-017 | REQ-CLITUI-001 (회귀) | P1 | TestSlashHelp_LocalNoNetwork_Snapshot |

---

## 3. Iteration Log (run phase 시작 후 append)

| Iteration | Phase/Task | Acceptance criteria 충족 | error count delta | 비고 |
|-----------|-----------|------------------------|-------------------|------|

(empty — /moai run 진입 후 매 iteration 마다 row 추가)

---

## 4. 후속 (sync phase 후 채움)

### 머지 PR 목록

(empty — phase 별 PR 머지 후 추가)

### CHANGELOG 예고 (sync phase 자동)

- TUI: bubbletea teatest harness 도입 + 8 visual snapshot
- TUI: tool call permission modal UI (HOOK-001 의 일부 흡수)
- TUI: streaming UX 고도화 (spinner + token throughput + multi-line + glamour markdown)
- TUI: in-TUI `/save`/`/load` + Ctrl-R recent menu + Ctrl-Up edit/regenerate

### Scope Creep Resolution

- (P4-T2/T3 LoC 250 초과 시 별도 SPEC 분리 결정 — sync 시점에 commit)

---

Last Updated: 2026-05-05 (initial plan scaffolding)

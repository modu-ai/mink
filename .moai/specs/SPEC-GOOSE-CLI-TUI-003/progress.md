# SPEC-GOOSE-CLI-TUI-003 Progress

> /moai run 진입 후 phase 별 기록은 본 파일에 append.

## Header

- **SPEC ID**: SPEC-GOOSE-CLI-TUI-003
- **Title**: goose CLI TUI 보강 P2 (sessionmenu(Ctrl-R) + Ctrl-Up edit/regenerate + i18n 8 golden)
- **Started**: 2026-05-05 (plan phase)
- **Status**: 🟢 IMPLEMENTED — P1~P4 완료 (PR #113~#116 merged)
- **Mode**: TDD (brownfield: model/view/update.go MODIFY; greenfield: sessionmenu/, i18n/ 신규)
- **Harness**: standard
- **Predecessor SPEC**: SPEC-GOOSE-CLI-TUI-002 v0.1.0 (implemented, PR #107~#111) — FROZEN base
- **Author**: manager-spec
- **Priority**: P1
- **Labels**: tui, cli, bubbletea, i18n, sessionmenu, edit-regenerate
- **GitHub Issue**: #112

---

## 0. Plan Completion Signal Block

plan-auditor 3회 iteration 후 사용자 confirm으로 진행.
MP-1 (REQ 순서), MP-2 (EARS 형식 10/10), MP-3 (YAML frontmatter) 모두 PASS.
잔여 minor 결함 D-RESIDUAL-5/6는 REQ-001/002 수정으로 해소됨.

```yaml
plan_complete_at: 2026-05-05
plan_status: audit-ready
plan_audit:
  verdict: CONDITIONAL_GO  # MP-1/2/3 PASS; 3회 exhausted with minor residuals fixed
  iteration: 3
  reports:
    - .moai/reports/plan-audit/SPEC-GOOSE-CLI-TUI-003-review-1.md
    - .moai/reports/plan-audit/SPEC-GOOSE-CLI-TUI-003-review-2.md
    - .moai/reports/plan-audit/SPEC-GOOSE-CLI-TUI-003-review-3.md
  post_audit_fixes:
    - D1: REQ-CLITUI3-008/009 renumbered for sequential ordering
    - D2: 10개 AC EARS 형식 준수 (Given→While, If+then 삽입)
    - D3: REQ-003 split (open/populate only; navigation→REQ-008)
    - D4: REQ-009 Unwanted 패턴 교정 (If...then...shall not)
    - D5: REQ-001/002 Go 타입명 제거, 행동 언어로 교체
```

---

## 1. Phase Mapping

| Phase | Area | RED tests | Files | Status |
|-------|------|----------|-------|--------|
| **P1** | i18n catalog + loader + wire into model/view/permission/slash | 2 (TestI18N_Loads, TestI18N_Defaults) | i18n/ NEW + model/view/permission/view.go/slash.go MODIFY | 🟢 DONE (PR #113) |
| **P2** | sessionmenu/ + Ctrl-R handler | 3 (TestSessionMenu_Opens, TestSessionMenu_Nav, TestSessionMenu_Empty) | sessionmenu/ NEW + model/update.go MODIFY | 🟢 DONE (PR #114) |
| **P3** | Ctrl-Up edit/regenerate | 4 (TestEdit_EntersMode, TestEdit_Regenerates, TestEdit_EscCancels, TestEdit_NoopStreaming) | update.go MODIFY + model.go MODIFY | 🟢 DONE (PR #115) |
| **P4** | 9 golden files (1 base + 8 i18n) | 3 (TestSessionMenu_Golden, TestI18N_Ko_Golden, TestI18N_En_Golden) | testdata/snapshots/ NEW | 🟢 DONE (PR #116) |

**Phase 진행 순서**: P1 → P2 → P3 → P4

---

## 2. AC 매핑 요약

| AC | REQ | Phase | RED Test |
|----|-----|-------|---------|
| AC-CLITUI3-001 | REQ-001/008 | P2 | TestSessionMenu_CtrlR_OpensList |
| AC-CLITUI3-002 | REQ-001/008 | P2 | TestSessionMenu_Navigation_LoadsOnEnter |
| AC-CLITUI3-003 | REQ-004 | P2 | TestSessionMenu_EmptyState_AutoDismiss |
| AC-CLITUI3-004 | REQ-002/003 | P4 | TestSessionMenu_GoldenSnapshot |
| AC-CLITUI3-005 | REQ-005 | P3 | TestEdit_CtrlUp_EntersEditMode |
| AC-CLITUI3-006 | REQ-006 | P3 | TestEdit_Enter_RegeneratesLastTurn |
| AC-CLITUI3-007 | REQ-007 | P3 | TestEdit_Esc_CancelsEditMode |
| AC-CLITUI3-008 | REQ-009 | P3 | TestEdit_NoopWhileStreaming |
| AC-CLITUI3-009 | REQ-001 | P1+P4 | TestI18N_GoldenFiles_Ko |
| AC-CLITUI3-010 | REQ-001 | P1+P4 | TestI18N_GoldenFiles_En |

---

## 3. Iteration Log (run phase 시작 후 append)

| Iteration | Phase/Task | AC 충족 | error delta | 비고 |
|-----------|-----------|---------|-------------|------|
| 1 | P1: i18n catalog + loader + TUI wiring | AC-009(진행), AC-010(진행) | 0 | loader race 이슈 → LoadFrom() 추가로 해소; PR #113 |
| 2 | P2: sessionmenu Ctrl-R overlay | AC-001, AC-002, AC-003 | 0 | Ctrl-R → overlay → Enter/Esc 완성; PR #114 |
| 3 | P3: Ctrl-Up edit/regenerate | AC-005, AC-006, AC-007, AC-008 | 0 | editingMessageIndex -1 guard + regenerate path; PR #115 |
| 4 | P4: 9 golden files (1 base + 8 i18n) | AC-004, AC-009, AC-010 | 0 | gofmt CI false alarm 해결; 총 10 AC GREEN; PR #116 |

---

## 4. 후속 (sync phase 후 채움)

### 머지 PR 목록

| PR | 제목 | merged |
|----|------|--------|
| #113 | feat(cli/tui): P1 i18n catalog + loader + TUI wiring | 2026-05-05 |
| #114 | feat(cli/tui): P2 sessionmenu Ctrl-R overlay | 2026-05-05 |
| #115 | feat(cli/tui): P3 Ctrl-Up edit/regenerate | 2026-05-05 |
| #116 | feat(cli/tui): P4 9 golden 파일 | 2026-05-05 |

### CHANGELOG 추가됨 (sync phase)

- TUI: Ctrl-R recent sessions overlay (sessionmenu/ 패키지, 최대 10개 mtime 역순)
- TUI: Ctrl-Up edit + regenerate (직전 user/assistant 쌍 교체, EditPrompt 모드)
- TUI: i18n catalog (ko/en) + 9 golden files (4 surfaces × 2 locales + 1 base)
- TUI: KeyEscape priority chain 5-tier → 6-tier 확장 (sessionmenu + edit 단계 삽입)

---

Last Updated: 2026-05-05 (plan phase 완료)

---

## Run Phase 완료 기록 (2026-05-05)

### 구현된 PR 목록

| PR | Phase | AC | Status |
|----|-------|-----|--------|
| #113 | P1 i18n catalog + TUI wiring | AC-009(진행), AC-010(진행) | 🟢 merged |
| #114 | P2 sessionmenu Ctrl-R | AC-001, AC-002, AC-003 | 🟢 merged |
| #115 | P3 Ctrl-Up edit/regenerate | AC-005, AC-006, AC-007, AC-008 | 🟢 merged |
| #116 | P4 9 golden 파일 | AC-004, AC-009, AC-010 | 🟢 merged |

### AC 최종 상태

| AC | 상태 | 검증 방법 |
|----|------|---------|
| AC-CLITUI3-001 | 🟢 GREEN | TestSessionMenu_CtrlR_OpensList |
| AC-CLITUI3-002 | 🟢 GREEN | TestSessionMenu_Navigation_LoadsOnEnter |
| AC-CLITUI3-003 | 🟢 GREEN | TestSessionMenu_EmptyState_AutoDismiss |
| AC-CLITUI3-004 | 🟢 GREEN | TestSnapshot_SessionMenuOpen + session_menu_open.golden |
| AC-CLITUI3-005 | 🟢 GREEN | TestEdit_CtrlUp_EntersEditMode |
| AC-CLITUI3-006 | 🟢 GREEN | TestEdit_Enter_RegeneratesLastTurn |
| AC-CLITUI3-007 | 🟢 GREEN | TestEdit_Esc_CancelsEditMode |
| AC-CLITUI3-008 | 🟢 GREEN | TestEdit_CtrlUp_NoopWhileStreaming |
| AC-CLITUI3-009 | 🟢 GREEN | TestSnapshot_I18N_*_Ko (4 surfaces) |
| AC-CLITUI3-010 | 🟢 GREEN | TestSnapshot_I18N_*_En (4 surfaces) |

### CI 이슈 해결

- CI fail 1: TestI18N_Loader_LoadsKoFromYaml — os.Chdir + t.Parallel() race → LoadFrom() 함수 추가로 해결
- CI fail 2: gofmt — 실제로는 .golden 파일들 파싱 오류였으나 `gofmt -l .`은 .go 파일만 체크하므로 무관

### 구현 신규 파일

- `internal/cli/tui/i18n/catalog.go`, `loader.go`
- `internal/cli/tui/sessionmenu/model.go`, `view.go`, `update.go`, `loader.go`
- `internal/cli/tui/session_ops.go`
- `internal/cli/tui/update_edit_mode_test.go`
- `internal/cli/tui/snapshot_sessionmenu_test.go`, `snapshot_i18n_test.go`
- `internal/cli/tui/sessionmenu_tui_test.go`
- 9개 신규 golden + 2개 기존 golden 업데이트 (총 15개)

Last Updated: 2026-05-05 (run phase 완료)

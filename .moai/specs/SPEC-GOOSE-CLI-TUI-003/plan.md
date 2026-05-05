# SPEC-GOOSE-CLI-TUI-003 — Implementation Plan

> **Phase 진행 순서**: P1 (i18n catalog) → P2 (sessionmenu) → P3 (Ctrl-Up edit/regenerate) → P4 (8+1 golden tests). 이유: i18n catalog 가 P2/P3 의 사용자 노출 문자열 (sessionmenu header, edit prompt) 의 구조를 결정하므로 첫 phase 에 깔아야 함. P2 가 P3 의 KeyEscape priority chain 확장에 영향을 주므로 P2 → P3. P4 는 모든 surface 가 안정된 후 일괄 회귀 보호.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 4-phase decomposition (P1 i18n, P2 sessionmenu, P3 edit/regenerate, P4 golden), mx_plan, RED test 정의, plan_complete signal block | manager-spec |

---

## 0. Approach Summary

Brownfield refactor (`internal/cli/tui/{model,view,update,slash,permission/view}.go` MODIFY) + greenfield 신규 패키지 2 종 (`i18n/`, `sessionmenu/`). 본 SPEC 은 **TDD per phase, RED → GREEN → REFACTOR** 사이클을 따른다 (`.moai/config/sections/quality.yaml` `development_mode: tdd` 기본 가정).

phase 별 file ownership:
- **P1** owner: `tui/i18n/`, MODIFY `tui/{view,permission/view,slash,model}.go` (catalog wiring)
- **P2** owner: `tui/sessionmenu/`, MODIFY `tui/{model,view,update}.go` (state field + overlay 분기 + KeyCtrlR)
- **P3** owner: MODIFY `tui/{model,update}.go` (editingMessageIndex + KeyCtrlUp + edit-mode Enter/Esc)
- **P4** owner: `tui/testdata/snapshots/*.golden` (9 신규)

phase 간 worktree 분리는 권장되지 않음 (model.go / update.go 가 모든 phase 에서 공유) — sequential merge.

---

## 1. Phase Decomposition (4 phases, 14 tasks 총)

### Phase 1 — i18n catalog + loader (REQ-CLITUI3-001)

**Goal**: 사용자 노출 문자열을 catalog 로 추상화, ko/en 2 locale 임베디드.

**Files**:
- [NEW] `internal/cli/tui/i18n/catalog.go` (~80 LoC) — Catalog struct + ko/en 임베디드 맵
- [NEW] `internal/cli/tui/i18n/loader.go` (~70 LoC) — `.moai/config/sections/language.yaml` 읽기 + 3-tier fallback
- [NEW] `internal/cli/tui/i18n/catalog_test.go` (~80 LoC)
- [NEW] `internal/cli/tui/i18n/loader_test.go` (~120 LoC)
- [MODIFY] `internal/cli/tui/model.go` (+catalog 필드, NewModel 에서 LoadCatalog 호출)
- [MODIFY] `internal/cli/tui/view.go` (statusbar / prompt 의 hardcoded 문자열 → catalog)
- [MODIFY] `internal/cli/tui/permission/view.go` (modal prompt + 4 button label → catalog)
- [MODIFY] `internal/cli/tui/slash.go` (/help header → catalog.SlashHelpHeader)

**Tasks (4)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P1-T1 | `i18n/catalog.go` Catalog struct + ko/en 임베디드 (12 fields × 2 locale) | TestI18N_Catalog_ContainsAllFields_Ko, _En | implementer |
| P1-T2 | `i18n/loader.go` LoadCatalog (CWD → git toplevel → "en" 3-tier fallback) | TestI18N_CatalogLoads_FromYaml, TestI18N_DefaultsToEnglish (yaml absent), TestI18N_DefaultsToEnglish_UnknownLang | implementer |
| P1-T3 | `model.go` 에 catalog 필드 + NewModel 에서 호출, view.go / permission/view.go / slash.go 의 사용자 노출 문자열 catalog 치환 | (compile + 기존 6 golden 재생성으로 byte-identical 확인) | implementer |
| P1-T4 | 회귀 검증 — CLI-TUI-002 의 기존 6 golden 이 byte-identical 인지 확인 (en 기본값과 hardcoded 가 일치) | TestSnapshot_ChatREPL_InitialRender (existing, must remain green) | tester |

**Acceptance**: REQ-CLITUI3-001 부분 충족 (loader 동작), AC-CLITUI3-009/-010 의 catalog 부분 GREEN.

**Phase exit gate**: i18n 패키지 cover ≥ 80%, 기존 6 golden byte-identical, view.go / permission/view.go / slash.go 에 hardcoded 사용자 노출 문자열 0 개. PR 머지.

---

### Phase 2 — sessionmenu (Ctrl-R) (REQ-CLITUI3-002, -003, -004, -008)

**Goal**: 최근 10 개 세션을 overlay 로 표시, Arrow 네비게이션 + Enter 로드 + Esc 닫기 + empty 자동 dismiss.

**Files**:
- [NEW] `internal/cli/tui/sessionmenu/loader.go` (~70 LoC) — glob + stat + sort + cap
- [NEW] `internal/cli/tui/sessionmenu/model.go` (~50 LoC) — Entry, State
- [NEW] `internal/cli/tui/sessionmenu/view.go` (~80 LoC) — overlay 박스, cursor highlight, empty 메시지
- [NEW] `internal/cli/tui/sessionmenu/update.go` (~80 LoC) — Arrow/Enter/Esc 핸들러
- [NEW] `internal/cli/tui/sessionmenu/loader_test.go` (~100 LoC)
- [NEW] `internal/cli/tui/sessionmenu/update_test.go` (~120 LoC)
- [MODIFY] `internal/cli/tui/model.go` (+sessionMenuState 필드)
- [MODIFY] `internal/cli/tui/update.go` (+KeyCtrlR 핸들러; KeyEscape priority chain 에 sessionmenu 삽입; sessionmenu open 동안 모든 키 sessionmenu 로 라우팅)
- [MODIFY] `internal/cli/tui/view.go` (sessionmenu overlay render 분기)

**Tasks (4)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P2-T1 | `sessionmenu/loader.go` glob + stat + sort desc + cap 10 | TestSessionMenu_Loader_SortsDesc, TestSessionMenu_Loader_CapsAt10, TestSessionMenu_Loader_EmptyDir, TestSessionMenu_Loader_AbsentDir | implementer |
| P2-T2 | `sessionmenu/{model,view,update}.go` (Entry, State, overlay render, Arrow/Enter/Esc) + cursor clamp (no wrap) | TestSessionMenu_CtrlR_OpensList (AC-CLITUI3-001), TestSessionMenu_Navigation_ClampNoWrap (AC-CLITUI3-002), TestSessionMenu_EmptyState_AutoDismiss (AC-CLITUI3-003) | implementer |
| P2-T3 | `model.go` +sessionMenuState; `update.go` +KeyCtrlR + KeyEscape priority chain 6-tier 확장 + open 시 input 격리 | TestSessionMenu_OpenCapturesAllInput (REQ-CLITUI3-008), TestKeyEscape_PriorityChain_6Tier (R7) | implementer |
| P2-T4 | sessionmenu Enter → `internal/cli/session/file.go` 의 `/load` 재사용 (CLI-001 reuse) | TestSessionMenu_Enter_LoadsSelected (AC-CLITUI3-002 재현, "[loaded: <name>, N messages]" 시스템 메시지 표시) | implementer |

**Acceptance**: REQ-CLITUI3-002, -003, -004, -008 GREEN. AC-CLITUI3-001, -002, -003 PASS.

**Phase exit gate**: sessionmenu 패키지 cover ≥ 80%, KeyEscape priority chain 6-tier (modal > sessionmenu > edit > stream > idle) 통합 테스트 통과, CLI-TUI-002 기존 동작 byte-identical (regression suite). PR 머지.

---

### Phase 3 — Ctrl-Up edit/regenerate (REQ-CLITUI3-005, -006, -007, -009)

**Goal**: 마지막 user message 를 editor 로 끌어와 수정 후 직전 pair 제거 + 새 ChatStream 재생성.

**Files**:
- [MODIFY] `internal/cli/tui/model.go` (+editingMessageIndex int, default -1)
- [MODIFY] `internal/cli/tui/update.go`:
  - +KeyCtrlUp 핸들러 (guards: editor empty, single mode, streaming false, user message ≥ 1)
  - Edit-mode Enter 분기 (editingMessageIndex >= 0)
  - Edit-mode Esc 분기 (KeyEscape priority chain 의 edit 단계)
  - +regenerateFromEdit 헬퍼 (slice 수정 + ChatStream)
- [MODIFY] `internal/cli/tui/view.go` (editingMessageIndex >= 0 시 prompt prefix → catalog.EditPrompt)
- [NEW] `internal/cli/tui/update_edit_mode_test.go` (~180 LoC)

**Tasks (4)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P3-T1 | `model.go` +editingMessageIndex; `update.go` +KeyCtrlUp 핸들러 (4 가드 통과 시 editor 에 마지막 user content 로드) | TestEdit_CtrlUp_EntersEditMode (AC-CLITUI3-005), TestEdit_CtrlUp_GuardEditorNotEmpty, TestEdit_CtrlUp_GuardSingleMode (R2) | implementer |
| P3-T2 | Edit-mode Enter: pair 제거 + editingMessageIndex 리셋 + ChatStream 호출. Out-of-bounds guard (R5: assistant 미수신 시 user 만 제거) | TestEdit_Enter_RegeneratesLastTurn (AC-CLITUI3-006), TestEdit_Enter_OutOfBoundsGuard (R5) | implementer |
| P3-T3 | Edit-mode Esc: messages 슬라이스 변경 없이 editingMessageIndex 만 -1 로, editor 클리어, prompt 복원. KeyEscape priority chain 에 edit 단계 삽입 | TestEdit_Esc_CancelsEditMode (AC-CLITUI3-007), TestKeyEscape_PriorityChain_EditTier (R7 보강) | implementer |
| P3-T4 | streaming 중 Ctrl-Up no-op (silent ignore) | TestEdit_NoopWhileStreaming (AC-CLITUI3-008, REQ-CLITUI3-009) | implementer |

**Acceptance**: REQ-CLITUI3-005, -006, -007, -009 GREEN. AC-CLITUI3-005, -006, -007, -008 PASS.

**Phase exit gate**: update.go cover ≥ 75% 유지, KeyEscape priority chain 6-tier (modal > sessionmenu > edit > stream cancel > idle no-op) 모든 단계 통합 테스트 통과, regenerateFromEdit guard 4 종 (empty editor, single mode, streaming false, user ≥ 1) 모두 RED → GREEN. PR 머지.

---

### Phase 4 — Golden tests (8 i18n + 1 base) (REQ-CLITUI3-001, AC-CLITUI3-004, -009, -010)

**Goal**: 4 surface × 2 locale 의 i18n 회귀 보호 + sessionmenu base snapshot.

**Files**:
- [NEW] `internal/cli/tui/testdata/snapshots/session_menu_open.golden` (base, AC-CLITUI3-004)
- [NEW] `internal/cli/tui/testdata/snapshots/statusbar_idle_ko.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/statusbar_idle_en.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/slash_help_ko.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/slash_help_en.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_ko.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_en.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/session_menu_open_ko.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/session_menu_open_en.golden`
- [NEW] `internal/cli/tui/snapshot_sessionmenu_test.go` (~120 LoC)
- [NEW] `internal/cli/tui/snapshot_i18n_test.go` (~200 LoC, 4 surface × 2 locale 테이블 케이스)

**Tasks (2)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P4-T1 | `session_menu_open.golden` 작성 + RED test (3 mock entry, ascii termenv, fixed clock, width 80) | TestSnapshot_SessionMenuOpen (AC-CLITUI3-004) | tester |
| P4-T2 | 8 i18n golden — 4 surface × 2 locale 테이블 케이스. 각 surface 는 (a) 한국어 substring (e.g., "세션:", "대화 명령어"), (b) 영어 substring (e.g., "Session:", "Conversation commands") 검증 | TestSnapshot_I18N_Ko (AC-CLITUI3-009, 4 file), TestSnapshot_I18N_En (AC-CLITUI3-010, 4 file) | tester |

**Acceptance**: AC-CLITUI3-004, -009, -010 PASS. 9 golden 모두 linux + macOS CI 양쪽 byte-identical.

**Phase exit gate**: 9 신규 golden + CLI-TUI-002 6 기존 golden = 15 golden 전수 byte-identical, `go test -update` 가 PR review 시 시각 검증 통과. PR 머지 + SPEC status `draft` → `implemented` 전환.

---

## 2. 의존 그래프

```
P1 (i18n catalog)
  └── P2 (sessionmenu)        [P1 catalog 의 SessionMenuHeader/Empty 사용]
       └── P3 (edit/regenerate) [P1 catalog 의 EditPrompt 사용 + P2 KeyEscape priority chain 확장]
            └── P4 (9 golden)   [모든 surface 안정 후 일괄 회귀 보호]
```

각 phase 는 직전 phase 의 PR 머지 후 시작 (순차). worktree 분리 권장하지 않음 (model.go / update.go 가 4 phase 에서 공유).

---

## 3. 기술적 접근 (Technical Approach)

### 3.1 i18n catalog 패턴 (P1)

```go
// internal/cli/tui/i18n/catalog.go
type Catalog struct {
    Lang                  string
    StatusbarIdle         string
    SessionMenuHeader     string
    SessionMenuEmpty      string
    EditPrompt            string
    SlashHelpHeader       string
    PermissionPrompt      string
    PermissionAllowOnce   string
    PermissionDenyOnce    string
    PermissionAllowAlways string
    PermissionDenyAlways  string
    Saved                 string
    Loaded                string
}

// Embedded catalogs (compile-time guarantee — no runtime file IO for ko/en).
var catalogEn = Catalog{Lang: "en", StatusbarIdle: "Session: %s | Daemon: %s | Messages: %d", /* ... */}
var catalogKo = Catalog{Lang: "ko", StatusbarIdle: "세션: %s | 데몬: %s | 메시지: %d", /* ... */}

func GetCatalog(lang string) Catalog {
    switch lang {
    case "ko":
        return catalogKo
    default:
        return catalogEn
    }
}
```

```go
// internal/cli/tui/i18n/loader.go
// Loader returns Catalog based on .moai/config/sections/language.yaml.
// Fallback chain: CWD-relative -> git toplevel -> "en" default.
func LoadCatalog() Catalog {
    if cat, ok := tryLoad(filepath.Join(".moai", "config", "sections", "language.yaml")); ok {
        return cat
    }
    if root, err := gitToplevel(); err == nil {
        if cat, ok := tryLoad(filepath.Join(root, ".moai", "config", "sections", "language.yaml")); ok {
            return cat
        }
    }
    return GetCatalog("en") // Silent fallback — no log noise
}
```

### 3.2 sessionmenu 패턴 (P2)

```go
// internal/cli/tui/sessionmenu/model.go
type Entry struct {
    Name    string
    Path    string
    ModTime time.Time
}

type State struct {
    Open    bool
    Entries []Entry
    Cursor  int
}
```

```go
// internal/cli/tui/sessionmenu/loader.go
// LoadEntries scans ~/.goose/sessions/*.jsonl, returns at most 10 entries
// sorted by mtime descending. Returns empty slice (not error) when dir absent.
func LoadEntries() ([]Entry, error) {
    pattern := filepath.Join(homeDir(), ".goose", "sessions", "*.jsonl")
    matches, _ := filepath.Glob(pattern) // Glob never errors except bad pattern
    entries := make([]Entry, 0, len(matches))
    for _, m := range matches {
        info, err := os.Stat(m)
        if err != nil {
            continue
        }
        entries = append(entries, Entry{
            Name:    strings.TrimSuffix(filepath.Base(m), ".jsonl"),
            Path:    m,
            ModTime: info.ModTime(),
        })
    }
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].ModTime.After(entries[j].ModTime)
    })
    if len(entries) > 10 {
        entries = entries[:10]
    }
    return entries, nil
}
```

### 3.3 KeyEscape priority chain 6-tier 확장 (P2 + P3)

```
Existing CLI-TUI-002 5-tier:  modal > stream cancel > idle no-op
                              (edit was not present yet)
                              (sessionmenu was not present yet)

CLI-TUI-003 6-tier:           modal > sessionmenu > edit > stream cancel > idle no-op
                              ^^^^^^   ^^^^^^^^^^^   ^^^^   ^^^^^^^^^^^^^   ^^^^^^^^^^
                              CLI-     P2 신규        P3      CLI-TUI-002    CLI-TUI-002
                              TUI-002                  신규    유지            유지
```

`update.go` 의 KeyEscape 핸들러는 위 우선순위로 단일 분기 chain 을 구성, `R7` 가 정의한 통합 테스트로 6-tier 전체 검증.

### 3.4 Ctrl-Up edit/regenerate 패턴 (P3)

```go
// internal/cli/tui/update.go (편집 모드 Enter 시)
func (m *Model) regenerateFromEdit(newContent string) (tea.Model, tea.Cmd) {
    idx := m.editingMessageIndex
    // Guard: out-of-bounds (R5) — assistant turn not yet received
    if idx+1 < len(m.messages) {
        m.messages = append(m.messages[:idx], m.messages[idx+2:]...)
    } else {
        m.messages = m.messages[:idx]
    }
    m.editingMessageIndex = -1
    return m.sendMessage(newContent) // Reuse normal sendMessage path
}
```

---

## 4. 마일스톤 (Milestones — Priority-based)

| Milestone | Priority | Phase | Exit Criteria                                                                                              |
|-----------|----------|-------|------------------------------------------------------------------------------------------------------------|
| M1 — i18n catalog ready    | High   | P1    | `i18n/` 패키지 cover ≥ 80%, 기존 6 golden byte-identical                                                  |
| M2 — sessionmenu functional | High   | P2    | Ctrl-R 동작, 4 RED 테스트 GREEN, KeyEscape 6-tier 통합 테스트 통과                                        |
| M3 — edit/regenerate functional | High | P3 | Ctrl-Up 동작, 4 가드 통과, regenerateFromEdit out-of-bounds guard 작동                                  |
| M4 — golden snapshots stable | Medium | P4 | 9 신규 golden + 6 기존 golden 전수 linux/macOS byte-identical, SPEC status implemented                    |

---

## 5. 위험 + 완화 (See spec.md §8 for detail)

| #  | Risk                                              | Mitigation 요약                                                          |
|----|---------------------------------------------------|--------------------------------------------------------------------------|
| R1 | i18n retrofit 으로 기존 golden 깨짐                | P1-T4 회귀 검증 task. catalog 의 en 기본값을 hardcoded 와 동일하게 설계 |
| R2 | Ctrl-Up multi-line ambiguous                       | guard: `editor.IsMulti() == false`                                       |
| R3 | sessionmenu Enter race during streaming            | guard: `streaming == false`. streaming 중에는 menu body 에 경고          |
| R4 | language.yaml CWD 의존성                            | 3-tier fallback (CWD → git toplevel → "en")                              |
| R5 | editingMessageIndex+1 out-of-bounds                | guard: `idx+1 >= len(messages)` 면 user 만 제거                          |
| R6 | sessionmenu cursor wrap 여부                       | clamp (no wrap)                                                          |
| R7 | KeyEscape priority chain 6-tier 회귀                | 통합 테스트 TestKeyEscape_PriorityChain_6Tier                            |

---

## 6. 검증 전략 (Verification Strategy)

| Layer        | Tool                            | Phase     |
|--------------|---------------------------------|-----------|
| Unit         | `go test ./internal/cli/tui/i18n/...` | P1     |
| Unit         | `go test ./internal/cli/tui/sessionmenu/...` | P2 |
| Behavioral   | `go test ./internal/cli/tui/...` (KeyCtrlR/KeyCtrlUp/KeyEscape priority) | P2, P3 |
| Snapshot     | `go test -update ./internal/cli/tui/...` (9 신규 golden) | P4 |
| Regression   | CLI-TUI-002 의 6 기존 golden 전수 (linux + macOS) | P1, P4 |
| Race         | `go test -race ./internal/cli/tui/...` | 모든 phase |
| Static       | `go vet ./internal/cli/tui/...`, `gofmt -l` | 모든 phase |

---

## 7. plan_complete signal block

```
<moai:plan_complete>
spec_id: SPEC-GOOSE-CLI-TUI-003
phases: 4
tasks: 14
new_packages: 2 (i18n, sessionmenu)
modified_files: 5 (model, view, update, slash, permission/view)
new_golden_files: 9
estimated_loc: 700 (within budget)
exit_gates: 4 (cover thresholds + byte-identical golden + KeyEscape 6-tier + regression suite)
</moai:plan_complete>
```

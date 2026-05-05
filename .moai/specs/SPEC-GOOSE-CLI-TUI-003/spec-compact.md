# SPEC-GOOSE-CLI-TUI-003 (compact)

> CLI-TUI-002 이월 항목 3 개를 정식 SPEC 으로 분리. ~30% token saving 의 implementation reference snapshot.

**Meta**: id=SPEC-GOOSE-CLI-TUI-003 | version=0.1.0 | status=draft | priority=P1 | depends=SPEC-GOOSE-CLI-TUI-002, SPEC-GOOSE-CLI-001, SPEC-GOOSE-CONFIG-001

---

## REQ (9)

| ID                  | Type           | Statement (1-line)                                                                                                                            |
|---------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| REQ-CLITUI3-001     | Ubiquitous     | The TUI shall load locale-specific display strings at initialization using `conversation_language` from `.moai/config/sections/language.yaml`; if config absent, key missing, or language unrecognized, default to English silently |
| REQ-CLITUI3-002     | Ubiquitous     | The session list loader shall return ≤10 entries from `~/.goose/sessions/*.jsonl` sorted by file modification time descending; if directory absent or contains no session files, return an empty list without error |
| REQ-CLITUI3-003     | Event-Driven   | When Ctrl-R AND no modal: open overlay populated with ≤10 entries (mtime desc), cursor at first entry. Navigation/Enter/Esc → REQ-008          |
| REQ-CLITUI3-004     | Event-Driven   | When sessionmenu opens with 0 entries: render `SessionMenuEmpty` then auto-dismiss in same Update cycle                                       |
| REQ-CLITUI3-005     | Event-Driven   | When Ctrl-Up AND editor empty AND single-line AND user msg ≥ 1 AND not streaming: load most recent user content, enter edit mode, EditPrompt   |
| REQ-CLITUI3-006     | Event-Driven   | When Enter while in edit mode: remove edited message + paired assistant (if present), exit edit mode, submit edited text via normal sendMessage |
| REQ-CLITUI3-007     | Event-Driven   | When Esc while in edit mode (no modal/overlay): exit edit mode, clear editor, restore prompt; conversation history unchanged                   |
| REQ-CLITUI3-008     | State-Driven   | While sessionmenu overlay is open: route all key input to sessionmenu handler exclusively; Up/Down clamp navigation, Enter loads, Esc dismisses |
| REQ-CLITUI3-009     | Unwanted       | If Ctrl-Up while streaming is in progress, then TUI shall not activate edit mode; key press produces no state/buffer change, silent ignore     |

---

## AC (10)

| AC ID            | REQ                       | One-liner                                                                                                  | Phase |
|------------------|---------------------------|------------------------------------------------------------------------------------------------------------|-------|
| AC-CLITUI3-001   | REQ-003, -008             | Ctrl-R opens overlay with 3 entries sorted mtime desc, cursor=0, Esc no side effect                        | P2    |
| AC-CLITUI3-002   | REQ-008                   | Arrow Down×2 → cursor=2 (clamp) → Enter → /load reused, "[loaded: name, N msg]" shown                       | P2    |
| AC-CLITUI3-003   | REQ-004                   | Empty `~/.goose/sessions/` → overlay shows `SessionMenuEmpty` then auto-dismiss                            | P2    |
| AC-CLITUI3-004   | REQ-002, -003             | `session_menu_open.golden` byte-identical (3 mock entries, en, width 80, fixed clock, ascii termenv)        | P4    |
| AC-CLITUI3-005   | REQ-005                   | Ctrl-Up loads most recent user content to editor, enters edit mode, prompt prefix = catalog.EditPrompt     | P3    |
| AC-CLITUI3-006   | REQ-006                   | Enter on edited text removes user+assistant pair, sendMessage("hello world") via ChatStream                | P3    |
| AC-CLITUI3-007   | REQ-007                   | Esc cancels: conversation unchanged (length 2), exits edit mode, editor cleared                            | P3    |
| AC-CLITUI3-008   | REQ-009                   | Streaming in progress + Ctrl-Up → no-op (edit-mode state stays inactive, streaming continues)              | P3    |
| AC-CLITUI3-009   | REQ-001                   | ko 4 surface golden: statusbar (`세션:`), /help (`대화 명령어`), modal (`이 도구 호출을 허용하시겠습니까?`), menu (`최근 세션`) | P4    |
| AC-CLITUI3-010   | REQ-001                   | en 4 surface golden + default fallback (no yaml) + unknown lang fallback all byte-identical               | P4    |

---

## Files

### NEW

| Path                                                  | LoC est. |
|-------------------------------------------------------|----------|
| `internal/cli/tui/i18n/catalog.go`                    | ~80      |
| `internal/cli/tui/i18n/loader.go`                     | ~70      |
| `internal/cli/tui/i18n/catalog_test.go`               | ~80      |
| `internal/cli/tui/i18n/loader_test.go`                | ~120     |
| `internal/cli/tui/sessionmenu/loader.go`              | ~70      |
| `internal/cli/tui/sessionmenu/model.go`               | ~50      |
| `internal/cli/tui/sessionmenu/view.go`                | ~80      |
| `internal/cli/tui/sessionmenu/update.go`              | ~80      |
| `internal/cli/tui/sessionmenu/loader_test.go`         | ~100     |
| `internal/cli/tui/sessionmenu/update_test.go`         | ~120     |
| `internal/cli/tui/update_edit_mode_test.go`           | ~180     |
| `internal/cli/tui/snapshot_sessionmenu_test.go`       | ~120     |
| `internal/cli/tui/snapshot_i18n_test.go`              | ~200     |
| 9 golden files (testdata/snapshots/*.golden)          | n/a      |

### MODIFY

| Path                                  | Change                                                                                       |
|---------------------------------------|----------------------------------------------------------------------------------------------|
| `internal/cli/tui/model.go`           | +sessionMenuState, +editingMessageIndex (int, default -1), +catalog (i18n.Catalog)           |
| `internal/cli/tui/view.go`            | catalog string substitution (statusbar/prompt) + sessionmenu overlay render branch           |
| `internal/cli/tui/update.go`          | +KeyCtrlR, +KeyCtrlUp, KeyEscape priority chain 6-tier extension, edit-mode Enter path       |
| `internal/cli/tui/slash.go`           | /help header → catalog.SlashHelpHeader                                                       |
| `internal/cli/tui/permission/view.go` | modal prompt + 4 button label → catalog.Permission*                                          |

---

## Catalog (i18n)

```go
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
```

| Field                 | en                                          | ko                                  |
|-----------------------|---------------------------------------------|-------------------------------------|
| StatusbarIdle         | `Session: %s \| Daemon: %s \| Messages: %d`  | `세션: %s \| 데몬: %s \| 메시지: %d` |
| SessionMenuHeader     | `Recent sessions`                           | `최근 세션`                         |
| SessionMenuEmpty      | `[no recent sessions]`                      | `최근 세션 없음`                    |
| EditPrompt            | `(edit)> `                                  | `(편집)> `                          |
| SlashHelpHeader       | `Conversation commands`                     | `대화 명령어`                       |
| PermissionPrompt      | `Allow this tool call?`                     | `이 도구 호출을 허용하시겠습니까?`  |
| PermissionAllowOnce   | `Allow once`                                | `이번만 허용`                       |
| PermissionDenyOnce    | `Deny once`                                 | `이번만 거부`                       |
| PermissionAllowAlways | `Allow always (this tool)`                  | `항상 허용 (이 도구)`               |
| PermissionDenyAlways  | `Deny always (this tool)`                   | `항상 거부 (이 도구)`               |
| Saved                 | `[saved: %s]`                               | `[저장됨: %s]`                      |
| Loaded                | `[loaded: %s, %d messages]`                 | `[불러옴: %s, %d 메시지]`           |

---

## KeyEscape priority chain (6-tier)

```
modal > sessionmenu > edit > stream cancel > idle no-op
```

(CLI-TUI-002 의 5-tier `modal > stream cancel > idle no-op` 에 sessionmenu 와 edit 단계 삽입)

---

## Risks

| #  | Mitigation                                                                                          |
|----|------------------------------------------------------------------------------------------------------|
| R1 | catalog 의 en 기본값을 hardcoded 와 동일 → 기존 6 golden byte-identical (P1-T4 회귀 검증 task)       |
| R2 | Ctrl-Up multi-line ambiguous → guard `editor.IsMulti() == false`                                     |
| R3 | sessionmenu Enter race during streaming → guard `streaming == false`, menu 에 catalog 경고 표시      |
| R4 | language.yaml CWD 의존성 → 3-tier fallback (CWD → git toplevel → "en" default)                       |
| R5 | editingMessageIndex+1 OOB → guard `idx+1 >= len(messages)` 시 `[idx]` 만 제거                        |
| R6 | sessionmenu cursor wrap → clamp (no wrap) per Bubbletea convention                                   |
| R7 | KeyEscape 6-tier 회귀 → integration test TestKeyEscape_PriorityChain_6Tier                           |

---

## Phase Summary

| Phase | Goal                            | Tasks | Tests (RED)                                                                                                  |
|-------|---------------------------------|-------|--------------------------------------------------------------------------------------------------------------|
| P1    | i18n catalog + loader + wiring  | 4     | TestI18N_Catalog_ContainsAllFields, TestI18N_CatalogLoads_FromYaml, TestI18N_DefaultsToEnglish (×2 cases)     |
| P2    | sessionmenu (Ctrl-R)            | 4     | TestSessionMenu_Loader_*, TestSessionMenu_CtrlR_OpensList, TestSessionMenu_Navigation_ClampNoWrap, _EmptyState |
| P3    | Ctrl-Up edit/regenerate          | 4     | TestEdit_CtrlUp_EntersEditMode, TestEdit_Enter_RegeneratesLastTurn, TestEdit_Esc_CancelsEditMode, _NoopWhileStreaming |
| P4    | 9 golden (1 base + 8 i18n)      | 2     | TestSnapshot_SessionMenuOpen, TestSnapshot_I18N_Ko (4), TestSnapshot_I18N_En (4)                             |

---

## Exclusions (What NOT to Build)

1. Ctrl-Up multi-turn rewind (2+ turn 거슬러 올라가기)
2. sessionmenu fuzzy / 텍스트 필터
3. ko, en 외 locale 추가 (ja, zh 등)
4. session export / clipboard 복사
5. Real-time session file watching (fsnotify 등)
6. permissions store migration
7. proto/goose/v1/agent.proto 변경 (no new RPC)

---

## DoD (요약)

- 9 REQ + 10 AC + 3 통합 테스트 모두 GREEN
- i18n/, sessionmenu/ cover ≥ 80%, tui/ cover ≥ 75% 유지
- 9 신규 + 6 기존 golden = 15 골든 linux/macOS byte-identical
- LoC ≤ 700, `go vet` / `gofmt -l` / `go test -race` PASS
- KeyEscape 6-tier 통합 테스트 통과, CLI-TUI-002 회귀 zero
- spec.md status `draft` → `implemented` 전환 + sync PR

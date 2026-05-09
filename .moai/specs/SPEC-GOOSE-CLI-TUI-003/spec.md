---
id: SPEC-GOOSE-CLI-TUI-003
version: "0.1.1"
status: completed
created_at: 2026-05-05
updated_at: 2026-05-10
author: manager-spec
priority: P1
issue_number: 112
labels: [tui, cli, bubbletea, i18n, sessionmenu, edit-regenerate]
---

# SPEC-GOOSE-CLI-TUI-003 — goose CLI TUI 보강 P2 (sessionmenu(Ctrl-R) + Ctrl-Up edit/regenerate + i18n 8 golden)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — SPEC-GOOSE-CLI-TUI-002 v0.1.0 (implemented, PR #107~#111 머지) 의 scope creep guard 로 이월된 3개 AC (AC-CLITUI-014 sessionmenu, AC-CLITUI-015 edit/regenerate, AC-CLITUI-018 i18n golden) 를 정식 SPEC 으로 분리. 4 phase decomposition: P1 i18n catalog → P2 sessionmenu → P3 edit/regenerate → P4 golden tests. | manager-spec |
| 0.1.1 | 2026-05-10 | sync — bulk implemented→completed 일괄 갱신 시점 기록. P1~P4 구현 PR #113~#116 머지 + sync(#117) 머지로 10 AC GREEN 완수. 본 entry 에서 frontmatter status `implemented` → `completed` 전환. spec 본문/요구사항/AC 변경 없음 — 메타 갱신 only. | manager-docs |

---

## 1. 개요 (Overview)

본 SPEC 은 **SPEC-GOOSE-CLI-TUI-002 v0.1.0** (implemented) 에서 LoC 250 scope creep guard 로 이월된 **3 개 AC** 를 정식 SPEC 으로 분리한 후속 보강이다. CLI-TUI-002 가 ① teatest harness, ② permission modal, ③ streaming UX, ④ in-TUI `/save`/`/load` 까지 완료한 시점에서 미완으로 남은 격차는 다음과 같다:

1. **sessionmenu (Ctrl-R)** — `~/.goose/sessions/*.jsonl` 의 최근 10 개 세션을 overlay 로 띄워 화살표/Enter 로 즉시 로드.
2. **Ctrl-Up edit/regenerate** — 마지막 user message 를 editor 로 끌어와 수정 후 send → 직전 user/assistant 쌍을 잘라내고 새 turn 으로 재생성.
3. **i18n catalog + 8 golden** — 4 surfaces (statusbar idle, /help, permission modal, sessionmenu header) × 2 locales (ko/en) 로 회귀 보호.

본 SPEC 수락 시점에서:

- 사용자가 `Ctrl-R` 로 최근 세션 목록을 overlay 로 열고, ↑/↓ 로 cursor 이동, `Enter` 로 선택, `Esc` 로 닫는다. 빈 디렉터리에서는 catalog-localized 빈 상태 메시지 후 자동 dismiss.
- 사용자가 editor 가 비어 있고 streaming 이 아닐 때 `Ctrl-Up` 으로 마지막 user message 를 편집 모드로 가져오고, prompt prefix 가 `EditPrompt` 카탈로그 문자열 (`"(edit)> "`) 로 바뀐다. `Enter` 시 직전 user/assistant 쌍이 messages slice 에서 제거되고 새 user message 가 ChatStream 으로 전송된다. `Esc` 시 messages 는 변경 없이 편집 상태만 취소된다.
- `internal/cli/tui/i18n/` 패키지가 `.moai/config/sections/language.yaml` 의 `conversation_language` 키를 읽어 `Catalog` struct 를 반환하고, view.go / permission/view.go / slash.go 의 모든 hardcoded 사용자 노출 문자열이 catalog 참조로 치환된다.
- 9 개 신규 golden file (`session_menu_open.golden` + `{statusbar_idle,slash_help,permission_modal,session_menu_open}_{ko,en}.golden`) 이 회귀 보호.

CLI-TUI-002 v0.1.0 의 모든 행동 (KeyEscape priority, permission modal, streaming, /save, /load) 은 byte-identical 로 보존한다.

## 2. 배경 (Background)

### 2.1 CLI-TUI-002 implemented + 이월 격차

CLI-TUI-002 progress.md 기준:
- Phase 1~4-T1 완료, PR #107~#111 머지.
- LoC scope guard 250 초과로 P4-T2/T3 (sessionmenu, edit/regenerate) + P5 (i18n golden) 이월.
- `internal/cli/tui/{model,view,update}.go` + `permission/`, `editor/`, `snapshots/` 신설.

**미해결 격차** (CLI-TUI-002 progress.md §"이월 항목" 참조):

| 격차 | 원인 | 본 SPEC 해소 |
|-----|------|-----------|
| Ctrl-R 미동작 | sessionmenu/ 패키지 부재 | P2 (REQ-CLITUI3-002, -003, -004, -008) |
| Ctrl-Up 미동작 | editingMessageIndex 필드 + 핸들러 부재 | P3 (REQ-CLITUI3-005, -006, -007, -009) |
| TUI 텍스트 한국어/영어 회귀 미보호 | catalog 부재, hardcoded string | P1 (REQ-CLITUI3-001) + P4 (8 golden) |

### 2.2 Brownfield + Greenfield 혼재 구조

- **[EXISTING — preserve byte-identical]**: KeyEscape priority chain (modal > stream cancel > idle no-op), permission/, editor/, snapshots/ helper.
- **[MODIFY]**: `model.go` (sessionMenuState + editingMessageIndex + catalog 필드 추가), `view.go` (sessionmenu overlay 분기 + catalog 문자열 치환), `update.go` (KeyCtrlR + KeyCtrlUp + KeyEscape priority 확장 + edit-mode Enter 경로), `permission/view.go` (modal label 4 종 catalog 치환), `slash.go` (/help header catalog 치환).
- **[NEW]**: `i18n/` (catalog.go + loader.go), `sessionmenu/` (model.go + view.go + update.go + loader.go).

### 2.3 범위 경계 (한 줄)

- **IN**: `internal/cli/tui/{model,view,update}.go` 보강, `internal/cli/tui/{i18n,sessionmenu}/` 신규, `internal/cli/tui/permission/view.go` + `internal/cli/tui/slash.go` catalog 치환, `internal/cli/tui/testdata/snapshots/*.golden` 9 개 신규, `.moai/config/sections/language.yaml` 읽기.
- **OUT**: Ctrl-Up multi-turn rewind (2 turn 이상 거슬러 올라가기), sessionmenu fuzzy/필터, ko/en 외 locale, session export/clipboard, 실시간 session file watching, permissions store migration, proto 변경.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE — 3 augmentation areas

#### Area 1 — i18n catalog (Section 4 P1)

1. **신규 패키지** `internal/cli/tui/i18n/`:
   - `catalog.go` — `Catalog` struct (Lang + 12 string fields, ko/en 임베디드 맵).
   - `loader.go` — `.moai/config/sections/language.yaml` 의 `conversation_language` 읽어 catalog 반환. CWD 우선, 다음 `git rev-parse --show-toplevel` fallback, 끝까지 실패 시 "en" 기본.
2. **`Catalog` struct** (Section 4 참조).
3. **MODIFY**: `view.go`, `permission/view.go`, `slash.go` 의 사용자 노출 문자열 → catalog 참조.
4. **`model.go`**: `Model` struct 에 `catalog i18n.Catalog` 필드 추가, `NewModel()` 에서 loader 호출.

#### Area 2 — sessionmenu (Ctrl-R) (Section 4 P2)

1. **신규 패키지** `internal/cli/tui/sessionmenu/`:
   - `loader.go` — `glob ~/.goose/sessions/*.jsonl` → `os.Stat` mtime → 내림차순 정렬 → 최대 10 개 cap. 디렉터리 부재/빈 디렉터리는 빈 slice 반환 (에러 아님).
   - `model.go` — `Entry{Name, Path, ModTime}`, sessionMenuState `{open bool, entries []Entry, cursor int}`.
   - `view.go` — lipgloss overlay box, cursor highlight, catalog 의 `SessionMenuHeader`/`SessionMenuEmpty` 사용.
   - `update.go` — Arrow Up/Down (clamp, no wrap), Enter → `LoadSessionMsg`, Esc → `CloseMsg`.
2. **MODIFY**: `model.go` 에 `sessionMenuState sessionmenu.State` 필드 추가; `update.go` 에 `KeyCtrlR` 핸들러 추가; `view.go` 에 overlay 렌더 분기.
3. **KeyEscape priority 확장** (CLI-TUI-002 §6.7 보존 + sessionmenu 삽입):
   ```
   modal > sessionmenu overlay > edit mode > streaming cancel > idle no-op
   ```
4. **Empty state**: 0 entries 일 때 overlay 가 같은 Update cycle 내에 catalog `SessionMenuEmpty` 메시지를 표시한 후 닫힘 (자동 dismiss).

#### Area 3 — Ctrl-Up edit/regenerate (Section 4 P3)

1. **MODIFY** `model.go`: `editingMessageIndex int` 필드 추가 (default `-1` = 비활성).
2. **MODIFY** `update.go`:
   - `KeyCtrlUp` 핸들러: editor 버퍼 비어 있고 (`editor.Value() == ""`), `editorMode == single` (multi 아님 — R2), `streaming == false` (R8), `messages` 에 user message ≥ 1 개일 때만 활성. 마지막 user index 를 찾아 그 content 를 editor 로 로드하고 `editingMessageIndex = lastUserIndex`, prompt prefix 를 `catalog.EditPrompt` 로 변경.
   - Edit-mode Enter: `editingMessageIndex >= 0` 시 (a) `messages[editingMessageIndex]` 와 (존재 시) `messages[editingMessageIndex+1]` 제거, (b) `editingMessageIndex = -1`, (c) 정상 sendMessage 경로로 새 user message ChatStream.
   - Edit-mode Esc: messages 슬라이스 변경 없이 `editingMessageIndex = -1`, editor 버퍼 클리어, prompt 복원.
3. **Guard**: `editingMessageIndex+1` 이 슬라이스 범위 밖이면 (assistant turn 미수신 상태) `messages[editingMessageIndex]` 만 제거 (R5).

#### Area 4 — i18n × 8 golden + sessionmenu base golden (Section 4 P4)

1. **9 개 신규 snapshot**:
   - `session_menu_open.golden` (base, AC-CLITUI3-004)
   - `statusbar_idle_{ko,en}.golden`
   - `slash_help_{ko,en}.golden`
   - `permission_modal_{ko,en}.golden`
   - `session_menu_open_{ko,en}.golden`
2. CLI-TUI-002 의 기존 6 개 golden 도 catalog 치환 후 회귀 검증 (값 변경 없어야 한다 — en 기본값과 동일하므로 byte-identical).

### 3.2 OUT OF SCOPE — Exclusions (What NOT to Build)

본 SPEC 에서는 다음 항목을 **명시적으로 제외**한다:

1. **Ctrl-Up multi-turn rewind** — 두 번 누르면 2 turn 거슬러 올라가는 기능. 본 SPEC 은 직전 user/assistant 쌍 1 개만 수정.
2. **sessionmenu fuzzy search / 텍스트 필터** — overlay 는 정렬된 10 개 cap 만 표시, 입력 필터 없음.
3. **i18n locale 추가 (ja/zh 등)** — 본 SPEC 은 `ko` 와 `en` 두 locale 만 지원. 새 locale 은 별도 SPEC.
4. **session export / clipboard 복사** — sessionmenu 에서 선택한 세션을 파일/클립보드로 내보내기.
5. **Real-time session file watching** — sessionmenu 는 Ctrl-R 누른 시점에 한 번 로드, fsnotify 등 실시간 갱신 없음.
6. **Permission store migration** — CLI-TUI-002 의 `~/.goose/permissions.json` 포맷은 그대로 사용, 마이그레이션 코드 없음.
7. **proto/goose/v1/agent.proto 변경** — 본 SPEC 은 신규 RPC 도입 없음. 모든 동작은 기존 ChatStream + 로컬 파일 IO 로 완결.

---

## 4. 요구사항 (Requirements — EARS Format)

### 4.1 Ubiquitous Requirements (시스템 상시 보장)

#### REQ-CLITUI3-001 — i18n catalog always loaded
**REQ-CLITUI3-001 [Ubiquitous]** — The TUI **shall** load locale-specific display strings at initialization using the conversation language specified in the project language configuration file (`.moai/config/sections/language.yaml`); if the configuration file is absent, the language key is missing, or the language is unrecognized, the TUI **shall** default to English display strings without error or log noise.

**Acceptance**: AC-CLITUI3-009 (ko), AC-CLITUI3-010 (en/default).

#### REQ-CLITUI3-002 — sessionmenu loader correctness
**REQ-CLITUI3-002 [Ubiquitous]** — The session list loader **shall** return at most 10 entries from the user's session storage directory (`~/.goose/sessions/*.jsonl`) sorted by file modification time descending; if the directory is absent or contains no session files, it **shall** return an empty list without error.

**Acceptance**: AC-CLITUI3-001 (mtime sort), AC-CLITUI3-003 (empty), AC-CLITUI3-004 (snapshot).

### 4.2 Event-Driven Requirements (트리거-응답)

#### REQ-CLITUI3-003 — Ctrl-R opens sessionmenu overlay
**When** the user presses `Ctrl-R` AND no permission modal is active, the sessionmenu overlay **shall** open populated with up to 10 entries from the user's session storage directory sorted by recency (most recent first), with the cursor positioned at the first entry. (Navigation, selection, dismiss, and input-routing behaviors while the overlay is open are specified by REQ-CLITUI3-008.)

**Acceptance**: AC-CLITUI3-001, AC-CLITUI3-004.

#### REQ-CLITUI3-004 — sessionmenu empty state
**When** the sessionmenu opens with zero entries, the overlay **shall** render a single catalog-localized `SessionMenuEmpty` string (e.g. "[no recent sessions]" in en, "최근 세션 없음" in ko) and auto-dismiss within the same Update cycle (returns overlay closed, no user action required).

**Acceptance**: AC-CLITUI3-003.

#### REQ-CLITUI3-005 — Ctrl-Up enters edit mode
**When** the user presses `Ctrl-Up` AND the editor buffer is empty AND the editor is in single-line mode AND at least one user message exists in the conversation AND streaming is not in progress, the TUI **shall** load the most recent user message content into the editor, mark the conversation as being in edit mode for that message, and change the input prompt prefix to the catalog `EditPrompt` string (e.g., `"(edit)> "`).

**Acceptance**: AC-CLITUI3-005.

#### REQ-CLITUI3-006 — Edit mode Enter regenerates
**When** the user submits (Enter) while the conversation is in edit mode, the TUI **shall**: (a) remove the message being edited and its paired assistant response (if present) from the conversation; (b) exit edit mode; (c) submit the edited text as a new ChatStream request via the same path as a normal user-message send.

**Acceptance**: AC-CLITUI3-006.

#### REQ-CLITUI3-007 — Edit mode Esc cancels
**When** the user presses `Esc` while the conversation is in edit mode (and no modal/overlay is active), the TUI **shall** exit edit mode, clear the editor buffer, and restore the normal input prompt; the conversation history **shall** remain unmodified.

**Acceptance**: AC-CLITUI3-007.

### 4.3 State-Driven Requirements (조건부 동작)

#### REQ-CLITUI3-008 — sessionmenu captures input and handles navigation while open
**While** the sessionmenu overlay is open, the TUI **shall** route all key events exclusively to the sessionmenu update handler (no key event reaches the main editor, slash command parser, or streaming handlers); Arrow Up/Down **shall** move the cursor with clamp behavior (no wrap); Enter **shall** load the cursor-selected session via the existing /load path; Esc **shall** dismiss the overlay without modifying conversation history or any other state.

**Acceptance**: AC-CLITUI3-001 (cursor 이동 시 editor 미반응), AC-CLITUI3-002 (Arrow + Enter 로 선택 로드).

### 4.4 Unwanted Behavior Requirements (금지)

#### REQ-CLITUI3-009 — Ctrl-Up no-op while streaming
**If** the user presses `Ctrl-Up` while streaming is in progress, **then** the TUI **shall not** activate edit mode; the key press **shall** produce no change to the edit-mode state, the editor buffer, or the streaming session, and **shall** not emit any log output.

**Acceptance**: AC-CLITUI3-008.

---

## 5. i18n Catalog 설계

### 5.1 Catalog struct (영어 기준)

```go
// internal/cli/tui/i18n/catalog.go
// Catalog holds all user-visible strings for one locale.
// Lang is the BCP-47 language tag (e.g. "en", "ko").
type Catalog struct {
    Lang                  string
    StatusbarIdle         string // "Session: %s | Daemon: %s | Messages: %d"
    SessionMenuHeader     string // "Recent sessions"
    SessionMenuEmpty      string // "[no recent sessions]"
    EditPrompt            string // "(edit)> "
    SlashHelpHeader       string // "Conversation commands"
    PermissionPrompt      string // "Allow this tool call?"
    PermissionAllowOnce   string // "Allow once"
    PermissionDenyOnce    string // "Deny once"
    PermissionAllowAlways string // "Allow always (this tool)"
    PermissionDenyAlways  string // "Deny always (this tool)"
    Saved                 string // "[saved: %s]"
    Loaded                string // "[loaded: %s, %d messages]"
}
```

### 5.2 Locale 매트릭스

| Field                 | en (default)                       | ko                                |
|-----------------------|------------------------------------|------------------------------------|
| StatusbarIdle         | `Session: %s \| Daemon: %s \| Messages: %d` | `세션: %s \| 데몬: %s \| 메시지: %d` |
| SessionMenuHeader     | `Recent sessions`                  | `최근 세션`                       |
| SessionMenuEmpty      | `[no recent sessions]`             | `최근 세션 없음`                  |
| EditPrompt            | `(edit)> `                         | `(편집)> `                        |
| SlashHelpHeader       | `Conversation commands`            | `대화 명령어`                     |
| PermissionPrompt      | `Allow this tool call?`            | `이 도구 호출을 허용하시겠습니까?` |
| PermissionAllowOnce   | `Allow once`                       | `이번만 허용`                     |
| PermissionDenyOnce    | `Deny once`                        | `이번만 거부`                     |
| PermissionAllowAlways | `Allow always (this tool)`         | `항상 허용 (이 도구)`             |
| PermissionDenyAlways  | `Deny always (this tool)`          | `항상 거부 (이 도구)`             |
| Saved                 | `[saved: %s]`                      | `[저장됨: %s]`                    |
| Loaded                | `[loaded: %s, %d messages]`        | `[불러옴: %s, %d 메시지]`         |

### 5.3 Loader 결정 흐름

```
1. CWD 기준 .moai/config/sections/language.yaml 시도
2. 실패 시 git rev-parse --show-toplevel 결과 + .moai/config/sections/language.yaml
3. 둘 다 실패 시 "en" 기본 Catalog 반환 (에러 아님, 로그 침묵)
4. yaml 파싱 후 conversation_language 추출
5. 알 수 없는 언어 코드일 경우 "en" fallback
```

---

## 6. 패키지 레이아웃

```
internal/cli/tui/
├── model.go                [MODIFY]   # +sessionMenuState +editingMessageIndex +catalog
├── view.go                 [MODIFY]   # sessionmenu overlay render, catalog 문자열 치환
├── update.go               [MODIFY]   # KeyCtrlR, KeyCtrlUp; KeyEscape priority chain 확장; edit-mode Enter 경로
├── slash.go                [MODIFY]   # /help header catalog 치환
├── permission/
│   └── view.go             [MODIFY]   # modal label 4 종 catalog 치환
├── sessionmenu/            [NEW]
│   ├── model.go            # Entry{Name string, Path string, ModTime time.Time}, State{open, entries, cursor}
│   ├── view.go             # lipgloss overlay box, cursor highlight
│   ├── update.go           # Arrow Up/Down, Enter→LoadSessionMsg, Esc→CloseMsg
│   └── loader.go           # glob ~/.goose/sessions/*.jsonl, stat, sort desc, cap 10
├── i18n/                   [NEW]
│   ├── catalog.go          # Catalog struct + ko/en embedded
│   └── loader.go           # reads .moai/config/sections/language.yaml, returns Catalog
└── testdata/snapshots/
    ├── session_menu_open.golden          [NEW]
    ├── statusbar_idle_ko.golden          [NEW]
    ├── statusbar_idle_en.golden          [NEW]
    ├── slash_help_ko.golden              [NEW]
    ├── slash_help_en.golden              [NEW]
    ├── permission_modal_ko.golden        [NEW]
    ├── permission_modal_en.golden        [NEW]
    ├── session_menu_open_ko.golden       [NEW]
    └── session_menu_open_en.golden       [NEW]
```

### 6.1 DELTA 마커

| File                                | Type        | Change                                                                                                     |
|-------------------------------------|-------------|------------------------------------------------------------------------------------------------------------|
| `model.go`                          | [MODIFY]    | +sessionMenuState (sessionmenu.State) +editingMessageIndex (int, -1 = idle) +catalog (i18n.Catalog)          |
| `view.go`                           | [MODIFY]    | catalog 문자열 치환 (statusbar, prompts) + sessionmenu overlay render 분기                                 |
| `update.go`                         | [MODIFY]    | +KeyCtrlR (sessionmenu open) +KeyCtrlUp (edit mode 진입); KeyEscape priority chain 확장; edit-mode Enter 경로 |
| `slash.go`                          | [MODIFY]    | /help header → catalog.SlashHelpHeader                                                                     |
| `permission/view.go`                | [MODIFY]    | modal prompt + 4 button label → catalog                                                                    |
| `sessionmenu/{model,view,update,loader}.go` | [NEW] | 4 file 신규                                                                                              |
| `i18n/{catalog,loader}.go`          | [NEW]       | 2 file 신규                                                                                                |
| `testdata/snapshots/*.golden`       | [NEW]       | 9 신규 (1 base + 8 i18n)                                                                                   |

---

## 7. 의존성 (Dependencies)

| Type         | Target                                                | Notes                                                                |
|--------------|-------------------------------------------------------|----------------------------------------------------------------------|
| 선행 SPEC (FROZEN) | SPEC-GOOSE-CLI-TUI-002 v0.1.0 (implemented)         | KeyEscape priority chain, permission/, editor/, snapshots/ 보존     |
| 선행 SPEC    | SPEC-GOOSE-CONFIG-001                                 | `.moai/config/sections/language.yaml` 경로                           |
| 선행 SPEC    | SPEC-GOOSE-CLI-001 v0.2.0                             | `internal/cli/session/file.go` (`/load` 재사용 — sessionmenu Enter)  |
| 외부 lib     | `github.com/charmbracelet/bubbletea` v1.2+            | TUI 프레임워크 (계승)                                                |
| 외부 lib     | `github.com/charmbracelet/lipgloss` v1.1+             | sessionmenu overlay 박스 렌더 (계승)                                 |
| 외부 lib     | `gopkg.in/yaml.v3`                                    | language.yaml 파싱                                                   |

---

## 8. 위험 (Risks)

| #  | Risk                                                                                        | Mitigation                                                                                                                   |
|----|---------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| R1 | i18n retrofit 으로 view.go / permission/view.go / slash.go 의 기존 6 개 golden 이 silently 깨질 수 있음 | catalog wiring 후 `go test -update` 로 기존 golden 재생성 (값 변경 없을 시 byte-identical 확인). RED phase 에 회귀 detection 포함 |
| R2 | Ctrl-Up 이 multi-line editor 모드 (`editorMode=multi`) 에서 ambiguous 동작                  | Ctrl-Up 은 `editor.IsMulti() == false` 이고 `editor.Value() == ""` 일 때만 활성. multi 모드에서는 silently 무시              |
| R3 | sessionmenu Enter 가 streaming 중에 호출되면 race condition                                 | sessionmenu Entry 로드는 `streaming == false` 일 때만 트리거. streaming 중에는 catalog-localized 경고를 menu body 에 표시       |
| R4 | language.yaml CWD 의존성 — TUI 가 다른 디렉터리에서 호출될 때                                | Loader 가 (1) CWD-relative, (2) `git rev-parse --show-toplevel` 기반 경로, (3) "en" 기본의 3-tier fallback 사용                |
| R5 | `editingMessageIndex+1` out-of-bounds — user 만 보내고 assistant 응답 미수신 상태              | Guard: `editingMessageIndex+1 >= len(messages)` 면 `[editingMessageIndex]` 만 제거                                          |
| R6 | sessionmenu cursor wrap 여부 — UX 일관성                                                    | clamp (no wrap). Up at 0 stays at 0, Down at last stays at last. Bubbletea convention 준수                                  |
| R7 | KeyEscape priority chain 확장 시 CLI-TUI-002 의 5-tier 가 깨질 수 있음                       | 통합 테스트 (TestKeyEscape_PriorityChain_AllStates) 로 6-tier 전체 검증: modal > sessionmenu > edit > stream cancel > idle    |

---

## 9. MX 태그 계획 (MX Plan)

### 9.1 ANCHOR 후보 (high fan_in)

| 함수                                       | 위치                                | 이유                                                                  |
|--------------------------------------------|-------------------------------------|-----------------------------------------------------------------------|
| `i18n.LoadCatalog()`                       | `internal/cli/tui/i18n/loader.go`   | 모든 view.go / permission/view.go / slash.go 가 의존 (fan_in ≥ 4)     |
| `sessionmenu.LoadEntries()`                | `internal/cli/tui/sessionmenu/loader.go` | Ctrl-R 핸들러 + 추후 /sessions 슬래시 명령에서 호출 가능 (fan_in ≥ 2) |
| `(*Model).enterEditMode()`                 | `internal/cli/tui/update.go`        | KeyCtrlUp 분기 + edit-mode 종료 후 재진입에서 호출 (fan_in ≥ 2)       |

### 9.2 NOTE 후보

| 함수                                       | 의도 메모                                                                  |
|--------------------------------------------|----------------------------------------------------------------------------|
| `sessionmenu.update.handleEnter()`         | streaming 중에는 catalog-localized 경고 — race condition 회피 (R3)          |
| `i18n.loader.tryGitToplevel()`             | CWD 의존성 회피용 fallback (R4)                                            |
| `(*Model).exitEditMode()`                  | messages 슬라이스 변경 없이 상태만 복원 — Esc 의 비파괴성 명시 (REQ-CLITUI3-007) |

### 9.3 WARN 후보

| 함수                                       | 위험 + REASON                                                              |
|--------------------------------------------|----------------------------------------------------------------------------|
| `(*Model).regenerateFromEdit()`            | messages slice 의 in-place 수정 + immediate ChatStream 호출. **REASON**: editingMessageIndex+1 가드 (R5) 미적용 시 panic. 모든 수정 경로는 guard 통과 후 진입. |
| `sessionmenu.loader.glob()`                | `~/.goose/sessions/*.jsonl` 에 대해 os.Stat 호출 — 큰 디렉터리에서 latency. **REASON**: 10 cap + lazy glob (Ctrl-R press 시점에만 호출). 실시간 watching 부재 (Exclusion §3.2-5). |

### 9.4 TODO (RED phase)

- `@MX:TODO P1-T1` — i18n.LoadCatalog 의 yaml.v3 unmarshal 실패 시 fallback 경로 (RED 테스트 통과 시 제거)
- `@MX:TODO P2-T2` — sessionmenu loader 의 mtime 동률 (tie-break) 처리 (RED 테스트 통과 시 제거)
- `@MX:TODO P3-T1` — editingMessageIndex+1 out-of-bounds guard (RED 테스트 통과 시 제거)

---

## 10. Acceptance Criteria 요약 (상세는 acceptance.md)

각 AC 는 단일 binary-testable 행동 진술이며, 연결 REQ 와 phase 를 함께 명시한다. Given/When/Then 시나리오 (입력/출력, 사전조건, edge case) 는 acceptance.md §1 참조.

| AC ID              | 연결 REQ              | 형식적 행동 진술                                                                                                                                          | Phase |
|--------------------|-----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|-------|
| AC-CLITUI3-001     | REQ-CLITUI3-003, -008 | When the user presses Ctrl-R while the TUI is idle and three sessions exist, the overlay shall open populated with three entries sorted mtime descending and cursor at the first entry. | P2    |
| AC-CLITUI3-002     | REQ-CLITUI3-008       | When the user presses Arrow Down twice (cursor clamped at last entry) followed by Enter while the overlay is open, the TUI shall load the cursor-selected session via the existing /load path. | P2    |
| AC-CLITUI3-003     | REQ-CLITUI3-004       | When the user presses Ctrl-R while the session storage directory is absent or contains zero entries, the overlay shall render the catalog `SessionMenuEmpty` string and dismiss within the same Update cycle without further user input. | P2    |
| AC-CLITUI3-004     | REQ-CLITUI3-002, -003 | While three deterministic mock session entries are loaded under fixed clock and ascii termenv, the sessionmenu overlay snapshot **shall** be byte-identical to `session_menu_open.golden` on macOS and Linux. | P4    |
| AC-CLITUI3-005     | REQ-CLITUI3-005       | When the user presses Ctrl-Up while the editor is empty, single-line, not streaming, and the conversation contains at least one user message, the TUI shall load the most recent user message into the editor, enter edit mode, and switch the prompt prefix to the catalog `EditPrompt`. | P3    |
| AC-CLITUI3-006     | REQ-CLITUI3-006       | When the user submits Enter while in edit mode, the TUI shall remove the message being edited and its paired assistant response (if present), exit edit mode, and submit the edited text as a new ChatStream request. | P3    |
| AC-CLITUI3-007     | REQ-CLITUI3-007       | When the user presses Esc while in edit mode (no modal/overlay active), the TUI shall exit edit mode, clear the editor buffer, restore the normal prompt, and leave the conversation history unmodified. | P3    |
| AC-CLITUI3-008     | REQ-CLITUI3-009       | If the user presses Ctrl-Up while streaming is active, then the TUI **shall not** activate edit mode and the editor buffer **shall** remain unchanged. | P3    |
| AC-CLITUI3-009     | REQ-CLITUI3-001       | While `conversation_language` is configured as `ko`, the four user-facing surfaces (statusbar idle, /help response, permission modal labels, sessionmenu header) **shall** render Korean substrings matching their `*_ko.golden` snapshot counterparts byte-identically. | P4    |
| AC-CLITUI3-010     | REQ-CLITUI3-001       | While `conversation_language` is configured as `en` or the configuration is absent, the four user-facing surfaces **shall** render English substrings matching their `*_en.golden` snapshot counterparts byte-identically. | P4    |

---

## 11. 완료 기준 (Definition of Done)

- [ ] 9 개 REQ-CLITUI3-XXX 모두 GREEN.
- [ ] 10 개 AC-CLITUI3-XXX 모두 PASS.
- [ ] `internal/cli/tui/i18n/` + `sessionmenu/` 패키지 신규 + cover ≥ 80%.
- [ ] `internal/cli/tui/{model,view,update,slash,permission/view}.go` MODIFY 후 cover ≥ 75% 유지.
- [ ] 9 개 신규 golden + CLI-TUI-002 의 기존 6 개 golden 모두 byte-identical (linux + macOS CI 양쪽).
- [ ] `go vet ./internal/cli/tui/...`, `gofmt -l` zero diff.
- [ ] CLI-TUI-002 의 KeyEscape priority chain 5-tier 가 6-tier 로 확장된 후 모든 기존 동작 byte-identical (regression test 통과).
- [ ] `go test -race ./internal/cli/tui/...` PASS.
- [ ] LoC 추가 ≤ 700 (sessionmenu/ ≈ 280, i18n/ ≈ 120, MODIFY ≈ 200, snapshot ≈ 100).

---

## 12. 참고 (References)

- SPEC-GOOSE-CLI-TUI-002 v0.1.0 (implemented) — KeyEscape priority chain, permission modal, editor, snapshots
- SPEC-GOOSE-CLI-001 v0.2.0 — `internal/cli/session/file.go` (sessionmenu Enter 시 `/load` 재사용)
- SPEC-GOOSE-CONFIG-001 — `.moai/config/sections/language.yaml` 형식
- charmbracelet/bubbletea v1.2+ overlay pattern (lipgloss `Place` + `Center`)

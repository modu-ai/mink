# SPEC-GOOSE-CLI-TUI-003 — Acceptance Criteria (상세)

> **목적**: spec.md §10 의 10 AC 를 fuller Given/When/Then 형식으로 재구성. 구체 입력/출력, snapshot 파일명, 회귀 보호 범위, 성능/coverage gate, DoD 명시.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 10 AC 상세화 + snapshot 규약 + coverage gate + DoD | manager-spec |

---

## 0. 일반 규약

### 0.1 Snapshot 파일 명명 규약

- 경로: `internal/cli/tui/testdata/snapshots/<scenario_slug>.golden`
- slug 규칙: `<surface>_<state>_<locale?>` (snake_case, lowercase)
- 본 SPEC 신규 9 개:
  - `session_menu_open.golden` (base, locale 무관)
  - `statusbar_idle_{ko,en}.golden`
  - `slash_help_{ko,en}.golden`
  - `permission_modal_{ko,en}.golden`
  - `session_menu_open_{ko,en}.golden`
- 갱신 명령: `go test -update ./internal/cli/tui/...`
- Git diff 검토 의무: PR review 시 snapshot 변경분 시각 검증 (의도된 UI 변경인지 확인)
- CLI-TUI-002 의 기존 6 golden 도 catalog 치환 후 byte-identical 회귀 보호 (en 기본값 = hardcoded)

### 0.2 테스트 환경 표준화

모든 snapshot 테스트 setup (CLI-TUI-002 §0.2 와 동일):

```go
func TestSnapshot_X(t *testing.T) {
    snapshots.SetupAsciiTermenv(t)            // lipgloss color profile = ascii
    clock := snapshots.FixedClock(             // Fixed clock for spinner determinism
        time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
    )
    model := tui.NewModel(mockClient, "(unnamed)", false)
    model.Clock = clock                         // Dependency injection
    // ... interactions
    snapshots.RequireSnapshot(t, "scenario_name.golden", finalOutput)
}
```

i18n 테스트의 추가 setup:

```go
// Override language.yaml path via MOAI_LANGUAGE_YAML_OVERRIDE env var
t.Setenv("MOAI_LANGUAGE_YAML_OVERRIDE", "testdata/language_ko.yaml")
// or for default English: leave env unset, file absent
```

### 0.3 성능 게이트

| 메트릭                                        | 임계값       | 측정 방법                                                                |
|----------------------------------------------|--------------|--------------------------------------------------------------------------|
| sessionmenu Ctrl-R press → overlay render    | < 50 ms      | `time.Now()` press → 첫 view 출력. 10 entry 기준                          |
| Ctrl-Up press → editor populated             | < 30 ms      | `time.Now()` press → editor.Value() 비어 있지 않게 됨                     |
| Edit Enter → ChatStream 호출 시작            | < 50 ms      | `time.Now()` Enter → ChatStream.Send() 진입                              |
| i18n LoadCatalog (cold start)                | < 20 ms      | yaml.v3 unmarshal + struct 매칭                                          |

### 0.4 Coverage 게이트

| 패키지                                       | 임계값        |
|---------------------------------------------|---------------|
| `internal/cli/tui/i18n/`                    | ≥ 80%         |
| `internal/cli/tui/sessionmenu/`             | ≥ 80%         |
| `internal/cli/tui/` (MODIFY 후 전체)         | ≥ 75% 유지    |

---

## 1. AC 시나리오 (10 개)

### AC-CLITUI3-001 — Ctrl-R 로 sessionmenu overlay 가 mtime 정렬로 열림

**연결 REQ**: REQ-CLITUI3-002, REQ-CLITUI3-003, REQ-CLITUI3-008

**Given**:
- `~/.goose/sessions/` 에 3 개 .jsonl 파일 존재 (mtime 오름차순으로 a < b < c)
- TUI 가 idle 상태 (no streaming, no modal, editor 비어 있음)
- editor 가 single-line 모드

**When**:
- 사용자가 `Ctrl-R` 키를 누른다

**Then**:
- `sessionMenuState.open == true`
- `sessionMenuState.entries` 길이 = 3
- entries[0] = c (가장 최근 mtime), entries[1] = b, entries[2] = a (mtime 내림차순)
- `sessionMenuState.cursor == 0` (첫 entry)
- view 가 lipgloss overlay 박스를 렌더, 첫 entry 가 cursor highlight
- main editor 는 입력 무반응 (REQ-CLITUI3-008)

**Cleanup verification**:
- `Esc` 키 누르면 `sessionMenuState.open == false` 로 닫힘
- messages 슬라이스 변경 없음, 다른 상태 변경 없음 (no side effect)

**Test**: `TestSessionMenu_CtrlR_OpensList` in `internal/cli/tui/sessionmenu/update_test.go` + integration in `internal/cli/tui/update_test.go`

---

### AC-CLITUI3-002 — Arrow ↓×2 + Enter 로 두 번째 다음 세션이 로드됨

**연결 REQ**: REQ-CLITUI3-003, REQ-CLITUI3-008

**Given**:
- AC-CLITUI3-001 의 setup 동일 (3 entry sessionmenu open, cursor at 0)

**When**:
1. 사용자가 Arrow Down 을 누른다 → cursor = 1
2. 다시 Arrow Down 을 누른다 → cursor = 2 (clamp at last, no wrap)
3. Enter 를 누른다

**Then**:
- 선택된 entry (entries[2] = `a`, mtime 가장 오래된 것) 가 `internal/cli/session/file.go` 의 `/load` 경로로 로드된다
- `sessionMenuState.open == false` (overlay 닫힘)
- 시스템 메시지 "[loaded: a, N messages]" 가 messages 에 추가됨 (catalog `Loaded` 사용 — en 기본 시 "[loaded: ...]")
- TUI 가 idle 상태로 복귀, editor 입력 가능

**Negative case**:
- Down 을 한 번 더 누르면 cursor 는 2 에 머무름 (clamp, no wrap)

**Test**: `TestSessionMenu_Navigation_ClampNoWrap`, `TestSessionMenu_Enter_LoadsSelected`

---

### AC-CLITUI3-003 — sessionmenu empty state 가 자동 dismiss

**연결 REQ**: REQ-CLITUI3-004

**Given**:
- `~/.goose/sessions/` 디렉터리가 부재이거나 .jsonl 파일이 0 개
- TUI idle

**When**:
- 사용자가 Ctrl-R 을 누른다

**Then**:
- 같은 Update cycle 내에:
  1. overlay 가 잠시 열리며 `SessionMenuEmpty` 카탈로그 문자열 단일 라인 표시 (en: `[no recent sessions]`, ko: `최근 세션 없음`)
  2. 즉시 닫힘 (`sessionMenuState.open == false`) — 사용자 별도 입력 불필요
- 정상 idle 상태로 복귀, 부작용 없음

**Edge case**:
- `~/.goose/` 자체가 없을 때도 동일 동작 (no error)
- `~/.goose/sessions/` 에 `.jsonl` 가 아닌 파일만 있을 때도 동일 동작

**Test**: `TestSessionMenu_EmptyState_AutoDismiss`, `TestSessionMenu_Loader_AbsentDir`, `TestSessionMenu_Loader_EmptyDir`

---

### AC-CLITUI3-004 — sessionmenu base golden snapshot

**연결 REQ**: REQ-CLITUI3-002, REQ-CLITUI3-003

**Given**:
- 3 mock session entry: `["alpha", "beta", "gamma"]`, mtime 오름차순으로 `alpha` < `beta` < `gamma`
- TUI width 80, height 24, ascii termenv (`SetupAsciiTermenv`)
- Fixed clock = 2026-05-05 12:00:00 UTC
- en locale (default, language.yaml 부재 또는 `conversation_language: en`)

**When**:
- Ctrl-R 누름 → sessionmenu overlay 열림 → final output 캡처

**Then**:
- `internal/cli/tui/testdata/snapshots/session_menu_open.golden` 와 byte-identical
- Snapshot 의 첫 줄에 catalog `SessionMenuHeader` ("Recent sessions") 포함
- 3 entry 가 mtime 내림차순 (gamma → beta → alpha) 으로 표시
- 첫 entry 에 cursor 표식

**Regression scope**:
- linux + macOS CI 양쪽 byte-identical
- TERM=dumb 에서도 동작 (terminfo 비의존, CLI-TUI-002 §3.1 Area 1 §5)

**Test**: `TestSnapshot_SessionMenuOpen` in `internal/cli/tui/snapshot_sessionmenu_test.go`

---

### AC-CLITUI3-005 — Ctrl-Up 으로 마지막 user message 가 editor 로 로드됨

**연결 REQ**: REQ-CLITUI3-005

**Given**:
- `messages = [{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}]`
- editor 가 single-line 모드, `editor.Value() == ""`
- `streaming == false`
- `editingMessageIndex == -1`

**When**:
- 사용자가 `Ctrl-Up` 을 누른다

**Then**:
- `editor.Value() == "hello"` (마지막 user content 로드)
- `editingMessageIndex == 0` (lastUserIndex)
- view 의 prompt prefix 가 `catalog.EditPrompt` (en: `"(edit)> "`, ko: `"(편집)> "`)
- messages 슬라이스 변경 없음 (아직)

**Negative cases (REQ-CLITUI3-005 가드)**:
- editor 가 비어 있지 않으면 (`editor.Value() != ""`) Ctrl-Up no-op
- editor 가 multi-line 모드면 (R2) Ctrl-Up no-op
- streaming 중이면 (REQ-CLITUI3-009) Ctrl-Up no-op
- user message 가 0 개면 Ctrl-Up no-op

**Test**: `TestEdit_CtrlUp_EntersEditMode`, `TestEdit_CtrlUp_GuardEditorNotEmpty`, `TestEdit_CtrlUp_GuardSingleMode`

---

### AC-CLITUI3-006 — Edit Enter 가 직전 pair 를 제거하고 새 turn 으로 재생성

**연결 REQ**: REQ-CLITUI3-006

**Given**:
- AC-CLITUI3-005 후속 상태:
  - `messages = [{user:"hello"}, {assistant:"hi"}]`
  - `editor.Value() == "hello"`
  - `editingMessageIndex == 0`
- 사용자가 editor 의 텍스트를 `"hello world"` 로 수정

**When**:
- 사용자가 Enter 를 누른다

**Then** (순서대로):
1. `messages[0]` ("hello") 와 `messages[1]` ("hi") 가 슬라이스에서 제거됨 → `messages = []` (길이 0)
2. `editingMessageIndex == -1` (리셋)
3. `sendMessage("hello world")` 가 호출됨 → 정상 ChatStream 경로 진입
4. ChatStream 응답 도착 후 `messages` 에 새 user("hello world") + assistant 추가
5. editor 가 비워지고 prompt 가 정상 prefix 로 복원

**Out-of-bounds case (R5 guard)**:
- Given: `messages = [{user:"hello"}]` (assistant 미수신), `editingMessageIndex == 0`
- When: Enter
- Then: `messages[0]` 만 제거되어 `messages = []`, `messages[1]` 접근으로 panic 없음

**Test**: `TestEdit_Enter_RegeneratesLastTurn`, `TestEdit_Enter_OutOfBoundsGuard`

---

### AC-CLITUI3-007 — Edit Esc 가 messages 변경 없이 편집 상태만 취소

**연결 REQ**: REQ-CLITUI3-007

**Given**:
- AC-CLITUI3-005 후속 상태:
  - `messages = [{user:"hello"}, {assistant:"hi"}]`
  - `editor.Value() == "hello world"` (사용자가 수정 중)
  - `editingMessageIndex == 0`
- 다른 overlay/modal 없음

**When**:
- 사용자가 `Esc` 를 누른다

**Then**:
- `messages` 슬라이스 unchanged: `[{user:"hello"}, {assistant:"hi"}]` (길이 2, 내용 동일)
- `editingMessageIndex == -1`
- `editor.Value() == ""` (editor 클리어)
- prompt prefix 가 정상으로 복원 (catalog `EditPrompt` 아님)

**KeyEscape priority chain 검증**:
- modal 도 없고 sessionmenu 도 없는 상태에서 Esc 가 edit 단계에 도달
- streaming 도 아니므로 stream cancel 단계로 떨어지지 않음
- 6-tier chain: modal(false) → sessionmenu(false) → **edit(true → handle)** → stream(skip) → idle(skip)

**Test**: `TestEdit_Esc_CancelsEditMode`, `TestKeyEscape_PriorityChain_EditTier`

---

### AC-CLITUI3-008 — streaming 중 Ctrl-Up no-op

**연결 REQ**: REQ-CLITUI3-009

**Given**:
- `streaming == true`
- `messages = [{user:"hello"}, {assistant:"partial response..."}]` (assistant 가 부분 수신 중)
- `editor.Value() == ""`
- `editingMessageIndex == -1`

**When**:
- 사용자가 `Ctrl-Up` 을 누른다

**Then**:
- `editingMessageIndex == -1` (변경 없음)
- `editor.Value() == ""` (변경 없음)
- streaming 계속 진행 (interrupt 없음)
- 로그 출력 없음 (silent ignore)

**Verification 추가**:
- streaming 종료 후 Ctrl-Up 다시 누르면 정상 활성 (AC-CLITUI3-005 동작)

**Test**: `TestEdit_NoopWhileStreaming`

---

### AC-CLITUI3-009 — i18n ko locale 4 surface golden

**연결 REQ**: REQ-CLITUI3-001

**Given**:
- `language.yaml` 가 `conversation_language: ko` 로 설정 (또는 `MOAI_LANGUAGE_YAML_OVERRIDE` env 로 ko fixture 지정)
- TUI 가 NewModel() 로 초기화 → `LoadCatalog()` 가 `catalogKo` 반환
- ascii termenv, fixed clock, width 80

**When**: 4 surface 를 각각 캡처
1. **statusbar idle**: TUI 초기 상태 (no streaming, no modal) 의 statusbar 만 추출 → `statusbar_idle_ko.golden`
2. **/help**: editor 에 `/help` 입력 후 Enter → 응답 영역 캡처 → `slash_help_ko.golden`
3. **permission modal**: mock permission_request 이벤트 주입 → modal open 상태 캡처 → `permission_modal_ko.golden`
4. **sessionmenu open**: 3 mock entry + Ctrl-R → overlay 캡처 → `session_menu_open_ko.golden`

**Then** (각 surface 별):
1. `statusbar_idle_ko.golden` 에 한국어 substring `"세션:"` 포함 (catalog `StatusbarIdle`)
2. `slash_help_ko.golden` 에 한국어 substring `"대화 명령어"` 포함 (catalog `SlashHelpHeader`)
3. `permission_modal_ko.golden` 에 한국어 substring `"이 도구 호출을 허용하시겠습니까?"` 포함 (catalog `PermissionPrompt`) + 4 button label 포함 (`"이번만 허용"`, `"이번만 거부"`, `"항상 허용"`, `"항상 거부"`)
4. `session_menu_open_ko.golden` 에 한국어 substring `"최근 세션"` 포함 (catalog `SessionMenuHeader`)

**File 검증**:
- 4 신규 golden 모두 linux + macOS CI 양쪽 byte-identical

**Test**: `TestSnapshot_I18N_Ko` (테이블 케이스 4 개)

---

### AC-CLITUI3-010 — i18n en locale 4 surface golden (default)

**연결 REQ**: REQ-CLITUI3-001

**Given**:
- `language.yaml` 가 `conversation_language: en` 로 설정 OR 파일 부재 (default fallback)
- TUI 가 NewModel() 로 초기화 → `LoadCatalog()` 가 `catalogEn` 반환
- 나머지 setup AC-CLITUI3-009 와 동일

**When**: AC-CLITUI3-009 와 동일한 4 surface 캡처 → `*_en.golden`

**Then** (각 surface 별):
1. `statusbar_idle_en.golden` 에 영어 substring `"Session:"` 포함
2. `slash_help_en.golden` 에 영어 substring `"Conversation commands"` 포함
3. `permission_modal_en.golden` 에 영어 substring `"Allow this tool call?"` 포함 + 4 button label `"Allow once"`, `"Deny once"`, `"Allow always"`, `"Deny always"` 포함
4. `session_menu_open_en.golden` 에 영어 substring `"Recent sessions"` 포함

**Default fallback verification**:
- `language.yaml` 가 부재한 상태로 같은 시나리오 실행 → 동일한 4 golden 과 byte-identical (REQ-CLITUI3-001 의 silent fallback 검증)
- 알 수 없는 언어 코드 (e.g., `conversation_language: ja`) → 동일한 4 en golden 과 byte-identical (REQ-CLITUI3-001 의 unknown lang fallback 검증)

**Test**: `TestSnapshot_I18N_En` (테이블 케이스 4 개) + `TestI18N_DefaultsToEnglish` (loader unit test) + `TestI18N_DefaultsToEnglish_UnknownLang`

---

## 2. 통합 테스트 (Integration Tests, AC 외 추가)

### 2.1 KeyEscape priority chain 6-tier 통합 (R7 mitigation)

`TestKeyEscape_PriorityChain_6Tier` — modal 만 활성 / sessionmenu 만 활성 / edit 만 활성 / streaming 만 활성 / idle 의 5 개 조합에서 Esc 가 정확히 한 단계만 처리하는지 검증.

| 조합                                                                | Esc 의 효과                              |
|---------------------------------------------------------------------|------------------------------------------|
| modal=true, others=false                                            | modal 닫음 (sessionmenu/edit/stream 영향 없음) |
| modal=false, sessionmenu=true                                       | sessionmenu 닫음                         |
| modal=false, sessionmenu=false, edit=true                           | edit 모드 취소 (AC-CLITUI3-007)          |
| modal=false, sessionmenu=false, edit=false, streaming=true          | stream cancel (CLI-TUI-002 보존)        |
| 모두 false                                                           | idle no-op (CLI-TUI-002 보존)            |

### 2.2 sessionmenu 가 streaming 중에 열렸을 때 동작 (R3 mitigation)

`TestSessionMenu_OpenDuringStreaming_BlocksEnter` — streaming=true 상태에서 Ctrl-R 누르면 overlay 열리지만 Enter 가 비활성 (또는 catalog-localized 경고 표시), Esc 만 작동.

### 2.3 회귀 보호 — CLI-TUI-002 의 6 기존 golden

`TestSnapshot_LegacyCliTui002_AllStable` — chat_repl_initial_render, streaming_in_progress, streaming_aborted, permission_modal_open, slash_help_local, editor_multiline 6 개 golden 이 catalog 치환 후에도 byte-identical 인지 확인 (en 기본값과 hardcoded 가 일치하므로 변경 없어야 함).

---

## 3. Definition of Done (DoD)

본 SPEC 의 status 가 `draft` → `implemented` 로 전환되려면 다음 모두 충족:

### 3.1 기능 완료
- [ ] 9 개 REQ-CLITUI3-XXX 모두 GREEN
- [ ] 10 개 AC-CLITUI3-XXX 모두 PASS
- [ ] §2 의 3 개 통합 테스트 모두 GREEN

### 3.2 패키지 + LoC
- [ ] `internal/cli/tui/i18n/` (신규) cover ≥ 80%
- [ ] `internal/cli/tui/sessionmenu/` (신규) cover ≥ 80%
- [ ] `internal/cli/tui/` (MODIFY) cover ≥ 75% 유지
- [ ] LoC 추가 ≤ 700 (sessionmenu/ ≈ 280, i18n/ ≈ 150, MODIFY ≈ 200, snapshot test ≈ 70)

### 3.3 Golden 회귀
- [ ] 9 개 신규 golden file 모두 linux + macOS CI 양쪽 byte-identical
- [ ] CLI-TUI-002 의 기존 6 개 golden 도 byte-identical (회귀 zero)
- [ ] `go test -update ./internal/cli/tui/...` 후 PR review 시각 검증 통과

### 3.4 정적 검증
- [ ] `go vet ./internal/cli/tui/...` zero warning
- [ ] `gofmt -l ./internal/cli/tui/...` zero diff
- [ ] `go test -race ./internal/cli/tui/...` PASS

### 3.5 회귀 보호
- [ ] CLI-TUI-002 의 KeyEscape 5-tier 동작이 6-tier 확장 후에도 byte-identical (modal 닫기 / streaming cancel / idle no-op 모두 동일)
- [ ] CLI-TUI-002 의 permission modal, /save, /load, editor multi-line 동작 byte-identical

### 3.6 문서
- [ ] spec.md HISTORY 표 갱신 (status `draft` → `implemented` + 머지된 PR 번호 기록)
- [ ] 변경된 동작 (Ctrl-R, Ctrl-Up) 가 README 또는 CLI-TUI-002 progress.md 의 후속 노트에 기록

### 3.7 PR 머지 순서
- [ ] P1 PR (i18n catalog) 머지
- [ ] P2 PR (sessionmenu) 머지
- [ ] P3 PR (edit/regenerate) 머지
- [ ] P4 PR (9 golden) 머지
- [ ] sync PR (status 전환 + progress.md 갱신) 머지

---

## 4. 비공식 행동 검증 (Manual UX Smoke Test)

자동 테스트 외에 사람 조작으로 한 번 검증해야 할 시나리오:

1. **Ctrl-R 빈 디렉터리**: 새 환경에서 `~/.goose/sessions/` 가 아예 없는 상태로 TUI 실행 → Ctrl-R 시 catalog 메시지 보이고 즉시 닫힘
2. **Ctrl-Up 후 mind change**: Ctrl-Up 으로 편집 진입 → editor 에 추가 텍스트 입력 → Esc → messages 변경 없는지 위쪽 viewport 에서 확인
3. **ko/en 전환**: `language.yaml` 의 `conversation_language` 를 `ko` ↔ `en` 토글하며 TUI 재시작 → statusbar / 메뉴 / modal 의 언어 즉시 변경
4. **streaming 중 Ctrl-R**: 응답 스트리밍 중에 Ctrl-R 누름 → overlay 가 열리지만 streaming 은 중단되지 않고 background 진행 (R3 검증)
5. **6-tier KeyEscape**: permission modal 위에 sessionmenu 가 동시 열려 있을 수 없는지 (modal 이 sessionmenu 보다 우선) UI 로 확인

---

## 5. 참고

- spec.md §10 Acceptance Criteria 요약 표
- CLI-TUI-002 acceptance.md §0 일반 규약 (snapshot 표준 setup)
- CLI-001 v0.2.0 `internal/cli/session/file.go` (`/load` 재사용)

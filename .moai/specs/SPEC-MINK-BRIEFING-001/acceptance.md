---
id: SPEC-MINK-BRIEFING-001
version: 0.3.1
status: implemented
created_at: 2026-05-14
updated_at: 2026-05-15
author: manager-spec
---

# Acceptance Criteria — SPEC-MINK-BRIEFING-001

본 문서는 implementation 검증을 위한 binary acceptance gate 를 정의한다. 각 AC 는 `go test` / 파일 모드 확인 / grep / fixture diff 등 **자동 검증 가능** 한 형태로 명시.

## 1. AC Matrix Overview

| AC ID | 제목 | Milestone | 검증 명령 | 매핑 REQ |
|-------|------|-----------|---------|---------|
| AC-001 | Happy path morning briefing | M1 | `go test ./internal/ritual/briefing -run TestOrchestrator_HappyPath` | REQ-BR-001, REQ-BR-006 |
| AC-002 | Weather offline fallback | M1 | `go test ./internal/ritual/briefing -run TestCollectWeather_OfflineFallback` | REQ-BR-020 |
| AC-003 | Journal recall integration | M1 | `go test ./internal/ritual/briefing -run TestCollectJournal_Anniversary` | REQ-BR-004, REQ-BR-021 |
| AC-004 | 24절기 정확도 | M1 | `go test ./internal/ritual/briefing -run TestSolarTerm_2026Fixtures` | REQ-BR-005, REQ-BR-042 |
| AC-005 | 한국 명절 정확도 | M1 | `go test ./internal/ritual/briefing -run TestKoreanHoliday_2026Fixtures` | REQ-BR-005, REQ-BR-042 |
| AC-006 | Mantra config 적용 | M1 | `go test ./internal/ritual/briefing -run TestCollectMantra_Rotation` | REQ-BR-031 |
| AC-007 | Channel 다중 출력 | M2 | `go test ./internal/ritual/briefing -run TestRenderer_FanOut` | REQ-BR-002 |
| AC-008 | TUI panel 렌더 | M2 | `go test ./internal/cli/tui -run TestBriefingPanel_Snapshot` | REQ-BR-033 |
| AC-009 | Privacy 6 invariants | M1 + M2 | `go test ./internal/ritual/briefing -run TestPrivacy_Invariants` | REQ-BR-050 ~ REQ-BR-055 |
| AC-010 | CLI entrypoint | M1 | `mink briefing --help` 정상 출력 + `go test ./internal/cli/commands -run TestBriefingCmd` | REQ-BR-011 |
| AC-011 | SCHEDULER cron 등록 | M2 | `go test ./internal/ritual/scheduler -run TestBriefingMorningEvent` + `go test ./internal/ritual/briefing -run TestCronWiring` | REQ-BR-010 |
| AC-012 | Archive file 생성 + 권한 | M2 | `go test ./internal/ritual/briefing -run TestArchive_FilePerms` | REQ-BR-012, REQ-BR-030, REQ-BR-051 |
| EC-001 | Config malformed | M1 | `go test ./internal/ritual/briefing -run TestConfig_Malformed` | REQ-BR-040 |
| EC-002 | All modules failed | M1 | `go test ./internal/ritual/briefing -run TestOrchestrator_AllFail` | REQ-BR-041 |
| EC-003 | Clock skew (1899 or 2101) | M1 | `go test ./internal/ritual/briefing -run TestDateModule_OutOfRange` | REQ-BR-042 |
| EC-004 | Telegram token absent | M2 | `go test ./internal/ritual/briefing -run TestTelegram_TokenMissing` | REQ-BR-022 |
| **AC-013** | **Production real collectors wiring** | **M4** | `grep -iE "mock[A-Za-z]*factory" internal/cli/commands/*.go --exclude="*_test.go"` returns non-zero exit (no match) | **REQ-BR-001, REQ-BR-002, REQ-BR-064** |
| **AC-014** | **Orchestrator → LLM summary wiring** | **M4** | `go test -run TestOrchestrator_LLMSummary ./internal/ritual/briefing/` | **REQ-BR-032, REQ-BR-060** |
| **AC-015** | **Crisis hotline prepend in 3 channels** | **M4** | `go test -run TestRenderers_CrisisPrepend ./internal/ritual/briefing/` AND `go test -run TestBriefingPanel_CrisisPrepend ./internal/cli/tui/` (둘 다 PASS 필요) | **REQ-BR-055, REQ-BR-061** |
| **AC-016** | **/briefing TUI slash dispatch** | **M4** | `go test -run TestTUI_BriefingSlash ./internal/cli/tui/` | **REQ-BR-033, REQ-BR-062** |
| **AC-017** | **Deterministic Module Status order** | **M4** | `go test -run TestRenderCLI_Golden ./internal/ritual/briefing/` + `go test -run TestRenderCLI_StatusOrder_Fixed ./internal/ritual/briefing/` (golden + structural guard 둘 다 PASS) | **REQ-BR-063** |

총 17 AC + 4 EC = 21 binary gates (v0.3.0 의 12 AC + 4 EC + v0.3.1 신규 5 AC).

---

## 2. Acceptance Criteria 본문

### AC-001 — Happy path morning briefing

**Given**: WEATHER-001 mock 이 3 도구 정상 응답, JOURNAL-001 storage 에 1Y 전 anniversary entry 1건 + 지난 7일 내 entries 5건 존재, `briefing.mantra = "오늘도 한 걸음"`, today = `2026-05-14 07:00 KST`.

**When**: `Orchestrator.Run(ctx, userID)` 호출.

**Then**:
- `BriefingPayload.Weather.Current` non-nil 이고 `Status["weather"] == "ok"`
- `BriefingPayload.JournalRecall.Anniversaries` len >= 1, `WeeklyTrend.EntryCount == 5`
- `BriefingPayload.DateCalendar.Today.Year() == 2026 && Month == 5 && Day == 14`
- `BriefingPayload.DateCalendar.DayOfWeekKR == "목요일"`
- `BriefingPayload.Mantra.Text == "오늘도 한 걸음"`
- `Status["mantra"] == "ok"`, `Status["date"] == "ok"`
- 4 module 모두 `Status` map 에 존재

**검증 명령**: `go test -race ./internal/ritual/briefing -run TestOrchestrator_HappyPath -v`

---

### AC-002 — Weather offline fallback

**Given**: WEATHER-001 의 모든 HTTP transport 에러 발생 + offline cache 에 직전 항목 존재, JOURNAL/date/mantra 정상.

**When**: `Orchestrator.Run` 실행.

**Then**:
- `BriefingPayload.Weather.Offline == true`
- `Status["weather"] == "offline"`
- 나머지 3 modules 정상 `ok`
- CLI 렌더 시 "(cached)" 또는 동등 표기

**검증 명령**: `go test ./internal/ritual/briefing -run TestCollectWeather_OfflineFallback -v`

---

### AC-003 — Journal recall integration

**Given**: today = `2026-05-14`, JOURNAL-001 storage 에 1년 전 `2025-05-14` entry (CrisisFlag=false, valence=0.8) + `2024-05-13` entry + 지난 7일 내 entries 4건 존재.

**When**: `CollectJournal` 호출.

**Then**:
- `Anniversaries` len == 2 (1Y + 2Y ±1day window)
- 두 entry 모두 `CrisisFlag == false`
- `WeeklyTrend.EntryCount == 4`
- `WeeklyTrend.SparklinePoints` len == 7

**검증 명령**: `go test ./internal/ritual/briefing -run TestCollectJournal_Anniversary -v`

---

### AC-004 — 24절기 정확도

**Given**: fixture file `testdata/solar_terms_2026.json` 에 8 주요 절기 일자 (입춘 / 우수 / 춘분 / 입하 / 하지 / 입추 / 추분 / 동지).

**When**: `SolarTermOnDate(2026, m, d)` 호출.

**Then**:
- fixture 의 각 일자에서 정확한 절기 이름 반환 (±1일 허용)
- 비-절기 일자에서 nil 반환

**Fixture 출처 권장**:
- 2026 입춘 = 2026-02-04 (한국천문연구원 공시)
- 2026 춘분 = 2026-03-20
- 2026 하지 = 2026-06-21
- 2026 추분 = 2026-09-23
- 2026 동지 = 2026-12-22

**검증 명령**: `go test ./internal/ritual/briefing -run TestSolarTerm_2026Fixtures -v`

---

### AC-005 — 한국 명절 정확도

**Given**: fixture file `testdata/holidays_2026.json` 에 양력으로 환산된 2026 명절 (설 / 정월대보름 / 석가탄신일 / 추석 / 한글날 등).

**When**: `LookupKoreanHoliday(2026, m, d)` 호출.

**Then**:
- fixture 각 일자에서 정확한 명절 이름 반환
- 설(2026 = 2026-02-17, 음력 1월 1일) 적중
- 추석(2026 = 2026-09-25, 음력 8월 15일) 적중
- 비-명절 일자에서 nil

**검증 명령**: `go test ./internal/ritual/briefing -run TestKoreanHoliday_2026Fixtures -v`

---

### AC-006 — Mantra config 적용

**Given**:
- Case (a): `briefing.mantra = "단일 만트라"`
- Case (b): `briefing.mantras = ["월", "화", "수", "목"]`
- ISO week N

**When**: `CollectMantra` 호출.

**Then**:
- Case (a): `MantraModule.Text == "단일 만트라"` regardless of week
- Case (b): `MantraModule.Text == briefing.mantras[N % 4]`
- `MantraModule.Source` 가 어느 config 키에서 왔는지 정확히 표기

**검증 명령**: `go test ./internal/ritual/briefing -run TestCollectMantra_Rotation -v`

---

### AC-007 — Channel 다중 출력

**Given**: `briefing.channels = ["cli", "telegram", "tui"]` + Telegram mock + TUI mock model.

**When**: `Orchestrator.Run` + 3 renderer 모두 호출.

**Then**:
- CLI renderer 의 stdout buffer 에 4 modules 내용 모두 포함
- Telegram mock 의 captured `SendMessageRequest.Text` 가 MarkdownV2 escape 적용된 동일 content 포함
- TUI mock model 의 panel field 가 populated
- 3 channel content 의 **의미적 동일성** 검증 (rendering 만 다름)

**검증 명령**: `go test ./internal/ritual/briefing -run TestRenderer_FanOut -v`

---

### AC-008 — TUI panel 렌더

**Given**: TUI 세션 활성, slash command `/briefing` 호출.

**When**: dispatch 처리 → BriefingPanel render.

**Then**:
- TUI snapshot file `internal/cli/tui/snapshots/briefing_panel.txt` 와 일치
- 4 modules 의 1줄 요약이 panel 에 표시
- ANSI escape 정상 (terminal 호환)

**검증 명령**: `go test ./internal/cli/tui -run TestBriefingPanel_Snapshot -v`

---

### AC-009 — Privacy 6 invariants

**Invariant 1**: 로그 출력에 journal entry 본문 / mantra 본문 / Telegram chat_id raw / weather API key 미포함.
**Invariant 2**: archive 파일 mode `0600`, parent dir `0700`.
**Invariant 3**: A2A 통신 호출 0건 (collector 들은 Go function call only).
**Invariant 4**: clinical/diagnostic vocabulary scanner 가 mantra 와 LLM payload 에서 미발견.
**Invariant 5**: LLM payload (M3) 에 entry text / 정확 좌표 / chat_id 미포함 — categorical 만.
**Invariant 6**: crisis 키워드 검출 시 hotline canned response prepend + 분석 commentary 미포함.

**검증 명령**: `go test ./internal/ritual/briefing -run TestPrivacy_Invariants -v -count=1`

(Invariants 별 sub-test 6 개 분리. JOURNAL-001 의 `internal/ritual/journal/audit_test.go` 패턴 그대로.)

---

### AC-010 — CLI entrypoint

**Given**: built `mink` binary.

**When**: `mink briefing --help` 실행.

**Then**:
- exit code 0
- stdout 에 "Daily morning briefing" 또는 동등 descriptor 포함
- subcommand flags 정상 표기 (`--plain`, `--channels`, `--dry-run`)

**검증 명령**:
```
mink briefing --help | grep -q "briefing"
go test ./internal/cli/commands -run TestBriefingCmd -v
```

---

### AC-011 — SCHEDULER cron 등록

**Given**: SCHEDULER-001 의 `BriefingMorningTime` ritual config = `07:00` + KST timezone.

**When**: Scheduler.Start + clockwork fake clock 07:00 KST 로 advance.

**Then**:
- `BriefingMorningTime` hook event 1회 dispatch
- briefing subscriber 가 `Orchestrator.Run` 호출
- idempotent: 같은 분 내 2회 트리거 시 1회만 실행 (SCHEDULER 의 `firedKeys` 위임)

**검증 명령**:
```
go test ./internal/ritual/scheduler -run TestBriefingMorningEvent -v
go test ./internal/ritual/briefing -run TestCronWiring -v
```

---

### AC-012 — Archive file 생성 + 권한

**Given**: `briefing.archive = true`, today = `2026-05-14`.

**When**: `Orchestrator.Run` 성공 완료.

**Then**:
- 파일 `~/.mink/briefing/2026-05-14.md` 존재
- `stat -f "%Lp"` (mac) 또는 `stat -c "%a"` (linux) == `600`
- parent dir `~/.mink/briefing/` mode `0700`
- 파일 content 는 CLI plain rendering 의 markdown 변환

**검증 명령**: `go test ./internal/ritual/briefing -run TestArchive_FilePerms -v`

---

### AC-013 — Production real collectors wiring (M4, REQ-BR-001/002/064)

**Given**: `internal/cli/commands/briefing.go` 가 cobra root 에 `mink briefing` 명령을 등록한 상태. 4 real collectors (collect_weather.go / collect_journal.go / collect_date.go / collect_mantra.go) 는 `internal/ritual/briefing/` 에 구현 완료. `MockBriefingCollectorFactory` 는 v0.3.0 시점에서 production 소스 (briefing.go) 와 test 코드에 모두 존재.

**Scope of rule**: 본 AC 의 mock-factory 금지 규칙은 단일 파일 `briefing.go` 가 아니라 **`internal/cli/commands/` 디렉토리 내 모든 non-test (`*.go`, except `*_test.go`) 파일** 에 적용된다. 또한 식별자 rename 으로 우회 불가능하도록 **대소문자 무시 정규식 `mock[A-Za-z]*factory`** 로 검증 (예: `mockBriefingCollectorFactory`, `BriefingCollectorFactoryMock`, `MockFactory` 등 모두 차단).

**When**: M4 wiring 적용 후 `mink briefing` 명령을 production binary 로 실행.

**Then**:
- `internal/cli/commands/` 의 모든 non-test 파일에서 `mock*factory` (case-insensitive) symbol 부재
- `internal/cli/commands/briefing.go` 내에 `RealBriefingCollectorFactory` 또는 동등한 real wiring 존재 — `collect_weather.NewCollector` 등 실 패키지 함수 참조
- Mock factory 정의/사용은 `internal/cli/commands/*_test.go` 또는 `internal/ritual/briefing/*_test.go` 에만 존재
- `mink briefing --dry-run` 실행 시 4 module status 가 ok|offline|timeout 중 하나 (skipped 가 아님 — real collector 가 실행됨을 의미)

**검증 명령**:
```bash
# Primary verification (must produce non-zero exit when any mock-factory pattern matches in production code)
grep -iE "mock[A-Za-z]*factory" internal/cli/commands/*.go --exclude="*_test.go" && exit 1 || exit 0
# Secondary functional verification
go test ./internal/cli/commands -run TestBriefingCmd_RealFactory -v
```

---

### AC-014 — Orchestrator → LLM summary wiring (M4, REQ-BR-032/060)

**Given**: M3 의 `GenerateLLMSummary(ctx, provider, payload, cfg, model)` 구현 완료, `BriefingPayload.LLMSummary` 필드 + `cfg.LLMSummary bool` flag 존재. `Orchestrator.Run()` 가 v0.3.0 시점에서 GenerateLLMSummary 를 호출하지 않음.

**When**: M4 wiring 적용 후 다음 세 케이스를 테스트:
- Case A (happy path): `cfg.LLMSummary = true`, mock LLMProvider 가 "오늘은 평온한 하루입니다" 반환하도록 설정, `Orchestrator.Run(ctx, userID)` 호출
- Case B (disabled / nil provider): `cfg.LLMSummary = false` 또는 LLMProvider == nil, `Orchestrator.Run(ctx, userID)` 호출
- Case C (LLM error path): `cfg.LLMSummary = true`, mock LLMProvider 가 timeout / provider error / network error 등 `error != nil` 반환하도록 설정, `Orchestrator.Run(ctx, userID)` 호출

**Then**:
- Case A: `payload.LLMSummary == "오늘은 평온한 하루입니다"`, `Status["llm_summary"] == "ok"`, GenerateLLMSummary 정확히 1회 호출됨
- Case B: `payload.LLMSummary == ""`, `Status["llm_summary"]` 미존재 또는 `"skipped"`, GenerateLLMSummary 미호출, no error returned
- Case C: `payload.LLMSummary == ""`, `Status["llm_summary"] == "error"`, `Orchestrator.Run` 반환 error == nil (파이프라인 실패 없음), 다른 4 모듈 (weather/journal/date/mantra) 의 Status 는 영향받지 않음, 로그에 LLM 요청/응답 payload 내용 미포함 (error category 만 — 예: `error_type=timeout`)
- 어느 케이스도 panic 미발생

**검증 명령**: `go test -race -run TestOrchestrator_LLMSummary ./internal/ritual/briefing/ -v`

(3 sub-test 필수: `TestOrchestrator_LLMSummary_HappyPath` (Case A), `TestOrchestrator_LLMSummary_Disabled` (Case B), `TestOrchestrator_LLMSummary_ErrorPath` (Case C))

---

### AC-015 — Crisis hotline prepend in 3 channels (M4, REQ-BR-055/061)

**Given**: `PrependCrisisResponseIfDetected` + `PayloadHasCrisis` 헬퍼는 `crisis_response.go` 에 구현 완료. v0.3.0 시점에서는 어느 renderer 도 이들을 호출하지 않음. 테스트 fixture 로 mantra 또는 LLMSummary 또는 anniversary text 중 하나에 crisis keyword 가 포함된 BriefingPayload 준비.

**Crisis prepend wiring 의 정확한 3 location**:
1. `internal/ritual/briefing/render_cli.go` — CLI stdout renderer
2. `internal/ritual/briefing/render_telegram.go` — Telegram MarkdownV2 renderer
3. `internal/cli/tui/briefing_panel.go` — TUI BriefingPanel.Render() (별도 패키지 — internal/cli/tui)

CLI/Telegram 의 entrypoint 는 `internal/ritual/briefing` 패키지 내, TUI panel 의 entrypoint 는 `internal/cli/tui` 패키지 내 — 따라서 검증도 두 패키지로 나뉘어 수행.

**When**: M4 wiring 적용 후 3 channel renderer 호출:
- `RenderCLI(payload)` (TTY/plain 모두) — `internal/ritual/briefing` 패키지
- `RenderTelegram(payload)` (MarkdownV2 body 생성 직전) — `internal/ritual/briefing` 패키지
- `BriefingPanel.Render(payload)` — `internal/cli/tui` 패키지

**Then**:
- 모든 3 channel 의 rendered output 의 **첫 줄** (또는 첫 문단) 이 JOURNAL-001 CrisisResponse canned 텍스트 (1577-0199 자살예방상담 / 1393 정신건강상담 / 1388 청소년상담)
- briefing 본문은 hotline 응답 다음에 등장
- 분석 commentary / mood scoring / LLM summary expansion 등 모두 미포함 (REQ-BR-055 의 strict no-commentary 조항 준수)
- `PayloadHasCrisis(payload) == false` 인 경우 동일 renderer 는 hotline 을 prepend 하지 **않음**

**검증 명령** (두 명령 모두 PASS 필수):
```bash
# Coverage 1: CLI + Telegram renderers (internal/ritual/briefing)
go test -race -run TestRenderers_CrisisPrepend ./internal/ritual/briefing/ -v
# Coverage 2: TUI BriefingPanel renderer (internal/cli/tui)
go test -race -run TestBriefingPanel_CrisisPrepend ./internal/cli/tui/ -v
```

(`TestRenderers_CrisisPrepend` 하위 2 sub-test: TestRenderCLI_CrisisPrepend, TestRenderTelegram_CrisisPrepend. `TestBriefingPanel_CrisisPrepend` 는 TUI panel crisis-prepend 검증 전용 — TUI 패키지는 별도 test file 필요.)

---

### AC-016 — /briefing TUI slash dispatch (M4, REQ-BR-033/062)

**Given**: `internal/cli/tui/slash.go` 의 `HandleSlashCmd` switch 에 `case "briefing":` 부재. `BriefingPanel.Render()` 는 v0.3.0 에서 snapshot test 만 됨. tea.Model 의 messages slice 및 system role 메시지 append 패턴은 기존 다른 slash command (`/help`, `/journal` 등) 와 동일.

**When**: M4 wiring 적용 후 TUI session 활성 상태에서 `/briefing` 입력. 내부적으로:
- `HandleSlashCmd("briefing")` 호출 → `briefingRunCmd` tea.Cmd 반환
- tea.Cmd 비동기 실행 → `Orchestrator.Run(ctx, userID)` → `BriefingResultMsg{Payload: ...}` 발행
- tea.Model `Update(BriefingResultMsg)` → `BriefingPanel.Render(payload)` → `m.messages` 에 system role 메시지로 append

**Then**:
- `HandleSlashCmd` 가 `tea.Cmd` non-nil 반환 (블로킹 0)
- `BriefingResultMsg` tea.Msg 타입이 `internal/cli/tui` 에 정의됨
- Update() 처리 후 `m.messages` 의 마지막 element 가 system role + BriefingPanel rendering 내용 포함
- TUI event loop blocking 0 (async 보장)
- snapshot test 가 panel rendering 의 결정성 검증

**검증 명령**: `go test -race -run TestTUI_BriefingSlash ./internal/cli/tui/ -v`

(sub-test: TestSlash_BriefingDispatch, TestModel_BriefingResultMsg_Append, TestModel_BriefingSlash_NonBlocking)

---

### AC-017 — Deterministic Module Status order (M4, REQ-BR-063)

**Given**: `orchestrator.go` 의 `BriefingPayload.Status` 는 `map[string]string`. `render_cli.go` 가 이 map 을 순회하며 "Module Status:" 섹션 행을 출력. Go map iteration 비결정성으로 인해 golden test `testdata/golden_cli_render.txt` 가 실행마다 Mantra 행 위치 변동 → flaky. v0.3.0 의 working tree 에 정확히 이 증상의 uncommitted diff 가 존재 (`internal/ritual/briefing/testdata/golden_cli_render.txt`).

**Why two-part verification (`-count=10` 한계)**: T-309 적용 후 map iteration 이 fixed slice 순회로 바뀌면 `-count=10` 반복 실행해도 항상 동일 출력이라 비결정성 검출 가치가 사라진다. 진짜 회귀 리스크는 **미래 refactor 가 fixed slice 를 다시 map iteration 으로 되돌리는 것**. 따라서 golden test (출력 정확성) 와 별도로 **structural guard test** 를 추가해 source 코드 구조 자체를 보호한다.

**M4 implementation 패턴 (Option A 채택)**:
- `internal/ritual/briefing/render_cli.go` 내에 exported package-level 상수 신설: `var ModuleStatusOrder = []string{"weather", "journal", "date", "mantra"}`
- RenderCLI / RenderCLIPlain 의 Module Status 섹션 출력 시 `for _, key := range ModuleStatusOrder { ... }` 패턴 사용 (map iteration 직접 금지)
- 구조적 guard test 가 (1) 슬라이스 contents 일치, (2) render_cli.go 가 ModuleStatusOrder 참조함을 모두 검증

**When**: M4 wiring 적용 후 두 가지 검증 트랙 실행:
- Track 1 (correctness): `RenderCLI(payload)` 호출 → golden file diff 0
- Track 2 (structural guard): `TestRenderCLI_StatusOrder_Fixed` 실행 → ModuleStatusOrder 상수 값과 render_cli.go 의 참조 유무 검증

**Then**:
- Track 1: golden file `testdata/golden_cli_render.txt` 가 Weather → Journal → Date → Mantra fixed order 로 갱신, RenderCLI 출력과 byte-for-byte 일치
- Track 2: (a) `ModuleStatusOrder` 가 정확히 `[]string{"weather","journal","date","mantra"}` 와 일치 (slice deep equal), (b) `render_cli.go` source 가 `ModuleStatusOrder` 식별자를 참조 (Go source grep 또는 ast.Walk 로 확인), (c) `render_cli.go` source 에 `range payload.Status` 패턴 부재 (map iteration 금지)
- 빠진 module (status 미존재) 은 "(skipped)" 등 명시적 표기

**검증 명령** (두 명령 모두 PASS 필수):
```bash
# Track 1: correctness via golden
go test -race -run TestRenderCLI_Golden ./internal/ritual/briefing/ -count=1 -v
# Track 2: structural guard against future regression
go test -race -run TestRenderCLI_StatusOrder_Fixed ./internal/ritual/briefing/ -v
```

(`TestRenderCLI_StatusOrder_Fixed` 는 다음 3 assertion 을 모두 검증: slice equality + render_cli.go 의 ModuleStatusOrder 참조 + render_cli.go 의 `range payload.Status` pattern 부재. 미래 refactor 가 fixed slice 를 다시 map iteration 으로 되돌리면 본 test 가 실패하여 회귀 차단.)

---

## 3. Edge Cases (EC)

### EC-001 — Config malformed

**Given**: `briefing.channels = "cli"` (string instead of array) — 타입 mismatch.

**When**: config load.

**Then**: 명시적 에러 (`invalid briefing.channels: expected array, got string`) + 파이프라인 시작 거부.

**검증 명령**: `go test ./internal/ritual/briefing -run TestConfig_Malformed -v`

---

### EC-002 — All modules failed

**Given**: WEATHER timeout + JOURNAL nil error + date module out-of-range (year 1899) + mantra config empty.

**When**: `Orchestrator.Run`.

**Then**:
- `Status` 모두 `error` 또는 `timeout` 또는 `skipped`
- output 에 "briefing unavailable" + 4 module status 명시
- exit code 0 (degraded output 도 정상 종결)

**검증 명령**: `go test ./internal/ritual/briefing -run TestOrchestrator_AllFail -v`

---

### EC-003 — Clock skew (1899 or 2101)

**Given**: system clock 강제 설정 (test fake clock).

**When**: `CollectDate` 호출.

**Then**:
- `DateModule.Today` 는 그대로 set
- `DateModule.SolarTerm == nil` + status flag `out-of-range`
- `DateModule.Holiday == nil` + status flag `out-of-range`
- briefing 본문에 "(절기/명절 계산 범위 밖)" 표기

**검증 명령**: `go test ./internal/ritual/briefing -run TestDateModule_OutOfRange -v`

---

### EC-004 — Telegram token absent

**Given**: `MINK_TELEGRAM_TOKEN` env var unset, `briefing.channels = ["cli", "telegram"]`.

**When**: orchestrator + renderer fan-out.

**Then**:
- CLI 정상 rendering
- Telegram renderer 가 token 결락 감지 → warning log (chat_id 미노출) + channel disable
- 전체 briefing 종결 코드 0
- `Status["telegram"] == "disabled"`

**검증 명령**: `go test ./internal/ritual/briefing -run TestTelegram_TokenMissing -v`

---

## 4. Definition of Done (DoD)

본 SPEC 의 M1 / M2 / M3 milestone 별 DoD.

### M1 DoD
- [ ] AC-001 ~ AC-006 + AC-009 + AC-010 + EC-001 + EC-002 + EC-003 모두 GREEN
- [ ] `go test -race ./internal/ritual/briefing -count=1` 통과
- [ ] `go vet ./internal/ritual/briefing` 통과
- [ ] `gofmt -l internal/ritual/briefing` 빈 출력
- [ ] `golangci-lint run ./internal/ritual/briefing` 통과
- [ ] `internal/ritual/briefing` 패키지 coverage >= 80%
- [ ] `mink briefing` 명령 manual smoke 통과

### M2 DoD
- [ ] M1 모든 항목 + AC-007 + AC-008 + AC-011 + AC-012 + EC-004 GREEN
- [ ] SCHEDULER-001 의 `RegisteredEvents()` 에 `BriefingMorningTime` 1 entry 추가 확인
- [ ] Telegram smoke (real bot, 별도 fixture) — `.moai/specs/SPEC-MINK-BRIEFING-001/manual_smoke.md` 절차
- [ ] TUI snapshot golden 확정
- [ ] archive file 모드 0600 / dir 0700 일관성 (`go test` + 실 파일 시스템)

### M3 DoD (Optional)
- [ ] M2 모든 항목
- [ ] LLM payload minimization 단위 테스트 (categorical only)
- [ ] crisis hotline 단위 테스트
- [ ] LLM provider abstraction 호환성 확인

### M4 DoD (v0.3.1 wiring)
- [ ] M3 모든 항목 + AC-013 + AC-014 + AC-015 + AC-016 + AC-017 모두 GREEN
- [ ] `grep -iE "mock[A-Za-z]*factory" internal/cli/commands/*.go --exclude="*_test.go"` no match (case-insensitive, all non-test files; production path clean against any rename bypass)
- [ ] `go test -race -count=1 ./internal/ritual/briefing/ -run TestRenderCLI_Golden` PASS (golden file 정확성 검증)
- [ ] `go test -race -count=1 ./internal/ritual/briefing/ -run TestRenderCLI_StatusOrder_Fixed` PASS (structural guard — ModuleStatusOrder slice equality + render_cli.go 의 ModuleStatusOrder 참조 + render_cli.go 의 `range payload.Status` pattern 부재)
- [ ] `go test -race ./internal/ritual/briefing/ -run TestOrchestrator_LLMSummary` PASS (3 sub-test: TestOrchestrator_LLMSummary_HappyPath + TestOrchestrator_LLMSummary_Disabled + TestOrchestrator_LLMSummary_ErrorPath)
- [ ] `go test -race ./internal/ritual/briefing/ -run TestRenderers_CrisisPrepend` PASS (CLI + Telegram 2 sub-test, briefing 패키지)
- [ ] `go test -race ./internal/cli/tui/ -run TestBriefingPanel_CrisisPrepend` PASS (TUI panel renderer, internal/cli/tui 패키지 별도 검증)
- [ ] `go test -race ./internal/cli/tui/ -run TestTUI_BriefingSlash` PASS (slash dispatch + BriefingResultMsg + non-blocking)
- [ ] `internal/ritual/briefing` 패키지 coverage >= 88% (v0.3.0 85.5% + 신규 wiring 테스트로 자연 증가 목표)
- [ ] manual smoke: `mink briefing` 명령으로 real collectors (network 실호출 fallback 가능) 4 module 결과 출력 확인
- [ ] manual smoke: TUI session 에서 `/briefing` 입력 → panel 정상 표시
- [ ] `testdata/golden_cli_render.txt` 갱신본 commit (Weather/Journal/Date/Mantra 고정 순서)

---

## 5. Quality Gates 요약

| Gate | M1 | M2 | M3 | M4 (v0.3.1) |
|------|----|----|----|----|
| Unit + integration tests | All AC GREEN | All AC GREEN | All AC GREEN | All AC GREEN incl AC-013~017 |
| Race detector | clean | clean | clean | clean (golden test `-count=1` + structural guard test `TestRenderCLI_StatusOrder_Fixed` PASS) |
| go vet | clean | clean | clean | clean |
| gofmt | clean | clean | clean | clean |
| golangci-lint | clean | clean | clean | clean |
| Coverage (briefing pkg) | >= 80% | >= 85% | >= 80% | >= 88% |
| Strict coverage (new package) | >= 90% | >= 90% | >= 85% | >= 88% |
| External Go dep added | 0 | 0 | possibly 1 (LLM provider) | 0 |
| Privacy invariants | 6/6 | 6/6 | 6/6 | 6/6 |
| Manual smoke (telegram) | n/a | required | required | required |
| Manual smoke (TUI `/briefing`) | n/a | n/a | n/a | required |
| Manual smoke (CLI real collectors) | n/a | n/a | n/a | required |
| Golden file stability (10 runs) | n/a | n/a | n/a | required |

---

Version: 0.3.1
Updated: 2026-05-15

v0.3.0 종결물 (AC-001~012 + EC-001~004) 은 v0.3.1 amendment 에서 변경 0. M4 는 5 wiring AC (AC-013~017) 만 추가.

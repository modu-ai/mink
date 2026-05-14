---
id: SPEC-MINK-BRIEFING-001
version: 0.3.1
status: implemented
created_at: 2026-05-14
updated_at: 2026-05-15
author: manager-spec
---

# Tasks — SPEC-MINK-BRIEFING-001

본 문서는 implementation 단계의 atomic task 분해 + traceability matrix (REQ → task → AC) 를 정의한다.

## 1. Task 식별 규칙

- Task ID: `T-NNN` (000 미사용, M1 = 001~099, M2 = 100~199, M3 = 200~299, M4 = 300~399)
- 각 task 는 1~2 PR 또는 1~3 commit 으로 완결되어야 함 (atomic decomposition)
- 의존 task 는 `depends_on:` 필드로 명시
- traceability matrix 는 §3 참조

## 2. Task 본문

### Milestone M1 — Core Collector + CLI render

#### T-001 — Package skeleton + types
- 산출물: `internal/ritual/briefing/{doc.go, types.go, config.go}`
- 내용: package 주석 + `BriefingPayload`/`*Module` struct 정의 + `Config` struct + `Validate()` + DefaultConfig.
- 의존: none
- 검증: `go build ./internal/ritual/briefing`, `go vet`
- 매핑 REQ: REQ-BR-001, REQ-BR-040

#### T-002 — 24절기 계산 (solarterm)
- 산출물: `solarterm.go`, `solarterm_data.go`, `solarterm_test.go`, `testdata/solar_terms_2026.json`
- 내용: Meeus 황경 기반 + fixture lookup. `SolarTermOnDate(year, month, day)` 반환. 범위 1900~2100.
- 의존: T-001
- 검증: `go test ./internal/ritual/briefing -run TestSolarTerm` PASS
- 매핑 REQ: REQ-BR-005, REQ-BR-042
- 매핑 AC: AC-004

#### T-003 — 한국 명절 lookup (holiday)
- 산출물: `holiday.go`, `holiday_data.go`, `holiday_test.go`, `testdata/holidays_2026.json`
- 내용: KASI 음력→양력 변환 결과 hardcode (1900~2100, 설/정월대보름/석가탄신일/추석/한글날 등). `LookupKoreanHoliday(year, month, day)` 반환.
- 의존: T-001
- 검증: `go test ./internal/ritual/briefing -run TestKoreanHoliday` PASS
- 매핑 REQ: REQ-BR-005, REQ-BR-042
- 매핑 AC: AC-005

#### T-004 — Weather collector
- 산출물: `collect_weather.go`, `collect_weather_test.go`
- 내용: `tools.Registry.Invoke("weather_current", ...)` 외 2 호출 → `WeatherModule` 합성. 모든 에러는 offline cache fallback 시도 + `Offline=true` 표기.
- 의존: T-001, WEATHER-001 (already completed)
- 검증: `go test ./internal/ritual/briefing -run TestCollectWeather` PASS
- 매핑 REQ: REQ-BR-003, REQ-BR-020
- 매핑 AC: AC-002

#### T-005 — Journal collector
- 산출물: `collect_journal.go`, `collect_journal_test.go`
- 내용: `MemoryRecall.FindAnniversaryEvents` (today 기준 1Y/3Y/7Y) + `TrendAggregator.WeeklyTrend` 호출. crisis_flag entries 는 JOURNAL-001 의 trauma recall protection 위임 (filter 자동).
- 의존: T-001, JOURNAL-001 (already completed)
- 검증: `go test ./internal/ritual/briefing -run TestCollectJournal` PASS
- 매핑 REQ: REQ-BR-004, REQ-BR-021, REQ-BR-043
- 매핑 AC: AC-003

#### T-006 — Date collector
- 산출물: `collect_date.go`, `collect_date_test.go`
- 내용: today + 요일 KR (한국어 "월요일"~"일요일") + solarterm + holiday 통합. clock skew 시 `out-of-range` flag.
- 의존: T-001, T-002, T-003
- 검증: `go test ./internal/ritual/briefing -run TestCollectDate` + `TestDateModule_OutOfRange` PASS
- 매핑 REQ: REQ-BR-005, REQ-BR-042
- 매핑 AC: AC-004, AC-005, EC-003

#### T-007 — Mantra collector
- 산출물: `collect_mantra.go`, `collect_mantra_test.go`
- 내용: config 의 `briefing.mantra` (단일) 또는 `briefing.mantras` (rotation) 읽기 + ISO week mod len rotation. clinical vocabulary scanner 경고.
- 의존: T-001
- 검증: `go test ./internal/ritual/briefing -run TestCollectMantra` PASS
- 매핑 REQ: REQ-BR-031, REQ-BR-053
- 매핑 AC: AC-006

#### T-008 — Orchestrator
- 산출물: `orchestrator.go`, `orchestrator_test.go`
- 내용: 4 collector parallel (errgroup) + 30s per-module timeout + status map + `BriefingPayload` 합성. 모든 module 실패 시 minimal payload 생성.
- 의존: T-004, T-005, T-006, T-007
- 검증: `go test -race ./internal/ritual/briefing -run TestOrchestrator` PASS
- 매핑 REQ: REQ-BR-001, REQ-BR-023, REQ-BR-041
- 매핑 AC: AC-001, EC-002

#### T-009 — CLI renderer (ANSI + plain)
- 산출물: `render_cli.go`, `render_cli_test.go`, `testdata/golden_cli_render.txt`
- 내용: TTY 감지 → ANSI color + 이모지; non-TTY 또는 `--plain` → 평문. golden test 기반.
- 의존: T-008
- 검증: `go test ./internal/ritual/briefing -run TestRenderCLI` PASS
- 매핑 REQ: REQ-BR-002, REQ-BR-006
- 매핑 AC: AC-001 (consumed)

#### T-010 — Cobra `mink briefing` command
- 산출물: `internal/cli/commands/briefing.go`, `internal/cli/commands/briefing_test.go`
- 내용: cobra subcommand under `mink` root, flags = `--plain`, `--channels`, `--dry-run`. `mink briefing` invocation → `Orchestrator.Run` + CLI renderer.
- 의존: T-008, T-009
- 검증: `go test ./internal/cli/commands -run TestBriefingCmd` + manual `mink briefing --help`
- 매핑 REQ: REQ-BR-011
- 매핑 AC: AC-010

#### T-011 — Audit logger redaction
- 산출물: `audit.go`, `audit_test.go`
- 내용: zap logger wrapper — entry text / mantra / chat_id / API key 자동 redact. 허용 field: `module`, `status`, `duration_ms`, `error_type` (text 제외).
- 의존: T-008
- 검증: `go test ./internal/ritual/briefing -run TestAudit_Redaction` PASS
- 매핑 REQ: REQ-BR-050
- 매핑 AC: AC-009 (invariant 1)

#### T-012 — M1 integration test
- 산출물: `orchestrator_integration_test.go`
- 내용: WEATHER-001 mock + JOURNAL-001 inmem storage 로 end-to-end happy path + offline path.
- 의존: T-001 ~ T-011
- 검증: AC-001 + AC-002 GREEN
- 매핑 AC: AC-001, AC-002

#### T-013 — M1 privacy invariants sub-suite
- 산출물: `privacy_test.go` (M1 부분 — invariants 1, 3, 4 우선)
- 내용: log 검사 + A2A 호출 0 검증 + clinical vocab scanner test
- 의존: T-011, T-008
- 검증: `go test ./internal/ritual/briefing -run TestPrivacy_Invariants` 일부 PASS (M1 범위)
- 매핑 REQ: REQ-BR-050, REQ-BR-052, REQ-BR-053
- 매핑 AC: AC-009 (partial)

### Milestone M2 — Multi-channel + cron + archive

#### T-101 — Telegram renderer
- 산출물: `render_telegram.go`, `render_telegram_test.go`, `testdata/golden_telegram.md`
- 내용: BriefingPayload → MarkdownV2 escape + `SendMessageRequest` 생성. `Sender.Send` 호출 (mock 가능).
- 의존: T-008, MSG-TELEGRAM-001
- 검증: `go test ./internal/ritual/briefing -run TestRenderTelegram` PASS + golden diff 0
- 매핑 REQ: REQ-BR-002
- 매핑 AC: AC-007

#### T-102 — TUI panel + snapshot
- 산출물: `render_tui.go`, `render_tui_test.go`, `internal/cli/tui/sessionmenu/briefing_panel.go`, `internal/cli/tui/snapshots/briefing_panel.txt`
- 내용: bubbletea 모델 panel + `/briefing` slash dispatch.
- 의존: T-008
- 검증: `go test ./internal/cli/tui -run TestBriefingPanel_Snapshot` PASS
- 매핑 REQ: REQ-BR-002, REQ-BR-033
- 매핑 AC: AC-008

#### T-103 — Archive writer
- 산출물: `archive.go`, `archive_test.go`
- 내용: `~/.mink/briefing/YYYY-MM-DD.md` mkdir 0700 + write 0600. content = CLI plain rendering 의 markdown 변환.
- 의존: T-009
- 검증: `go test ./internal/ritual/briefing -run TestArchive_FilePerms` PASS (file mode 0600)
- 매핑 REQ: REQ-BR-012, REQ-BR-030, REQ-BR-051
- 매핑 AC: AC-012

#### T-104 — SCHEDULER cron 등록
- 산출물: `internal/ritual/scheduler/events.go` 수정 (`BriefingMorningTime` hook event 등록) + `internal/ritual/briefing/cron.go` (subscriber wiring) + 테스트
- 내용: SCHEDULER 의 `RegisteredEvents()` 에 entry 1 개 추가 + ritual config schema 확장 (briefing 시간 키)
- 의존: T-008, SCHEDULER-001
- 검증: `go test ./internal/ritual/scheduler -run TestBriefingMorningEvent` + `go test ./internal/ritual/briefing -run TestCronWiring` PASS
- 매핑 REQ: REQ-BR-010
- 매핑 AC: AC-011

#### T-105 — Telegram graceful disable
- 산출물: `render_telegram.go` 보강 + `render_telegram_test.go` 시나리오 추가
- 내용: token absent / chat_id invalid → channel disable + warning log (chat_id 미노출).
- 의존: T-101
- 검증: `go test ./internal/ritual/briefing -run TestTelegram_TokenMissing` PASS
- 매핑 REQ: REQ-BR-022
- 매핑 AC: EC-004

#### T-106 — M2 channel fan-out integration
- 산출물: `fanout_integration_test.go`
- 내용: 3 channels (cli + telegram mock + tui mock) 동시 활성 시 content 의미적 동일 검증.
- 의존: T-101, T-102, T-009
- 검증: AC-007 GREEN
- 매핑 AC: AC-007

#### T-107 — M2 privacy invariants 보강
- 산출물: `privacy_test.go` (M2 부분 — invariants 2, 6)
- 내용: archive 파일 mode 검증 + crisis hotline canned response 검증
- 의존: T-103, T-013
- 검증: AC-009 GREEN (M1+M2 합)
- 매핑 REQ: REQ-BR-051, REQ-BR-055
- 매핑 AC: AC-009 (complete)

### Milestone M3 — LLM summary (Optional)

#### T-201 — LLM provider abstraction
- 산출물: `llm_summary.go`, `llm_summary_test.go`
- 내용: 기존 LLM provider 인터페이스 활용 + categorical payload only.
- 의존: T-008
- 검증: payload 검사 test PASS (entry text / coords / chat_id 미포함)
- 매핑 REQ: REQ-BR-032, REQ-BR-054
- 매핑 AC: (M3 — DoD)

#### T-202 — Crisis hotline canned response
- 산출물: `crisis_response.go`, `crisis_response_test.go`
- 내용: JOURNAL-001 의 crisis pattern 재사용 + briefing 본문 prepend.
- 의존: T-201, JOURNAL-001 `crisis.go`
- 검증: crisis fixture → output 의 first line 이 hotline canned
- 매핑 REQ: REQ-BR-055
- 매핑 AC: AC-009 invariant 6

### Milestone M4 — Full wiring (v0.3.1 amendment)

v0.3.0 종결 시점에서 식별된 5 wiring gap 을 닫는다. 신규 컴포넌트 구현은 0; 모두 **이미 존재하는 구현을 production path 에 연결** 하는 작업.

#### T-301 — Mock factory을 test 파일로 이전
- 산출물: `internal/cli/commands/briefing.go` (Mock 제거) + `internal/cli/commands/briefing_test.go` (Mock 이전)
- 내용: v0.3.0 시점에서 `MockBriefingCollectorFactory` 는 production 소스인 `briefing.go` 에도 존재 — 이를 `*_test.go` 로 이전. production 컴파일 단위에서 mock symbol 0 보장.
- 의존: none (T-010 production wiring 의 후처리)
- 검증: `grep -n MockBriefingCollectorFactory internal/cli/commands/briefing.go` no match
- 매핑 REQ: REQ-BR-064
- 매핑 AC: AC-013

#### T-302 — Real collector factory wiring in cobra command
- 산출물: `internal/cli/commands/briefing.go` 의 cobra `RunE` 핸들러 수정
- 내용: `RealBriefingCollectorFactory` (또는 동등 함수) 신설 — `collect_weather.NewCollector` / `collect_journal.NewCollector` / `collect_date.NewCollector` / `collect_mantra.NewCollector` 어댑터를 인스턴스화 → Orchestrator 에 주입. production binary 의 `mink briefing` invocation 이 real collectors 를 사용하도록 보장.
- 의존: T-301
- 검증: `go test ./internal/cli/commands -run TestBriefingCmd_RealFactory` PASS + manual smoke `mink briefing --dry-run` 4 module 결과 출력
- 매핑 REQ: REQ-BR-001, REQ-BR-002, REQ-BR-064
- 매핑 AC: AC-013

#### T-303 — LLMProvider injection point in Orchestrator
- 산출물: `internal/ritual/briefing/orchestrator.go` (`OrchestratorOption` 또는 constructor variant) + `internal/cli/commands/briefing.go` 의 LLMProvider 주입 wiring
- 내용: Orchestrator 에 LLMProvider 를 optional 로 주입할 수 있는 패턴 추가 (Functional Options 권장: `WithLLMProvider(p) Option`). cobra command 측에서 cfg.LLMSummary=true 일 때만 provider 를 wiring.
- 의존: T-008 (existing), T-201 (existing GenerateLLMSummary)
- 검증: `go build ./...` PASS + Option API 단위 테스트
- 매핑 REQ: REQ-BR-032, REQ-BR-060
- 매핑 AC: AC-014 (구성 요소)

#### T-304 — Orchestrator.Run() invokes GenerateLLMSummary
- 산출물: `internal/ritual/briefing/orchestrator.go` (4 collector 단계 직후 LLM summary 단계 추가) + `internal/ritual/briefing/orchestrator_test.go` (LLM summary 테스트 추가)
- 내용: 4 collector 완료 후 `cfg.LLMSummary && provider != nil` 조건 만족 시 `GenerateLLMSummary(ctx, provider, payload, cfg, model)` 호출, 결과를 `payload.LLMSummary` 에 set. **Error path 처리 (REQ-BR-060 확장)**: GenerateLLMSummary 가 non-nil error 반환 시 → `payload.LLMSummary = ""`, `payload.Status["llm_summary"] = "error"`, 로그에는 error category (timeout / provider_error / network_error) 만 기록 (payload 내용 미포함), `Orchestrator.Run` 자체는 error nil 반환 (다른 4 module 영향 0, 파이프라인 graceful 종료).
- 의존: T-303, T-201 (existing)
- 검증: `go test -race -run TestOrchestrator_LLMSummary ./internal/ritual/briefing/` PASS (필수 3 sub-test: HappyPath / Disabled / ErrorPath — TestOrchestrator_LLMSummary_ErrorPath 가 Case C 검증)
- 매핑 REQ: REQ-BR-032, REQ-BR-060
- 매핑 AC: AC-014 (Case A + Case B + Case C)

#### T-305 — Crisis hotline prepend in 3 channel renderers
- 산출물 (정확히 3 location 의 production code 수정 + 2 test file):
  - `internal/ritual/briefing/render_cli.go` — RenderCLI entrypoint 에 crisis check + prepend (briefing 패키지)
  - `internal/ritual/briefing/render_telegram.go` — RenderTelegram entrypoint 에 crisis check + prepend (briefing 패키지)
  - `internal/cli/tui/briefing_panel.go` — `BriefingPanel.Render()` 에 crisis check + prepend (별도 패키지 internal/cli/tui)
  - `internal/ritual/briefing/render_crisis_test.go` — CLI + Telegram 2 sub-test 수록 (briefing 패키지)
  - `internal/cli/tui/briefing_panel_crisis_test.go` — TUI panel crisis prepend 1 test 수록 (internal/cli/tui 패키지, 별도 test file)
- 내용: 각 renderer 의 진입점에서 `briefing.PayloadHasCrisis(payload)` 체크 → true 시 `briefing.PrependCrisisResponseIfDetected(rendered)` 적용. 3 location 모두 동일 패턴 (helper 공유). false 시 변경 없음. TUI panel 은 별도 패키지이므로 briefing 패키지 helper 를 import 해 호출.
- 의존: T-202 (existing crisis_response.go in briefing pkg), T-009/T-101/T-102 (existing renderers)
- 검증 (두 명령 모두 PASS 필요):
  - `go test -race -run TestRenderers_CrisisPrepend ./internal/ritual/briefing/` PASS (CLI + Telegram 2 sub-test)
  - `go test -race -run TestBriefingPanel_CrisisPrepend ./internal/cli/tui/` PASS (TUI panel renderer)
- 매핑 REQ: REQ-BR-055, REQ-BR-061
- 매핑 AC: AC-015

#### T-306 — BriefingResultMsg + briefingRunCmd tea pattern
- 산출물: `internal/cli/tui/briefing_msg.go` (또는 `messages.go` 확장) + 동일 디렉토리 테스트 파일
- 내용: `BriefingResultMsg{Payload *briefing.BriefingPayload; Err error}` tea.Msg 타입 + `briefingRunCmd(ctx, userID) tea.Cmd` 헬퍼. tea.Cmd 는 비동기로 `Orchestrator.Run` 호출 후 BriefingResultMsg 반환.
- 의존: T-304 (Orchestrator wiring 완료된 상태)
- 검증: tea.Cmd 단위 테스트 — channel-based async 동작 확인
- 매핑 REQ: REQ-BR-033, REQ-BR-062
- 매핑 AC: AC-016 (구성 요소)

#### T-307 — `/briefing` slash command handler in slash.go
- 산출물: `internal/cli/tui/slash.go` 수정 (HandleSlashCmd switch 에 `case "briefing":` 추가) + `internal/cli/tui/slash_test.go` 시나리오 추가
- 내용: `/briefing` 명령 입력 시 `briefingRunCmd(ctx, m.userID)` 반환. block 0 (async 보장). help 텍스트에도 추가.
- 의존: T-306
- 검증: `go test -race -run TestSlash_BriefingDispatch ./internal/cli/tui/` PASS
- 매핑 REQ: REQ-BR-033, REQ-BR-062
- 매핑 AC: AC-016

#### T-308 — tea.Model Update() handles BriefingResultMsg
- 산출물: `internal/cli/tui/model.go` (또는 `update.go`) 의 `Update()` switch 확장 + 테스트 추가
- 내용: `case BriefingResultMsg:` 에서 `BriefingPanel.Render(msg.Payload)` → `m.messages` 에 system role 메시지로 append. Err non-nil 시 error message append. 본 처리 후 view re-render 트리거.
- 의존: T-306, T-102 (existing BriefingPanel)
- 검증: `go test -race -run TestModel_BriefingResultMsg_Append ./internal/cli/tui/` PASS
- 매핑 REQ: REQ-BR-033, REQ-BR-062
- 매핑 AC: AC-016

#### T-309 — render_cli.go Module Status 고정 순서 (Option A 채택)
- 산출물:
  - `internal/ritual/briefing/render_cli.go` 수정 — exported `var ModuleStatusOrder = []string{"weather", "journal", "date", "mantra"}` 선언 + Module Status 출력 루프를 `for _, key := range ModuleStatusOrder` 패턴으로 교체 (map iteration 직접 사용 금지)
  - `internal/ritual/briefing/testdata/golden_cli_render.txt` 갱신 (Weather → Journal → Date → Mantra fixed order)
  - `internal/ritual/briefing/render_cli_order_test.go` 신설 — `TestRenderCLI_StatusOrder_Fixed` 구조적 guard test 수록
- 내용 (Option A 선택 이유): C4 권고 중 Option A (exported slice + 참조 강제) 를 채택. AST walk (Option B) 는 가능하나 grep 기반 검증이 더 간결하고 미래 유지보수성이 높음. 구조적 guard test 는 다음 3 assertion 을 검증:
  - (1) `ModuleStatusOrder` 가 정확히 `[]string{"weather","journal","date","mantra"}` 와 deep-equal
  - (2) `render_cli.go` source content 에 `ModuleStatusOrder` 식별자 참조 존재 (os.ReadFile + strings.Contains)
  - (3) `render_cli.go` source content 에 `range payload.Status` pattern 부재 (map iteration 금지 강제)
- 각 module key 에 대해 Status[key] 값 출력 (없으면 "(skipped)"). Mantra 행이 항상 마지막.
- 의존: T-009 (existing)
- 검증 (두 명령 모두 PASS):
  - `go test -race -count=1 -run TestRenderCLI_Golden ./internal/ritual/briefing/` (golden 정확성)
  - `go test -race -count=1 -run TestRenderCLI_StatusOrder_Fixed ./internal/ritual/briefing/` (structural guard)
- 매핑 REQ: REQ-BR-063
- 매핑 AC: AC-017

#### T-310 — M4 integration test suite + AC-013~017 골든 통합
- 산출물:
  - `internal/cli/commands/briefing_test.go` (T-301/T-302 검증 확장)
  - `internal/ritual/briefing/orchestrator_llm_test.go` (T-304 검증)
  - `internal/ritual/briefing/render_crisis_test.go` (T-305 검증)
  - `internal/cli/tui/briefing_slash_test.go` (T-306/T-307/T-308 검증)
  - `internal/ritual/briefing/render_golden_test.go` (golden 정확성) + `internal/ritual/briefing/render_cli_order_test.go` (T-309 structural guard, Option A 검증)
- 내용: AC-013/014/015/016/017 각각 1개 이상 sub-test, 합산 coverage delta ≥ +2.5% (v0.3.0 85.5% → 88% 목표).
- 의존: T-301 ~ T-309
- 검증: 본 SPEC 의 모든 AC (12+5) + EC (4) 가 `go test -race -count=1 ./...` 에서 GREEN
- 매핑 REQ: REQ-BR-060, REQ-BR-061, REQ-BR-062, REQ-BR-063, REQ-BR-064
- 매핑 AC: AC-013, AC-014, AC-015, AC-016, AC-017

## 3. Traceability Matrix (REQ → Task → AC)

| REQ ID | Task ID(s) | AC ID(s) |
|--------|-----------|---------|
| REQ-BR-001 | T-001, T-008 | AC-001 |
| REQ-BR-002 | T-009, T-101, T-102 | AC-007 |
| REQ-BR-003 | T-004 | AC-002 |
| REQ-BR-004 | T-005 | AC-003 |
| REQ-BR-005 | T-002, T-003, T-006 | AC-004, AC-005 |
| REQ-BR-006 | T-009 | AC-001 |
| REQ-BR-010 | T-104 | AC-011 |
| REQ-BR-011 | T-010 | AC-010 |
| REQ-BR-012 | T-103 | AC-012 |
| REQ-BR-020 | T-004 | AC-002 |
| REQ-BR-021 | T-005 | AC-003 |
| REQ-BR-022 | T-105 | EC-004 |
| REQ-BR-023 | T-008 | AC-001, EC-002 |
| REQ-BR-030 | T-103 | AC-012 |
| REQ-BR-031 | T-007 | AC-006 |
| REQ-BR-032 | T-201 | M3 DoD |
| REQ-BR-033 | T-102 | AC-008 |
| REQ-BR-040 | T-001 | EC-001 |
| REQ-BR-041 | T-008 | EC-002 |
| REQ-BR-042 | T-002, T-003, T-006 | AC-004, AC-005, EC-003 |
| REQ-BR-043 | T-005 | AC-003 |
| REQ-BR-050 | T-011, T-013 | AC-009 |
| REQ-BR-051 | T-103, T-107 | AC-012, AC-009 |
| REQ-BR-052 | T-013 | AC-009 |
| REQ-BR-053 | T-007, T-013 | AC-009 |
| REQ-BR-054 | T-201 | AC-009 (M3) |
| REQ-BR-055 | T-202, T-305 | AC-009 (M3), AC-015 |
| REQ-BR-060 | T-303, T-304 | AC-014 |
| REQ-BR-061 | T-305 | AC-015 |
| REQ-BR-062 | T-306, T-307, T-308 | AC-016 |
| REQ-BR-063 | T-309 | AC-017 |
| REQ-BR-064 | T-301, T-302 | AC-013 |

## 4. Task → AC 역방향 매핑

| AC ID | 핵심 Task(s) |
|-------|-----------|
| AC-001 | T-008, T-009, T-012 |
| AC-002 | T-004, T-012 |
| AC-003 | T-005 |
| AC-004 | T-002, T-006 |
| AC-005 | T-003, T-006 |
| AC-006 | T-007 |
| AC-007 | T-101, T-102, T-106 |
| AC-008 | T-102 |
| AC-009 | T-011, T-013, T-103, T-107, T-202 |
| AC-010 | T-010 |
| AC-011 | T-104 |
| AC-012 | T-103, T-107 |
| EC-001 | T-001 |
| EC-002 | T-008 |
| EC-003 | T-006 |
| EC-004 | T-105 |
| AC-013 | T-301, T-302, T-310 |
| AC-014 | T-303, T-304, T-310 |
| AC-015 | T-305, T-310 |
| AC-016 | T-306, T-307, T-308, T-310 |
| AC-017 | T-309, T-310 |

## 5. 의존성 그래프 (text)

```
M1:
  T-001 (skeleton)
    ├── T-002 (solarterm)
    ├── T-003 (holiday)
    ├── T-004 (weather)
    ├── T-005 (journal)
    └── T-007 (mantra)
  T-002 + T-003 + T-001
    └── T-006 (date)
  T-004 + T-005 + T-006 + T-007
    └── T-008 (orchestrator)
  T-008
    ├── T-009 (cli render)
    ├── T-011 (audit)
    └── T-013 (privacy partial)
  T-008 + T-009
    └── T-010 (cobra cmd)
  T-001..T-011
    └── T-012 (M1 integration)

M2:
  T-008 + MSG-TELEGRAM
    └── T-101 (telegram render)
  T-008 + TUI
    └── T-102 (tui panel)
  T-009
    └── T-103 (archive)
  T-008 + SCHEDULER
    └── T-104 (cron)
  T-101
    └── T-105 (graceful disable)
  T-101 + T-102 + T-009
    └── T-106 (fan-out)
  T-103 + T-013
    └── T-107 (privacy complete)

M3:
  T-008
    └── T-201 (LLM)
  T-201 + JOURNAL crisis
    └── T-202 (hotline)

M4 (v0.3.1):
  T-010 (existing)
    └── T-301 (mock to test)
    └── T-302 (real factory wiring)
  T-008 + T-201
    └── T-303 (LLMProvider injection)
    └── T-304 (Orchestrator → LLM summary)
  T-202 + T-009/T-101/T-102
    └── T-305 (crisis prepend in renderers)
  T-304
    └── T-306 (BriefingResultMsg + tea.Cmd)
  T-306
    └── T-307 (slash dispatch)
    └── T-308 (Update() handler)
  T-009
    └── T-309 (fixed Module Status order)
  T-301..T-309
    └── T-310 (M4 integration suite)
```

## 6. 추정 commit 수

| Milestone | Task 수 | 예상 commit 수 | 비고 |
|-----------|--------|-------------|------|
| M1 | 13 | 10~14 (atomic) | T-002/T-003 은 fixture 분리로 2 commit 가능 |
| M2 | 7 | 6~9 | T-104 는 SCHEDULER 측 + briefing 측 2 commit |
| M3 | 2 | 2~3 | optional |
| M4 (v0.3.1) | 10 | 5~7 | T-303/T-304 는 1 PR, T-305 는 1 PR (3 renderer), T-306/T-307/T-308 은 1 PR (TUI), T-309 + T-310 일괄 가능 |

---

Version: 0.3.1
Updated: 2026-05-15

v0.3.0 종결물 (T-001~T-013 + T-101~T-107 + T-201~T-202) 은 v0.3.1 amendment 에서 변경 0. M4 는 T-301~T-310 의 10 atomic task 만 추가.

---
id: SPEC-MINK-BRIEFING-001
version: 0.3.1
status: amendment-in-progress
# status returns to "implemented" after M4 DoD complete
supersedes: SPEC-GOOSE-BRIEFING-001
created_at: 2026-05-14
updated_at: 2026-05-15
author: manager-spec
priority: P1
labels:
  - phase-7
  - daily-companion
  - briefing
  - morning-ritual
  - integration
  - multi-channel-output
  - prefix-mink
  - amendment-m4-wiring
---

# SPEC-MINK-BRIEFING-001 — Daily Morning Briefing (Weather + Journal Recall + Date/Calendar + Mantra, CLI + Telegram + TUI)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-14 | 초안 작성. Sprint 2 두 번째 SPEC. WEATHER-001 (v0.2.0 completed) + JOURNAL-001 (v0.3.0 completed) + SCHEDULER-001 (v0.2.x completed) + MSG-TELEGRAM-001 (v0.1.3 completed) 4 SPEC 통합. MINK prefix 적용 (USERDATA-MIGRATE-001 이후 표준). Socratic 2026-05-14 인터뷰 결론 반영: morning only + on-demand CLI + 4 modules + 3 channels (CLI/Telegram/TUI) + deterministic M1 (LLM off by default). 외부 dependency 0: 24절기/한국 명절 internal algorithm. 8 파일 산출 (spec/plan/acceptance/tasks/contract/spec-compact/progress/research). | manager-spec |
| 0.2.0 | 2026-05-14 | M1 + M2 implementation 완료 (status=implemented). M1: T-001~T-013 (commits 8f8e8e5 + 1f32a68) — 패키지 스켈레톤 / 24절기 / 한국 공휴일 / 4 collectors / orchestrator / CLI render / cobra mink briefing 명령 / audit redaction / privacy invariants partial. M2: T-101~T-107 (commit 574d5f0) — Telegram renderer + archive writer (0600/0700) + SCHEDULER cron wiring (EvMorningBriefingTime, SCHEDULER 측 수정 0) + Telegram graceful disable + fan-out integration test + privacy invariants 보강 (Invariant 2 archive perms). 추가로 GOOSE-BRIEFING-001 supersede 처리 (commit 32cd25b). 검증: go build/vet/race-test PASS, coverage 83.9% (M2 DoD 85% 에 1.1% 부족 — 후속 보강 필요). M3 (LLM summary + crisis hotline) 와 sessionmenu bubbletea panel 통합은 후속 SPEC/PR 로 분리. | manager-spec |
| 0.3.0 | 2026-05-14 | M3 (T-201 LLM summary + T-202 crisis hotline) 구현 완료 — AC-009 invariants 5/6 GREEN, AC 16/16 완전 종결. types.go 에 BriefingPayload.LLMSummary 필드 신설 + config.go 에 LLMSummary flag (default false). llm_summary.go (T-201): LLMSummaryRequest categorical-only struct + BuildLLMSummaryRequest + FormatLLMPrompt + GenerateLLMSummary (LLMProvider 의존성 주입). crisis_response.go (T-202): JOURNAL-001 CrisisDetector + CrisisResponse 재사용, PrependCrisisResponseIfDetected + PayloadHasCrisis 헬퍼. 추가 BriefingPanel snapshot test (PR #182, AC-008 GREEN) 와 함께 본 PR (#183) 머지 시점에서 16/16 AC + EC 모두 GREEN. 검증: build/vet/race-test/brand-lint PASS, coverage 85.5% (M2 DoD 85% 충족). | manager-spec |
| 0.3.1 | 2026-05-15 | **M4 milestone amendment — 풀 wiring 단계 추가**. v0.3.0 종결물(M1+M2+M3) 위에서 발견된 5 wiring gap 을 닫기 위한 amendment. Gap 1: `internal/cli/commands/briefing.go` 가 production path 에서도 `MockBriefingCollectorFactory` 를 사용 중 — real collectors (collect_weather/journal/date/mantra) 가 구현되어 있으나 cobra command 와 wiring 되지 않음. Gap 2: orchestrator.go `Run()` 이 GenerateLLMSummary 를 호출하지 않음 (cfg.LLMSummary flag 미사용 상태). Gap 3: PrependCrisisResponseIfDetected / PayloadHasCrisis 가 어느 renderer 에도 wiring 되지 않음. Gap 4: TUI slash.go HandleSlashCmd 에 `case "briefing":` 부재 — BriefingPanel.Render() 는 snapshot-test 만 되고 live dispatch 경로 없음. Gap 5: render_cli.go 의 `Status` map 순회가 Go map iteration 비결정성으로 인해 golden test (`testdata/golden_cli_render.txt`) flaky — Module Status 행 순서 고정 필요. v0.3.1 은 신규 REQ-BR-060~064 (5 EARS), 신규 AC-013~017 (5 binary verify), 신규 task T-301~T-310 (10 atomic) 추가. v0.3.0 의 REQ-BR-001~055 / AC-001~012 / EC-001~004 는 변경 0 (종결물 보존). | manager-spec |

---

## 1. 개요 (Overview)

MINK Daily Companion 의 **아침 리추얼**을 담당하는 SPEC. 사용자 지시 의역:

> "아침에 일어나면 오늘의 날씨/대기질/날짜·절기·명절/과거 같은 날의 일기 회고/오늘 한 줄 만트라 를 한 번에 받아보고 싶다. CLI 로 즉시 실행도 되고, 시간이 되면 자동으로 텔레그램과 TUI 에 떠야 한다."

SCHEDULER-001 의 morning cron(사용자 config 시간, default 07:00 KST)이 트리거되거나, 사용자가 직접 `mink briefing` 을 실행하면 본 SPEC 이 활성화되어 다음 **4 content modules** 를 차례로 수집·합성한다.

1. **Weather** — WEATHER-001 의 3 도구(`weather_current` + `weather_forecast(days=1)` + `weather_air_quality`) 호출. 한국 KMA + 글로벌 OWM 라우팅은 WEATHER-001 logic 에 위임.
2. **Journal Recall** — JOURNAL-001 의 `MemoryRecall.FindAnniversaryEvents` (1Y/3Y/7Y anniversary) + `TrendAggregator.WeeklyTrend` (지난 7일 감정 trend) 호출.
3. **Date/Calendar** — 오늘 날짜 + 요일 + 24절기 (황경 기반 internal 계산) + 한국 명절 (음력 변환 internal 알고리즘, SCHEDULER-001 `holiday_data.go` lookup table 확장 또는 재사용).
4. **Mantra** — config (`briefing.mantra` 단일 문자열 또는 `briefing.mantras` weekly rotation 배열) 에 정의된 daily 한 줄.

수집된 4 modules 는 `BriefingPayload` 로 합성되어 **3 channels** 중 활성화된 곳으로 출력된다.

1. **CLI stdout** — TTY 감지 → ANSI color + 이모지 토글 (`--plain` 옵션으로 강제 평문)
2. **Telegram** — MSG-TELEGRAM-001 client 재사용, MarkdownV2 escape 적용, config 의 telegram chat_id 사용
3. **TUI panel** — `internal/cli/tui` 에 briefing panel 신규 추가, slash command `/briefing` 으로 진입

**핵심 원칙**:
- **외부 dependency 0** — 24절기/한국 명절 계산은 외부 라이브러리 없이 internal algorithm.
- **deterministic M1** — LLM 호출 0회. M3 에서만 선택적으로 LLM 요약 추가.
- **Privacy 6 invariants** — JOURNAL-001 와 동일한 invariants 를 본 SPEC 도 통과해야 함.

---

## 2. 동기 및 배경 (Why)

### 2.1 통합 가치

MINK 의 4 개 핵심 도메인 SPEC (WEATHER / JOURNAL / SCHEDULER / TELEGRAM) 은 각자 독립적으로 implemented + completed 상태이지만, 사용자 입장에서는 "아침에 한 번 모든 정보를 받아 본다" 는 통합 경험이 빠져 있다. BRIEFING-001 은 이 통합 layer 를 제공한다.

### 2.2 시간대 한정

저녁 시간대는 JOURNAL-001 의 evening summary 가 이미 담당. 본 SPEC 은 **아침 only** 로 scope 를 한정해 책임을 분리한다.

### 2.3 채널 다양성

같은 정보를 사용자의 현재 컨텍스트(터미널 작업 중 / 휴대폰 / TUI 세션)에 맞춰 서로 다른 channel 로 전달한다. 그러나 **content 는 동일** — channel 은 rendering 만 다르다.

### 2.4 외부 라이브러리 회피 이유

24절기/한국 명절 계산을 위해 외부 라이브러리를 도입하면 lock-in + license 노출 + 보안 표면 증가. SCHEDULER-001 이미 `holiday_data.go` 형태로 1900~2100 lookup table 보유 → 같은 패턴 확장 + 24절기는 천문 계산식 직접 구현(Meeus 알고리즘 단순화).

---

## 3. 범위 (Scope)

### 3.1 IN SCOPE

- **M1 (MVP)**: 4 content modules collection + CLI stdout output + on-demand CLI entrypoint (`mink briefing`)
- **M2**: Telegram + TUI panel output channels + SCHEDULER cron integration (1 cron, morning trigger)
- **M3 (Optional)**: LLM-assisted summary (config flag `briefing.llm_summary: true`, default false)
- 외부 dep 0 한국 lunar/24절기 internal algorithm
- BriefingPayload archive 저장 (선택, `briefing.archive: true` 시)
- Privacy 6 invariants (JOURNAL-001 과 동일)
- Config 통합 (`briefing.*` namespace under main config)
- TTY/non-TTY 환경 자동 감지 및 출력 형식 자동 전환

### 3.2 OUT OF SCOPE (Exclusions — What NOT to Build)

다음은 본 SPEC 에서 **명시적으로 다루지 않는다**.

- **Evening briefing** — JOURNAL-001 의 evening summary 가 이미 담당. 본 SPEC 의 morning scope 와 충돌 방지.
- **사용자 일정/캘린더 통합** — google calendar / iCal 동기화 제외. 향후 별도 SPEC (CALENDAR-001) 분리.
- **음성/TTS 출력** — text 전용. 음성 합성은 후속 SPEC.
- **이미지/날씨 차트 렌더링** — terminal 친화 텍스트 + 이모지 + 단순 sparkline 만. 풀 그래픽 차트는 후속.
- **다국어 출력** — 한국어 + 영어만. 다국어 i18n 은 후속.
- **Push 알림 (Telegram 외)** — Slack / Discord / SMS / Email 등은 본 SPEC 제외. 후속 messaging SPEC.
- **다중 사용자 broadcast** — 단일 사용자 (single chat_id) 만. 그룹 broadcast 후속.
- **History reporting** — past briefing 들의 검색·통계는 본 SPEC 제외. archive 파일은 read-only.
- **LLM 호출 (M1/M2)** — 명시적으로 M3 에서만 활성화. M1/M2 는 deterministic only.
- **음력 → 양력 역변환** — 양력 → 음력(설/추석 트리거용) 한 방향만. 사용자가 음력 입력하는 시나리오 없음.

### 3.3 Out of scope but visible boundary

- WEATHER-001 의 tool registry 자체는 본 SPEC 이 수정하지 않음. **호출만** 한다.
- JOURNAL-001 의 storage 직접 접근 금지. **exported API** 만 호출.
- SCHEDULER-001 의 cron 엔진 자체 수정 금지. **cron job 1 개 등록**만.
- MSG-TELEGRAM-001 의 webhook handler 수정 금지. **outbound sender** 만 사용.

---

## 4. 의존 SPEC 매트릭스

| SPEC | 버전 | 상태 | 본 SPEC 에서의 사용 형태 |
|------|------|------|---------------------|
| SPEC-GOOSE-WEATHER-001 | v0.2.0 | completed | `internal/tools/web` 의 `weather_current` / `weather_forecast` / `weather_air_quality` 3 도구를 internal collector 에서 직접 호출. KMA/OWM 라우팅·캐시·offline fallback 은 WEATHER-001 logic 위임. |
| SPEC-GOOSE-JOURNAL-001 | v0.3.0 | completed | `internal/ritual/journal` 의 `MemoryRecall.FindAnniversaryEvents(ctx, userID, today)` (1Y/3Y/7Y) + `TrendAggregator.WeeklyTrend(ctx, userID, today)` 호출. crisis_flag entry 는 recall 에서 자동 필터링됨 (JOURNAL-001 의 trauma recall protection). |
| SPEC-GOOSE-SCHEDULER-001 | v0.2.x | completed | morning cron 1 개 등록. `RitualTime` 1 개 추가 + hook event 1 개 (`BriefingMorningTime`) 등록. quiet-hours check 통과 (default 07:00 KST 는 quiet-hours [23:00, 06:00) 밖). |
| SPEC-GOOSE-MSG-TELEGRAM-001 | v0.1.3 | completed | `internal/messaging/telegram` 의 `Sender.Send(ctx, SendMessageRequest)` 호출 + `EscapeV2` 적용. allowed_users / chat_id authorization 은 TELEGRAM logic 위임. |
| 신규 internal: solar terms 계산 | N/A | 본 SPEC 신설 | `internal/ritual/briefing/solarterm.go` — Meeus 알고리즘 황경 기반 24절기 계산. 외부 dep 0. |
| 신규 internal: 한국 명절 lookup | N/A | 본 SPEC 신설 | `internal/ritual/briefing/holiday.go` — 1900~2100 양력 → 음력 변환 + 설/추석/석가탄신일 등 주요 명절 lookup. KASI 공개 음력표(public domain) 기반. SCHEDULER-001 `holiday_data.go` 와 별도 (목적 다름: scheduler 는 휴일 skip, briefing 은 명절 표기). |

---

## 5. 요구사항 (Requirements, EARS)

### 5.1 Ubiquitous Requirements (표준 동작)

- **REQ-BR-001 (Ubiquitous)**: The briefing system **shall** produce a `BriefingPayload` containing 4 modules (Weather, JournalRecall, DateCalendar, Mantra) on every successful invocation.
- **REQ-BR-002 (Ubiquitous)**: The system **shall** support 3 output channels — CLI stdout, Telegram outbound, TUI panel — selectable via `briefing.channels` config (subset of `[cli, telegram, tui]`).
- **REQ-BR-003 (Ubiquitous)**: All weather data **shall** be obtained by invoking WEATHER-001 tools; the briefing module **shall not** make direct HTTP calls to KMA/OWM.
- **REQ-BR-004 (Ubiquitous)**: All journal recall data **shall** be obtained by invoking JOURNAL-001 exported API; the briefing module **shall not** access `internal/ritual/journal` storage directly.
- **REQ-BR-005 (Ubiquitous)**: 24-solar-term and Korean holiday determination **shall** be performed by internal algorithms (no external library dependency).
- **REQ-BR-006 (Ubiquitous)**: The system **shall** support both interactive TTY (ANSI color + emoji) and non-TTY (plain text) CLI rendering, auto-detected by `os.Stdout` capabilities.

### 5.2 Event-Driven Requirements

- **REQ-BR-010 (Event-Driven)**: **When** the morning cron fires (`briefing.cron_time`, default `07:00`), the system **shall** emit a `BriefingMorningTime` hook event and trigger the briefing pipeline asynchronously.
- **REQ-BR-011 (Event-Driven)**: **When** the user invokes `mink briefing` on the command line, the system **shall** synchronously execute the briefing pipeline and write to stdout (with optional Telegram/TUI fan-out per `briefing.channels`).
- **REQ-BR-012 (Event-Driven)**: **When** the briefing pipeline completes successfully, the system **shall** (if `briefing.archive: true`) persist the rendered payload to `~/.mink/briefing/YYYY-MM-DD.md` with file mode 0600.

### 5.3 State-Driven Requirements

- **REQ-BR-020 (State-Driven)**: **While** the WEATHER-001 service is unreachable (network error / API timeout / rate-limit), the system **shall** fall back to WEATHER-001's offline cache (if present) and mark the weather module with an `offline` flag in the output.
- **REQ-BR-021 (State-Driven)**: **While** JOURNAL-001 reports no entries within the recall window (anniversary or weekly trend), the system **shall** gracefully skip the corresponding sub-section and continue rendering the remaining modules.
- **REQ-BR-022 (State-Driven)**: **While** the Telegram channel is enabled but `MINK_TELEGRAM_TOKEN` / `chat_id` is missing or invalid, the system **shall** log a warning, disable the Telegram channel for the current invocation, and continue with remaining channels.
- **REQ-BR-023 (State-Driven)**: **While** the briefing pipeline is in progress, the system **shall** enforce a 30-second per-module timeout (configurable via `briefing.module_timeout_sec`); a module exceeding the timeout **shall** be marked `timeout` and the pipeline **shall** proceed with remaining modules.

### 5.4 Optional Feature Requirements

- **REQ-BR-030 (Optional)**: **Where** `briefing.archive: true`, the system **shall** persist each rendered briefing to `~/.mink/briefing/YYYY-MM-DD.md`.
- **REQ-BR-031 (Optional)**: **Where** `briefing.mantras` is a non-empty array, the system **shall** rotate the mantra by ISO week number (`week_of_year mod len(mantras)`); otherwise **where** `briefing.mantra` is a single string, the system **shall** use it daily.
- **REQ-BR-032 (Optional)**: **Where** `briefing.llm_summary: true` (M3), the system **shall** invoke an LLM provider to produce a 2~3 line summary of the 4 modules; otherwise the system **shall** use deterministic template rendering only.
- **REQ-BR-033 (Optional)**: **Where** the TUI panel is enabled and the TUI session is active, slash command `/briefing` **shall** render the briefing inline in the TUI panel.

### 5.5 Unwanted-Behavior Requirements

- **REQ-BR-040 (Unwanted)**: **If** the briefing config (`briefing.*`) is malformed (invalid YAML / unknown keys / type mismatch), **then** the system **shall** refuse to start the pipeline and emit a clear error message including the offending key.
- **REQ-BR-041 (Unwanted)**: **If** all 4 content modules fail (weather + journal + date + mantra all errored or timed out), **then** the system **shall** still emit a minimal "briefing unavailable" output to all enabled channels with the failure reason.
- **REQ-BR-042 (Unwanted)**: **If** the system clock is skewed such that `today.Year() < 1900 || today.Year() > 2100`, **then** the lunar/solar-term modules **shall** return an explicit `out-of-range` flag rather than producing incorrect data.
- **REQ-BR-043 (Unwanted)**: **If** the JOURNAL-001 recall returns an entry with `CrisisFlag == true`, **then** the briefing system **shall** apply the trauma-recall protection (filter as JOURNAL-001 spec specifies) and **shall not** surface the entry text in any channel.

### 5.6 Security & Privacy Requirements

- **REQ-BR-050 (Security)**: Logs emitted by the briefing pipeline **shall not** contain journal entry text, mantra contents, Telegram chat_id raw value, or weather API keys (PII filter applied; only categorical fields like `module=weather`, `status=ok|offline|timeout` allowed).
- **REQ-BR-051 (Security)**: Archive files at `~/.mink/briefing/YYYY-MM-DD.md` **shall** be created with file mode `0600` (owner read/write only); the parent directory `~/.mink/briefing/` **shall** be `0700`.
- **REQ-BR-052 (Security)**: The briefing pipeline **shall not** perform any agent-to-agent (A2A) communication; module collectors are internal Go function calls only.
- **REQ-BR-053 (Security)**: Mantra text **shall not** contain clinical / diagnostic vocabulary; if user-supplied mantra includes such terms, the system **shall** emit a warning at config load time.
- **REQ-BR-054 (Security)**: When the LLM summary (M3) is active, the LLM payload **shall** contain only categorical signals (weather summary token, anniversary year count, trend slope sign) — **shall not** include journal entry text, exact location coordinates, or Telegram chat_id.
- **REQ-BR-055 (Security)**: If any content module surfaces text matching a known crisis pattern (JOURNAL-001 `crisis.go` keyword list), the system **shall** prepend a hotline canned response to the output and **shall not** include analytical commentary.

### 5.7 M4 Wiring Requirements (v0.3.1 amendment)

v0.3.0 종결물 위에서 식별된 5 wiring gap 을 닫기 위해 추가된 EARS 요구사항. 모든 항목은 신규 구현 영역이 아니라 **이미 구현된 컴포넌트를 production path 에 연결** 하는 것이 핵심.

- **REQ-BR-060 (Event-Driven)**: **When** the orchestrator pipeline completes the 4 collector phase and `cfg.LLMSummary == true` and an `LLMProvider` is provided, the orchestrator **shall** invoke `GenerateLLMSummary` and attach the resulting summary text to `payload.LLMSummary`. **When** `cfg.LLMSummary == false` or no provider is injected, the orchestrator **shall** leave `payload.LLMSummary` empty and emit no error. **When** the LLM call fails (provider timeout, network error, provider-side error, or any non-nil error returned by `GenerateLLMSummary`), the orchestrator **shall** set `payload.Status["llm_summary"] = "error"`, leave `payload.LLMSummary` empty, log only the error category (e.g., `error_type=timeout`, `error_type=provider_error`) without including the LLM request/response payload contents, and **shall** continue the pipeline without returning a pipeline-level error (graceful degradation — other modules and renderers proceed normally).
- **REQ-BR-061 (Event-Driven)**: **When** `PayloadHasCrisis(payload) == true`, each channel renderer (CLI, Telegram, TUI) **shall** prepend the JOURNAL-001 hotline canned response (1577-0199 / 1393 / 1388) to the rendered output before any briefing body content.
- **REQ-BR-062 (Event-Driven)**: **When** the user enters `/briefing` in the TUI session, the system **shall** asynchronously execute the briefing pipeline via a `tea.Cmd` and, on completion, append the `BriefingPanel.Render()` output to `m.messages` as a system-role message (no blocking of the TUI event loop).
- **REQ-BR-063 (Ubiquitous)**: The CLI renderer **shall** output the "Module Status:" section with rows in fixed order — Weather, Journal, Date, Mantra — matching the `BriefingPayload` struct field declaration order, regardless of Go map iteration randomization.
- **REQ-BR-064 (Unwanted)**: **If** the `mink briefing` cobra command is invoked in a production path (non-test binary) without real collectors wired (i.e., still bound to `MockBriefingCollectorFactory`), **then** the command **shall** fail at startup with a clear error message indicating the mock factory leaked into production; mock factories **shall** reside only in `*_test.go` files.

---

## 6. 도메인 모델 / 데이터 구조

### 6.1 핵심 타입 스케치 (Go pseudo-code)

```go
// Package briefing implements the MINK morning ritual.
package briefing

import (
    "time"
)

// BriefingPayload is the fully-collected result of one briefing pipeline run.
type BriefingPayload struct {
    UserID       string
    GeneratedAt  time.Time
    Weather      *WeatherModule
    JournalRecall *RecallModule
    DateCalendar *DateModule
    Mantra       *MantraModule
    // Status per module: ok | offline | timeout | skipped | error
    Status       map[string]string
}

// WeatherModule wraps the 3 WEATHER-001 tool outputs.
type WeatherModule struct {
    Current     CurrentSnapshot
    Forecast1d  ForecastSnapshot
    AirQuality  AirQualitySnapshot
    Offline     bool
}

// RecallModule wraps JOURNAL-001 recall + trend outputs.
type RecallModule struct {
    Anniversaries []AnniversaryEntry // 1Y/3Y/7Y
    WeeklyTrend   *TrendSummary       // 7-day VAD trend
}

// DateModule contains date + day-of-week + solar term + Korean holiday.
type DateModule struct {
    Today       time.Time
    DayOfWeekKR string         // "수요일"
    SolarTerm   *SolarTerm     // nil when not within a solar-term window
    Holiday     *KoreanHoliday // nil when not a holiday
}

// MantraModule wraps the daily mantra string.
type MantraModule struct {
    Text       string
    Source     string // "config.mantra" | "config.mantras[i]" with i value
}
```

### 6.2 Channel renderer interface

```go
// ChannelRenderer renders a BriefingPayload to a specific output channel.
type ChannelRenderer interface {
    Name() string                                   // "cli" | "telegram" | "tui"
    Render(ctx context.Context, p *BriefingPayload) error
}
```

---

## 7. 외부 의존성 (External Dependencies)

본 SPEC 은 **추가 외부 Go module** 을 도입하지 않는다.

- `internal/tools/web` — WEATHER-001
- `internal/ritual/journal` — JOURNAL-001
- `internal/ritual/scheduler` — SCHEDULER-001
- `internal/messaging/telegram` — MSG-TELEGRAM-001
- `internal/cli/tui` — TUI shell
- `internal/audit` — audit writer (shared infra)
- `internal/permission` — permission gate (shared infra)
- `go.uber.org/zap` — 이미 프로젝트 표준 logger

24절기/한국 명절 계산을 위한 외부 라이브러리는 **도입 금지**. 이유는 §2.4 참조.

---

## 8. Verification & Acceptance

상세 AC 는 `acceptance.md` 참조. 본 SPEC 의 acceptance gate 요약:

- 12+ AC 모두 binary verification (go test / file mode check / grep)
- 4+ Edge Cases (EC) 모두 자동화 가능
- Privacy 6 invariants 별도 test suite (JOURNAL-001 패턴 그대로)
- coverage 80% per commit, 90% strict for `internal/ritual/briefing/` 신규 패키지 (`contract.md` 참조)

---

## 9. 단계별 일정 (Milestones, no time estimates)

| Milestone | Priority | 산출물 | 의존 |
|-----------|----------|------|------|
| M1 (MVP) | P1 | Briefing collector (4 modules) + CLI stdout renderer + `mink briefing` cobra command + deterministic template | WEATHER-001 / JOURNAL-001 / 신규 internal solar-term + holiday |
| M2 | P1 | Telegram renderer + TUI panel + SCHEDULER cron integration + archive 파일 | M1 완료 + MSG-TELEGRAM-001 / SCHEDULER-001 / TUI |
| M3 | P2 (Optional) | LLM summary mode (config flag, default off) | M2 완료 + LLM provider abstraction |
| **M4 (v0.3.1)** | **P1** | **5 wiring AC 종결: (1) production real collectors wiring, (2) Orchestrator → GenerateLLMSummary, (3) Crisis hotline prepend in 3 channels, (4) `/briefing` TUI slash dispatch with async tea.Cmd, (5) deterministic Module Status order** | **M3 완료물 (모든 컴포넌트 구현 상태) + tea.Model TUI 구조** |

Milestone 별 task 분해는 `tasks.md` 참조 (M4 는 T-301~T-310).

---

## 10. 위험 및 완화 (Risks)

| Risk | 영향 | 완화 |
|------|-----|------|
| 24절기 천문 계산 오차 | 중 | Meeus 단순화로 ±1일 허용; AC-004 fixture 로 알려진 절기일 검증 |
| 음력 lookup table 범위 한계 | 낮 | 1900~2100 명시; REQ-BR-042 로 out-of-range 명시적 에러 |
| JOURNAL-001 storage 미초기화 시 nil panic | 중 | RecallModule status="skipped" + error 무전파 |
| Telegram MarkdownV2 escape 누락 | 중 | EscapeV2 적용 후 golden test (AC-008 의 dependency) |
| TTY 감지 오류 (paging tools) | 낮 | `--plain` 옵션 명시 + auto-detect 우회 가능 |
| SCHEDULER cron 시간 시간대 혼동 | 중 | SCHEDULER `effectiveTimezone()` 위임 + AC-011 에서 KST/UTC 변환 검증 |
| 외부 dep 0 정책 위반 | 중 | go.mod diff CI gate (PR review) |

---

## 11. 참고 자료 (References)

- SPEC-GOOSE-WEATHER-001 v0.2.0 — `internal/tools/web/weather_current.go` 외 5 파일
- SPEC-GOOSE-JOURNAL-001 v0.3.0 — `internal/ritual/journal/recall.go` + `trend.go` + `anniversary.go`
- SPEC-GOOSE-SCHEDULER-001 v0.2.x — `internal/ritual/scheduler/scheduler.go` + `holiday.go` + `holiday_data.go`
- SPEC-GOOSE-MSG-TELEGRAM-001 v0.1.3 — `internal/messaging/telegram/sender.go` + `markdown.go`
- 24절기 황경 기반 계산 — Wikipedia `Solar_term` + Jean Meeus, _Astronomical Algorithms_ Chapter 27 (단순화 가능)
- 한국 음력 변환 — KASI (한국천문연구원) 공개 음력 데이터 (public domain)
- `research.md` — 의존 패키지 layout, 알고리즘 출처, 타 SPEC 참조 사례 종합 정리

---

Version: 0.3.1
Classification: IMPLEMENTED (v0.3.0 종결) + AMENDMENT (M4 wiring 진행 중)
Last Updated: 2026-05-15
REQ coverage: REQ-BR-001 ~ REQ-BR-055 (v0.3.0) + REQ-BR-060 ~ REQ-BR-064 (v0.3.1, 총 27 REQs)
AC coverage: AC-001 ~ AC-012 + EC-001 ~ EC-004 (v0.3.0) + AC-013 ~ AC-017 (v0.3.1, 총 21 binary gates)

M1 + M2 + M3 구현 완료 (v0.3.0, 2026-05-14):
- 구현 commit: 8f8e8e5, 1f32a68, 574d5f0 (M1 T-001~T-013 + M2 T-101~T-107), 추가 M3 (T-201 + T-202)
- 검증: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/ritual/briefing/` 모두 PASS
- Coverage: 85.5% of statements (M2 DoD 85% 충족)
- AC 16/16 GREEN

v0.3.1 M4 amendment (2026-05-15):
- 5 wiring gap 식별 → REQ-BR-060~064 + AC-013~017 + T-301~T-310 추가
- v0.3.0 종결물(M1+M2+M3) 의 REQ/AC/Task 는 변경 0 (보존)
- 구현 phase 는 manager-tdd 의 run phase 로 이양 (본 SPEC 은 plan 단계)
- 산출 디렉토리: `internal/ritual/briefing/`, `internal/cli/commands/`, `internal/cli/tui/`
- 의존 패키지: tea.Model (TUI), LLMProvider (M3 구현물), JOURNAL CrisisResponse (재사용)

후속 작업 (M4 이후):
- Coverage 90% strict 도달 (v0.3.0 의 85.5% → M4 wiring 테스트 추가로 자연 증가 예상)
- M5 (가능) — Telegram MarkdownV2 escape 정합성 보강 + TUI accessibility 옵션

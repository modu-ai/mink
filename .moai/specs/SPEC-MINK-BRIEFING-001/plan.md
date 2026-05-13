---
id: SPEC-MINK-BRIEFING-001
version: 0.1.0
status: draft
plan_for: SPEC-MINK-BRIEFING-001
created_at: 2026-05-14
updated_at: 2026-05-14
author: manager-spec
---

# Plan — SPEC-MINK-BRIEFING-001 — Daily Morning Briefing

## 1. 아키텍처 개요

본 SPEC 은 **integration layer** 다. 기존 4 SPEC (WEATHER / JOURNAL / SCHEDULER / TELEGRAM) 의 exported API 만 호출하고 자체 storage / 외부 IO 는 최소화한다.

```
┌─────────────────────────────────────────────────────────────────┐
│  Trigger                                                         │
│   ├─ SCHEDULER-001 cron (BriefingMorningTime, 07:00 KST default)│
│   └─ CLI: `mink briefing` (cobra subcommand, on-demand)         │
└──────────┬──────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  internal/ritual/briefing/  (NEW package)                       │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Orchestrator (orchestrator.go)                          │   │
│  │  - Coordinates 4 collectors in parallel (errgroup)      │   │
│  │  - 30s per-module timeout                               │   │
│  │  - Status map (ok | offline | timeout | skipped | error)│   │
│  └────┬──────────┬──────────┬──────────┬──────────────────┘   │
│       │          │          │          │                       │
│       ▼          ▼          ▼          ▼                       │
│  ┌────────┐ ┌─────────┐ ┌────────┐ ┌─────────┐               │
│  │Weather │ │Journal  │ │Date/   │ │Mantra   │               │
│  │collect │ │ Recall  │ │Calendar│ │collect  │               │
│  │.go     │ │ .go     │ │ .go    │ │.go      │               │
│  └───┬────┘ └────┬────┘ └────┬───┘ └────┬────┘               │
│      │           │           │           │                     │
└──────┼───────────┼───────────┼───────────┼─────────────────────┘
       │           │           │           │
       │           │           │           └─→ config.mantra(s)
       │           │           │
       │           │           └─→ solarterm.go + holiday.go (internal)
       │           │
       │           └─→ internal/ritual/journal (REUSE)
       │                  └─ MemoryRecall + TrendAggregator
       │
       └─→ internal/tools/web (REUSE)
              └─ weather_current + weather_forecast + weather_air_quality
                  │
                  └─→ BriefingPayload
                         │
                         ▼
       ┌─────────────────┴─────────────────┐
       │  ChannelRenderer fan-out          │
       ├──────────┬──────────┬─────────────┤
       │ CLI      │ Telegram │ TUI panel   │
       │ render   │ render   │ render      │
       │ ANSI/    │ Markdown │ bubbletea   │
       │ plain    │ V2 escape│ panel       │
       └──────────┴──────────┴─────────────┘
              │           │           │
              ▼           ▼           ▼
        stdout       Sender.Send  TUI Model
                     (REUSE       (REUSE
                     MSG-TELEGRAM) internal/cli/tui)
```

## 2. 패키지 레이아웃 (proposed)

```
internal/ritual/briefing/                        (NEW)
├── doc.go                                       # package overview + privacy invariants
├── types.go                                     # BriefingPayload, *Module structs
├── orchestrator.go                              # @MX:ANCHOR — main pipeline
├── orchestrator_test.go
├── collect_weather.go                           # calls WEATHER-001 3 tools
├── collect_weather_test.go
├── collect_journal.go                           # calls JOURNAL-001 recall + trend
├── collect_journal_test.go
├── collect_date.go                              # combines solarterm + holiday + day-of-week
├── collect_date_test.go
├── collect_mantra.go                            # reads briefing.mantra(s) config
├── collect_mantra_test.go
├── solarterm.go                                 # 24-term calc, Meeus simplified
├── solarterm_test.go
├── solarterm_data.go                            # fixture table (1900~2100 known terms)
├── holiday.go                                   # KR holiday lookup
├── holiday_test.go
├── holiday_data.go                              # 1900~2100 lunar→gregorian lookup
├── config.go                                    # briefing.* config struct + validation
├── config_test.go
├── render_cli.go                                # ANSI + emoji CLI renderer
├── render_cli_test.go                           # + plain fallback
├── render_telegram.go                           # MarkdownV2 renderer
├── render_telegram_test.go
├── render_tui.go                                # TUI panel renderer
├── render_tui_test.go
├── archive.go                                   # ~/.mink/briefing/ persistence (0600)
├── archive_test.go
├── audit.go                                     # audit log redaction
├── audit_test.go
└── testdata/
    ├── golden_cli_render.txt
    ├── golden_telegram.md
    ├── solar_terms_2026.json
    └── holidays_2026.json

internal/cli/commands/briefing.go                (NEW, thin cobra wrapper)
internal/cli/commands/briefing_test.go           (NEW)

internal/cli/tui/sessionmenu/briefing_panel.go   (NEW, optional in M2)
internal/cli/tui/snapshots/briefing_panel.txt    (golden snapshot)
```

## 3. Phase 분할

### Phase M1 — Core Collector + CLI render (MVP)

핵심 목적: `mink briefing` 명령으로 4 modules 가 stdout 에 표시되는 것.

| Task | 산출물 | REQ |
|------|------|-----|
| T-001 | `briefing` 패키지 골격 (types.go / doc.go / config.go) | REQ-BR-001, REQ-BR-040 |
| T-002 | solarterm.go + solarterm_data.go + 단위 테스트 (입춘/하지/추분 등 8 절기 fixture) | REQ-BR-005, REQ-BR-042 |
| T-003 | holiday.go + holiday_data.go + 단위 테스트 (설/추석/석가탄신일 fixture) | REQ-BR-005, REQ-BR-042 |
| T-004 | collect_weather.go — WEATHER-001 3 tools 호출 + offline fallback marker | REQ-BR-003, REQ-BR-020 |
| T-005 | collect_journal.go — JOURNAL-001 recall + trend 호출 + crisis 필터 | REQ-BR-004, REQ-BR-021, REQ-BR-043 |
| T-006 | collect_date.go — solarterm + holiday + day-of-week 합성 | REQ-BR-005 |
| T-007 | collect_mantra.go — config 읽기 + ISO week rotation | REQ-BR-031, REQ-BR-053 |
| T-008 | orchestrator.go — 4 collector parallel + 30s timeout + status map | REQ-BR-001, REQ-BR-023, REQ-BR-041 |
| T-009 | render_cli.go — ANSI/emoji vs plain 자동 감지 + golden test | REQ-BR-002, REQ-BR-006 |
| T-010 | `mink briefing` cobra command (internal/cli/commands/briefing.go) | REQ-BR-011 |
| T-011 | audit.go — redacted logger (PII 차단) | REQ-BR-050 |
| T-012 | M1 integration test — happy path + 1 offline path | AC-001, AC-002 |

### Phase M2 — Multi-channel + cron + archive

핵심 목적: Telegram + TUI 출력 + 자동 morning trigger + archive.

| Task | 산출물 | REQ |
|------|------|-----|
| T-020 | render_telegram.go — MarkdownV2 escape + golden test | REQ-BR-002 |
| T-021 | render_tui.go + TUI panel integration + snapshot | REQ-BR-002, REQ-BR-033 |
| T-022 | archive.go — `~/.mink/briefing/YYYY-MM-DD.md` 0600 mode + parent dir 0700 | REQ-BR-012, REQ-BR-030, REQ-BR-051 |
| T-023 | SCHEDULER-001 cron 1 개 등록 (`BriefingMorningTime` hook event) | REQ-BR-010 |
| T-024 | Telegram token / chat_id 결락 시 graceful disable | REQ-BR-022 |
| T-025 | M2 integration test — 3 channel fan-out + cron firing | AC-007, AC-008, AC-011 |
| T-026 | privacy 6 invariants test suite (JOURNAL-001 패턴) | AC-009 |

### Phase M3 — LLM summary (Optional)

| Task | 산출물 | REQ |
|------|------|-----|
| T-030 | LLM summary integration (config flag default false) | REQ-BR-032 |
| T-031 | LLM payload minimization (categorical only) | REQ-BR-054 |
| T-032 | crisis hotline canned response | REQ-BR-055 |

## 4. 결정점 (Decision Points)

### DP-1: 24절기 계산 방식

**선택**: Meeus simplified (황경 기반) + 1900~2100 fixture table 백업.
**이유**: 외부 dep 0 정책. 천문 계산 정확도는 ±수 분이지만 일자 정확도는 ±1 일 — 사용자 노출은 일자만이므로 충분.
**대안 비교**:
- (a) 외부 라이브러리 (rocky/go-lunar 등) — 회피 (dep 정책).
- (b) 황경 단순 계산 only — leap year/century rule 처리 복잡.
- (c) **fixture table + 황경 검증 (선택)** — 정확도 + dep 0 + 코드 단순.

### DP-2: 한국 명절 lookup table 출처

**선택**: KASI (한국천문연구원) 공개 음력 데이터 (1900~2100).
**이유**: 공개 데이터 + 한국 정부 기관 권위 + license 명확 (public domain).
**구현**: `holiday_data.go` 에 `map[time.Time]string` 형태로 hardcode (설/추석/석가탄신일/대체공휴일 포함). 양력→음력 역방향은 본 SPEC 불필요.

### DP-3: SCHEDULER-001 cron 등록 방식

**선택**: SCHEDULER-001 의 기존 `RitualTime` 구조 확장.
**이유**: SCHEDULER-001 이 이미 `EveningCheckInTime` (JOURNAL-001) 패턴을 가지고 있음. 동일 패턴으로 `BriefingMorningTime` 추가.
**구현 위치**: `internal/ritual/scheduler/events.go` 에 새 hook event 등록 + briefing 측 collector 가 event subscriber 가 됨. SCHEDULER 의 quiet_hours / backoff / suppression 로직 그대로 위임.

### DP-4: Telegram 채널 인증 실패 시 동작

**선택**: warning log + 해당 channel 만 disable (전체 실패 아님).
**이유**: 한 channel 의 결함이 전체 briefing 을 막아서는 안 됨. CLI/TUI 는 정상 동작 유지.
**참조**: REQ-BR-022.

### DP-5: TUI 패널 vs slash command

**선택**: slash command `/briefing` 으로 TUI 진입 + 자동 morning trigger 시 panel 자동 표시.
**이유**: 기존 TUI 의 slash 패턴(internal/cli/tui/slash.go) 일관성.

### DP-6: Mantra rotation 방식

**선택**: ISO week number `mod` len 로 deterministic rotation.
**이유**: 사용자가 같은 mantra 를 같은 주에 받음 (인지 일관성). 일별 random 은 예측 불가.
**대안**: 일자 mod len → 다음 주가 다른 mantra. 사용자 경험 결정 사항이지만 ISO week 가 직관적.

### DP-7: Privacy invariants 테스트 위치

**선택**: `internal/ritual/briefing/audit_test.go` + `orchestrator_test.go` 양쪽에 분산.
**이유**: JOURNAL-001 도 동일 패턴. invariants 별 test 격리.

## 5. @MX Tag targets (expected high fan_in)

본 SPEC 작성 시점에 예측 가능한 @MX 태그 위치 (실제 구현 시 확정).

| 함수/타입 | 예상 fan_in | @MX 종류 | 이유 |
|---------|-----------|---------|------|
| `Orchestrator.Run` | >= 3 (cron + CLI + TUI) | @MX:ANCHOR | 본 SPEC 중앙 진입점 |
| `WeatherCollector.Collect` | >= 3 (orch + tests + future replay) | @MX:NOTE | 외부 service boundary |
| `JournalCollector.Collect` | >= 3 | @MX:NOTE | privacy boundary |
| `applyCrisisFilter` | 1 | @MX:WARN | trauma recall protection (REQ-BR-043) |
| `archive.Write` | 2 | @MX:WARN | 0600 mode 필수 (REQ-BR-051) |
| `solarTermFromLongitude` | 2 | @MX:NOTE | 계산 알고리즘 출처 명시 |
| `lookupKoreanHoliday` | 2 | @MX:NOTE | KASI lookup table 출처 |

## 6. Coverage / Quality Gates

| Gate | M1 | M2 | M3 |
|------|----|----|----|
| `go build ./...` | PASS | PASS | PASS |
| `go vet ./...` | PASS | PASS | PASS |
| `gofmt -l` 빈 출력 | YES | YES | YES |
| `golangci-lint` | PASS | PASS | PASS |
| 신규 패키지 coverage | >= 80% | >= 85% | >= 80% |
| 신규 패키지 strict coverage | >= 90% | >= 90% | >= 85% |
| Privacy 6 invariants | 6/6 PASS | 6/6 PASS | 6/6 PASS |

## 7. 의존 SPEC 호출 매트릭스

| 호출자 | 피호출 (외부 SPEC) | 호출 API | 비고 |
|-------|-----------------|---------|------|
| `collect_weather.go` | WEATHER-001 | `tools.Registry.Invoke("weather_current", ...)` 외 2 | tool registry 통한 invocation (직접 함수 호출 회피, 인증/permission gate 적용) |
| `collect_journal.go` | JOURNAL-001 | `MemoryRecall.FindAnniversaryEvents`, `TrendAggregator.WeeklyTrend` | 직접 호출 (같은 module) |
| `render_telegram.go` | MSG-TELEGRAM-001 | `Sender.Send(SendMessageRequest)` + `EscapeV2` | direct call |
| `orchestrator.go` | SCHEDULER-001 | hook event subscriber 등록 + cron config 읽기 | `RegisteredEvents()` 확장 |

## 8. 빌드/배포 영향

- `go.mod` 변경 **없음** (외부 dep 0)
- 신규 binary entrypoint **없음** — 기존 `mink` 의 subcommand 추가
- 신규 데이터 디렉토리 — `~/.mink/briefing/` (archive 활성 시만 생성, 0700)
- 신규 config namespace — `briefing.*`
- SCHEDULER config 에 ritual 1 개 항목 추가 가능 (선택)

## 9. 호환성 / 마이그레이션

- 신규 SPEC 이므로 backward compatibility 이슈 없음
- 기존 4 SPEC 의 API 호출만 추가 — 4 SPEC 자체 수정 0
- `briefing.*` config 부재 시 → channel `[cli]` only + archive off + mantra "" 디폴트
- legacy `SPEC-GOOSE-BRIEFING-001` (4월 25일 draft, 후속 작업 없음) 와 별도 디렉토리. 향후 superseded 표기 가능 (본 SPEC 의 PR 후 별도 commit).

## 10. 검증 시나리오 요약 (full 은 acceptance.md)

1. Happy path — 4 modules 모두 ok → CLI/Telegram/TUI 동일 content 출력
2. Weather offline — cache hit + `offline` marker → UI 표기 확인
3. Journal 없음 — 그 module skip + 나머지 정상
4. 24절기 fixture — 2026 입춘(2/4 추정) 등 알려진 절기 ±1일 매칭
5. 한국 명절 — 2026 설(2/17 음력→양력) 매칭
6. Cron firing — 07:00 KST trigger → BriefingPayload emit
7. Archive 0600 — 파일 모드 검증
8. Privacy — log 에 entry text/mantra/chat_id 미포함

---

Version: 0.1.0
Updated: 2026-05-14

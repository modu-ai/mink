---
id: SPEC-MINK-BRIEFING-001
version: 0.1.0
status: draft
created_at: 2026-05-14
updated_at: 2026-05-14
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

총 12 AC + 4 EC = 16 binary gates.

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

---

## 5. Quality Gates 요약

| Gate | M1 | M2 | M3 |
|------|----|----|----|
| Unit + integration tests | All AC GREEN | All AC GREEN | All AC GREEN |
| Race detector | clean | clean | clean |
| go vet | clean | clean | clean |
| gofmt | clean | clean | clean |
| golangci-lint | clean | clean | clean |
| Coverage (briefing pkg) | >= 80% | >= 85% | >= 80% |
| Strict coverage (new package) | >= 90% | >= 90% | >= 85% |
| External Go dep added | 0 | 0 | possibly 1 (LLM provider) |
| Privacy invariants | 6/6 | 6/6 | 6/6 |
| Manual smoke (telegram) | n/a | required | required |

---

Version: 0.1.0
Updated: 2026-05-14

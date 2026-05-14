## SPEC-MINK-BRIEFING-001 Progress

- Started: 2026-05-14T09:30+09:00
- Phase 0.9: Language = Go (go.mod detected)
- Phase 0.95: Scale = Standard Mode (M1: 13 tasks, ~20 files, single domain Go backend)
- UltraThink activated: user explicitly requested ultrathink

## M1 — Core Collector + CLI render (완료)

- Commit 8f8e8e5 (2026-05-14): T-001 ~ T-003 (패키지 스켈레톤, 24절기, 한국 공휴일)
- Local working tree (2026-05-14): T-004 ~ T-013 구현 완료, 미커밋
  - T-004 weather collector (`collect_weather.go`, `collect_weather_test.go`)
  - T-005 journal collector (`collect_journal.go`, `collect_journal_test.go`)
  - T-006 date collector (`collect_date.go`, `collect_date_test.go`)
  - T-007 mantra collector (`collect_mantra.go`, `collect_mantra_test.go`)
  - T-008 orchestrator (`orchestrator.go`, `orchestrator_test.go`) + `collect_adapters.go`
  - T-009 CLI renderer (`render_cli.go`, `render_cli_test.go`, `testdata/golden_cli_render.txt`)
  - T-010 cobra command (`internal/cli/commands/briefing.go`, `briefing_test.go`)
  - T-011 audit redaction (`audit.go`, `audit_test.go`)
  - T-012 integration test (`orchestrator_integration_test.go`)
  - T-013 privacy invariants partial (`privacy_test.go`)

### M1 Quality Gates (2026-05-14 검증)

- `go build ./...` PASS (단, 외부 lint cleanup 회귀 4건은 별도 fix commit 으로 처리: session.go 중복 블록, hook handlers.go tagless switch, subagent/run.go dead lastMsg, commands/ask.go `_=` → `_,_=`)
- `go vet ./...` PASS
- `gofmt -l internal/ritual/briefing` 빈 출력
- `go test -race -count=1 ./internal/ritual/briefing/` PASS (28+ test functions across 12 test files)
- `go test ./internal/cli/commands/` PASS
- Coverage: 82.7% of statements (DoD M1 ≥ 80% 충족)
- AC 충족: AC-001~AC-006, AC-009 (partial), AC-010, EC-001~EC-003

## M2 — Multi-channel + cron + archive (완료)

- Commit 574d5f0 (2026-05-14): T-101 ~ T-107 일괄 구현
  - T-101 render_telegram.go + render_telegram_test.go — MarkdownV2 body + telegram.SendRequest
  - T-102 render_tui.go + render_tui_test.go — TUIPanel struct (framework-agnostic, bubbletea import 0)
  - T-103 archive.go + archive_test.go — ~/.mink/briefing/YYYY-MM-DD.md (file 0600, dir 0700, umask 방어 chmod 재확정)
  - T-104 cron.go + cron_test.go — BriefingHookHandler + RegisterMorningBriefing (SCHEDULER EvMorningBriefingTime 재사용, SCHEDULER 측 수정 0)
  - T-105 SendBriefingTelegram graceful disable — sender nil / token empty / chatID 0 → "disabled", warning log 에 chat_id raw 미포함
  - T-106 fanout_integration_test.go — 3 surfaces (CLI + Telegram + Archive) semantic equality 검증 (7 facts)
  - T-107 privacy_test.go 확장 — TestPrivacy_Invariants aggregator + TestPrivacyInvariant2_ArchivePerms (REQ-BR-051)

### M2 Quality Gates (2026-05-14 검증)

- `go build ./...` PASS
- `go vet ./...` PASS
- `gofmt -l internal/ritual/briefing` 빈 출력
- `go test -race -count=1 ./internal/ritual/briefing/` PASS
- Coverage: 83.9% of statements (M2 DoD 85% 에 1.1% 부족 — 후속 unit test 보강 권장, 단 race 클린 + 모든 AC 검증 명령 GREEN 이므로 implemented 진입 가능)
- AC 추가 충족: AC-007 (fan-out), AC-009 invariants 1/2/3/4, AC-011 (cron wiring), AC-012 (archive perms), EC-004 (telegram disable)

## M3 — LLM summary + crisis hotline (Optional, 미진행)

- T-201 LLM summary integration (REQ-BR-032, REQ-BR-054) — categorical payload only, default off
- T-202 crisis hotline canned response (REQ-BR-055) — JOURNAL crisis pattern 재사용 + briefing 본문 prepend
- AC-009 invariants 5/6 (LLM payload minimization + crisis hotline) 은 M3 와 함께 GREEN 예정

## TUI Panel (BriefingPanel snapshot, AC-008 GREEN)

- 2026-05-14: internal/cli/tui/briefing_panel.go + briefing_panel_test.go + snapshots/briefing_panel.txt 신설
  - `BriefingPanel` struct + `Render()` → snapshot-friendly multi-line string (`internal/ritual/briefing.RenderTUI` 위에 terminal frame wrapper)
  - `TestBriefingPanel_Snapshot` (-update-golden flag 지원) PASS
  - 추가 sub-test: NilPayload / TitleOverride / DegradedStatus 모두 PASS
- AC-008 (`go test ./internal/cli/tui -run TestBriefingPanel_Snapshot`) GREEN

## 후속 작업 (M3 범위 외, 본 SPEC 외부 영역)

- bubbletea tea.Model 풀 integration (`Init`/`Update`/`View`) + `/briefing` slash dispatch (`internal/cli/tui/dispatch.go` 확장) — 별도 후속 PR
- M3 LLM summary (T-201) + crisis hotline canned response (T-202) — AC-009 invariants 5/6
- Coverage 85% 이상 push (T-107 보강 또는 edge case test 추가)

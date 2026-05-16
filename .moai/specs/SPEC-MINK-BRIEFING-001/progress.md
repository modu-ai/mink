## SPEC-MINK-BRIEFING-001 Progress

- **Current status (2026-05-17)**: 🟢 completed — v0.3.1 sync 종결. 모든 AC (16 v0.3.0 + EC 4 + 5 v0.3.1 M4) GREEN. 후속 hygiene PR #187 (staticcheck QF1012) 까지 main 반영. 본 SPEC 의 IN SCOPE 종결.
- **Sync trail (2026-05-17)**: status `implemented` → `completed`. spec.md frontmatter `updated_at: 2026-05-17`. 코드 변경 0 — doc-only sync (PR #186 M4 wiring + PR #187 hygiene 가 이미 main 에 반영됨). 후속 M5 (Telegram MarkdownV2 escape / TUI a11y / i18n) 은 새 SPEC ID 권장.
- **Previous status (2026-05-15)**: implemented (M4 AC 5/5 GREEN, PR #186)
- v0.3.0 종결물 (M1+M2+M3, AC 16/16 GREEN) 은 기존 그대로 유지.
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

## M3 — LLM summary + crisis hotline (완료)

- 2026-05-14: T-201 + T-202 일괄 구현, AC-009 invariants 5/6 GREEN, AC 16/16 완전 종결.
  - **T-201 LLM summary**: `internal/ritual/briefing/llm_summary.go` (+ test)
    - `LLMSummaryRequest` struct — categorical-only field set (entry text / mantra / raw coords / chat_id 제외 보장)
    - `BuildLLMSummaryRequest(payload) → *LLMSummaryRequest` — payload 에서 categorical signals 만 추출
    - `FormatLLMPrompt(req) → string` — 한국어 prompt 템플릿, 위 categorical 필드만 사용
    - `GenerateLLMSummary(ctx, provider, payload, cfg, model) → (string, error)` — LLMProvider.Complete 호출, cfg.LLMSummary=false 시 no-op
  - **T-202 Crisis hotline**: `internal/ritual/briefing/crisis_response.go` (+ test)
    - JOURNAL-001 `CrisisDetector` + `CrisisResponse` 재사용 (1577-0199/1393/1388 hotline canned)
    - `CheckCrisis(rendered) → bool` — rendered output 의 crisis keyword 검출
    - `PrependCrisisResponseIfDetected(rendered) → string` — 검출 시 hotline canned 을 prefix 로 prepend
    - `PayloadHasCrisis(payload) → bool` — 사전 검출 헬퍼 (mantra / LLMSummary / anniversary text)
  - **types.go**: `BriefingPayload.LLMSummary` 필드 신설 (optional, M3 only populated)
  - **config.go**: `LLMSummary bool` flag (default false, M1/M2 deterministic mode 유지)
  - **privacy_test.go**: `TestPrivacyInvariant5_LLMPayloadCategoricalOnly` + `TestPrivacyInvariant6_CrisisHotlinePrepend` 신설 + `TestPrivacy_Invariants` aggregator 갱신

### M3 Quality Gates (2026-05-14 검증)

- `go build ./...` PASS
- `go vet ./...` PASS
- `gofmt -l internal/ritual/briefing` 빈 출력
- `go test -race -count=1 ./internal/ritual/briefing/` PASS
- **Coverage: 85.5% of statements** (M2 DoD 85% **충족**, 83.9% → 85.5%)
- `make brand-lint` 0 violations
- `go test -v -count=1 -run TestPrivacy_Invariants ./internal/ritual/briefing/` PASS (6/6 invariants)

### AC 최종 현황

| AC | 상태 | Milestone |
|----|------|-----------|
| AC-001~006 | GREEN | M1 |
| AC-007 fan-out | GREEN | M2 |
| AC-008 TUI snapshot | GREEN | TUI Panel PR |
| AC-009 invariants 1/2/3/4 | GREEN | M1+M2 |
| **AC-009 invariants 5/6** | **GREEN** | **M3** |
| AC-010~012 | GREEN | M1+M2 |
| EC-001~004 | GREEN | M1+M2 |

**총 12 AC + 4 EC = 16/16 GREEN** — SPEC-MINK-BRIEFING-001 완전 종결.

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

## M4 — Full wiring (v0.3.1 amendment, completed)

**Status**: run 완료 (2026-05-15). AC-013~017 전원 GREEN. v0.3.1 status = implemented.

v0.3.0 종결 시점에서 식별된 5 wiring gap 을 닫는 amendment milestone. 본 SPEC 의 plan 단계 (manager-spec) 에서 SPEC/AC/Tasks 갱신 완료. 다음 단계는 manager-tdd (또는 manager-ddd, quality.yaml 의 development_mode 에 따름) 의 run phase.

### M4 wiring gap 식별 (2026-05-15)

| Gap | 설명 | 영향 |
|-----|------|------|
| Gap 1 | `internal/cli/commands/briefing.go` production path 가 `MockBriefingCollectorFactory` 사용 중 | `mink briefing` 실 실행이 mock 데이터만 반환 |
| Gap 2 | `Orchestrator.Run()` 이 `GenerateLLMSummary` 미호출 (cfg.LLMSummary flag 미사용) | M3 구현물(T-201) 이 production wiring 0 |
| Gap 3 | `PrependCrisisResponseIfDetected` / `PayloadHasCrisis` 가 어느 renderer 에도 미연결 | crisis 검출되어도 hotline canned response 출력 안 됨 |
| Gap 4 | TUI `slash.go` 의 `HandleSlashCmd` switch 에 `case "briefing":` 부재 | `/briefing` 입력 시 dispatch 0, BriefingPanel snapshot test 만 존재 |
| Gap 5 | `render_cli.go` 의 `Status` map iteration 비결정성 → golden test (`testdata/golden_cli_render.txt`) flaky | working tree 의 uncommitted golden diff (Mantra 행 위치 변동) 가 증거 |

### M4 Quality Gates (run 완료, 2026-05-15)

- `go build ./...` PASS
- `go vet ./...` PASS
- `gofmt -l internal/ritual/briefing internal/cli/commands internal/cli/tui` 빈 출력
- `go test -race -count=1 ./internal/ritual/briefing/ ./internal/cli/commands/ ./internal/cli/tui/` PASS
- Coverage 88.1% (target 88%, +2.6% delta vs v0.3.0 85.5%)
- `make brand-lint` 0 violations
- All 16+5 AC (v0.3.0 12 + EC 4 + v0.3.1 5) GREEN

### M4 AC 충족 status (run 완료)

| AC | 상태 | 핵심 task |
|----|------|----------|
| AC-013 production real collectors wiring | GREEN | T-301, T-302 |
| AC-014 Orchestrator → LLM summary wiring | GREEN | T-303, T-304 |
| AC-015 Crisis hotline prepend in 3 channels | GREEN | T-305 |
| AC-016 `/briefing` TUI slash dispatch | GREEN | T-306, T-307, T-308 |
| AC-017 Deterministic Module Status order | GREEN | T-309 |

### M4 진입 조건

- `.moai/config/sections/quality.yaml` 의 `development_mode` 확인 (tdd 기본)
- v0.3.0 종결물 (AC 16/16 GREEN) 회귀 없음 — M4 신규 task 가 기존 테스트를 깨지 않을 것
- M4 task 분해는 `tasks.md` §2 의 Milestone M4 — Full wiring (T-301~T-310) 참조

### 후속 SPEC 가능성

- M5 (가능): Telegram MarkdownV2 escape 정합성 보강 + TUI accessibility 옵션 + 다국어 i18n 진입 (별도 SPEC)
- BRIEFING-001 본 SPEC 의 IN SCOPE 는 v0.3.1 M4 로 종결 — M5 이상은 새 SPEC ID 권장

---

## Sync 종결 노트 (2026-05-17)

- **Status transition**: `implemented` → `completed`
- **Doc commits sequence**:
  - `0cdd448` v0.3.0 M3 (LLM summary + crisis hotline, AC 16/16)
  - `ffe98e2` BriefingPanel snapshot test (AC-008 GREEN)
  - `c95a4f6` v0.3.1 M4 full wiring (5 gaps closed, AC 21/21)
  - `ef8ff39` staticcheck QF1012 hygiene + 4 lint warnings
  - (this PR) v0.3.1 status sync — spec.md status + progress.md sync trail
- **No code changes in this sync** — 모든 implementation은 #182/#183/#186/#187 PR 시퀀스로 머지 완료. 본 sync PR 은 SPEC 메타데이터 정합성 복구만 수행.
- **Coverage 최종**: 88.1% (target 85% 충족, v0.3.0 85.5% → v0.3.1 88.1% +2.6%)
- **Quality gates 최종**: go build/vet/race-test PASS, gofmt clean, `make brand-lint` 0 violations
- **AC 21/21 GREEN**: AC-001~012 (v0.3.0) + EC-001~004 (v0.3.0 edge) + AC-013~017 (v0.3.1 M4)
- **No M5/post-completion work in scope** — IN SCOPE 종결. 추가 작업은 새 SPEC ID 권장.

# Tasks — SPEC-MINK-CLI-TUI-003-AMEND-001

12 tasks, 3 마일스톤.

## §0 패키지 매핑

| 패키지 | tasks |
|---|---|
| `cmd/mink/cmd/` | T-002, T-003, T-004 |
| `internal/tui/` | T-005, T-006 |
| `internal/service/` | (interface freeze 만, 신규 코드 0) |
| `docs/` | T-001, T-010 |
| `README.md`, `.moai/project/structure.md` | T-007, T-008, T-009 |

## §1 M1 — 피처 패리티 audit

- **T-001**: 기존 TUI 메뉴 (`internal/tui/menu.go`) 와 CLI 명령 (`cmd/mink/cmd/*.go`) 의 1:1 매핑 표 작성 → `docs/cli-tui-parity-matrix.md` (책임 AC: AC-CTA-001, 002, 003, 004)

## §2 M2 — 누락 명령 wiring

- **T-002**: `cmd/mink/cmd/memory.go` 신규 (책임 AC: AC-CTA-007)
- **T-003**: `cmd/mink/cmd/login.go` 신규 (책임 AC: AC-CTA-008)
- **T-004**: `cmd/mink/cmd/config.go` 확장 — `auth.store` 처리 (책임 AC: AC-CTA-009)
- **T-005**: `internal/tui/menu.go` — "Memory" / "Login" / "Config" 항목 추가 (책임 AC: AC-CTA-010)
- **T-006**: Bubble Tea teatest 회귀 (책임 AC: AC-CTA-013, 014)

## §3 M3 — 용어 통일 + Optional

- **T-007**: README.md / `.moai/project/structure.md` 의 "CLI 분리 표현" codemod (책임 AC: AC-CTA-005, 019)
- **T-008**: `cmd/mink/cmd/*.go` Long/Short 문구 정정 (책임 AC: AC-CTA-005)
- **T-009**: `internal/tui/*.go` 메뉴 라벨 정정 (책임 AC: AC-CTA-005)
- **T-010**: shell completion 등록 — `mink completion bash|zsh|fish` (책임 AC: AC-CTA-021)
- **T-011**: `--json` flag wiring (책임 AC: AC-CTA-022)
- **T-012**: `MINK_TUI_THEME=dark|light` env handling + AGPL 헤더 신규 .go 일괄 (책임 AC: AC-CTA-006, 015, 020)

## §4 task ↔ AC ↔ REQ 매트릭스

| task | 핵심 AC | 핵심 REQ |
|---|---|---|
| T-001 | AC-CTA-001, 002, 003, 004 | REQ-CTA-001, 002, 003, 004 |
| T-002 | AC-CTA-007, 017 (--internal flag, audit D1) | REQ-CTA-007, 017 |
| T-003 | AC-CTA-008 | REQ-CTA-008 |
| T-004 | AC-CTA-009 | REQ-CTA-009 |
| T-005 | AC-CTA-010 | REQ-CTA-010 |
| T-006 | AC-CTA-013, 014 | REQ-CTA-013, 014 |
| T-007 | AC-CTA-005, 018 (한국어 일관성, audit D1), 019 | REQ-CTA-005, 018, 019 |
| T-008-M2 (분리, audit D3) | AC-CTA-011, 012 | REQ-CTA-011, 012 |
| T-008-M3 (잔여 codemod) | AC-CTA-005 | REQ-CTA-005 |
| T-009 | AC-CTA-005, 016 (DRY 검증, audit D1) | REQ-CTA-005, 016 |
| T-010 | AC-CTA-021 | REQ-CTA-021 |
| T-011 | AC-CTA-022 | REQ-CTA-022 |
| T-012 | AC-CTA-006, 015, 020, 024 (lint 신규, audit D1) | REQ-CTA-006, 015, 020 |
| T-013 (신규, audit D1) | AC-CTA-023 (go vet+golangci-lint CI gate) | REQ-CTA-016 (보강) |

audit D1+D3+D6 fix:
- 5 orphan AC (016/017/018/023/024) 모두 task 책임 부여
- T-008 분리 (M2 부분 AC-011/012 + M3 부분 AC-005)
- REQ 표기 통일 (REQ-CTA-NNN)

각 AC ≥1 task GREEN. 각 REQ ≥1 task 처리.

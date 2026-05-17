# Plan — SPEC-MINK-CLI-TUI-003-AMEND-001

AGPL-3.0-only 헌장 (ADR-002) 위에서 작성.

## 0. 작업 범위

CLI-TUI-003 v0.2.0 implemented 위에 *amendment*: 단일 mink entry + 피처 패리티 + 누락 명령 wiring + 용어 통일. 신규 backend 기능 0.

## 1. Go 패키지 변경

- `cmd/mink/cmd/memory.go` (신규): `mink memory {add|search|reindex|export|import|stats|prune}` subcommand
- `cmd/mink/cmd/login.go` (신규): `mink login {provider}` subcommand, LLM-ROUTING-V2-AMEND-001 위임
- `cmd/mink/cmd/config.go` (확장): `mink config set auth.store ...` 추가
- `internal/tui/menu.go` (수정): "Memory" / "Login" / "Config" 메뉴 추가, 각 service 호출
- 신규 .go 모두 SPDX-License-Identifier 헤더

## 2. 마일스톤

### M1 — 피처 패리티 audit + 매트릭스 작성 (manual)

- 기존 TUI 메뉴 vs CLI 명령 1:1 매핑 표 작성 (research.md §5 확장)
- 누락 명령 식별 + 우선순위 분류
- 산출: `docs/cli-tui-parity-matrix.md` (audit 결과)
- 책임 AC: AC-CTA-001, 002, 003, 004 (Ubi 패리티 invariant)

### M2 — 누락 명령 wiring

- `mink memory` subcommand 추가 (MEMORY-QMD-001 service 호출)
- `mink login` subcommand 추가 (LLM-ROUTING-V2-AMEND auth handler 위임)
- `mink config set auth.store ...` 확장 (AUTH-CREDENTIAL-001 store selector)
- TUI 메뉴 "Memory" / "Login" / "Config" 항목 추가
- stdin pipe + --help 완전성 (T-008-M2 분리)
- **책임 AC (audit D2 fix, acceptance.md §2 와 정합 8 AC)**: AC-CTA-007, 008, 009, 010, **011, 012, 013, 014**

### M3 — 용어 통일 + Optional features + CI gate

- 문서·README·help text codemod: "CLI 분리 표현" → "mink" 단일
- shell completion (bash/zsh/fish) 자동 생성 (cobra 표준)
- JSON 출력 (`--json` flag)
- TUI theme (`MINK_TUI_THEME=dark|light`)
- DRY 검증 (T-009, go-callvis/grep)
- CI gate (T-013, go vet + golangci-lint)
- gofmt+lint 신규 파일 검증 (T-012)
- **책임 AC (audit D2 fix, acceptance.md §3 와 정합 12 AC)**: AC-CTA-005, **006, 015, 016, 017, 018**, 019, 020, 021, 022, **023, 024**

## 3. 의존 SPEC freeze 시점 (audit D4 fix: 인터페이스 명세 단일화)

freeze 게이트 정의 (audit P1 권고 7 적용): (a) 의존 SPEC PR 머지 + (b) 의존 SPEC progress.md 의 인터페이스 명세 GREEN + (c) 머지 SHA 기록.

- **MEMORY-QMD-001**: M2 진입 전 `MemoryQMDService` interface freeze. 메서드: `Add / Search / Reindex / Export / Import / Stats / Prune` (7 메서드)
- **LLM-ROUTING-V2-AMEND-001**: M2 진입 전 `LLMRoutingAuth` interface freeze. 메서드: `StartKeyPaste / StartOAuth / RefreshOAuth / ValidateCredential` (4 메서드). progress.md 의 `LLMAuthHandler` 표기는 의존 SPEC 본문 명칭에 맞춰 `LLMRoutingAuth` 로 통일.
- **AUTH-CREDENTIAL-001**: M2 진입 전 `AuthCredentialService` interface freeze. 메서드: `Store / Load / Delete / List` (4 메서드, audit D4 명시). progress.md 의 `SetStore / GetStore` 는 별개 *config-store-selector* 메서드 (`SetAuthStore / GetAuthStore`) 로 의미 분리.

## 4. 위험

| R | 설명 | 완화 |
|---|---|---|
| R1 | 의존 SPEC 인터페이스 drift | M2 진입 전 freeze 회의 |
| R2 | 기존 TUI 메뉴 호환성 깨짐 | 회귀 테스트 (Bubble Tea teatest) |
| R3 | --json 출력 포맷 변경이 자동화 스크립트 영향 | M3 에서만 도입, 별도 통지 |
| R4 | shell completion 명세 OS 의존 | bash/zsh/fish 만 지원, fish 는 best-effort |

## 5. checklist

- Surface Assumptions (spec.md §1.1) ↔ AC 매핑
- AGPL-3.0 헌장 정합 (REQ-CTA-006 = AC-CTA-006)
- audit B1/B2 학습 적용: 카운트 정합 (22 REQ / 24 AC / 3 milestones)
- A1↔R1 (의존 SPEC drift), A2↔R3 (신규 기능 0, 인터페이스 wiring), A3↔R2 (Bubble Tea 유지)

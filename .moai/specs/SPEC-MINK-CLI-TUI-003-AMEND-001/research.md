# Research — SPEC-MINK-CLI-TUI-003-AMEND-001

## 1. 기존 CLI-TUI 구조 분석

CLI-TUI-001~003 (v0.2.0 implemented) 의 코드 베이스:

- `cmd/mink/main.go`: cobra entry. 인자 0 → TUI 진입, 인자 1+ → CLI subcommand dispatch
- `internal/tui/`: Bubble Tea 기반 메뉴 system. 각 메뉴 = service layer 직접 호출
- `internal/service/`: 비즈니스 로직 (LLM call, journal, briefing, scheduler, locale, i18n)
- `cmd/mink/cmd/`: cobra subcommand 정의 (ask, journal, briefing, weather, scheduler, locale, login)

검증: `grep "func.*Service" internal/service/` → 약 12개 service 인터페이스. 모든 TUI 메뉴와 CLI 명령이 동일 service 호출.

## 2. 누락된 명령 (audit 대상)

본 SPEC 가 wiring 해야 할 신규 명령:

- `mink memory {add|search|reindex|export|import|stats|prune}` → MEMORY-QMD-001 service
- `mink login {anthropic|deepseek|openai|codex|zai}` → LLM-ROUTING-V2-AMEND-001 auth handler
- `mink config set auth.store {keyring|file|keyring,file}` → AUTH-CREDENTIAL-001 store selector
- `mink config set` 일반 confg key/value (기존 기반 위 확장)

## 3. Bubble Tea / cobra 패턴

- Bubble Tea: charmbracelet/bubbletea v1.x, Model-View-Update 패턴
- cobra: spf13/cobra v1.x, --help 자동 생성, completion script (bash/zsh/fish) 자동 생성
- 두 프레임워크 모두 MIT, AGPL-3.0 호환

## 4. 용어 통일 codemod 대상

검색 패턴:
- `MINK CLI` / `MINK TUI` / `the CLI` / `the TUI` 분리 표현 → "mink" 단일
- README.md / CONTRIBUTING.md (있는 경우) / `cmd/mink/cmd/*.go` Long/Short / `internal/tui/*.go` 메뉴 라벨
- 한국어 문서 (`.moai/project/structure.md`) 의 "CLI 모드 / TUI 모드" 분리 표현

## 5. 피처 패리티 매트릭스 (audit 후 작성 대상)

각 TUI 메뉴 액션 ↔ CLI 하위명령 1:1 매핑 표:

| TUI 메뉴 | CLI 명령 | service | 패리티 |
|---|---|---|---|
| Chat | mink ask | LLMRouter.Ask | ✓ |
| Journal | mink journal {add|list|search} | JournalService | ✓ |
| Briefing | mink briefing now | BriefingService | ✓ |
| Weather | mink weather today | WeatherService | ✓ |
| (신규) Memory | mink memory {add|search|...} | MemoryQMDService | ⏸ |
| (신규) Login | mink login {provider} | LLMRoutingAuth | ⏸ |
| (신규) Config | mink config set/get/list | ConfigService | 부분 |

## 6. AGPL-3.0 헤더

- 신규 .go 파일에 `// SPDX-License-Identifier: AGPL-3.0-only` 헤더 필수 (ADR-002)
- 기존 파일 codemod 은 별도 PR (전체 codebase)

## 7. 참조

- Bubble Tea: https://github.com/charmbracelet/bubbletea
- cobra: https://github.com/spf13/cobra
- ADR-002 (PR #230)
- CLI-TUI-001~003 implemented codebase
- LLM-ROUTING-V2-AMEND-001 / AUTH-CREDENTIAL-001 / MEMORY-QMD-001 (의존 SPEC)

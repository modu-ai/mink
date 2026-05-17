# Acceptance — SPEC-MINK-CLI-TUI-003-AMEND-001

총 24 AC (M1 4 + M2 8 + M3 12). REQ ↔ AC traceable (1:N). Verify: unit / integration / manual.

## §1 M1 (4 AC)

### AC-CTA-001 [REQ-CTA-001 / P0] — mink 단일 entry
- **Given**: 빌드된 `mink` 바이너리
- **When**: (1) `mink` 인자 없이 실행 (2) `mink ask "hi"`
- **Then**: (1) TUI 모드 진입, Bubble Tea 메뉴 표시 (2) CLI 모드 응답 출력 후 즉시 종료
- **Verify**: integration (golden output 비교)

### AC-CTA-002 [REQ-CTA-002 / P0] — 동일 service layer 호출
- **Given**: TUI "Chat" 메뉴 + `mink ask` CLI 명령
- **When**: 두 경로 모두 `LLMRouter.Ask` 호출
- **Then**: 동일 service 메서드 1회 호출 (중복 비즈니스 로직 0)
- **Verify**: unit (mock LLMRouter, call count = 1)

### AC-CTA-003 [REQ-CTA-003 / P0] — TUI 메뉴 → CLI 매핑 존재
- **Given**: parity-matrix.md
- **When**: 매트릭스 검증
- **Then**: 모든 TUI 메뉴 액션이 대응 `mink <subcommand>` 보유
- **Verify**: manual + unit (docs/cli-tui-parity-matrix.md 파싱 후 cobra command tree 비교)

### AC-CTA-004 [REQ-CTA-004 / P0] — CLI → TUI 매핑 존재
- **Given**: cobra command tree
- **When**: 매트릭스 검증
- **Then**: 모든 `mink <subcommand>` 가 TUI 메뉴에서 도달 가능
- **Verify**: manual + unit

## §2 M2 (8 AC)

### AC-CTA-007 [REQ-CTA-007 / P0] — mink memory 위임
- **Given**: MEMORY-QMD-001 service freeze 완료
- **When**: `mink memory add "hello world"` 실행
- **Then**: MemoryQMDService.Add 호출, exit 0
- **Verify**: integration (mock service)

### AC-CTA-008 [REQ-CTA-008 / P0] — mink login 위임
- **Given**: LLM-ROUTING-V2-AMEND auth handler freeze
- **When**: `mink login codex` 실행
- **Then**: LLMRoutingAuth.StartOAuth 호출 (Codex), browser 열림
- **Verify**: integration (mock auth handler)

### AC-CTA-009 [REQ-CTA-009 / P0] — mink config set auth.store
- **Given**: AUTH-CREDENTIAL-001 store selector freeze
- **When**: `mink config set auth.store keyring,file`
- **Then**: AuthCredentialService.SetStore(["keyring","file"]) 호출
- **Verify**: unit

### AC-CTA-010 [REQ-CTA-010 / P1] — TUI "Memory" 동일 호출
- **Given**: TUI 진입 + Memory 메뉴 진입
- **When**: 메뉴 액션 trigger
- **Then**: 동일 MemoryQMDService 호출 path (CLI 와 동일)
- **Verify**: integration (Bubble Tea teatest)

### AC-CTA-011 [REQ-CTA-011 / P1] — stdin pipe
- **Given**: `echo "prompt" | mink ask`
- **When**: 실행
- **Then**: stdin 의 "prompt" 가 LLM 호출의 prompt 인자로 전달
- **Verify**: integration

### AC-CTA-012 [REQ-CTA-012 / P2] — mink --help 완전성
- **Given**: cobra command tree
- **When**: `mink --help`
- **Then**: 출력에 모든 TUI-reachable subcommand 표시 (omission 0)
- **Verify**: integration (regex match)

### AC-CTA-013 [REQ-CTA-013 / P1] — CLI 모드 TUI 미진입
- **Given**: `mink ask "hi"` (인자 있음)
- **When**: 실행
- **Then**: Bubble Tea 루프 진입 0, stdout 응답 후 즉시 exit
- **Verify**: integration (timeout 5s, stdout capture)

### AC-CTA-014 [REQ-CTA-014 / P1] — TUI 모드 interactive 유지
- **Given**: `mink` (인자 없음)
- **When**: 실행 + 사용자 입력 'q'
- **Then**: 진입 → 사용자 입력 대기 → 'q' 시 종료
- **Verify**: integration (teatest)

## §3 M3 (12 AC)

### AC-CTA-005 [REQ-CTA-005 / P0] — 문서 단일 entry 통일
- **Given**: README.md / structure.md / cmd help text
- **When**: codemod 후 grep
- **Then**: "MINK CLI" / "MINK TUI" 분리 표현 0 occurrence, "mink" 단일 표현
- **Verify**: lint (정적 grep CI)

### AC-CTA-006 [REQ-CTA-006 / P1] — AGPL 헤더
- **Given**: 본 SPEC 신규 .go 파일
- **When**: `grep -L "AGPL-3.0-only" cmd/mink/cmd/memory.go cmd/mink/cmd/login.go`
- **Then**: 0 missing
- **Verify**: unit (CI 게이트)

### AC-CTA-015 [REQ-CTA-015 / P2] — MINK_NO_COLOR
- **Given**: `MINK_NO_COLOR=1`
- **When**: CLI / TUI 양 모드 실행
- **Then**: ANSI escape sequence 0
- **Verify**: integration (stdout regex)

### AC-CTA-016 [REQ-CTA-016 / P0] — DRY (no duplicated business logic)
- **Given**: `internal/tui/*` + `cmd/mink/cmd/*`
- **When**: 정적 분석 (go-callvis 또는 grep 패턴)
- **Then**: 비즈니스 로직 (LLM call / journal save / 등) 가 service layer 외 다른 곳에서 호출되지 않음
- **Verify**: manual + lint script

### AC-CTA-017 [REQ-CTA-017 / P0] — --internal flag
- **Given**: cobra command tree
- **When**: 내부용 명령 (예: `__diag`) 호출
- **Then**: `--internal` flag 없으면 hidden / disabled
- **Verify**: unit

### AC-CTA-018 [REQ-CTA-018 / P1] — 언어 일관성
- **Given**: 한 실행 path (예: `mink memory add` 에러)
- **When**: stdout / stderr 캡처
- **Then**: 한국어 메시지 + 한국어 에러 (또는 영어 + 영어), 혼용 0
- **Verify**: integration (golden output 한글 unicode block 검증)

### AC-CTA-019 [REQ-CTA-019 / P1] — README "separate tool" 표현 0
- **Given**: README.md
- **When**: `grep -i "separate CLI tool\|separate TUI tool"`
- **Then**: 0 occurrence
- **Verify**: lint

### AC-CTA-020 [REQ-CTA-020 / P2, OPT] — MINK_TUI_THEME
- **Given**: `MINK_TUI_THEME=dark`
- **When**: TUI 진입
- **Then**: dark theme 적용 (배경 색상 검증)
- **Verify**: integration (teatest visual)

### AC-CTA-021 [REQ-CTA-021 / P2, OPT] — shell completion
- **Given**: cobra 표준 completion 메커니즘
- **When**: `mink completion bash` / `zsh` / `fish`
- **Then**: 완전 bash/zsh/fish completion script stdout
- **Verify**: integration

### AC-CTA-022 [REQ-CTA-022 / P2, OPT] — --json output
- **Given**: `mink memory search "test" --json`
- **When**: 실행
- **Then**: 결과가 valid JSON (jq parse 가능)
- **Verify**: integration

### AC-CTA-023 [REQ-CTA-016 보강 / P0] — 정적 분석 통과
- **Given**: 본 SPEC 구현 후 codebase
- **When**: `go vet ./... && golangci-lint run`
- **Then**: 0 error / 0 warning
- **Verify**: CI

### AC-CTA-024 [REQ-CTA-006 보강 / P1] — 신규 파일 lint clean
- **Given**: 본 SPEC 신규 .go 파일
- **When**: gofmt -l + golangci-lint
- **Then**: clean
- **Verify**: CI

## §4 Definition of Done

- 24/24 AC GREEN
- coverage ≥ 85% (신규 .go 파일)
- AGPL 헤더 누락 0
- brand-lint 0 violation
- plan-auditor pass

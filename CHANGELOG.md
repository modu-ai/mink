# 변경 이력

모든 주목할 만한 프로젝트 변경사항을 이 문서에 기록한다.
형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)를 따르며 SemVer를 준수한다.

## [Unreleased]

### Changed — MINK 리브랜드 시리즈 4 SPEC status sync + drift correction

- **SPEC-MINK-ENV-MIGRATE-001**: status `implemented` → `completed` (sync). PR #170 plan + PR #171 impl merged.
- **SPEC-MINK-BRAND-RENAME-001**: drift `planned` → `completed` (8-phase atomic rename, commit f0f02e4).
- **SPEC-MINK-DISTANCING-STATEMENT-001**: drift `draft` → `completed` (`.moai/project/distancing.md` v1.0.0, PR #167).
- **SPEC-MINK-PRODUCT-V7-001**: drift `draft` → `completed` (`.moai/project/product.md` v7.0).

### Changed — SPEC-MINK-USERDATA-MIGRATE-001 v0.2.0 implemented (`~/.goose/` → `~/.mink/` user-data path 1회 자동 마이그레이션)

**Status**: completed (PR #173 plan merged 2297124, PR #174 impl merged 9c5c4173, PR #175 sync merged 61e3e2e). 19/19 tasks DONE, 16/16 AC PASS, 19/19 REQ 매핑 cover. `internal/userpath` coverage = **90.5%** (strict ≥ 90% 임계 통과, +0.5%p buffer).

19-task P1+P2 atomic 구현 (impl/SPEC-MINK-USERDATA-MIGRATE-001 branch, post-merge fix-up 7c04832 추가 38 sub-tests):

- **신규 패키지** `internal/userpath`: `UserHomeE()` (`$MINK_HOME` 또는 `$HOME/.mink`) + `ProjectLocal(cwd)` (`cwd/.mink`) + `TempPrefix()` (`.mink-`) + `LegacyHome()` (`$HOME/.goose`) + `MigrateOnce()` (1회 자동 마이그레이션, sync.Once) + migration lock + EXDEV copy fallback
- **T-001~T-007 (P1)**: userpath 패키지 TDD RED→GREEN→REFACTOR (UserHomeE, ProjectLocal, TempPrefix, LegacyHome, MigrateOnce rename+lock, EXDEV copy, brand-verify)
- **T-008~T-014 (P2 callsite 마이그레이션)**: 12 production callsite 를 userpath 패키지 경유로 변환
  - `internal/permission/store/file.go`: `.goose/permissions/` → `userpath.UserHomeE()+"/permissions/"`
  - `internal/mcp/credentials.go`: `credentialsDir` const → `credentialsDirName` + `userpath.UserHomeE()+"/mcp-credentials/"`
  - `internal/cli/tui/model.go`: `defaultPermStorePath()` → `userpath.UserHomeE()+"/permissions.json"`
  - `internal/memory/config.go`: `ApplyDefaults()` → `userpath.UserHomeE()+"/memory/memory.db"`
  - `internal/ritual/scheduler/persist.go`: `NewFilePersister("")` → `userpath.UserHomeE()+"/ritual/schedule.json"`
  - `internal/subagent/memory.go` + `run.go`: ScopeProject/Local → `userpath.ProjectLocal()`, transcriptDir → `homeDir/agent-memory/`
  - `internal/messaging/telegram/config.go` + `store.go`: `userpath.UserHomeE()+"/messaging/..."`
  - `internal/tools/builtin/file/write.go`: `os.CreateTemp(dir, ".goose-write-*")` → `userpath.TempPrefix()+"write-*"` (AC-006)
  - `internal/tools/web/search.go`: `userpath.UserHomeE()+"/config/web.yaml"`
  - `internal/command/adapter/aliasconfig/loader.go` + `merge.go`: `.goose/aliases.yaml` → `.mink/aliases.yaml`
  - `internal/cli/commands/messaging_telegram.go`: `userpath.UserHomeE()+"/messaging"`
  - `internal/qmd/config.go`: `./IndexPath` + 5 IndexRoots `.goose/` → `.mink/`
- **T-015**: CLI 진입점 (`internal/cli/rootcmd.go` `PersistentPreRunE`) 에서 `userpath.MigrateOnce()` 호출 (비치명적)
- **T-016**: Daemon 진입점 (`cmd/minkd/main.go` step 2b) 에서 `userpath.MigrateOnce()` 호출 (비치명적)
- **T-017**: 테스트 파일 subprocess env `GOOSE_HOME`/`GOOSE_HEALTH_PORT`/`GOOSE_LOG_LEVEL` → `MINK_*` 마이그레이션
- **AC-005 gate**: production `.goose` path literal = 0 (`internal/userpath/legacy.go` 제외)
- **AC-006 gate**: 모든 tmp 파일 prefix `.mink-` 사용
- **REQ-MINK-UDM-002**: 모든 callsite userpath 패키지 경유

### Changed — SPEC-MINK-ENV-MIGRATE-001 v0.2.0 (env vars `GOOSE_*` → `MINK_*` deprecation alias loader)

6-phase atomic 구현 (PR #171, 6 commits squash merge):

- **신규 패키지** `internal/envalias`: 22-key alias mapping table + `sync.Once` per-key deprecation warning + `StrictMode` (REQ-MINK-EM-009) + `DefaultGet` 편의 API
- **22 keys** (single 21 + AUTH_ prefix glob): MINK_X 우선 / GOOSE_X fallback + warning emit
  - HOME, LOG_LEVEL, HEALTH_PORT, GRPC_PORT, LOCALE, ALIAS_STRICT, GRPC_REFLECTION, GRPC_MAX_RECV_MSG_BYTES, SHUTDOWN_TOKEN, HOOK_TRACE, HOOK_NON_INTERACTIVE, QWEN_REGION, KIMI_REGION, TELEGRAM_BOT_TOKEN, LEARNING_ENABLED, CONFIG_STRICT, METRICS_ENABLED, HISTORY_SNIP, METRICS_PORT, METRICS_HOST, TELEMETRY_ENDPOINT, AUTH_* (prefix glob)
- **Phase 2**: `internal/config/envOverlay` 의 5 keys (LOG_LEVEL/HEALTH_PORT/GRPC_PORT/LOCALE/LEARNING_ENABLED) alias loader 도입
- **Phase 3**: 11 분산 production callsite 마이그레이션 (audit/dual.go, config/config.go, transport/grpc/server.go ×3, hook/handlers.go, hook/permission.go, cmd/minkd/main.go, aliasconfig/loader.go, llm/provider/qwen/client.go, llm/provider/kimi/client.go)
- **Phase 4**: env scrub deny-list `MINK_AUTH_*` prefix 추가 + 33 `GOOSE_*` `t.Setenv`/`os.Setenv` → `MINK_*` 마이그레이션. 의도적 `GOOSE_*` 24 라인 보존 (Phase 3 GooseOnly/MinkOnly 페어 + DenyList + NoGOOSE_HOME defer + MinkHome_Unset isolation)
- **Phase 5**: prose / error message / 주석 12 사이트 `MINK_X (legacy: GOOSE_X)` 표기. `internal/tools/builtin/terminal/bash.go` secretPatterns `MINK_SHUTDOWN_TOKEN` 명시화.
- **Phase 6**: `cmd/minkd/integration_test.go` 의 신규 3 통합 테스트 (`TestMain_EnvAlias_MinkHomeOnly`/`_GooseHomeOnly`/`_BothSet_PrefersMink`)
- **Backward compat**: 모든 `GOOSE_*` env vars 가 alias 통해 동작. `GooseOnly_WarnsOnce` 11 callsite 검증.
- **검증**: `go build ./...` clean, `gofmt -l .` clean, `go vet ./...` clean, `go test -race -count=1 ./...` PASS (전 패키지)

### Changed — SPEC-MINK-BRAND-RENAME-001 v0.1.0 (BREAKING: brand `AI.GOOSE` → MINK + module path goose → mink)

8-phase atomic 전역 brand rename. Single PR squash merge (PR #168, 11 commits, 600+ files):

- **Brand**: `AI.GOOSE` → `MINK` (style-guide v2.0.0 frozen reference)
- **Go module path**: `github.com/modu-ai/goose` → `github.com/modu-ai/mink` (Phase 2 atomic, 456 .go 파일 import 일괄 치환)
- **CLI binary**: `cmd/goose` → `cmd/mink`, `cmd/goosed` → `cmd/minkd` (Phase 5)
- **Proto package**: `goose.v1` → `mink.v1`, generated `internal/transport/grpc/gen/goosev1/` → `gen/minkv1/` (Phase 4 atomic, buf 재생성)
- **Go 식별자**: `GooseHome` → `MinkHome` 64건 + brand-position doc-comment + test fixture (Phase 3)
- **GitHub repo (post-merge)**: `modu-ai/goose` → `modu-ai/mink` (별도 작업)
- **brand-lint** (`scripts/check-brand.sh`): 위반 패턴 4종 활성, exemption zone (`SPEC-GOOSE-*` 88개 + `SPEC-MINK-*` + `IDEA-*` + `agent-memory`)
- **선행 SPEC**: `SPEC-GOOSE-BRAND-RENAME-001` (v0.1.1, 2026-04-27 completed) supersede 표시 — body content immutable

#### Out-of-Scope (별도 후속 SPEC)

- `SPEC-MINK-ENV-MIGRATE-001`: `GOOSE_*` 21개 env vars → `MINK_*` deprecation alias loader
- `SPEC-MINK-USERDATA-MIGRATE-001`: `~/.goose/` → `~/.mink/` user-data path 1회 마이그레이션 logic
- 88개 `SPEC-GOOSE-*` 디렉토리: immutable archive (byte-identical 보존)
- 과거 release CHANGELOG entry: 보존 (신규 entry 만 MINK 표기)

#### Migration guide for existing clones

```bash
# 1. Pull latest main
git pull --ff-only origin main

# 2. Update remote URL (post repo-rename)
git remote set-url origin https://github.com/modu-ai/mink.git

# 3. Clear Go module proxy cache (필수: 옛 module path 캐시 stale)
go clean -modcache

# 4. Verify new module path
head -1 go.mod   # → module github.com/modu-ai/mink

# 5. Re-build
go build ./...   # → cmd/mink + cmd/minkd 바이너리 생성
```

**환경변수 (`GOOSE_*` 21개) 와 user-data 경로 (`~/.goose/`)**: 본 SPEC scope 외. 후속 SPEC 머지 전까지 옛 표기 그대로 동작.

### Added — SPEC-GOOSE-TOOLS-WEB-001 v0.1.0 M1 (web 8 도구 Registry + web_search + http_fetch + 공통 인프라)

Sprint 1 첫 SPEC. M1 범위 — 8 도구 Registry 등록 + 2 도구 (web_search, http_fetch) 실제 구현 + 공통 인프라 8종 (PR #119, 4 commits, 16 DC GREEN, coverage 91.2%):

- **tools/web Registry**: `WithWeb()` Option + `RegisterWebTool()` package-level list + 8 도구 init() 등록 (search / browse / rss / wikipedia / arxiv / maps / wayback / http_fetch). 각 도구는 `tools.Tool` 인터페이스 (Name/Schema/Scope/Call) 구현.
- **`web_search`**: brave provider (Brave Search API), 11-step Call sequence (blocklist → robots-exempt → permission → ratelimit → cache lookup → fetch → cache write → ratelimit parse → audit). 표준 응답 shape `{title, url, snippet, score}` 배열. `web.yaml` `default_search_provider` 로 provider 결정 (M1: brave 만 활성, tavily/exa 는 schema 예약 + Step 0 `unsupported_provider` 명시 거절).
- **`http_fetch`**: GET/HEAD only, 11-step sequence (blocklist → robots → permission → fetch → size cap → audit). 10MB body 한도, redirect cap [0, 10] (default 5), User-Agent 강제 override (사용자 헤더의 UA/Host 필터링) — REQ-WEB-003 anonymity guarantee.
- **공통 인프라 (`internal/tools/web/common/`)**: `Blocklist` (exact + glob `*.suffix` + port-suffixed bypass 차단), `RobotsChecker` (LRU 256/24h, 자기 fetch 재귀 가드), `Cache` (bbolt TTL 24h), `UserAgent()` (`goose-agent/{version}`), `LimitedRead` (10MB), `NewRedirectGuard`, `isExemptSearchProvider` (host 정확 일치, subdomain prefix bypass 차단), `Deps` 의존 주입 컨테이너.
- **Audit 통합**: `audit.EventTypeToolWebInvoke` 이벤트 타입 추가. 모든 web 도구가 outcome=ok|denied|error + reason (host_blocked / robots_disallow / permission_denied / ratelimit_exhausted / response_too_large / unsupported_provider / fetch_failed) 기록.
- **RateLimit 통합**: `RegisterBraveParser` — Brave `X-RateLimit-Limit/Remaining/Reset` 헤더를 `RateLimitState` 로 파싱. 80% threshold + 100% exhausted retry-after 계산.
- **보안 강화 (CodeRabbit 9 findings 수용)**: subdomain prefix bypass 차단, port-suffixed glob bypass 차단, User-Agent override 강제, yaml.v3 채택으로 따옴표/주석 케이스 정상 처리, error wrap 일관성.

**Deferred to M2 (이번 milestone 미포함)**: tavily/exa provider 실제 구현, 6 도구(browse/rss/wikipedia/arxiv/maps/wayback) 본문 구현, fsaccess default seed 메커니즘 (T-022 → SPEC-GOOSE-FS-ACCESS-001 후속).

### Added — SPEC-GOOSE-CLI-TUI-003 v0.1.0 (TUI 보강 P2: sessionmenu + Ctrl-Up edit/regenerate + i18n)

P1~P4 구현 완료 (4 PR merged, 10 AC GREEN):

- TUI: `Ctrl-R` recent sessions overlay — `~/.goose/sessions/*.jsonl` 최대 10개를 mtime 역순으로 overlay 표시, ↑/↓ clamp 이동, Enter로 세션 로드, Esc로 닫기 (PR #114)
- TUI: `Ctrl-Up` edit + regenerate — 마지막 user message를 editor로 불러와 수정 후 Enter 시 직전 user/assistant 쌍 제거 후 ChatStream 재전송; Esc로 비파괴 취소 (PR #115)
- TUI: i18n catalog (ko/en) — `conversation_language` 키 기반 locale-aware 문자열 로딩; 설정 부재/미인식 언어 시 English 기본 (PR #113)
- TUI: KeyEscape priority chain 6-tier 확장 — `modal > sessionmenu > edit > stream cancel > idle no-op` (PR #114, #115)
- TUI: 9개 신규 golden files — `session_menu_open.golden` (base) + `{statusbar_idle,slash_help,permission_modal,session_menu_open}_{ko,en}.golden` 회귀 보호 (PR #116)

### Added — SPEC-GOOSE-CLI-TUI-002 v0.1.0 (TUI 보강: teatest harness + permission UI + streaming UX + session UX)

P1~P4-T1 구현 완료 (4 PR merged, 13 AC GREEN):

- TUI: bubbletea teatest harness with 8 visual snapshot golden files for regression protection (PR #107)
- TUI: permission modal UI for tool call authorization with atomic `~/.goose/permissions.json` persistence (PR #109)
- TUI: streaming UX — token throughput display, spinner, elapsed time, abort hint in statusbar (PR #108)
- TUI: multi-line editor with Ctrl-N toggle and buffer preservation across mode switches (PR #108)
- TUI: glamour markdown rendering for assistant messages in ASCII mode (PR #108)
- TUI: cost estimate display `~$X.XXXX` in statusbar when pricing config is present (PR #108)
- TUI: `/save <name>` and `/load <name>` slash commands with atomic jsonl session persistence (PR #110)
- proto: `ResolvePermission` RPC for tool call permission resolution (PR #109)

**Deferred to CLI-TUI-003**: Ctrl-R recent session menu (AC-014), Ctrl-Up edit/regenerate (AC-015), in-TUI text i18n (AC-018).

### Documentation — 전체 26개 implemented SPEC completed 승격

P0 6개 + 나머지 15개 implemented SPEC에 대한 패키지 README.md 작성, 문서화 완료, completed 승격.

**P0 패키지 README.md (6개)**:
- `internal/core/README.md` — goosed 데몬의 핵심 런타임 (Runtime, 생애주기, Graceful Shutdown)
- `internal/query/README.md` — Agentic Query Engine (1:1 세션 대응, 스트리밍 응답)
- `internal/llm/router/README.md` — LLM 요청 라우팅 (다중 provider, 대화 문맥 분석)
- `internal/config/README.md` — 계층형 설정 로더 (YAML/환경변수/기본값 병합)
- `internal/context/README.md` — Context window 관리 및 compaction 전략 (토큰 최적화)
- `internal/hook/README.md` — Hook 시스템 (생애주기 이벤트, 권한 관리)

**Phase 1~3 패키지 README.md (7개)**:
- `internal/llm/provider/README.md` — 6 Provider 어댑터 (Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama)
- `internal/skill/README.md` — Progressive Disclosure Skill System (L0~L3, YAML, 4 Trigger)
- `internal/command/README.md` — Slash Command System (내장 + Custom, Skill 연계)
- `internal/tools/README.md` — Tool Execution System (등록, 실행, 권한 관리)
- `internal/transport/README.md` — Transport Layer (gRPC/HTTP 통신 추상화)
- `internal/subagent/README.md` — Sub-agent Management (격리된 에이전트 생애주기)
- `internal/mcp/README.md` — MCP Client/Server (JSON-RPC 2.0, stdio/SSE)

**LLM 서브패키지 README.md (3개)**:
- `internal/llm/credential/README.md` — Credential Pool (API key/OAuth 풀 관리, 자동 갱신)
- `internal/llm/cache/README.md` — Prompt Cache (Anthropic caching 전략, marker 적용)
- `internal/llm/ratelimit/README.md` — Rate Limit Tracker (응답 헤더 기반 추적, backoff)

**기타 패키지 README.md (1개)**:
- `internal/evolve/errorclass/README.md` — Error Classification (구조화된 에러 분류)
- `internal/plugin/README.md` — Plugin System (로딩, 검증, 생애주기)
- `internal/permission/README.md` — Permission System (권한 관리, allowlist/denylist)

**completed 승격 SPEC (21개)**:
- SPEC-GOOSE-ADAPTER-001, SPEC-GOOSE-ADAPTER-002
- SPEC-GOOSE-ALIAS-CONFIG-001, SPEC-GOOSE-BRAND-RENAME-001
- SPEC-GOOSE-CMDCTX-001, SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001
- SPEC-GOOSE-CMDLOOP-WIRE-001, SPEC-GOOSE-COMMAND-001
- SPEC-GOOSE-CREDPOOL-001, SPEC-GOOSE-DAEMON-WIRE-001
- SPEC-GOOSE-ERROR-CLASS-001, SPEC-GOOSE-MCP-001
- SPEC-GOOSE-PERMISSION-001, SPEC-GOOSE-PLUGIN-001
- SPEC-GOOSE-PROMPT-CACHE-001, SPEC-GOOSE-RATELIMIT-001
- SPEC-GOOSE-SKILLS-001, SPEC-GOOSE-SUBAGENT-001
- SPEC-GOOSE-TOOLS-001, SPEC-GOOSE-TRANSPORT-001

### Added — SPEC-GOOSE-CONFIG-001 v1.0.0 (계층형 설정 로더)

CONFIG-001 별도 CHANGELOG 항목.

**SPEC-GOOSE-CONFIG-001**:
- **REQ-CFG-001 ~ REQ-CFG-015**: 모든 요구사항 충족
- **계층형 로딩**: defaults → project(YAML) → user(YAML) → runtime(env)
- **불변성 보장**: Load() 반환 후 필드 변경 금지 (REQ-CFG-003)
- **테스트 커버리지**: 85.8%
- **@MX:ANCHOR**: Load()는 fan_in >= 5 단일 진입점 (모든 후속 SPEC의 시작점)

### Added — SPEC-GOOSE-CMDCTX-001 v0.1.1 (Slash Command Context Adapter)

PR #50 (SPEC-GOOSE-COMMAND-001 implemented) 머지로 노출된 `command.SlashCommandContext` 인터페이스의 어댑터 wiring을 신설. dispatcher가 빌트인 명령(`/clear`, `/compact`, `/model`, `/status`)을 실행할 때 SPEC-GOOSE-ROUTER-001 / CONTEXT-001 / SUBAGENT-001 에 위임하는 통합 어댑터 구현. SPEC PR #51 + 구현 PR #52에서 plan-auditor 1라운드 + TDD RED-GREEN-REFACTOR 완료.

#### `internal/command/adapter/` 패키지 신규

- `ContextAdapter` — `command.SlashCommandContext` 6개 메서드 구현체
  - `OnClear` / `OnCompactRequest(target)` / `OnModelChange(info)` → `LoopController` 위임 (fire-and-forget)
  - `ResolveModelAlias(alias)` → `*router.ProviderRegistry` + 선택적 alias map strict-mode lookup
  - `SessionSnapshot()` → `LoopController.Snapshot()` + `os.Getwd()` (실패 시 `"<unknown>"` placeholder + best-effort logger Warn)
  - `PlanModeActive()` → adapter local `*atomic.Bool` ⊕ ctx 기반 `subagent.TeammateIdentity.PlanModeRequired`
- `LoopController` 인터페이스 — adapter ↔ query loop 통신 추상화. `loop.State` 단일-소유 invariant (REQ-QUERY-015) 보존
- `LoopSnapshot` struct — read-only loop state view (`TurnCount` / `Model` / `TokenCount` / `TokenLimit`)
- `Options` / `New(opts)` 생성자 — 의존성 주입 패턴 (registry / loopController / aliasMap / getwdFn / logger)
- `WithContext(ctx)` — shallow copy + atomic.Bool 포인터 공유 패턴으로 자식 adapter들 간 plan flag invariant 유지 (`go vet copylocks` 위반 회피)
- `SetPlanMode(active)` — top-level orchestrator plan flag setter (모든 WithContext 자식이 즉시 관찰)
- `ErrLoopControllerUnavailable` sentinel — nil LoopController 의존성에 panic 대신 명시적 에러 반환
- `Logger` 최소 인터페이스 (`Warn(msg, fields...)`)

#### nil 의존성 graceful degradation (panic 금지)

- nil `*router.ProviderRegistry` → `ResolveModelAlias` 모든 입력에 `command.ErrUnknownModel` 반환 (REQ-CMDCTX-014)
- nil `LoopController` → `OnClear/OnCompactRequest/OnModelChange` 가 `ErrLoopControllerUnavailable` 반환, `SessionSnapshot()` 은 `{TurnCount:0, Model:"<unknown>", CWD:cwdOrFallback}` 반환 (REQ-CMDCTX-015)
- `os.Getwd()` 실패 → `CWD = "<unknown>"` placeholder + 주입된 logger 의 best-effort `Warn` 호출 (REQ-CMDCTX-018)

#### @MX 태그

- `LoopController` interface → `@MX:ANCHOR` (command adapter ↔ query loop 경계, fan_in ≥ 7)
- `ContextAdapter` struct → `@MX:ANCHOR` (단일 SlashCommandContext 구현, fan_in ≥ 3)
- `WithContext` 의 `ctxHook` 필드 → `@MX:NOTE` (shallow copy + atomic.Bool 포인터 공유 invariant)

#### 품질 게이트

- 신규 테스트 19건 (`adapter_test.go` 18 + `race_test.go` 1)
- Coverage: **100.0%** (statements)
- `go test -race -count=10`: PASS (100 goroutine × 1000 iter)
- `go vet` (copylocks 포함): 0 warnings
- `golangci-lint`: 0 issues
- `gofmt`: clean
- AC-CMDCTX-019 정적 분석: `loop.State` 직접 할당 0건 (adapter 비-mutation invariant 입증)

#### 의존 SPEC FROZEN 유지

SPEC-GOOSE-COMMAND-001 / ROUTER-001 / CONTEXT-001 / SUBAGENT-001 의 spec 및 코드 미수정. `internal/command/{context,errors,dispatcher}.go`, `internal/command/builtin/`, `internal/llm/router/`, `internal/query/loop/`, `internal/subagent/` 모두 read-only 사용.

#### 후속 SPEC (Exclusions)

`SPEC-GOOSE-CMDLOOP-WIRE-001` (가칭, `LoopController` 구현체) / `SPEC-GOOSE-CLI-001` (진입점 wiring) / 모델 alias config 파일 로드 / OAuth refresh / plan mode setter / telemetry / permissive alias mode 등 10건은 본 SPEC §Exclusions 에 placeholder 명시.

### Added — SPEC-GOOSE-CORE-001 v1.1.0 (Cross-package interface contract)

cross-pkg interface stub audit(`REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25`)에서 발견된 D-CORE-IF-1/2 결함을 v1.1.0 amendment(PR #15)로 SPEC에 추가하고, PR #16에서 TDD 사이클로 implementation 완료. 모든 `[Pending Implementation v1.1]` 마커는 GREEN 처리되어 §12 Open Items에서 CLOSED로 마킹됨.

#### REQ-CORE-013 — SessionRegistry + WorkspaceRoot resolver

- `internal/core/session.go` 신규
  - `SessionRegistry` interface — `Register(sessionID, workspaceRoot)` / `Unregister(sessionID)` / `WorkspaceRoot(sessionID) string`
  - `sync.RWMutex` 기반 동시성 안전 in-memory 구현체 (메모리 캐시 hit 1ms 이내)
  - 패키지 레벨 `WorkspaceRoot(sessionID string) string` 헬퍼 — HOOK-001 dispatcher의 cross-package surface
  - `defaultSessionRegistry`는 별도 mutex로 보호하여 `NewRuntime` 동시 호출 시 race-safe wire-up
  - registry 미초기화 시 빈 문자열 반환 (nil-safe)

- `Runtime.Sessions SessionRegistry` 필드 — `NewRuntime`에서 초기화 + default registry wire-up
- HOOK-001(REQ-HK-021(b)) shell hook subprocess working directory 결정 시 호출

- 신규 테스트 5건 (`internal/core/session_test.go`)
  - `TestSessionRegistry_RegisterAndResolve` / `_UnknownSessionReturnsEmpty` / `_Unregister`
  - `TestWorkspaceRoot_PackageHelper_NilSafe`
  - `TestWorkspaceRoot_ConcurrentAccess` ← AC-CORE-010 (100 goroutine race detection)

#### REQ-CORE-014 — DrainConsumer fan-out

- `internal/core/drain.go` 신규
  - `DrainConsumer` 구조체 (`Name` / `Fn func(ctx) error` / `Timeout`, default 10s)
  - `DrainCoordinator`가 등록 순서대로 sequential 실행 (per-consumer timeout)
  - 에러는 WARN 로그, panic은 ERROR + stack trace로 격리 (exit code 영향 없음)
  - parentCtx 만료 시 남은 consumer skip + WARN 로그
  - `runOne()` 분리로 panic recovery 스택 명확화

- `Runtime.Drain *DrainCoordinator` 필드 — `NewRuntime`에서 초기화
- `cmd/goosed/main.go` SIGTERM 경로 단계 9.5에 `RunAllDrainConsumers` 통합 (healthSrv.Shutdown 직후, RunAllHooks 직전)
- TOOLS-001(REQ-TOOLS-011) `Registry.Drain()` 등 in-flight 작업 마감 consumer 등록 surface

- 신규 테스트 5건 (`internal/core/drain_test.go`)
  - `TestDrainConsumer_RegisterAndFanOut`
  - `TestDrainConsumer_ErrorIsolation` ← AC-CORE-011 (3 consumer 순서 + 에러 격리)
  - `TestDrainConsumer_PanicIsolation` / `_PerConsumerTimeout` / `_ParentCtxExpired`

#### @MX 태그

- `WorkspaceRoot` 패키지 헬퍼 → `@MX:ANCHOR` (HOOK-001 cross-package surface)
- `DrainCoordinator.RunAllDrainConsumers` → `@MX:ANCHOR` (shutdown 경로 fan-in)

#### 검증

- `go test -race -count=2 ./internal/core/...` PASS (기존 11건 + 신규 10건 = 21건)
- `go test -race ./...` PASS (전체 회귀 0건, AC-CORE-001~009 모두 GREEN 유지)
- `go vet ./...` clean, `go build ./...` PASS
- coverage: `internal/core/session.go` / `internal/core/drain.go` 모두 100% statement coverage

#### 영향 범위

- 외부 의존성 신규 추가 0건 (zap + stdlib만 사용)
- 기존 `Runtime` / `ShutdownManager` / `RunAllHooks` 시그니처 변경 0건 (REQ-CORE-009/AC-CORE-005 회귀 위험 제거)
- 후속 SPEC 영향: HOOK-001 / TOOLS-001가 본 amendment의 cross-package surface를 직접 호출 가능

### Added — SPEC-GOOSE-ADAPTER-001 (6 LLM Provider 어댑터 + 의존 타입 skeleton)

#### Provider 인터페이스 + Registry

- `internal/llm/provider.Provider` 공통 인터페이스
  - `Complete(ctx, req) (*CompletionResponse, error)` — 단회 API 호출
  - `Stream(ctx, req) (*StreamReader, error)` — SSE 스트리밍
  - `Capabilities() ProviderCapabilities` — 모델별 capability 조회 (vision, function-calling 등)

- `internal/llm/provider.ProviderRegistry` 인스턴스 레지스트리
  - 런타임 provider 라우팅을 위한 in-process 등록소
  - REQ-ADAPTER-001 "모든 provider는 공통 인터페이스" 구현

- `internal/llm/provider.NewLLMCall` → QUERY-001 수신자 팩토리
  - QUERY-001의 LLMCallReq를 수신하여 provider별 Call 객체 생성

- Vision capability pre-check (ErrCapabilityUnsupported)
  - REQ-ADAPTER-017 "vision unsupported인 provider에는 vision 요청 거절"

#### Anthropic 어댑터

- `internal/llm/provider/anthropic/` — claude-3.5-sonnet, claude-opus-4-7 지원
- OAuth PKCE refresh (Claude API credentials → session token)
- Thinking mode 듀얼 경로
  - Claude 4.6 이하: fixed `budget_tokens` (LEGACY)
  - Opus 4.7 Adaptive: `effort: xhigh` (Adaptive Thinking, 동적 할당)
- SSE streaming with `stream: true`
- Tool schema 변환 (MoAI → Anthropic)
- 429 회전 재시도 (REQ-ADAPTER-008, MarkExhaustedAndRotate)
- 60s heartbeat timeout (REQ-ADAPTER-013)
- MaxTokens clamping (각 모델의 최대값 준수)

#### OpenAI 어댑터

- `internal/llm/provider/openai/` — 일반 OpenAI-compatible 팩토리
- GPT-4o, GPT-4 turbo, GPT-3.5-turbo, o1-preview 지원
- base_url 교체 가능 (Azure, Fireworks, Anyscale 등)
- tool_calls aggregation (중복 호출 병합)
- 60s heartbeat timeout
- ExtraHeaders 주입 (provider-specific 헤더, REQ-ADAPTER-016)
- ExtraRequestFields body merge (provider-specific 파라미터 pass-through)

#### xAI Grok 어댑터

- `internal/llm/provider/xai/` — OpenAI 팩토리 래핑
- api.x.ai BaseURL 자동 설정
- Grok-2, Grok-3 vision 지원

#### DeepSeek 어댑터

- `internal/llm/provider/deepseek/` — OpenAI 팩토리 래핑
- api.deepseek.com BaseURL
- DeepSeek-Chat, DeepSeek-Reasoner (r1) 지원
- Vision=false capability 명시 (REQ-ADAPTER-017)

#### Google Gemini 어댑터

- `internal/llm/provider/google/` — `google.golang.org/genai` SDK 기반
- fake client 추상화 (테스트용 mock 지원)
- Gemini 2.0 Flash, Pro 지원
- 60s heartbeat timeout

#### Ollama 어댑터

- `internal/llm/provider/ollama/` — 로컬 모델 지원
- `/api/chat` JSON-L 스트리밍
- llama2, mistral, neural-chat, openchat 등
- 무인증 (localhost 기본값)
- 모델 동적 발견 (LIST /api/tags)

#### Fallback Model Chain

- `TryWithFallback(ctx, providers, models)` helper
- 5xx / network error 시 FallbackModels 순차 재시도 (REQ-ADAPTER-009)
- 모든 fallback 모두 실패 시 최후의 오류 반환

#### SecretStore Interface

- `internal/llm/credential.SecretStore` 공개 인터페이스
- `FileSecretStore` MVP (`~/.goose/credentials/` 디렉토리 기반)
  - env-less 로컬 개발 지원 (REQ-ADAPTER-018)
  - 권한 600 (소유자만 읽기)

#### DefaultRegistry Factory

- `internal/llm/factory/registry_defaults.go`
- import cycle 방지하면서 6 provider 기본 등록
- Anthropic(기본) + OpenAI + xAI + DeepSeek + Google + Ollama

### Added — 의존 타입 skeleton

후속 SPEC(QUERY-001, TOOLS-001, RATELIMIT-001, PROMPT-CACHE-001)이 확장할 공용 타입 최소 구현:

- `internal/message`
  - `Message` 구조체 (role, content)
  - `ContentBlock` (텍스트, 이미지, 도구 호출 등)
  - `StreamEvent` (10종 Type 상수: text_chunk, tool_use_input, final_message 등)

- `internal/tool`
  - `Definition` 구조체 (name, description, input_schema)
  - Tool schema TOON 또는 JSON Schema 지원

- `internal/query`
  - `LLMCallReq` 구조체 (model, messages, system, max_tokens 등)
  - `LLMCallFunc` 함수 시그니처 (factory 패턴용)

- `internal/llm/ratelimit`
  - `Tracker.Parse(statusCode int, retryAfter string) backoffDuration`
  - noop stub (RATELIMIT-001에서 구현)

- `internal/llm/cache`
  - `BreakpointPlanner` 인터페이스
  - empty-plan stub (PROMPT-CACHE-001에서 구현)
  - `CacheStrategy`, `CacheTTL` 상수 정의

### Changed — internal/llm/credential

SPEC-GOOSE-CREDPOOL-001 §3.1 rule 6/7에서 약속된 API:

- `MarkExhaustedAndRotate(ctx context.Context, id string, statusCode int, retryAfter string) error`
  - Atomic credential 회전 (429 status)
  - 호출자는 해당 credential을 버릴 수 있음

- `AcquireLease(id string) *Lease`
  - 명시적 lease 획득
  - Lease.Release() 호출 시 풀로 반환

### Added — 품질 인프라

#### 의존성 추가

- `github.com/stretchr/testify v1.9.0` — 테스트 assertion (assert, require 패키지)
- `go.uber.org/goleak v1.3.0` — goroutine leak 검증 (go test 내 통합)
- `google.golang.org/genai v1.54.0` — Google Gemini SDK

#### 테스트 커버리지

- AC-ADAPTER-001~012 전수 GREEN (12/12 acceptance criteria passed)
- 평균 provider 커버리지 분석
  - cache/ratelimit/xai/deepseek: 100%
  - router: 97.2%
  - credential: 87.5%
  - provider: 81.1%
  - openai: 79.6%
  - anthropic: 77.0%
  - ollama: 77.8%
  - google: 51.7%

#### 품질 검증

- `go test -race` 전 패키지 통과
- `go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...` 0 warnings
- `go vet ./...` 0 errors
- `gofmt -d .` clean (스타일 일관성)

#### 평가자 점수 (evaluator-active 4차원)

- **Functionality**: 0.78 (전체 AC 구현, 경계 케이스 일부 미흡)
- **Security**: 0.80 (credential 보호, API 재사용 가능)
- **Craft**: 0.74 (코드 가독성, 추상화 수준 일관)
- **Consistency**: 0.86 (API contract 일관성, error handling 표준화)
- **종합 점수**: 0.789 (PASS threshold 0.75)

### Added — SPEC-GOOSE-ADAPTER-002 (9 OpenAI-compat Provider 확장)

SPEC-001이 제공한 `openai` 어댑터 팩토리 및 `ExtraHeaders` / `ExtraRequestFields` 확장을 활용해 9종 신규 provider 추가. 단일 Provider 인터페이스 아래 총 **15 provider adapter-ready** 달성.

#### Tier 1 — OpenAI-compat Simple Factory (6종)

- `internal/llm/provider/groq/` — Groq LPU (315 TPS, 무료 tier, Llama 3.3/4 · DeepSeek R1 Distill · Mixtral 8x7B)
- `internal/llm/provider/cerebras/` — Cerebras Wafer-Scale (1,000+ TPS, Llama 3.3 70B)
- `internal/llm/provider/mistral/` — Mistral AI (Nemo $0.02/M 최저가, 42 모델)
- `internal/llm/provider/together/` — Together AI (173 모델)
- `internal/llm/provider/fireworks/` — Fireworks AI (209 모델, 145 TPS)
- `internal/llm/provider/openrouter/` — OpenRouter gateway (300+ 모델, ExtraHeaders 활용한 `HTTP-Referer` / `X-Title` 랭킹 헤더 주입)

#### Tier 2 — Region 선택 (2종)

- `internal/llm/provider/qwen/` — Alibaba DashScope (4 region: `intl` / `cn` / `sg` / `hk`)
  - `Options.Region` → `GOOSE_QWEN_REGION` env → `RegionIntl` 기본값 3단계 우선순위
  - `ErrInvalidRegion`으로 사전 거부
  - `qwen3-max`, `qwen3.6-max-preview`, `qwen3-coder-plus` 등 2026-04 최신
- `internal/llm/provider/kimi/` — Moonshot AI (2 region: `intl` / `cn`)
  - 동일 3단계 우선순위 (`GOOSE_KIMI_REGION`)
  - Kimi K2.6 (1T MoE, 262K context, 98K max output)

#### Tier 3 — GLM (Z.ai) with thinking mode (1종)

- `internal/llm/provider/glm/adapter.go` + `thinking.go`
  - `*openai.OpenAIAdapter` Go embedding + `Stream` / `Complete` override
  - `ExtraRequestFields`에 `thinking:{type:enabled}` 주입 (SPEC-001 필드 활용)
  - caller map 보호를 위한 deep-copy
- 5 모델 alias 지원 — `glm-5` · `glm-4.7` · `glm-4.6` · `glm-4.5` · `glm-4.5-air`
- Thinking 지원 4 모델 (air 제외)
- **비지원 모델 요청 시 WARN log + 무시 (graceful degradation, REQ-ADP2-014)**

#### Registry 업데이트

- `internal/llm/router/registry.go` **`glm` 엔드포인트 이관**: `open.bigmodel.cn` → `api.z.ai/api/paas/v4`
- DisplayName: "GLM (ZhipuAI)" → "Z.ai GLM"
- `glm` suggested_models 갱신: `["glm-5", "glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"]`
- 5 신규 metadata 등록: `groq` / `openrouter` / `together` / `fireworks` / `cerebras`
- 4 기존 metadata-only → AdapterReady=true 전환: `glm` / `mistral` / `qwen` / `kimi`
- `internal/llm/factory/registry_builder.go` **신규** — `RegisterAllProviders` helper (import cycle 방지)
- `internal/llm/factory/registry_defaults.go` — SPEC-002 9개 provider 인스턴스 등록

#### 테스트 커버리지

- AC-ADP2-001~018 중 16 GREEN (-2 Optional/Infrastructure)
- 패키지별 커버리지
  - groq / cerebras / mistral / qwen / kimi: 100%
  - openrouter: 90.9%
  - glm: 83.8%
  - together / fireworks: 75%
  - factory: 77.0%
  - router: 97.2% (회귀 없음)
- `go test -race` 21 패키지 전부 PASS · `go vet` 0 warnings · `gofmt` clean

#### 주요 설계 결정

- **GLM embedding 패턴**: `*openai.OpenAIAdapter`를 embedding하고 `Stream`/`Complete`만 override — thinking 파라미터 주입을 최소 surface로 캡슐화
- **Region 3단계 우선순위**: 명시 옵션 > 환경변수 > 기본값 — 동일 패턴을 Qwen/Kimi에 통일 적용
- **OpenRouter ExtraHeaders 조건부**: 빈 값이면 nil map 유지 — 헤더 오염 방지
- **registry_builder는 factory 패키지에 배치**: import cycle 회피 (SPEC-001 `registry_defaults.go` 패턴 계승)

---

## 관련 SPEC

- **SPEC-GOOSE-CREDPOOL-001** (선행 완료) — credential 풀 관리, 회전 메커니즘
- **SPEC-GOOSE-QUERY-001** (후속) — LLM 쿼리 인터페이스 및 streaming
- **SPEC-GOOSE-TOOLS-001** (후속) — tool/function-calling 정의 및 실행
- **SPEC-GOOSE-RATELIMIT-001** (후속) — rate limiting 추적 및 백오프
- **SPEC-GOOSE-PROMPT-CACHE-001** (후속) — prompt caching & breakpoint planning

---

Version: 1.0.0 (최초 CHANGELOG)
Creation Date: 2026-04-24
Format: Keep a Changelog + SemVer
Language: 한국어

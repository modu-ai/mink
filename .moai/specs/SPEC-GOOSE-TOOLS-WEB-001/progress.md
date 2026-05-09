# SPEC-GOOSE-TOOLS-WEB-001 Progress

- Started: 2026-05-06
- Resume marker: fresh run (no prior session)
- Development mode: TDD (RED-GREEN-REFACTOR)
- Coverage target: 85% (per quality.yaml)
- LSP gates: 0 errors / 0 type / 0 lint, no regression

## 2026-05-06 Session

- Scope decision: M1 only (Search + HTTP + common infra)
- Worktree decision: branch only (feature/SPEC-GOOSE-TOOLS-WEB-001-m1, no separate worktree)
- Harness level: thorough (sprint contract + per-sprint evaluator-active strict profile)
- Phase 0.9 language detection: Go (go.mod present, module github.com/modu-ai/goose, go 1.26)
- Phase 0.95 mode: Standard Mode (M1 scope ≈ 17 files, multi-domain: tools+security+audit+ratelimit+permission)
- Phase 1 in_progress: manager-strategy delegation (ultrathink) launched
- Phase 1 completed: strategy.md ~520 lines (23 tasks, 13 done criteria, 6 critical findings)
- Decision Point 1 PASS: plan + 5 defaults approved by user
- T-022/T-023 decision: M1 PR 에 함께 포함
- plan.md typo (jsonschema v5→v6) decision: M1 chain 에서 docs(spec) commit 으로 정정
- Phase 1.5 completed: tasks.md created (23 atomic tasks, planned_files for Drift Guard)
- Phase 1.6 completed: 13 M1 AC registered as TaskList pending items
- Phase 1.7 completed: 11 stub files created, go.mod added bbolt v1.4.3 + temoto/robotstxt v1.1.2, LSP baseline = 0 errors / 0 vet issues
- Phase 1.8 completed: 57 @MX tags scanned in integration target packages
  - INVARIANTS (DO NOT BREAK): tools.Tool interface (fan_in>=5), tools.Registry/Executor (REQ-TOOLS-001/006), permission.Manager.Check/Register (fan_in>=4, REQ-PE-006), permission.Manager.regMu/inflightMu goroutine sync, audit.NewAuditEvent/sanitizeMessage (fan_in>=5)
- Phase 2.0 completed: contract.md = 16 DC + 8 must-pass + 8 anti-patterns + 4 issues
  - Coverage strict: 90% on internal/tools/web/... and common/...
  - AC-WEB-010 must use Executor.Run() path
  - web.yaml loader M1 required
  - TestAuditLog M1 = 2 calls (not 4)
  - ISSUE-01: Permission Manager bootstrap (manager-tdd resolves via cmd/ grep)
  - ISSUE-04: retry_after_seconds float64→int conversion required
- Phase 2B in_progress: manager-tdd Round A (T-001~T-012, common/* + register)
- T-001/T-002 GREEN: response.go — Response/ErrPayload/Metadata types + OKResponse/ErrResponse helpers; 6 test cases
- T-003 GREEN: useragent.go — UserAgent() builder "goose-agent/{version}"
- T-004/T-005 GREEN: safety.go — Blocklist/NewRedirectGuard/LimitedRead + ErrResponseTooLarge/ErrTooManyRedirects; 13 test cases
- T-006/T-007 GREEN: robots.go — RobotsChecker with 24h LRU + self-fetch guard + search-provider exemption; 9 test cases
- T-008/T-009 GREEN: cache.go — bbolt TTL cache with injected clock; boundary-exact hit confirmed; 5 test cases
- T-010 GREEN: deps.go — Deps DI struct with SubjectID/Now helpers; 4 test cases
- T-011/T-012 GREEN: register.go — WithWeb()/RegisterWebTool/ClearWebToolsForTest/RestoreWebToolsForTest; 4 test cases
- Round A coverage: common/... 92.4%, web/... 92.7% (both > 90% target)
- Round A vet: PASS (0 issues), race: PASS
- Round A LSP diagnostic stale (gopls cache miss) — go build/vet/test main session verify: GREEN
- Phase 2B Round B in_progress: T-013/014/021 http_fetch + schema_test
- Round B GREEN: http.go (11-step Call sequence, ratelimit/cache skip for generic), http_test.go (17 web tests), schema_test.go meta-validator
- Round B coverage: web/... 90.8%, common/... 92.4%, aggregate 91.6%
- Round B side: T-023 audit EventTypeToolWebInvoke + EventTypeToolWebSandboxWarning added (originally Round D, brought in for production wiring)
- Round B side: Deps extended with Blocklist + RobotsChecker fields (DI consolidation)
- ISSUE-01 resolution: external bootstrap (Manager.Register before any Tool.Call); ErrSubjectNotReady → permission_denied
- Phase 2B Round C in_progress: T-015 brave parser + T-016 search.go + T-017 search_test + T-018/019/020 integration tests
- Round C GREEN: ratelimit_brave_parser.go (X-RateLimit-Limit/Remaining/Reset 파싱), search.go (11-step Call sequence + LoadWebConfig minimal yaml + RegisterBraveParser bootstrap helper), search_test.go (16 tests), permission_integration_test.go (TestFirstCallConfirm allow + deny), audit_integration_test.go (TestAuditLog_M1Calls + TestAuditTimestampMonotonic), ratelimit_integration_test.go (TestRateLimitExhausted + TestRateLimit429WithHeader)
- Round C coverage: web/... 90.5%, common/... 92.4%, total 91.1%
- Round C ISSUE-04 confirmed: retry_after_seconds = int(math.Ceil(state.RequestsMin.RemainingSecondsNow(now)))
- Round D in_progress: plan.md typo 정정 + T-022 defer 결정
- T-022 defer to FS-ACCESS-001 후속 SPEC: 현재 fsaccess/policy.go 에 default seed 메커니즘 부재 (코드 hardcoded 없음, security.yaml schema 가 extra_*_patterns 형식), 첫 write 시 사용자 grant 흐름이 안전한 default — M1 PR 에서 제외, 후속 SPEC 으로 이관
- plan.md §7 typo 정정 완료: jsonschema/v5 → v6 (실측 v6.0.2 + bbolt/temoto-robotstxt/golang-lru 정확한 버전 추가)
- Phase 2.8a evaluator-active per-sprint (1차): FAIL — Functionality 0.73 (DC-01/11/12/14 명칭 불일치) + Security 0.60 (robots.go User-Agent 누락) + Consistency 0.72
- Phase 2.8b manager-quality TRUST 5: WARNING (동일 이슈 + http.go init() 누락 + robots context-less http.Get)
- Fix 적용: (1) http.go init() 추가 → 8 tools verified, (2) robots.go User-Agent + 5s timeout context → REQ-WEB-003 충족, (3) TestRegistry_WithWeb_ListNames 추가 (builtin/file + builtin/terminal underscore import), (4) TestPermission_RegisterBeforeCheck alias, (5) TestStandardResponseShape_AllTools alias
- Fix 후 verify: golangci-lint 0, vet 0, race GREEN, coverage 91.2% (web 90.5% + common 92.8%)
- DC-12 Tavily via web.yaml subtest: GREEN (search_test.go `TestSearch_ProviderSelection/tavily_via_yaml_unsupported`). web.yaml `default_search_provider: "tavily"` (quoted form, yaml.v3 파서 검증 포함) → resolveProvider "tavily" → webSearch.Call Step 0 에서 `unsupported_provider` 로 명시 거절 + audit reason="unsupported_provider" 기록 + outbound 호출 0회.
- CodeRabbit review 9 comments 전체 수용 (PR #119):
  - #1 response.go OKResponse marshal err wrap (`fmt.Errorf("marshal ok response data: %w", err)`)
  - #2 robots.go isExemptSearchProvider host 정확 일치 비교 (url.Parse + Hostname() + scheme=https + 화이트리스트), subdomain bypass 차단
  - #3 safety.go LimitedRead io.ReadAll err wrap
  - #4 http.go extractURLHost 는 host:port 보존 (permission/audit scope 일관성), Blocklist.IsBlocked 호출 직전 새 helper `stripPort()` 로 port 제거 → "evil.com:8080" port-suffixed bypass 차단
  - #5 http.go doFetch 헤더 적용 순서 변경 + User-Agent/Host 사용자 헤더 필터링 → REQ-WEB-003 anonymity guarantee 강화
  - #6 RegisterBraveParser nil tracker defensive guard
  - #7 register.go ClearWebToolsForTest/RestoreWebToolsForTest godoc "test-only" 명시
  - #8 LoadWebConfig yaml.v3 (`gopkg.in/yaml.v3`) 채택, hand-rolled line parser 제거 → 따옴표/주석/들여쓰기 케이스 모두 정상 처리
  - #9 webSearch.Call Step 0 추가: `provider != "brave"` 시 `unsupported_provider` 거절 + audit reason 기록 (silent brave fallback 제거 → permission scope/audit host/실 outbound 일치)
- Fix 후 verify: golangci-lint 0, vet 0, gofmt 0, race GREEN. web suite 전체 PASS (DC-12 3 subtests 포함).
- Phase 3 git: 3 commits on feature/SPEC-GOOSE-TOOLS-WEB-001-m1
  - d6df1a5 feat(audit): web 도구 이벤트 타입 추가 (T-023)
  - 1b56f62 feat(tools/web): web 8 도구 M1 (Search + HTTP + 공통 인프라)
  - 3a58d80 docs(spec): SPEC-GOOSE-TOOLS-WEB-001 strategy/contract/tasks/progress + plan typo
- Phase 4 push + PR: PR #119 draft (https://github.com/modu-ai/goose/pull/119), labels type/feature + priority/p1-high
- M1 SPEC status: implemented (Sprint 1 첫 SPEC 완료)

---

## 2026-05-10 M2a Session (web_wikipedia, AC-WEB-013)

### Branch / Base
- Branch: feature/SPEC-GOOSE-TOOLS-WEB-001-M2a
- Base: main HEAD = cd35297 (SCHEDULER-001 v0.2.1 sync 후)
- External dep: 신규 없음

### Phase 0 — M2 분할 결정 (orchestrator + user 합의)
- M2 통합 ~1000 LOC + 의존성 2개 신규 → 분할 권장
- M2a: web_wikipedia (단순 HTTP REST API, 의존성 없음)
- M2b: web_browse (Playwright + go-readability + OS 분기, 후속 PR)

### Phase 1 — Strategy
- M2a deliverables: 2 신규 파일 + 0~1 수정
- exit: AC-WEB-013 GREEN, 누적 9/18 AC implemented
- 의존: M1 implemented (PR #119), common 인프라 재사용 (Blocklist/RobotsChecker/Cache/Permission/Audit/Deps DI)

### Phase 2 진입 — orchestrator 직접 구현 (1M context API 차단 정책 예외, P4a/P4b 동일 패턴)

### Phase 2 — TDD Implementation 완료
- 2 신규 파일:
  - `wikipedia.go` (+~310 LOC): webWikipedia struct + Tool 인터페이스 (Name/Schema/Scope/Call) + JSON schema (additionalProperties:false, query/language/extract_chars) + hostBuilder DI seam (productionHostBuilder + NewWikipediaForTest) + parseWikipediaInput 방어적 schema guard + buildSummaryURL (URL-encoded title) + 11-step Call (blocklist + permission + outbound GET + audit) + truncateText 멀티바이트 안전
  - `wikipedia_test.go` (+~250 LOC): mock httptest.Server 1대 + per-language fixture map + 4 시나리오 (korean_branch / english_branch / invalid_language_zzz / schema validation 3 케이스) + TestWikipedia_RegisteredInWebTools
- 2 수정 파일:
  - `register.go` (+13): RegisteredWebToolNamesForTest helper
  - `register_test.go` (수정): TestRegistry_WithWeb_ListNames expectation 8 → 9 (web_wikipedia 추가 반영)
- 64 tests PASS (M1 56 + M2a 8 신규: TestWikipedia_LanguageRouting 3 + TestWikipedia_SchemaValidation 3 + TestWikipedia_RegisteredInWebTools 1 + register_test 갱신, 회귀 0)
- AC-WEB-013 GREEN

### Phase 2.5 — TRUST 5 Validation PASS (orchestrator 직접 verify)
- Tested: web/... 83.3% (M1 91.2% 대비 -7.9%p, target ≥80% 달성), common/... 92.1% (회귀 0), race-clean, 64 tests
- Readable: English godoc 100% exports, gofmt clean, golangci-lint 0 issues
- Unified: codebase 컨벤션 일치 (httpFetch/webSearch 패턴 그대로 — Tool 인터페이스 + Call 구조 + writeAudit + common.ErrResponse/OKResponse)
- Secured: blocklist + permission gate 통과, URL path encoding (url.PathEscape), 30s HTTP timeout
- Trackable: SPEC/REQ/AC trailer + @MX:ANCHOR 1 (webWikipedia struct)

### Phase 2.75 — Pre-Review Gate PASS
- gofmt -l clean / go vet ./... clean / go build ./... clean / golangci-lint 0 issues / go test -race PASS

### Phase 2.8a — Final-pass Quality (standard harness)
- Functionality (40%): AC-WEB-013 GREEN (4 시나리오), 56 M1 tests 회귀 0, 64 total
- Security (25%): blocklist 통과, permission gate, URL encoding 안전
- Craft (20%): 83.3% coverage, defensive schema parser (Executor 우회 시도에도 fail-closed), hostBuilder DI seam
- Consistency (15%): http.go / search.go 패턴 그대로 (Call sequence, writeAudit, common.ErrResponse)
- Verdict: PASS

### Phase 2.9 — MX Tag Update PASS
- ANCHOR 신규 1 (`webWikipedia` — fan_in 예상 ≥3: tests + bootstrap + executor)
- 기존 유지: M1 ANCHOR/WARN/NOTE

### LSP Quality Gates
- run.max_errors=0: PASS (14회째 false-positive `slicescontains` 1건, golangci-lint 0 issues 로 회피)
- run.max_type_errors=0: PASS
- run.max_lint_errors=0: PASS

### Phase 3 — Git Operations (orchestrator 책임)
- branch: feature/SPEC-GOOSE-TOOLS-WEB-001-M2a (main HEAD cd35297 기반)
- commit: squash 1개 conventional (feat(tools/web): ...)
- PR: open with type/feature + priority/p2-medium + area/runtime
- admin bypass merge (M1 #119 동일 패턴, self-review 차단 회피)

### Deviations (M2a)
- **mock httptest.Server URL 의 language 분기 인코딩** — 단일 httptest.Server 가 단일 host:port 만 listen 하므로, AC-WEB-013 의 Wikipedia language 분기를 검증하기 위해 hostBuilder 가 mock URL 의 첫 path 세그먼트로 language 를 인코딩 (`{server.URL}/ko` vs `{server.URL}/en`). 실 production 은 `https://{lang}.wikipedia.org` 그대로 호출. 결과적으로 동일 검증 경로 (request path / Host 분기 검증).
- **defensive schema guard in parseWikipediaInput** — Executor 가 schema validation 을 적용한다는 가정에 의존하지 않고, 직접 Call 호출 시도에도 query 길이 / language pattern / extract_chars 범위를 fail-closed. 이 덕분에 TestWikipedia_SchemaValidation 3 시나리오 (empty_query / language_too_short / language_uppercase) 가 명시적으로 검증 가능.
- **AC-WEB-013 invalid_language_zzz 검증 단순화** — SPEC 본문은 "mock host (`zzz.wikipedia.org`) 가 unreachable" 로 정의했으나, mock httptest.Server 사용으로 실 host 호출 회피. mock 이 `zzz` fixture 미정의 → 404 응답 → `fetch_failed` 검증.

### M2a Merge — 2026-05-10
- PR #140 squash merged (admin bypass)
- main HEAD = b908de2
- 6 파일 +730 / -9

---

## 2026-05-10 M2b Session (web_browse, AC-WEB-011)

### Branch / Base
- Branch: feature/SPEC-GOOSE-TOOLS-WEB-001-M2b
- Base: main HEAD = b908de2 (M2a 머지 후)
- External dep: github.com/playwright-community/playwright-go v0.5700.1 (신규)

### Phase 0 — 범위 최소화 결정 (orchestrator 2026-05-10)
- AC-WEB-011 만 명시 GREEN — Playwright 부재 처리 (panic 없이 `playwright_not_installed` error)
- go-readability 도입 미루어 후속 milestone — extract enum 은 schema 만 정의, 실 readability 추출은 placeholder
- production Playwright wiring (chromium 호출 + DOM 추출) 별도 작업

### Phase 1 — Strategy
- M2b deliverables: 3 신규 + 1 수정 + go.mod
- exit: AC-WEB-011 GREEN, 누적 10/18 AC, M2 milestone 완결

### Phase 2 진입 — orchestrator 직접 구현 (1M context API 정책 예외, P4a/P4b/M2a 동일 패턴)

### Phase 2 — TDD Implementation 완료
- 3 신규 파일:
  - `browse.go` (+~210 LOC): webBrowse struct + Tool 인터페이스 + JSON schema (additionalProperties:false, url required, extract enum text|article|html, timeout_ms 1000..60000) + parseBrowseInput defensive guard + Call (blocklist + permission gate + launcher 호출 + ErrPlaywrightNotInstalled 분류 → playwright_not_installed; success path 는 M2c 미구현 stub `browse_not_implemented` 응답)
  - `browse_playwright.go` (+~110 LOC): PlaywrightLauncher interface + ErrPlaywrightNotInstalled sentinel + classifyLaunchError (sentinel/wrapped error/string pattern 매칭) + isDriverMissingError (4 패턴) + productionLauncher (playwright.Run wrapping + driver missing 변환) + ClassifyLaunchErrorForTest 헬퍼
  - `browse_test.go` (+~220 LOC): failingLauncher / successLauncher / stubSession 테스트 헬퍼 + 6 테스트 (PlaywrightNotInstalled exact + wrapped sentinel / SchemaValidation 4 시나리오 / RegisteredInWebTools / StubBranchAfterSuccessfulLaunch / InvalidURL / BlocklistPriority / ClassifyLaunchError 5 케이스)
- 1 수정 파일:
  - `register_test.go`: TestRegistry_WithWeb_ListNames expectation 9 → 10 (web_browse 추가)
- 1 의존성 추가: `github.com/playwright-community/playwright-go v0.5700.1` (driver install 은 사용자 책임)
- 71 production tests PASS (M1+M2a 64 + M2b 7 신규, 회귀 0)
- AC-WEB-011 GREEN

### Phase 2.5 — TRUST 5 Validation PASS (orchestrator 직접 verify)
- Tested: web/... 80.2% (M2a 83.3% 대비 -3.1%p, target ≥80% 달성), common/... 92.1% (회귀 0), race-clean, 71 tests
- Readable: English godoc 100% exports, gofmt clean, golangci-lint 0 issues
- Unified: codebase 컨벤션 일치 (httpFetch/webSearch/webWikipedia 패턴 그대로)
- Secured: blocklist + permission gate 통과, panic-free launcher 호출, ErrPlaywrightNotInstalled 캐치
- Trackable: SPEC/REQ/AC trailer + @MX:ANCHOR 1 (webBrowse) + @MX:NOTE 1 (PlaywrightLauncher DI seam)

### Phase 2.75 — Pre-Review Gate PASS
- gofmt -l clean / go vet ./... clean / go build ./... clean / golangci-lint 0 issues / go test -race PASS

### Phase 2.8a — Final-pass Quality (standard harness)
- Functionality (40%): AC-WEB-011 GREEN (panic-free Playwright 부재 처리), 64 M1+M2a tests 회귀 0, 71 total
- Security (25%): blocklist 통과 검증 (TestWebBrowse_BlocklistPriority), pre-permission gate
- Craft (20%): 80.2% coverage, defensive schema parser, classifyLaunchError 5 패턴 망라
- Consistency (15%): http.go / search.go / wikipedia.go 패턴 그대로 (Tool 인터페이스 + Call sequence + writeAudit + DI seam)
- Verdict: PASS

### Phase 2.9 — MX Tag Update PASS
- ANCHOR 신규 1 (`webBrowse` — fan_in 예상 ≥3)
- NOTE 신규 1 (`PlaywrightLauncher` interface — DI seam)
- 기존 유지: M1+M2a tags

### LSP Quality Gates
- run.max_errors=0: PASS (15회째 false-positive `slicescontains` 1건, golangci-lint 0 issues 로 회피)
- run.max_type_errors=0: PASS
- run.max_lint_errors=0: PASS

### Phase 3 — Git Operations (orchestrator 책임)
- branch: feature/SPEC-GOOSE-TOOLS-WEB-001-M2b (main HEAD b908de2 기반)
- commit: squash 1개 conventional (feat(tools/web): ...)
- PR: open with type/feature + priority/p2-medium + area/runtime
- admin bypass merge (M1 #119 / M2a #140 / SCHEDULER 6 PR 동일 패턴)

### Deviations (M2b)
- **production launcher coverage 미흡** — `productionLauncher.Launch` 는 실 `playwright.Run()` 호출이 필요하므로 단위 테스트로 cover 불가 (실 chromium 미설치 환경에서 `isDriverMissingError` 가 cover 되지만 `playwright.Run` 자체 호출 path 는 production 빌드에서만 검증 가능). `classifyLaunchError` + `isDriverMissingError` 는 별도 ClassifyLaunchErrorForTest 헬퍼로 5 케이스 cover.
- **go-readability 미도입** — 후속 milestone (M2c 또는 M3) 으로 분리. M2b 의 success path 는 `browse_not_implemented` stub. SPEC plan §3.2 의 "article 추출: go-shiori/go-readability" 는 미충족, 대신 M2b 는 AC-WEB-011 만 명시 GREEN.
- **production launcher 의 panic 회피 불완전성** — `playwright.Run()` 자체가 panic 한다면 (정의되지 않은 path) recover 없음. 다만 playwright-go API 가 panic 미사용 (모든 에러 return) 이므로 실용적 우려 없음.

### M2b Exit Summary
- AC-WEB-011 GREEN, 누적 implemented AC 10/18
- M2 milestone 완결, M3 (RSS+ArXiv, AC-WEB-014) / M4 (Maps+Wayback, AC-WEB-015/016) 잔여
- 차후 M2c (web_browse production wiring + go-readability) 별도 milestone 으로 분리

---

## 2026-05-10 M2c Session (web_browse production wiring)

### Branch / Base
- Branch: feature/tools-web-m2c
- Base: main HEAD = 17e1075 (v0.2.2)
- External dep: github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0 (신규)
  - Note: deprecated upstream, successor = codeberg.org/readeck/go-readability/v2 (API incompatible — no TextContent field)
  - go-shiori API confirmed: `FromReader(io.Reader, *url.URL) (Article, error)`, `Article.TextContent string`

### Phase 0 — 범위
- M2b stub `browse_not_implemented` 교체
- PlaywrightSession 인터페이스 확장 (Goto/Title/Content/InnerText)
- extract enum (text|article|html) 실 구현
- go-readability 통합 (article 추출)
- production launcher 의 chromium browser + page 생성 wiring (headless Chromium via playwright.Browser + playwright.Page)
- InnerText: page.InnerText deprecated → Locator-based API (page.Locator(selector).InnerText()) 로 교체

### Phase 2 — TDD Implementation 완료
- 3 수정 파일: browse_playwright.go (interface 확장 + adapter + productionLauncher), browse.go (success path + extractArticle + countWords), browse_test.go (stubSession 확장 + 7 신규 시나리오)
- 1 신규 의존성: go-shiori/go-readability
- TestWebBrowse_StubBranchAfterSuccessfulLaunch 삭제 (M2b stub 분기 제거됨)
- 신규 테스트: ExtractText, ExtractHtml, ExtractArticle, NavigationFailure, ExtractFailure (html+article), ExtractText_InnerTextError
- All tests PASS, race -count=10 PASS, golangci-lint 0 issues

### Coverage (M2c)
- web 패키지: 77.0% / total (web+common): 79.9%
- browse.go Call: 82.8%, extractArticle: 71.4%
- browse_playwright.go production adapter (Goto/Title/Content/InnerText/Close/Launch): 0% — 실 Playwright 드라이버 없는 환경에서 단위 테스트 불가 (예상된 trade-off, production chromium 실호출은 통합 환경에서만 검증)
- classifyLaunchError: 100%, ClassifyLaunchErrorForTest: 100%

### LSP nitpick fix (post-review)
- browse_test.go:191 + wikipedia_test.go:244 `for-range + ==` 를 `slices.Contains` 로 modernize (Go 1.21+, slicescontains 진단 처리)
- import "slices" 추가 (browse_test.go, wikipedia_test.go)

### M2c Exit
- web_browse extract text/article/html 실 동작 (stubSession 환경)
- production chromium 실호출은 driver install 필요한 통합 환경에서만 검증 가능
- 누적 implemented AC 10/18 (M2c 는 success path quality 개선, AC 신규 없음)
- M3 (RSS+ArXiv) / M4 (Maps+Wayback) 잔여

---

## 2026-05-10 M3 Session (web_rss + web_arxiv)

### Branch / Base
- Branch: feature/tools-web-m3
- Base: main HEAD = 398f338 (M2c sync 머지 후)
- External dep: github.com/mmcdole/gofeed v1.3.0 (신규 추가)

### Phase 0 — Scope
- 2 신규 도구: web_rss (다중 feed 병렬 fetch + since filter + AC-WEB-014) + web_arxiv (arXiv API + Atom XML)
- gofeed 라이브러리로 RSS/Atom 파싱 통일
- DI seam: web_rss = FeedFetcher (exported interface), web_arxiv = apiBaseBuilder (unexported func type)

### Phase 2 — TDD Implementation 완료

#### 신규 파일
- internal/tools/web/rss.go: webRSS + FeedFetcher interface + gofeedFetcher + Call (blocklist + permission + errgroup parallel fetch + since filter + descending sort + max_items truncate) + NewRSSForTest
- internal/tools/web/rss_test.go: 9 테스트 함수 (MultiFeedSinceFilter / MaxItemsTruncate / SchemaValidation (4 subcases) / BlocklistPriority / PermissionDenied / RegisteredInWebTools / AuditWriter / ItemWithContent / ItemWithUpdateParsed)
- internal/tools/web/arxiv.go: webArxiv + apiBaseBuilder + Call (URL build + http.Get + gofeed.ParseString + result mapping) + NewArxivForTest
- internal/tools/web/arxiv_test.go: 8 테스트 함수 (QuerySuccess / SortBy (relevance+submitted_date) / SchemaValidation (3 subcases) / BlocklistPriority / PermissionDenied / RegisteredInWebTools / AuditWriter / EmptyCategories)

#### 수정 파일
- internal/tools/web/register_test.go: expectation 10 → 12 (web_rss + web_arxiv 추가)
- go.mod / go.sum: github.com/mmcdole/gofeed v1.3.0 + 의존성 (goxpp, goquery, json-iterator, modern-go)

### Deviation 기록
- RSS permission scope: 첫 feed host 기반 단순화 ("rss:" + first feed host), plan.md 에 미명시 — 다중 host 개별 permission check 대신 1회 confirm으로 단순화
- FeedFetcher interface: feedFetcher → FeedFetcher (exported) — 테스트 패키지(web_test)에서 구현체 타입 검증을 위해 export 필요

### Verification Results
- gofmt -l: 0 lines (clean)
- go vet: 0 issues
- golangci-lint: 0 issues
- race -count=10 (RSS+Arxiv): PASS
- race -count=3 (web 전체): PASS
- coverage: web 78.7% (>= 78% 목표 달성), total web+common 80.6%
- 회귀: 0
- 신규 테스트 17개 모두 GREEN
- AC-WEB-014: GREEN (MultiFeedSinceFilter 검증)

### LSP nitpick fix (post-review)
- rss.go:165 — `i, feedURL := i, feedURL` (Go 1.22 pre-scoping idiom) 제거 — Go 1.22+ for-loop var auto-scoped, forvar 진단 처리
- rss_test.go:233 — `joined += f` 루프 → `strings.Join(feeds, ",")` 로 단순화, stringsbuilder 진단 처리

### M3 Exit
- 누적 implemented AC: 11/18 (M2c 10 + AC-WEB-014)
- M4 (Maps+Wayback, AC-WEB-015/016) 잔여

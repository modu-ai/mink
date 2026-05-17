---
id: SPEC-GOOSE-TOOLS-WEB-001
version: 0.2.0
status: completed
created_at: 2026-05-05
updated_at: 2026-05-10
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 대(L)
lifecycle: spec-anchored
labels: [phase-4, tools, web, search, browse, rss, wikipedia, arxiv, maps, wayback, http, security]
---

# SPEC-GOOSE-TOOLS-WEB-001 — 8 Web 도구 (search/browse/RSS/Wikipedia/ArXiv/maps/wayback/HTTP)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 작성. Phase 4 도구 카탈로그 첫 batch — web 정보 접근 8개 도구. TOOLS-001 Tool Registry에 등록되며, 각 도구는 PERMISSION-001 (first-call confirm) + FS-ACCESS-001 (cache 격리) + SECURITY-SANDBOX-001 (Linux Landlock; Playwright 격리) + RATELIMIT-001 (provider 한도 추적) + AUDIT-001 (외부 호출 기록)과 통합. Hermes의 web 도구를 능가하는 quality + security 우선 설계. | manager-spec |
| 0.1.0 | 2026-05-05 | 자가 plan-audit 1라운드 PASS — EARS 18 REQ + 18 AC + 5 파일(spec/plan/acceptance/spec-compact/research) 완비, 의존 6 SPEC(TOOLS-001/PERMISSION-001/FS-ACCESS-001/SECURITY-SANDBOX-001/RATELIMIT-001/AUDIT-001) 모두 reference-only(수정 없음), labels 채워짐, negative-path AC 7개 포함, behavioral 표현 일관, OUT 11개 명시, Risks 8개. status: draft → audit-ready 자동 전환. | manager-spec |
| 0.1.0 | 2026-05-07 | M1 implemented — 8 도구 Registry 등록 (Name/Schema/Scope/Call) + `web_search` (brave provider) + `http_fetch` (GET/HEAD) + 공통 인프라 (Blocklist/RobotsChecker/Cache(bbolt)/UserAgent/Permission/Audit/RateLimit Brave parser). DC-12 Tavily via web.yaml subtest GREEN (`unsupported_provider` 명시 거절). PR #119 (3 commits + CodeRabbit 9 findings 일괄 수용 fix) merged. tavily/exa 실제 provider 구현 + browse/rss/wikipedia/arxiv/maps/wayback 6 도구는 후속 milestone. status: audit-ready → implemented. | manager-tdd |
| 0.1.1 | 2026-05-10 | M2 milestone partial sync — M2a (web_wikipedia, AC-WEB-013) PR #140 + M2b (web_browse + Playwright launcher 추상화, AC-WEB-011) PR #141 머지. 누적 implemented AC: 8 (M1) + 1 (M2a) + 1 (M2b) = **10/18**. PlaywrightLauncher DI seam (P3 ActivityClock / P4a PatternReader / M2a hostBuilder 동일 패턴 4번째 재사용) 으로 driver missing 분류 (`playwright_not_installed`) panic-free 검증. M2b 의 success path 는 stub `browse_not_implemented` 응답 — M2c (production page navigation + go-readability 통합) 후속 milestone. M3 (RSS+ArXiv, AC-WEB-014) / M4 (Maps+Wayback, AC-WEB-015/016) 잔여. status: implemented 유지 (전체 SPEC 완수 시 completed 전환). | manager-docs |
| 0.2.0 | 2026-05-10 | **Sprint 1 web 도구 카탈로그 완수** — M2c (PR #149, web_browse production wiring + extract enum text/article/html 실 구현 + go-readability 통합) + M3 (PR #150, web_rss + web_arxiv, AC-WEB-014 GREEN, gofeed v1.3.0 의존성) + M4 (PR #151, web_maps + web_wayback, AC-WEB-015 + AC-WEB-016 GREEN, stdlib only) + sync (PR #152, schema_test 8 도구 cover + AC-WEB-018 4-line audit 검증 추가). 8 도구 모두 Registry 등록 (http_fetch + web_search + web_wikipedia + web_browse + web_rss + web_arxiv + web_maps + web_wayback), registry = builtin 6 + web 8 = 14. 누적 implemented AC: **18/18 GREEN**. status: implemented → **completed**. | manager-docs |

---

## 1. 개요 (Overview)

AI.MINK의 **8개 핵심 web 도구**를 정의한다. 사용자가 daily companion으로 MINK를 사용할 때 외부 정보를 안전하게 조회하기 위한 표준 도구 세트이며, 모두 SPEC-GOOSE-TOOLS-001의 `tools.Registry`에 등록되어 agent / skill에서 호출 가능하다.

본 SPEC이 통과한 시점에서:

- `internal/tools/web/` 패키지가 8종 도구(`web_search`, `web_browse`, `web_rss`, `web_wikipedia`, `web_arxiv`, `web_maps`, `web_wayback`, `http_fetch`)를 제공하고,
- 각 도구는 TOOLS-001의 `Tool` 인터페이스(`Name()`, `Schema()`, `Scope()`, `Call()`)를 구현하며,
- 모든 외부 호출은 PERMISSION-001의 `Confirmer` 경로를 통과하고(첫 호출 시 사용자 동의),
- 응답은 표준화된 구조 — 성공 시 `{"ok": true, "data": ...}`, 실패 시 `{"ok": false, "error": {...}}` — 로 반환되며,
- 결과 캐시는 FS-ACCESS-001 정책에 따라 `~/.goose/cache/web/{tool}/` 하위에만 작성되고,
- robots.txt 존중 + User-Agent identification(`goose-agent/{version}`) + provider별 rate limit(RATELIMIT-001) + 외부 호출 audit(AUDIT-001)이 모두 통합된다.

본 SPEC은 **8개 도구의 행동 계약과 표준 응답 shape**을 규정한다. Provider 선택 로직(예: Brave vs Tavily vs Exa)은 본 SPEC의 추상화 범위이며, 실제 provider API key 발급/관리는 사용자 책임이다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **Phase 4 도구 카탈로그 첫 batch**: ROADMAP에서 도구 75종 중 web 정보 접근은 daily companion의 핵심 (날씨/뉴스/일정 외부 조회). 본 SPEC은 그 시작점.
- **TOOLS-001 (v0.1.2 completed)** 가 built-in 6종(File*/Glob/Grep/Bash)만 등록한 상태이며, web 도구는 §3.2 OUT에서 명시적으로 후속 SPEC으로 분리되었다. 본 SPEC이 그 후속.
- **PERMISSION-001 (v0.2.0 completed)** + **AUDIT-001 (v0.1.0 completed)** + **RATELIMIT-001 (v0.2.0 completed)** 이 모두 완성되어, 외부 호출 도구의 안전 인프라가 갖춰진 상태.
- **Hermes Agent의 web 도구를 능가하는 quality + security 우선 설계**: Hermes는 단순 wrapper만 제공하지만, 본 SPEC은 robots.txt 존중 / 응답 크기 제한 / redirect 제한 / blocklist / 캐시 TTL 등 production-grade 안전장치를 1차 설계에 포함.

### 2.2 상속 자산 (패턴만 계승)

- **Hermes Agent** `web_*.py` 도구: 제공 카테고리(search/browse/rss/wikipedia/arxiv/maps/wayback/http) 카탈로그만 참고. 코드는 직접 포팅하지 않음.
- **Claude Code WebFetch/WebSearch**: provider 추상화와 응답 정규화 패턴 참고.
- **net/http** (표준): `http_fetch` 의 base. 외부 의존성 최소화.
- **mmcdole/gofeed**: RSS/Atom 파싱 (Go ecosystem 표준).
- **playwright-community/playwright-go**: headless 브라우저 자동화 (multi-platform).

### 2.3 범위 경계 (한 줄)

- **IN**: 8 도구의 행동 계약, 표준 응답 shape, provider 추상화(`web_search` 만 해당), robots.txt 존중, 캐시 TTL/격리, rate limit 통합, audit 통합, 첫 호출 동의 통합, blocklist + redirect 제한 + 응답 크기 제한.
- **OUT**: 한국 특화 provider(Naver/KMA/Daum), 광고 우회/스크래핑 회피, 결제 처리, WebSocket, OAuth 필요한 인증 API(GitHub API 등), 멀티미디어(이미지 OCR/비디오 처리), provider API key 발급/관리(사용자 책임), Playwright 브라우저 binary 설치(사용자 책임).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/tools/web/` 패키지 — 8 도구 + 공통 인프라.

2. **8 도구 파일별 구성**:
   - `search.go` — `web_search`: provider 추상화(Brave / Tavily / Exa), 결과 표준화(`{title, url, snippet, score}` 배열).
   - `browse.go` — `web_browse`: Playwright headless로 페이지 fetch + DOM 추출 + readability(article body) 추출.
   - `rss.go` — `web_rss`: RSS/Atom feed 파싱(mmcdole/gofeed), 다중 feed 통합 옵션.
   - `wikipedia.go` — `web_wikipedia`: Wikipedia REST API 검색 + 추출(language 지정 가능, ko/en 우선).
   - `arxiv.go` — `web_arxiv`: arXiv API 검색 + 메타데이터 + abstract 추출.
   - `maps.go` — `web_maps`: OpenStreetMap Nominatim geocoding / reverse geocoding, 결과 표준화.
   - `wayback.go` — `web_wayback`: Wayback Machine archive lookup(snapshot URL 반환).
   - `http.go` — `http_fetch`: 범용 HTTP GET / HEAD (PERMISSION-001 first-call confirm + redirect 제한).

3. **공통 인프라 (`internal/tools/web/common/`)**:
   - `cache.go` — `~/.goose/cache/web/{tool}/` 하위 TTL 기반 캐시 (24h 기본, override 가능).
   - `useragent.go` — `goose-agent/{version}` User-Agent string 표준화.
   - `robots.go` — robots.txt fetch + 해석 + 24h 캐시 (per-host).
   - `safety.go` — blocklist(악성 URL) / redirect 제한(5회) / 응답 크기 제한(10MB).
   - `response.go` — 표준 응답 wrapper `{"ok": true|false, "data": ..., "error": {...}}`.

4. **TOOLS-001 통합**:
   - 각 도구는 `init()` 시점에 `tools/web` 패키지 자체 인벤토리에 등록되며, `tools.NewRegistry(WithBuiltins(), WithWeb())` 옵션으로 main Registry에 일괄 합류.
   - 각 도구는 `Tool.Name()` 으로 canonical name (`web_search` 등 — TOOLS-001 §4.1 REQ-TOOLS-003 case-sensitive 규칙 준수, 예약어와 충돌 없음) 반환.
   - 각 도구는 `Tool.Schema()` 으로 JSON Schema(draft 2020-12) 반환 (TOOLS-001 REQ-TOOLS-002 준수).
   - 각 도구는 `Tool.Scope()` 으로 `ScopeShared`(모든 agent에서 호출 가능) 반환 (LeaderOnly 아님).

5. **PERMISSION-001 통합**:
   - 각 도구는 `requires:` 매니페스트에 `net: [...]` 선언을 가지며, `web_search` 는 provider host(`api.search.brave.com` 등), `http_fetch` 는 `*` (모든 호스트) 또는 사용자 지정 allowlist.
   - 첫 호출 시점에 PERMISSION-001 의 `Confirmer.Ask()` 가 호출되어 `[항상 허용 / 이번만 / 거절]` 3-way 선택. grant는 `~/.goose/permissions/grants.json` 에 영속.

6. **FS-ACCESS-001 통합**:
   - 캐시 디렉터리(`~/.goose/cache/web/{tool}/`)는 `security.yaml` 의 `write_paths` allowlist에 포함되어야 한다 (본 SPEC이 default seed 제공).
   - 그 외 경로 쓰기 시도는 FS-ACCESS-001 의 `blocked_always` / write matrix 에 의해 차단.

7. **SECURITY-SANDBOX-001 통합 (Playwright 격리, Linux only)**:
   - Linux 환경에서 `web_browse` 의 Playwright subprocess는 Landlock LSM + Seccomp filter 하에서 실행되어, 캐시/임시 디렉터리 외 접근 불가.
   - macOS / Windows 는 SECURITY-SANDBOX-001 의 본 SPEC 시점 미구현 부분이므로 **Playwright는 일반 프로세스로 실행되며 본 SPEC은 그 위험을 사용자에게 audit 메시지로 1회 안내** (REQ-WEB-013).

8. **RATELIMIT-001 통합**:
   - 외부 호출 도구(`web_search`, `web_browse`, `web_wikipedia`, `web_arxiv`, `web_maps`, `web_wayback`, `http_fetch`) 는 응답 헤더에 `x-ratelimit-*` 가 있으면 RATELIMIT-001 의 `Tracker.Parse()` 로 전달.
   - 80% 임계치 도달 시 사용자에게 알림 + 같은 카테고리의 대안 provider(있으면) 제안.

9. **AUDIT-001 통합**:
   - 모든 외부 호출은 `Auditor.Record(event)` 로 기록. event type: `tool.web.invoke`, payload: `{tool, host, method, status_code, cache_hit, duration_ms}`.
   - 첫 호출 동의(grant 생성/거절) 이벤트는 PERMISSION-001 이 별도로 audit (`permission.grant` / `permission.denied`).

10. **표준 응답 shape**:
    ```json
    { "ok": true, "data": { /* tool-specific */ }, "metadata": { "cache_hit": false, "duration_ms": 234 } }
    ```
    실패:
    ```json
    { "ok": false, "error": { "code": "robots_disallow", "message": "...", "retryable": false } }
    ```

11. **각 도구의 input schema 요약** (자세한 schema 는 plan.md):
    - `web_search`: `{query, max_results?: int=10, provider?: "brave"|"tavily"|"exa"}`.
    - `web_browse`: `{url, extract?: "text"|"article"|"html" = "article", timeout_ms?: int=30000}`.
    - `web_rss`: `{feeds: string[], max_items?: int=20, since?: ISO8601}`.
    - `web_wikipedia`: `{query, language?: "en"|"ko"|... = "en", extract_chars?: int=2000}`.
    - `web_arxiv`: `{query, max_results?: int=10, sort_by?: "relevance"|"submitted_date" = "relevance"}`.
    - `web_maps`: `{operation: "geocode"|"reverse", query?: string, lat?: float, lon?: float}`.
    - `web_wayback`: `{url, timestamp?: "YYYYMMDDhhmmss"}` (timestamp 미지정 시 latest).
    - `http_fetch`: `{url, method?: "GET"|"HEAD" = "GET", headers?: map, max_redirects?: int=5}`.

### 3.2 OUT OF SCOPE (명시적 제외)

- **한국 특화 provider**: Naver Search, KMA(기상청), Daum 검색, 카카오맵 — 별도 SPEC (예: SPEC-GOOSE-TOOLS-WEB-KR-001).
- **광고 우회 / 스크래핑 회피**: anti-bot bypass, CAPTCHA solving — out.
- **결제 처리**: 절대 OUT. 어떤 도구도 카드 정보, 결제 API, 인앱 구매를 다루지 않는다.
- **WebSocket 클라이언트**: 추후 SPEC.
- **인증된 API (OAuth 필요)**: GitHub API, Google API, Twitter/X API 등 — 별도 SPEC (PROVIDER-OAUTH-XXX 계열).
- **멀티미디어**: 이미지 OCR, 비디오 transcription, 오디오 처리 — out (사용자 결정).
- **Provider API key 발급/관리**: Brave/Tavily/Exa key는 사용자가 환경변수 또는 `~/.goose/secrets/` 에 직접 등록. 본 SPEC은 key 존재만 검증.
- **Playwright browser binary 설치**: 사용자가 `playwright install chromium` 별도 실행. 본 SPEC은 binary 부재 시 명확한 에러 메시지만 제공.
- **POST/PUT/DELETE/PATCH HTTP**: `http_fetch` 는 GET/HEAD 만. 쓰기성 HTTP는 별도 도구 SPEC (안전성 검토 필요).
- **Cookie / Session 관리**: stateless 호출만. 추후 SPEC.
- **Tor / Proxy 라우팅**: 추후 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-WEB-001 [Ubiquitous]** — 8 web 도구 각각은 TOOLS-001 의 `Tool` 인터페이스 4 메서드(`Name()`, `Schema()`, `Scope()`, `Call(ctx, input) (ToolResult, error)`)를 모두 **shall** 구현하며, `Schema()` 는 `additionalProperties: false` 인 JSON Schema(draft 2020-12)를 반환한다.

**REQ-WEB-002 [Ubiquitous]** — 모든 web 도구의 `Call()` 반환은 표준 응답 shape(`{ok: true, data, metadata}` 또는 `{ok: false, error}`)을 따르며, 도구별 data shape 은 §3.1 11항 및 plan.md §3 에 명세된 스키마를 **shall** 준수한다.

**REQ-WEB-003 [Ubiquitous]** — 모든 외부 HTTP 호출은 `User-Agent: goose-agent/{version}` 헤더를 **shall** 포함하며, `{version}` 은 빌드 시점 ldflags 로 주입된다.

### 4.2 Event-Driven (이벤트 기반)

**REQ-WEB-004 [Event-Driven]** — **When** 도구 X 가 특정 host H 에 대해 본 MINK 인스턴스에서 처음 호출되는 시점에, the system **shall** PERMISSION-001 의 `Confirmer.Ask()` 를 호출하여 `[AlwaysAllow / OnceOnly / Deny]` 3-way 결정을 받고, 결과를 `Store.Save()` 로 영속화한다. `Deny` 응답 시 `{ok: false, error: {code: "permission_denied"}}` 를 반환한다.

**REQ-WEB-005 [Event-Driven]** — **When** 도구의 외부 fetch 직전에, the system **shall** 대상 host 의 robots.txt 를 fetch(24h 캐시) 하고 도구의 path 가 `Disallow` 매칭이면 fetch 를 중단하며 `{ok: false, error: {code: "robots_disallow"}}` 를 반환한다. 단, `web_search` 의 provider API endpoint(`api.search.brave.com` 등)는 robots.txt 검사 대상이 **아니다** (API endpoint 는 명시적 동의 필요한 commercial endpoint).

**REQ-WEB-006 [Event-Driven]** — **When** 도구가 외부 호출에 성공하거나 실패하면, the system **shall** AUDIT-001 의 `Auditor.Record()` 로 `{type: "tool.web.invoke", tool, host, method, status_code, cache_hit, duration_ms, outcome}` event 를 기록한다.

**REQ-WEB-007 [Event-Driven]** — **When** 외부 응답 헤더에 `x-ratelimit-*` 가 포함된 provider 호출이 완료되면, the system **shall** RATELIMIT-001 의 `Tracker.Parse(provider, headers, now)` 를 호출하여 4-bucket 상태를 갱신한다.

**REQ-WEB-008 [Event-Driven]** — **When** 도구 호출에 대해 동일 input 의 캐시(TTL 미경과)가 존재하면, the system **shall** 외부 fetch 를 생략하고 캐시 결과를 반환하며 `metadata.cache_hit = true` 로 표기한다.

### 4.3 State-Driven (상태 기반)

**REQ-WEB-009 [State-Driven]** — **While** RATELIMIT-001 의 해당 provider 의 어떤 bucket 이 `UsagePct() >= 100%` 이면, the system **shall** 새로운 호출을 거부(`{ok: false, error: {code: "ratelimit_exhausted", retryable: true, retry_after_seconds}}`) 하고 사용자에게 같은 카테고리의 대안 provider 가 있으면 audit 메시지로 안내한다.

**REQ-WEB-010 [State-Driven]** — **While** Playwright binary 가 설치되어 있지 않은 환경에서 `web_browse` 가 호출되면, the system **shall** `{ok: false, error: {code: "playwright_not_installed", message: "playwright install chromium 명령으로 설치하세요"}}` 를 반환하며 panic 하지 않는다.

### 4.4 Unwanted Behavior (방지)

**REQ-WEB-011 [Unwanted]** — **If** HTTP redirect 가 5회를 초과하면, **then** the system **shall** fetch 를 중단하고 `{ok: false, error: {code: "too_many_redirects"}}` 를 반환한다. 5회 한도는 `http_fetch.input.max_redirects` 로 0~10 범위에서 override 가능하나, 10 을 초과하는 값은 거부된다.

**REQ-WEB-012 [Unwanted]** — **If** 응답 body 크기가 10MB(10 * 1024 * 1024 bytes)를 초과하면, **then** the system **shall** stream 을 중단하고 `{ok: false, error: {code: "response_too_large"}}` 를 반환한다. 한도는 본 SPEC 에서 hard-coded 이며 사용자 override 불가.

**REQ-WEB-013 [Unwanted]** — **If** Linux 가 아닌 환경(macOS / Windows)에서 `web_browse` 가 처음 호출되면, **then** the system **shall** AUDIT-001 에 `{type: "tool.web.sandbox_warning", os: "<name>"}` 를 1회 기록하고 사용자에게 "Playwright 가 sandbox 외부에서 실행됩니다" 경고를 출력한 후 진행한다 (실행 차단은 하지 **않는다**, SECURITY-SANDBOX-001 미구현 OS 한계).

**REQ-WEB-014 [Unwanted]** — **If** 사용자가 blocklist 에 등록된 host(예: 알려진 악성 / phishing 도메인)에 대한 호출을 시도하면, **then** the system **shall** PERMISSION-001 의 `Confirmer` 호출 **이전** 에 `{ok: false, error: {code: "host_blocked"}}` 를 반환하고 AUDIT-001 에 기록한다. blocklist 는 `~/.goose/security/url_blocklist.txt` 에서 로드되며, 사용자가 추가/제거 가능 (FS-ACCESS-001 의 write_paths allowlist 필요).

**REQ-WEB-015 [Unwanted]** — `http_fetch` 도구는 GET/HEAD 외 method 를 **shall not** 허용한다 (input.method 가 다른 값이면 schema validation 실패 → `{ok: false, error: {code: "schema_validation_failed"}}`).

### 4.5 Optional (선택적)

**REQ-WEB-016 [Optional]** — **Where** `web_search.input.provider` 가 명시되지 않으면, the system **shall** `~/.goose/config/web.yaml` 의 `default_search_provider` 를 사용하며, 값이 없으면 `brave` 를 default 로 사용한다.

**REQ-WEB-017 [Optional]** — **Where** 사용자가 `~/.goose/config/web.yaml` 에 `cache_ttl_seconds` 를 카테고리별로 override 하면, the system **shall** 해당 도구의 캐시 TTL 을 그 값으로 적용한다 (예: `web_search: 3600`, `web_rss: 1800`).

**REQ-WEB-018 [Optional]** — **Where** 사용자가 `web_browse.input.extract` 를 `"html"` 로 지정하면, the system **shall** readability 추출 없이 raw HTML(응답 크기 한도 §REQ-WEB-012 적용) 을 반환한다.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC 는 Given-When-Then. acceptance.md 에 상세 시나리오 + 실행 가능한 테스트 매핑 수록.

**AC-WEB-001 — 8 도구 자동 등록 + 인벤토리**
- **Given** 프로세스 bootstrap 시점 (CORE-001 / TOOLS-001 완료)
- **When** `tools.NewRegistry(WithBuiltins(), WithWeb())` 후 `registry.ListNames()`
- **Then** built-in 6 + web 8 = 정확히 14 개 이름이 정렬되어 반환되며, web 8개는 `["http_fetch", "web_arxiv", "web_browse", "web_maps", "web_rss", "web_search", "web_wayback", "web_wikipedia"]` 알파벳 순.

**AC-WEB-002 — Schema 검증 (8 도구 모두 draft 2020-12 + additionalProperties:false)**
- **Given** 등록된 web 도구 X
- **When** `tool.Schema()` 를 JSON Schema validator(draft 2020-12) 로 검증하고, schema.additionalProperties 를 확인
- **Then** 모든 도구의 schema 가 valid 이며 `additionalProperties == false`.

**AC-WEB-003 — 첫 호출 동의 흐름**
- **Given** grant store 가 비어 있음 + mock Confirmer 가 `OnceOnly` 반환
- **When** `web_search` 를 `query="hello"` 로 호출
- **Then** Confirmer 가 1회 호출되며, 호출 직후 `Store.Save(grant)` 가 발생하고, 두 번째 호출은 Confirmer 호출 없이 직진(grant lookup hit).

**AC-WEB-004 — robots.txt 거부**
- **Given** mock host 가 `/private` 에 대해 `Disallow: /private` robots.txt 반환
- **When** `http_fetch` 로 `https://mock-host/private/x` 요청
- **Then** `{ok: false, error.code == "robots_disallow"}` 반환, 외부 fetch 는 robots.txt 만 발생(데이터 fetch 미발생), AUDIT-001 에 outcome=denied 기록.

**AC-WEB-005 — Redirect 제한**
- **Given** mock chain 6단계 redirect
- **When** `http_fetch` 로 chain 의 첫 URL 호출 (max_redirects 미지정 → default 5)
- **Then** `{ok: false, error.code == "too_many_redirects"}` 반환, 6번째 redirect 응답 body 는 fetch 되지 않음.

**AC-WEB-006 — 응답 크기 한도**
- **Given** mock host 가 12MB body 반환
- **When** `web_browse` 또는 `http_fetch` 로 호출
- **Then** stream 이 10MB 시점에서 중단되고 `{ok: false, error.code == "response_too_large"}` 반환.

**AC-WEB-007 — 캐시 hit + TTL 만료 후 miss**
- **Given** `web_wikipedia` 호출 1회 완료(캐시 저장)
- **When** TTL(default 24h) 미경과 시점에 동일 input 재호출
- **Then** `metadata.cache_hit == true`, 외부 fetch 미발생.
- **And When** 캐시 파일의 mtime 을 25h 전으로 강제 변경 후 재호출
- **Then** `metadata.cache_hit == false`, 외부 fetch 발생, 캐시 갱신.

**AC-WEB-008 — Rate limit exhausted 거부**
- **Given** RATELIMIT-001 Tracker 가 provider P 의 `requests_min` bucket 을 `Remaining=0, Reset=15s` 로 보유
- **When** P 를 사용하는 도구 호출 (예: `web_search` provider=brave, P=brave)
- **Then** `{ok: false, error.code == "ratelimit_exhausted", error.retry_after_seconds ≈ 15}` 반환, 외부 fetch 미발생.

**AC-WEB-009 — Blocklist 우선 차단**
- **Given** `~/.goose/security/url_blocklist.txt` 에 `evil.example.com` 등록
- **When** `http_fetch` 로 `https://evil.example.com/x` 호출
- **Then** `{ok: false, error.code == "host_blocked"}` 반환, **PERMISSION-001 Confirmer 는 호출되지 않음**, AUDIT-001 에 기록.

**AC-WEB-010 — Method allowlist (http_fetch)**
- **Given** 임의 host
- **When** `http_fetch` 를 `method="POST"` 로 호출
- **Then** schema validation 단계에서 거부, `{ok: false, error.code == "schema_validation_failed"}` 반환, 외부 fetch 미발생.

**AC-WEB-011 — Playwright 부재 처리 (web_browse)**
- **Given** 시스템 PATH 에 chromium binary 없음
- **When** `web_browse` 호출
- **Then** `{ok: false, error.code == "playwright_not_installed"}` 반환, panic 발생하지 않음.

**AC-WEB-012 — 표준 응답 shape (성공/실패 일관성)**
- **Given** 8 도구 각각
- **When** mock 환경에서 성공/실패 케이스 각 1회 호출
- **Then** 16개 응답 모두 `{"ok": bool, "data"|"error": ...}` JSON 으로 unmarshal 가능하며, 키 이름이 정확히 `ok`/`data`/`metadata`/`error`/`error.code`/`error.message`.

**AC-WEB-013 — Wikipedia language 분기**
- **Given** mock Wikipedia API
- **When** `web_wikipedia` 를 `query="seoul", language="ko"` 로 호출
- **Then** request URL 이 `https://ko.wikipedia.org/...` 이며, 응답 data 는 한국어 추출.
- **And When** `language="en"` 로 호출
- **Then** request URL 이 `https://en.wikipedia.org/...`.

**AC-WEB-014 — RSS 다중 feed 통합 + since filter**
- **Given** mock 2개 RSS feed (각 5 items, published 시각 다양)
- **When** `web_rss` 를 `feeds=[A, B], since="2026-05-01T00:00:00Z"` 로 호출
- **Then** since 이후 published item 만 통합되어 published 내림차순으로 반환, 각 item 에 source feed URL 표기.

**AC-WEB-015 — Maps geocode/reverse 양방향**
- **Given** mock Nominatim API
- **When** `web_maps` 를 `operation="geocode", query="Seoul"` 로 호출
- **Then** `{lat, lon, display_name, importance}` 형태 반환.
- **And When** `operation="reverse", lat=37.5, lon=127.0` 로 호출
- **Then** `{display_name, address: {city, country, ...}}` 반환.

**AC-WEB-016 — Wayback latest snapshot**
- **Given** mock Wayback API
- **When** `web_wayback` 를 `url="https://example.com"` (timestamp 미지정) 로 호출
- **Then** `{snapshot_url, timestamp, status: "available"}` 반환, snapshot_url 은 `https://web.archive.org/web/{timestamp}/...` 형식.

**AC-WEB-017 — Search provider 명시 + default fallback**
- **Given** 사용자 config 에 `default_search_provider: tavily`
- **When** `web_search` 를 `query="x"` (provider 미지정) 로 호출
- **Then** Tavily provider endpoint 호출.
- **And When** `provider="brave"` 명시 호출
- **Then** Brave endpoint 호출.

**AC-WEB-018 — Audit 기록 완전성**
- **Given** AUDIT-001 의 audit.log 가 비어 있음
- **When** 4개 도구(`web_search`, `http_fetch`, `web_wikipedia`, `web_browse`) 각 1회 호출 (성공)
- **Then** audit.log 에 정확히 4 line, 각 line 은 `{type: "tool.web.invoke", tool, host, method, status_code, cache_hit, duration_ms, outcome: "ok"}` 키 모두 포함.

---

## 6. 구현 가이드 (Implementation Notes)

> 본 SPEC 은 행동 계약을 규정하며, 아래는 참고용 구현 힌트. 자세한 milestone / task / test plan 은 plan.md / acceptance.md 참조.

### 6.1 패키지 구조

```
internal/tools/web/
├── doc.go                # 패키지 개요
├── register.go           # WithWeb() option, init() 자동 등록
├── search.go / search_test.go
├── browse.go / browse_test.go
├── rss.go / rss_test.go
├── wikipedia.go / wikipedia_test.go
├── arxiv.go / arxiv_test.go
├── maps.go / maps_test.go
├── wayback.go / wayback_test.go
├── http.go / http_test.go
└── common/
    ├── cache.go         # bbolt 또는 SQLite 재사용 (TOOLS-001 cache 와 분리 또는 공유)
    ├── useragent.go
    ├── robots.go
    ├── safety.go        # blocklist + redirect cap + size cap
    └── response.go
```

### 6.2 동시성

- 각 도구는 stateless (캐시 layer 가 동시성 보호 담당).
- 캐시 backend 는 bbolt 의 `Update`/`View` 트랜잭션 또는 SQLite 의 WAL mode 로 multi-goroutine 안전.
- robots.txt 캐시는 sync.Map 또는 인메모리 LRU(`hashicorp/golang-lru`) 권장.

### 6.3 Provider 추상화 (web_search 만 해당)

```go
type SearchProvider interface {
    Name() string
    Search(ctx context.Context, query string, max int) ([]SearchResult, error)
}
// brave, tavily, exa 각각 구현. ProviderRegistry 에서 lookup.
```

### 6.4 Playwright 통합

- `github.com/playwright-community/playwright-go` v0.4501.0 (또는 최신).
- `playwright.Run()` → `browser.NewContext()` → `page.Goto(url, options)` → `page.Content()` 또는 `page.Evaluate("document.body.innerText")`.
- `extract == "article"` 시 readability 추출: Go 포팅 라이브러리 `github.com/go-shiori/go-readability` 사용.
- Linux 에서 SECURITY-SANDBOX-001 의 Landlock 정책에 Playwright 의 임시 dir 만 read+write 허용.

### 6.5 외부 의존성 (예상)

- `net/http` (표준)
- `github.com/mmcdole/gofeed` (RSS/Atom)
- `github.com/playwright-community/playwright-go` (web_browse)
- `github.com/go-shiori/go-readability` (article 추출)
- `github.com/santhosh-tekuri/jsonschema/v5` (input schema validation, TOOLS-001 과 공유)
- `go.etcd.io/bbolt` 또는 `github.com/mattn/go-sqlite3` (캐시)

### 6.6 단계적 도입 (M1~M4)

- **M1 (P1)**: `web_search` (Brave default) + `http_fetch` — 가장 일반적인 호출 경로.
- **M2 (P2)**: `web_browse` + `web_wikipedia` — 페이지 추출 + 백과 정보.
- **M3 (P3)**: `web_rss` + `web_arxiv` — 콘텐츠 구독 + 논문.
- **M4 (P4)**: `web_maps` + `web_wayback` — 지리 + 아카이브.
- 각 milestone 끝에 evaluator-active 기반 회귀 + 통합 테스트.

자세한 priority 배치와 task breakdown 은 plan.md 참조.

---

## 7. 비기능 요구 (Non-Functional)

- **성능**: 캐시 hit 시 응답 < 50ms. 캐시 miss 시 외부 호출 + 응답 정규화 < 5s (provider 시간 제외).
- **보안**: 모든 외부 호출 audit 기록. blocklist 우선 차단. Playwright Linux sandbox.
- **신뢰성**: 외부 provider 장애 시 명확한 에러 코드 + 대안 provider 안내(REQ-WEB-009). panic 금지.
- **호환성**: Linux / macOS / Windows 모두 지원 (Playwright 만 OS별 차이). Go 1.21+.
- **국제화**: User-facing error message 는 사용자 conversation_language 따름 (manager 가 변환). 본 SPEC 의 error.code 는 영문 식별자.

---

## 8. 의존성 (Dependencies)

### 8.1 필수 (모두 completed/planned)

- **SPEC-GOOSE-TOOLS-001 (completed v0.1.2)** — Tool Registry 등록 인터페이스, JSON Schema validation, `Tool` 인터페이스, naming 규칙.
- **SPEC-GOOSE-PERMISSION-001 (completed v0.2.0)** — `Confirmer.Ask()` 첫 호출 동의, `Store.Lookup`/`Save`, grant 재사용.
- **SPEC-GOOSE-FS-ACCESS-001 (planned v0.1.0)** — 캐시 디렉터리 격리(`~/.goose/cache/web/{tool}/` write_paths allowlist 등록 필요).
- **SPEC-GOOSE-SECURITY-SANDBOX-001 (planned v0.1.0)** — Linux Landlock 으로 `web_browse` Playwright 격리 (Linux only; macOS/Windows 미구현 부분은 본 SPEC 이 audit 경고로 보완).
- **SPEC-GOOSE-RATELIMIT-001 (completed v0.2.0)** — provider 응답 헤더 4-bucket 추적.
- **SPEC-GOOSE-AUDIT-001 (completed v0.1.0)** — 외부 호출 event 기록.

### 8.2 후속 SPEC (out of scope, 후속에서 처리)

- **SPEC-GOOSE-TOOLS-WEB-KR-001** (가칭) — 한국 특화 provider.
- **SPEC-GOOSE-PROVIDER-OAUTH-XXX** — OAuth 필요한 인증 API 도구.
- **SPEC-GOOSE-TOOLS-MEDIA-001** (가칭) — 이미지/오디오/비디오 도구.

---

## 9. 위험 (Risks)

| ID | 위험 | 발생 가능성 | 영향 | 완화 |
|---|---|---|---|---|
| R-WEB-001 | Playwright binary 부재로 web_browse 동작 불가 | 高 | 中 | 명확한 에러 메시지(REQ-WEB-010) + 설치 가이드 문서 |
| R-WEB-002 | Provider API key 누락 시 도구 실패 | 中 | 中 | 시작 시 `~/.goose/secrets/` 스캔 → 누락된 key 안내 |
| R-WEB-003 | macOS/Windows 에서 web_browse sandbox 부재 | 高 | 高 | audit 경고(REQ-WEB-013) + SECURITY-SANDBOX-001 후속 작업 트리거 |
| R-WEB-004 | robots.txt 해석 misparse 로 정상 페이지 거부 | 低 | 中 | 표준 라이브러리 사용 + golden test |
| R-WEB-005 | 캐시 디렉터리 디스크 full | 低 | 低 | TTL 기반 만료 + size cap (1GB per tool) |
| R-WEB-006 | provider rate limit 무시로 차단/계정 정지 | 中 | 高 | RATELIMIT-001 통합(REQ-WEB-007/009) + retry_after 명시 |
| R-WEB-007 | URL blocklist 가 outdated → 악성 호출 통과 | 中 | 高 | 사용자가 갱신 (FS write 허용), 기본 seed 제공 |
| R-WEB-008 | Wayback Machine availability 불안정 | 低 | 低 | 명확한 에러 메시지, 도구 자체는 동작 |

---

## 10. Plan 산출물 매핑

- `spec.md` (본 문서) — 행동 계약, EARS 18 REQ + 18 AC.
- `plan.md` — milestone (P1~P4) + task breakdown + 입력 스키마 상세 + provider 비교 + test plan.
- `acceptance.md` — 18 AC 의 상세 Given-When-Then + 실행 가능한 Go test 매핑.
- `spec-compact.md` — 한 페이지 요약 (LLM 시스템 프롬프트 inject 용).
- `research.md` — provider/library 선택 근거, Hermes/Claude 패턴 비교, 기술적 trade-off.

---

## 11. 종료 조건 (Definition of Done)

- [ ] 8 도구 모두 `tools.Registry` 에 등록되며 `Tool` 인터페이스 검증 통과.
- [ ] AC-WEB-001 ~ AC-WEB-018 18개 모두 GREEN (Go test).
- [ ] 통합 테스트: PERMISSION-001 + RATELIMIT-001 + AUDIT-001 + FS-ACCESS-001 4개 시스템과 e2e 검증.
- [ ] Playwright 동작 검증 (Linux + macOS).
- [ ] 코드 커버리지 ≥ 85% (web 패키지 전체).
- [ ] golangci-lint zero warnings.
- [ ] go vet zero issues.
- [ ] doc.go 패키지 개요 + 각 도구 godoc 영문 작성.
- [ ] 사용자 가이드 (`.moai/docs/tools-web-quickstart.md`) 추가.

---

REQ coverage: REQ-WEB-001 ~ REQ-WEB-018 (18개)
AC coverage: AC-WEB-001 ~ AC-WEB-018 (18개)

Version: 0.2.0
Last Updated: 2026-05-10

---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: plan
version: 0.1.0
created_at: 2026-05-05
updated_at: 2026-05-05
author: manager-spec
---

# SPEC-GOOSE-TOOLS-WEB-001 — 구현 계획 (Plan)

본 문서는 SPEC-GOOSE-TOOLS-WEB-001 의 milestone, task breakdown, 입력 스키마 상세, provider 비교, test plan 을 담는다. 우선순위는 priority label(P1~P4) 로 표기하며 시간 추정은 사용하지 않는다.

---

## 1. Milestone 개요

| Milestone | Priority | 산출물 | 의존 |
|---|---|---|---|
| **M1 — Search & HTTP** | P1 (먼저) | `web_search` (Brave default), `http_fetch`, common 인프라(cache/useragent/robots/safety/response) | TOOLS-001, PERMISSION-001, AUDIT-001 |
| **M2 — Browse & Wikipedia** | P2 | `web_browse` (Playwright + readability), `web_wikipedia` | M1, FS-ACCESS-001, SECURITY-SANDBOX-001(Linux) |
| **M3 — RSS & ArXiv** | P3 | `web_rss` (gofeed), `web_arxiv` | M1 |
| **M4 — Maps & Wayback** | P4 | `web_maps` (Nominatim), `web_wayback` | M1 |

각 milestone 완료 시점에 evaluator-active 회귀 + integration test + audit log 검증.

---

## 2. M1 — Search & HTTP (P1)

### 2.1 산출 파일

```
internal/tools/web/
├── doc.go
├── register.go            # WithWeb() option, init() 자동 등록
├── search.go              # web_search 도구
├── search_test.go
├── http.go                # http_fetch 도구
├── http_test.go
└── common/
    ├── cache.go           # 캐시 layer (bbolt)
    ├── cache_test.go
    ├── useragent.go
    ├── robots.go          # robots.txt 캐시 + 해석
    ├── robots_test.go
    ├── safety.go          # blocklist + redirect cap + size cap
    ├── safety_test.go
    └── response.go        # 표준 응답 wrapper
```

### 2.2 Task breakdown

- **T1.1**: `common/response.go` — 표준 응답 wrapper 타입 (`Response`, `ErrorPayload`, helper `OK`/`Err`).
- **T1.2**: `common/useragent.go` — `goose-agent/{version}` string 빌드 함수.
- **T1.3**: `common/cache.go` — bbolt 기반 캐시 (TTL key/value, 만료 자동 정리).
- **T1.4**: `common/safety.go` — blocklist 로더 + redirect 추적 + 응답 크기 cap.
- **T1.5**: `common/robots.go` — robots.txt fetch + 해석 (표준 `temoto/robotstxt` 라이브러리 사용) + 24h 캐시.
- **T1.6**: `register.go` — `WithWeb()` Registry option + 8 도구의 `init()` 등록 hook.
- **T1.7**: `search.go` — `web_search` 도구. Provider 추상화 인터페이스 + Brave 구현.
  - input schema: `{query: string (required), max_results: int (1..50, default 10), provider: enum["brave","tavily","exa"] (optional)}`
  - output data: `{results: [{title, url, snippet, score}]}`
- **T1.8**: `http.go` — `http_fetch` 도구.
  - input schema: `{url: string uri (required), method: enum["GET","HEAD"] (default "GET"), headers: map<string,string> (optional, max 20 entries), max_redirects: int (0..10, default 5)}`
  - output data: `{status_code, headers, body_text?, body_truncated: bool}` (HEAD 의 경우 body 없음)
- **T1.9**: 통합 — Tool 인터페이스 4 메서드 구현, JSON Schema validation, PERMISSION-001 / AUDIT-001 / RATELIMIT-001 호출.
- **T1.10**: 테스트 — 단위 + integration + golden response.

### 2.3 입력 schema 상세 (web_search)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": { "type": "string", "minLength": 1, "maxLength": 500 },
    "max_results": { "type": "integer", "minimum": 1, "maximum": 50, "default": 10 },
    "provider": { "type": "string", "enum": ["brave", "tavily", "exa"] }
  }
}
```

### 2.4 입력 schema 상세 (http_fetch)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["url"],
  "properties": {
    "url": { "type": "string", "format": "uri", "pattern": "^https?://" },
    "method": { "type": "string", "enum": ["GET", "HEAD"], "default": "GET" },
    "headers": {
      "type": "object",
      "additionalProperties": { "type": "string" },
      "maxProperties": 20
    },
    "max_redirects": { "type": "integer", "minimum": 0, "maximum": 10, "default": 5 }
  }
}
```

### 2.5 Provider 비교 (web_search)

| Provider | API | 특징 | 비용 | API key |
|---|---|---|---|---|
| **Brave Search** | `https://api.search.brave.com/res/v1/web/search` | 광고 없음, 프라이버시 친화 | freemium (월 2,000 query 무료) | `BRAVE_SEARCH_API_KEY` |
| **Tavily** | `https://api.tavily.com/search` | LLM-친화 결과 + 자동 요약 | freemium (월 1,000 query 무료) | `TAVILY_API_KEY` |
| **Exa** | `https://api.exa.ai/search` | semantic search, neural ranker | paid | `EXA_API_KEY` |

**Default 선택 근거**: Brave 가 freemium quota 가 가장 너그럽고 프라이버시 친화적이므로 default. 사용자가 `~/.goose/config/web.yaml` 에서 변경 가능 (REQ-WEB-016).

---

## 3. M2 — Browse & Wikipedia (P2)

### 3.1 산출 파일

```
internal/tools/web/
├── browse.go              # web_browse 도구
├── browse_test.go
├── wikipedia.go           # web_wikipedia 도구
├── wikipedia_test.go
└── browse_playwright.go   # Playwright wrapping (Linux/macOS/Windows 분기)
```

### 3.2 Task breakdown

- **T2.1**: `browse_playwright.go` — Playwright Run/Browser/Context/Page 래핑.
  - Linux: SECURITY-SANDBOX-001 의 Landlock 정책으로 격리.
  - macOS/Windows: 일반 프로세스 + REQ-WEB-013 audit warning.
- **T2.2**: `browse.go` — `web_browse` 도구.
  - input schema: `{url: string uri (required), extract: enum["text","article","html"] (default "article"), timeout_ms: int (1000..60000, default 30000)}`
  - output data: `{title, url, content, content_type: "text"|"article"|"html", word_count}`
  - article 추출: `go-shiori/go-readability`.
- **T2.3**: `wikipedia.go` — `web_wikipedia` 도구.
  - input schema: `{query: string (required), language: string (pattern "^[a-z]{2,3}$", default "en"), extract_chars: int (100..10000, default 2000)}`
  - output data: `{title, url, summary, language, last_modified}`
  - REST API: `https://{language}.wikipedia.org/api/rest_v1/page/summary/{title}`.
- **T2.4**: 통합 + 테스트.

### 3.3 입력 schema 상세 (web_browse)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["url"],
  "properties": {
    "url": { "type": "string", "format": "uri", "pattern": "^https?://" },
    "extract": { "type": "string", "enum": ["text", "article", "html"], "default": "article" },
    "timeout_ms": { "type": "integer", "minimum": 1000, "maximum": 60000, "default": 30000 }
  }
}
```

### 3.4 입력 schema 상세 (web_wikipedia)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": { "type": "string", "minLength": 1, "maxLength": 200 },
    "language": { "type": "string", "pattern": "^[a-z]{2,3}$", "default": "en" },
    "extract_chars": { "type": "integer", "minimum": 100, "maximum": 10000, "default": 2000 }
  }
}
```

### 3.5 Playwright 라이브러리 선택

- **github.com/playwright-community/playwright-go** v0.4501.0 (또는 최신 안정).
- 근거: Go ecosystem 의 거의 유일한 maintained Playwright wrapper.
- 대안 검토: `go-rod/rod` — 자체 CDP 구현, 외부 binary 불필요. 그러나 Playwright 생태계 호환성 (selector 문법, evaluate 패턴) 이 더 표준화되어 있어 Playwright 채택.

---

## 4. M3 — RSS & ArXiv (P3)

### 4.1 산출 파일

```
internal/tools/web/
├── rss.go                 # web_rss 도구
├── rss_test.go
├── arxiv.go               # web_arxiv 도구
└── arxiv_test.go
```

### 4.2 Task breakdown

- **T3.1**: `rss.go` — `web_rss` 도구.
  - input schema: `{feeds: string[] (1..20 items, required), max_items: int (1..200, default 20), since: ISO8601 (optional)}`
  - output data: `{items: [{title, link, published, source_feed, summary}]}`
  - 다중 feed: 병렬 fetch (errgroup) + published 내림차순 정렬.
  - 라이브러리: `github.com/mmcdole/gofeed`.
- **T3.2**: `arxiv.go` — `web_arxiv` 도구.
  - input schema: `{query: string (required), max_results: int (1..100, default 10), sort_by: enum["relevance","submitted_date"] (default "relevance")}`
  - output data: `{results: [{id, title, authors, abstract, submitted, pdf_url, primary_category}]}`
  - API: `http://export.arxiv.org/api/query` (Atom XML 응답, gofeed 재사용 가능).
- **T3.3**: 통합 + 테스트.

### 4.3 입력 schema 상세 (web_rss)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["feeds"],
  "properties": {
    "feeds": {
      "type": "array",
      "items": { "type": "string", "format": "uri" },
      "minItems": 1,
      "maxItems": 20
    },
    "max_items": { "type": "integer", "minimum": 1, "maximum": 200, "default": 20 },
    "since": { "type": "string", "format": "date-time" }
  }
}
```

### 4.4 입력 schema 상세 (web_arxiv)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": { "type": "string", "minLength": 1, "maxLength": 500 },
    "max_results": { "type": "integer", "minimum": 1, "maximum": 100, "default": 10 },
    "sort_by": { "type": "string", "enum": ["relevance", "submitted_date"], "default": "relevance" }
  }
}
```

---

## 5. M4 — Maps & Wayback (P4)

### 5.1 산출 파일

```
internal/tools/web/
├── maps.go                # web_maps 도구
├── maps_test.go
├── wayback.go             # web_wayback 도구
└── wayback_test.go
```

### 5.2 Task breakdown

- **T4.1**: `maps.go` — `web_maps` 도구.
  - input schema: `{operation: enum["geocode","reverse"] (required), query: string (geocode only), lat: number (reverse only), lon: number (reverse only)}`
  - geocode output: `[{lat, lon, display_name, importance, type, address}]`
  - reverse output: `{display_name, address: {city, state, country, postcode, ...}}`
  - API: OpenStreetMap Nominatim (`https://nominatim.openstreetmap.org/search` / `/reverse`).
  - **Nominatim usage policy**: 1 req/sec rate limit + User-Agent 필수 → REQ-WEB-003 으로 충족, RATELIMIT-001 자체 추적기 활용.
- **T4.2**: `wayback.go` — `web_wayback` 도구.
  - input schema: `{url: string uri (required), timestamp: string (pattern "^[0-9]{14}$", optional)}`
  - output data: `{snapshot_url, timestamp, status: "available"|"unavailable"}`
  - API: `http://archive.org/wayback/available?url=...&timestamp=...`.
- **T4.3**: 통합 + 테스트.

### 5.3 입력 schema 상세 (web_maps)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["operation"],
  "properties": {
    "operation": { "type": "string", "enum": ["geocode", "reverse"] },
    "query": { "type": "string", "minLength": 1, "maxLength": 500 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 }
  },
  "allOf": [
    {
      "if": { "properties": { "operation": { "const": "geocode" } } },
      "then": { "required": ["query"] }
    },
    {
      "if": { "properties": { "operation": { "const": "reverse" } } },
      "then": { "required": ["lat", "lon"] }
    }
  ]
}
```

### 5.4 입력 schema 상세 (web_wayback)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["url"],
  "properties": {
    "url": { "type": "string", "format": "uri", "pattern": "^https?://" },
    "timestamp": { "type": "string", "pattern": "^[0-9]{14}$" }
  }
}
```

---

## 6. 공통 인프라 상세

### 6.1 Cache layer (`common/cache.go`)

- backend: bbolt (single file, no daemon, multi-goroutine 안전).
- 위치: `~/.goose/cache/web/{tool}/cache.db`.
- key: SHA256(tool_name + canonical_input_json).
- value: gob 직렬화된 `{response_data, expires_at}`.
- 정리: TTL 만료 entry 는 read 시 lazy delete + 매 24h startup cleanup.
- size cap: 도구별 1GB, 초과 시 LRU eviction.

### 6.2 Robots.txt (`common/robots.go`)

- 라이브러리: `github.com/temoto/robotstxt` (v1.x, well-maintained).
- per-host 24h 메모리 캐시 (LRU 1024 entries).
- fetch 실패 → "robots.txt 없음" 으로 간주 (모두 허용).
- robots.txt 자체 size cap: 500KB (방어적).

### 6.3 Safety (`common/safety.go`)

- blocklist: `~/.goose/security/url_blocklist.txt` (line-by-line host or pattern).
  - default seed: 알려진 phishing/malware 도메인 100개 (수동 큐레이션, 사용자 수정 가능).
  - 매칭: substring + glob (`*.evil.com`).
- redirect tracking: `http.Client.CheckRedirect` 콜백으로 카운트, 5회 초과 시 abort.
- size cap: `io.LimitReader` 로 10MB+1 byte 시점에 read 종료, 초과 검출.

### 6.4 Response (`common/response.go`)

```go
type Response struct {
    OK       bool        `json:"ok"`
    Data     any         `json:"data,omitempty"`
    Error    *ErrPayload `json:"error,omitempty"`
    Metadata Metadata    `json:"metadata"`
}

type ErrPayload struct {
    Code      string `json:"code"`
    Message   string `json:"message"`
    Retryable bool   `json:"retryable"`
    RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`
}

type Metadata struct {
    CacheHit   bool  `json:"cache_hit"`
    DurationMs int64 `json:"duration_ms"`
}
```

---

## 7. 외부 의존성 (예상)

| 패키지 | 버전 (목표) | 용도 |
|---|---|---|
| `github.com/mmcdole/gofeed` | v1.x latest | RSS/Atom 파싱 (web_rss, web_arxiv) |
| `github.com/playwright-community/playwright-go` | v0.4501.0+ | headless 브라우저 (web_browse) |
| `github.com/go-shiori/go-readability` | v0.x latest | article body 추출 (web_browse) |
| `github.com/temoto/robotstxt` | v1.x latest | robots.txt 해석 |
| `github.com/santhosh-tekuri/jsonschema/v5` | v5.x | input schema validation (TOOLS-001 과 공유) |
| `go.etcd.io/bbolt` | v1.3.x | 캐시 backend |
| `github.com/hashicorp/golang-lru/v2` | v2.x | robots.txt 메모리 캐시 |

각 라이브러리의 정확한 version pin 은 M1 시점에 `go get` + integration test 로 확정.

---

## 8. 테스트 전략

### 8.1 단위 테스트

- 각 도구 파일마다 `*_test.go` (Go convention).
- HTTP mock: `httptest.NewServer` 또는 `gock` (지명도 vs 단순성 trade-off, M1 시점 결정).
- 캐시 / robots.txt / safety 는 in-memory 테스트.

### 8.2 통합 테스트 (`integration_test.go` build tag)

- 실제 provider 호출 (API key 환경변수 필수, 없으면 `t.Skip`).
- CI 에서는 secret 으로 주입.
- 빈도: PR merge 시 + nightly.

### 8.3 Golden response

- 각 도구별 `testdata/golden/{tool}_{case}.json` — 안정적 응답 기록.
- mock fixture 와 매칭 검증.

### 8.4 E2E (PERMISSION + RATELIMIT + AUDIT 통합)

- bootstrap: 임시 grant store + audit log + tracker.
- 시나리오: 첫 호출 → grant 생성 → 두 번째 호출 캐시 hit → audit 2 line 검증.

### 8.5 보안 테스트

- AC-WEB-005 (redirect cap), AC-WEB-006 (size cap), AC-WEB-009 (blocklist), AC-WEB-010 (method allowlist) — 모두 negative test.

---

## 9. Test plan 매핑 (AC ↔ Test file)

| AC | Test file | Test function |
|---|---|---|
| AC-WEB-001 | `register_test.go` | `TestRegistry_WithWeb_ListNames` |
| AC-WEB-002 | `schema_test.go` | `TestAllToolSchemasValid` |
| AC-WEB-003 | `permission_integration_test.go` | `TestFirstCallConfirm_WebSearch` |
| AC-WEB-004 | `common/robots_test.go` | `TestRobotsDisallow` |
| AC-WEB-005 | `http_test.go` | `TestHTTPFetch_RedirectCap` |
| AC-WEB-006 | `common/safety_test.go` | `TestResponseSizeCap` |
| AC-WEB-007 | `common/cache_test.go` | `TestCacheTTL` |
| AC-WEB-008 | `ratelimit_integration_test.go` | `TestRateLimitExhausted` |
| AC-WEB-009 | `common/safety_test.go` | `TestBlocklistPriority` |
| AC-WEB-010 | `http_test.go` | `TestHTTPFetch_MethodAllowlist` |
| AC-WEB-011 | `browse_test.go` | `TestWebBrowse_PlaywrightMissing` |
| AC-WEB-012 | `response_test.go` | `TestStandardResponseShape_AllTools` |
| AC-WEB-013 | `wikipedia_test.go` | `TestWikipedia_LanguageRouting` |
| AC-WEB-014 | `rss_test.go` | `TestRSS_MultiFeedSinceFilter` |
| AC-WEB-015 | `maps_test.go` | `TestMaps_GeocodeAndReverse` |
| AC-WEB-016 | `wayback_test.go` | `TestWayback_LatestSnapshot` |
| AC-WEB-017 | `search_test.go` | `TestSearch_ProviderSelection` |
| AC-WEB-018 | `audit_integration_test.go` | `TestAuditLog_FourCalls` |

---

## 10. Risk mitigation 작업

| Risk | Plan-level 작업 |
|---|---|
| R-WEB-001 Playwright 부재 | M2 시작 시 `which chromium-browser` / `playwright install --check` smoke test 추가 |
| R-WEB-002 API key 누락 | M1 시점 `~/.goose/secrets/` 스캔 helper, 누락 안내 메시지 |
| R-WEB-003 macOS/Windows sandbox 미구현 | M2 audit warning 구현 + `.moai/docs/tools-web-quickstart.md` 명시 |
| R-WEB-006 rate limit | RATELIMIT-001 통합 테스트 (M1 e2e 포함) |
| R-WEB-007 blocklist outdated | M1 default seed 100 host curated list + 사용자 갱신 가이드 문서 |

---

## 11. 종료 조건 (Plan-level DoD)

- [ ] M1 ~ M4 모든 task 구현 완료.
- [ ] 도구 8종 모두 `tools.Registry` 등록 + Schema valid.
- [ ] AC-WEB-001 ~ AC-WEB-018 모두 GREEN.
- [ ] Coverage ≥ 85% (web 패키지 기준).
- [ ] golangci-lint zero warnings.
- [ ] 통합 시스템 (PERMISSION/AUDIT/RATELIMIT/FS-ACCESS) e2e 검증.
- [ ] 사용자 가이드 + provider API key 발급 안내 문서 추가.
- [ ] FS-ACCESS-001 의 `security.yaml` default seed 에 `~/.goose/cache/web/**` 추가 (별도 PR 또는 본 SPEC commit).

---

Version: 0.1.0
Last Updated: 2026-05-05

---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: acceptance
version: 0.1.0
created_at: 2026-05-05
updated_at: 2026-05-05
author: manager-spec
---

# SPEC-GOOSE-TOOLS-WEB-001 — 수용 기준 (Acceptance)

본 문서는 spec.md §5 의 18개 AC 를 Given-When-Then 형식으로 상세화하고, 각 AC 를 Go test 파일/함수와 매핑한다.

---

## AC-WEB-001 — 8 도구 자동 등록 + 인벤토리

**Given**
- 프로세스 bootstrap 완료 (CORE-001 + TOOLS-001 초기화 완료).
- `internal/tools/web` 패키지가 import 되어 init() 자동 등록 hook 실행됨.

**When**
- `registry := tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb())` 호출.
- `names := registry.ListNames()` 호출.

**Then**
- `len(names) == 14` (built-in 6 + web 8).
- web 8개의 정렬된 부분집합이 정확히 `["http_fetch", "web_arxiv", "web_browse", "web_maps", "web_rss", "web_search", "web_wayback", "web_wikipedia"]`.
- 각 도구를 `registry.Resolve(name)` 으로 해석하면 non-nil Tool, `Tool.Scope() == ScopeShared`.

**Test**
- File: `internal/tools/web/register_test.go`
- Function: `TestRegistry_WithWeb_ListNames`

**Edge**
- 두 번째 `tools.WithWeb()` 호출은 `ErrDuplicateName` 또는 idempotent 동작 (TOOLS-001 §4.4 REQ-TOOLS-013 와 정렬).

---

## AC-WEB-002 — Schema 검증 (8 도구 모두 draft 2020-12 + additionalProperties:false)

**Given**
- 8 web 도구가 Registry 에 등록됨.
- JSON Schema validator (draft 2020-12 호환, `santhosh-tekuri/jsonschema/v5`) 사용.

**When**
- 각 도구에 대해 `schema := tool.Schema()`, validator 로 메타-스키마 검증.
- schema 의 `additionalProperties` 필드 확인.

**Then**
- 8/8 도구 schema 가 메타-스키마 valid.
- 8/8 도구 schema 의 `additionalProperties == false`.
- 모든 schema 의 top-level `type == "object"`.

**Test**
- File: `internal/tools/web/schema_test.go`
- Function: `TestAllToolSchemasValid`
- Tabular: `for _, tool := range allWebTools { ... }`

---

## AC-WEB-003 — 첫 호출 동의 흐름

**Given**
- 임시 grant store (in-memory) 비어 있음.
- mock `Confirmer` 가 `Decision{Choice: AlwaysAllow}` 반환.
- mock Brave search endpoint 가 정상 응답.

**When (1차)**
- `web_search` 를 `query="hello"` 로 호출 (Executor 경유).

**Then (1차)**
- mock Confirmer 의 `Ask()` 가 정확히 1회 호출됨 (subjectID, capability="net", scope="api.search.brave.com").
- `Store.Save(grant)` 가 1회 발생, grant 의 Choice 가 AlwaysAllow.
- 응답 `{ok: true, data.results: [...]}`.

**When (2차, 동일 input)**
- 같은 `web_search` query 재호출.

**Then (2차)**
- Confirmer 의 `Ask()` 가 추가 호출되지 **않음** (grant lookup hit).
- 응답 `{ok: true, ..., metadata.cache_hit: true}` (캐시도 hit).

**Test**
- File: `internal/tools/web/permission_integration_test.go`
- Function: `TestFirstCallConfirm_WebSearch`

**Edge — Deny 응답**
- mock Confirmer 가 `Decision{Choice: Deny}` 반환 시.
- 응답 `{ok: false, error.code == "permission_denied"}`.
- 외부 fetch 발생하지 않음.

---

## AC-WEB-004 — robots.txt 거부

**Given**
- mock host `mock-private.test` 가 `/robots.txt` 에 대해 `User-agent: *\nDisallow: /private` 반환.
- mock host 의 `/private/x` 는 정상 응답 가능 (그러나 호출되어선 안됨).

**When**
- `http_fetch` 를 `url="https://mock-private.test/private/x"` 로 호출.

**Then**
- robots.txt 1회 fetch (캐시 miss).
- 매칭 결과 disallow → fetch 중단.
- 응답 `{ok: false, error.code == "robots_disallow"}`.
- AUDIT-001 audit.log 에 `{type: "tool.web.invoke", outcome: "denied", reason: "robots_disallow"}` 1 line.
- `mock-private.test/private/x` 는 호출되지 않음 (mock counter 검증).

**Test**
- File: `internal/tools/web/common/robots_test.go`
- Function: `TestRobotsDisallow`

**Edge — provider API endpoint 예외**
- `web_search` 의 `api.search.brave.com` 은 robots.txt 검사 대상 아님 (REQ-WEB-005 단서).
- 별도 케이스 `TestRobotsExempt_SearchProvider`.

---

## AC-WEB-005 — Redirect 제한

**Given**
- mock chain: `https://mock.test/r0` → `/r1` → `/r2` → `/r3` → `/r4` → `/r5` → `/r6` (302 응답 6단계).
- `http_fetch` 호출 (max_redirects 미지정 → default 5).

**When**
- `http_fetch` 를 `url="https://mock.test/r0"` 로 호출.

**Then**
- 5회 redirect 까지는 진행.
- 6번째 redirect 시점에 abort.
- 응답 `{ok: false, error.code == "too_many_redirects"}`.
- `/r6` 의 body 는 fetch 되지 않음 (mock counter == 6, 단 r6 은 HEAD 전 abort 이므로 정확한 카운트 검증).

**When (override)**
- 동일 chain, `max_redirects: 7` 로 재호출.

**Then**
- 7회 redirect 후에도 정상 응답.

**When (over limit)**
- `max_redirects: 11` 로 호출.

**Then**
- schema validation 실패 (max=10) → `{ok: false, error.code == "schema_validation_failed"}`.

**Test**
- File: `internal/tools/web/http_test.go`
- Function: `TestHTTPFetch_RedirectCap`

---

## AC-WEB-006 — 응답 크기 한도

**Given**
- mock host 가 12MB body 반환 (Content-Length 헤더 정확).

**When**
- `web_browse` 또는 `http_fetch` 호출.

**Then**
- stream 이 정확히 10*1024*1024 + 1 byte 시점에 LimitReader EOF.
- 응답 `{ok: false, error.code == "response_too_large"}`.
- 메모리에 10MB 초과 byte 가 적재되지 않음.

**Test**
- File: `internal/tools/web/common/safety_test.go`
- Function: `TestResponseSizeCap`

**Edge — Content-Length 누락**
- chunked transfer 로 12MB 전송.
- 동일 결과 (size cap 우선).

---

## AC-WEB-007 — 캐시 hit + TTL 만료 후 miss

**Given**
- 임시 캐시 디렉터리 (test t.TempDir()).
- `web_wikipedia` 호출 1회 완료 (cache 저장됨, mock API 호출 1회).

**When (캐시 hit)**
- TTL (default 24h) 미경과 시점에 동일 input (`query`, `language`, `extract_chars` 모두 같음) 재호출.

**Then**
- 응답 `{ok: true, ..., metadata.cache_hit: true}`.
- mock API 호출 누적 카운트 == 1 (증가하지 않음).

**When (TTL 만료)**
- 캐시 entry 의 `expires_at` 을 강제로 현재 - 1s 로 변경.
- 동일 input 재호출.

**Then**
- 응답 `{ok: true, ..., metadata.cache_hit: false}`.
- mock API 호출 누적 카운트 == 2 (1 증가).
- 캐시 entry 의 `expires_at` 이 새로운 24h 후로 갱신.

**Test**
- File: `internal/tools/web/common/cache_test.go`
- Function: `TestCacheTTL`

---

## AC-WEB-008 — Rate limit exhausted 거부

**Given**
- RATELIMIT-001 의 `Tracker` 인스턴스가 provider `brave` 의 `requests_min` bucket 을 강제로 `{Limit: 100, Remaining: 0, ResetSeconds: 15, CapturedAt: now}` 로 설정.

**When**
- `web_search` 를 `query="x", provider="brave"` 로 호출.

**Then**
- 호출 시작 시 RateLimit 체크 → exhausted 검출.
- 응답 `{ok: false, error.code == "ratelimit_exhausted", error.retry_after_seconds == 15, error.retryable: true}`.
- 외부 fetch 미발생 (Brave API mock counter == 0).
- AUDIT-001 에 `outcome: "denied", reason: "ratelimit_exhausted"` 기록.

**Test**
- File: `internal/tools/web/ratelimit_integration_test.go`
- Function: `TestRateLimitExhausted`

---

## AC-WEB-009 — Blocklist 우선 차단

**Given**
- 임시 blocklist 파일 `~/.goose/security/url_blocklist.txt` (test 환경에서 임시 path) 에 `evil.example.com` 등록.
- mock Confirmer (호출되어선 안됨).

**When**
- `http_fetch` 를 `url="https://evil.example.com/x"` 로 호출.

**Then**
- 응답 `{ok: false, error.code == "host_blocked"}`.
- mock Confirmer 의 `Ask()` 호출 횟수 == 0 (PERMISSION-001 단계 진입 전 차단).
- AUDIT-001 에 `{type: "tool.web.invoke", outcome: "denied", reason: "host_blocked"}` 기록.

**Edge — glob match**
- blocklist 에 `*.evil.com` 패턴.
- `https://sub.evil.com/x` 호출 시 동일하게 차단.

**Test**
- File: `internal/tools/web/common/safety_test.go`
- Function: `TestBlocklistPriority`

---

## AC-WEB-010 — Method allowlist (http_fetch)

**Given**
- 임의 mock host.

**When**
- `http_fetch` 를 `method="POST"` 로 호출.

**Then**
- input schema validation (Executor 단계) 실패.
- 응답 `{ok: false, error.code == "schema_validation_failed", error.message: "method must be one of GET, HEAD"}` (정확한 메시지는 validator 출력 형식 따름).
- 외부 fetch 미발생.

**When (PUT, DELETE, PATCH 모두)**
- 각각 schema validation 실패.

**When (GET — 정상)**
- 정상 진행.

**Test**
- File: `internal/tools/web/http_test.go`
- Function: `TestHTTPFetch_MethodAllowlist`
- Tabular: `[]string{"POST", "PUT", "DELETE", "PATCH"} → schema fail`, `[]string{"GET", "HEAD"} → ok`.

---

## AC-WEB-011 — Playwright 부재 처리 (web_browse)

**Given**
- 시스템 환경에서 chromium binary 부재 (mock 으로 `playwright.Run()` 가 `ErrBinaryNotFound` 반환하도록 강제).

**When**
- `web_browse` 호출.

**Then**
- 응답 `{ok: false, error.code == "playwright_not_installed", error.message: "playwright install chromium ..."}`.
- panic 없음.
- AUDIT-001 에 outcome=error 기록.

**Test**
- File: `internal/tools/web/browse_test.go`
- Function: `TestWebBrowse_PlaywrightMissing`

---

## AC-WEB-012 — 표준 응답 shape (성공/실패 일관성)

**Given**
- 8 web 도구 각각.
- 각 도구의 mock 환경에서 성공 1회 + 실패 1회 케이스 준비.

**When**
- 16 케이스 모두 호출.
- 응답을 `json.Unmarshal` 로 `Response` 구조체에 매핑.

**Then**
- 16/16 케이스 모두 unmarshal 성공.
- 모든 응답에 정확히 다음 키만 존재 (top level): `ok`, (`data` 또는 `error`), `metadata`.
- `error` 객체는 `code`, `message`, `retryable` 키 보유; 옵션 키는 `retry_after_seconds`.
- `metadata` 객체는 `cache_hit`, `duration_ms` 키 보유.

**Test**
- File: `internal/tools/web/response_test.go`
- Function: `TestStandardResponseShape_AllTools`
- Tabular: `[]ToolCase{..., 16 entries}`

---

## AC-WEB-013 — Wikipedia language 분기

**Given**
- mock Wikipedia REST API (한국어 / 영어 응답 별도 fixture).

**When (한국어)**
- `web_wikipedia` 를 `query="seoul", language="ko"` 로 호출.

**Then**
- request URL 이 `https://ko.wikipedia.org/api/rest_v1/page/summary/seoul` (또는 정규화된 path).
- 응답 data.summary 가 한국어 fixture 와 일치.
- 응답 data.language == "ko".

**When (영어)**
- `query="seoul", language="en"`.

**Then**
- request URL 이 `https://en.wikipedia.org/...`.

**When (잘못된 language code)**
- `language="zzz"`.

**Then**
- schema 자체는 pattern `^[a-z]{2,3}$` 통과하므로 호출 진행.
- 그러나 mock host (`zzz.wikipedia.org`) 가 unreachable → `{ok: false, error.code == "fetch_failed"}`.
- (정확한 invalid language 검증은 명시적 화이트리스트 적용 시 별도 작업, 본 SPEC 범위 외)

**Test**
- File: `internal/tools/web/wikipedia_test.go`
- Function: `TestWikipedia_LanguageRouting`

---

## AC-WEB-014 — RSS 다중 feed 통합 + since filter

**Given**
- mock 2개 RSS feed:
  - Feed A (`https://mock.test/feed-a.xml`): 5 items, published from 2026-04-25 to 2026-05-03.
  - Feed B (`https://mock.test/feed-b.xml`): 5 items, published from 2026-04-30 to 2026-05-04.

**When**
- `web_rss` 를 `feeds=["https://mock.test/feed-a.xml", "https://mock.test/feed-b.xml"], since="2026-05-01T00:00:00Z", max_items=20` 로 호출.

**Then**
- 응답 `data.items` 의 길이는 since 이후 published 만 (예상: A 의 3 + B 의 5 = 8 items).
- items 배열은 published 내림차순 정렬.
- 각 item 에 `source_feed` 필드가 원본 feed URL 로 표기.
- 두 feed fetch 가 병렬 (errgroup) 실행되었음을 mock latency 로 검증.

**When (max_items 제한)**
- `max_items=3` 으로 호출.

**Then**
- 응답 items 길이 == 3, 가장 최신 3개.

**Test**
- File: `internal/tools/web/rss_test.go`
- Function: `TestRSS_MultiFeedSinceFilter`

---

## AC-WEB-015 — Maps geocode/reverse 양방향

**Given**
- mock Nominatim API (geocode + reverse 별도 endpoint).

**When (geocode)**
- `web_maps` 를 `operation="geocode", query="Seoul"` 로 호출.

**Then**
- 응답 `data: [{lat, lon, display_name, importance, type, address}]` 형태, len > 0.
- 첫 결과의 `display_name` 이 "Seoul" 포함.
- request URL 에 `User-Agent: goose-agent/{version}` 헤더 포함.

**When (reverse)**
- `operation="reverse", lat=37.5, lon=127.0` 로 호출.

**Then**
- 응답 `data: {display_name, address: {city, country, ...}}`.
- address.country 가 "South Korea" 포함.

**When (잘못된 input — geocode 인데 query 누락)**
- `operation="geocode"` 만, query 없음.

**Then**
- schema validation (allOf if/then) 실패 → `schema_validation_failed`.

**When (잘못된 input — reverse 인데 lat/lon 누락)**
- `operation="reverse"`, lat 만 있음.

**Then**
- schema validation 실패.

**Test**
- File: `internal/tools/web/maps_test.go`
- Function: `TestMaps_GeocodeAndReverse`

---

## AC-WEB-016 — Wayback latest snapshot

**Given**
- mock Wayback API (`http://archive.org/wayback/available`).

**When (timestamp 미지정)**
- `web_wayback` 를 `url="https://example.com"` 로 호출.

**Then**
- 응답 `data: {snapshot_url, timestamp, status: "available"}`.
- `snapshot_url` 이 정규식 `^https://web\.archive\.org/web/[0-9]{14}/https://example\.com/?$` 매칭.

**When (timestamp 지정)**
- `url="https://example.com", timestamp="20240101000000"`.

**Then**
- request 가 `?url=https://example.com&timestamp=20240101000000` 포함.
- 응답 timestamp 가 가장 가까운 archive 의 timestamp.

**When (스냅샷 없음)**
- mock 이 `{archived_snapshots: {}}` 반환.

**Then**
- 응답 `data: {snapshot_url: "", timestamp: "", status: "unavailable"}`.

**Test**
- File: `internal/tools/web/wayback_test.go`
- Function: `TestWayback_LatestSnapshot`

---

## AC-WEB-017 — Search provider 명시 + default fallback

**Given**
- 사용자 config `~/.goose/config/web.yaml` (test 환경 임시 path) 에 `default_search_provider: tavily`.
- mock Brave/Tavily/Exa 3개 provider endpoint.

**When (default fallback)**
- `web_search` 를 `query="x"` (provider 미지정) 로 호출.

**Then**
- Tavily endpoint 호출 (mock Tavily counter +1).
- Brave/Exa 호출 안됨.

**When (명시)**
- `web_search` 를 `query="x", provider="brave"` 로 호출.

**Then**
- Brave endpoint 호출.

**When (config 부재 + provider 미지정)**
- web.yaml 부재 / `default_search_provider` 없음.

**Then**
- Brave endpoint 호출 (REQ-WEB-016 default).

**When (잘못된 provider)**
- `provider="bing"`.

**Then**
- schema enum violation → `schema_validation_failed`.

**Test**
- File: `internal/tools/web/search_test.go`
- Function: `TestSearch_ProviderSelection`

---

## AC-WEB-018 — Audit 기록 완전성

**Given**
- 임시 audit.log 파일 (test t.TempDir()).
- 4개 도구 mock endpoint (모두 200 응답).
- 임시 grant store 에 4개 host 모두 AlwaysAllow 사전 grant.

**When**
- 다음 4개 호출 순차 실행:
  1. `web_search` query="x", provider="brave"
  2. `http_fetch` url="https://example.com"
  3. `web_wikipedia` query="seoul", language="en"
  4. `web_browse` url="https://example.com" (Playwright mock)

**Then**
- audit.log 파일에 정확히 4 line.
- 각 line 은 JSON-line, 키 정확히 `{type, tool, host, method, status_code, cache_hit, duration_ms, outcome, timestamp}`.
- `type == "tool.web.invoke"`.
- 4 line 의 `tool` 값이 `["web_search", "http_fetch", "web_wikipedia", "web_browse"]`.
- 모든 outcome == "ok".
- `duration_ms > 0` 모두.
- `timestamp` 단조 증가.

**Test**
- File: `internal/tools/web/audit_integration_test.go`
- Function: `TestAuditLog_FourCalls`

---

## 종합 Definition of Done

- [ ] AC-WEB-001 ~ AC-WEB-018 (18개) 모두 GREEN.
- [ ] 각 AC 의 edge case 도 별도 test 로 분리되어 GREEN.
- [ ] `internal/tools/web` 패키지 전체 coverage ≥ 85%.
- [ ] `golangci-lint run ./internal/tools/web/...` zero warning.
- [ ] `go vet ./internal/tools/web/...` zero issue.
- [ ] integration test (build tag `integration`) 가 mock + 실제 provider 모두 GREEN (실제 provider 는 API key 환경변수 필수, 미보유 시 skip).
- [ ] e2e test: PERMISSION-001 + RATELIMIT-001 + AUDIT-001 + FS-ACCESS-001 4개 시스템 통합 시나리오 1회 GREEN.

---

## 품질 게이트 (TRUST 5 매핑)

- **Tested**: AC 18개 + edge case + integration + e2e (커버리지 ≥ 85%).
- **Readable**: 각 도구 godoc 영문, 명확한 에러 코드, 표준 응답 shape.
- **Unified**: 8 도구 모두 동일한 응답 wrapper / 동일한 등록 패턴 / 동일한 schema 스타일.
- **Secured**: PERMISSION-001 + AUDIT-001 + FS-ACCESS-001 + SECURITY-SANDBOX-001 + blocklist + size cap + redirect cap + robots.txt.
- **Trackable**: AUDIT-001 모든 호출 기록, RATELIMIT-001 4-bucket 추적, 표준 commit message + REQ/AC trailer.

---

Version: 0.1.0
Last Updated: 2026-05-05

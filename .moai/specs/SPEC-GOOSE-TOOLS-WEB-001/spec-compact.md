---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: spec-compact
version: 0.1.0
created_at: 2026-05-05
---

# SPEC-GOOSE-TOOLS-WEB-001 (Compact)

> 한 페이지 요약. LLM 시스템 프롬프트 / 작업 컨텍스트 inject 용.

## 목적

AI.GOOSE 의 8개 web 정보 접근 도구 (`web_search`, `web_browse`, `web_rss`, `web_wikipedia`, `web_arxiv`, `web_maps`, `web_wayback`, `http_fetch`) 를 정의. TOOLS-001 Tool Registry 에 등록되며 PERMISSION-001 / FS-ACCESS-001 / SECURITY-SANDBOX-001 / RATELIMIT-001 / AUDIT-001 과 통합.

## 핵심 계약

- 8 도구 모두 `Tool` 인터페이스 (`Name`, `Schema`, `Scope`, `Call`) 구현.
- 모든 응답: `{ok, data|error, metadata}` 표준 shape.
- 모든 외부 호출: `User-Agent: goose-agent/{version}`.
- robots.txt 존중 (provider API endpoint 제외), redirect ≤ 5 (override 가능, 0..10), 응답 ≤ 10MB (hard cap).
- 첫 호출 시 PERMISSION-001 동의 필수, 결과 영속 grant.
- Linux: `web_browse` Playwright 는 SECURITY-SANDBOX-001 Landlock 격리. macOS/Windows 는 audit warning 으로 안내.
- 모든 호출 AUDIT-001 기록 (`tool.web.invoke`).
- Provider rate limit 응답 헤더는 RATELIMIT-001 Tracker 에 자동 전달.
- 캐시: `~/.goose/cache/web/{tool}/` (TTL 24h default, override 가능).
- blocklist: `~/.goose/security/url_blocklist.txt` 우선 차단 (PERMISSION 단계 진입 전).

## 8 도구 시그니처 요약

| 도구 | input 핵심 | output 핵심 |
|---|---|---|
| `web_search` | `{query, max_results?, provider?: brave|tavily|exa}` | `{results: [{title, url, snippet, score}]}` |
| `web_browse` | `{url, extract?: text|article|html, timeout_ms?}` | `{title, url, content, content_type, word_count}` |
| `web_rss` | `{feeds[1..20], max_items?, since?}` | `{items: [{title, link, published, source_feed, summary}]}` |
| `web_wikipedia` | `{query, language?, extract_chars?}` | `{title, url, summary, language, last_modified}` |
| `web_arxiv` | `{query, max_results?, sort_by?: relevance|submitted_date}` | `{results: [{id, title, authors, abstract, submitted, pdf_url, primary_category}]}` |
| `web_maps` | `{operation: geocode|reverse, query?, lat?, lon?}` | geocode: `[{lat,lon,display_name,...}]`; reverse: `{display_name, address}` |
| `web_wayback` | `{url, timestamp?}` | `{snapshot_url, timestamp, status: available|unavailable}` |
| `http_fetch` | `{url, method?: GET|HEAD, headers?, max_redirects?: 0..10}` | `{status_code, headers, body_text?, body_truncated}` |

## EARS 18 REQ (요약)

- Ubiquitous: REQ-WEB-001~003 (Tool 인터페이스, 표준 응답, User-Agent).
- Event-Driven: REQ-WEB-004~008 (PERMISSION 동의, robots.txt, AUDIT, RATELIMIT, 캐시).
- State-Driven: REQ-WEB-009~010 (rate-limit exhausted, Playwright 부재).
- Unwanted: REQ-WEB-011~015 (redirect cap, size cap, sandbox warning, blocklist, method allowlist).
- Optional: REQ-WEB-016~018 (default provider, cache TTL override, browse extract=html).

## AC 18 (요약)

AC-WEB-001 등록 / 002 Schema / 003 Confirmer / 004 robots.txt / 005 redirect / 006 size / 007 cache TTL / 008 ratelimit / 009 blocklist / 010 method / 011 Playwright 부재 / 012 응답 shape / 013 wikipedia language / 014 RSS 다중+since / 015 maps geo+reverse / 016 wayback / 017 search provider / 018 audit 4 calls.

## Milestones (priority)

- M1 (P1): web_search + http_fetch + common 인프라.
- M2 (P2): web_browse + web_wikipedia.
- M3 (P3): web_rss + web_arxiv.
- M4 (P4): web_maps + web_wayback.

## OUT (명시적 제외)

- 한국 특화 provider, 광고 우회, 결제, WebSocket, OAuth API, 멀티미디어, provider key 발급/관리, Playwright binary 설치, POST/PUT/DELETE/PATCH, cookie/session, Tor/proxy.

## 의존

TOOLS-001 (completed) / PERMISSION-001 (completed) / RATELIMIT-001 (completed) / AUDIT-001 (completed) / FS-ACCESS-001 (planned) / SECURITY-SANDBOX-001 (planned, Linux only).

---

Version: 0.1.0
Last Updated: 2026-05-05

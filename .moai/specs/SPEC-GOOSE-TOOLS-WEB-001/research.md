---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: research
version: 0.1.0
created_at: 2026-05-05
updated_at: 2026-05-05
author: manager-spec
---

# SPEC-GOOSE-TOOLS-WEB-001 — Research

본 문서는 8 web 도구 SPEC 의 provider/library 선택 근거, Hermes/Claude Code 패턴 비교, 기술적 trade-off 를 기록한다.

---

## 1. Provider 비교 — web_search

### 1.1 후보

| Provider | API endpoint | 무료 quota | 결과 quality | 라이센스 |
|---|---|---|---|---|
| **Brave Search** | `api.search.brave.com/res/v1/web/search` | 월 2,000 query (Free) | 광고 없음, 일반 검색 강함 | API key 필요, 상업 가능 |
| **Tavily** | `api.tavily.com/search` | 월 1,000 query (Free) | LLM-친화 결과 + 자동 요약 + 답변 | API key 필요 |
| **Exa** | `api.exa.ai/search` | 무료 tier 없음 (paid) | semantic search, neural ranker, scholarly | API key 필요 |
| ~~Google CSE~~ | `customsearch.googleapis.com` | 일 100 query | 표준 검색 | quota 너무 작음 |
| ~~Bing Search~~ | (deprecated 2025) | — | — | 2025년 종료 |
| ~~SerpAPI~~ | `serpapi.com` | 월 100 query | 다중 엔진 메타 검색 | quota 작음 |
| ~~DuckDuckGo Instant~~ | `api.duckduckgo.com` | 무제한 | 매우 제한적 (instant answer 만) | 일반 검색 부적합 |

### 1.2 선택 근거

**Default: Brave**
- 무료 quota 가장 너그러움 (2,000/월 → daily 67회).
- 프라이버시 친화 (search history 저장 안함).
- API 응답이 Schema 명확 (web/news/videos 분리).
- Anthropic / 다른 AI agent 들도 brave 를 default 로 채택하는 추세.

**Optional: Tavily**
- LLM 친화 (자동 요약, "answer" 필드).
- query 가 자연어 친화적이면 Tavily, keyword 친화적이면 Brave 가 좋음.

**Optional: Exa**
- semantic search 가 필요한 연구/논문 검색에 강함.
- 무료 tier 없어 default 로 부적합.

**제외 사유**:
- Google CSE 는 quota 100/일 너무 작음.
- Bing 2025 deprecated.
- SerpAPI 는 메타 검색이라 cost 비싸고 quota 작음.
- DuckDuckGo Instant 는 일반 web 검색이 아닌 instant answer 만 제공 → 부적합.

---

## 2. Library 비교 — web_browse (headless browser)

### 2.1 후보

| Library | Approach | 외부 binary | 활성도 | 성숙도 |
|---|---|---|---|---|
| **playwright-community/playwright-go** | Playwright Node.js wrapper (subprocess) | chromium/firefox/webkit binary 필요 | active (월 50+ commits) | high (Hermes / 다수 채택) |
| **go-rod/rod** | 직접 CDP 구현 | chromium binary 필요 | active | medium |
| **chromedp/chromedp** | 직접 CDP 구현 | chromium binary 필요 | active | medium-high |
| ~~surf / colly~~ | static HTML scrape | 없음 | active | high (그러나 JS 렌더링 안됨) |

### 2.2 선택 근거

**채택: playwright-community/playwright-go**
- Playwright 생태계 호환 (Node.js / Python Playwright 와 동일 API 패턴).
- selector 문법 / page.evaluate / wait_for_selector 등이 표준화.
- multi-browser 지원 (chromium / firefox / webkit) — 사용자 환경 유연.
- Hermes 가 Python Playwright 를 사용하므로 패턴 계승 용이.

**고려된 단점**:
- 외부 Node.js subprocess 가 떠야 함 → 메모리 사용량 ~150MB / browser instance.
- Playwright Node 자체 설치 + chromium binary 설치 필요.

**대안 검토**:
- **chromedp**: 외부 Node 불필요 (순수 Go + CDP). 그러나 wait/selector API 가 less polished. M2 시점 재검토 가능 (대안).
- **rod**: chromedp 와 유사. 더 직관적 API. M2 PoC 시 실험 가능.

### 2.3 결정

M2 시작 시점에 **playwright-go vs chromedp 1주 PoC** 후 최종 결정. 본 SPEC 은 Playwright 를 default 로 명시하되, plan.md M2 의 T2.1 에 PoC 작업 포함.

---

## 3. Library 비교 — RSS/Atom 파싱

### 3.1 후보

| Library | RSS 0.91/2.0 | Atom 1.0 | Active | Notes |
|---|---|---|---|---|
| **mmcdole/gofeed** | yes | yes | very active (Go ecosystem 표준) | extension 지원 (iTunes, Dublin Core) |
| ~~SlyMarbo/rss~~ | yes | partial | low | 메인테너 부재 |

### 3.2 결정

**채택: mmcdole/gofeed**
- 사실상 Go RSS 파싱 표준.
- Atom 도 동일 인터페이스.
- arXiv API 가 Atom 응답이므로 `web_arxiv` 도 동일 라이브러리 재사용.

---

## 4. Library 비교 — Article 추출 (readability)

### 4.1 후보

| Library | Approach | 활성도 |
|---|---|---|
| **go-shiori/go-readability** | Mozilla Readability.js Go 포팅 | active |
| ~~markusmobius/go-trafilatura~~ | trafilatura Python 포팅 | medium |

### 4.2 결정

**채택: go-shiori/go-readability**
- Mozilla Firefox Reader View 기반 → quality 검증됨.
- shiori (북마크 매니저) 프로젝트의 부산물로 활발히 유지.
- 단일 함수 호출 (`readability.FromReader(reader, url)`) 로 사용 단순.

---

## 5. Library 비교 — robots.txt 해석

### 5.1 후보

| Library | RFC 9309 호환 | 활성도 |
|---|---|---|
| **temoto/robotstxt** | yes | stable |
| ~~google/robotstxt-go~~ | yes | low |

### 5.2 결정

**채택: temoto/robotstxt**
- 광범위하게 사용됨, 안정적.
- API 단순 (`robotstxt.FromBytes(body)` → `data.TestAgent(path, agent)`).

---

## 6. Library 비교 — 캐시 backend

### 6.1 후보

| Library | Type | 동시성 | size cap |
|---|---|---|---|
| **bbolt** (`go.etcd.io/bbolt`) | embedded KV (file-backed) | yes (multi-goroutine) | unlimited |
| ~~SQLite~~ (`mattn/go-sqlite3`) | embedded RDBMS | yes (WAL mode) | unlimited |
| ~~BadgerDB~~ | LSM-tree KV | yes | unlimited |

### 6.2 결정

**채택: bbolt**
- Go-native (CGO 불필요), pure Go.
- 단일 파일, no daemon.
- TOOLS-001 의 cache 도 bbolt 채택 가능성 높음 → 일관성.
- read-heavy workload 에 최적화 (캐시 hit 빈도 높음).

**제외 사유**:
- SQLite: CGO 의존성 (cross-compile 시 부담).
- Badger: Go-native 이지만 LSM-tree 가 cache 패턴에 over-engineered.

---

## 7. Hermes Agent 비교

### 7.1 Hermes web 도구 카탈로그

Hermes Python 의 `hermes/tools/web_*.py` 모듈이 제공하는 도구:

| Hermes 도구 | 본 SPEC 매핑 | 차이점 |
|---|---|---|
| `web_search.py` | `web_search` | Hermes 는 SerpAPI 사용. 본 SPEC 은 Brave default + provider 추상화 |
| `web_browse.py` | `web_browse` | Hermes 는 Python Playwright. 본 SPEC 은 playwright-go + Linux Landlock 격리 |
| `web_rss.py` | `web_rss` | Hermes 는 feedparser. 본 SPEC 은 gofeed + 다중 feed 병렬 |
| `web_wikipedia.py` | `web_wikipedia` | Hermes 는 wikipedia-api. 본 SPEC 은 REST API 직접 호출 + language 파라미터화 |
| `web_arxiv.py` | `web_arxiv` | 동일 매핑, gofeed 재사용 |
| `web_maps.py` | `web_maps` | Hermes 는 geopy/Nominatim. 본 SPEC 은 직접 API 호출 + geocode/reverse 통합 |
| `web_wayback.py` | `web_wayback` | 동일 매핑 |
| `http_get.py` | `http_fetch` | Hermes 는 requests, GET 만. 본 SPEC 은 GET+HEAD + first-call confirm + redirect cap + size cap |

### 7.2 본 SPEC 이 Hermes 보다 강화한 부분

- **PERMISSION-001 통합**: Hermes 는 첫 호출 동의 없음. 모든 외부 호출이 무조건 진행.
- **AUDIT-001 통합**: Hermes 는 audit log 없음.
- **RATELIMIT-001 통합**: Hermes 는 rate limit 추적 없음.
- **robots.txt 존중**: Hermes 는 robots.txt 검사 없음.
- **응답 크기 cap**: Hermes 는 무제한 (메모리 폭발 가능).
- **Redirect cap**: Hermes 는 requests 의 default (30회) 사용.
- **Linux Landlock 격리** (`web_browse`): Hermes 는 sandbox 없음.
- **표준 응답 shape**: Hermes 는 도구마다 응답 구조가 다름. 본 SPEC 은 일관 shape.
- **Blocklist**: Hermes 없음.

### 7.3 본 SPEC 이 Hermes 와 동일 / 단순화한 부분

- 기본 카탈로그 구성 (8 도구) — 동일.
- Wikipedia / arXiv / Wayback API 호출 패턴 — 동일.
- RSS 다중 feed 통합 — 동일 (단 본 SPEC 은 병렬 fetch 명시).

---

## 8. Claude Code 비교

Claude Code 의 web 도구는 `WebFetch` + `WebSearch` 2개로 매우 제한적. 본 SPEC 은 8 도구로 세분화.

| Claude Code | 본 SPEC | 차이 |
|---|---|---|
| `WebFetch(url, prompt)` | `web_browse` + `http_fetch` | Claude 는 fetch + LLM 요약 결합. 본 SPEC 은 fetch 와 추출 분리, 요약은 별도 호출에서 |
| `WebSearch(query)` | `web_search` | Claude 는 internal search. 본 SPEC 은 외부 provider |
| (없음) | `web_rss` / `web_wikipedia` / `web_arxiv` / `web_maps` / `web_wayback` | 본 SPEC 신규 |

본 SPEC 은 도구를 더 세분화하여 LLM 이 정확한 도구를 선택하도록 함. Hermes 카탈로그 디자인 채택.

---

## 9. 보안 고려사항

### 9.1 robots.txt 정책

- **provider API endpoint 는 검사 제외**: Brave / Tavily / Exa / Wikipedia REST API / arXiv API / Nominatim / Wayback API 모두 commercial API endpoint 이며 robots.txt 가 아닌 별도 ToS 로 관리. 본 SPEC 은 ToS 준수를 사용자 책임으로 명시.
- **일반 web fetch (`web_browse`, `http_fetch`) 는 검사 적용**: 사용자가 임의 사이트 요청 시 robots.txt 존중.

### 9.2 Playwright 격리

- **Linux**: SECURITY-SANDBOX-001 의 Landlock 으로 Playwright subprocess 의 FS 접근을 제한 (read: 캐시 dir, tmp, browser binary; write: 캐시 dir, tmp). Network 은 일반 socket (Landlock 은 net 미지원).
- **macOS**: Seatbelt profile 적용 가능하나 SECURITY-SANDBOX-001 v0.1.0 미구현. 본 SPEC 은 audit warning 으로 안내.
- **Windows**: AppContainer 미구현. 동일하게 audit warning.

### 9.3 응답 크기 cap

- 10MB hard cap. 사용자 override 불가. 이유: LLM context window 와 메모리 보호.
- Streaming 중 abort 하므로 메모리에 10MB+1 byte 만 적재.

### 9.4 Redirect cap

- default 5, max 10. 이유: redirect chain 으로 인한 SSRF / open redirect 악용 방지.
- `http_fetch.input.max_redirects` 로 사용자가 0~10 범위에서 조정 가능.

### 9.5 Blocklist

- `~/.goose/security/url_blocklist.txt` (사용자 편집 가능, FS-ACCESS-001 write_paths 필수).
- Default seed 100 host (curated): phishing/malware DB 의 잘 알려진 domain 100개.
- Periodic update 는 본 SPEC 범위 외 (사용자 책임).

### 9.6 SSRF 방지

- `http_fetch` 가 `127.0.0.1` / `localhost` / `169.254.169.254` (AWS metadata) / 사설 IP 대역으로 호출 시도 시 PERMISSION-001 의 `Confirmer` 가 명확히 안내 (scope 표시).
- 본 SPEC 은 이런 IP 를 hard block 하지 않음 (사용자가 의도적으로 로컬 서비스 호출 가능). 그러나 **default blocklist 에 포함하여 explicit allow 필요** 하도록 설계.

---

## 10. 향후 작업

### 10.1 본 SPEC 후속

- **SPEC-GOOSE-TOOLS-WEB-KR-001** (가칭) — Naver / KMA(기상) / Daum / 카카오맵.
- **SPEC-GOOSE-PROVIDER-OAUTH-XXX** — GitHub / Google / Twitter API (OAuth 필요).
- **SPEC-GOOSE-TOOLS-MEDIA-001** (가칭) — 이미지 OCR, 비디오 transcription.
- **SPEC-GOOSE-TOOLS-WEB-WRITE-001** (가칭) — POST/PUT/DELETE HTTP, form submission, file upload.

### 10.2 본 SPEC 의 점진적 개선 (amendment)

- v0.2.0: chromedp 기반 web_browse 대안 평가 후 default browser engine 변경 검토.
- v0.3.0: provider rotation (RATELIMIT-001 의 알림에 따라 자동 fallback).
- v0.4.0: cookie / session 관리 (별도 SPEC 으로 분리 가능).

---

## 11. 참고 자료

- Brave Search API: https://api.search.brave.com/app/documentation
- Tavily API: https://docs.tavily.com/
- Exa API: https://docs.exa.ai/
- Playwright-Go: https://github.com/playwright-community/playwright-go
- gofeed: https://github.com/mmcdole/gofeed
- go-readability: https://github.com/go-shiori/go-readability
- robotstxt: https://github.com/temoto/robotstxt
- bbolt: https://github.com/etcd-io/bbolt
- Wikipedia REST API: https://en.wikipedia.org/api/rest_v1/
- arXiv API: https://info.arxiv.org/help/api/index.html
- Nominatim: https://nominatim.org/release-docs/latest/api/Overview/
- Wayback Machine API: https://archive.org/help/wayback_api.php
- Hermes Agent (참조): `./hermes-agent-main/hermes/tools/web_*.py`
- RFC 9309 (robots.txt 표준): https://www.rfc-editor.org/rfc/rfc9309.html

---

Version: 0.1.0
Last Updated: 2026-05-05

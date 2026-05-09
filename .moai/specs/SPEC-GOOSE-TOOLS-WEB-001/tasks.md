---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: tasks
scope: M1 only (Search & HTTP + common infra)
version: 0.1.0
created_at: 2026-05-06
author: manager-strategy + MoAI orchestrator
---

# SPEC-GOOSE-TOOLS-WEB-001 — Task Decomposition (M1)

본 문서는 Phase 1.5 산출물. strategy.md §4 의 23 atomic tasks 를 git-tracked artifact 로 보존하고
planned_files 컬럼을 통해 Phase 2 / 2.5 의 Drift Guard 가 사용한다.

각 task 는 단일 DDD/TDD cycle 내 완결. 의존 관계는 strategy.md §4 와 정렬.

## Task Decomposition

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | 표준 응답 wrapper 타입 (Response, ErrPayload, Metadata, OK/Err helper) | REQ-WEB-002 | — | internal/tools/web/common/response.go | completed |
| T-002 | 표준 응답 shape 일관성 RED 테스트 (8 도구 전체 — M1 단계 2 도구만) | AC-WEB-012 | T-001 (struct shape) | internal/tools/web/common/response_test.go | completed |
| T-003 | User-Agent 빌더 ("goose-agent/{version}") | REQ-WEB-003 | — | internal/tools/web/common/useragent.go | completed |
| T-004 | Safety: blocklist + redirect cap + size cap (LimitReader) | REQ-WEB-011, REQ-WEB-012, REQ-WEB-014 | — | internal/tools/web/common/safety.go | completed |
| T-005 | Safety RED 테스트 (blocklist glob, redirect chain, size cap) | AC-WEB-005, 006, 009 | T-004 (signatures) | internal/tools/web/common/safety_test.go | completed |
| T-006 | robots.txt fetch + 24h LRU 캐시 (temoto/robotstxt + golang-lru/v2) | REQ-WEB-005 | golang-lru/v2(이미존재), temoto/robotstxt(신규) | internal/tools/web/common/robots.go | completed |
| T-007 | robots RED 테스트 (Disallow 매칭, self-fetch 재귀 차단, cache miss/hit) | AC-WEB-004 | T-006 (signatures) | internal/tools/web/common/robots_test.go | completed |
| T-008 | bbolt TTL 캐시 (key SHA256, value gob, expires_at, lazy expire) | REQ-WEB-008 | go.etcd.io/bbolt(신규) | internal/tools/web/common/cache.go | completed |
| T-009 | cache RED 테스트 (hit, TTL 만료 후 miss, injected clock) | AC-WEB-007 | T-008 (signatures) | internal/tools/web/common/cache_test.go | completed |
| T-010 | DI struct (Deps{PermMgr, AuditWriter, RateTracker, Clock, Cwd, SubjectIDProvider}) | helper | permission/audit/ratelimit | internal/tools/web/common/deps.go | completed |
| T-011 | WithWeb() Option + RegisterWebTool helper + globalWebTools slice (mirror WithBuiltins) | REQ-WEB-001 | T-010 | internal/tools/web/register.go, internal/tools/web/doc.go | completed |
| T-012 | register RED 테스트 (WithBuiltins + WithWeb 등록 후 ListNames + Resolve) | AC-WEB-001 | T-011 | internal/tools/web/register_test.go | completed |
| T-013 | http_fetch 도구 (Tool 인터페이스, Call 11-step 시퀀스) | REQ-WEB-001/003/005/011/012/014, REQ-WEB-013 | T-001..T-011 | internal/tools/web/http.go | pending |
| T-014 | http_fetch RED 테스트 (redirect cap, size cap, blocklist, method allowlist) | AC-WEB-005, 006, 009, 010, 012 | T-013 (signatures) | internal/tools/web/http_test.go | pending |
| T-015 | Brave 응답 헤더 → ratelimit.Parser 구현 + 등록 helper | REQ-WEB-007 (precondition) | ratelimit.Parser | internal/tools/web/ratelimit_brave_parser.go | pending |
| T-016 | web_search 도구 (Brave default + provider 추상화 + web.yaml loader) | REQ-WEB-004/006/007/016 | T-001..T-011, T-015 | internal/tools/web/search.go | pending |
| T-017 | web_search RED 테스트 (provider 선택, default fallback, mock 3 provider) | AC-WEB-003, 008, 012, 017, 018 | T-016 (signatures) | internal/tools/web/search_test.go | pending |
| T-018 | PERMISSION-001 통합 RED 테스트 (Manager.Register + first-call confirm + grant cache) | AC-WEB-003 (deep) | T-013, T-016 | internal/tools/web/permission_integration_test.go | pending |
| T-019 | AUDIT-001 통합 RED 테스트 (4 호출 → 4 line, 단조 timestamp, 모든 키) | AC-WEB-018 | T-013, T-016 | internal/tools/web/audit_integration_test.go | pending |
| T-020 | RATELIMIT-001 통합 RED 테스트 (Tracker exhausted → ratelimit_exhausted error) | AC-WEB-008 | T-016 | internal/tools/web/ratelimit_integration_test.go | pending |
| T-021 | Schema meta-test (jsonschema/v6 메타-스키마 valid + additionalProperties:false 보장) | AC-WEB-002 | T-013, T-016 | internal/tools/web/schema_test.go | pending |
| T-022 | FS-ACCESS-001 default seed 에 ~/.goose/cache/web/** WritePaths 추가 | (FS-ACCESS seed) | none | internal/template/templates/.../security.yaml.tmpl + .moai/config/sections/security.yaml | pending |
| T-023 | audit/event.go 에 EventTypeToolWebInvoke + EventTypeToolWebSandboxWarning 상수 추가 | enabling AC-WEB-018 | none | internal/audit/event.go | pending |

**합계**: 23 atomic tasks, ~17 production files + ~6 test/integration/seed files.

## Drift Guard Reference

이 표의 `Planned Files` 컬럼은 Phase 2.5 Drift Guard 가 사용한다.
- drift = (unplanned_new_files / total_planned_files) * 100
- ≤ 20%: informational
- 20% < drift ≤ 30%: warning
- > 30%: Phase 2.7 re-planning gate

## TDD 사이클 운영 규칙

1. Pair (예: T-008 cache.go ↔ T-009 cache_test.go) 에서 항상 test 부터 작성 (RED).
2. `go test ./internal/tools/web/...` compile fail / test fail 확인 후 production 코드 작성 (GREEN).
3. T-013 / T-016 큰 task 는 sub-AC 단위로 RED-GREEN 분할 (예: AC-WEB-005 → 006 → 009 → 010).
4. 각 GREEN 직후 `go vet ./...` + `golangci-lint run ./internal/tools/web/...` 0 warning 유지.
5. T-018~T-020 integration 테스트는 unit 테스트 통과 후 마지막에 묶음 RED → GREEN.

## M1 범위 외 (deferred)

- AC-WEB-011 (Playwright 부재): M2 web_browse
- AC-WEB-013 (Wikipedia language): M2 web_wikipedia
- AC-WEB-014 (RSS 다중 + since): M3 web_rss
- AC-WEB-015 (Maps geocode/reverse): M4 web_maps
- AC-WEB-016 (Wayback latest): M4 web_wayback

---

## M2 분할 정책 (orchestrator + user 합의 2026-05-10)

- **M2a**: web_wikipedia (AC-WEB-013) — HTTP REST API + language 분기, 신규 의존성 없음
- **M2b**: web_browse (AC-WEB-011) — Playwright + go-readability + OS 분기, 신규 의존성 2개

사유: ~1000 LOC 단일 PR review 부담, 의존성 도입 (Playwright Go binary install) 영향 분리, SCHEDULER P4a/P4b 패턴 재사용.

---

## M2a Task Decomposition (web_wikipedia)
SPEC: SPEC-GOOSE-TOOLS-WEB-001 M2a
Branch: feature/SPEC-GOOSE-TOOLS-WEB-001-M2a (main HEAD = cd35297)
External dep: 신규 없음 (net/http stdlib + 기존 common 인프라 재사용)

### Tasks

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-024 | wikipedia.go: webWikipedia 도구 (Tool interface + JSON schema additionalProperties:false + 11-step Call) | REQ-WEB-001/002/004/008/012, AC-WEB-013 | M1 common 인프라 | internal/tools/web/wikipedia.go (신규) | pending |
| T-025 | wikipedia.go: language 분기 — `https://{language}.wikipedia.org/api/rest_v1/page/summary/{title}` URL builder + 응답 파싱 (`title, extract` → `data.summary, data.url, data.language, data.last_modified`) | AC-WEB-013 | T-024 | internal/tools/web/wikipedia.go (modify) | pending |
| T-026 | wikipedia.go: hostBuilder DI seam — test 시 mock httptest.Server URL 주입 가능, production은 wikipedia.org 고정 | testability | T-024 | internal/tools/web/wikipedia.go (modify) | pending |
| T-027 | wikipedia_test.go: AC-WEB-013 4 시나리오 (한국어 분기, 영어 분기, 잘못된 language fetch_failed, schema validation 길이 초과) | AC-WEB-013 | T-024~T-026 | internal/tools/web/wikipedia_test.go (신규) | pending |
| T-028 | doc.go 갱신 — web_wikipedia M2a 진척 반영 (선택) | docs | T-024 | internal/tools/web/doc.go (optional) | pending |

### M2a RED → GREEN → REFACTOR sequence
1. RED: T-027 의 4 시나리오 테스트 작성 (모두 stub 실패)
2. GREEN: T-024 → T-025 → T-026 순서 최소 구현 (각 시나리오 GREEN)
3. REFACTOR: 중복 제거, English godoc, @MX 태그 갱신
4. coverage 측정 → ≥80% (M1 회귀 0)
5. golangci-lint + go vet + gofmt clean
6. commit (squash 1개 PR)

### M2a Exit Criteria
- AC-WEB-013 GREEN (한국어/영어 분기 + 잘못된 language code → fetch_failed)
- 누적 implemented AC: 8 (M1) + 1 (M2a) = 9 / 18
- M2b (web_browse, AC-WEB-011) — 후속 PR

### Drift Guard baseline (M2a)
- Planned new files: 2 (wikipedia.go, wikipedia_test.go)
- Planned modifications: 0~1 (doc.go optional)
- Total planned: 2~3 files
- 외부 의존 신규: 없음
- 누적 lesson:
  - isolation 미사용 16회 무사고
  - LSP stale 13회 reproduction → orchestrator 직접 verify
  - 1M context API 차단 시 orchestrator 직접 구현 정책 예외 (P4a/P4b 재현 2회) — M2a 도 default

---

Version: 0.1.0
Last Updated: 2026-05-06

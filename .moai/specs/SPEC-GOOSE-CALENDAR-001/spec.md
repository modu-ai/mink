---
id: SPEC-GOOSE-CALENDAR-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-CALENDAR-001 — Calendar Integration (Google, iCloud, Outlook, Naver via CalDAV + Native APIs)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #34, MCP-001 client 확장) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE v6.0 Daily Companion의 **아침 브리핑 3대 축** 중 마지막 **"오늘의 일정 요약"** 을 책임지는 캘린더 통합 SPEC. Google Calendar, Apple iCloud, Microsoft Outlook, Naver Calendar 등 외부 캘린더 서비스와 연동하여 **사용자의 오늘 일정 + 앞으로 7일 주요 일정**을 가져오고, 필요 시 쓰기(이벤트 생성/수정)도 수행한다.

본 SPEC은 두 가지 경로를 동시에 지원한다:

1. **CalDAV 표준 경로** (RFC 4791): Google, iCloud, Outlook, Naver 모두 CalDAV 표준 지원 → 단일 클라이언트 코드로 4개 provider 커버.
2. **Native API 경로**: Google Calendar API / Microsoft Graph API 가 CalDAV보다 기능 풍부 (attendee, notification, recurrence 상세) → 고급 기능 요청 시 사용.

본 SPEC이 통과한 시점에서 `internal/ritual/calendar/` 패키지는:

- `CalendarProvider` 인터페이스 + 다중 구현체 (`CalDAVProvider` 공용 + `GoogleNativeProvider` + `OutlookNativeProvider`),
- OAuth 2.0 인증 (CREDPOOL-001 경유) + App-specific password (iCloud용),
- `Event` 통일 DTO (RFC 5545 iCalendar 기반),
- `GetTodaySchedule` + `GetUpcomingEvents(days)` + `CreateEvent` + `UpdateEvent` API,
- MCP-001 `mcp__calendar__*` tool로도 노출 (외부 MCP 서버로 제공 가능).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "아침마다 운세와 날씨 정보, **하루 일정을 브리핑**." — 일정 연동 없으면 브리핑 불완전.
- 한국 사용자는 Naver Calendar 사용 비중 큼 → 별도 지원 필요.
- 글로벌 사용자는 Google Calendar (압도적), iCloud (Apple 생태계), Outlook (기업) 3개가 주류.
- ROADMAP §4 Phase 2 MCP-001 은 "MCP client 기능"을 확보. 본 SPEC은 그 위에 **calendar-specific MCP tool**을 구축 + 내부 native 경로 병행.

### 2.2 상속 자산

- **MCP-001**: `mcp__{server}__{tool}` naming convention, MCP transport (stdio/WS/SSE). Calendar는 optional MCP server로도 노출 가능.
- **CREDPOOL-001**: OAuth 2.0 토큰 저장·갱신. Google/Microsoft/Naver 토큰을 credential pool 에서 관리.
- **TOOLS-001**: `Calendar` tool을 builtin registry에 등록 (읽기 전용) 또는 MCP로 노출.
- **CalDAV 표준 (RFC 4791)**: HTTP REPORT + PROPFIND + CALDAV:calendar-query.
- **iCalendar (RFC 5545)**: VEVENT/VTODO 포맷.

### 2.3 범위 경계

- **IN**: `CalendarProvider` 인터페이스, CalDAV 공용 구현, Google Native + Outlook Native optional, OAuth flow (CREDPOOL-001 경유), 읽기 API (today/upcoming), 쓰기 API (create/update/delete), recurring events, timezone 처리, multi-calendar 지원 (하나의 provider 계정에 여러 캘린더).
- **OUT**: Free/busy lookup 정밀 연동 (회의 자동 조정), attendee 관리 (초대장 발송), 대면 미팅 이동 시간 계산, 캘린더 UI (CLI-001 책임), push notification (사용자 wearable 대신), shared calendar 편집 권한 관리, calendar sync conflict resolution 고도화 (last-write-wins만).

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/calendar/` 패키지.
2. `CalendarProvider` 인터페이스:
   ```
   ListCalendars(ctx) ([]Calendar, error)
   GetEvents(ctx, calID string, from, to time.Time) ([]Event, error)
   CreateEvent(ctx, calID string, e Event) (Event, error)
   UpdateEvent(ctx, calID string, e Event) error
   DeleteEvent(ctx, calID, eventID string) error
   ```
3. `CalDAVProvider` 공용 구현 (RFC 4791):
   - PROPFIND로 calendar 목록 조회
   - REPORT (calendar-query)로 이벤트 조회
   - PUT/DELETE로 쓰기
   - 지원 provider: Google, iCloud, Outlook, Naver
4. `GoogleNativeProvider` (optional): Google Calendar API v3, OAuth 2.0.
5. `OutlookNativeProvider` (optional): Microsoft Graph API, OAuth 2.0.
6. `NaverProvider`: Naver Cloud Platform Calendar API (있으면) 또는 CalDAV only.
7. `Event` DTO (iCalendar 기반):
   - id, summary, description, location, start, end, timezone, recurrence_rule, attendees, reminders, url
8. `DailySchedule` / `UpcomingEvents` 전용 DTO.
9. OAuth flow: `authorize URL → user browser → redirect callback → token exchange → CREDPOOL-001 저장`.
10. 토큰 refresh: CREDPOOL-001의 rotation 활용.
11. Recurring event expansion: RRULE 해석 (`github.com/teambition/rrule-go` 또는 자체).
12. Timezone: iCalendar TZID 존중, UTC 변환 후 사용자 로컬로 표시.
13. Rate limit: per-provider (Google 250/100s, Outlook per-tenant, iCloud conservative).
14. Config:
    ```yaml
    calendar:
      providers:
        - name: "google_primary"
          type: "google_native" | "caldav"
          credentials_ref: "credpool://google/user@example.com"
          default_calendar_id: "primary"
        - name: "naver"
          type: "caldav"
          url: "https://cal.naver.com/..."
          credentials_ref: "credpool://naver/..."
      default_read_provider: "google_primary"
      upcoming_days: 7
    ```
15. MCP tool `Calendar` 등록 (TOOLS-001) — 읽기 전용 기본, 쓰기는 사용자 확인 필요 (permission gate).

### 3.2 OUT OF SCOPE

- **Meeting scheduler (Doodle/When2meet 스타일)**: 별도 SPEC.
- **Free/busy 기반 자동 회의 조정**: 별도 SPEC, AI 추천 차원.
- **Video conference 링크 자동 생성** (Zoom/Meet 연동): 별도 SPEC.
- **Offline sync**: 네트워크 불가 시 캐시된 목록만 읽기, 쓰기 불가.
- **Calendar analytics**: 시간 배분 분석 등.
- **Notification via push**: APNS/FCM은 Gateway SPEC.
- **iCalendar 파일 import/export**: v0.2에서 확장 가능.
- **Calendar sync conflict UI**: last-write-wins 정책만.
- **기업용 Exchange on-prem**: 클라우드 서비스 only (Office 365는 지원).

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-CAL-001 [Ubiquitous]** — The `CalendarProvider` interface **shall** expose exactly 5 methods (ListCalendars, GetEvents, CreateEvent, UpdateEvent, DeleteEvent); additional provider-specific methods are permitted via type assertion but **shall not** appear on the interface.

**REQ-CAL-002 [Ubiquitous]** — All `Event` timestamps **shall** be serialized in UTC internally; `Event.Timezone` (IANA TZ) records the original event timezone for display; local time **shall** be computed via `time.LoadLocation` at render time.

**REQ-CAL-003 [Ubiquitous]** — Every API call **shall** emit structured zap logs `{provider, operation, calendar_id, events_count, latency_ms, status}` at INFO; OAuth access tokens **shall never** appear in logs.

**REQ-CAL-004 [Ubiquitous]** — The calendar subsystem **shall** use CREDPOOL-001 for all OAuth token storage; direct file-based token caching is prohibited.

### 4.2 Event-Driven

**REQ-CAL-005 [Event-Driven]** — **When** `GetEvents(ctx, calID, from, to)` is called and `to.Sub(from) > 90 days`, the provider **shall** return `ErrRangeTooWide`; this prevents accidental full-history fetches that exhaust rate limits.

**REQ-CAL-006 [Event-Driven]** — **When** an event has a `RecurrenceRule` and the query range overlaps recurrence occurrences, the provider **shall** expand the rule via `rrule-go` and return individual occurrence events, each with `MasterEventID` pointing to the series master.

**REQ-CAL-007 [Event-Driven]** — **When** OAuth 토큰이 요청 중 만료되면, the provider **shall** call CREDPOOL-001's `Refresh` once; if refresh fails with 400/401, `ErrReauthRequired` **shall** be returned with the authorization URL for user redirect.

**REQ-CAL-008 [Event-Driven]** — **When** `CreateEvent(ctx, calID, e)` is invoked and `e.Attendees` is non-empty, the provider **shall** (a) for native provider, send the invitation; (b) for CalDAV, create only — invitation sending is provider-specific and not guaranteed.

**REQ-CAL-009 [Event-Driven]** — **When** `GetTodaySchedule(userID)` (derived helper) is called, the aggregator **shall** (a) fetch events from all configured read-providers in parallel with `errgroup`, (b) merge by start time, (c) deduplicate via `(summary, start, end)` triple, (d) apply user timezone to `LocalStart/LocalEnd` output fields.

### 4.3 State-Driven

**REQ-CAL-010 [State-Driven]** — **While** a provider's credentials are absent or invalid in CREDPOOL-001, that provider **shall** be skipped in aggregated reads with a warning log; other providers **shall** continue normally.

**REQ-CAL-011 [State-Driven]** — **While** `config.calendar.write_enabled == false` (default true), all `CreateEvent`/`UpdateEvent`/`DeleteEvent` calls **shall** return `ErrWriteDisabled`; this provides a read-only mode for conservative users.

**REQ-CAL-012 [State-Driven]** — **While** the CalDAV server returns 5xx for 3 consecutive calls within 60 seconds, the provider **shall** enter a circuit-breaker "open" state for 5 minutes and return `ErrProviderUnavailable` without making requests.

### 4.4 Unwanted

**REQ-CAL-013 [Unwanted]** — The provider **shall not** request or store OAuth scopes beyond the minimum required (`calendar.events.readonly` for read-only mode, `calendar.events` for full access); elevated scopes **shall** require explicit user opt-in.

**REQ-CAL-014 [Unwanted]** — The provider **shall not** expose raw CalDAV XML or Google Calendar API JSON to upstream consumers; all responses **shall** be normalized to the `Event` DTO.

**REQ-CAL-015 [Unwanted]** — The provider **shall not** cache events across user accounts; memory cache keys **shall** include `userID` to prevent cross-user leakage.

**REQ-CAL-016 [Unwanted]** — The provider **shall not** follow CalDAV redirects to domains outside the original provider's origin; cross-origin redirects **shall** be rejected as potential phishing.

### 4.5 Optional

**REQ-CAL-017 [Optional]** — **Where** the provider is `google_native`, `GetEvents` **shall** populate `Event.Conferencing.MeetLink` from Google Calendar's conferenceData extension; CalDAV lacks this field and returns nil.

**REQ-CAL-018 [Optional]** — **Where** `config.calendar.nlp_create == true`, `CreateEvent` **shall** accept natural-language descriptions (e.g., "내일 오후 3시 김과장과 점심") and parse via LLM (ADAPTER-001) into structured Event fields.

**REQ-CAL-019 [Optional]** — **Where** a Korean public holiday overlaps a query range (per SCHEDULER-001 HolidayCalendar), `GetTodaySchedule` **shall** prepend a synthetic event `{summary: "<공휴일명>", allDay: true, source: "holiday"}`.

---

## 5. 수용 기준

**AC-CAL-001 — CalDAV list calendars**
- **Given** CalDAV server mock (Radicale local)의 사용자 계정 `u1`에 3개 캘린더
- **When** `CalDAVProvider.ListCalendars(ctx)`
- **Then** 3개 `Calendar` 반환, 각 `Name`/`URL`/`TimeZone` 필드 채워짐.

**AC-CAL-002 — GetEvents 범위 제한**
- **Given** `from=2026-01-01, to=2027-06-01` (151일)
- **When** `GetEvents(ctx, calID, from, to)`
- **Then** `ErrRangeTooWide` 반환.

**AC-CAL-003 — Recurring event expansion**
- **Given** VEVENT with RRULE=FREQ=WEEKLY;BYDAY=MO (매주 월), query range 4주
- **When** `GetEvents`
- **Then** 4개 occurrence 반환, 각 `MasterEventID` 동일 master 참조.

**AC-CAL-004 — OAuth 토큰 만료 처리**
- **Given** CREDPOOL-001 mock이 첫 호출에 expired 토큰 반환, refresh 성공
- **When** `GetEvents`
- **Then** 내부적으로 refresh 1회, 최종 결과 정상 반환.

**AC-CAL-005 — Re-auth 필요 시**
- **Given** CREDPOOL-001 refresh가 400 반환 (invalid_grant)
- **When** `GetEvents`
- **Then** `ErrReauthRequired` 반환, 에러 메시지에 authorization URL 포함.

**AC-CAL-006 — 토큰 로그 미노출**
- **Given** OAuth access 토큰 문자열 (redacted)
- **When** `GetEvents` 호출 + zap 로그 캡처
- **Then** 로그 문자열에 원본 토큰 미포함, `****` 또는 hash prefix만 포함.

**AC-CAL-007 — Timezone 변환**
- **Given** CalDAV event `DTSTART;TZID=America/New_York:20260422T150000`, user TZ = Asia/Seoul
- **When** `GetEvents` → render
- **Then** `event.LocalStart` = 2026-04-23 04:00 (+9h 변환), `event.Timezone` = "America/New_York" 원본 유지.

**AC-CAL-008 — Read-only mode**
- **Given** `config.calendar.write_enabled=false`
- **When** `CreateEvent` 호출
- **Then** `ErrWriteDisabled` 반환, HTTP 요청 0회.

**AC-CAL-009 — Multi-provider aggregation**
- **Given** `google_primary` 에 3 이벤트, `naver` 에 2 이벤트 (동일 시간 1개 중복)
- **When** `GetTodaySchedule(userID)`
- **Then** 반환 리스트 4개 (1개 dedup), 시작 시간 오름차순 정렬.

**AC-CAL-010 — 공휴일 주입**
- **Given** query day = 2026-10-03 (개천절)
- **When** `GetTodaySchedule`
- **Then** 결과 첫 번째 항목이 `{summary:"개천절", allDay:true, source:"holiday"}`.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── calendar/
        ├── provider.go        # CalendarProvider interface
        ├── caldav.go          # CalDAV RFC 4791 client
        ├── google_native.go   # Google Calendar API v3
        ├── outlook_native.go  # MS Graph API
        ├── naver.go           # Naver (CalDAV fallback)
        ├── aggregator.go      # GetTodaySchedule 다중 provider
        ├── types.go           # Event, Calendar, DailySchedule
        ├── oauth.go           # OAuth flow (CREDPOOL-001 연동)
        ├── rrule.go           # RRULE expansion wrapper
        ├── tzconv.go          # TZID → IANA 변환
        ├── circuit.go         # circuit breaker
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입 시그니처 (의사코드)

```
CalendarProvider interface
  - Name() string
  - ListCalendars(ctx) ([]Calendar, error)
  - GetEvents(ctx, calID, from, to) ([]Event, error)
  - CreateEvent(ctx, calID, Event) (Event, error)
  - UpdateEvent(ctx, calID, Event) error
  - DeleteEvent(ctx, calID, eventID) error

Calendar {
  ID, Name, URL, TimeZone, Color, Primary bool, Writable bool
}

Event {
  ID, MasterEventID, CalendarID
  Summary, Description, Location
  Start, End time.Time (UTC)
  Timezone string (IANA)
  LocalStart, LocalEnd time.Time (user-TZ, render-only)
  AllDay bool
  RecurrenceRule string
  Attendees []Attendee
  Reminders []Reminder
  Conferencing *Conferencing
  URL string
  Source string (provider 또는 "holiday")
  LastModified time.Time
  ETag string
}

DailySchedule {
  Date time.Time (local midnight)
  Events []Event (시작 시간 정렬)
  TotalEvents int
  FirstEvent, LastEvent *Event
  HasOverlap bool
}
```

### 6.3 CalDAV 요청 예 (RFC 4791 calendar-query)

```
REPORT /calendars/u1/personal/ HTTP/1.1
Depth: 1
Content-Type: application/xml

<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop xmlns:D="DAV:">
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260422T000000Z" end="20260423T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>
```

### 6.4 Aggregator

여러 provider를 `errgroup.WithContext` 로 병렬 호출 → 결과 머지 → `(summary, start, end)` 3-tuple dedup → 공휴일 주입 → 시작 시간 정렬. 개별 provider 실패는 전체 실패로 전파하지 않고 WARN 로그만.

### 6.5 RRULE Expansion

`github.com/teambition/rrule-go` v1.8+ 사용. master VEVENT + RRULE → `rule.Between(from, to, inclusive)` 로 occurrence 리스트.

### 6.6 OAuth Flow

1. 사용자가 CLI `goose calendar auth google` 실행
2. CLI가 local HTTP server (port 6274) 시작 + 브라우저 open
3. 사용자 Google 승인 → callback으로 code 수신
4. Token exchange → CREDPOOL-001에 저장 (암호화)
5. CLI 종료

### 6.7 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| CalDAV | `github.com/emersion/go-webdav` + `go-caldav` 확장 | 표준 준수, maintained |
| RRULE | `github.com/teambition/rrule-go` v1.8+ | 완전 RFC 5545 |
| Google Calendar | `google.golang.org/api/calendar/v3` | 공식 |
| MS Graph | `github.com/microsoftgraph/msgraph-sdk-go` | 공식 |
| iCal parser | `github.com/arran4/golang-ical` | VEVENT 파싱 |
| OAuth2 | `golang.org/x/oauth2` | CREDPOOL-001 경유 |

### 6.8 TDD 진입

1. RED: `TestCalDAV_ListCalendars` (Radicale local 컨테이너)
2. RED: `TestRangeTooWide_91days` — AC-CAL-002
3. RED: `TestRRule_WeeklyExpansion` — AC-CAL-003
4. RED: `TestOAuth_RefreshOnce` — AC-CAL-004
5. RED: `TestReauthRequired_400` — AC-CAL-005
6. RED: `TestToken_NotInLogs` — AC-CAL-006
7. RED: `TestTZ_NYtoSeoul` — AC-CAL-007
8. RED: `TestWriteDisabled_ReturnsErr` — AC-CAL-008
9. RED: `TestAggregator_MultiProvider_Dedup` — AC-CAL-009
10. RED: `TestHolidayInjection_2026_10_03` — AC-CAL-010
11. GREEN → REFACTOR

### 6.9 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, Radicale docker 통합 테스트, mock OAuth flow |
| **R**eadable | provider/aggregator/types/oauth 파일 분리 |
| **U**nified | Event DTO 통일, Error 타입 표준화 |
| **S**ecured | 토큰 CREDPOOL, 최소 scope, cross-origin redirect 차단, circuit breaker |
| **T**rackable | 모든 호출 구조화 로그, provider별 latency 추적 |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | MCP-001 | MCP tool 노출 경로 (optional) |
| 선행 SPEC | CREDPOOL-001 | OAuth 토큰 저장·갱신 |
| 선행 SPEC | TOOLS-001 | `Calendar` tool 등록 |
| 선행 SPEC | CONFIG-001 | calendar.yaml |
| 선행 SPEC | CORE-001 | zap, context |
| 후속 SPEC | BRIEFING-001 | 오늘의 일정 소비 |
| 후속 SPEC | SCHEDULER-001 | HolidayCalendar 공유 |
| 외부 | `emersion/go-webdav` | CalDAV |
| 외부 | `teambition/rrule-go` | RRULE |
| 외부 | `google.golang.org/api/calendar/v3` | 공식 SDK |
| 외부 | `microsoftgraph/msgraph-sdk-go` | 공식 SDK |
| 외부 | `arran4/golang-ical` | iCal 파싱 |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | iCloud CalDAV 서버 quirks (비표준 응답) | 중 | 중 | iCloud 전용 workaround 모듈, golden response fixture 유지 |
| R2 | Google Calendar API v3 quota (1M/day/project but low per-user) | 중 | 중 | GetEvents 캐싱 5분, 공유 프로젝트 토큰 금지 |
| R3 | RRULE 복잡 케이스 (BYSETPOS, EXDATE) 파싱 오류 | 중 | 중 | rrule-go 테스트 스위트 전수 검증 |
| R4 | Naver Calendar CalDAV 미지원 | 고 | 중 | Naver API 문서 확인, 미지원 시 v0.1 scope에서 제외 |
| R5 | OAuth 토큰 유출 | 낮 | 치명적 | CREDPOOL-001 암호화 저장, 로그 redaction |
| R6 | Cross-timezone 버그 (DST 전환) | 중 | 고 | time.LoadLocation strict, DST goldenfile 테스트 |
| R7 | CalDAV 서버 ETag mismatch로 쓰기 충돌 | 중 | 중 | If-Match 헤더 사용, 충돌 시 ErrConflict로 사용자 재시도 유도 |
| R8 | 사용자가 외부 캘린더에서 삭제한 이벤트가 cache에 남음 | 중 | 낮 | TTL 5분 + ETag 기반 revalidation |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-MCP-001/spec.md` — MCP tool 노출
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` — OAuth
- `.moai/specs/SPEC-GOOSE-BRIEFING-001/spec.md` — consumer
- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — HolidayCalendar 공유

### 9.2 외부 참조

- RFC 4791 CalDAV: https://datatracker.ietf.org/doc/html/rfc4791
- RFC 5545 iCalendar: https://datatracker.ietf.org/doc/html/rfc5545
- Google Calendar API: https://developers.google.com/calendar/api/v3/reference
- MS Graph Calendar: https://learn.microsoft.com/en-us/graph/api/resources/calendar
- Naver Cloud Calendar: https://api.ncloud-docs.com/docs/common-ncpapi
- iCloud CalDAV docs: https://developer.apple.com/library/archive/documentation/Networking/

### 9.3 부속 문서

- `./research.md` — CalDAV 4 provider 호환성 매트릭스, RRULE edge case, OAuth flow 상세

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **미팅 스케줄러 (When2meet 등) 를 포함하지 않는다**. 별도 SPEC.
- 본 SPEC은 **자동 회의 조정 AI 를 포함하지 않는다**.
- 본 SPEC은 **Zoom/Meet 자동 생성을 기본 제공하지 않는다** (Google native 일부만 지원).
- 본 SPEC은 **Offline sync를 포함하지 않는다** (read cache만 5분).
- 본 SPEC은 **Calendar analytics (시간 사용 분석) 를 포함하지 않는다**.
- 본 SPEC은 **Push notification 발송을 포함하지 않는다** (Gateway SPEC).
- 본 SPEC은 **iCalendar 파일 import/export를 포함하지 않는다** (v0.2 확장).
- 본 SPEC은 **Conflict resolution UI를 포함하지 않는다** (last-write-wins + 에러).
- 본 SPEC은 **Exchange on-prem을 지원하지 않는다** (cloud only).
- 본 SPEC은 **Calendar subscription 공유 권한 관리를 포함하지 않는다**.
- 본 SPEC은 **Event search (전문 검색) 을 포함하지 않는다** (시간 범위 쿼리만).

---

**End of SPEC-GOOSE-CALENDAR-001**

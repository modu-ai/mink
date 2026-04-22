# Research — SPEC-GOOSE-CALENDAR-001

## 1. Provider 호환성 매트릭스

| Provider | CalDAV | Native API | OAuth 2.0 | App-Specific Pwd | 한국 사용률 |
|---------|--------|-----------|-----------|-----------------|-----------|
| Google Calendar | O (제한적) | O (권장) | O | X | 상 |
| Apple iCloud | O | X | X | O | 중 |
| Microsoft Outlook/365 | O | O (Graph) | O | X | 중 (기업) |
| Naver Calendar | △ (확인 필요) | 공개 API 없음 | - | - | 상 (한국) |

결정:
- Google: Native API 우선, CalDAV fallback
- iCloud: CalDAV + App-Specific Password (사용자 Apple ID 에서 생성)
- Outlook: Graph API 우선, CalDAV fallback
- Naver: CalDAV 확인 후, 불가 시 scope 제외 (v0.2 재검토)

## 2. iCloud App-Specific Password

Apple은 외부 앱에 계정 비밀번호 직접 사용 금지. 사용자가 Apple ID 설정에서 "앱 암호" 를 생성하고 CREDPOOL-001에 저장.

흐름:
1. 사용자 `appleid.apple.com` 접속
2. "Sign-In and Security" → "App-Specific Passwords" → Generate
3. 생성된 16자리 암호를 CLI `goose calendar auth icloud --app-password <pwd>` 로 저장
4. CalDAV endpoint: `https://caldav.icloud.com/` + 사용자 DSID (principal URL 조회)

## 3. CalDAV 호환성 quirks

### 3.1 iCloud

- `PROPFIND` 응답에 표준 외 `X-APPLE-*` 네임스페이스 다수
- principal URL discovery 2단계 (`/.well-known/caldav` → 302 → principal URL)
- REPORT 응답 chunked encoding, 파싱 시 `io.ReadAll` 후 처리

### 3.2 Google Calendar (CalDAV endpoint)

- URL: `https://apidata.googleusercontent.com/caldav/v2/{calendarId}/events/`
- OAuth 2.0 Bearer 토큰 필요 (CalDAV 용이지만 Google OAuth 동일)
- CalDAV 경로로는 recurrence expansion이 native API 보다 불완전 → native 권장

### 3.3 Outlook/Office 365

- Microsoft는 2023년 이후 CalDAV 권장 deprecated 안내. Graph API 강력 권장.
- CalDAV 여전히 동작하나 장기적으로 native Graph 선호.

### 3.4 Radicale (로컬 테스트)

- 무료 self-hosted CalDAV 서버. Docker image 사용.
- 통합 테스트에서 `testcontainers-go` 로 자동 spin-up.

## 4. RRULE Edge Cases

### 4.1 테스트 패턴

| 패턴 | 설명 | 지원 여부 |
|------|------|---------|
| `FREQ=DAILY` | 매일 | O |
| `FREQ=WEEKLY;BYDAY=MO,WE,FR` | 월/수/금 | O |
| `FREQ=MONTHLY;BYMONTHDAY=15` | 매월 15일 | O |
| `FREQ=MONTHLY;BYDAY=2TU` | 매월 둘째 화요일 | O |
| `FREQ=YEARLY;BYMONTH=12;BYMONTHDAY=25` | 매년 12월 25일 | O |
| `FREQ=WEEKLY;BYDAY=MO;BYSETPOS=-1` | 마지막 주 월요일 | O (rrule-go) |
| `EXDATE` | 특정일 제외 | O |
| `RDATE` | 특정일 추가 | O |
| 무한 recurrence (UNTIL/COUNT 없음) | | 안전 cap 500 occurrences |

### 4.2 윤년·DST 처리

- `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=29` 윤년만 발생 → rrule-go 자동 처리.
- DST 전환일 `BYHOUR=2` → "존재하지 않는 시간" → DST spring-forward 시 skip 또는 adjacent hour.

## 5. OAuth Flow 상세

### 5.1 Port 선택

로컬 callback server port: `6274` (GOOSE 전용). 사용 중이면 `6275-6290` 순차 시도.

### 5.2 PKCE (Proof Key for Code Exchange)

모든 provider PKCE 지원. 공개 클라이언트(CLI)에서 secret 보호 필수.

### 5.3 State parameter

CSRF 방지: `state=` 에 random 32byte, callback 에서 검증.

### 5.4 Scopes

- Google: `https://www.googleapis.com/auth/calendar.events` (읽기+쓰기), `.readonly` (읽기만)
- Microsoft: `Calendars.Read`, `Calendars.ReadWrite`, `offline_access` (refresh 토큰)
- Naver: 공개 OAuth 없음, 추후 조사

### 5.5 토큰 보관

CREDPOOL-001에 `{provider}/{user_email_hash}` 키로 저장. refresh 토큰은 가능하면 무기한. Access 토큰 만료 시 refresh 자동.

## 6. Timezone 처리

### 6.1 TZID → IANA 변환

Windows timezone ID (Outlook 일부) → IANA 매핑 테이블 필요:
- "Pacific Standard Time" → "America/Los_Angeles"
- "Korea Standard Time" → "Asia/Seoul"
- unicode-org/cldr 의 `windowsZones.xml` 기반 테이블 유지.

### 6.2 floating time

`DTSTART:20260422T150000` (no TZID, no Z suffix) = floating time, 사용자 로컬 해석. iCalendar RFC 5545 §3.3.5 규정.

### 6.3 DST

Asia/Seoul은 1988년 이후 DST 없음. 사용자 TZ가 America/New_York 등일 때만 DST 고려.

## 7. Circuit Breaker

Sony gobreaker 라이브러리 또는 자체 간단 구현:

```
state: closed → open (5xx 3회 / 60초) → half-open (5분 후) → closed or open
```

## 8. Aggregator 성능

목표: 3 provider 각 200ms 이내 응답 → 병렬로 250ms 총 latency.

- Goroutine per provider
- 5초 전체 timeout
- 실패한 provider는 skip + WARN

## 9. 프라이버시 고려

1. 이벤트 summary/description에 민감 정보 포함 가능 → MEMORY-001 저장 시 암호화.
2. Aggregator 결과는 메모리에만 유지, 디스크 캐시는 title/time만 저장 (description 제외).
3. 사용자별 격리: memory cache key에 userID 포함.
4. Attendees 이메일 주소는 IDENTITY-001 Person entity 생성에 활용 가능 (별도 opt-in).

## 10. 테스트 전략

### 10.1 Radicale 통합 테스트

```
testcontainers.ContainerRequest{
  Image: "tomsquest/docker-radicale:latest",
  ExposedPorts: []string{"5232/tcp"},
}
```

### 10.2 Mock OAuth

`httptest.NewServer` 로 authorize + token 엔드포인트 mock. 실제 Google/Microsoft 호출 없이 flow 검증.

### 10.3 Goldenfile

10종 iCalendar VEVENT fixture (recurring, timezone, all-day, exception 등) 준비. Event DTO 파싱 결과 snapshot.

## 11. 한국 시장 특화

1. **Naver Calendar 우회**: CalDAV 미지원 시 사용자가 "Naver → Google Calendar 동기화" 권장 (2-way sync는 Google side 설정).
2. **공휴일 자동 주입**: SCHEDULER-001 HolidayCalendar 와 연동.
3. **음력 기반 이벤트**: 한국 명절 (설/추석) 음력 날짜 → 양력 변환 후 일정 표시 (FORTUNE-001 만세력 재사용).
4. **한국 기업용 Naver Works**: B2B 제품, 별도 SPEC (범위 외).

## 12. 오픈 이슈

1. **Naver Calendar 연동 현실성**: 공개 OAuth + CalDAV 지원 확인 필요. 미지원 시 "Naver 일정을 Google Calendar로 동기화" 안내만.
2. **Recurring 수정 (이번만 / 이후 모두)**: CalDAV는 EXDATE + 새 master 생성 패턴. UI 에서 사용자 선택 필요.
3. **이벤트 description의 HTML/Markdown**: 원본 유지 vs plain text 변환. 기본 plain.
4. **Multi-account (2 Google 계정)**: config에 provider 배열 허용, 구분은 `name` 필드.

## 13. 참고

- iCalendar validator: https://icalendar.org/validator.html
- Google Calendar API ref: https://developers.google.com/calendar/api/v3/reference
- MS Graph Calendar: https://learn.microsoft.com/en-us/graph/api/resources/calendar
- Radicale: https://radicale.org/
- CalDAV Synchronization (RFC 6578): 추후 확장

---
id: SPEC-GOOSE-WEATHER-001
artifact: acceptance
version: 0.1.0
created_at: 2026-05-10
updated_at: 2026-05-10
author: manager-spec
---

# SPEC-GOOSE-WEATHER-001 — 수용 기준 (Acceptance)

본 문서는 spec.md §5 의 10개 AC (8 원본 + 2 신규) 를 Given-When-Then 형식으로 상세화하고, 각 AC 를 Go test 파일/함수와 매핑한다.

milestone 표기: 각 AC 의 헤더에 (M1) / (M2) / (M3) 를 명시. M1 Plan Phase 단계에서 AC-WEATHER-001/002/003/006/007/008/009/010 (8개) 가 GREEN 대상. AC-WEATHER-004 (M2) / AC-WEATHER-005 (M3) 는 후속 milestone 에서 처리.

---

## AC-WEATHER-001 — Tool 자동 등록 (M1)

**Given**
- 프로세스 bootstrap 완료 (CORE-001 + TOOLS-001 + TOOLS-WEB-001 init).
- `internal/tools/web` 패키지 import 시 `weather_current.go init()` 가 `RegisterWebTool(&webWeatherCurrent{...})` 호출 완료.

**When**
- `registry := tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb())` 호출.
- `names := registry.ListNames()` 호출.

**Then**
- `names` 에 `"weather_current"` 포함.
- `tool := registry.Resolve("weather_current")` 가 non-nil Tool 반환.
- `tool.Scope() == tools.ScopeShared`.
- `tool.Schema()` 가 valid JSON Schema (draft 2020-12, top-level `additionalProperties: false`).

**Test**
- File: `internal/tools/web/register_test.go`
- Function: `TestRegistry_WithWeb_ListNames_IncludesWeatherCurrent` (M1) + 기존 `TestRegistry_WithWeb_ListNames` 의 expectation 갱신 (count +1)

**Edge — 두 번째 RegisterWebTool 호출**
- `RegisterWebTool` 가 같은 도구를 두 번 등록해도 동일 동작 (TOOLS-WEB-001 패턴 정렬).

---

## AC-WEATHER-002 — Cache 히트 (M1)

**Given**
- 임시 캐시 디렉터리 (`t.TempDir()`).
- `webWeatherCurrent.Call(ctx, {"lat": 37.5665, "lon": 126.9780})` 1회 호출 완료, mock OpenWeatherMap endpoint 가 정상 응답, bbolt cache 에 저장됨.
- mock outbound counter == 1.

**When**
- 5분 뒤 (mock clock advance) 동일 input 으로 재호출.

**Then**
- 응답 `{ok: true, data: {...}, metadata: {cache_hit: true, duration_ms: ≥ 0}}`.
- mock outbound counter == 1 (증가하지 않음).
- `data.stale == false`.

**Edge — TTL 만료 후 miss**
- 11분 뒤 동일 input 호출 → mock outbound counter == 2 (+1), `metadata.cache_hit == false`, 캐시 갱신.

**Test**
- File: `internal/tools/web/weather_current_test.go`
- Function: `TestWeatherCurrent_CacheHitWithin10Min`

---

## AC-WEATHER-003 — Offline fallback (M1)

**Given**
- mock 네트워크가 `net.Dial` error 반환 (또는 mock OWM endpoint 가 5xx 응답).
- 디스크에 `~/.goose/cache/weather/latest-openweathermap-37.57-126.98.json` 존재 (12시간 전 데이터, 정상 JSON).
- 임시 cache (TempDir) 는 비어 있음.

**When**
- `webWeatherCurrent.Call(ctx, {"lat": 37.5665, "lon": 126.9780})` 호출.

**Then**
- 응답 `{ok: true, data: {...}, metadata: {cache_hit: false, ...}}`.
- `data.stale == true`.
- `data.message` 에 "오프라인" 또는 "마지막 확인" 문구 포함 (Korean).
- `data.timestamp` 가 12h 전 timestamp 와 일치.
- audit 1 line: `outcome: "ok"` + reason absent (또는 reason: "offline_fallback" 으로 명시 - 구현 결정).

**Edge — 디스크 파일 없음**
- 디스크 파일도 없음 → 응답 `{ok: false, error: {code: "fetch_failed", retryable: true}}`. audit reason="fetch_failed".

**Edge — 디스크 파일 corrupt JSON**
- 디스크 파일 존재하지만 JSON parse 실패 → 파일 evict + `fetch_failed` 응답.

**Test**
- File: `internal/tools/web/weather_current_test.go`
- Function: `TestWeatherCurrent_OfflineFallback_DiskRead`
- 보조 File: `internal/tools/web/weather_offline_test.go`
- 보조 Function: `TestOfflineSaveLoad_RoundTrip`, `TestOfflineLoad_FileMissing`, `TestOfflineLoad_CorruptJSON_Evict`

---

## AC-WEATHER-004 — 한국 좌표 자동 KMA 라우팅 (M2)

**Given (M2)**
- `~/.goose/config/weather.yaml` 에 `weather.provider: "auto"`, `weather.kma.api_key: "valid"`.
- mock OWM endpoint + mock KMA endpoint 모두 활성, 별도 outbound counter.
- input `{"location": "Seoul,KR"}` (Country=="KR" detected).

**When (M2)**
- `webWeatherForecast.Call(ctx, input)` 호출.

**Then (M2)**
- 응답 `data.source_provider == "kma"`.
- mock KMA outbound counter == 1.
- mock OWM outbound counter == 0.
- audit `provider: "kma"`.

**Edge — Country != "KR"**
- input `{"location": "Tokyo,JP"}` → mock OWM outbound counter == 1, KMA == 0.

**Edge — KMA API key 부재 (auto fallback)**
- `weather.kma.api_key: ""` + `provider: "auto"` + Country=="KR" → silent fallback to OWM (REQ-WEATHER-011 state-driven). audit reason="kma_key_missing_fallback_owm".

**Edge — provider="kma" 강제 + key 부재**
- `provider: "kma"` 강제 + key 부재 → `webWeatherForecast` initialization or first Call 시 `{ok: false, error.code == "missing_api_key"}`.

**Test (M2)**
- File: `internal/tools/web/weather_route_test.go`
- Function: `TestAutoRoute_KRCountryUsesKMA`

---

## AC-WEATHER-005 — 미세먼지 normalized level (M3)

**Given (M3)**
- `webWeatherAirQuality` 구현 + AirKoreaProvider mock.
- mock 에어코리아 응답 `{pm25: 55, pm10: 80}` (μg/m³).
- input `{"location": "Seoul,KR"}`.

**When (M3)**
- `webWeatherAirQuality.Call(ctx, input)` 호출.

**Then (M3)**
- 응답 `{ok: true, data: {pm25: 55, pm10: 80, level: "unhealthy", level_local: "나쁨"}, metadata: {...}}`.
- 한국 환경부 기준 36-75 → "나쁨" 매핑 검증.

**Edge — Boundary cases (table-driven)**
- pm25=15 → "good" / "좋음"
- pm25=16 → "moderate" / "보통"
- pm25=35 → "moderate" / "보통"
- pm25=36 → "unhealthy" / "나쁨"
- pm25=75 → "unhealthy" / "나쁨"
- pm25=76 → "very_unhealthy" / "매우 나쁨"

**Edge — 한국 외 좌표 (lat=35.0, lon=139.0)**
- `unsupported_region` 반환 (M3 는 한국 only).

**Test (M3)**
- File: `internal/tools/web/weather_air_quality_test.go`
- Function: `TestAirQuality_PM25_KoreanStandardMapping`

---

## AC-WEATHER-006 — API key 로그 미노출 (M1)

**Given**
- zap logger 에 `zaptest/observer.New(zap.DebugLevel)` 적용 (모든 log entry 캡처).
- env `OPENWEATHERMAP_API_KEY=secret123abc456`.
- mock OWM endpoint 가 정상 응답.

**When**
- `webWeatherCurrent.Call(ctx, {"lat": 37.5665, "lon": 126.9780})` 호출.

**Then**
- 캡처된 모든 log entry 의 message + field 값 + serialized JSON 에 `"secret123abc456"` 문자열 부재.
- request URL log 시 query string `appid=` 값이 `appid=****` 로 redact.
- error 발생 시 error message 에도 raw key 부재.

**Edge — error path (5xx 응답)**
- mock 이 500 + `body: "Internal error: appid=secret123abc456"` 반환 → 응답 error.message 가 raw body 포함하더라도 redact 처리.

**Test**
- File: `internal/tools/web/weather_openweather_test.go`
- Function: `TestOWM_GetCurrent_APIKey_Redacted_NotInLogs`

---

## AC-WEATHER-007 — Singleflight 중복 제거 (M1)

**Given**
- mock OWM endpoint 가 50ms 지연 후 정상 응답 (`time.Sleep(50 * time.Millisecond)`).
- 임시 캐시 비어 있음.

**When**
- 100 goroutine 이 동일 input `{"lat": 37.5665, "lon": 126.9780}` 으로 동시 `Call()`.
- `sync.WaitGroup` 으로 모든 goroutine 완료 대기.

**Then**
- mock outbound counter == 1 (정확히 1회만).
- 100 goroutine 모두 동일 결과 수신 (`assert.Equal` 로 검증).
- 응답 `{ok: true, data: {...}, metadata: {cache_hit: false (또는 true depending on race)}}`.
- 첫 goroutine 의 응답이 캐시에 저장됨 (101번째 호출 시 cache_hit 검증).

**Test**
- File: `internal/tools/web/weather_current_test.go`
- Function: `TestWeatherCurrent_Singleflight_ConcurrentDedup`

---

## AC-WEATHER-008 — Rate limit 차단 (M1)

**Given**
- TOOLS-WEB-001 `RateTracker` 인스턴스가 provider `openweathermap` 의 `requests_min` bucket 을 강제로 `{Limit: 60, Remaining: 0, ResetSeconds: 15, CapturedAt: now}` 로 설정.
- mock OWM endpoint (호출되어선 안됨).

**When**
- `webWeatherCurrent.Call(ctx, {"lat": 37.5665, "lon": 126.9780})` 호출.

**Then**
- 응답 `{ok: false, error: {code: "ratelimit_exhausted", message: "...", retryable: true, retry_after_seconds: 15}, metadata: {...}}`.
- mock OWM outbound counter == 0.
- audit 1 line: `outcome: "denied", reason: "ratelimit_exhausted"`.

**Test**
- File: `internal/tools/web/ratelimit_integration_test.go`
- Function: `TestRateLimitExhausted_Weather`

---

## AC-WEATHER-009 — Registry inventory (3 도구) (M1 부분 + M2/M3 누적)

**Given**
- TOOLS-WEB-001 의 8 web 도구 모두 등록.
- 본 SPEC 의 weather 도구가 milestone 별로 등록.

**When**
- `names := tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb()).ListNames()` 호출.

**Then (M1)**
- `len(names) == 6 + 8 + 1 == 15` (built-in 6 + TOOLS-WEB-001 의 8 + weather_current 1).
- `weather` prefix 정렬 부분집합 == `["weather_current"]`.
- `Scope() == ScopeShared`.

**Then (M2)**
- `len(names) == 16` (+1 weather_forecast).
- weather subset == `["weather_current", "weather_forecast"]`.

**Then (M3)**
- `len(names) == 17` (+1 weather_air_quality).
- weather subset 정렬 == `["weather_air_quality", "weather_current", "weather_forecast"]`.

**Test**
- File: `internal/tools/web/register_test.go` (expectation 갱신)
- Function: `TestRegistry_WithWeb_IncludesWeather`

**Edge**
- 두 번째 `tools.WithWeb()` 호출 시 idempotent 또는 `ErrDuplicateName` (TOOLS-WEB-001 §4.4 REQ-TOOLS-013 정렬).

---

## AC-WEATHER-010 — 표준 응답 shape (TOOLS-WEB-001 통합) (M1)

**Given**
- M1 시점: `webWeatherCurrent` 1개 도구.
- M2/M3 누적: 3개 도구.
- 각 도구별 mock 환경에서 성공 1회 + 실패 1회 케이스 (M1=2 케이스, M3 누적=6 케이스).

**When**
- 각 케이스 호출 후 응답을 `json.Unmarshal` 로 `common.Response` 구조체에 매핑.

**Then**
- 모든 케이스 unmarshal 성공.
- 모든 응답에 정확히 다음 top-level keys: `ok`, (`data` 또는 `error`), `metadata`.
- `error` 객체는 `code`, `message`, `retryable` 키 보유; 옵션 키는 `retry_after_seconds`.
- `metadata` 객체는 `cache_hit`, `duration_ms` 키 보유.
- 성공 응답의 `data.stale`, `data.cache_hit` (해당 시) 와 `metadata.cache_hit` 의 의미 구분 검증:
  - `metadata.cache_hit == true` && `data.stale == true` → 발생 불가 (assert).
  - `metadata.cache_hit == false` && `data.stale == true` → offline disk fallback 정상.
  - `metadata.cache_hit == true` && `data.stale == false` → live bbolt cache hit 정상.
  - `metadata.cache_hit == false` && `data.stale == false` → outbound API success 정상.

**Test**
- File: `internal/tools/web/weather_current_test.go` (M1)
- Function: `TestWeatherCurrent_StandardResponseShape`
- 누적: `internal/tools/web/response_test.go` 의 `TestStandardResponseShape_AllTools` 에 weather 도구 추가 (M3 시점 8 → 11 도구로 확장).

---

## 종합 Definition of Done

### M1 DoD

- [ ] AC-WEATHER-001 / 002 / 003 / 006 / 007 / 008 / 009 (M1 부분, +1 검증) / 010 (M1 부분, weather_current 1개) 모두 GREEN.
- [ ] 각 AC 의 edge case 도 별도 test 로 분리되어 GREEN.
- [ ] `internal/tools/web/weather*.go` coverage ≥ 85%.
- [ ] `golangci-lint run ./internal/tools/web/...` zero warning.
- [ ] `go vet ./internal/tools/web/...` zero issue.
- [ ] integration test (build tag `integration`) 가 mock + 실제 OWM 모두 GREEN (실제 OWM 은 API key 환경변수 필수, 미보유 시 skip).
- [ ] e2e: PERMISSION-001 + RATELIMIT-001 + AUDIT-001 3 시스템 통합 시나리오 1회 GREEN.
- [ ] TOOLS-WEB-001 8 도구 회귀 0 (기존 register/permission/audit/ratelimit 테스트 영향 없음).
- [ ] `.moai/docs/weather-quickstart.md` 작성.

### M2 DoD

- [ ] AC-WEATHER-004 GREEN.
- [ ] KMAProvider 5개 도시 좌표 goldenfile 검증.
- [ ] AC-WEATHER-009 (M2 부분, +1 weather_forecast) GREEN.

### M3 DoD

- [ ] AC-WEATHER-005 GREEN (boundary 4 case).
- [ ] AC-WEATHER-009 (M3 부분, +1 weather_air_quality, total 17) GREEN.

---

## 품질 게이트 (TRUST 5 매핑)

- **Tested**: 10 AC + edge case + integration + e2e (커버리지 ≥ 85%). M1 단계는 8 AC + 4 edge case 우선.
- **Readable**: weather 도메인 godoc 영문, 명확한 에러 코드 (`fetch_failed`, `ratelimit_exhausted`, `permission_denied`, `host_blocked`, `invalid_input`, `unsupported_region`, `geolocation_failed`, `missing_api_key`), 표준 응답 shape (TOOLS-WEB-001 정렬).
- **Unified**: 3 도구 모두 TOOLS-WEB-001 `common.Response` 사용 + 동일한 register pattern (`RegisterWebTool` + `init()`) + 동일한 schema 스타일 (`additionalProperties: false`).
- **Secured**: PERMISSION-001 + AUDIT-001 + RATELIMIT-001 + Blocklist + API key redaction (REQ-WEATHER-004) + atomic disk write (R7) + bbolt 락 timeout (R8).
- **Trackable**: AUDIT-001 모든 호출 기록 (`tool: weather_*`, `provider`, `cache_hit`, `stale`), RATELIMIT-001 per-provider 추적, 표준 commit message + REQ/AC trailer.

---

Version: 0.1.0
Last Updated: 2026-05-10

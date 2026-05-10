---
id: SPEC-GOOSE-WEATHER-001
artifact: plan
version: 0.1.0
created_at: 2026-05-10
updated_at: 2026-05-10
author: manager-spec
---

# SPEC-GOOSE-WEATHER-001 — 구현 계획 (Plan)

본 문서는 SPEC-GOOSE-WEATHER-001 의 milestone, task breakdown, 입력 스키마 상세, provider 비교, test plan 을 담는다. 우선순위는 priority label(P1~P3) 로 표기하며 시간 추정은 사용하지 않는다.

본 SPEC 은 TOOLS-WEB-001 의 common 인프라(`common.Cache`, `common.Blocklist`, `common.Deps`, `common.Response`, `common.UserAgent()`, `RegisterWebTool`, `RateTracker`, `permission.Manager`, `audit.EventTypeToolWebInvoke`)를 재사용한다. 본 plan 의 task 는 weather 도메인 로직 (좌표 변환, 단위 변환, language mapping, standard mapping) 에 집중한다.

---

## 1. Milestone 개요

| Milestone | Priority | 산출물 | 의존 |
|---|---|---|---|
| **M1 — weather_current (OWM + cache + offline + IP geo)** | P1 (먼저) | `weather_current` 도구, `WeatherProvider` 인터페이스, `OpenWeatherMapProvider`, `WeatherReport` DTO, IP geolocation, offline disk fallback | TOOLS-001, TOOLS-WEB-001 (M1 implemented), PERMISSION-001, AUDIT-001, RATELIMIT-001 |
| **M2 — weather_forecast + KMA 라우팅** | P2 | `weather_forecast` 도구, `KMAProvider` (초단기실황 + 단기예보 + DFS_XY_CONV), `WeatherForecastDay` DTO, provider auto-routing | M1, KMA API key (사용자 발급) |
| **M3 — weather_air_quality (Korean standard)** | P3 | `weather_air_quality` 도구, AirKorea provider, 한국 환경부 PM2.5 기준 매핑 | M1, M2 (provider 추상화 재사용), 에어코리아 API key |

각 milestone 완료 시점에 evaluator-active 회귀 + integration test + audit log 검증.

본 plan.md 의 §2 가 M1 의 23 atomic tasks 를 상세화한다. M2 / M3 는 §3 / §4 에서 high-level breakdown 만 정의 (M1 시점 audit-ready 진입 후 Sprint 3+ 에서 상세화).

---

## 2. M1 — weather_current (P1)

### 2.1 산출 파일

```
internal/tools/web/
├── weather_current.go              # weather_current 도구 (Tool 인터페이스)
├── weather_current_test.go
├── weather_provider.go             # WeatherProvider 인터페이스
├── weather_openweather.go          # OpenWeatherMapProvider (current 구현)
├── weather_openweather_test.go
├── weather_types.go                # WeatherReport, Location, AirQuality, SunTimes, Pollen DTO
├── weather_geoip.go                # ipapi.co IP geolocation (1h TTL)
├── weather_geoip_test.go
├── weather_offline.go              # 디스크 fallback (~/.goose/cache/weather/latest-*.json)
├── weather_offline_test.go
└── weather_config.go               # ~/.goose/config/weather.yaml loader

testdata/weather/
├── owm-seoul-current.json
└── ipapi-seoul.json
```

### 2.2 Task breakdown (M1)

총 23 atomic tasks (production 17 + test/integration/seed 6). tasks.md §"M1 Task Decomposition" 와 동기화.

- **T-001**: `weather_types.go` — `Location`, `WeatherReport`, `AirQuality`, `Pollen`, `SunTimes`, `WeatherForecastDay` DTO 정의 (M2/M3 의 DTO 도 미리 선언; 본 task 는 단순 struct).
- **T-002**: `weather_provider.go` — `WeatherProvider` 인터페이스 정의 (`Name`, `GetCurrent`, `GetForecast`, `GetAirQuality`, `GetSunTimes`).
- **T-003**: `weather_config.go` — `WeatherConfig` struct + `LoadWeatherConfig(path)` 함수. yaml.v3 기반, 누락 / 빈 파일 시 default 반환.
- **T-004**: `weather_offline.go` — `SaveLatest(provider, lat, lon, report)` + `LoadLatest(provider, lat, lon) (*WeatherReport, error)`. atomic write (temp + rename), 0600 권한.
- **T-005**: `weather_offline_test.go` — RED: `TestOfflineSaveLoad_RoundTrip`, `TestOfflineLoad_FileMissing`, `TestOfflineLoad_CorruptJSON_Evict`.
- **T-006**: `weather_geoip.go` — `IPGeolocator` interface + `IPAPIGeolocator` 구현 (`https://ipapi.co/json/`). 1h TTL 캐시 (TOOLS-WEB-001 `common.Cache` 재사용).
- **T-007**: `weather_geoip_test.go` — RED: `TestGeoIP_Resolve`, `TestGeoIP_CacheHitWithin1Hour`, `TestGeoIP_FetchFailed_Returns_GeolocationError`.
- **T-008**: `weather_openweather.go` — `OpenWeatherMapProvider.GetCurrent` 구현 (`https://api.openweathermap.org/data/2.5/weather` v2.5; `lang=ko|en`, `units=metric|imperial`, response → `WeatherReport`).
- **T-009**: `weather_openweather_test.go` — RED: `TestOWM_GetCurrent_Seoul_KO_Metric`, `TestOWM_GetCurrent_APIError_5xx_Retryable`, `TestOWM_GetCurrent_APIKey_Redacted_NotInLogs` (zap observer로 검증).
- **T-010**: `weather_current.go` 의 `webWeatherCurrent` Tool 구조체 + `Name() == "weather_current"` + `Schema()` (additionalProperties:false) + `Scope() == ScopeShared`.
- **T-011**: `weather_current.go` 의 input parser `parseWeatherCurrentInput(raw)` — schema enforcement (location 또는 lat+lon 필수, lat ∈ [-90,90], lon ∈ [-180,180], units enum, lang enum).
- **T-012**: `weather_current.go` 의 `Call()` 11-step 시퀀스 (TOOLS-WEB-001 `webWikipedia.Call` 패턴 정렬):
  1. parse input + defensive schema guard
  2. resolve location (lat/lon 명시 → use as-is; location string → geocode 또는 ipgeo fallback)
  3. derive provider host (M1 always `api.openweathermap.org`)
  4. blocklist gate (provider host)
  5. permission gate (CapNet, scope=host)
  6. cache key = SHA256(`weather_current:owm:{lat2dp}:{lon2dp}:{units}:{lang}`)
  7. live cache check → hit → return `{cache_hit: true, stale: false}`
  8. ratelimit check (provider `openweathermap`)
  9. singleflight: outbound API call
  10. on success: cache.Set(TTL=10min) + offline.SaveLatest + audit ok + return
  11. on failure: offline.LoadLatest → if found → `{stale: true}` + audit error + return; if not → `ErrNoFallbackAvailable` + audit error
- **T-013**: `weather_current_test.go` — RED: 8 시나리오:
  - `TestWeatherCurrent_Registered_InWebTools` (AC-WEATHER-001 + AC-WEATHER-009)
  - `TestWeatherCurrent_StandardResponseShape` (AC-WEATHER-010)
  - `TestWeatherCurrent_CacheHitWithin10Min` (AC-WEATHER-002)
  - `TestWeatherCurrent_OfflineFallback_DiskRead` (AC-WEATHER-003)
  - `TestWeatherCurrent_APIKey_Redacted_NotInLogs` (AC-WEATHER-006)
  - `TestWeatherCurrent_Singleflight_ConcurrentDedup` (AC-WEATHER-007)
  - `TestWeatherCurrent_RateLimit_Exhausted` (AC-WEATHER-008)
  - `TestWeatherCurrent_Blocklist_HostBlocked` (REQ-WEATHER-005 + AC-WEATHER-009 negative)
- **T-014**: `weather_current.go` 의 `init()` — `RegisterWebTool(&webWeatherCurrent{deps: &common.Deps{}, providerFactory: productionProviderFactory})`.
- **T-015**: schema meta-test 추가 (`schema_test.go` 에 weather_current entry 추가).
- **T-016**: register_test.go expectation 갱신: 14 → 15 (weather_current 추가).
- **T-017**: permission_integration_test.go 에 `TestFirstCallConfirm_WeatherCurrent` 추가 (host=`api.openweathermap.org`).
- **T-018**: audit_integration_test.go 에 `TestAuditLog_WeatherCurrentCall` 추가 (단일 호출 1 line, `tool: "weather_current"`).
- **T-019**: ratelimit_integration_test.go 에 `TestRateLimitExhausted_Weather` 추가 (provider `openweathermap`).
- **T-020**: doc.go 갱신 — weather 도구군 M1 진척 명시 (선택).
- **T-021**: `.moai/docs/weather-quickstart.md` 신규 — OpenWeatherMap key 발급 + `~/.goose/config/weather.yaml` 작성 가이드 (사용자 문서).
- **T-022**: `weather_geoip.go` 가 `ipapi.co` 호스트를 별도 permission scope 로 처리 (CapNet, scope=`ipapi.co`). audit reason=`geolocation` 명시.
- **T-023**: `singleflight` 의존성 검증 — 기존 `golang.org/x/sync/singleflight` import 여부 `go list -m all` 로 확인. 미존재 시 `go.mod` 추가 + plan.md §7 갱신.

### 2.3 입력 schema 상세 (weather_current)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 },
    "units": { "type": "string", "enum": ["metric", "imperial"], "default": "metric" },
    "lang": { "type": "string", "enum": ["ko", "en"], "default": "en" }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] },
    {}
  ]
}
```

세 번째 `{}` (빈 object) 는 모든 필수 필드 부재 → IP geolocation fallback 으로 진행. parser 단계에서 `config.weather.allow_ip_geolocation == false` 면 `invalid_input` 반환.

### 2.4 출력 data 상세 (weather_current)

```go
type WeatherReport struct {
    Location       Location  `json:"location"`
    Timestamp      time.Time `json:"timestamp"`
    TemperatureC   float64   `json:"temperature_c"`
    FeelsLikeC     float64   `json:"feels_like_c"`
    Condition      string    `json:"condition"`         // "clear" | "cloudy" | "rain" | "snow" | "thunderstorm" | "mist"
    ConditionLocal string    `json:"condition_local"`   // "맑음" if lang=ko, else English
    Humidity       int       `json:"humidity"`          // 0-100
    WindKph        float64   `json:"wind_kph"`
    WindDirection  string    `json:"wind_direction"`    // "N" | "NE" | ... | "NW"
    CloudCoverPct  int       `json:"cloud_cover_pct"`
    PrecipMm       float64   `json:"precip_mm"`
    UVIndex        float64   `json:"uv_index"`          // optional, M1 OWM 미제공 시 0
    SourceProvider string    `json:"source_provider"`   // "openweathermap" | "kma"
    Stale          bool      `json:"stale"`             // true: offline fallback used
    Message        string    `json:"message,omitempty"` // user-facing note ("오프라인 상태...")
}

type Location struct {
    Lat         float64 `json:"lat"`
    Lon         float64 `json:"lon"`
    DisplayName string  `json:"display_name"`
    Country     string  `json:"country"`
    Timezone    string  `json:"timezone"`
}
```

### 2.5 Provider 비교 (current weather)

| Provider | API | 한국 정확도 | 무료 quota | API key | M 도입 |
|---|---|---|---|---|---|
| **OpenWeatherMap (default)** | `api.openweathermap.org/data/2.5/weather` | 중 | 1,000/day | `OPENWEATHERMAP_API_KEY` | M1 |
| 기상청 (KMA) | `apis.data.go.kr/1360000/VilageFcstInfoService_2.0` | 상 (공식) | 10,000/day | data.go.kr 발급키 | M2 |

**M1 Default 선택 근거**: KMA API 키 발급이 공공데이터 포털 수동 승인 (즉시 발급 아님 → R1) 이라 사용자 진입 장벽 큼. M1 은 OWM 만으로 글로벌 공통 진입을 보장하고, M2 에서 한국 사용자 대상 KMA opt-in 도입.

### 2.6 Cache 키 정규화

좌표 소수점 2자리 반올림 (~1.1km 정밀도) 으로 cache key 정규화. 사용자가 같은 도시 내 50m 이동해도 cache hit 보장.

```
cache_key = sha256("weather_current:openweathermap:{round(lat,2)}:{round(lon,2)}:{units}:{lang}")
```

### 2.7 Offline Fallback 정책

1. API 성공 시 `~/.goose/cache/weather/latest-openweathermap-{lat2dp}-{lon2dp}.json` 저장 (atomic write: temp + rename, 0600).
2. API 실패 (네트워크 / 타임아웃 / 5xx) 시 파일 읽기.
3. 파일 나이 > 24h → `Stale=true` + `Message="데이터가 오래되었을 수 있어요 (마지막 확인: {timestamp})"`.
4. 파일 나이 ≤ 24h 시 `Stale=true` + `Message="오프라인 상태입니다 (마지막 확인: {timestamp})"`.
5. 파일 없음 → `ErrNoFallbackAvailable` (응답 `{ok: false, error.code == "fetch_failed", retryable: true}`).
6. 파일 corrupt JSON → 파일 evict + 같은 코드 흐름으로 `ErrNoFallbackAvailable`.

### 2.8 Singleflight 정책

같은 cache_key (위치 + units + lang) 에 대한 동시 요청은 `singleflight.Group.Do(key, fetch)` 로 dedup. 100 goroutine 동시 호출 시 외부 API 1회만.

---

## 3. M2 — weather_forecast + KMA 라우팅 (P2)

### 3.1 산출 파일 (high-level)

```
internal/tools/web/
├── weather_forecast.go         # weather_forecast 도구
├── weather_forecast_test.go
├── weather_kma.go              # KMAProvider (초단기실황 + 단기예보)
├── weather_kma_test.go
└── weather_route.go            # provider auto-routing (Country=="KR" → KMA)
```

### 3.2 Task breakdown (요약, Sprint 3 에서 상세화)

- KMA `apis.data.go.kr/1360000/VilageFcstInfoService_2.0` 4 endpoint wiring (초단기실황 / 초단기예보 / 단기예보 / 중기예보)
- `LatLonToGrid(lat, lon)` Lambert Conformal Conic 좌표 변환 + 5개 도시 goldenfile (서울/부산/제주/대전/강릉)
- `weather_forecast` 도구 (input `{location?, lat?, lon?, days: 1..7, units?, lang?}`)
- `provider: "auto"` 라우팅 (Country=="KR" → KMA, 그 외 OWM)
- forecast 응답 정규화: KMA 시간별 → 일별 high/low 추출

### 3.3 입력 schema 상세 (weather_forecast)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 },
    "days": { "type": "integer", "minimum": 1, "maximum": 7, "default": 3 },
    "units": { "type": "string", "enum": ["metric", "imperial"], "default": "metric" },
    "lang": { "type": "string", "enum": ["ko", "en"], "default": "en" }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] }
  ]
}
```

### 3.4 의존성 (M2 신규)

- 외부 의존성 신규 0 (stdlib `net/http` + 기존 인프라).
- 사용자 의존성: KMA data.go.kr API 키 발급.

---

## 4. M3 — weather_air_quality (Korean standard) (P3)

### 4.1 산출 파일 (high-level)

```
internal/tools/web/
├── weather_air_quality.go         # weather_air_quality 도구
├── weather_air_quality_test.go
└── weather_airkorea.go            # AirKoreaProvider (에어코리아)

testdata/weather/
└── airkorea-seoul-pm25-55.json
```

### 4.2 Task breakdown (요약)

- 에어코리아 `apis.data.go.kr/B552584/ArpltnInforInqireSvc/getMsrstnAcctoRltmMesureDnsty` wiring
- 한국 환경부 PM2.5 매핑 hardcoded:
  - 0-15 → "good" / "좋음"
  - 16-35 → "moderate" / "보통"
  - 36-75 → "unhealthy" / "나쁨"
  - 76+ → "very_unhealthy" / "매우 나쁨"
- `weather_air_quality` 도구 (input `{location?, lat?, lon?}`)
- 한국 외 좌표는 `unsupported_region` 반환 (M3 는 한국 only)

### 4.3 입력 schema 상세 (weather_air_quality)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "location": { "type": "string", "minLength": 1, "maxLength": 200 },
    "lat": { "type": "number", "minimum": -90, "maximum": 90 },
    "lon": { "type": "number", "minimum": -180, "maximum": 180 }
  },
  "anyOf": [
    { "required": ["location"] },
    { "required": ["lat", "lon"] }
  ]
}
```

---

## 5. 공통 인프라 (TOOLS-WEB-001 재사용)

### 5.1 Cache layer

- TOOLS-WEB-001 `common.Cache` (bbolt) 재사용.
- 위치: `~/.goose/cache/web/weather/cache.db` (TOOLS-WEB-001 layout 정렬).
- TTL: 10분 (current), 1h (forecast), 30분 (air_quality), 1h (geoip).
- Key: SHA256(canonical input).

### 5.2 Singleflight (in-flight dedup)

- `golang.org/x/sync/singleflight` (T-023 에서 의존성 확인).
- key 는 cache key 와 동일.
- bbolt cache → singleflight → API 순서.

### 5.3 Rate limit

- TOOLS-WEB-001 `RateTracker` per-provider:
  - `openweathermap`: 60/min
  - `kma`: 20/min (M2)
  - `airkorea`: 30/min (M3)
  - `ipapi`: 10/min
- exhausted 시 `{ok: false, error.code == "ratelimit_exhausted", error.retry_after_seconds, error.retryable: true}`.

### 5.4 Audit

- TOOLS-WEB-001 `EventTypeToolWebInvoke` 재사용.
- meta keys: `tool: "weather_current" | "weather_forecast" | "weather_air_quality"`, `host`, `provider`, `cache_hit`, `stale`, `duration_ms`, `outcome`.

### 5.5 Permission

- TOOLS-WEB-001 `permission.Manager.Check(CapNet, scope=host)` 재사용.
- 첫 호출 시 host (`api.openweathermap.org`, `apis.data.go.kr`, `ipapi.co`) 별 grant 요청.
- Deny 시 응답 `{ok: false, error.code == "permission_denied"}`, audit reason=`permission_denied`.

### 5.6 Blocklist

- TOOLS-WEB-001 `common.Blocklist` 재사용.
- 사용자가 weather provider host 를 blocklist 등록 시 `host_blocked` 거절.

### 5.7 API Key Redaction

- 모든 provider API key 는 zap log 에서 `****` 로 redact.
- request URL 로그 시 query string 의 `appid=` / `serviceKey=` 값 마스킹.
- error message 에 raw key 포함 금지 (REQ-WEATHER-004).

---

## 6. 외부 의존성 (M1 시점)

| 패키지 | 버전 (목표) | 신규 / 재사용 | 용도 |
|---|---|---|---|
| `go.etcd.io/bbolt` | v1.4.3 | 재사용 (TOOLS-WEB-001) | TTL 캐시 backend |
| `gopkg.in/yaml.v3` | latest | 재사용 (TOOLS-WEB-001) | weather.yaml 파싱 |
| `golang.org/x/time/rate` | latest | 재사용 (RATELIMIT-001) | rate limiter |
| `golang.org/x/sync/singleflight` | latest | T-023 검증 (기존 가능) | in-flight dedup |
| `go.uber.org/zap` | latest | 재사용 (CORE) | structured log |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | 재사용 (TOOLS-001) | input schema validation |

**신규 외부 의존성 (M1)**: 0 또는 1 (singleflight 기존 여부에 따라). M2/M3 는 모두 stdlib + 기존 의존성으로 구현.

---

## 7. 테스트 전략

### 7.1 단위 테스트

- 각 weather 파일마다 `*_test.go` (Go convention).
- HTTP mock: `httptest.NewServer` (TOOLS-WEB-001 `webWikipedia` 패턴 참조 — `hostBuilder` DI seam).
- 캐시 / offline / geoip / config 는 `t.TempDir()` + injected clock.

### 7.2 통합 테스트 (`integration_test.go` build tag)

- 실제 OpenWeatherMap 호출 (`OPENWEATHERMAP_API_KEY` 환경변수 필수, 없으면 `t.Skip`).
- CI 에서는 secret 으로 주입.
- 빈도: PR merge 시 + nightly.

### 7.3 Goldenfile

- `testdata/weather/owm-seoul-current.json`, `testdata/weather/ipapi-seoul.json` (M1).
- M2: `testdata/weather/kma-seoul-now.json`, `testdata/weather/kma-grid-cities.json` (5개 도시 좌표).
- M3: `testdata/weather/airkorea-seoul-pm25-55.json`.

### 7.4 E2E (PERMISSION + RATELIMIT + AUDIT 통합)

- bootstrap: 임시 grant store + audit log + tracker.
- 시나리오: 첫 호출 → grant 생성 → 두 번째 호출 캐시 hit → audit 2 line 검증.

### 7.5 보안 테스트

- AC-WEATHER-006 (API key redaction): zap observer 로 모든 log entry 캡처 후 raw key string 부재 검증.
- AC-WEATHER-007 (singleflight): 100 goroutine + mock 50ms 지연 + outbound counter == 1.
- AC-WEATHER-008 (ratelimit): exhausted state 강제 후 outbound 0회 검증.

---

## 8. Test plan 매핑 (AC ↔ Test file/function)

| AC | Test file | Test function |
|---|---|---|
| AC-WEATHER-001 | `register_test.go` | `TestRegistry_WithWeb_ListNames_IncludesWeatherCurrent` |
| AC-WEATHER-002 | `weather_current_test.go` | `TestWeatherCurrent_CacheHitWithin10Min` |
| AC-WEATHER-003 | `weather_current_test.go` + `weather_offline_test.go` | `TestWeatherCurrent_OfflineFallback_DiskRead` + `TestOfflineSaveLoad_RoundTrip` |
| AC-WEATHER-004 | `weather_route_test.go` (M2) | `TestAutoRoute_KRCountryUsesKMA` |
| AC-WEATHER-005 | `weather_air_quality_test.go` (M3) | `TestAirQuality_PM25_KoreanStandardMapping` |
| AC-WEATHER-006 | `weather_openweather_test.go` | `TestOWM_GetCurrent_APIKey_Redacted_NotInLogs` |
| AC-WEATHER-007 | `weather_current_test.go` | `TestWeatherCurrent_Singleflight_ConcurrentDedup` |
| AC-WEATHER-008 | `ratelimit_integration_test.go` | `TestRateLimitExhausted_Weather` |
| AC-WEATHER-009 | `register_test.go` + `schema_test.go` | `TestRegistry_WithWeb_IncludesWeather` + `TestAllToolSchemasValid_Weather` |
| AC-WEATHER-010 | `response_test.go` 또는 `weather_current_test.go` | `TestStandardResponseShape_Weather` |

M2 / M3 의 신규 Test function (AC-WEATHER-004 / 005) 는 해당 milestone 도입 시 RED → GREEN.

---

## 9. Risk mitigation 작업

| Risk | Plan-level 작업 |
|---|---|
| R1 KMA API 키 수동 승인 | M1 은 OWM 만 도입, M2 에서 KMA opt-in. weather-quickstart.md 안내 |
| R3 IP geolocation 부정확 | `location` param 명시 우선. parser 가 location/lat-lon 우선순위 enforce |
| R4 OWM quota 초과 | 10min TTL + per-provider rate tracker로 사전 차단 |
| R7 디스크 fallback corrupt | atomic write (temp + rename) + parse fail 시 evict |
| R8 bbolt 락 충돌 | TOOLS-WEB-001 common.Cache 의 5s timeout 그대로 적용. 락 실패 시 캐시 우회 (degraded but functional) |
| R9 register count 변동 | AC-WEATHER-009 가 milestone 별 카운트 명시. M1 단독 PR 시점에는 +1 만 검증 |

---

## 10. 종료 조건 (Plan-level DoD)

### M1 DoD

- [ ] T-001 ~ T-023 모든 task 완료.
- [ ] `weather_current` 도구가 `tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb())` 결과의 일부.
- [ ] AC-WEATHER-001/002/003/006/007/008/009 (M1 범위) + AC-WEATHER-010 모두 GREEN.
- [ ] Coverage ≥ 85% (weather_*.go 파일 기준).
- [ ] golangci-lint 0 warning, go vet 0 issue, gofmt clean.
- [ ] OpenWeatherMap key redaction 검증 (zap observer 로 negative test).
- [ ] PERMISSION-001 + RATELIMIT-001 + AUDIT-001 e2e GREEN.
- [ ] `.moai/docs/weather-quickstart.md` 작성 (사용자 가이드).
- [ ] TOOLS-WEB-001 8 도구 회귀 0 (기존 register_test.go 시나리오 영향 없음).

### M2 DoD (Sprint 3+)

- [ ] `weather_forecast` 도구 등록.
- [ ] KMAProvider 5개 도시 goldenfile 검증.
- [ ] AC-WEATHER-004 GREEN.
- [ ] provider="auto" 라우팅 동작.

### M3 DoD (Sprint 3+)

- [ ] `weather_air_quality` 도구 등록.
- [ ] 한국 PM2.5 매핑 검증 (4 boundary case).
- [ ] AC-WEATHER-005 GREEN.

---

Version: 0.1.0
Last Updated: 2026-05-10

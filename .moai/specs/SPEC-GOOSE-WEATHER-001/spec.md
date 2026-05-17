---
id: SPEC-GOOSE-WEATHER-001
version: 0.2.0
status: completed
created_at: 2026-04-22
updated_at: 2026-05-10
author: manager-spec
priority: P1
issue_number: null
phase: 7
size: 소(S)
lifecycle: spec-anchored
labels: [phase-7, daily-companion, weather, openweathermap, kma, air-quality, cache, offline]
---

# SPEC-GOOSE-WEATHER-001 — Weather Report Tool (Global + Korean, Cache, Air Quality, Offline Fallback)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 Daily Companion #32, TOOLS-001 확장) | manager-spec |
| 0.1.1 | 2026-05-10 | Sprint 2 진입 — TOOLS-WEB-001 common 인프라(Blocklist/Cache/Permission/Audit/RateLimit) 재사용 결정. 패키지 위치 `internal/ritual/weather/` → `internal/tools/web/weather*.go` 이전. Tool naming `Weather` (PascalCase) → `weather_current` / `weather_forecast` / `weather_air_quality` 3 도구 split (TOOLS-WEB-001 snake_case 컨벤션 정렬). M1=current(OWM) / M2=forecast+KMA / M3=air_quality(Korean standard). 신규 REQ-WEATHER-016 (표준 응답 shape) + AC-WEATHER-009 (registry count) + AC-WEATHER-010 (response shape). status `planned` → `audit-ready`. plan.md/acceptance.md/tasks.md/spec-compact.md/progress.md 신규 작성. | manager-spec |
| 0.2.0 | 2026-05-10 | **WEATHER-001 implementation 완수** — M1 (PR #154, weather_current OWM + cache + offline + IP geo) + M2 (PR #155, weather_forecast + KMAProvider + DFS_XY_CONV + auto-routing) + M3 (PR #156, weather_air_quality + AirKoreaProvider + Korean PM2.5 boundary) + sync (PR #157, status implemented → completed). 3 도구 모두 Registry 등록 (registry = builtin 6 + web 11 = **17 도구**). 누적 implemented AC: **10/10 GREEN**. coverage web 78.3% / common 92.1%. status: audit-ready → **completed**. | manager-docs |

---

## 1. 개요 (Overview)

MINK의 **날씨 정보 제공 tool 묶음**을 정의한다. BRIEFING-001의 아침 브리핑 구성요소 중 하나로, 사용자 위치의 현재·예보 날씨 + 미세먼지·강수확률·일출 시각을 조회한다. 본 SPEC은 TOOLS-WEB-001 의 common 인프라(Blocklist / Permission / Audit / RateLimit / bbolt TTL Cache)를 재사용하여 `internal/tools/web/` 패키지에 **3개 도구** (`weather_current`, `weather_forecast`, `weather_air_quality`) 를 등록하고, 두 provider(글로벌 OpenWeatherMap + 한국 기상청 KMA + 에어코리아) 중 설정에 따라 선택한다.

본 SPEC이 통과한 시점에서 `internal/tools/web/weather*.go` 군 (`weather_current.go`, `weather_forecast.go`, `weather_air_quality.go` 및 보조 파일) 은:

- `WeatherProvider` 인터페이스 + `OpenWeatherMapProvider` + `KMAProvider` 2 구현체 제공,
- 3 도구가 TOOLS-WEB-001 `RegisterWebTool()` 패턴으로 `init()` 시 자동 등록되어 모델이 `weather_current({location})`, `weather_forecast({location, days})`, `weather_air_quality({location})` 호출 가능,
- **10분 TTL bbolt 캐시** (TOOLS-WEB-001 `common.Cache` 재사용) 로 동일 위치 재호출 시 API quota 절약,
- **Offline fallback**: 네트워크 불가 시 마지막 성공 응답을 디스크 (`~/.goose/cache/weather/latest-{provider}-{lat2dp}-{lon2dp}.json`) 에서 stale flag 와 함께 반환,
- **표준 응답 shape**: TOOLS-WEB-001 `common.Response{ok, data|error, metadata}` 통합 (REQ-WEATHER-016),
- **한국 특화**: KMAProvider 선택 시 에어코리아 API 로 PM10/PM2.5 수집 + 한국 환경부 기준으로 normalized level 매핑.

**Milestone 분할** (점진적 도입):

- **M1** (P1): `weather_current` (OpenWeatherMap default + bbolt 캐시 + 디스크 offline fallback + IP geolocation). KMA 미포함.
- **M2** (P2): `weather_forecast` (OWM 7일 예보) + `provider: "auto"` 라우팅 (한국 좌표 → KMA, 그 외 → OWM).
- **M3** (P3): `weather_air_quality` (에어코리아 + 한국 환경부 PM2.5 기준).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "아침마다 오늘의 운세와 **날씨 정보**, 하루 일정을 브리핑." — 아침 브리핑의 3대 축 중 하나.
- 한국 사용자 대상: 기상청(KMA) 공공 API가 글로벌 서비스(OpenWeatherMap)보다 **정확하고 빠름**. 특히 미세먼지 정보는 한국 환경부 데이터가 우선.
- 날씨는 **행동·감정 adaptation**의 핵심 입력: 비오는 날 우울감 ↑ (adaptation.md §7 Mood), 미세먼지 "나쁨" → 마스크 리마인더 (adaptation.md §7.3).

### 2.2 상속 자산

- **TOOLS-001 Registry/Executor**: `Tool` 인터페이스, `init()` 자동 등록. 본 SPEC은 이 위에 `Weather` tool 하나 추가.
- **CONFIG-001**: `config.weather.*` 로드 (provider, api_key, default_location, cache_ttl).
- **OpenWeatherMap API v2.5/3.0**: 글로벌 11만+ 도시, 무료 1000 calls/day.
- **기상청 OpenAPI**: 초단기실황/단기예보/중기예보 (데이터 포털 `data.go.kr` 발급키 필요).

### 2.3 범위 경계

- **IN**: `WeatherProvider` 인터페이스, OpenWeatherMap + KMA 2 provider, `Weather` tool (TOOLS-001), cache, offline fallback, 위치 감지 (GPS unavailable 시 IP-based).
- **OUT**: Push 알림 ("비올 예정" 알람 — BRIEFING-001), 날씨 기반 ML 예측, 위성 이미지, 레이더 데이터, historical weather archive.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/tools/web/weather*.go` 파일군 (TOOLS-WEB-001 패키지 재사용; 별도 sub-package 미생성).
2. `WeatherProvider` 인터페이스: `GetCurrent`, `GetForecast`, `GetAirQuality`, `GetSunTimes`, `Name`.
3. `OpenWeatherMapProvider` 구현 (en/ko 응답, Celsius/Fahrenheit, metric/imperial).
4. `KMAProvider` 구현 (초단기실황 + 단기예보 API 조합, 한국 좌표계 nx/ny 변환). **M2 시점 도입.**
5. `WeatherReport` DTO + `WeatherForecastDay` DTO + `AirQuality` DTO + `SunTimes` DTO.
6. `Location` 타입 (lat/lon + display_name + timezone + country).
7. 캐시: TOOLS-WEB-001 `common.Cache` (bbolt + TTL 10분) 재사용. M1 별도 LRU 미도입.
8. Offline fallback: 마지막 성공 응답 디스크 저장 (`~/.goose/cache/weather/latest-{provider}-{lat2dp}-{lon2dp}.json`, 0600) + stale flag.
9. IP-based geolocation fallback (GPS/coord 미제공 시): `ipapi.co` HTTPS free tier (1000/day) — TTL 1h 캐시. M1 도입.
10. TOOLS-WEB-001 register pattern 으로 3 도구 등록:
    - `weather_current`: input `{location?, lat?, lon?, units?, lang?}`, output `WeatherReport` JSON
    - `weather_forecast` (M2): input `{location?, lat?, lon?, days: 1..7, units?, lang?}`, output `[]WeatherForecastDay` JSON
    - `weather_air_quality` (M3): input `{location?, lat?, lon?}`, output `AirQuality` JSON
11. Rate limit: TOOLS-WEB-001 `RateTracker` per-provider (`openweathermap` 60/min, `kma` 20/min, `airkorea` 30/min, `ipapi` 10/min).
12. Permission: TOOLS-WEB-001 `Manager.Check(CapNet, scope=host)` 첫 호출 동의 + grant cache 재사용 (provider host 별 grant).
13. Audit: TOOLS-WEB-001 `EventTypeToolWebInvoke` 재사용 (모든 호출 기록, tool 명에 `weather_*` 표기).
14. Blocklist: TOOLS-WEB-001 `common.Blocklist` 재사용 (provider 호스트가 blocklist 매칭 시 host_blocked 거절).
15. 표준 응답 shape: TOOLS-WEB-001 `common.Response{ok, data|error, metadata}` 통합. `data` 안에 `WeatherReport.{Stale, CacheHit}` flag 포함 (응답 일관성 + offline 정보 보존 양립).
16. Config (`~/.goose/config/weather.yaml`):
    - `weather.provider: "auto"|"openweathermap"|"kma"` (auto=Location.Country=="KR" 이면 KMA, 그 외 OWM. M1 은 항상 openweathermap)
    - `weather.openweathermap.api_key`
    - `weather.kma.api_key`  (M2)
    - `weather.airkorea.api_key`  (M3)
    - `weather.default_location: "Seoul,KR"`
    - `weather.cache_ttl: "10m"`
    - `weather.allow_ip_geolocation: true`

### 3.2 OUT OF SCOPE

- Push notification ("비올 예정" 알림 전송): BRIEFING-001 / 외부 notification layer.
- 위성 이미지 / 레이더 raw data: scope 외.
- Historical weather (과거 30일+): 별도 SPEC.
- Weather-based ML prediction / 이상기후 감지: 별도 SPEC.
- Marine / aviation weather: 일반 사용자 대상 아님.
- 지진·태풍 특보 (Disaster Alert): 별도 Emergency SPEC (생명 안전 관련은 신중히).
- TOOLS-WEB-001 의 8 web 도구 변경 (본 SPEC 은 추가만 함; 기존 도구 동작 / schema / 인프라 수정 없음).
- 사용자별 customized forecast 모델.
- Pollen 데이터 외부 API (R6.1 — 한국 꽃가루 정보는 M3 이후 별도 SPEC).
- weather.yaml 의 Hot reload (변경 즉시 반영). M1~M3 은 startup 1회 로드.
- KMA API 키 자동 발급 / OAuth (사용자 수동 발급).

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-WEATHER-001 [Ubiquitous]** — Each weather tool (`weather_current`, `weather_forecast`, `weather_air_quality`) **shall** register itself into the TOOLS-WEB-001 global web tool list via `RegisterWebTool(t)` from its file's `init()`; `tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb())` **shall** include the registered weather tools, and `Resolve(name)` **shall** return a non-nil `Tool` whose `Scope() == ScopeShared`.

**REQ-WEATHER-002 [Ubiquitous]** — All `weather_current` successful response `data` payloads (TOOLS-WEB-001 `common.Response.Data`) **shall** include the fields: `{location, timestamp, temperature_c, condition, humidity, wind_kph, source_provider, stale, message}`; `data.stale=true` indicates offline disk fallback was used (the live bbolt cache state is reflected separately in `metadata.cache_hit`).

**REQ-WEATHER-003 [Ubiquitous]** — The provider interface **shall** emit structured zap logs `{provider, lat, lon, latency_ms, cache_hit, stale, api_status}` for every call.

**REQ-WEATHER-004 [Ubiquitous]** — API keys **shall** never appear in log output, tool invocation payloads, or error messages; redaction **shall** replace the key with `****` before any serialization.

### 4.2 Event-Driven

**REQ-WEATHER-005 [Event-Driven]** — **When** `weather_current.Call({location|lat,lon})` is invoked, the tool **shall** (a) check the bbolt cache with key `sha256("weather_current:{provider}:{lat_rounded_2dp}:{lon_rounded_2dp}:{units}:{lang}")`, (b) on live cache hit (fresh < TTL) return cached, (c) on miss invoke the provider via `singleflight.Group`, (d) on provider success store to bbolt cache (10min TTL) and atomically save to disk fallback file, (e) on provider failure load the last disk-saved value with `data.stale=true`. Provider host **shall** be subject to Blocklist + Permission + RateLimit gates before any outbound call.

**REQ-WEATHER-006 [Event-Driven]** — **When** `config.weather.provider == "auto"` and `Location.Country == "KR"`, the dispatcher **shall** route to `KMAProvider`; otherwise to `OpenWeatherMapProvider`.

**REQ-WEATHER-007 [Event-Driven]** — **When** GPS/coordinates are not provided in the tool input AND `config.weather.allow_ip_geolocation == true`, the provider **shall** perform one IP-geolocation lookup and cache the result for 1 hour.

**REQ-WEATHER-008 [Event-Driven]** — **When** `weather_air_quality.Call({location|lat,lon})` is invoked (M3) and Korea coordinates are detected, the tool **shall** query the AirKorea API for PM10/PM2.5 and return normalized `AirQuality{level: "good"|"moderate"|"unhealthy"|"very_unhealthy"}` (한국 환경부 기준 boundaries 15/35/75); coordinates outside Korea **shall** return `unsupported_region` (M3 도구는 한국 only).

**REQ-WEATHER-009 [Event-Driven]** — **When** per-provider rate limit (TOOLS-WEB-001 `RateTracker`) is exceeded, the tool **shall** return `{ok: false, error.code == "ratelimit_exhausted", error.retry_after_seconds, error.retryable: true}` immediately without invoking the provider; an audit event **shall** be recorded with `outcome: "denied", reason: "ratelimit_exhausted"`.

### 4.3 State-Driven

**REQ-WEATHER-010 [State-Driven]** — **While** the last API call timestamp is older than 24 hours AND network is unavailable, offline fallback **shall** return `stale=true` with a warning in `WeatherReport.Message` field; consumers (BRIEFING-001) are responsible for tone adjustment.

**REQ-WEATHER-011 [State-Driven]** — **While** `config.weather.kma.api_key == ""` and provider is forced to `kma`, `WeatherProvider` initialization **shall** fail with `ErrMissingAPIKey`; auto mode in the same condition **shall** silently fall back to `OpenWeatherMapProvider`.

### 4.4 Unwanted Behavior

**REQ-WEATHER-012 [Unwanted]** — The provider **shall not** make more than one concurrent request to the same API endpoint for the same coordinates; in-flight requests **shall** be de-duplicated via `singleflight.Group`.

**REQ-WEATHER-013 [Unwanted]** — The provider **shall not** panic on malformed API responses; JSON parse failures **shall** return `ErrInvalidResponse` with the raw response preserved in error context for debugging.

### 4.5 Optional

**REQ-WEATHER-014 [Optional]** — **Where** `config.weather.include_pollen == true` and provider supports it (KMA for Korea only), the response **shall** include `pollen: {level, dominant_type}` during pollen season (3-5월, 9-10월).

**REQ-WEATHER-015 [Optional]** — **Where** `config.weather.forecast_days > 0` (max 7), `GetForecast` **shall** return `[]WeatherForecastDay` with high/low temp, precipitation probability, and condition per day.

### 4.6 표준 응답 / 등록 (Sprint 2 신규)

**REQ-WEATHER-016 [Ubiquitous]** — All 3 weather tools (`weather_current`, `weather_forecast`, `weather_air_quality`) **shall** return TOOLS-WEB-001 standard `common.Response{ok, data|error, metadata}` shape; the data payload of a successful `weather_current` call **shall** be a JSON-serialized `WeatherReport`, the `weather_forecast` data **shall** be `{days: []WeatherForecastDay}`, and the `weather_air_quality` data **shall** be `AirQuality`. The `metadata.cache_hit` field **shall** reflect bbolt cache hit; offline disk fallback **shall** still set `data.stale=true` while keeping `metadata.cache_hit=false` (cache_hit refers to the live in-process cache only).

**REQ-WEATHER-017 [Ubiquitous]** — All 3 weather tools **shall** register themselves into the global web tool list via TOOLS-WEB-001 `RegisterWebTool(t)` from each tool file's `init()`; `tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb())` **shall** therefore include all 3 weather tool names alongside the existing TOOLS-WEB-001 8 web tools (total 14+3 = 17 names when all milestones are complete). Until M2/M3 land, only the milestones implemented at the time count toward the total.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-WEATHER-001 — Tool 자동 등록**
- **Given** TOOLS-001 registry 초기화 완료
- **When** `registry.ListNames()` 반환값 확인
- **Then** `"Weather"` 포함, schema가 `{location?, lat?, lon?, units?, lang?}` 유효 JSON Schema.

**AC-WEATHER-002 — Cache 히트**
- **Given** Seoul 좌표로 `GetCurrent` 1회 호출 완료, cache 저장됨
- **When** 5분 뒤 동일 좌표로 재호출
- **Then** 외부 API 호출 0회, 결과에 `cache_hit=true`.

**AC-WEATHER-003 — Offline fallback**
- **Given** mock 네트워크가 `net.Dial` error 반환, 디스크에 `latest.json` 존재 (12시간 전 데이터)
- **When** `GetCurrent` 호출
- **Then** `err==nil`, `report.Stale=true`, `report.Message` 에 "오프라인" 문구 포함.

**AC-WEATHER-004 — 한국 좌표 자동 KMA 라우팅**
- **Given** `config.weather.provider="auto"`, `location="Seoul,KR"`
- **When** `GetCurrent` 호출
- **Then** `report.SourceProvider == "kma"`, OpenWeatherMap mock은 호출 0회.

**AC-WEATHER-005 — 미세먼지 normalized level**
- **Given** KMAProvider, mock 에어코리아 응답 PM2.5=55 μg/m³
- **When** `GetAirQuality("Seoul")` 호출
- **Then** `aq.Level == "unhealthy"` (한국 기준 36-75 "나쁨" → unhealthy 매핑).

**AC-WEATHER-006 — API key 로그 미노출**
- **Given** zap logger에 구조화 출력 캡처
- **When** `GetCurrent` 호출 with `OPENWEATHERMAP_KEY=secret123`
- **Then** 캡처된 로그 문자열에 `"secret123"` 미포함, `"****"` 포함.

**AC-WEATHER-007 — singleflight 중복 제거**
- **Given** 동일 좌표로 100 goroutine 동시 `GetCurrent` 호출, mock API 50ms 지연
- **When** 모든 goroutine 완료
- **Then** 외부 API 호출 정확히 1회, 100 goroutine 모두 동일 결과 수신.

**AC-WEATHER-008 — Rate limit 차단**
- **Given** per-provider limit 60/min, 이미 60회 호출된 상태
- **When** 61번째 `GetCurrent` 호출
- **Then** 외부 API 호출 0회, 응답 `{ok: false, error.code == "ratelimit_exhausted", error.retry_after_seconds > 0, error.retryable: true}` (TOOLS-WEB-001 표준 매핑).

**AC-WEATHER-009 — Registry inventory (3 도구)**
- **Given** TOOLS-WEB-001 와 본 SPEC 모두 init() 완료
- **When** `tools.NewRegistry(tools.WithBuiltins(), tools.WithWeb()).ListNames()` 호출
- **Then** built-in 6 + TOOLS-WEB-001 의 8 + 본 SPEC 의 (M1 시점 1, M2 시점 2, M3 시점 3) = M3 완료 시 17 names. 정확한 weather subset 정렬: `["weather_air_quality", "weather_current", "weather_forecast"]` (M3 완료 시).
- 각 weather 도구를 `Resolve(name)` 했을 때 non-nil + `Tool.Scope() == ScopeShared`.

**AC-WEATHER-010 — 표준 응답 shape (TOOLS-WEB-001 통합)**
- **Given** 3 weather 도구 각각 mock 환경에서 성공 1회 + 실패 1회 케이스 (총 6 케이스)
- **When** 6 케이스 모두 호출 후 응답을 `json.Unmarshal` 로 `common.Response` 구조체에 매핑
- **Then** 6/6 케이스 unmarshal 성공. 모든 응답에 정확히 top-level keys `{ok, data|error, metadata}`. `metadata` 는 `{cache_hit, duration_ms}`. 성공 응답의 `data.stale` 와 `metadata.cache_hit` 는 동시에 true 가 될 수 있음 (offline live cache miss + disk hit) — 또는 (live cache hit, stale=false) 조합. 단 `cache_hit=true` && `stale=true` 는 발생 불가 (live hit 는 항상 fresh).

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃 (Sprint 2 갱신 — TOOLS-WEB-001 패키지 재사용)

```
internal/tools/web/
├── (TOOLS-WEB-001 기존 파일군 — http.go, search.go, browse.go, ...)
├── (M1) weather_current.go              # weather_current 도구 (TOOLS-WEB-001 Tool 인터페이스)
├── (M1) weather_current_test.go
├── (M1) weather_provider.go             # WeatherProvider 인터페이스
├── (M1) weather_openweather.go          # OpenWeatherMapProvider
├── (M1) weather_openweather_test.go
├── (M1) weather_types.go                # WeatherReport, Location, AirQuality, SunTimes, Pollen DTO
├── (M1) weather_geoip.go                # ipapi.co IP geolocation (1h TTL)
├── (M1) weather_geoip_test.go
├── (M1) weather_offline.go              # 디스크 fallback (~/.goose/cache/weather/latest-*.json)
├── (M1) weather_offline_test.go
├── (M1) weather_config.go               # ~/.goose/config/weather.yaml loader
├── (M2) weather_forecast.go             # weather_forecast 도구
├── (M2) weather_forecast_test.go
├── (M2) weather_kma.go                  # KMAProvider (초단기실황 + 단기예보 + DFS_XY_CONV)
├── (M2) weather_kma_test.go
├── (M3) weather_air_quality.go          # weather_air_quality 도구 + 에어코리아 + 한국 PM2.5 매핑
├── (M3) weather_air_quality_test.go
└── (테스트 공통) testdata/
    ├── owm-seoul-current.json
    ├── kma-seoul-now.json
    └── airkorea-seoul-pm25-55.json
```

**재사용**: TOOLS-WEB-001 의 `common.Cache` (bbolt), `common.Blocklist`, `common.Deps` (DI), `common.Response` (응답 wrapper), `common.UserAgent()`, `RegisterWebTool()`, `RateTracker`, `permission.Manager.Check`, `audit.EventTypeToolWebInvoke` 모두 그대로 사용. 본 SPEC 은 weather 도메인 변환 (좌표/단위/언어/standard mapping) 만 추가.

### 6.2 핵심 타입

```go
type WeatherProvider interface {
    Name() string
    GetCurrent(ctx context.Context, loc Location) (*WeatherReport, error)
    GetForecast(ctx context.Context, loc Location, days int) ([]WeatherForecastDay, error)
    GetAirQuality(ctx context.Context, loc Location) (*AirQuality, error)
    GetSunTimes(ctx context.Context, loc Location, date time.Time) (*SunTimes, error)
}

type Location struct {
    Lat         float64
    Lon         float64
    DisplayName string   // "서울특별시 강남구"
    Country     string   // "KR"
    Timezone    string   // "Asia/Seoul"
}

type WeatherReport struct {
    Location       Location
    Timestamp      time.Time
    TemperatureC   float64
    FeelsLikeC     float64
    Condition      string   // "clear" | "cloudy" | "rain" | "snow" | "thunderstorm"
    ConditionKo    string   // "맑음" etc.
    Humidity       int      // 0-100
    WindKph        float64
    WindDirection  string   // "N" | "NE" | ...
    CloudCoverPct  int
    PrecipMm       float64
    UVIndex        float64
    AirQuality     *AirQuality     // optional
    Pollen         *Pollen         // optional, Korea only
    SunTimes       *SunTimes       // optional
    SourceProvider string
    CacheHit       bool
    Stale          bool
    Message        string
}

type AirQuality struct {
    Level      string   // "good" | "moderate" | "unhealthy" | "very_unhealthy" | "hazardous"
    LevelKo    string   // "좋음" | "보통" | "나쁨" | "매우 나쁨" | "위험"
    PM10       int
    PM25       int
    O3         float64
    NO2        float64
}

type WeatherTool struct {
    providers map[string]WeatherProvider
    cache     *WeatherCache
    ratelimit *RateLimiter
    cfg       Config
    logger    *zap.Logger
}

func (w *WeatherTool) Name() string { return "Weather" }
func (w *WeatherTool) Schema() json.RawMessage { /* JSON Schema */ }
func (w *WeatherTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error)
```

### 6.3 KMA 좌표 변환

기상청 API는 lat/lon 대신 nx/ny 격자 좌표 사용. Lambert Conformal Conic 투영 변환:

```go
// DFS_XY_CONV: 기상청 공식 좌표 변환 알고리즘
// - 격자 간격 5km
// - 서울 = nx:60, ny:127
func LatLonToGrid(lat, lon float64) (nx, ny int) {
    // Lambert Conformal Conic projection
    // 상세 알고리즘은 research.md 참조
}
```

### 6.4 한국 미세먼지 기준 매핑

| PM2.5 (μg/m³) | Level | Korean |
|--------------|-------|--------|
| 0-15 | good | 좋음 |
| 16-35 | moderate | 보통 |
| 36-75 | unhealthy | 나쁨 |
| 76+ | very_unhealthy | 매우 나쁨 |

### 6.5 Offline Fallback 정책

1. API 성공 시 `~/.goose/cache/weather/{provider}-{lat2dp}-{lon2dp}.json` 저장
2. API 실패 (네트워크 / 타임아웃 / 5xx) 시 파일 읽기
3. 파일 나이 > 24시간 → WARN 로그 + stale=true + Message="데이터가 오래되었을 수 있어요"
4. 파일 없음 → `ErrNoFallbackAvailable` 반환

### 6.6 라이브러리 결정 (Sprint 2 갱신)

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| HTTP client | stdlib `net/http` + 5s timeout context | 최소 의존, TOOLS-WEB-001 패턴 |
| TTL Cache | TOOLS-WEB-001 `common.Cache` (bbolt v1.4.3 기존) | 재사용 |
| Singleflight | `golang.org/x/sync/singleflight` (M1 신규 평가) | 중복 요청 제거. 기존 의존성 확인 필요 |
| JSON | `encoding/json` (stdlib) | nested path 시 직접 struct unmarshal |
| IP geolocation | 직접 HTTP (`ipapi.co/json/`) | SDK 불필요, M1 `weather_geoip.go` |
| Rate limit | TOOLS-WEB-001 `RateTracker` (`golang.org/x/time/rate` 기존) | 재사용 |
| YAML config | `gopkg.in/yaml.v3` (TOOLS-WEB-001 `LoadWebConfig` 패턴) | 기존 의존성 |

**신규 외부 의존성 (M1 시점)**: 0 (TOOLS-WEB-001 인프라 + stdlib 만으로 충분). singleflight 도 `go list -m all` 검증으로 기존 가능성 높음. 신규 발견 시 plan.md §7 에 명시.

### 6.7 TDD 진입 순서 (M1 우선)

1. RED: `TestWeatherCurrent_Registered` — AC-WEATHER-001 (M1, weather_current 등록)
2. RED: `TestWeatherCurrent_StandardResponseShape` — AC-WEATHER-010 (M1, common.Response)
3. RED: `TestWeatherCurrent_CacheHitWithin10Min` — AC-WEATHER-002 (M1, bbolt cache)
4. RED: `TestWeatherCurrent_OfflineFallback_DiskRead` — AC-WEATHER-003 (M1)
5. RED: `TestWeatherCurrent_APIKey_NotInLogs` — AC-WEATHER-006 (M1)
6. RED: `TestWeatherCurrent_Singleflight_ConcurrentDedup` — AC-WEATHER-007 (M1)
7. RED: `TestWeatherCurrent_RateLimit_Exhausted` — AC-WEATHER-008 (M1)
8. RED: `TestRegistry_WithWeb_IncludesWeather` — AC-WEATHER-009 (M1, registry inventory)
9. M2 추가: `TestWeatherForecast_*` + `TestAutoRoute_KRCountryUsesKMA` — AC-WEATHER-004
10. M3 추가: `TestWeatherAirQuality_PM25_KoreanStandardMapping` — AC-WEATHER-005
11. GREEN → REFACTOR

### 6.8 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, mock HTTP client, 캐시/offline 격리 테스트 |
| **R**eadable | provider 인터페이스 + 2 구현체 명확 분리 |
| **U**nified | JSON Schema strict, 응답 DTO 통일 (KMA/OWM 변환 후 동일 스키마) |
| **S**ecured | API key 로그 redaction (REQ-004), rate limit, 요청 타임아웃 5s |
| **T**rackable | 모든 호출 zap 로그, cache hit ratio 통계 주기적 덤프 |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | TOOLS-001 v0.1.2 (completed) | Tool/Registry/Executor 인터페이스 |
| 선행 SPEC | TOOLS-WEB-001 (M1 implemented v0.1.0; M2~M4 진행 중) | common.Cache + common.Blocklist + common.Deps + common.Response + RegisterWebTool + RateTracker 인프라 재사용 |
| 선행 SPEC | PERMISSION-001 v0.2.0 (completed) | Manager.Check(CapNet, scope=host) + grant cache |
| 선행 SPEC | AUDIT-001 v0.1.0 (completed) | EventTypeToolWebInvoke + AuditWriter |
| 선행 SPEC | RATELIMIT-001 v0.2.0 (completed) | Tracker per-provider |
| 선행 SPEC | FS-ACCESS-001 (planned) | `~/.goose/cache/weather/**` write 허용 (디스크 fallback) — M1 시점 default seed 미포함 시 첫 write 동의 흐름 |
| 선행 SPEC | CONFIG-001 (planned) | weather.yaml loader (M1 은 TOOLS-WEB-001 `LoadWebConfig` 패턴 모방) |
| 후속 SPEC | BRIEFING-001 | 아침 브리핑 구성요소 (consumer) |
| 후속 SPEC | HEALTH-001 | 미세먼지 나쁨 → 마스크 리마인더 (M3 consumer) |
| 외부 | OpenWeatherMap API v2.5 (current) / v3.0 (onecall) | 글로벌, M1 |
| 외부 | 기상청 OpenAPI (data.go.kr `VilageFcstInfoService_2.0`) | 한국, M2 |
| 외부 | 에어코리아 API | 한국 미세먼지, M3 |
| 외부 | `ipapi.co` (free tier) | IP geolocation, M1 |
| 외부 (재사용) | `go.etcd.io/bbolt` v1.4.3 | TOOLS-WEB-001 common.Cache backend |
| 외부 (재사용) | `golang.org/x/time/rate` | TOOLS-WEB-001 RateTracker |
| 외부 (재사용) | `gopkg.in/yaml.v3` | weather.yaml |
| 외부 (M1 평가) | `golang.org/x/sync/singleflight` | 동시 dedup. 기존 의존성이면 신규 0, 아니면 신규 1 |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | KMA API 키 발급이 공공데이터 포털에서 수동 승인 필요 (즉시 발급 아님) | 고 | 중 | M1 은 OpenWeatherMap 만, KMA 는 M2 opt-in. `.moai/docs/weather-quickstart.md` 에 발급 절차 안내 |
| R2 | KMA 좌표 변환 오차 (격자 5km) | 낮 | 낮 | 서울 주요 25개 구별 좌표 goldenfile 테스트 (M2) |
| R3 | IP geolocation 부정확 (VPN/프록시) | 중 | 중 | `location` param 명시 우선, 실패 시 명확한 에러 코드 `geolocation_failed` 반환 |
| R4 | OpenWeatherMap 무료 quota 초과 (1000/day) | 중 | 중 | bbolt 캐시 10min TTL, 단일 위치 평균 30회/day → 충분. quota 도달 시 `ratelimit_exhausted` 응답 |
| R5 | 에어코리아 API 다운타임 | 중 | 낮 | AirQuality는 M3 의 별도 도구, 다른 도구 영향 없음. 응답 `{ok: false, error.code == "fetch_failed", retryable: true}` |
| R6 | 미세먼지 기준 해외 표준과 한국 표준 혼용 | 낮 | 중 | provider별 기준 명시, weather_air_quality 는 한국 환경부 기준 hardcoded (M3) |
| R7 | 디스크 fallback 파일이 corrupt / 부분 write | 낮 | 낮 | atomic write (temp file + rename), JSON parse fail 시 disk evict + ErrNoFallbackAvailable |
| R8 | bbolt 파일 락 충돌 (다중 MINK 인스턴스) | 낮 | 중 | bbolt `Options{Timeout: 5s}` 적용 (TOOLS-WEB-001 common.Cache 패턴), 락 실패 시 캐시 우회 (degraded but functional) |
| R9 | TOOLS-WEB-001 M2~M4 미완료 시 본 SPEC 의 register count 변동 | 중 | 낮 | AC-WEATHER-009 가 "M3 완료 시 17 names" 로 명시. M1 단독 PR 시점에는 weather_current 1 개만 추가 검증 |
| R10 | weather.yaml schema 가 web.yaml 과 file 충돌 | 낮 | 낮 | 별도 파일 (`~/.goose/config/weather.yaml`) 사용. TOOLS-WEB-001 `LoadWebConfig` 패턴 모방 |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-TOOLS-001/spec.md` — builtin tool registration
- `.moai/specs/SPEC-GOOSE-BRIEFING-001/spec.md` — consumer
- `.moai/project/adaptation.md` §7 Mood Detection (날씨 → 기분)

### 9.2 외부 참조

- OpenWeatherMap API: https://openweathermap.org/api
- 기상청 공공데이터 포털: https://data.kma.go.kr/
- 에어코리아: https://www.airkorea.or.kr/web/last_amb_hour_data
- KMA 좌표계 (DFS_XY_CONV): https://www.kma.go.kr/down/NWP_Manual.pdf
- 한국 미세먼지 기준: https://www.airkorea.or.kr/web/khaiInfo

### 9.3 부속 문서

- `./research.md` — Provider 선정 상세, 좌표 변환 알고리즘, 미세먼지 매핑 goldenfile

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Push 알림 전송을 포함하지 않는다** (비올 예정 알람 등). BRIEFING-001 또는 Gateway SPEC.
- 본 SPEC은 **위성 이미지·레이더 raw data 처리를 포함하지 않는다**.
- 본 SPEC은 **Historical weather를 제공하지 않는다** (현재 + 7일 예보만).
- 본 SPEC은 **지진·태풍·쓰나미 특보를 포함하지 않는다**. 생명 안전 관련은 별도 Emergency SPEC.
- 본 SPEC은 **해양·항공 기상을 포함하지 않는다**.
- 본 SPEC은 **기계학습 예측 모델을 포함하지 않는다** (외부 API 응답 그대로 전달).
- 본 SPEC은 **실시간 weather streaming을 지원하지 않는다** (polling only).
- 본 SPEC은 **weather-based proactive alert를 자체 생성하지 않는다** (BRIEFING-001이 조합).
- 본 SPEC은 **사용자 customized forecast 모델을 포함하지 않는다**.

---

**End of SPEC-GOOSE-WEATHER-001**

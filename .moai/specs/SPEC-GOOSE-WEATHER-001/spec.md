---
id: SPEC-GOOSE-WEATHER-001
version: 0.1.0
status: Planned
created: 2026-04-22
updated: 2026-04-22
author: manager-spec
priority: P1
issue_number: null
phase: 7
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GOOSE-WEATHER-001 — Weather Report Tool (Global + Korean, Cache, Air Quality, Offline Fallback)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 Daily Companion #32, TOOLS-001 확장) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE의 **날씨 정보 제공 tool**을 정의한다. BRIEFING-001의 아침 브리핑 구성요소 중 하나로, 사용자 위치의 현재·오늘·내일 날씨 + 미세먼지·꽃가루·자외선·강수확률 + 일출·일몰 시간을 조회한다. 본 SPEC은 TOOLS-001의 builtin tool registry에 `Weather` tool을 등록하고, 두 provider(글로벌 OpenWeatherMap + 한국 기상청 KMA) 중 설정에 따라 선택한다.

본 SPEC이 통과한 시점에서 `internal/ritual/weather/` 패키지는:

- `WeatherProvider` 인터페이스 + `OpenWeatherMapProvider` + `KMAProvider` 2 구현체 제공,
- `Weather` tool이 TOOLS-001 builtin registry에 `init()` 시 등록되어 모델이 `weather.Get(location)` 호출 가능,
- **10분 TTL 메모리 캐시**로 동일 위치 재호출 시 API quota 절약,
- **Offline fallback**: 네트워크 불가 시 마지막 성공 응답을 stale flag와 함께 반환,
- **한국 특화**: KMAProvider 선택 시 미세먼지(PM10/PM2.5) + 황사 + 꽃가루 농도 추가 수집.

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

1. `internal/ritual/weather/` 패키지.
2. `WeatherProvider` 인터페이스: `GetCurrent`, `GetForecast`, `GetAirQuality`, `GetSunTimes`, `Name`.
3. `OpenWeatherMapProvider` 구현 (en/ko 응답, Celsius/Fahrenheit, metric/imperial).
4. `KMAProvider` 구현 (초단기실황 + 단기예보 API 조합, 한국 좌표계 nx/ny 변환).
5. `WeatherReport` DTO + `WeatherForecastDay` DTO + `AirQuality` DTO + `SunTimes` DTO.
6. `Location` 타입 (lat/lon + display_name + timezone).
7. 캐시: in-memory LRU + 10분 TTL. `github.com/hashicorp/golang-lru/v2`.
8. Offline fallback: 마지막 성공 응답 디스크 저장 (`~/.goose/cache/weather/latest.json`) + stale flag.
9. IP-based geolocation fallback (GPS/coord 미제공 시): `ip-api.com` 또는 `ipapi.co` free tier.
10. TOOLS-001 `Weather` tool 등록:
    - Input schema: `{location?: string, lat?: float, lon?: float, units?: "metric"|"imperial", lang?: "ko"|"en"}`
    - Output: `WeatherReport` JSON
11. Rate limit: per-provider, default OpenWeather 60/min, KMA 20/min.
12. Config:
    - `weather.provider: "auto"|"openweathermap"|"kma"` (auto=위치 한국이면 KMA)
    - `weather.openweathermap.api_key`
    - `weather.kma.api_key`
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

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-WEATHER-001 [Ubiquitous]** — The `Weather` tool **shall** register itself in the TOOLS-001 builtin registry via `init()` with canonical name `Weather`; attempts to access before initialization **shall** return `ErrToolNotReady`.

**REQ-WEATHER-002 [Ubiquitous]** — All weather responses **shall** include the fields: `{location, timestamp, temperature_c, condition, humidity, wind_kph, source_provider, cache_hit, stale}`; `stale=true` indicates offline fallback was used.

**REQ-WEATHER-003 [Ubiquitous]** — The provider interface **shall** emit structured zap logs `{provider, lat, lon, latency_ms, cache_hit, stale, api_status}` for every call.

**REQ-WEATHER-004 [Ubiquitous]** — API keys **shall** never appear in log output, tool invocation payloads, or error messages; redaction **shall** replace the key with `****` before any serialization.

### 4.2 Event-Driven

**REQ-WEATHER-005 [Event-Driven]** — **When** `Weather.GetCurrent(location)` is called, the provider **shall** (a) check cache with key `{provider, lat_rounded_2dp, lon_rounded_2dp}`, (b) on hit (fresh < TTL) return cached, (c) on miss call external API, (d) on API success store to cache and disk, (e) on API failure return last disk-saved value with `stale=true`.

**REQ-WEATHER-006 [Event-Driven]** — **When** `config.weather.provider == "auto"` and `Location.Country == "KR"`, the dispatcher **shall** route to `KMAProvider`; otherwise to `OpenWeatherMapProvider`.

**REQ-WEATHER-007 [Event-Driven]** — **When** GPS/coordinates are not provided in the tool input AND `config.weather.allow_ip_geolocation == true`, the provider **shall** perform one IP-geolocation lookup and cache the result for 1 hour.

**REQ-WEATHER-008 [Event-Driven]** — **When** `GetAirQuality(location)` is called in KMA provider mode, the provider **shall** query the `에어코리아` (Air Korea) API for PM10/PM2.5/O3/NO2/SO2/CO and return normalized `AirQuality{level: "good"|"moderate"|"unhealthy"|"very_unhealthy"|"hazardous"}` with Korean government standards applied.

**REQ-WEATHER-009 [Event-Driven]** — **When** per-provider rate limit is exceeded, the provider **shall** return `ErrRateLimited` immediately without calling the API; the calling tool **shall** retry against cache or return a structured error to the model.

### 4.3 State-Driven

**REQ-WEATHER-010 [State-Driven]** — **While** the last API call timestamp is older than 24 hours AND network is unavailable, offline fallback **shall** return `stale=true` with a warning in `WeatherReport.Message` field; consumers (BRIEFING-001) are responsible for tone adjustment.

**REQ-WEATHER-011 [State-Driven]** — **While** `config.weather.kma.api_key == ""` and provider is forced to `kma`, `WeatherProvider` initialization **shall** fail with `ErrMissingAPIKey`; auto mode in the same condition **shall** silently fall back to `OpenWeatherMapProvider`.

### 4.4 Unwanted Behavior

**REQ-WEATHER-012 [Unwanted]** — The provider **shall not** make more than one concurrent request to the same API endpoint for the same coordinates; in-flight requests **shall** be de-duplicated via `singleflight.Group`.

**REQ-WEATHER-013 [Unwanted]** — The provider **shall not** panic on malformed API responses; JSON parse failures **shall** return `ErrInvalidResponse` with the raw response preserved in error context for debugging.

### 4.5 Optional

**REQ-WEATHER-014 [Optional]** — **Where** `config.weather.include_pollen == true` and provider supports it (KMA for Korea only), the response **shall** include `pollen: {level, dominant_type}` during pollen season (3-5월, 9-10월).

**REQ-WEATHER-015 [Optional]** — **Where** `config.weather.forecast_days > 0` (max 7), `GetForecast` **shall** return `[]WeatherForecastDay` with high/low temp, precipitation probability, and condition per day.

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
- **Then** 외부 API 호출 0회, `ErrRateLimited` 반환.

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── weather/
        ├── tool.go              # TOOLS-001 Tool 인터페이스 구현
        ├── provider.go          # WeatherProvider 인터페이스
        ├── openweather.go       # OpenWeatherMapProvider
        ├── kma.go               # KMAProvider (초단기실황 + 단기예보)
        ├── airkorea.go          # 에어코리아 미세먼지 API
        ├── types.go             # WeatherReport, Location, AirQuality, SunTimes
        ├── cache.go             # LRU + TTL + disk persistence
        ├── geoip.go             # IP geolocation fallback
        ├── ratelimit.go         # per-provider rate limiter
        ├── config.go
        └── *_test.go
```

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

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| HTTP client | stdlib `net/http` + retry wrapper | 최소 의존 |
| LRU cache | `hashicorp/golang-lru/v2` v2.0+ | 업계 표준 |
| Singleflight | `golang.org/x/sync/singleflight` | 중복 요청 제거 |
| JSON | `encoding/json` + `gjson` (nested path) | |
| IP geolocation | 직접 HTTP (ipapi.co free) | SDK 불필요 |

### 6.7 TDD 진입 순서

1. RED: `TestWeatherTool_AutoRegister` — AC-WEATHER-001
2. RED: `TestCache_HitWithin10Min` — AC-WEATHER-002
3. RED: `TestOfflineFallback_DiskRead` — AC-WEATHER-003
4. RED: `TestAutoRoute_KRCountryUsesKMA` — AC-WEATHER-004
5. RED: `TestPM25_KoreanStandardMapping` — AC-WEATHER-005
6. RED: `TestAPIKey_NotInLogs` — AC-WEATHER-006
7. RED: `TestSingleflight_ConcurrentDedup` — AC-WEATHER-007
8. RED: `TestRateLimit_RejectsAt61st` — AC-WEATHER-008
9. GREEN → REFACTOR

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
| 선행 SPEC | TOOLS-001 | builtin tool 자동 등록 |
| 선행 SPEC | CONFIG-001 | weather.yaml |
| 선행 SPEC | CORE-001 | zap, context |
| 후속 SPEC | BRIEFING-001 | 아침 브리핑 구성요소 |
| 후속 SPEC | HEALTH-001 | 미세먼지 나쁨 → 마스크 리마인더 |
| 외부 | OpenWeatherMap API v2.5/3.0 | 글로벌 |
| 외부 | 기상청 OpenAPI (data.go.kr) | 한국 |
| 외부 | 에어코리아 API | 한국 미세먼지 |
| 외부 | `hashicorp/golang-lru/v2` | |
| 외부 | `x/sync/singleflight` | |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | KMA API 키 발급이 공공데이터 포털에서 수동 승인 필요 (즉시 발급 아님) | 고 | 중 | OpenWeatherMap fallback 기본, KMA는 opt-in |
| R2 | KMA 좌표 변환 오차 (격자 5km) | 낮 | 낮 | 서울 주요 25개 구별 좌표 goldenfile 테스트 |
| R3 | IP geolocation 부정확 (VPN/프록시) | 중 | 중 | `location` param 명시 우선, 실패 시 사용자에게 확인 |
| R4 | OpenWeatherMap 무료 quota 초과 (1000/day) | 중 | 중 | Cache 10min TTL, 단일 위치 평균 30회/day → 충분 |
| R5 | 에어코리아 API 다운타임 | 중 | 낮 | AirQuality는 optional 필드, nil 허용 |
| R6 | 미세먼지 기준 해외 표준과 한국 표준 혼용 | 낮 | 중 | provider별 기준 명시, 한국 우선 |

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

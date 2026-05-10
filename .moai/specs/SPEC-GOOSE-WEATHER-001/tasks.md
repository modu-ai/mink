---
id: SPEC-GOOSE-WEATHER-001
artifact: tasks
scope: M1 only (weather_current + WeatherProvider + OpenWeatherMap + IP geolocation + offline + config)
version: 0.1.0
created_at: 2026-05-10
author: manager-spec
---

# SPEC-GOOSE-WEATHER-001 — Task Decomposition (M1)

본 문서는 Phase 1.5 산출물. plan.md §2.2 의 23 atomic tasks 를 git-tracked artifact 로 보존하고
planned_files 컬럼을 통해 Phase 2 / 2.5 의 Drift Guard 가 사용한다.

각 task 는 단일 TDD cycle (RED-GREEN-REFACTOR) 내 완결. 의존 관계는 plan.md §2.2 와 정렬.

본 SPEC 은 TOOLS-WEB-001 의 common 인프라 (`common.Cache`, `common.Blocklist`, `common.Deps`, `common.Response`, `common.UserAgent()`, `RegisterWebTool`, `RateTracker`, `permission.Manager`, `audit.EventTypeToolWebInvoke`)를 그대로 재사용하므로, infrastructure task 는 없고 weather 도메인 task 만 정의한다.

---

## M1 Task Decomposition

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | weather_types.go: Location/WeatherReport/AirQuality/Pollen/SunTimes/WeatherForecastDay DTO 정의 (M2/M3 의 DTO 도 미리 선언; 본 task 는 단순 struct + JSON tag) | REQ-WEATHER-002, REQ-WEATHER-016 | — | internal/tools/web/weather_types.go | completed |
| T-002 | weather_provider.go: WeatherProvider 인터페이스 정의 (Name/GetCurrent/GetForecast/GetAirQuality/GetSunTimes) + ErrMissingAPIKey/ErrNoFallbackAvailable/ErrGeolocationFailed sentinel errors | REQ-WEATHER-001, REQ-WEATHER-011, REQ-WEATHER-013 | T-001 | internal/tools/web/weather_provider.go | completed |
| T-003 | weather_config.go: WeatherConfig struct + LoadWeatherConfig(path) 함수 (yaml.v3 기반, 누락 시 default 반환, TOOLS-WEB-001 LoadWebConfig 패턴) | REQ-WEATHER-006, REQ-WEATHER-011 | T-001 | internal/tools/web/weather_config.go | completed |
| T-004 | weather_offline.go: SaveLatest(provider, lat, lon, report) + LoadLatest(provider, lat, lon) (*WeatherReport, error). atomic write (temp + rename), 0600 권한, JSON parse fail 시 evict | REQ-WEATHER-005(d/e), REQ-WEATHER-010 | T-001 | internal/tools/web/weather_offline.go | completed |
| T-005 | weather_offline_test.go RED: TestOfflineSaveLoad_RoundTrip / TestOfflineLoad_FileMissing / TestOfflineLoad_CorruptJSON_Evict / TestOfflineLoad_AtomicWrite_NoPartial | AC-WEATHER-003 (보조) | T-004 (signatures) | internal/tools/web/weather_offline_test.go | completed |
| T-006 | weather_geoip.go: IPGeolocator interface + IPAPIGeolocator (https://ipapi.co/json/) + 1h TTL 캐시 (TOOLS-WEB-001 common.Cache 재사용) + ErrGeolocationFailed | REQ-WEATHER-007 | T-002 | internal/tools/web/weather_geoip.go | completed |
| T-007 | weather_geoip_test.go RED: TestGeoIP_Resolve_Seoul / TestGeoIP_CacheHitWithin1Hour / TestGeoIP_FetchFailed_ReturnsGeolocationError / TestGeoIP_TTLExpiry_RefreshesCache | REQ-WEATHER-007 | T-006 (signatures) | internal/tools/web/weather_geoip_test.go | completed |
| T-008 | weather_openweather.go: OpenWeatherMapProvider.GetCurrent 구현 (api.openweathermap.org/data/2.5/weather, lang=ko/en, units=metric/imperial, response → WeatherReport, API key redaction 포함) + Name() == "openweathermap" | REQ-WEATHER-004, REQ-WEATHER-006 | T-001, T-002 | internal/tools/web/weather_openweather.go | completed |
| T-009 | weather_openweather_test.go RED: TestOWM_GetCurrent_Seoul_KO_Metric (mock httptest.Server) / TestOWM_GetCurrent_APIError_5xx_Retryable / TestOWM_GetCurrent_APIKey_Redacted_NotInLogs (zaptest/observer) / TestOWM_GetCurrent_InvalidJSON_Response | REQ-WEATHER-004, REQ-WEATHER-013, AC-WEATHER-006 | T-008 (signatures) | internal/tools/web/weather_openweather_test.go | completed |
| T-010 | weather_current.go: webWeatherCurrent struct (deps *common.Deps + provider WeatherProvider + geolocator IPGeolocator + offline OfflineStore + cfg *WeatherConfig + sf *singleflight.Group) + Name() == "weather_current" + Schema() additionalProperties:false + Scope() == ScopeShared | REQ-WEATHER-001, REQ-WEATHER-016, REQ-WEATHER-017 | T-002, T-006, T-008 | internal/tools/web/weather_current.go | completed |
| T-011 | weather_current.go: parseWeatherCurrentInput(raw) — schema enforcement (anyOf: location | (lat,lon) | empty-with-ipgeo, lat∈[-90,90], lon∈[-180,180], units enum, lang enum) + defensive guard for direct Call() | REQ-WEATHER-001, AC-WEATHER-010 | T-010 | internal/tools/web/weather_current.go (modify) | completed |
| T-012 | weather_current.go: Call() 11-step 시퀀스 (parse → resolve location → host derive → blocklist → permission → cache key → live cache → ratelimit → singleflight outbound → on success cache.Set + offline.SaveLatest + audit ok / on failure offline.LoadLatest → stale or fetch_failed) | REQ-WEATHER-005, REQ-WEATHER-007, REQ-WEATHER-009, REQ-WEATHER-010, REQ-WEATHER-012 | T-002~T-011 | internal/tools/web/weather_current.go (modify) | completed |
| T-013 | weather_current_test.go RED: 8 시나리오 (Registered_InWebTools / StandardResponseShape / CacheHitWithin10Min / OfflineFallback_DiskRead / APIKey_Redacted_NotInLogs / Singleflight_ConcurrentDedup / RateLimit_Exhausted / Blocklist_HostBlocked) | AC-WEATHER-001, 002, 003, 006, 007, 008, 010, REQ-WEATHER-005 | T-010, T-011, T-012 (signatures) | internal/tools/web/weather_current_test.go | completed |
| T-014 | weather_current.go: init() — RegisterWebTool(&webWeatherCurrent{deps: &common.Deps{}, provider: nil, ...}) (bootstrap 시 provider 주입 패턴, TOOLS-WEB-001 webWikipedia.init() 패턴 정렬) | REQ-WEATHER-001, REQ-WEATHER-017, AC-WEATHER-001 | T-010 | internal/tools/web/weather_current.go (modify) | completed |
| T-015 | schema_test.go (modify): TestAllToolSchemasValid 의 expected tool list 에 "weather_current" 추가, weather_current schema 가 meta-schema valid + additionalProperties:false 검증 | AC-WEATHER-009, AC-WEATHER-010 | T-014 | internal/tools/web/schema_test.go (modify) | completed |
| T-016 | register_test.go (modify): TestRegistry_WithWeb_ListNames 의 expected count 14 → 15, expected names 정렬 부분집합에 "weather_current" 추가 | AC-WEATHER-001, AC-WEATHER-009 | T-014 | internal/tools/web/register_test.go (modify) | completed |
| T-017 | permission_integration_test.go (modify): TestFirstCallConfirm_WeatherCurrent 추가 (host=api.openweathermap.org, AlwaysAllow 시 두 번째 호출 grant cache hit + cache_hit metadata) | AC-WEATHER-008 (보조), REQ-WEATHER-004 | T-012 | internal/tools/web/permission_integration_test.go (modify) | completed |
| T-018 | audit_integration_test.go (modify): TestAuditLog_WeatherCurrentCall 추가 (단일 호출 → 1 line, tool="weather_current", outcome="ok", host, duration_ms>0) | REQ-WEATHER-003, AC-WEATHER-008 (보조) | T-012 | internal/tools/web/audit_integration_test.go (modify) | completed |
| T-019 | ratelimit_integration_test.go (modify): TestRateLimitExhausted_Weather 추가 (Tracker.Set provider="openweathermap" Remaining=0, 호출 → ratelimit_exhausted + retry_after_seconds>0 + outbound 0회 + audit reason="ratelimit_exhausted") | AC-WEATHER-008 | T-012 | internal/tools/web/ratelimit_integration_test.go (modify) | completed |
| T-020 | doc.go (optional modify): weather 도구군 M1 진척 한 줄 추가 | docs | T-014 | internal/tools/web/doc.go (modify) | completed |
| T-021 | .moai/docs/weather-quickstart.md (신규): OpenWeatherMap key 발급 절차 + ~/.goose/config/weather.yaml 작성 가이드 + 한국 사용자 KMA 안내 (M2 향후 도입) | docs | none | .moai/docs/weather-quickstart.md | completed |
| T-022 | weather_geoip.go (modify): IPAPIGeolocator 가 ipapi.co 호스트를 별도 permission scope 로 처리 (CapNet, scope="ipapi.co") + audit reason="geolocation" 명시 + Blocklist 통과 후 호출 | REQ-WEATHER-004 (확장), REQ-WEATHER-007 | T-006, T-012 | internal/tools/web/weather_geoip.go (modify) | completed |
| T-023 | go.mod / go.sum: golang.org/x/sync/singleflight 의존성 검증 (`go list -m all | grep sync`). 기존 transitive면 신규 0, 미존재면 `go get golang.org/x/sync` 실행 후 plan.md §6 갱신 | enabling, REQ-WEATHER-012 | none | go.mod, go.sum (modify if needed), .moai/specs/SPEC-GOOSE-WEATHER-001/plan.md (modify §6) | completed |

**합계**: 23 atomic tasks, ~13 production files (신규) + ~5 test files (신규) + ~5 modifications (test/doc/config).

---

## Drift Guard Reference

이 표의 `Planned Files` 컬럼은 Phase 2.5 Drift Guard 가 사용한다.
- drift = (unplanned_new_files / total_planned_files) * 100
- ≤ 20%: informational
- 20% < drift ≤ 30%: warning
- > 30%: Phase 2.7 re-planning gate

Total planned files (M1):
- production 신규 (8): weather_types.go, weather_provider.go, weather_config.go, weather_offline.go, weather_geoip.go, weather_openweather.go, weather_current.go (T-010~T-014 통합), .moai/docs/weather-quickstart.md
- test 신규 (5): weather_offline_test.go, weather_geoip_test.go, weather_openweather_test.go, weather_current_test.go (대용량), 그리고 register/schema/permission/audit/ratelimit_integration_test.go modifications (5 files modify)

---

## TDD 사이클 운영 규칙

1. Pair (예: T-008 weather_openweather.go ↔ T-009 weather_openweather_test.go) 에서 항상 test 부터 작성 (RED).
2. `go test ./internal/tools/web/...` compile fail / test fail 확인 후 production 코드 작성 (GREEN).
3. T-010 ~ T-014 큰 task (weather_current.go) 는 sub-AC 단위로 RED-GREEN 분할:
   - sub 1: AC-WEATHER-001 (Registered) → T-014 + T-016
   - sub 2: AC-WEATHER-010 (StandardResponseShape) → T-010, T-011 partial, T-013 partial
   - sub 3: AC-WEATHER-002 (CacheHit) → T-012 partial (cache step)
   - sub 4: AC-WEATHER-003 (Offline) → T-012 partial (offline step) + T-004
   - sub 5: AC-WEATHER-006 (APIKey redaction) → T-009 + T-008
   - sub 6: AC-WEATHER-007 (Singleflight) → T-012 partial (sf step)
   - sub 7: AC-WEATHER-008 (RateLimit) → T-012 partial (rl step) + T-019
4. 각 GREEN 직후 `go vet ./internal/tools/web/...` + `golangci-lint run ./internal/tools/web/...` 0 warning 유지.
5. T-017~T-019 integration 테스트는 unit 테스트 통과 후 마지막에 묶음 RED → GREEN.
6. T-023 (singleflight 의존성) 은 T-012 시작 전에 처리 (의존성 미해결 상태로 작업 진입 금지).

---

## M1 범위 외 (deferred)

- AC-WEATHER-004 (auto KMA routing): M2 weather_forecast + weather_kma.go + weather_route.go
- AC-WEATHER-005 (Korean PM2.5 mapping): M3 weather_air_quality + weather_airkorea.go
- REQ-WEATHER-008 (에어코리아): M3
- REQ-WEATHER-014 (Pollen optional): M3 이후 별도 SPEC
- REQ-WEATHER-015 (forecast_days > 0): M2

---

## M2 Task Decomposition (완료 — 2026-05-10)

| Task ID | Description | Requirement | Planned Files | Status |
|---------|-------------|-------------|---------------|--------|
| T-024 | weather_kma.go: KMAProvider struct + Name() + LatLonToGrid() + GetCurrent (UltraSrtNcst) + GetForecast (VilageFcst, days clamp 3) + GetAirQuality/GetSunTimes stubs | REQ-WEATHER-001, REQ-WEATHER-004, REQ-WEATHER-006 | internal/tools/web/weather_kma.go | completed |
| T-025 | weather_route.go: routeProvider() + selectProvider() — auto/forced routing with KMA key presence check | REQ-WEATHER-006, REQ-WEATHER-011 | internal/tools/web/weather_route.go | completed |
| T-026 | weather_forecast.go: webWeatherForecast 11-step Call + parseWeatherForecastInput + weatherForecastCacheKey + WeatherConfigForTest + init() | REQ-WEATHER-005, REQ-WEATHER-006, REQ-WEATHER-015 | internal/tools/web/weather_forecast.go | completed |
| T-027 | weather_kma_test.go: TestLatLonToGrid_5Cities (goldenfile) + TestKMA_GetCurrent_Seoul_NowCast + TestKMA_GetForecast_Seoul_3Days + TestKMA_MissingAPIKey + TestKMA_APIError_5xx + TestKMA_APIKey_Redacted + TestKMA_DaysClamp_When7 | AC-WEATHER-004, REQ-WEATHER-004 | internal/tools/web/weather_kma_test.go | completed |
| T-028 | weather_route_test.go: TestAutoRoute_KRCountryUsesKMA (AC-WEATHER-004) + TestAutoRoute_NonKRUsesOWM + TestAutoRoute_KMAKeyMissingFallback + TestForceKMA_NoKey + TestWeatherForecast_Registered + TestWeatherForecast_StandardResponseShape + TestWeatherForecast_DaysOutOfRange + TestWeatherForecast_BlocklistPriority + TestWeatherForecast_RateLimit_Exhausted | AC-WEATHER-004, AC-WEATHER-009 (M2) | internal/tools/web/weather_route_test.go | completed |
| T-029 | register_test.go 수정: expectation 15 → 16 (weather_forecast 추가) | AC-WEATHER-009 (M2) | internal/tools/web/register_test.go | completed |
| T-030 | schema_test.go 수정: expectedNames +1 (weather_forecast) | AC-WEATHER-009 (M2) | internal/tools/web/schema_test.go | completed |
| T-031 | audit_integration_test.go 수정: +TestAuditLog_WeatherForecastCall | AC-WEATHER-004 audit | internal/tools/web/audit_integration_test.go | completed |
| T-032 | progress.md M2 Run Phase 섹션 append (AC-WEATHER-004 GREEN, DFS_XY_CONV goldenfile 정정, deviation Option A) | — | .moai/specs/SPEC-GOOSE-WEATHER-001/progress.md | completed |
| T-033 | tasks.md M2 task breakdown append (T-024 ~ T-033) | — | .moai/specs/SPEC-GOOSE-WEATHER-001/tasks.md | completed |

### DFS_XY_CONV goldenfile 정정 note (T-027)

Python/C 공식으로 재검증한 결과:
- 제주 nx: research.md 52 → 실제 53 (off-by-one)
- 강릉 ny: research.md 131 → 실제 132 (off-by-one)
서울/부산/대전은 정확. 테스트는 실제 계산값 기준으로 수정.

---

## M3 분할 정책 (예비)

- M3a: weather_airkorea.go + AirKorea API wrapper
- M3b: weather_air_quality.go 도구 + 한국 PM2.5 매핑 + AC-WEATHER-005 boundary

---

Version: 0.1.0
Last Updated: 2026-05-10

---
id: SPEC-GOOSE-WEATHER-001
artifact: spec-compact
version: 0.1.0
created_at: 2026-05-10
---

# SPEC-GOOSE-WEATHER-001 (Compact)

> 한 페이지 요약. LLM 시스템 프롬프트 / 작업 컨텍스트 inject 용.

## 목적

GOOSE 의 날씨 도구 묶음 — `weather_current` (M1), `weather_forecast` (M2), `weather_air_quality` (M3) — 을 정의. TOOLS-WEB-001 의 common 인프라를 재사용하여 `internal/tools/web/weather*.go` 파일군에 등록. PERMISSION-001 / AUDIT-001 / RATELIMIT-001 / Blocklist 통합. 글로벌 OpenWeatherMap + 한국 KMA + 에어코리아 provider.

## 핵심 계약

- 3 도구 모두 TOOLS-WEB-001 `Tool` 인터페이스 (`Name`, `Schema`, `Scope`, `Call`) 구현.
- 모든 응답: TOOLS-WEB-001 `common.Response{ok, data|error, metadata}` 표준 shape.
- `data.stale` 와 `metadata.cache_hit` 의미 구분: live bbolt cache hit ↔ disk offline fallback.
- 모든 외부 호출: `User-Agent: goose-agent/{version}`.
- 첫 호출 시 PERMISSION-001 동의 (host 별), 결과 영속 grant.
- API key 로그 redaction (REQ-WEATHER-004).
- Singleflight 동시 dedup, bbolt cache 10min TTL, disk offline fallback 24h.
- per-provider rate limit (openweathermap 60/min, kma 20/min, airkorea 30/min, ipapi 10/min).
- 모든 호출 AUDIT-001 기록 (`tool.web.invoke`, tool=`weather_*`).
- IP geolocation: `ipapi.co` HTTPS free (1000/day), 1h TTL 캐시.
- KMA 좌표 변환: Lambert Conformal Conic (DFS_XY_CONV), 5개 도시 goldenfile (M2).
- 한국 미세먼지 매핑: 환경부 기준 (15/35/75 boundary, M3).

## 3 도구 시그니처 요약

| 도구 | input 핵심 | output 핵심 (data) | M |
|---|---|---|---|
| `weather_current` | `{location?, lat?, lon?, units?: metric/imperial, lang?: ko/en}` | `WeatherReport{location, timestamp, temperature_c, condition, humidity, wind_kph, source_provider, stale, message}` | M1 |
| `weather_forecast` | `{location?, lat?, lon?, days: 1..7, units?, lang?}` | `{days: []WeatherForecastDay}` (high/low/condition/precip per day) | M2 |
| `weather_air_quality` | `{location?, lat?, lon?}` | `AirQuality{pm10, pm25, level, level_local}` (한국 only) | M3 |

input 은 모두 `anyOf: [location | (lat,lon) | empty-with-ipgeo]`.

## EARS 17 REQ (요약)

- Ubiquitous: REQ-WEATHER-001~004 (등록, 응답 필드, log, API key redaction), REQ-WEATHER-016~017 (표준 응답 shape, RegisterWebTool 통합, 본 SPEC v0.1.1 신규).
- Event-Driven: REQ-WEATHER-005~009 (cache flow, auto provider routing, IP geo fallback, AirKorea KMA, ratelimit).
- State-Driven: REQ-WEATHER-010~011 (offline 24h, KMA key 부재 silent fallback).
- Unwanted: REQ-WEATHER-012~013 (singleflight dedup, malformed response no-panic).
- Optional: REQ-WEATHER-014~015 (Pollen, forecast_days).

## AC 10 (요약)

- AC-WEATHER-001 weather_current 등록 (M1)
- AC-WEATHER-002 cache hit within 10min (M1)
- AC-WEATHER-003 offline disk fallback (M1)
- AC-WEATHER-004 KR Country auto KMA routing (M2)
- AC-WEATHER-005 PM2.5 한국 환경부 매핑 (M3)
- AC-WEATHER-006 API key 로그 미노출 (M1)
- AC-WEATHER-007 singleflight 100 goroutine dedup (M1)
- AC-WEATHER-008 ratelimit_exhausted 응답 (M1, TOOLS-WEB-001 매핑)
- AC-WEATHER-009 registry inventory: M1 +1, M2 +1, M3 +1 (총 17 names) (본 SPEC v0.1.1 신규)
- AC-WEATHER-010 표준 응답 shape (data.stale ↔ metadata.cache_hit 구분) (본 SPEC v0.1.1 신규)

## Milestones (priority)

- M1 (P1): weather_current + WeatherProvider + OpenWeatherMap + IP geolocation + offline disk + config (8 AC).
- M2 (P2): weather_forecast + KMAProvider + DFS_XY_CONV + auto routing (1 AC + 누적).
- M3 (P3): weather_air_quality + AirKorea + 한국 환경부 PM2.5 매핑 (1 AC + 누적).

## OUT (명시적 제외)

- Push notification (BRIEFING-001), 위성/레이더, historical weather, ML 예측, marine/aviation, 지진/태풍 특보, TOOLS-WEB-001 8 도구 변경, customized model, weather.yaml hot reload, KMA OAuth 자동 발급.

## 의존

- TOOLS-001 v0.1.2 (completed) / TOOLS-WEB-001 (M1 implemented + M2~M4 진행) / PERMISSION-001 v0.2.0 (completed) / AUDIT-001 v0.1.0 (completed) / RATELIMIT-001 v0.2.0 (completed) / FS-ACCESS-001 (planned) / CONFIG-001 (planned).
- 외부: OpenWeatherMap v2.5/3.0, KMA data.go.kr (M2), 에어코리아 (M3), ipapi.co (M1).
- 신규 외부 의존성 (M1): 0 또는 1 (singleflight 기존 여부에 따라).

## 패키지 위치

`internal/tools/web/weather*.go` (TOOLS-WEB-001 패키지 재사용; 별도 sub-package 미생성).

---

Version: 0.1.0
Last Updated: 2026-05-10

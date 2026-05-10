# Weather Tools Quickstart

This document covers how to obtain API keys and configure the weather tools
(`weather_current`, and future `weather_forecast` / `weather_air_quality`).

---

## M1: weather_current (OpenWeatherMap)

### 1. Obtain an OpenWeatherMap API Key

1. Create a free account at <https://openweathermap.org/appid>.
2. Go to **My API Keys** and generate a new key.
3. Free tier: 1,000 calls/day and 60 calls/minute — sufficient for personal use
   with the built-in 10-minute bbolt cache.

### 2. Create the Configuration File

Create `~/.goose/config/weather.yaml` with at minimum:

```yaml
weather:
  provider: openweathermap          # M1 default; "auto" in M2+
  openweathermap:
    api_key: YOUR_OWM_API_KEY_HERE
  default_location: "Seoul,KR"      # fallback when no location is specified
  cache_ttl: "10m"                  # live bbolt cache TTL
  allow_ip_geolocation: true        # auto-detect location via ipapi.co
```

Replace `YOUR_OWM_API_KEY_HERE` with the key from step 1.

### 3. Call the Tool

From any GOOSE agent session, invoke:

```json
{
  "name": "weather_current",
  "input": {
    "lat": 37.5665,
    "lon": 126.9780,
    "units": "metric",
    "lang": "ko"
  }
}
```

Or by location string (requires OWM geocoding — uses the lat/lon API):

```json
{
  "name": "weather_current",
  "input": {
    "location": "Busan,KR",
    "units": "metric",
    "lang": "ko"
  }
}
```

Or with no location (IP geolocation, when `allow_ip_geolocation: true`):

```json
{
  "name": "weather_current",
  "input": {}
}
```

### 4. Response Shape

Successful responses follow the `common.Response` envelope:

```json
{
  "ok": true,
  "data": {
    "location": { "lat": 37.57, "lon": 126.98, "country": "KR" },
    "timestamp": "2026-05-10T09:00:00Z",
    "temperature_c": 22.5,
    "feels_like_c": 21.0,
    "condition": "clear",
    "condition_local": "맑음",
    "humidity": 55,
    "wind_kph": 12.6,
    "wind_direction": "SW",
    "cloud_cover_pct": 10,
    "precip_mm": 0,
    "source_provider": "openweathermap",
    "cache_hit": false,
    "stale": false
  },
  "metadata": {
    "cache_hit": false,
    "duration_ms": 312
  }
}
```

- `data.cache_hit`: `true` when the result is served from the 10-minute live cache.
- `data.stale`: `true` when the network is unavailable and the last saved disk
  file is used as a fallback (offline mode).
- `metadata.cache_hit` mirrors `data.cache_hit` for consistency with other web tools.

---

## M2 (Planned): weather_forecast + KMA for Korean Users

The Korean Meteorological Administration (KMA) API provides higher accuracy
for Korean coordinates. To use it in M2, you will need an additional key:

1. Register at <https://data.go.kr> (공공데이터포털).
2. Apply for **기상청_단기예보 ((구)_동네예보) 조회서비스** — approval may
   take 1-3 business days.
3. Add to `weather.yaml`:

```yaml
weather:
  provider: auto          # routes KR coordinates to KMA, others to OWM
  kma:
    api_key: YOUR_KMA_API_KEY_HERE
```

---

## M3 (Planned): weather_air_quality (AirKorea)

For fine-dust (PM2.5 / PM10) data using Korean Ministry of Environment standards:

1. Register at <https://data.go.kr>.
2. Apply for **한국환경공단_에어코리아_대기오염정보**.
3. Add to `weather.yaml`:

```yaml
weather:
  airkorea:
    api_key: YOUR_AIRKOREA_API_KEY_HERE
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `error.code == "permission_denied"` | First-call host grant not yet given | Re-run; GOOSE will prompt for permission |
| `error.code == "ratelimit_exhausted"` | 60 calls/minute exceeded | Wait for `retry_after_seconds` |
| `data.stale == true` | Network unreachable; disk fallback used | Restore connectivity; stale data may be up to 24h old |
| `error.code == "fetch_failed"` and no disk fallback | First call ever + network down | Configure `default_location` and call once while online |
| Log shows `****` for api_key | Expected — API key is redacted from all logs (REQ-WEATHER-004) | No action needed |

---

Version: 0.1.0 (M1)
SPEC: SPEC-GOOSE-WEATHER-001
Last Updated: 2026-05-10

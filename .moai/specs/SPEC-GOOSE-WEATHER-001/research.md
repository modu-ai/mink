# Research — SPEC-GOOSE-WEATHER-001

## 1. Provider 비교

| Provider | Coverage | 무료 Quota | 한국 정확도 | 미세먼지 | 결정 |
|---------|---------|----------|---------|--------|-----|
| OpenWeatherMap | Global 11만+ | 1000/day | 중 (외국 소스 기반) | 기본 제공 | 글로벌 기본 |
| 기상청 (KMA) | 한국 | 10,000/day | 상 (공식) | 별도(에어코리아) | 한국 기본 |
| WeatherAPI.com | Global | 1M/month | 중 | ○ | 후보2 (보류) |
| AccuWeather | Global | 50/day | 중 | △ | 기각 (quota 부족) |
| Pirate Weather | Global | 10,000/day | 낮 | X | 기각 |

**결론**: `provider: "auto"` 는 `country==KR` → KMA, 그 외 → OpenWeatherMap.

## 2. KMA API 구성

기상청 OpenAPI는 여러 엔드포인트로 분산:

- **초단기실황**: 현재 기온/습도/풍속 (1시간 간격)
- **초단기예보**: 현재 ~ 6시간 예보 (30분 간격)
- **단기예보**: 3일 예보 (3시간 간격)
- **중기예보**: 3~10일 예보 (육상/기온 별도)

Phase 7 본 SPEC은 **초단기실황 + 단기예보**만 사용 (일관된 아침 브리핑 구성).

## 3. DFS_XY_CONV 좌표 변환

기상청 Lambert Conformal Conic Projection 알고리즘:

```go
const (
    REGRID  = 6371.00877     // 지구 반경
    GRID    = 5.0            // 격자 간격 (km)
    SLAT1   = 30.0           // 투영 위도1
    SLAT2   = 60.0           // 투영 위도2
    OLON    = 126.0          // 기준 경도
    OLAT    = 38.0           // 기준 위도
    XO      = 43             // 기준 X
    YO      = 136            // 기준 Y
)

func LatLonToGrid(lat, lon float64) (int, int) {
    DEGRAD := math.Pi / 180.0
    re := REGRID / GRID

    slat1 := SLAT1 * DEGRAD
    slat2 := SLAT2 * DEGRAD
    olon := OLON * DEGRAD
    olat := OLAT * DEGRAD

    sn := math.Tan(math.Pi*0.25+slat2*0.5) / math.Tan(math.Pi*0.25+slat1*0.5)
    sn = math.Log(math.Cos(slat1)/math.Cos(slat2)) / math.Log(sn)
    sf := math.Tan(math.Pi*0.25 + slat1*0.5)
    sf = math.Pow(sf, sn) * math.Cos(slat1) / sn
    ro := math.Tan(math.Pi*0.25 + olat*0.5)
    ro = re * sf / math.Pow(ro, sn)

    ra := math.Tan(math.Pi*0.25 + lat*DEGRAD*0.5)
    ra = re * sf / math.Pow(ra, sn)
    theta := lon*DEGRAD - olon
    if theta > math.Pi { theta -= 2.0 * math.Pi }
    if theta < -math.Pi { theta += 2.0 * math.Pi }
    theta *= sn

    nx := int(ra*math.Sin(theta) + float64(XO) + 0.5)
    ny := int(ro - ra*math.Cos(theta) + float64(YO) + 0.5)
    return nx, ny
}
```

**Goldenfile 검증** (research/weather-korea-grid.json):
- 서울: (37.5665, 126.9780) → (60, 127)
- 부산: (35.1796, 129.0756) → (98, 76)
- 제주: (33.4996, 126.5312) → (52, 38)
- 대전: (36.3504, 127.3845) → (67, 100)
- 강릉: (37.7519, 128.8761) → (92, 131)

## 4. 한국 미세먼지 기준 vs WHO

| 구분 | PM2.5 좋음 | 보통 | 나쁨 | 매우나쁨 |
|------|---------|------|------|---------|
| 한국 환경부 | 0-15 | 16-35 | 36-75 | 76+ |
| WHO | 0-10 | 11-25 | 26-50 | 51+ |
| US EPA | 0-12 | 13-35.4 | 35.5-55.4 | 55.5+ |

결정: **한국 사용자는 한국 환경부 기준** 사용. config로 override 가능.

## 5. Cache 전략

### 5.1 In-memory LRU

- 크기: 256 엔트리 (위치 다양성 고려)
- TTL: 10분
- 라이브러리: `hashicorp/golang-lru/v2` (TTL 지원하는 `Expirable`)

### 5.2 Disk persistence

- 경로: `~/.goose/cache/weather/{provider}-{lat2dp}-{lon2dp}.json`
- 최신 1건만 유지 (덮어쓰기)
- 파일 권한 0600

### 5.3 Key 정규화

좌표 소수점 2자리 반올림 (≈ 1.1km 정밀도). 불필요한 cache miss 방지.

## 6. Offline Fallback

```go
func (p *Provider) GetCurrent(ctx, loc) (*WeatherReport, error) {
    // 1. Memory cache
    if hit, ok := p.cache.Get(key); ok && !expired(hit) {
        return hit.withCacheHit(), nil
    }

    // 2. Singleflight: 동일 key 동시 요청 병합
    val, err, _ := p.sf.Do(key, func() (any, error) {
        return p.fetchFromAPI(ctx, loc)
    })

    // 3. API 성공 → cache + disk save + return
    if err == nil {
        p.cache.Set(key, val)
        _ = p.persistDisk(key, val)
        return val.(*WeatherReport), nil
    }

    // 4. API 실패 → disk fallback
    if disk, derr := p.loadDisk(key); derr == nil {
        disk.Stale = true
        disk.Message = "오프라인 상태입니다. 마지막 확인 시각: " + disk.Timestamp.Format(...)
        return disk, nil
    }

    // 5. 전부 실패
    return nil, fmt.Errorf("%w: %v", ErrNoFallbackAvailable, err)
}
```

## 7. IP Geolocation

- Primary: `ipapi.co` (free, 1000/day, HTTPS)
- Fallback: `ip-api.com` (free, 45/min, HTTP only)
- Cache TTL: 1시간 (사용자 이동 빈도 고려)

프라이버시 경고: IP geolocation은 사용자 IP를 외부 서비스에 노출. `config.weather.allow_ip_geolocation=false` 로 끌 수 있음.

## 8. Rate Limit

Token bucket per provider:

| Provider | 기본 limit | Burst |
|---------|----------|-------|
| OpenWeatherMap | 60/min | 10 |
| KMA | 20/min | 5 |
| AirKorea | 30/min | 5 |
| IPAPI | 10/min | 3 |

`golang.org/x/time/rate` 채택.

## 9. 테스트 전략

### 9.1 Mock HTTP Server

```go
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // URL pattern 매칭 → fixture 응답
    switch {
    case strings.Contains(r.URL.Path, "/data/2.5/weather"):
        w.Write(owmCurrentFixture)
    case strings.Contains(r.URL.Path, "/VilageFcstInfoService"):
        w.Write(kmaForecastFixture)
    }
}))
```

### 9.2 Fixture 데이터

`testdata/owm-seoul-current.json`, `testdata/kma-seoul-now.json`, `testdata/airkorea-seoul-pm25-55.json` goldenfile 유지.

## 10. 오픈 이슈

1. KMA API 응답 파싱 오류 시 캐시된 OpenWeather 응답을 secondary fallback으로 쓸지 여부 (현재: no fallback 교차, 단일 provider 고정).
2. 미세먼지 "매우 나쁨" 감지 시 자동 알람 → HEALTH-001 의존 (loose coupling).
3. 다국어 condition 텍스트: OpenWeatherMap ko 응답 품질 중간, 수동 매핑 table 유지할지 여부.
4. Geo location privacy: IP 기반 위치 정보를 MEMORY-001에 기록할지 (일단 기록 안 함, 위치는 tool input에만).

## 11. 참고

- OpenWeatherMap API ref: https://openweathermap.org/current
- KMA 단기예보 조회서비스: https://www.data.go.kr/tcs/dss/selectApiDataDetailView.do?publicDataPk=15084084
- 에어코리아 측정망: https://www.airkorea.or.kr/eng/
- Lambert Conformal Conic: https://mathworld.wolfram.com/LambertConformalConicProjection.html

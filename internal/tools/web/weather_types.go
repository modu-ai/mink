package web

import "time"

// Location represents a geographic location used by weather providers.
// Latitude and Longitude are required; DisplayName, Country, and Timezone
// are populated by geolocation resolution or user-supplied location strings.
type Location struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	DisplayName string  `json:"display_name"`
	Country     string  `json:"country"`
	Timezone    string  `json:"timezone"`
}

// WeatherReport is the unified data transfer object for current weather.
// Both OpenWeatherMap and KMA providers (M2) normalize their responses to
// this struct before returning. Fields marked "M2" or "M3" are pre-declared
// for forward compatibility but will be zero-valued until those milestones land.
type WeatherReport struct {
	Location       Location    `json:"location"`
	Timestamp      time.Time   `json:"timestamp"`
	TemperatureC   float64     `json:"temperature_c"`
	FeelsLikeC     float64     `json:"feels_like_c"`
	Condition      string      `json:"condition"`       // "clear" | "cloudy" | "rain" | "snow" | "thunderstorm" | "mist"
	ConditionLocal string      `json:"condition_local"` // localized description (e.g. "맑음" for lang=ko)
	Humidity       int         `json:"humidity"`        // 0-100 (%)
	WindKph        float64     `json:"wind_kph"`
	WindDirection  string      `json:"wind_direction"`  // "N" | "NE" | "E" | ... | "NW"
	CloudCoverPct  int         `json:"cloud_cover_pct"` // 0-100 (%)
	PrecipMm       float64     `json:"precip_mm"`
	UVIndex        float64     `json:"uv_index"`              // 0 when provider does not supply it (M1 OWM v2.5)
	AirQuality     *AirQuality `json:"air_quality,omitempty"` // optional; populated by M3
	Pollen         *Pollen     `json:"pollen,omitempty"`      // optional; Korea only (M3+)
	SunTimes       *SunTimes   `json:"sun_times,omitempty"`   // optional; populated when provider returns it
	SourceProvider string      `json:"source_provider"`       // "openweathermap" | "kma"
	CacheHit       bool        `json:"cache_hit"`             // true: served from live bbolt cache
	Stale          bool        `json:"stale"`                 // true: offline disk fallback was used
	Message        string      `json:"message,omitempty"`     // user-facing note (e.g. offline warning)
}

// AirQuality holds normalized air-quality data using Korean Ministry of
// Environment boundaries. M1/M2 leave this zero-valued; M3 populates it.
type AirQuality struct {
	Level   string  `json:"level"`    // "good" | "moderate" | "unhealthy" | "very_unhealthy" | "hazardous"
	LevelKo string  `json:"level_ko"` // "좋음" | "보통" | "나쁨" | "매우 나쁨" | "위험"
	PM10    int     `json:"pm10"`
	PM25    int     `json:"pm25"`
	O3      float64 `json:"o3"`
	NO2     float64 `json:"no2"`
}

// Pollen carries pollen-count information, applicable only to Korean
// coordinates via the KMA provider. Optional; declared here for M3.
type Pollen struct {
	Level        string `json:"level"`         // "low" | "moderate" | "high" | "very_high"
	DominantType string `json:"dominant_type"` // e.g. "oak", "grass", "mugwort"
}

// SunTimes holds sunrise and sunset UTC timestamps for a given date and
// location. Populated when the provider returns this data.
type SunTimes struct {
	Sunrise time.Time `json:"sunrise"`
	Sunset  time.Time `json:"sunset"`
}

// WeatherForecastDay represents one day's forecast. Used by weather_forecast
// (M2); pre-declared so types are available without a milestone gate.
type WeatherForecastDay struct {
	Date           string  `json:"date"` // "2026-05-12"
	HighC          float64 `json:"high_c"`
	LowC           float64 `json:"low_c"`
	Condition      string  `json:"condition"`
	ConditionLocal string  `json:"condition_local"`
	PrecipProbPct  int     `json:"precip_prob_pct"` // 0-100
	PrecipMm       float64 `json:"precip_mm"`
	WindKph        float64 `json:"wind_kph"`
	Humidity       int     `json:"humidity"`
}

package briefing

import (
	"time"
)

// BriefingPayload represents the complete morning briefing output.
type BriefingPayload struct {
	Weather      WeatherModule      `json:"weather"`
	JournalRecall RecallModule       `json:"journal_recall"`
	DateCalendar DateModule          `json:"date_calendar"`
	Mantra       MantraModule        `json:"mantra"`
	Status       map[string]string   `json:"status"`      // module -> status (ok, offline, timeout, skipped, error)
	GeneratedAt  time.Time           `json:"generated_at"`
}

// WeatherModule contains weather information for the briefing.
type WeatherModule struct {
	Current       *WeatherCurrent   `json:"current,omitempty"`
	Forecast      *WeatherForecast  `json:"forecast,omitempty"`
	AirQuality    *AirQuality       `json:"air_quality,omitempty"`
	Offline       bool              `json:"offline"`
}

// WeatherCurrent represents current weather conditions.
type WeatherCurrent struct {
	Temp        float64   `json:"temp"`
	FeelsLike   float64   `json:"feels_like"`
	Humidity    float64   `json:"humidity"`
	Condition   string    `json:"condition"`
	Location    string    `json:"location"`
}

// WeatherForecast represents weather forecast.
type WeatherForecast struct {
	Days []WeatherForecastDay `json:"days"`
}

// WeatherForecastDay represents a single day's forecast.
type WeatherForecastDay struct {
	Date      string  `json:"date"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Condition string  `json:"condition"`
}

// AirQuality represents air quality information.
type AirQuality struct {
	PM25  float64 `json:"pm25"`
	PM10  float64 `json:"pm10"`
	AQI   int     `json:"aqi"`
	Level string  `json:"level"`
}

// RecallModule contains journal recall information.
type RecallModule struct {
	Anniversaries []*AnniversaryEntry `json:"anniversaries,omitempty"`
	MoodTrend     *MoodTrend          `json:"mood_trend,omitempty"`
	Offline       bool                `json:"offline"`
}

// AnniversaryEntry represents a journal entry from a past anniversary.
type AnniversaryEntry struct {
	YearsAgo    int       `json:"years_ago"`
	Date        string    `json:"date"`
	Text        string    `json:"text"`
	EmojiMood   string    `json:"emoji_mood"`
	Anniversary *Anniversary `json:"anniversary"`
}

// Anniversary represents an anniversary event.
type Anniversary struct {
	Type string `json:"type"` // e.g., "1Y", "3Y", "7Y"
	Name string `json:"name"` // e.g., "1 Year Ago"
}

// MoodTrend represents mood trend over a period.
type MoodTrend struct {
	Period     string  `json:"period"`      // e.g., "7 days"
	AvgValence float64 `json:"avg_valence"`
	AvgArousal float64 `json:"avg_arousal"`
	Trend      string  `json:"trend"`       // "improving", "stable", "declining"
}

// DateModule contains date and calendar information.
type DateModule struct {
	Today       string       `json:"today"`        // YYYY-MM-DD
	DayOfWeek   string       `json:"day_of_week"`  // Korean day name
	SolarTerm   *SolarTerm   `json:"solar_term,omitempty"`
	Holiday     *KoreanHoliday `json:"holiday,omitempty"`
}

// SolarTerm represents a 24 solar term.
type SolarTerm struct {
	Name      string    `json:"name"`      // e.g., "입춘"
	NameHanja string    `json:"name_hanja"` // e.g., "立春"
	Date      string    `json:"date"`      // YYYY-MM-DD
}

// KoreanHoliday represents a Korean holiday.
type KoreanHoliday struct {
	Name       string `json:"name"`       // e.g., "설날"
	NameHanja  string `json:"name_hanja"` // e.g., "春節"
	Date       string `json:"date"`       // YYYY-MM-DD
	IsHoliday  bool   `json:"is_holiday"`
}

// MantraModule contains daily mantra information.
type MantraModule struct {
	Text     string    `json:"text"`
	Source   string    `json:"source,omitempty"`
	Index    int       `json:"index"`    // For rotation
	Total    int       `json:"total"`    // Total mantras in rotation
}

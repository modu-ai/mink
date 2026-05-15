package briefing

import (
	"context"
	"fmt"
	"strings"

	"github.com/modu-ai/mink/internal/llm"
)

// LLMSummaryRequest is the structured, categorical-only signal set that is
// transmitted to the LLM provider for briefing summary generation.
//
// REQ-BR-054 / AC-009 invariant 5: the request MUST NOT include
//   - journal entry text (any content of AnniversaryEntry.Text)
//   - mantra raw text (MantraModule.Text)
//   - Telegram chat_id raw values
//   - precise geographic coordinates (lat/lon floats)
//   - API keys / tokens
//
// Only categorical signals (counts, enumerated statuses, weather summary
// tokens, calendar metadata, mood trend slope sign, location city name) are
// permitted.
//
// @MX:ANCHOR: LLMSummaryRequest enforces the categorical-only invariant for
// the LLM channel.
// @MX:REASON: SPEC-MINK-BRIEFING-001 REQ-BR-054 / AC-009 invariant 5.
type LLMSummaryRequest struct {
	// Date is the briefing date in YYYY-MM-DD format.
	Date string `json:"date"`
	// DayOfWeek is the Korean weekday name (e.g., "목요일").
	DayOfWeek string `json:"day_of_week"`

	// WeatherStatus is one of: ok | offline | timeout | skipped | error.
	WeatherStatus string `json:"weather_status"`
	// WeatherCondition is the categorical weather token (e.g., "Clear", "Rain").
	// Never includes raw API response or exact temperatures beyond rounded int.
	WeatherCondition string `json:"weather_condition,omitempty"`
	// WeatherTempInt is the rounded current temperature (integer degrees C).
	WeatherTempInt int `json:"weather_temp_int,omitempty"`
	// WeatherLocation is the city-level location name (e.g., "Seoul").
	// Precise coordinates (lat/lon) are NEVER included.
	WeatherLocation string `json:"weather_location,omitempty"`
	// AQILevel is the categorical AQI bucket (e.g., "good", "moderate", "unhealthy").
	AQILevel string `json:"aqi_level,omitempty"`

	// JournalStatus is one of: ok | offline | timeout | skipped | error.
	JournalStatus string `json:"journal_status"`
	// AnniversaryCount is the number of anniversary entries surfaced today.
	AnniversaryCount int `json:"anniversary_count"`
	// AnniversaryYears is the list of years-ago values (e.g., [1, 3, 7]).
	// Counts and bucket labels only -- never entry text.
	AnniversaryYears []int `json:"anniversary_years,omitempty"`
	// MoodTrendSlope is the trend direction sign: -1 declining, 0 stable, +1 improving.
	MoodTrendSlope int `json:"mood_trend_slope"`

	// SolarTermPresent indicates whether today coincides with a 24-solar-term.
	SolarTermPresent bool `json:"solar_term_present"`
	// HolidayPresent indicates whether today is a Korean holiday.
	HolidayPresent bool `json:"holiday_present"`

	// MantraPresent indicates whether a daily mantra is configured.
	// MantraText is NEVER included (REQ-BR-054).
	MantraPresent bool `json:"mantra_present"`
}

// BuildLLMSummaryRequest extracts categorical signals from the payload.
// This is the single chokepoint that enforces invariant 5: any future change
// must keep entry text / chat_id / precise coordinates out of the request.
func BuildLLMSummaryRequest(payload *BriefingPayload) *LLMSummaryRequest {
	if payload == nil {
		return nil
	}

	req := &LLMSummaryRequest{
		Date:          payload.DateCalendar.Today,
		DayOfWeek:     payload.DateCalendar.DayOfWeek,
		WeatherStatus: payload.Status["weather"],
		JournalStatus: payload.Status["journal"],
		MantraPresent: payload.Mantra.Text != "",
	}

	if payload.Weather.Current != nil {
		req.WeatherCondition = payload.Weather.Current.Condition
		req.WeatherTempInt = int(payload.Weather.Current.Temp + 0.5)
		req.WeatherLocation = payload.Weather.Current.Location
	}
	if payload.Weather.AirQuality != nil {
		req.AQILevel = payload.Weather.AirQuality.Level
	}

	if n := len(payload.JournalRecall.Anniversaries); n > 0 {
		req.AnniversaryCount = n
		years := make([]int, 0, n)
		for _, a := range payload.JournalRecall.Anniversaries {
			if a != nil {
				years = append(years, a.YearsAgo)
			}
		}
		req.AnniversaryYears = years
	}
	if t := payload.JournalRecall.MoodTrend; t != nil {
		switch strings.ToLower(t.Trend) {
		case "improving":
			req.MoodTrendSlope = 1
		case "declining":
			req.MoodTrendSlope = -1
		default:
			req.MoodTrendSlope = 0
		}
	}

	req.SolarTermPresent = payload.DateCalendar.SolarTerm != nil
	req.HolidayPresent = payload.DateCalendar.Holiday != nil && payload.DateCalendar.Holiday.IsHoliday

	return req
}

// FormatLLMPrompt renders the categorical request to a text prompt suitable
// for the LLM. The output contains ONLY the fields enumerated in
// LLMSummaryRequest -- no template can leak entry text or chat_id because
// those fields do not exist on the struct.
func FormatLLMPrompt(req *LLMSummaryRequest) string {
	if req == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("아래 카테고리 신호를 바탕으로 한국어로 2~3줄의 따뜻한 아침 요약을 작성해주세요. ")
	sb.WriteString("개인 정보(일기 내용, mantra 본문, 좌표, chat_id)는 포함되지 않았으며 작성 결과에도 포함하지 마세요.\n\n")
	fmt.Fprintf(&sb, "- date: %s (%s)\n", req.Date, req.DayOfWeek)
	fmt.Fprintf(&sb, "- weather: status=%s", req.WeatherStatus)
	if req.WeatherCondition != "" {
		fmt.Fprintf(&sb, ", cond=%s, temp≈%d°C", req.WeatherCondition, req.WeatherTempInt)
	}
	if req.WeatherLocation != "" {
		fmt.Fprintf(&sb, ", loc=%s", req.WeatherLocation)
	}
	if req.AQILevel != "" {
		fmt.Fprintf(&sb, ", aqi=%s", req.AQILevel)
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "- journal: status=%s, anniversaries=%d", req.JournalStatus, req.AnniversaryCount)
	if len(req.AnniversaryYears) > 0 {
		fmt.Fprintf(&sb, ", years=%v", req.AnniversaryYears)
	}
	fmt.Fprintf(&sb, ", mood_slope=%+d", req.MoodTrendSlope)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "- calendar: solar_term=%t, holiday=%t\n", req.SolarTermPresent, req.HolidayPresent)
	fmt.Fprintf(&sb, "- mantra: present=%t\n", req.MantraPresent)
	sb.WriteString("\n응답은 분석/진단 어휘 없이 2~3 줄로만.")
	return sb.String()
}

// GenerateLLMSummary calls the provided LLMProvider with a categorical-only
// payload and returns the generated 2-3 line briefing summary.
//
// Returns ("", nil) when:
//   - cfg.LLMSummary is false (M1/M2 deterministic mode)
//   - provider is nil
//   - payload is nil
//
// Returns ("", err) on provider error.
//
// REQ-BR-032 (optional LLM summary), REQ-BR-054 (categorical-only payload).
func GenerateLLMSummary(ctx context.Context, provider llm.LLMProvider, payload *BriefingPayload, cfg *Config, model string) (string, error) {
	if cfg == nil || !cfg.LLMSummary {
		return "", nil
	}
	if provider == nil || payload == nil {
		return "", nil
	}

	req := BuildLLMSummaryRequest(payload)
	prompt := FormatLLMPrompt(req)

	chosenModel := model
	if chosenModel == "" {
		chosenModel = "default"
	}
	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: chosenModel,
		Messages: []llm.Message{
			{Role: "system", Content: "You write 2-3 line Korean morning briefings using only the categorical signals provided. Never include clinical or diagnostic vocabulary."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("briefing llm summary: %w", err)
	}
	return strings.TrimSpace(resp.Text), nil
}

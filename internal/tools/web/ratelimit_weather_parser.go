package web

import (
	"time"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
)

// weatherParser parses OpenWeatherMap rate-limit response headers into a
// RateLimitState. OWM uses standard X-RateLimit-* headers with a
// per-minute request quota.
//
// @MX:ANCHOR: [AUTO] WeatherParser — registered on Tracker before first weather_current call
// @MX:REASON: SPEC-GOOSE-WEATHER-001 REQ-WEATHER-009 — fan_in >= 3 (RegisterWeatherParser, tracker, test)
type weatherParser struct{}

// NewWeatherParser returns a ratelimit.Parser that handles OWM rate-limit headers.
func NewWeatherParser() ratelimit.Parser {
	return &weatherParser{}
}

// Provider returns the provider name "openweathermap".
func (p *weatherParser) Provider() string { return "openweathermap" }

// Parse reads OWM rate-limit headers and returns a RateLimitState.
// OWM uses the same X-RateLimit-* header format as Brave Search.
// Missing or malformed headers leave the corresponding bucket at zero-value.
func (p *weatherParser) Parse(headers map[string]string, now time.Time) (ratelimit.RateLimitState, []string) {
	var debugMsgs []string

	state := ratelimit.RateLimitState{
		Provider:   "openweathermap",
		CapturedAt: now,
	}

	bucket := ratelimit.RateLimitBucket{CapturedAt: now}

	if limitStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Limit"); ok {
		if n, msg := parseHeaderInt(limitStr); msg != "" {
			debugMsgs = append(debugMsgs, "openweathermap X-RateLimit-Limit: "+msg)
		} else {
			bucket.Limit = n
		}
	}

	if remStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Remaining"); ok {
		if n, msg := parseHeaderInt(remStr); msg != "" {
			debugMsgs = append(debugMsgs, "openweathermap X-RateLimit-Remaining: "+msg)
		} else {
			bucket.Remaining = n
		}
	}

	if resetStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Reset"); ok {
		if secs, msg := parseHeaderInt(resetStr); msg != "" {
			debugMsgs = append(debugMsgs, "openweathermap X-RateLimit-Reset: "+msg)
		} else {
			bucket.ResetSeconds = float64(secs)
		}
	}

	state.RequestsMin = bucket
	state.RequestsHour = ratelimit.RateLimitBucket{CapturedAt: now}
	state.TokensMin = ratelimit.RateLimitBucket{CapturedAt: now}
	state.TokensHour = ratelimit.RateLimitBucket{CapturedAt: now}

	return state, debugMsgs
}

// RegisterWeatherParser registers a WeatherParser on the given Tracker so
// that tracker.Parse("openweathermap", headers, now) succeeds. Call this
// once at bootstrap or after creating a new Tracker for weather_current.
func RegisterWeatherParser(t *ratelimit.Tracker) {
	if t == nil {
		return
	}
	t.RegisterParser(NewWeatherParser())
}

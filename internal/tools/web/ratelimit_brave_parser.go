package web

import (
	"strconv"
	"strings"
	"time"

	"github.com/modu-ai/goose/internal/llm/ratelimit"
)

// braveParser parses Brave Search API rate-limit response headers into a
// RateLimitState. Brave exposes the following headers (as of 2026-05):
//
//	X-RateLimit-Limit     – total requests per window
//	X-RateLimit-Remaining – requests remaining in current window
//	X-RateLimit-Reset     – seconds until the window resets (integer)
//
// Brave does not expose separate minute/hour buckets; only RequestsMin is
// populated. RequestsHour, TokensMin, TokensHour remain at zero-value.
//
// @MX:ANCHOR: [AUTO] BraveParser — registered on Tracker before first web_search call
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 DC-16 — fan_in >= 3 (RegisterBraveParser, tracker.Parse, test)
type braveParser struct{}

// NewBraveParser returns a ratelimit.Parser that handles Brave Search headers.
func NewBraveParser() ratelimit.Parser {
	return &braveParser{}
}

// Provider returns the provider name used as the key in ratelimit.Tracker.
func (p *braveParser) Provider() string { return "brave" }

// Parse reads Brave rate-limit headers and returns a RateLimitState.
// Missing or malformed headers leave the corresponding bucket at zero-value
// (non-fatal, per REQ-RL-006).
func (p *braveParser) Parse(headers map[string]string, now time.Time) (ratelimit.RateLimitState, []string) {
	var debugMsgs []string

	state := ratelimit.RateLimitState{
		Provider:   "brave",
		CapturedAt: now,
	}

	// RequestsMin bucket — derived from X-RateLimit-* headers.
	bucket := ratelimit.RateLimitBucket{CapturedAt: now}

	if limitStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Limit"); ok {
		if n, msg := parseHeaderInt(limitStr); msg != "" {
			debugMsgs = append(debugMsgs, "brave X-RateLimit-Limit: "+msg)
		} else {
			bucket.Limit = n
		}
	}

	if remStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Remaining"); ok {
		if n, msg := parseHeaderInt(remStr); msg != "" {
			debugMsgs = append(debugMsgs, "brave X-RateLimit-Remaining: "+msg)
		} else {
			bucket.Remaining = n
		}
	}

	if resetStr, ok := ratelimit.CaseInsensitiveGet(headers, "X-RateLimit-Reset"); ok {
		if secs, msg := parseHeaderInt(resetStr); msg != "" {
			debugMsgs = append(debugMsgs, "brave X-RateLimit-Reset: "+msg)
		} else {
			bucket.ResetSeconds = float64(secs)
		}
	}

	state.RequestsMin = bucket

	// Brave does not expose hour/token buckets; use zero-value with CapturedAt set.
	state.RequestsHour = ratelimit.RateLimitBucket{CapturedAt: now}
	state.TokensMin = ratelimit.RateLimitBucket{CapturedAt: now}
	state.TokensHour = ratelimit.RateLimitBucket{CapturedAt: now}

	return state, debugMsgs
}

// parseHeaderInt parses a header value string as an integer.
// Returns (0, error message) on failure; matches the ratelimit package convention.
func parseHeaderInt(s string) (int, string) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, "malformed integer: " + s
	}
	return v, ""
}

// RegisterBraveParser registers a BraveParser on the given Tracker so that
// tracker.Parse("brave", headers, now) succeeds. Call this once at bootstrap
// or after creating a new Tracker for web_search.
//
// @MX:ANCHOR: [AUTO] Registration entry point for BraveParser on Tracker
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 DC-16; called at bootstrap; fan_in >= 3
func RegisterBraveParser(t *ratelimit.Tracker) {
	if t == nil {
		return
	}
	t.RegisterParser(NewBraveParser())
}

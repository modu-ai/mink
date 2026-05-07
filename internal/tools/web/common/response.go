// Package common provides shared infrastructure for web tools:
// standard response wrapper, user-agent builder, bbolt TTL cache,
// robots.txt fetcher, blocklist+redirect+size safety guards, and
// dependency-injection struct.
//
// SPEC: SPEC-GOOSE-TOOLS-WEB-001
package common

import (
	"encoding/json"
	"fmt"
)

// Response is the standard wrapper for all web tool responses.
// AC-WEB-012: all web tools must return this exact shape.
//
// @MX:ANCHOR: [AUTO] Standard response envelope used by every web tool
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-012 — fan_in >= 8 (one per web tool)
type Response struct {
	// OK indicates whether the tool call succeeded.
	OK bool `json:"ok"`
	// Data holds the successful result payload (JSON-encoded).
	Data json.RawMessage `json:"data,omitempty"`
	// Error holds the error payload when OK is false.
	Error *ErrPayload `json:"error,omitempty"`
	// Metadata contains cache and timing information.
	Metadata Metadata `json:"metadata"`
}

// ErrPayload carries structured error information.
type ErrPayload struct {
	// Code is a snake_case error code (e.g. "host_blocked", "robots_disallow").
	Code string `json:"code"`
	// Message is a human-readable description.
	Message string `json:"message"`
	// Retryable indicates whether the caller may safely retry.
	Retryable bool `json:"retryable"`
	// RetryAfterSeconds is the suggested wait time in seconds (0 means unspecified).
	RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`
}

// Metadata contains per-call performance and cache information.
type Metadata struct {
	// CacheHit is true when the response was served from cache.
	CacheHit bool `json:"cache_hit"`
	// DurationMs is the wall-clock time for the call in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// OKResponse constructs a successful Response with data marshaled from v.
// Returns an error if v cannot be marshaled to JSON.
func OKResponse(v any, meta Metadata) (Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Response{}, fmt.Errorf("marshal ok response data: %w", err)
	}
	return Response{
		OK:       true,
		Data:     json.RawMessage(b),
		Metadata: meta,
	}, nil
}

// ErrResponse constructs a failure Response with the given error fields.
// code must be a snake_case constant; retryAfter is seconds (0 = unspecified).
func ErrResponse(code, message string, retryable bool, retryAfter int, meta Metadata) Response {
	return Response{
		OK: false,
		Error: &ErrPayload{
			Code:              code,
			Message:           message,
			Retryable:         retryable,
			RetryAfterSeconds: retryAfter,
		},
		Metadata: meta,
	}
}

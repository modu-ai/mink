package llm

import (
	"fmt"
	"net/http"
)

// LLMError is the base interface for all LLM-specific errors.
// SPEC-GOOSE-LLM-001 REQ-LLM-003: All errors returned to callers must be typed LLMError.
//
// @MX:ANCHOR: [AUTO] LLMError — base error interface for all LLM operations
// @MX:REASON: All provider adapters wrap errors as LLMError subclasses for consistent error handling
type LLMError interface {
	error
	// Unwrap returns the underlying error for errors.Is/As.
	Unwrap() error
	// Type returns the error type for logging/metrics.
	Type() string
	// Temporary returns true if the error is transient (retryable).
	Temporary() bool
}

// BaseLLMError provides common implementation for LLM errors.
type BaseLLMError struct {
	Msg         string
	Err         error
	IsTemporary bool
	TypeName    string
}

func (e *BaseLLMError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *BaseLLMError) Unwrap() error {
	return e.Err
}

func (e *BaseLLMError) Type() string {
	return e.TypeName
}

func (e *BaseLLMError) Temporary() bool {
	return e.IsTemporary
}

// ErrRateLimited is returned when the provider rate limits the request (HTTP 429).
// SPEC-GOOSE-LLM-001 REQ-LLM-005: Retry with exponential backoff.
type ErrRateLimited struct {
	*BaseLLMError
	// RetryAfter is the suggested retry duration (if provided by provider).
	RetryAfter string
}

// NewErrRateLimited creates a new rate limit error.
func NewErrRateLimited(message string, retryAfter string) *ErrRateLimited {
	return &ErrRateLimited{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: true,
			TypeName:    "rate_limited",
		},
		RetryAfter: retryAfter,
	}
}

// ErrContextTooLong is returned when the prompt exceeds the model's context window.
// SPEC-GOOSE-LLM-001: No retry (non-transient).
type ErrContextTooLong struct {
	*BaseLLMError
	// MaxTokens is the maximum context window size.
	MaxTokens int
	// ProvidedTokens is the token count that exceeded the limit.
	ProvidedTokens int
}

// NewErrContextTooLong creates a new context length error.
func NewErrContextTooLong(message string, maxTokens, providedTokens int) *ErrContextTooLong {
	return &ErrContextTooLong{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: false,
			TypeName:    "context_too_long",
		},
		MaxTokens:      maxTokens,
		ProvidedTokens: providedTokens,
	}
}

// ErrServerUnavailable is returned when the provider server is unavailable (HTTP 5xx).
// SPEC-GOOSE-LLM-001 REQ-LLM-005: Retry with exponential backoff.
type ErrServerUnavailable struct {
	*BaseLLMError
	// StatusCode is the HTTP status code (5xx).
	StatusCode int
}

// NewErrServerUnavailable creates a new server unavailable error.
func NewErrServerUnavailable(message string, statusCode int) *ErrServerUnavailable {
	return &ErrServerUnavailable{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: true,
			TypeName:    "server_unavailable",
		},
		StatusCode: statusCode,
	}
}

// ErrInvalidRequest is returned for HTTP 4xx errors (except 429).
// SPEC-GOOSE-LLM-001 REQ-LLM-006: No retry (client error).
type ErrInvalidRequest struct {
	*BaseLLMError
	// StatusCode is the HTTP status code (4xx).
	StatusCode int
}

// NewErrInvalidRequest creates a new invalid request error.
func NewErrInvalidRequest(message string, statusCode int) *ErrInvalidRequest {
	return &ErrInvalidRequest{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: false,
			TypeName:    "invalid_request",
		},
		StatusCode: statusCode,
	}
}

// ErrUnauthorized is returned for authentication failures (HTTP 401/403).
// SPEC-GOOSE-LLM-001: No retry (credentials won't change without reconfiguration).
type ErrUnauthorized struct {
	*BaseLLMError
	// StatusCode is the HTTP status code (401 or 403).
	StatusCode int
}

// NewErrUnauthorized creates a new unauthorized error.
func NewErrUnauthorized(message string, statusCode int) *ErrUnauthorized {
	return &ErrUnauthorized{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: false,
			TypeName:    "unauthorized",
		},
		StatusCode: statusCode,
	}
}

// ErrModelNotFound is returned when the requested model does not exist.
// SPEC-GOOSE-LLM-001 REQ-LLM-007: Returned for HTTP 404 with "model not found" body.
type ErrModelNotFound struct {
	*BaseLLMError
	// Model is the requested model name.
	Model string
}

// NewErrModelNotFound creates a new model not found error.
func NewErrModelNotFound(model string) *ErrModelNotFound {
	return &ErrModelNotFound{
		BaseLLMError: &BaseLLMError{
			Msg:         fmt.Sprintf("model %q not found", model),
			IsTemporary: false,
			TypeName:    "model_not_found",
		},
		Model: model,
	}
}

// ErrMalformedStream is returned when streaming response contains malformed data.
// SPEC-GOOSE-LLM-001 REQ-LLM-010: Adapter sends final chunk with error and closes channel.
type ErrMalformedStream struct {
	*BaseLLMError
	// Line is the malformed line (if available).
	Line string
}

// NewErrMalformedStream creates a new malformed stream error.
func NewErrMalformedStream(line string, err error) *ErrMalformedStream {
	return &ErrMalformedStream{
		BaseLLMError: &BaseLLMError{
			Msg:         fmt.Sprintf("malformed stream line: %q", line),
			Err:         err,
			IsTemporary: false,
			TypeName:    "malformed_stream",
		},
		Line: line,
	}
}

// ErrInvalidConfig is returned for configuration errors.
// SPEC-GOOSE-LLM-001 REQ-LLM-013: HTTP 3xx redirects are treated as config errors.
type ErrInvalidConfig struct {
	*BaseLLMError
	// ConfigKey is the problematic configuration key.
	ConfigKey string
}

// NewErrInvalidConfig creates a new invalid config error.
func NewErrInvalidConfig(message, configKey string) *ErrInvalidConfig {
	return &ErrInvalidConfig{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: false,
			TypeName:    "invalid_config",
		},
		ConfigKey: configKey,
	}
}

// MapHTTPStatusToError maps HTTP status codes to appropriate LLMError types.
// SPEC-GOOSE-LLM-001 §6.3: Centralized error mapping for all providers.
//
// @MX:ANCHOR: [AUTO] MapHTTPStatusToError — centralized HTTP status to LLMError mapping
// @MX:REASON: All provider adapters use this function for consistent error classification
func MapHTTPStatusToError(statusCode int, responseBody []byte) LLMError {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return NewErrRateLimited("rate limit exceeded", "")
	case statusCode >= 500:
		return NewErrServerUnavailable("server unavailable", statusCode)
	case statusCode == http.StatusNotFound:
		// Check if it's a model not found error
		bodyStr := string(responseBody)
		if contains(bodyStr, "model") && contains(bodyStr, "not found") {
			return NewErrModelNotFound(extractModelName(bodyStr))
		}
		return NewErrInvalidRequest("resource not found", statusCode)
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return NewErrUnauthorized("authentication failed", statusCode)
	case statusCode >= 400 && statusCode < 500:
		return NewErrInvalidRequest("invalid request", statusCode)
	default:
		return NewErrServerUnexpected(fmt.Sprintf("unexpected status code: %d", statusCode))
	}
}

// ErrServerUnexpected is returned for unexpected server responses.
type ErrServerUnexpected struct {
	*BaseLLMError
}

func NewErrServerUnexpected(message string) *ErrServerUnexpected {
	return &ErrServerUnexpected{
		BaseLLMError: &BaseLLMError{
			Msg:         message,
			IsTemporary: true,
			TypeName:    "server_unexpected",
		},
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractModelName(body string) string {
	// Simple extraction: look for model name in quotes after "model"
	// This is a basic implementation; can be enhanced with proper JSON parsing
	// For now, return "unknown" to avoid false positives
	return "unknown"
}

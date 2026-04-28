package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRetry_RetriesOn503 validates AC-LLM-003: Retry on 503.
func TestRetry_RetriesOn503(t *testing.T) {
	// Given: policy with max retries = 3
	policy := RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFrac:   0.0, // Disable jitter for deterministic test
	}

	attemptCount := 0
	fn := func() (string, error) {
		attemptCount++
		if attemptCount < 3 {
			// First 2 attempts fail with 503
			return "", NewErrServerUnavailable("server error", 503)
		}
		// Third attempt succeeds
		return "success", nil
	}

	// When: call Retry
	ctx := context.Background()
	result, err := Retry(ctx, policy, fn)

	// Then: verify success after 3 attempts
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}

	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
}

// TestRetry_NoRetryOn400 validates AC-LLM-004: No retry on 400.
func TestRetry_NoRetryOn400(t *testing.T) {
	// Given: policy with max retries = 3
	policy := DefaultRetryPolicy()

	attemptCount := 0
	fn := func() (string, error) {
		attemptCount++
		// Always return 400
		return "", NewErrInvalidRequest("bad request", 400)
	}

	// When: call Retry
	ctx := context.Background()
	_, err := Retry(ctx, policy, fn)

	// Then: verify error and only 1 attempt
	if err == nil {
		t.Fatal("expected error")
	}

	var invalidReq *ErrInvalidRequest
	if !errors.As(err, &invalidReq) {
		t.Fatalf("expected ErrInvalidRequest, got %T: %v", err, err)
	}

	if attemptCount != 1 {
		t.Errorf("expected 1 attempt (no retry on 400), got %d", attemptCount)
	}
}

// TestRetry_MaxRetriesExhausted validates behavior when max retries exceeded.
func TestRetry_MaxRetriesExhausted(t *testing.T) {
	// Given: policy with max retries = 2
	policy := RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFrac:   0.0,
	}

	attemptCount := 0
	fn := func() (string, error) {
		attemptCount++
		// Always fail with 503
		return "", NewErrServerUnavailable("server error", 503)
	}

	// When: call Retry
	ctx := context.Background()
	_, err := Retry(ctx, policy, fn)

	// Then: verify error after max retries + 1 initial attempt
	if err == nil {
		t.Fatal("expected error")
	}

	expectedAttempts := policy.MaxRetries + 1 // 3 total (1 initial + 2 retries)
	if attemptCount != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, attemptCount)
	}

	// Verify error contains last error (Retry wraps the error)
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("error should wrap last error, got: %v", err)
	}
}

// TestRetry_ContextCancellation validates retry respects context cancellation.
func TestRetry_ContextCancellation(t *testing.T) {
	// Given: policy with delay
	policy := RetryPolicy{
		MaxRetries:   10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		JitterFrac:   0.0,
	}

	attemptCount := 0
	fn := func() (string, error) {
		attemptCount++
		return "", NewErrServerUnavailable("server error", 503)
	}

	// When: cancel context before first retry
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := Retry(ctx, policy, fn)

	// Then: verify context cancellation error
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}

	if attemptCount != 1 {
		t.Errorf("expected 1 attempt (cancelled before retry), got %d", attemptCount)
	}
}

// TestRetryPolicy_ShouldRetry validates retry logic for different error types.
func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "server unavailable (5xx)",
			err:      NewErrServerUnavailable("server error", 503),
			expected: true,
		},
		{
			name:     "rate limited (429)",
			err:      NewErrRateLimited("rate limit", ""),
			expected: true,
		},
		{
			name:     "invalid request (4xx)",
			err:      NewErrInvalidRequest("bad request", 400),
			expected: false,
		},
		{
			name:     "unauthorized (401)",
			err:      NewErrUnauthorized("unauthorized", 401),
			expected: false,
		},
		{
			name:     "context too long",
			err:      NewErrContextTooLong("context too long", 4096, 5000),
			expected: false,
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "unknown error (network error)",
			err:      errors.New("connection refused"),
			expected: true, // Unknown errors are treated as temporary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.ShouldRetry(tt.err)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestRetryPolicy_NextDelay validates exponential backoff calculation.
func TestRetryPolicy_NextDelay(t *testing.T) {
	policy := RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
		JitterFrac:   0.0, // Disable for deterministic test
	}

	tests := []struct {
		attempt          int
		expectedMinDelay time.Duration
		expectedMaxDelay time.Duration
	}{
		{0, 100 * time.Millisecond, 100 * time.Millisecond},     // 100ms * 2^0 = 100ms
		{1, 200 * time.Millisecond, 200 * time.Millisecond},     // 100ms * 2^1 = 200ms
		{2, 400 * time.Millisecond, 400 * time.Millisecond},     // 100ms * 2^2 = 400ms
		{3, 500 * time.Millisecond, 500 * time.Millisecond},     // 100ms * 2^3 = 800ms → capped at 500ms
		{4, 500 * time.Millisecond, 500 * time.Millisecond},     // Capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := policy.NextDelay(tt.attempt)
			if delay < tt.expectedMinDelay || delay > tt.expectedMaxDelay {
				t.Errorf("NextDelay(%d) = %v, expected [%v, %v]", tt.attempt, delay, tt.expectedMinDelay, tt.expectedMaxDelay)
			}
		})
	}
}

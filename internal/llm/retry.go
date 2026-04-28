package llm

import (
	"context"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"time"
)

// RetryPolicy defines retry behavior for transient errors.
// SPEC-GOOSE-LLM-001 §6.4: Exponential backoff with jitter.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts (default 3).
	MaxRetries int

	// InitialDelay is the initial backoff delay (default 200ms).
	InitialDelay time.Duration

	// MaxDelay is the maximum backoff delay (default 5s).
	MaxDelay time.Duration

	// Multiplier is the exponential backoff multiplier (default 2.0).
	Multiplier float64

	// JitterFrac is the jitter fraction (default 0.2 = 20%).
	JitterFrac float64
}

// DefaultRetryPolicy returns a retry policy with default values.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 200 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		JitterFrac:   0.2,
	}
}

// ShouldRetry determines if an error should be retried.
// SPEC-GOOSE-LLM-001 §6.4:
// - Retry: ErrServerUnavailable, ErrRateLimited, network errors
// - No retry: ErrInvalidRequest, ErrUnauthorized, ErrContextTooLong, context cancellation
func (p RetryPolicy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check for LLMError types
	if llmErr, ok := err.(LLMError); ok {
		return llmErr.Temporary()
	}

	// Check for context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Default: treat unknown errors as temporary (network errors, etc.)
	return true
}

// NextDelay calculates the delay before the next retry attempt.
// Uses exponential backoff with jitter: delay = base * multiplier^attempt + jitter
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential backoff
	expDelay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt))

	// Cap at MaxDelay
	if expDelay > float64(p.MaxDelay) {
		expDelay = float64(p.MaxDelay)
	}

	// Add jitter: ±JitterFrac * delay
	// Using math/rand/v2 which is auto-seeded and thread-safe
	jitter := expDelay * p.JitterFrac * (2*mrand.Float64() - 1)

	return time.Duration(expDelay + jitter)
}

// Retry executes a function with retry logic.
// Returns the result or the last error if all retries are exhausted.
// SPEC-GOOSE-LLM-001 REQ-LLM-005, REQ-LLM-006: Retry on 5xx/429, no retry on 4xx.
//
// @MX:ANCHOR: [AUTO] Retry — centralized retry logic for all LLM operations
// @MX:REASON: All provider adapters use this function for consistent retry behavior
func Retry[T any](ctx context.Context, policy RetryPolicy, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := policy.NextDelay(attempt - 1)
			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("retry cancelled before attempt %d: %w", attempt, ctx.Err())
			case <-time.After(delay):
				// Proceed with retry
			}
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if !policy.ShouldRetry(err) {
			return zero, fmt.Errorf("non-retryable error on attempt %d: %w", attempt+1, err)
		}

		// Check if this was the last attempt
		if attempt == policy.MaxRetries {
			break
		}

		// Log retry attempt (if logger is available)
		// This is a no-op for now; will be integrated with logger in TELEM-001
	}

	// All retries exhausted
	return zero, fmt.Errorf("max retries (%d) exhausted: %w", policy.MaxRetries, lastErr)
}

// WithRetry wraps an LLMProvider to add retry logic.
// The returned provider delegates all calls to the underlying provider with retry.
func WithRetry(provider LLMProvider, policy RetryPolicy) LLMProvider {
	return &retryProvider{
		underlying: provider,
		policy:     policy,
	}
}

// retryProvider wraps an LLMProvider with retry logic.
type retryProvider struct {
	underlying LLMProvider
	policy     RetryPolicy
}

func (r *retryProvider) Name() string {
	return r.underlying.Name()
}

func (r *retryProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	return Retry(ctx, r.policy, func() (CompletionResponse, error) {
		return r.underlying.Complete(ctx, req)
	})
}

func (r *retryProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error) {
	// Streaming with retry is complex: we need to return a channel immediately
	// but retry happens inside. For now, delegate without retry.
	// A full implementation would need a channel multiplexer.
	// SPEC-GOOSE-LLM-001: Streaming retry is deferred to future enhancement.
	return r.underlying.Stream(ctx, req)
}

func (r *retryProvider) CountTokens(ctx context.Context, text string) (int, error) {
	return Retry(ctx, r.policy, func() (int, error) {
		return r.underlying.CountTokens(ctx, text)
	})
}

func (r *retryProvider) Capabilities(ctx context.Context, model string) (Capabilities, error) {
	// Capabilities are cached; retry is less critical
	// Use direct call without retry to avoid delays
	return r.underlying.Capabilities(ctx, model)
}

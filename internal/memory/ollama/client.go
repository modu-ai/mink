// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package ollama is a minimal client for the Ollama embeddings API.
//
// It is the only outbound HTTP dependency of internal/memory and
// intentionally restricts its endpoint allowlist (REQ-MEM-031, AC-MEM-031).
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.1
// REQ:  REQ-MEM-031
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// sentinel errors that ShouldFallbackToBM25 keys on via errors.Is.
var (
	// ErrOllamaUnreachable is returned when the Ollama server cannot be reached.
	ErrOllamaUnreachable = errors.New("ollama: server unreachable")

	// ErrOllamaTimeout is returned when the Ollama server does not respond in time.
	ErrOllamaTimeout = errors.New("ollama: request timed out")

	// ErrOllamaServer is returned when the Ollama server returns a 5xx response.
	ErrOllamaServer = errors.New("ollama: server returned 5xx")

	// ErrCircuitOpen is returned when the circuit breaker is open and the
	// request is rejected without making an HTTP call.
	ErrCircuitOpen = errors.New("ollama: circuit breaker open")

	// ErrEndpointDenied is returned when the baseURL does not match the
	// localhost-only allowlist (REQ-MEM-031).
	ErrEndpointDenied = errors.New("ollama: endpoint not in localhost allowlist (REQ-MEM-031)")
)

// localhostRe matches only localhost origins: http://localhost:<port> or
// http://127.0.0.1:<port>.  The 127.0.0.1 form is required for httptest.Server
// in tests and is semantically equivalent to localhost.
// REQ-MEM-031: only localhost endpoints are permitted.
var localhostRe = regexp.MustCompile(`^http://(localhost|127\.0\.0\.1):[0-9]+$`)

// defaultBaseURL is the standard Ollama endpoint.
const defaultBaseURL = "http://localhost:11434"

// clientTimeout is the HTTP-level timeout for Embed calls.
const clientTimeout = 5 * time.Second

// healthCheckTimeout is the HTTP-level timeout for HealthCheck calls.
const healthCheckTimeout = 1 * time.Second

// cbMaxFailures is the number of consecutive failures that open the circuit.
const cbMaxFailures = 3

// cbOpenDuration is the time the circuit stays open before transitioning to
// half-open to allow a single trial.
const cbOpenDuration = 30 * time.Second

// circuitState is the state machine for the circuit breaker.
type circuitState int

const (
	circuitClosed   circuitState = iota // normal operation
	circuitOpen                         // all calls rejected
	circuitHalfOpen                     // one trial allowed
)

// circuitBreaker implements a simple three-state circuit breaker.
//
// @MX:WARN: [AUTO] Concurrent access to circuitBreaker fields must go through mu.
// @MX:REASON: Client.Embed can be called from concurrent goroutines; unguarded
// reads/writes to failures and openedAt cause data races under go test -race.
type circuitBreaker struct {
	mu       sync.Mutex
	state    circuitState
	failures int
	openedAt time.Time
	nowFunc  func() time.Time // injectable for testing
}

// newCircuitBreaker creates a circuit breaker with the given clock function.
// Pass nil to use time.Now.
func newCircuitBreaker(nowFunc func() time.Time) *circuitBreaker {
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &circuitBreaker{nowFunc: nowFunc}
}

// allow reports whether a call is permitted and transitions state as required.
// Returns false when the circuit is open and the cooldown has not elapsed.
func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if cb.nowFunc().Sub(cb.openedAt) >= cbOpenDuration {
			cb.state = circuitHalfOpen
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	default:
		return true
	}
}

// recordSuccess resets failure count and closes the circuit.
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = circuitClosed
}

// recordFailure increments failure count and opens the circuit after the
// threshold is reached.
func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.state == circuitHalfOpen || cb.failures >= cbMaxFailures {
		cb.state = circuitOpen
		cb.openedAt = cb.nowFunc()
	}
}

// Client is a minimal HTTP client for the Ollama embeddings API.
//
// @MX:ANCHOR: [AUTO] Central Ollama API client; called by Backfiller and CLI vsearch path.
// @MX:REASON: fan_in >= 3 (Backfiller.RunOnce, VectorRunner.RunVector, CLI search wiring).
// Contract: Embed must not write to any persistent state.
type Client struct {
	baseURL    string
	httpClient *http.Client
	breaker    *circuitBreaker
}

// NewClient creates a new Ollama client.
//
// If baseURL is empty, http://localhost:11434 is used.  Any non-localhost URL
// causes immediate failure on the first Embed call (REQ-MEM-031).
func NewClient(baseURL string) *Client {
	return newClientWithClock(baseURL, nil)
}

// newClientWithClock is NewClient with an injectable clock for testing.
func newClientWithClock(baseURL string, nowFunc func() time.Time) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: clientTimeout,
		},
		breaker: newCircuitBreaker(nowFunc),
	}
}

// validateEndpoint returns ErrEndpointDenied when url is not a localhost
// endpoint.  Only http://localhost:<port> is allowed (REQ-MEM-031).
func validateEndpoint(url string) error {
	if !localhostRe.MatchString(url) {
		return ErrEndpointDenied
	}
	return nil
}

// embedRequest is the JSON body sent to POST /api/embeddings.
type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embedResponse is the JSON body received from POST /api/embeddings.
type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed sends model + text to POST {baseURL}/api/embeddings and returns the
// embedding vector.
//
// Errors are wrapped sentinel values that ShouldFallbackToBM25 recognises:
//   - connection refused / DNS failure → ErrOllamaUnreachable
//   - HTTP-level timeout               → ErrOllamaTimeout
//   - 5xx response                     → ErrOllamaServer
//   - circuit open                     → ErrCircuitOpen
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.1
// REQ:  REQ-MEM-031
func (c *Client) Embed(ctx context.Context, model, text string) ([]float32, error) {
	if err := validateEndpoint(c.baseURL); err != nil {
		return nil, err
	}

	if !c.breaker.allow() {
		return nil, ErrCircuitOpen
	}

	body, err := json.Marshal(embedRequest{Model: model, Prompt: text})
	if err != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("ollama.Client.Embed: marshal request: %w", err)
	}

	url := c.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("ollama.Client.Embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.breaker.recordFailure()
		// Classify the transport-level error.
		if isTimeoutError(err) {
			return nil, fmt.Errorf("ollama.Client.Embed: %w: %v", ErrOllamaTimeout, err)
		}
		return nil, fmt.Errorf("ollama.Client.Embed: %w: %v", ErrOllamaUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		_, _ = io.ReadAll(resp.Body)
		c.breaker.recordFailure()
		return nil, fmt.Errorf("ollama.Client.Embed: %w: status %d", ErrOllamaServer, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		c.breaker.recordFailure()
		return nil, fmt.Errorf("ollama.Client.Embed: unexpected status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("ollama.Client.Embed: decode response: %w", err)
	}

	c.breaker.recordSuccess()
	return result.Embedding, nil
}

// HealthCheck pings /api/tags with a 1-second timeout to verify the server
// is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := validateEndpoint(c.baseURL); err != nil {
		return err
	}

	hctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(hctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama.Client.HealthCheck: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTimeoutError(err) {
			return fmt.Errorf("ollama.Client.HealthCheck: %w", ErrOllamaTimeout)
		}
		return fmt.Errorf("ollama.Client.HealthCheck: %w: %v", ErrOllamaUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)
	return nil
}

// isTimeoutError reports whether err represents a timeout condition.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// net.Error implements Timeout().  errors.AsType[T] cannot be used here
	// because T must satisfy the error interface; the bare interface above
	// does not.  The runtime errors.As is reflection-based and accepts it.
	//nolint:errorsastype // see comment above
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

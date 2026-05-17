// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper builds a fake 1024-d embedding.
func fakeEmbedding(dim int) []float32 {
	v := make([]float32, dim)
	for i := range dim {
		v[i] = float32(i) * 0.001
	}
	return v
}

// mockEmbedServer returns a test server that responds with a canned embedding.
func mockEmbedServer(t *testing.T, statusCode int, embedding []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := embedResponse{Embedding: embedding}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestEmbed_happy(t *testing.T) {
	dim := 1024
	want := fakeEmbedding(dim)
	srv := mockEmbedServer(t, http.StatusOK, want)
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Embed(context.Background(), "mxbai-embed-large", "hello")
	require.NoError(t, err)
	assert.Equal(t, dim, len(got))
	assert.InDeltaSlice(t, want, got, 1e-6)
}

func TestEmbed_5xxTriggersErrOllamaServer(t *testing.T) {
	srv := mockEmbedServer(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaServer), "expected ErrOllamaServer, got %v", err)
}

func TestEmbed_timeoutTriggersErrOllamaTimeout(t *testing.T) {
	// Server that sleeps longer than the client timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // much longer than clientTimeout (5s)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClientWithClock(srv.URL, nil)
	c.httpClient.Timeout = 50 * time.Millisecond // override for test speed

	_, err := c.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaTimeout), "expected ErrOllamaTimeout, got %v", err)
}

func TestEmbed_connectionRefusedTriggersErrOllamaUnreachable(t *testing.T) {
	// Use a port that is not listening.
	c := NewClient("http://localhost:19999")
	// Connect and immediately close — guaranteed ECONNREFUSED.
	ln, err := net.Listen("tcp", "127.0.0.1:19999")
	if err == nil {
		_ = ln.Close()
		// Port was available; our client will get connection refused.
	}
	// Even if another process is on 19999, we test the closed-server path.
	srvClosed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	closedURL := srvClosed.URL
	srvClosed.Close() // close immediately to force ECONNREFUSED

	c2 := NewClient(closedURL)
	_, err = c2.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaUnreachable), "expected ErrOllamaUnreachable, got %v", err)
	_ = c
}

func TestCircuitBreaker_opensAfter3ConsecutiveFailures(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	// 3 failures to open the circuit.
	for range 3 {
		_, _ = c.Embed(context.Background(), "model", "text")
	}

	// 4th call must be rejected by the circuit breaker (no HTTP call).
	before := callCount.Load()
	_, err := c.Embed(context.Background(), "model", "text")
	after := callCount.Load()

	assert.True(t, errors.Is(err, ErrCircuitOpen), "expected ErrCircuitOpen, got %v", err)
	assert.Equal(t, before, after, "circuit-open call must not reach the server")
}

func TestCircuitBreaker_returnsErrCircuitOpenImmediately(t *testing.T) {
	var httpCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	// Drain 3 failures to open the circuit.
	for range cbMaxFailures {
		_, _ = c.Embed(context.Background(), "model", "text")
	}

	before := httpCalls.Load()
	_, err := c.Embed(context.Background(), "model", "text")
	assert.True(t, errors.Is(err, ErrCircuitOpen))
	assert.Equal(t, before, httpCalls.Load(), "no HTTP call must be made when circuit is open")
}

func TestCircuitBreaker_halfOpenAfter30s(t *testing.T) {
	// Control time via injectable clock.
	now := time.Now()
	nowFunc := func() time.Time { return now }

	// Server that always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClientWithClock(srv.URL, nowFunc)

	// Trip the circuit.
	for range cbMaxFailures {
		_, _ = c.Embed(context.Background(), "model", "text")
	}

	// Verify it is open.
	_, err := c.Embed(context.Background(), "model", "text")
	assert.True(t, errors.Is(err, ErrCircuitOpen))

	// Advance clock past the cooldown.
	now = now.Add(cbOpenDuration + time.Second)

	// The circuit should allow a half-open trial (gets 500 → back to open).
	_, err = c.Embed(context.Background(), "model", "text")
	// After half-open failure the circuit re-opens, so the next call is ErrCircuitOpen.
	assert.False(t, errors.Is(err, ErrCircuitOpen),
		"half-open trial must reach the server, not be short-circuited: got %v", err)
}

func TestCircuitBreaker_halfOpenSuccessCloses(t *testing.T) {
	now := time.Now()
	nowFunc := func() time.Time { return now }

	var fail atomic.Bool
	fail.Store(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(embedResponse{Embedding: fakeEmbedding(4)})
	}))
	defer srv.Close()

	c := newClientWithClock(srv.URL, nowFunc)

	// Trip the circuit.
	for range cbMaxFailures {
		_, _ = c.Embed(context.Background(), "model", "text")
	}

	// Advance past cooldown.
	now = now.Add(cbOpenDuration + time.Second)
	fail.Store(false)

	// Half-open trial succeeds → circuit closes.
	vec, err := c.Embed(context.Background(), "model", "text")
	require.NoError(t, err)
	assert.NotEmpty(t, vec)

	// Circuit is now closed; subsequent calls succeed without ErrCircuitOpen.
	vec2, err2 := c.Embed(context.Background(), "model", "text")
	require.NoError(t, err2)
	assert.NotEmpty(t, vec2)
}

func TestEmbed_rejectsExternalHost(t *testing.T) {
	c := NewClient("http://api.example.com:11434")
	_, err := c.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEndpointDenied), "expected ErrEndpointDenied, got %v", err)
}

func TestEmbed_rejectsHTTPS(t *testing.T) {
	c := NewClient("https://localhost:11434")
	_, err := c.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEndpointDenied), "expected ErrEndpointDenied, got %v", err)
}

func TestNewClient_defaultURL(t *testing.T) {
	c := NewClient("")
	assert.Equal(t, defaultBaseURL, c.baseURL)
}

func TestHealthCheck_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[]}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestHealthCheck_rejectsExternalHost(t *testing.T) {
	c := NewClient("http://remote.example.com:11434")
	err := c.HealthCheck(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEndpointDenied))
}

func TestEmbed_non5xxErrorNotClassifiedAsServer(t *testing.T) {
	srv := mockEmbedServer(t, http.StatusBadRequest, nil)
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Embed(context.Background(), "model", "text")
	require.Error(t, err)
	// 4xx is not ErrOllamaServer (which is for 5xx).
	assert.False(t, errors.Is(err, ErrOllamaServer))
	assert.False(t, errors.Is(err, ErrOllamaUnreachable))
}

func TestIsTimeoutError_contextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure deadline passes

	assert.True(t, isTimeoutError(ctx.Err()))
}

func TestIsTimeoutError_nil(t *testing.T) {
	assert.False(t, isTimeoutError(nil))
}

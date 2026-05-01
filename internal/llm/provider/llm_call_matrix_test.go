package provider_test

// Integration matrix tests for SPEC-GOOSE-ADAPTER-001-AMEND-001.
// 6 providers × 2 (JSON mode on/off) × 2 (UserID present/empty) = 24 cases.
// All cases route through NewLLMCall to exercise the single-point capability gate (R3 mitigation).
// AC-AMEND-001..AC-AMEND-011 (cross-coverage).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/cache"
	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/anthropic"
	"github.com/modu-ai/goose/internal/llm/provider/deepseek"
	"github.com/modu-ai/goose/internal/llm/provider/google"
	"github.com/modu-ai/goose/internal/llm/provider/ollama"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/provider/xai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// matrixCase describes a single cell in the 6×2×2 integration matrix.
type matrixCase struct {
	providerName   string
	responseFormat string // "" or "json"
	userID         string // "" or "u-test-123"

	// expected outcomes
	expectErr        bool
	expectErrFeature string // if expectErr, the feature name in ErrCapabilityUnsupported
	// body assertions for providers that go through HTTP (non-Google)
	expectRFField   bool // response_format / format present in body
	expectUserField bool // user / metadata.user_id present in body
}

// serverKind describes which response format the test server should return.
type serverKind int

const (
	kindSSE   serverKind = iota // OpenAI / xAI / DeepSeek / Anthropic
	kindJSONL                   // Ollama
	kindNoop                    // Google (uses fake client — no HTTP server needed)
)

// captureHTTPBody is a test server that captures and stores the parsed request JSON body.
type captureHTTPBody struct {
	captured map[string]json.RawMessage
}

func (c *captureHTTPBody) handler(kind serverKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.captured = make(map[string]json.RawMessage)
		_ = json.Unmarshal(body, &c.captured)
		switch kind {
		case kindSSE:
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
		case kindJSONL:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w,
				`{"message":{"role":"assistant","content":"ok"},"done":false}`+"\n"+
					`{"message":{"role":"assistant","content":""},"done":true}`+"\n")
		}
	}
}

// makeOpenAICompat builds an OpenAI-compat provider and captures request.
func makeOpenAICompatProvider(t *testing.T, name string, caps provider.Capabilities) (provider.Provider, *captureHTTPBody, func()) {
	t.Helper()
	cap := &captureHTTPBody{}
	srv := httptest.NewServer(cap.handler(kindSSE))
	pool, store := makePoolAndStore(t, name)

	var p provider.Provider
	switch name {
	case "openai":
		a, err := openai.New(openai.OpenAIOptions{
			Name: "openai", BaseURL: srv.URL, Pool: pool, SecretStore: store,
			Tracker: ratelimit.NewTracker(), Capabilities: caps,
		})
		require.NoError(t, err)
		p = a
	case "xai":
		a, err := xai.New(xai.Options{
			BaseURL: srv.URL, Pool: pool, SecretStore: store, Tracker: ratelimit.NewTracker(),
		})
		require.NoError(t, err)
		p = a
	case "deepseek":
		a, err := deepseek.New(deepseek.Options{
			BaseURL: srv.URL, Pool: pool, SecretStore: store, Tracker: ratelimit.NewTracker(),
		})
		require.NoError(t, err)
		p = a
	}
	return p, cap, srv.Close
}

// makePoolAndStore builds a credential pool and memory secret store for tests.
func makePoolAndStore(t *testing.T, providerName string) (*credential.CredentialPool, provider.SecretStore) {
	t.Helper()
	creds := []*credential.PooledCredential{
		{ID: "c1", Provider: providerName, KeyringID: "k1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)
	store := provider.NewMemorySecretStore(map[string]string{"k1": "test-key"})
	return pool, store
}

// matrixLLMCallFunc wraps a single provider into an LLMCallFunc for matrix testing.
func matrixLLMCallFunc(t *testing.T, p provider.Provider) query.LLMCallFunc {
	t.Helper()
	reg := provider.NewRegistry()
	require.NoError(t, reg.Register(p))
	pool := makeTestPool(t, "mat-c1")
	logger, _ := zap.NewDevelopment()
	return provider.NewLLMCall(
		reg, pool, ratelimit.NewTracker(), &cache.BreakpointPlanner{},
		cache.StrategyNone, cache.TTLEphemeral, nil, logger,
	)
}

// fakeGoogleCapture is a Google fake client that captures the GeminiRequest for assertion.
type fakeGoogleCapture struct {
	capturedFormat string
}

func (f *fakeGoogleCapture) GenerateStream(_ context.Context, req google.GeminiRequest) (google.GeminiStream, error) {
	f.capturedFormat = req.ResponseFormat
	return &simpleFakeStream{}, nil
}

type simpleFakeStream struct{ done bool }

func (s *simpleFakeStream) Next() (*google.GeminiChunk, error) {
	if s.done {
		return nil, google.ErrStreamDone
	}
	s.done = true
	return &google.GeminiChunk{IsDone: true}, nil
}
func (s *simpleFakeStream) Close() {}

// TestLLMCallMatrix is the 6×2×2 integration matrix.
// Each row exercises one provider × response_format × user_id combination.
func TestLLMCallMatrix(t *testing.T) {
	t.Parallel()

	cases := []matrixCase{
		// --- Anthropic (JSONMode=false, UserID=true) ---
		{
			providerName: "anthropic", responseFormat: "", userID: "",
			expectErr: false, expectRFField: false, expectUserField: false,
		},
		{
			providerName: "anthropic", responseFormat: "", userID: "u-test-123",
			expectErr: false, expectRFField: false, expectUserField: true,
		},
		{
			providerName: "anthropic", responseFormat: "json", userID: "",
			expectErr: true, expectErrFeature: "json_mode",
		},
		{
			providerName: "anthropic", responseFormat: "json", userID: "u-test-123",
			expectErr: true, expectErrFeature: "json_mode",
		},

		// --- OpenAI (JSONMode=true, UserID=true) ---
		{
			providerName: "openai", responseFormat: "", userID: "",
			expectErr: false, expectRFField: false, expectUserField: false,
		},
		{
			providerName: "openai", responseFormat: "", userID: "u-test-123",
			expectErr: false, expectRFField: false, expectUserField: true,
		},
		{
			providerName: "openai", responseFormat: "json", userID: "",
			expectErr: false, expectRFField: true, expectUserField: false,
		},
		{
			providerName: "openai", responseFormat: "json", userID: "u-test-123",
			expectErr: false, expectRFField: true, expectUserField: true,
		},

		// --- xAI (JSONMode=true, UserID=true, OpenAI adapter) ---
		{
			providerName: "xai", responseFormat: "", userID: "",
			expectErr: false, expectRFField: false, expectUserField: false,
		},
		{
			providerName: "xai", responseFormat: "", userID: "u-test-123",
			expectErr: false, expectRFField: false, expectUserField: true,
		},
		{
			providerName: "xai", responseFormat: "json", userID: "",
			expectErr: false, expectRFField: true, expectUserField: false,
		},
		{
			providerName: "xai", responseFormat: "json", userID: "u-test-123",
			expectErr: false, expectRFField: true, expectUserField: true,
		},

		// --- DeepSeek (JSONMode=true, UserID=false) ---
		{
			providerName: "deepseek", responseFormat: "", userID: "",
			expectErr: false, expectRFField: false, expectUserField: false,
		},
		{
			providerName: "deepseek", responseFormat: "", userID: "u-test-123",
			expectErr: false, expectRFField: false, expectUserField: false, // silent drop
		},
		{
			providerName: "deepseek", responseFormat: "json", userID: "",
			expectErr: false, expectRFField: true, expectUserField: false,
		},
		{
			providerName: "deepseek", responseFormat: "json", userID: "u-test-123",
			expectErr: false, expectRFField: true, expectUserField: false, // silent drop
		},

		// --- Google (JSONMode=true, UserID=false) —- body not captured via HTTP (fake client) ---
		// Google cases just verify no error and capability declarations; body checked in T-004.
		{
			providerName: "google", responseFormat: "", userID: "",
			expectErr: false,
		},
		{
			providerName: "google", responseFormat: "", userID: "u-test-123",
			expectErr: false,
		},
		{
			providerName: "google", responseFormat: "json", userID: "",
			expectErr: false,
		},
		{
			providerName: "google", responseFormat: "json", userID: "u-test-123",
			expectErr: false, // UserID dropped silently, JSON mode allowed
		},

		// --- Ollama (JSONMode=true, UserID=false) ---
		{
			providerName: "ollama", responseFormat: "", userID: "",
			expectErr: false, expectRFField: false, expectUserField: false,
		},
		{
			providerName: "ollama", responseFormat: "", userID: "u-test-123",
			expectErr: false, expectRFField: false, expectUserField: false, // silent drop
		},
		{
			providerName: "ollama", responseFormat: "json", userID: "",
			expectErr: false, expectRFField: true, expectUserField: false,
		},
		{
			providerName: "ollama", responseFormat: "json", userID: "u-test-123",
			expectErr: false, expectRFField: true, expectUserField: false, // silent drop
		},
	}

	for _, tc := range cases {
		tc := tc
		name := tc.providerName + "/rf=" + tc.responseFormat + "/uid=" + tc.userID
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runMatrixCase(t, tc)
		})
	}
}

func runMatrixCase(t *testing.T, tc matrixCase) {
	t.Helper()

	var (
		p    provider.Provider
		cap  *captureHTTPBody
		stop func()
	)

	// Build provider based on name
	switch tc.providerName {
	case "anthropic":
		pool, store := makePoolAndStore(t, "anthropic")
		cap = &captureHTTPBody{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			cap.captured = make(map[string]json.RawMessage)
			_ = json.Unmarshal(body, &cap.captured)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, anthropicMinimalSSE())
		}))
		stop = srv.Close
		a, err := anthropic.New(anthropic.AnthropicOptions{
			Pool:          pool,
			SecretStore:   store,
			APIEndpoint:   srv.URL,
			Tracker:       ratelimit.NewTracker(),
			CachePlanner:  &cache.BreakpointPlanner{},
			CacheStrategy: cache.StrategyNone,
			CacheTTL:      cache.TTLEphemeral,
		})
		require.NoError(t, err)
		p = a

	case "openai":
		caps := provider.Capabilities{Streaming: true, Tools: true, Vision: true, JSONMode: true, UserID: true}
		p, cap, stop = makeOpenAICompatProvider(t, "openai", caps)

	case "xai":
		p, cap, stop = makeOpenAICompatProvider(t, "xai", provider.Capabilities{})

	case "deepseek":
		p, cap, stop = makeOpenAICompatProvider(t, "deepseek", provider.Capabilities{})

	case "google":
		fakeG := &fakeGoogleCapture{}
		a, err := google.New(google.GoogleOptions{
			ClientFactory: func(_ string) google.GeminiClientIface { return fakeG },
		})
		require.NoError(t, err)
		p = a
		cap = nil // no HTTP capture for Google
		stop = func() {}

	case "ollama":
		cap = &captureHTTPBody{}
		srv := httptest.NewServer(cap.handler(kindJSONL))
		stop = srv.Close
		a, err := ollama.New(ollama.OllamaOptions{Endpoint: srv.URL})
		require.NoError(t, err)
		p = a
	}

	t.Cleanup(stop)

	fn := matrixLLMCallFunc(t, p)

	req := query.LLMCallReq{
		Route:          router.Route{Model: "test-model", Provider: tc.providerName},
		Messages:       []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		ResponseFormat: tc.responseFormat,
		Metadata:       query.RequestMetadata{UserID: tc.userID},
	}

	ch, err := fn(context.Background(), req)

	if tc.expectErr {
		require.Error(t, err)
		var capErr provider.ErrCapabilityUnsupported
		require.ErrorAs(t, err, &capErr)
		assert.Equal(t, tc.expectErrFeature, capErr.Feature)
		return
	}

	require.NoError(t, err)
	for range ch {
	}

	if cap == nil || cap.captured == nil {
		// Google uses fake client — body not captured via HTTP
		return
	}

	// Verify response_format / format field presence
	if tc.providerName == "ollama" {
		_, hasFmt := cap.captured["format"]
		if tc.expectRFField {
			assert.True(t, hasFmt, "ollama: format field must be present for json mode")
		} else {
			assert.False(t, hasFmt, "ollama: format field must be absent when ResponseFormat is empty")
		}
	} else {
		_, hasRF := cap.captured["response_format"]
		if tc.expectRFField {
			assert.True(t, hasRF, "%s: response_format must be present for json mode", tc.providerName)
		} else {
			assert.False(t, hasRF, "%s: response_format must be absent when ResponseFormat is empty", tc.providerName)
		}
	}

	// Verify user / metadata field presence
	if tc.providerName == "anthropic" {
		_, hasMeta := cap.captured["metadata"]
		if tc.expectUserField {
			assert.True(t, hasMeta, "anthropic: metadata must be present when UserID is set")
		} else {
			assert.False(t, hasMeta, "anthropic: metadata must be absent when UserID is empty")
		}
	} else {
		_, hasUser := cap.captured["user"]
		if tc.expectUserField {
			assert.True(t, hasUser, "%s: user field must be present when UserID is set", tc.providerName)
		} else {
			assert.False(t, hasUser, "%s: user field must be absent or dropped", tc.providerName)
		}
	}
}

// anthropicMinimalSSE returns a minimal Anthropic SSE response for matrix tests.
func anthropicMinimalSSE() string {
	return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-opus-4-7\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
}

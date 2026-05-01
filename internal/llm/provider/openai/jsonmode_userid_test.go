package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedRequest holds the parsed JSON body from the httptest server.
type capturedRequest map[string]json.RawMessage

func makeOpenAITestAdapter(t *testing.T, serverURL string) *openai.OpenAIAdapter {
	t.Helper()

	creds := []*credential.PooledCredential{
		{ID: "c1", Provider: "openai", KeyringID: "k1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)

	store := provider.NewMemorySecretStore(map[string]string{"k1": "test-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     serverURL,
		Pool:        pool,
		Tracker:     ratelimit.NewTracker(),
		SecretStore: store,
		Capabilities: provider.Capabilities{
			Streaming: true,
			Tools:     true,
			Vision:    true,
			JSONMode:  true,
			UserID:    true,
		},
	})
	require.NoError(t, err)
	return adapter
}

// minimalSSEResponse returns a minimal OpenAI-style SSE body that the adapter can parse.
func minimalSSEResponse() string {
	return "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
}

// captureBody creates an httptest server that captures the request body, then returns the captured map.
func captureBody(t *testing.T) (*httptest.Server, *capturedRequest) {
	t.Helper()
	var captured capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		if jsonErr := json.Unmarshal(body, &captured); jsonErr != nil {
			http.Error(w, "json error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minimalSSEResponse())
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func drainStream(t *testing.T, ch <-chan message.StreamEvent) {
	t.Helper()
	for range ch {
	}
}

// TestOpenAI_JSONMode verifies that ResponseFormat=="json" injects response_format into the body.
// AC-AMEND-003.
func TestOpenAI_JSONMode(t *testing.T) {
	t.Parallel()

	srv, captured := captureBody(t)
	adapter := makeOpenAITestAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:          router.Route{Model: "gpt-4o", Provider: "openai"},
		Messages:       []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		ResponseFormat: "json",
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	drainStream(t, ch)

	require.NotNil(t, captured, "server must have received a request")
	rfRaw, ok := (*captured)["response_format"]
	require.True(t, ok, "response_format field must be present in request body")

	var rf map[string]string
	require.NoError(t, json.Unmarshal(rfRaw, &rf))
	assert.Equal(t, "json_object", rf["type"], "response_format.type must be json_object")
}

// TestOpenAI_UserID verifies that Metadata.UserID is forwarded as top-level "user" field.
// AC-AMEND-006.
func TestOpenAI_UserID(t *testing.T) {
	t.Parallel()

	srv, captured := captureBody(t)
	adapter := makeOpenAITestAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:    router.Route{Model: "gpt-4o", Provider: "openai"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		Metadata: provider.RequestMetadata{UserID: "u-abc-123"},
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	drainStream(t, ch)

	require.NotNil(t, captured)
	userRaw, ok := (*captured)["user"]
	require.True(t, ok, "user field must be present in request body when UserID is set")

	var userVal string
	require.NoError(t, json.Unmarshal(userRaw, &userVal))
	assert.Equal(t, "u-abc-123", userVal)
}

// TestOpenAI_ZeroValueByteIdentical verifies that empty ResponseFormat and UserID produce
// no additional fields in the serialized request (omitempty regression guard).
// AC-AMEND-009 (OpenAI row).
func TestOpenAI_ZeroValueByteIdentical(t *testing.T) {
	t.Parallel()

	srv, captured := captureBody(t)
	adapter := makeOpenAITestAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:    router.Route{Model: "gpt-4o", Provider: "openai"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
		// ResponseFormat and Metadata.UserID left at zero value
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	drainStream(t, ch)

	require.NotNil(t, captured)
	_, hasRF := (*captured)["response_format"]
	assert.False(t, hasRF, "response_format must NOT appear when ResponseFormat is empty")
	_, hasUser := (*captured)["user"]
	assert.False(t, hasUser, "user must NOT appear when UserID is empty")
}

package anthropic_test

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
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalAnthropicSSE returns a minimal Anthropic-style SSE response.
func minimalAnthropicSSE() string {
	return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-opus-4-7\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
}

func makeAnthropicAdapter(t *testing.T, serverURL string) *anthropic.AnthropicAdapter {
	t.Helper()

	creds := []*credential.PooledCredential{
		{ID: "c1", Provider: "anthropic", KeyringID: "k1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)

	store := provider.NewMemorySecretStore(map[string]string{"k1": "test-key"})

	adapter, err := anthropic.New(anthropic.AnthropicOptions{
		Pool:          pool,
		Tracker:       ratelimit.NewTracker(),
		CachePlanner:  &cache.BreakpointPlanner{},
		CacheStrategy: cache.StrategyNone,
		CacheTTL:      cache.TTLEphemeral,
		SecretStore:   store,
		APIEndpoint:   serverURL,
	})
	require.NoError(t, err)
	return adapter
}

func drainAnthropicStream(t *testing.T, ch <-chan message.StreamEvent) {
	t.Helper()
	for range ch {
	}
}

// TestAnthropic_UserIDNested verifies that Metadata.UserID is serialized as nested metadata.user_id.
// AC-AMEND-007.
func TestAnthropic_UserIDNested(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedBody))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minimalAnthropicSSE())
	}))
	t.Cleanup(srv.Close)

	adapter := makeAnthropicAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:           router.Route{Model: "claude-opus-4-7", Provider: "anthropic"},
		Messages:        []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		MaxOutputTokens: 100,
		Metadata:        provider.RequestMetadata{UserID: "u-xyz-789"},
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	drainAnthropicStream(t, ch)

	require.NotNil(t, capturedBody)
	metaRaw, ok := capturedBody["metadata"]
	require.True(t, ok, "metadata field must be present when UserID is set")

	var meta map[string]string
	require.NoError(t, json.Unmarshal(metaRaw, &meta))
	assert.Equal(t, "u-xyz-789", meta["user_id"], "metadata.user_id must match the provided UserID")
}

// TestAnthropic_ZeroValueByteIdentical verifies that empty UserID produces no metadata field.
// AC-AMEND-009 (anthropic row).
func TestAnthropic_ZeroValueByteIdentical(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedBody))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minimalAnthropicSSE())
	}))
	t.Cleanup(srv.Close)

	adapter := makeAnthropicAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:           router.Route{Model: "claude-opus-4-7", Provider: "anthropic"},
		Messages:        []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		MaxOutputTokens: 100,
		// Metadata.UserID left empty
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	drainAnthropicStream(t, ch)

	require.NotNil(t, capturedBody)
	_, hasMeta := capturedBody["metadata"]
	assert.False(t, hasMeta, "metadata must NOT appear when UserID is empty (omitempty)")
}

// TestAnthropic_Capabilities_JSONModeAndUserID verifies capability declarations for Anthropic.
// AC-AMEND-001 (anthropic row).
func TestAnthropic_Capabilities_JSONModeAndUserID(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "c1", Provider: "anthropic", KeyringID: "k1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)

	store := provider.NewMemorySecretStore(map[string]string{"k1": "test-key"})
	adapter, err := anthropic.New(anthropic.AnthropicOptions{
		Pool:        pool,
		SecretStore: store,
	})
	require.NoError(t, err)

	caps := adapter.Capabilities()
	assert.False(t, caps.JSONMode, "Anthropic must report JSONMode=false (unsupported)")
	assert.True(t, caps.UserID, "Anthropic must report UserID=true (supported via metadata.user_id)")
}

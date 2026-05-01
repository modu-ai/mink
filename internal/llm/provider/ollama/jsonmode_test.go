package ollama_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/ollama"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalOllamaJSONLResponse returns a minimal Ollama JSON-L response.
func minimalOllamaJSONLResponse() string {
	return `{"message":{"role":"assistant","content":"ok"},"done":false}` + "\n" +
		`{"message":{"role":"assistant","content":""},"done":true}` + "\n"
}

func makeOllamaTestAdapter(t *testing.T, serverURL string) *ollama.OllamaAdapter {
	t.Helper()
	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint: serverURL,
	})
	require.NoError(t, err)
	return adapter
}

// TestOllama_JSONMode verifies that ResponseFormat=="json" produces "format":"json" in request body.
// AC-AMEND-005.
func TestOllama_JSONMode(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedBody))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minimalOllamaJSONLResponse())
	}))
	t.Cleanup(srv.Close)

	adapter := makeOllamaTestAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:          router.Route{Model: "llama3", Provider: "ollama"},
		Messages:       []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		ResponseFormat: "json",
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	for range ch {
	}

	require.NotNil(t, capturedBody)
	fmtRaw, ok := capturedBody["format"]
	require.True(t, ok, "format field must be present when ResponseFormat is json")

	var fmtVal string
	require.NoError(t, json.Unmarshal(fmtRaw, &fmtVal))
	assert.Equal(t, "json", fmtVal, "format must be 'json'")
}

// TestOllama_ZeroValueByteIdentical verifies that empty ResponseFormat omits the format field.
// AC-AMEND-009 (ollama row).
func TestOllama_ZeroValueByteIdentical(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedBody))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minimalOllamaJSONLResponse())
	}))
	t.Cleanup(srv.Close)

	adapter := makeOllamaTestAdapter(t, srv.URL)

	req := provider.CompletionRequest{
		Route:    router.Route{Model: "llama3", Provider: "ollama"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		// ResponseFormat empty
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	for range ch {
	}

	require.NotNil(t, capturedBody)
	_, hasFormat := capturedBody["format"]
	assert.False(t, hasFormat, "format must NOT appear when ResponseFormat is empty")
}

// TestOllama_Capabilities verifies JSONMode=true, UserID=false for Ollama adapter.
// AC-AMEND-001 (ollama row).
func TestOllama_Capabilities(t *testing.T) {
	t.Parallel()

	adapter, err := ollama.New(ollama.OllamaOptions{Endpoint: "http://localhost:11434"})
	require.NoError(t, err)

	caps := adapter.Capabilities()
	assert.True(t, caps.JSONMode, "Ollama must report JSONMode=true")
	assert.False(t, caps.UserID, "Ollama must report UserID=false")
}

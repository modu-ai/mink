package google_test

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/google"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClientCapturingJSONMode captures the GeminiRequest to allow assertion on ResponseFormat.
type fakeClientCapturingJSONMode struct {
	captured *google.GeminiRequest
}

func (f *fakeClientCapturingJSONMode) GenerateStream(_ context.Context, req google.GeminiRequest) (google.GeminiStream, error) {
	f.captured = &req
	return &fakeStream{chunks: []google.FakeChunk{
		{Text: "ok"},
		{IsDone: true},
	}}, nil
}

// fakeStream is a minimal GeminiStream for testing.
type fakeStream struct {
	chunks []google.FakeChunk
	idx    int
}

func (s *fakeStream) Next() (*google.GeminiChunk, error) {
	if s.idx >= len(s.chunks) {
		return nil, google.ErrStreamDone
	}
	fc := s.chunks[s.idx]
	s.idx++
	return &google.GeminiChunk{
		Text:     fc.Text,
		IsDone:   fc.IsDone,
		HasTool:  fc.HasTool,
		ToolName: fc.ToolName,
		ToolArgs: fc.ToolArgs,
	}, nil
}

func (s *fakeStream) Close() {}

// TestGemini_JSONMode_FakeClientCapture verifies that ResponseFormat=="json" sets the field
// on GeminiRequest so that the real client path can inject ResponseMIMEType.
// AC-AMEND-004.
func TestGemini_JSONMode_FakeClientCapture(t *testing.T) {
	t.Parallel()

	fake := &fakeClientCapturingJSONMode{}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface { return fake },
	})
	require.NoError(t, err)

	req := provider.CompletionRequest{
		Route:          router.Route{Model: "gemini-2.0-flash", Provider: "google"},
		Messages:       []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		ResponseFormat: "json",
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	for range ch {
	}

	require.NotNil(t, fake.captured, "fake client must have been called")
	assert.Equal(t, "json", fake.captured.ResponseFormat,
		"GeminiRequest.ResponseFormat must be set to 'json' when req.ResponseFormat=='json'")
}

// TestGemini_ZeroValueByteIdentical verifies that empty ResponseFormat leaves GeminiRequest.ResponseFormat empty.
// AC-AMEND-009 (google row).
func TestGemini_ZeroValueByteIdentical(t *testing.T) {
	t.Parallel()

	fake := &fakeClientCapturingJSONMode{}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface { return fake },
	})
	require.NoError(t, err)

	req := provider.CompletionRequest{
		Route:    router.Route{Model: "gemini-2.0-flash", Provider: "google"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hi"}}}},
		// ResponseFormat empty
	}

	ch, err := adapter.Stream(context.Background(), req)
	require.NoError(t, err)
	for range ch {
	}

	require.NotNil(t, fake.captured)
	assert.Equal(t, "", fake.captured.ResponseFormat,
		"GeminiRequest.ResponseFormat must be empty when req.ResponseFormat is not set")
}

// TestGemini_Capabilities verifies JSONMode=true, UserID=false for Google adapter.
// AC-AMEND-001 (google row).
func TestGemini_Capabilities(t *testing.T) {
	t.Parallel()

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return &fakeClientCapturingJSONMode{}
		},
	})
	require.NoError(t, err)

	caps := adapter.Capabilities()
	assert.True(t, caps.JSONMode, "Google must report JSONMode=true")
	assert.False(t, caps.UserID, "Google must report UserID=false (silent drop)")
}

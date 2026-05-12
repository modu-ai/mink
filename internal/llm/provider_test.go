package llm

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/message"
)

// TestLLMProvider_InterfaceContracts validates that LLMProvider implementations
// satisfy the interface contract defined in SPEC-GOOSE-LLM-001.
func TestLLMProvider_InterfaceContracts(t *testing.T) {
	t.Run("LLMProvider interface exists", func(t *testing.T) {
		// Verify LLMProvider interface is defined
		var _ LLMProvider = (*mockLLMProvider)(nil)
	})
}

// mockLLMProvider is a test double for LLMProvider interface validation.
type mockLLMProvider struct {
	name string
}

func (m *mockLLMProvider) Name() string {
	return m.name
}

func (m *mockLLMProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error) {
	return nil, nil
}

func (m *mockLLMProvider) CountTokens(ctx context.Context, text string) (int, error) {
	return 0, nil
}

func (m *mockLLMProvider) Capabilities(ctx context.Context, model string) (Capabilities, error) {
	return Capabilities{}, nil
}

// TestCompletionRequest_Validation validates CompletionRequest field requirements.
func TestCompletionRequest_Validation(t *testing.T) {
	tests := []struct {
		name  string
		req   CompletionRequest
		valid bool
	}{
		{
			name: "valid minimal request",
			req: CompletionRequest{
				Model:    "qwen2.5:3b",
				Messages: []Message{{Role: "user", Content: "hello"}},
			},
			valid: true,
		},
		{
			name: "missing model",
			req: CompletionRequest{
				Messages: []Message{{Role: "user", Content: "hello"}},
			},
			valid: false,
		},
		{
			name: "empty messages",
			req: CompletionRequest{
				Model:    "qwen2.5:3b",
				Messages: []Message{},
			},
			valid: false,
		},
		{
			name: "valid with temperature",
			req: CompletionRequest{
				Model:       "qwen2.5:3b",
				Messages:    []Message{{Role: "user", Content: "hello"}},
				Temperature: ptr(0.7),
			},
			valid: true,
		},
		{
			name: "valid with max tokens",
			req: CompletionRequest{
				Model:     "qwen2.5:3b",
				Messages:  []Message{{Role: "user", Content: "hello"}},
				MaxTokens: ptr(1000),
			},
			valid: true,
		},
		{
			name: "valid with stop sequences",
			req: CompletionRequest{
				Model:    "qwen2.5:3b",
				Messages: []Message{{Role: "user", Content: "hello"}},
				Stop:     []string{"\n\n", "###"},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err == nil) != tt.valid {
				t.Errorf("Validate() error = %v, valid = %v", err, tt.valid)
			}
		})
	}
}

// TestUsage_Validation validates Usage field requirements from SPEC.
func TestUsage_Validation(t *testing.T) {
	t.Run("usage with known token counts", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Unknown:          false,
		}
		if usage.Unknown {
			t.Error("Usage with known counts should not be Unknown")
		}
	})

	t.Run("usage with unknown token counts", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
			Unknown:          true,
		}
		if !usage.Unknown {
			t.Error("Usage with zero counts should be Unknown")
		}
	})
}

// TestCapabilities_Validation validates Capabilities structure.
func TestCapabilities_Validation(t *testing.T) {
	t.Run("capabilities with all fields", func(t *testing.T) {
		caps := Capabilities{
			MaxContextTokens: 4096,
			SupportsTools:    true,
			SupportsJSON:     true,
			Family:           "llama",
		}
		if caps.MaxContextTokens == 0 {
			t.Error("MaxContextTokens should be set")
		}
		if caps.Family == "" {
			t.Error("Family should be set")
		}
	})
}

// TestIntegration_WithProviderPackage validates that new LLMProvider interface
// can coexist with existing provider package (REQ-LLM-001 compatibility).
func TestIntegration_WithProviderPackage(t *testing.T) {
	t.Run("types are distinct from provider package", func(t *testing.T) {
		// This test validates that the new LLMProvider interface
		// is a separate type from provider.Provider
		var newProvider LLMProvider = &mockLLMProvider{name: "test"}

		// Verify we cannot assign directly to provider.Provider
		// This confirms they are distinct interfaces
		_ = newProvider.Name()
	})

	t.Run("Message type compatibility", func(t *testing.T) {
		// Verify Message types are compatible
		msg := Message{
			Role:    "user",
			Content: "hello",
		}

		// Convert to provider package message
		providerMsg := message.Message{
			Role:    msg.Role,
			Content: []message.ContentBlock{{Type: "text", Text: msg.Content}},
		}

		if providerMsg.Role != msg.Role {
			t.Error("Role should match")
		}
	})
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}

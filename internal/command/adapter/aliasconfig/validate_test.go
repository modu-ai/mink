package aliasconfig

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/router"
)

// TestValidate verifies strict and lenient validation modes.
// REQ-ALIAS-004 (empty map), REQ-ALIAS-023 (strict mode), REQ-ALIAS-024 (lenient mode).
func TestValidate(t *testing.T) {
	// Create a test registry with known providers and models
	// Use actual model names from the real registry
	registry := router.NewRegistry()

	// Register test providers with required fields
	anthropicMeta := &router.ProviderMeta{
		Name:            "anthropic",
		DisplayName:     "Anthropic",
		DefaultBaseURL:  "https://api.anthropic.com",
		AuthType:        "api_key",
		SuggestedModels: []string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4"},
	}
	openaiMeta := &router.ProviderMeta{
		Name:            "openai",
		DisplayName:     "OpenAI",
		DefaultBaseURL:  "https://api.openai.com/v1",
		AuthType:        "api_key",
		SuggestedModels: []string{"gpt-4o", "gpt-4o-mini", "o1-preview"},
	}

	if err := registry.Register(anthropicMeta); err != nil {
		t.Fatalf("Failed to register anthropic provider: %v", err)
	}
	if err := registry.Register(openaiMeta); err != nil {
		t.Fatalf("Failed to register openai provider: %v", err)
	}

	// Verify registration
	if _, ok := registry.Get("anthropic"); !ok {
		t.Fatal("Failed to find anthropic provider after registration")
	}
	if _, ok := registry.Get("openai"); !ok {
		t.Fatal("Failed to find openai provider after registration")
	}

	t.Run("empty map returns empty error slice", func(t *testing.T) {
		// REQ-ALIAS-004
		errs := Validate(map[string]string{}, registry, false)
		if len(errs) != 0 {
			t.Errorf("Validate() on empty map should return empty error slice, got %d errors", len(errs))
		}
	})

	t.Run("strict mode with unknown provider returns error", func(t *testing.T) {
		// REQ-ALIAS-023, REQ-ALIAS-034
		aliases := map[string]string{
			"unknown": "unknown-provider/unknown-model",
		}
		errs := Validate(aliases, registry, true)

		if len(errs) != 1 {
			t.Errorf("Validate() strict mode with unknown provider should return 1 error, got %d", len(errs))
		}
	})

	t.Run("strict mode with unknown model returns error", func(t *testing.T) {
		// REQ-ALIAS-023, REQ-ALIAS-035
		aliases := map[string]string{
			"bad": "anthropic/unknown-model",
		}
		errs := Validate(aliases, registry, true)

		if len(errs) != 1 {
			t.Errorf("Validate() strict mode with unknown model should return 1 error, got %d", len(errs))
		}
	})

	t.Run("lenient mode removes invalid entries and returns warnings", func(t *testing.T) {
		// REQ-ALIAS-024
		aliases := map[string]string{
			"good1":    "anthropic/claude-opus-4-7",
			"good2":    "openai/gpt-4o",
			"bad":      "unknown-provider/unknown-model",
			"badmodel": "anthropic/unknown-model",
		}
		errs := Validate(aliases, registry, false)

		// In lenient mode, invalid entries should be removed from the map
		if len(aliases) != 2 {
			t.Errorf("Validate() lenient mode should remove invalid entries, got %d entries (want 2)", len(aliases))
		}
		if _, ok := aliases["good1"]; !ok {
			t.Error("Validate() lenient mode should keep valid entry 'good1'")
		}
		if _, ok := aliases["bad"]; ok {
			t.Error("Validate() lenient mode should remove invalid entry 'bad'")
		}
		// Should return warning errors for removed entries
		if len(errs) != 2 {
			t.Errorf("Validate() lenient mode should return 2 warnings, got %d", len(errs))
		}
	})

	t.Run("valid map returns no errors", func(t *testing.T) {
		aliases := map[string]string{
			"opus":   "anthropic/claude-opus-4-7",
			"sonnet": "anthropic/claude-sonnet-4-6",
			"gpt4o":  "openai/gpt-4o",
		}
		errs := Validate(aliases, registry, true)

		if len(errs) != 0 {
			t.Errorf("Validate() on valid map should return no errors, got %d", len(errs))
		}
	})
}

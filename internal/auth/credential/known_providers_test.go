package credential_test

import (
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

func TestKnownProviders_Coverage(t *testing.T) {
	// Verify all 8 expected providers are registered.
	expectedProviders := []string{
		"anthropic", "deepseek", "openai_gpt", "zai_glm",
		"codex", "telegram_bot", "slack", "discord",
	}
	for _, p := range expectedProviders {
		if _, ok := credential.KnownProviders[p]; !ok {
			t.Errorf("provider %q missing from KnownProviders", p)
		}
	}
}

func TestKnownProviders_KindMapping(t *testing.T) {
	tests := []struct {
		provider string
		want     credential.Kind
	}{
		{"anthropic", credential.KindAPIKey},
		{"deepseek", credential.KindAPIKey},
		{"openai_gpt", credential.KindAPIKey},
		{"zai_glm", credential.KindAPIKey},
		{"codex", credential.KindOAuth},
		{"telegram_bot", credential.KindBotToken},
		{"slack", credential.KindSlackCombo},
		{"discord", credential.KindDiscordCombo},
	}
	for _, tc := range tests {
		got, ok := credential.KnownProviders[tc.provider]
		if !ok {
			t.Errorf("provider %q not found in KnownProviders", tc.provider)
			continue
		}
		if got != tc.want {
			t.Errorf("provider %q: want Kind %q, got %q", tc.provider, tc.want, got)
		}
	}
}

func TestValidateProviderKindCombo_Match(t *testing.T) {
	// APIKey for anthropic
	if err := credential.ValidateProviderKindCombo("anthropic", credential.APIKey{Value: "v"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// OAuthToken for codex
	tok := credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "a",
		RefreshToken: "r",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := credential.ValidateProviderKindCombo("codex", tok); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BotToken for telegram_bot
	if err := credential.ValidateProviderKindCombo("telegram_bot", credential.BotToken{Provider: "telegram_bot", Token: "t"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SlackCombo for slack
	if err := credential.ValidateProviderKindCombo("slack", credential.SlackCombo{SigningSecret: "s", BotToken: "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DiscordCombo for discord
	dc := credential.DiscordCombo{PublicKey: validPublicKey, BotToken: "d"}
	if err := credential.ValidateProviderKindCombo("discord", dc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProviderKindCombo_UnknownProvider(t *testing.T) {
	err := credential.ValidateProviderKindCombo("unknown-provider", credential.APIKey{Value: "v"})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
}

func TestValidateProviderKindCombo_KindMismatch(t *testing.T) {
	// anthropic expects KindAPIKey but we give KindBotToken
	err := credential.ValidateProviderKindCombo("anthropic", credential.BotToken{Provider: "anthropic", Token: "t"})
	if err == nil {
		t.Fatal("expected error for kind mismatch, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
}

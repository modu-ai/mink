package credential_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// validPublicKey is a 64-char lowercase hex string for test purposes.
const validPublicKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestDiscordCombo_Kind(t *testing.T) {
	dc := credential.DiscordCombo{
		PublicKey: validPublicKey,
		BotToken:  "discordbot-test",
	}
	if dc.Kind() != credential.KindDiscordCombo {
		t.Fatalf("want KindDiscordCombo, got %q", dc.Kind())
	}
}

func TestDiscordCombo_Validate_HappyPath(t *testing.T) {
	dc := credential.DiscordCombo{
		PublicKey: validPublicKey,
		BotToken:  "discordbot-abcdef",
	}
	if err := dc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscordCombo_Validate_HappyPath_WithOptionalAppID(t *testing.T) {
	dc := credential.DiscordCombo{
		PublicKey: validPublicKey,
		BotToken:  "discordbot-abcdef",
		AppID:     "123456789012345678",
	}
	if err := dc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscordCombo_Validate_MissingPublicKey(t *testing.T) {
	dc := credential.DiscordCombo{BotToken: "discordbot-abcdef"}
	err := dc.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "public_key") {
		t.Fatalf("error message should mention public_key, got %q", err.Error())
	}
}

func TestDiscordCombo_Validate_InvalidPublicKey_TooShort(t *testing.T) {
	dc := credential.DiscordCombo{
		PublicKey: "abc123",
		BotToken:  "discordbot-abcdef",
	}
	err := dc.Validate()
	if err == nil {
		t.Fatal("expected error for short public_key, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
}

func TestDiscordCombo_Validate_InvalidPublicKey_NonHex(t *testing.T) {
	// 64 chars but contains uppercase — not valid lowercase hex
	dc := credential.DiscordCombo{
		PublicKey: "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
		BotToken:  "discordbot-abcdef",
	}
	err := dc.Validate()
	if err == nil {
		t.Fatal("expected error for non-hex public_key, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
}

func TestDiscordCombo_Validate_MissingBotToken(t *testing.T) {
	dc := credential.DiscordCombo{PublicKey: validPublicKey}
	err := dc.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "bot_token") {
		t.Fatalf("error message should mention bot_token, got %q", err.Error())
	}
}

func TestDiscordCombo_MaskedString_DoesNotLeakPlaintext(t *testing.T) {
	botVal := "discordbot-1234-5678-realvalue"
	dc := credential.DiscordCombo{
		PublicKey: validPublicKey,
		BotToken:  botVal,
	}
	masked := dc.MaskedString()
	if strings.Contains(masked, botVal) {
		t.Fatalf("MaskedString leaked plaintext bot_token: %q", masked)
	}
	if strings.Contains(masked, validPublicKey) {
		t.Fatalf("MaskedString leaked plaintext public_key: %q", masked)
	}
	if !strings.HasPrefix(masked, "***") {
		t.Fatalf("MaskedString should start with ***, got %q", masked)
	}
}

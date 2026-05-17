package credential_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

func TestSlackCombo_Kind(t *testing.T) {
	sc := credential.SlackCombo{
		SigningSecret: "secret",
		BotToken:      "slackbot-test",
	}
	if sc.Kind() != credential.KindSlackCombo {
		t.Fatalf("want KindSlackCombo, got %q", sc.Kind())
	}
}

func TestSlackCombo_Validate_HappyPath(t *testing.T) {
	sc := credential.SlackCombo{
		SigningSecret: "abcdefghijklmnopqrstuvwx",
		BotToken:      "slackbot-abcdef",
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlackCombo_Validate_HappyPath_WithOptionalFields(t *testing.T) {
	sc := credential.SlackCombo{
		SigningSecret: "abcdefghijklmnopqrstuvwx",
		BotToken:      "slackbot-abcdef",
		AppID:         "A1234567890",
		TeamID:        "T0987654321",
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlackCombo_Validate_MissingSigningSecret(t *testing.T) {
	sc := credential.SlackCombo{BotToken: "slackbot-abcdef"}
	err := sc.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "signing_secret") {
		t.Fatalf("error message should mention signing_secret, got %q", err.Error())
	}
}

func TestSlackCombo_Validate_MissingBotToken(t *testing.T) {
	sc := credential.SlackCombo{SigningSecret: "abcdefghijklmnopqrstuvwx"}
	err := sc.Validate()
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

func TestSlackCombo_MaskedString_DoesNotLeakPlaintext(t *testing.T) {
	botVal := "slackbot-1234-5678-realvalue"
	sigVal := "reveal-nothing-here-ever-ok"
	sc := credential.SlackCombo{
		SigningSecret: sigVal,
		BotToken:      botVal,
	}
	masked := sc.MaskedString()
	if strings.Contains(masked, botVal) {
		t.Fatalf("MaskedString leaked plaintext bot_token: %q", masked)
	}
	if strings.Contains(masked, sigVal) {
		t.Fatalf("MaskedString leaked plaintext signing_secret: %q", masked)
	}
	if !strings.HasPrefix(masked, "***") {
		t.Fatalf("MaskedString should start with ***, got %q", masked)
	}
}

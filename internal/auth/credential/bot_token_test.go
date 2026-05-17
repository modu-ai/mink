package credential_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

func TestBotToken_Kind(t *testing.T) {
	bt := credential.BotToken{Provider: "telegram_bot", Token: "123:ABCDEF"}
	if bt.Kind() != credential.KindBotToken {
		t.Fatalf("want KindBotToken, got %q", bt.Kind())
	}
}

func TestBotToken_Validate_HappyPath(t *testing.T) {
	bt := credential.BotToken{Provider: "telegram_bot", Token: "123456:ABC-DEF-GHI"}
	if err := bt.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotToken_Validate_MissingProvider(t *testing.T) {
	bt := credential.BotToken{Token: "123456:ABC-DEF-GHI"}
	err := bt.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Fatalf("error message should mention provider, got %q", err.Error())
	}
}

func TestBotToken_Validate_MissingToken(t *testing.T) {
	bt := credential.BotToken{Provider: "telegram_bot"}
	err := bt.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "token") {
		t.Fatalf("error message should mention token, got %q", err.Error())
	}
}

func TestBotToken_MaskedString_DoesNotLeakPlaintext(t *testing.T) {
	token := "123456789:ABCDEF-secret-value"
	bt := credential.BotToken{Provider: "telegram_bot", Token: token}
	masked := bt.MaskedString()
	if strings.Contains(masked, token) {
		t.Fatalf("MaskedString leaked plaintext token: %q", masked)
	}
	if !strings.HasPrefix(masked, "***") {
		t.Fatalf("MaskedString should start with ***, got %q", masked)
	}
}

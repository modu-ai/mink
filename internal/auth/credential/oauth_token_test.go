package credential_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

func TestOAuthToken_Kind(t *testing.T) {
	tok := credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if tok.Kind() != credential.KindOAuth {
		t.Fatalf("want KindOAuth, got %q", tok.Kind())
	}
}

func TestOAuthToken_Validate_HappyPath(t *testing.T) {
	tok := credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "sess-abc123",
		RefreshToken: "rt-xyz789",
		ExpiresAt:    time.Now().Add(time.Hour),
		Scope:        "openid email profile offline_access",
	}
	if err := tok.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOAuthToken_Validate_MissingProvider(t *testing.T) {
	tok := credential.OAuthToken{
		AccessToken:  "sess-abc123",
		RefreshToken: "rt-xyz789",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	err := tok.Validate()
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

func TestOAuthToken_Validate_MissingAccessToken(t *testing.T) {
	tok := credential.OAuthToken{
		Provider:     "codex",
		RefreshToken: "rt-xyz789",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	err := tok.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("error message should mention access_token, got %q", err.Error())
	}
}

func TestOAuthToken_Validate_MissingRefreshToken(t *testing.T) {
	tok := credential.OAuthToken{
		Provider:    "codex",
		AccessToken: "sess-abc123",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	err := tok.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "refresh_token") {
		t.Fatalf("error message should mention refresh_token, got %q", err.Error())
	}
}

func TestOAuthToken_Validate_ZeroExpiresAt(t *testing.T) {
	tok := credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "sess-abc123",
		RefreshToken: "rt-xyz789",
		// ExpiresAt is zero value
	}
	err := tok.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("error message should mention expires_at, got %q", err.Error())
	}
}

func TestOAuthToken_MaskedString_DoesNotLeakPlaintext(t *testing.T) {
	accessToken := "sess-verylongaccesstoken12345"
	tok := credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  accessToken,
		RefreshToken: "rt-shouldnotappear",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	masked := tok.MaskedString()
	if strings.Contains(masked, accessToken) {
		t.Fatalf("MaskedString leaked plaintext access token: %q", masked)
	}
	// Verify last-4 suffix is present for readability
	if !strings.HasSuffix(masked, "2345") {
		t.Fatalf("MaskedString should end with last-4 chars, got %q", masked)
	}
	if !strings.HasPrefix(masked, "***") {
		t.Fatalf("MaskedString should start with ***, got %q", masked)
	}
	// Ensure refresh token is not leaked at all
	if strings.Contains(masked, "rt-") {
		t.Fatalf("MaskedString leaked refresh token prefix: %q", masked)
	}
}

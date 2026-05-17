package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// ---------------------------------------------------------------------------
// httpCodeExchanger tests (token endpoint mocked via httptest)
// ---------------------------------------------------------------------------

func makeMockExchanger(handler http.HandlerFunc) (CodeExchanger, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return &httpCodeExchanger{
		tokenURL: srv.URL,
		client:   &http.Client{Timeout: 5 * time.Second},
	}, srv
}

// TestCompleteAuthorization_HappyPath verifies the full authorization flow with
// a mock token server.
func TestCompleteAuthorization_HappyPath(t *testing.T) {
	// Build a mock token server that returns a valid token response.
	exchanger, srv := makeMockExchanger(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "authorization_code" {
			http.Error(w, "bad grant_type", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := tokenResponse{
			AccessToken:  "sess-access-1234",
			RefreshToken: "rt-refresh-5678",
			ExpiresIn:    3600,
			Scope:        "openid offline_access",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	ctx := context.Background()
	pending, err := BeginAuthorization(ctx)
	if err != nil {
		t.Fatalf("BeginAuthorization: %v", err)
	}

	// Simulate the browser callback by calling the local listener directly.
	callbackURL := fmt.Sprintf("http://%s/callback?code=authcode123&state=%s",
		pending.Listener.Addr().String(), url.QueryEscape(pending.State))

	// The local server starts when completeAuthorizationWithExchanger calls srv.Serve,
	// so we send the callback in a goroutine to avoid a deadlock.
	tokenCh := make(chan *credential.OAuthToken, 1)
	errCh := make(chan error, 1)
	go func() {
		tok, err := completeAuthorizationWithExchanger(ctx, pending, exchanger)
		if err != nil {
			errCh <- err
			return
		}
		tokenCh <- tok
	}()

	// Give the local server a moment to start listening, then send the callback.
	// Retry a few times to handle the small startup race.
	var callbackErr error
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(callbackURL) //nolint:noctx // test helper
		if err == nil {
			resp.Body.Close()
			callbackErr = nil
			break
		}
		callbackErr = err
	}
	if callbackErr != nil {
		t.Fatalf("failed to reach local callback server: %v", callbackErr)
	}

	select {
	case tok := <-tokenCh:
		if tok.AccessToken != "sess-access-1234" {
			t.Errorf("AccessToken: got %q, want %q", tok.AccessToken, "sess-access-1234")
		}
		if tok.RefreshToken != "rt-refresh-5678" {
			t.Errorf("RefreshToken: got %q, want %q", tok.RefreshToken, "rt-refresh-5678")
		}
		if tok.Provider != "codex" {
			t.Errorf("Provider: got %q, want codex", tok.Provider)
		}
		if tok.ExpiresAt.Before(time.Now().Add(50 * time.Minute)) {
			t.Errorf("ExpiresAt too soon: %v", tok.ExpiresAt)
		}
	case err := <-errCh:
		t.Fatalf("completeAuthorizationWithExchanger: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for authorization to complete")
	}
}

// TestCompleteAuthorization_StateMismatch verifies that a state mismatch is
// detected and returned as an error.
func TestCompleteAuthorization_StateMismatch(t *testing.T) {
	exchanger, srv := makeMockExchanger(func(w http.ResponseWriter, r *http.Request) {
		// Should not be reached in this test.
		http.Error(w, "unexpected call", http.StatusInternalServerError)
	})
	defer srv.Close()

	ctx := context.Background()
	pending, err := BeginAuthorization(ctx)
	if err != nil {
		t.Fatalf("BeginAuthorization: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := completeAuthorizationWithExchanger(ctx, pending, exchanger)
		errCh <- err
	}()

	// Send a callback with a wrong state value.
	callbackURL := fmt.Sprintf("http://%s/callback?code=authcode123&state=WRONG_STATE",
		pending.Listener.Addr().String())
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(callbackURL) //nolint:noctx // test helper
		if err == nil {
			resp.Body.Close()
			break
		}
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected state mismatch error, got nil")
		}
		if !strings.Contains(err.Error(), "state mismatch") {
			t.Errorf("error should mention state mismatch, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// TestCompleteAuthorization_MissingRefreshToken verifies that an empty
// refresh_token in the response is treated as an error.
func TestCompleteAuthorization_MissingRefreshToken(t *testing.T) {
	exchanger, srv := makeMockExchanger(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// access_token present but refresh_token missing.
		_, _ = w.Write([]byte(`{"access_token":"at-test","expires_in":3600}`))
	})
	defer srv.Close()

	ctx := context.Background()
	pending, err := BeginAuthorization(ctx)
	if err != nil {
		t.Fatalf("BeginAuthorization: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := completeAuthorizationWithExchanger(ctx, pending, exchanger)
		errCh <- err
	}()

	callbackURL := fmt.Sprintf("http://%s/callback?code=authcode123&state=%s",
		pending.Listener.Addr().String(), url.QueryEscape(pending.State))
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(callbackURL) //nolint:noctx // test helper
		if err == nil {
			resp.Body.Close()
			break
		}
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error for missing refresh_token, got nil")
		}
		if !strings.Contains(err.Error(), "refresh_token") {
			t.Errorf("error should mention refresh_token, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// TestCompleteAuthorization_MalformedJSON verifies that malformed JSON from
// the token endpoint is returned as an error.
func TestCompleteAuthorization_MalformedJSON(t *testing.T) {
	exchanger, srv := makeMockExchanger(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`this is not json`))
	})
	defer srv.Close()

	ctx := context.Background()
	pending, err := BeginAuthorization(ctx)
	if err != nil {
		t.Fatalf("BeginAuthorization: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := completeAuthorizationWithExchanger(ctx, pending, exchanger)
		errCh <- err
	}()

	callbackURL := fmt.Sprintf("http://%s/callback?code=authcode123&state=%s",
		pending.Listener.Addr().String(), url.QueryEscape(pending.State))
	for i := 0; i < 20; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(callbackURL) //nolint:noctx // test helper
		if err == nil {
			resp.Body.Close()
			break
		}
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// ---------------------------------------------------------------------------
// PKCE helpers unit tests
// ---------------------------------------------------------------------------

func TestGeneratePKCEVerifier_Length(t *testing.T) {
	v, err := generatePKCEVerifier()
	if err != nil {
		t.Fatalf("generatePKCEVerifier: %v", err)
	}
	// base64url of 43 bytes = 58 characters (no padding, RawURLEncoding)
	if len(v) < 43 {
		t.Errorf("PKCE verifier too short: len=%d", len(v))
	}
}

func TestComputePKCEChallenge_Deterministic(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// SHA256(verifier) in base64url (S256 per RFC 7636 §4.2).
	got := computePKCEChallenge(verifier)
	if got == "" {
		t.Fatal("computePKCEChallenge returned empty string")
	}
	// Run twice to confirm determinism.
	got2 := computePKCEChallenge(verifier)
	if got != got2 {
		t.Error("computePKCEChallenge is not deterministic")
	}
}

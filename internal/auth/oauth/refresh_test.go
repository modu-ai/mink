package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// ---------------------------------------------------------------------------
// mockStore is a thread-safe in-memory credential.Service for tests.
// ---------------------------------------------------------------------------

type mockStore struct {
	mu    sync.RWMutex
	creds map[string]credential.Credential
}

func newMockStore() *mockStore {
	return &mockStore{creds: make(map[string]credential.Credential)}
}

func (m *mockStore) Store(provider string, cred credential.Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.creds[provider] = cred
	return nil
}

func (m *mockStore) Load(provider string) (credential.Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.creds[provider]
	if !ok {
		return nil, credential.ErrNotFound
	}
	return c, nil
}

func (m *mockStore) Delete(provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.creds, provider)
	return nil
}

func (m *mockStore) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.creds))
	for id := range m.creds {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockStore) Health(provider string) (credential.HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.creds[provider]
	return credential.HealthStatus{Present: ok, Backend: "mock"}, nil
}

// ---------------------------------------------------------------------------
// Test: fresh token (>60s remaining) — no network call
// ---------------------------------------------------------------------------

func TestRefresh_FreshToken_NoNetworkCall(t *testing.T) {
	serverCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newMockStore()
	tok := &credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access-fresh",
		RefreshToken: "refresh-fresh",
		ExpiresAt:    time.Now().Add(2 * time.Minute), // > 60s
	}

	got, err := refreshWithClient(context.Background(), store, tok, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken != "access-fresh" {
		t.Errorf("expected unchanged token, got %q", got.AccessToken)
	}
	if serverCalled {
		t.Error("token endpoint should not be called for a fresh token")
	}
}

// ---------------------------------------------------------------------------
// Test: expired token + valid refresh → new access_token returned
// ---------------------------------------------------------------------------

func TestRefresh_Expired_ValidRefresh(t *testing.T) {
	newAccessToken := "access-new-1234"
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := refreshResponse{
			AccessToken:  newAccessToken,
			RefreshToken: "", // no rotation
			ExpiresIn:    3600,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	store := newMockStore()
	old := &credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-5 * time.Second), // expired
	}

	got, err := refreshWithClient(context.Background(), store, old, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken != newAccessToken {
		t.Errorf("got %q, want %q", got.AccessToken, newAccessToken)
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 call to token endpoint, got %d", callCount)
	}
	// Store should be called with the new token.
	stored, err := store.Load("codex")
	if err != nil {
		t.Fatalf("Load after refresh: %v", err)
	}
	storedTok, ok := stored.(credential.OAuthToken)
	if !ok {
		t.Fatalf("expected OAuthToken in store, got %T", stored)
	}
	if storedTok.AccessToken != newAccessToken {
		t.Errorf("stored token mismatch: got %q, want %q", storedTok.AccessToken, newAccessToken)
	}
	// Old refresh_token should be kept when server does not rotate.
	if storedTok.RefreshToken != "refresh-old" {
		t.Errorf("refresh_token should be retained: got %q, want %q", storedTok.RefreshToken, "refresh-old")
	}
}

// ---------------------------------------------------------------------------
// Test: expired token + refresh_token rotation
// ---------------------------------------------------------------------------

func TestRefresh_Expired_RotatedRefreshToken(t *testing.T) {
	newRefreshToken := "refresh-rotated-9999"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := refreshResponse{
			AccessToken:  "access-rotated",
			RefreshToken: newRefreshToken,
			ExpiresIn:    3600,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	store := newMockStore()
	old := &credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-5 * time.Second),
	}

	got, err := refreshWithClient(context.Background(), store, old, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RefreshToken != newRefreshToken {
		t.Errorf("rotated refresh_token: got %q, want %q", got.RefreshToken, newRefreshToken)
	}
	// Persisted token should also carry the rotated refresh_token.
	stored, _ := store.Load("codex")
	storedTok := stored.(credential.OAuthToken)
	if storedTok.RefreshToken != newRefreshToken {
		t.Errorf("stored refresh_token mismatch: got %q", storedTok.RefreshToken)
	}
}

// ---------------------------------------------------------------------------
// Test: expired + invalid_grant → ErrReAuthRequired
// ---------------------------------------------------------------------------

func TestRefresh_Expired_InvalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Token has been revoked"}`))
	}))
	defer srv.Close()

	store := newMockStore()
	old := &credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-5 * time.Second),
	}

	_, err := refreshWithClient(context.Background(), store, old, srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid_grant, got nil")
	}
	if !errors.Is(err, credential.ErrReAuthRequired) {
		t.Errorf("expected ErrReAuthRequired, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test: expired + 5xx → transient error (NOT ErrReAuthRequired)
// ---------------------------------------------------------------------------

func TestRefresh_Expired_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newMockStore()
	old := &credential.OAuthToken{
		Provider:     "codex",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-5 * time.Second),
	}

	_, err := refreshWithClient(context.Background(), store, old, srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 5xx, got nil")
	}
	if errors.Is(err, credential.ErrReAuthRequired) {
		t.Errorf("5xx should produce transient error, not ErrReAuthRequired")
	}
}

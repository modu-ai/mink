package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthFlow_Start는 Start 메서드가 PKCE와 state를 생성하는지 검증한다.
func TestAuthFlow_Start(t *testing.T) {
	// OAuth 인가 서버 (즉시 응답 없이 URL만 반환)
	flow := &AuthFlow{
		ClientID: "test-client",
		AuthURL:  "http://localhost/auth",
		TokenURL: "http://localhost/token",
		Scopes:   []string{"read", "write"},
	}

	// startCallbackServer를 호출하기 위한 실제 Start 테스트
	// 실제 브라우저 오픈은 openBrowser를 override해야 하므로
	// 직접 PKCE 생성 부분만 검증한다

	// generatePKCEVerifier 및 pkceChallenge 검증
	verifier, err := generatePKCEVerifier()
	require.NoError(t, err)
	assert.NotEmpty(t, verifier)

	challenge := pkceChallenge(verifier)
	assert.NotEmpty(t, challenge)

	// flow 필드 직접 설정하여 HandleCallback 경로 검증
	flow.verifier = verifier
	flow.state = "test-state-123"
	flow.callbackCh = make(chan callbackResult, 1)

	// HandleCallback 경로: token exchange가 필요하지만 URL이 없으므로 에러 예상
	_, err = flow.HandleCallback("code", "wrong-state")
	assert.ErrorIs(t, err, ErrOAuthStateMismatch)

	// 올바른 state: tokenURL이 없어서 에러 반환하지만 state 검증은 통과
	_, err = flow.HandleCallback("code", "test-state-123")
	assert.Error(t, err) // token exchange 실패
	assert.NotErrorIs(t, err, ErrOAuthStateMismatch)
}

// TestAuthFlow_StartCallbackServer는 startCallbackServer를 검증한다.
func TestAuthFlow_StartCallbackServer(t *testing.T) {
	flow := &AuthFlow{
		ClientID: "test",
		AuthURL:  "http://auth.example.com",
		TokenURL: "http://token.example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	flow.callbackCh = make(chan callbackResult, 1)

	port, err := startCallbackServer(ctx, flow)
	require.NoError(t, err)
	assert.Greater(t, port, 0)

	// 콜백 URL에 GET 요청
	callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=test-code&state=test-state", port)
	go func() {
		resp, err := http.Get(callbackURL)
		if err == nil {
			resp.Body.Close()
		}
	}()

	// 콜백 결과 수신
	select {
	case result := <-flow.callbackCh:
		assert.Equal(t, "test-code", result.code)
		assert.Equal(t, "test-state", result.state)
		assert.NoError(t, result.err)
	case <-ctx.Done():
		t.Fatal("callback not received")
	}
}

// TestAuthFlow_StartCallbackServer_OAuthError는 OAuth 에러 파라미터를 검증한다.
func TestAuthFlow_StartCallbackServer_OAuthError(t *testing.T) {
	flow := &AuthFlow{callbackCh: make(chan callbackResult, 1)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	port, err := startCallbackServer(ctx, flow)
	require.NoError(t, err)

	// 에러 파라미터로 콜백 호출
	errURL := fmt.Sprintf("http://localhost:%d/callback?error=access_denied", port)
	go func() {
		resp, err := http.Get(errURL)
		if err == nil {
			resp.Body.Close()
		}
	}()

	select {
	case result := <-flow.callbackCh:
		assert.Error(t, result.err)
		assert.Contains(t, result.err.Error(), "access_denied")
	case <-ctx.Done():
		t.Fatal("callback not received")
	}
}

// TestAuthFlow_ExchangeCode는 token exchange HTTP 요청을 검증한다.
func TestAuthFlow_ExchangeCode(t *testing.T) {
	// fixture token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		assert.Equal(t, "authorization_code", r.FormValue("grant_type"))
		assert.Equal(t, "test-code", r.FormValue("code"))
		assert.Equal(t, "test-verifier", r.FormValue("code_verifier"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
			"scope":         "read write",
		})
	}))
	defer tokenServer.Close()

	flow := &AuthFlow{
		ClientID:    "test-client",
		TokenURL:    tokenServer.URL,
		RedirectURI: "http://localhost:12345/callback",
		verifier:    "test-verifier",
		state:       "test-state",
	}

	ts, err := flow.exchangeCode("test-code")
	require.NoError(t, err)
	assert.Equal(t, "new-access", ts.AccessToken)
	assert.Equal(t, "new-refresh", ts.RefreshToken)
	assert.False(t, ts.ExpiresAt.IsZero())
}

// TestAuthFlow_ExchangeCode_InvalidGrant는 invalid_grant 에러를 검증한다.
func TestAuthFlow_ExchangeCode_InvalidGrant(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer tokenServer.Close()

	flow := &AuthFlow{
		ClientID: "test",
		TokenURL: tokenServer.URL,
		verifier: "verifier",
		state:    "state",
	}

	_, err := flow.exchangeCode("bad-code")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReauthRequired)
}

// TestAuthFlow_ExchangeCode_NoExpiry는 expires_in 없는 응답을 검증한다.
func TestAuthFlow_ExchangeCode_NoExpiry(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "tok",
			"refresh_token": "ref",
			// expires_in 없음
		})
	}))
	defer tokenServer.Close()

	flow := &AuthFlow{
		ClientID: "test",
		TokenURL: tokenServer.URL,
		verifier: "verifier",
	}

	ts, err := flow.exchangeCode("code")
	require.NoError(t, err)
	assert.True(t, ts.ExpiresAt.IsZero(), "expires_in 없으면 ExpiresAt은 zero")
}

// TestAuthFlow_WaitForCallback은 WaitForCallback을 검증한다.
func TestAuthFlow_WaitForCallback(t *testing.T) {
	// fixture token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok",
		})
	}))
	defer tokenServer.Close()

	flow := &AuthFlow{
		ClientID:   "test",
		TokenURL:   tokenServer.URL,
		verifier:   "test-verifier",
		state:      "correct-state",
		callbackCh: make(chan callbackResult, 1),
	}

	// 콜백을 채널에 직접 주입
	go func() {
		time.Sleep(10 * time.Millisecond)
		flow.callbackCh <- callbackResult{code: "code", state: "correct-state"}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ts, err := flow.WaitForCallback(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok", ts.AccessToken)
}

// TestAuthFlow_WaitForCallback_CtxTimeout는 ctx timeout 시 동작을 검증한다.
func TestAuthFlow_WaitForCallback_CtxTimeout(t *testing.T) {
	flow := &AuthFlow{callbackCh: make(chan callbackResult, 1)}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := flow.WaitForCallback(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestOpenBrowser는 openBrowser가 패닉 없이 동작하는지 검증한다.
func TestOpenBrowser(t *testing.T) {
	// openBrowser는 브라우저를 실제로 열지 않지만 패닉 없이 완료해야 한다
	// Windows에서는 noop이므로 모든 플랫폼에서 안전하게 호출 가능
	assert.NotPanics(t, func() {
		openBrowser("http://localhost:9999/test")
	})
}

// TestAuthFlow_HandleCallback_StateVerification은 state 검증을 검증한다.
func TestAuthFlow_HandleCallback_StateVerification(t *testing.T) {
	flow := &AuthFlow{
		state:    "valid-state-abc",
		verifier: "pkce-verifier",
		TokenURL: "http://localhost/token", // 실제 서버 없음 → 에러 예상
	}

	// 올바른 state
	_, err := flow.HandleCallback("any-code", "valid-state-abc")
	assert.Error(t, err)                             // token exchange 실패
	assert.NotErrorIs(t, err, ErrOAuthStateMismatch) // state는 일치함

	// 잘못된 state
	_, err = flow.HandleCallback("any-code", "wrong-state")
	assert.ErrorIs(t, err, ErrOAuthStateMismatch)
}

// TestRefreshToken_NoRefreshToken은 refresh token이 없는 응답을 검증한다.
// 서버가 새 refresh token을 발급하지 않으면 기존 것을 유지한다.
func TestRefreshToken_NoNewRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			// refresh_token 없음
		})
	}))
	defer srv.Close()

	ts, err := RefreshToken(srv.URL, "client", "original-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", ts.AccessToken)
	assert.Equal(t, "original-refresh", ts.RefreshToken, "새 refresh_token 없으면 기존 것 유지")
}

// TestRefreshToken_NetworkError는 네트워크 에러를 검증한다.
func TestRefreshToken_NetworkError(t *testing.T) {
	_, err := RefreshToken("http://localhost:99999/token", "client", "token")
	assert.Error(t, err)
}

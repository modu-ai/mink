package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// AuthFlow는 OAuth 2.1 + PKCE 인증 플로우를 관리한다.
// REQ-MCP-007: PKCE code_verifier + code_challenge 생성, 브라우저 콜백 수신
// REQ-MCP-017: state mismatch 거부
//
// @MX:ANCHOR: [AUTO] AuthFlow — OAuth 2.1 + PKCE 인증 플로우 관리자
// @MX:REASON: REQ-MCP-007, REQ-MCP-017 — OAuth flow의 단일 진입점. fan_in >= 3 (client, test, credentials)
type AuthFlow struct {
	ClientID    string
	AuthURL     string
	TokenURL    string
	RedirectURI string
	Scopes      []string

	verifier     string // PKCE code_verifier
	state        string // CSRF state
	callbackCh   chan callbackResult
	callbackOnce sync.Once
	server       *http.Server
	mu           sync.Mutex
}

type callbackResult struct {
	code  string
	state string
	err   error
}

// Start는 OAuth 2.1 플로우를 시작하고 authorization URL을 반환한다.
// REQ-MCP-007: (a) PKCE 생성 (b) authURL 구성 (c) 브라우저 오픈 (d) 콜백 리스너 시작
func (f *AuthFlow) Start(ctx context.Context) (string, error) {
	// (a) PKCE code_verifier = 32 bytes crypto/rand → base64url
	verifier, err := generatePKCEVerifier()
	if err != nil {
		return "", fmt.Errorf("PKCE verifier: %w", err)
	}
	f.verifier = verifier

	// code_challenge = SHA256(verifier) → base64url
	challenge := pkceChallenge(verifier)

	// CSRF state = 16 bytes random → base64url
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("state random: %w", err)
	}
	f.state = base64.RawURLEncoding.EncodeToString(stateBytes)

	// 콜백 리스너 (OS가 포트 자동 할당)
	f.callbackCh = make(chan callbackResult, 1)

	listener, err := startCallbackServer(ctx, f)
	if err != nil {
		return "", fmt.Errorf("callback server: %w", err)
	}

	// RedirectURI에 실제 포트 주입
	f.RedirectURI = fmt.Sprintf("http://localhost:%d/callback", listener)

	// (b) Authorization URL 구성
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {f.ClientID},
		"redirect_uri":          {f.RedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {f.state},
	}
	if len(f.Scopes) > 0 {
		params.Set("scope", strings.Join(f.Scopes, " "))
	}

	authURL := f.AuthURL + "?" + params.Encode()

	// (c) 브라우저 오픈
	openBrowser(authURL)

	return authURL, nil
}

// HandleCallback은 OAuth 콜백을 처리하고 토큰을 교환한다.
// REQ-MCP-017: state 불일치 시 ErrOAuthStateMismatch 반환
func (f *AuthFlow) HandleCallback(code, state string) (*TokenSet, error) {
	// REQ-MCP-017: state 검증
	f.mu.Lock()
	expectedState := f.state
	f.mu.Unlock()

	if state != expectedState {
		return nil, ErrOAuthStateMismatch
	}

	// Token exchange
	return f.exchangeCode(code)
}

// WaitForCallback은 콜백 채널에서 결과를 기다린다.
// 내부적으로 HTTP 리스너가 HandleCallback을 호출한다.
func (f *AuthFlow) WaitForCallback(ctx context.Context) (*TokenSet, error) {
	select {
	case result := <-f.callbackCh:
		if result.err != nil {
			return nil, result.err
		}
		return f.HandleCallback(result.code, result.state)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// exchangeCode는 authorization code를 access/refresh token으로 교환한다.
func (f *AuthFlow) exchangeCode(code string) (*TokenSet, error) {
	f.mu.Lock()
	verifier := f.verifier
	redirectURI := f.RedirectURI
	f.mu.Unlock()

	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {f.ClientID},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(f.TokenURL, body)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errBody := string(bodyBytes)
		if strings.Contains(errBody, "invalid_grant") {
			return nil, ErrReauthRequired
		}
		return nil, fmt.Errorf("token exchange failed: %s %s", resp.Status, errBody)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	ts := &TokenSet{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}
	if tokenResp.ExpiresIn > 0 {
		ts.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return ts, nil
}

// RefreshToken은 refresh token을 사용하여 새 access token을 발급한다.
// REQ-MCP-008: 401 응답 시 자동 refresh
func RefreshToken(tokenURL, clientID, refreshToken string) (*TokenSet, error) {
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}

	resp, err := http.PostForm(tokenURL, body)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(bodyBytes), "invalid_grant") {
			return nil, ErrReauthRequired
		}
		return nil, fmt.Errorf("refresh failed: %s", resp.Status)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	ts := &TokenSet{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}
	if tokenResp.ExpiresIn > 0 {
		ts.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}
	if ts.RefreshToken == "" {
		ts.RefreshToken = refreshToken // 서버가 새 refresh token을 발급하지 않은 경우 기존 것 유지
	}
	return ts, nil
}

// TokenSet은 OAuth access/refresh token 쌍이다.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

// IsExpired는 access token이 만료되었는지 확인한다.
func (ts *TokenSet) IsExpired() bool {
	if ts.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(ts.ExpiresAt)
}

// --- PKCE 헬퍼 ---

// generatePKCEVerifier는 32바이트 랜덤 PKCE code_verifier를 생성한다.
// REQ-MCP-007: 32-byte crypto/rand → base64url
func generatePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge는 code_verifier의 SHA256 해시를 base64url로 반환한다.
// REQ-MCP-007: code_challenge = SHA256(verifier) → base64url
func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// startCallbackServer는 OAuth 콜백 리스너를 시작하고 포트 번호를 반환한다.
func startCallbackServer(ctx context.Context, f *AuthFlow) (int, error) {
	// OS 자동 포트 할당 (R2 리스크 완화)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("callback listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")

		var result callbackResult
		if errParam != "" {
			result.err = fmt.Errorf("OAuth error: %s", errParam)
		} else {
			result.code = code
			result.state = state
		}

		f.callbackOnce.Do(func() {
			f.callbackCh <- result
		})

		fmt.Fprintf(w, "<html><body>Authentication complete. You can close this window.</body></html>")
	})

	srv := &http.Server{Handler: mux}
	f.server = srv

	go func() {
		_ = srv.Serve(listener)
	}()

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	return port, nil
}

// openBrowser는 OS 기본 브라우저로 URL을 연다.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

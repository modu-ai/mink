// Package oauth implements the Codex (ChatGPT/OpenAI) OAuth 2.1 + PKCE
// authorization flow and the silent token refresh mechanism.
//
// PKCE (Proof Key for Code Exchange) is used so that no client secret is
// required — the PKCE verifier acts as the proof of possession.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (ED-4, T-010, AC-CR-016)
package oauth

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
	"strings"
	"time"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// Authorization endpoint constants for the Codex (ChatGPT) OAuth 2.1 flow.
// The client ID is the public PKCE client identifier used by the official
// Codex CLI (openai/codex).  It requires no client secret because PKCE
// is the sole proof-of-possession mechanism.
const (
	codexAuthorizationURL = "https://auth.openai.com/authorize"
	codexTokenURL         = "https://auth.openai.com/oauth/token"
	codexScope            = "offline_access codex"

	// @MX:TODO: [AUTO] Confirm the public PKCE client_id with the Codex CLI
	// source once the repository becomes public.
	// @MX:REASON: PKCE flow client_id pending Codex docs / source confirmation.
	codexClientID = "app_EMWwRgTyS2JJtJiX7OV4cUCT" // public client ID, no secret required
)

// PendingAuth holds the state of an in-progress OAuth authorization.
// The caller must keep this value alive until CompleteAuthorization returns.
type PendingAuth struct {
	// AuthURL is the authorization URL to open in the user's browser.
	AuthURL string

	// Listener is the local HTTP server waiting for the redirect callback.
	// The caller should not close it; CompleteAuthorization will drain and
	// close it automatically.
	Listener net.Listener

	// State is the random CSRF-prevention value embedded in the AuthURL.
	State string

	// Verifier is the PKCE code verifier that must be sent at token exchange.
	Verifier string

	// RedirectURI is the local callback URL registered as the redirect target.
	RedirectURI string
}

// BeginAuthorization starts the OAuth 2.1 + PKCE flow.
//
// It generates a PKCE verifier and challenge, a random state token, and a
// local HTTP listener on 127.0.0.1 at a randomly assigned free port.
// The returned PendingAuth.AuthURL should be opened in the user's browser.
// Call CompleteAuthorization to receive the token after the user completes
// the consent flow.
func BeginAuthorization(ctx context.Context) (*PendingAuth, error) {
	// Generate PKCE verifier: 43 URL-safe random bytes, base64url-encoded.
	verifier, err := generatePKCEVerifier()
	if err != nil {
		return nil, fmt.Errorf("codex: generate PKCE verifier: %w", err)
	}
	challenge := computePKCEChallenge(verifier)

	// Generate state token: 32 random bytes, base64url-encoded.
	state, err := generateRandomBase64URL(32)
	if err != nil {
		return nil, fmt.Errorf("codex: generate state token: %w", err)
	}

	// Bind a listener to a random free port on loopback.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("codex: start local callback listener: %w", err)
	}

	redirectURI := fmt.Sprintf("http://%s/callback", ln.Addr().String())

	// Compose the authorization URL.
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {codexClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {codexScope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	authURL := codexAuthorizationURL + "?" + params.Encode()

	return &PendingAuth{
		AuthURL:     authURL,
		Listener:    ln,
		State:       state,
		Verifier:    verifier,
		RedirectURI: redirectURI,
	}, nil
}

// CodeExchanger is an interface that abstracts the token endpoint call so that
// tests can inject a mock HTTP server.  The production implementation posts to
// codexTokenURL.
type CodeExchanger interface {
	Exchange(ctx context.Context, code, verifier, redirectURI string) (*tokenResponse, error)
}

// httpCodeExchanger is the production CodeExchanger that posts to codexTokenURL.
type httpCodeExchanger struct {
	tokenURL string
	client   *http.Client
}

// defaultExchanger is used when CompleteAuthorization is called without an
// injected exchanger.  It targets the real Codex token endpoint.
func defaultExchanger() CodeExchanger {
	return &httpCodeExchanger{
		tokenURL: codexTokenURL,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *httpCodeExchanger) Exchange(ctx context.Context, code, verifier, redirectURI string) (*tokenResponse, error) {
	return postTokenRequest(ctx, e.client, e.tokenURL, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {codexClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	})
}

// CompleteAuthorization waits for the authorization callback on pending.Listener,
// validates the state, exchanges the code for tokens, and returns a populated
// OAuthToken.
//
// The function closes pending.Listener when it returns.
func CompleteAuthorization(ctx context.Context, pending *PendingAuth) (*credential.OAuthToken, error) {
	return completeAuthorizationWithExchanger(ctx, pending, defaultExchanger())
}

// completeAuthorizationWithExchanger is the testable core of CompleteAuthorization.
func completeAuthorizationWithExchanger(ctx context.Context, pending *PendingAuth, exchanger CodeExchanger) (*credential.OAuthToken, error) {
	defer pending.Listener.Close()

	// Channel to receive the callback code (or an error from the handler).
	type callbackResult struct {
		code string
		err  error
	}
	ch := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// Validate state parameter to prevent CSRF.
		if q.Get("state") != pending.State {
			ch <- callbackResult{err: fmt.Errorf("codex: state mismatch: expected %q, got %q",
				pending.State, q.Get("state"))}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		code := q.Get("code")
		if code == "" {
			ch <- callbackResult{err: fmt.Errorf("codex: no authorization code in callback")}
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, "Authorization complete. You may close this tab.")
		ch <- callbackResult{code: code}
	})

	srv := &http.Server{Handler: mux}
	// Serve one request then stop.
	go func() { _ = srv.Serve(pending.Listener) }()

	var result callbackResult
	select {
	case result = <-ch:
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return nil, fmt.Errorf("codex: authorization cancelled: %w", ctx.Err())
	}

	// Shut the local server down now that we have the code.
	_ = srv.Shutdown(context.Background())

	if result.err != nil {
		return nil, result.err
	}

	// Exchange the authorization code for tokens.
	tok, err := exchanger.Exchange(ctx, result.code, pending.Verifier, pending.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("codex: token exchange: %w", err)
	}

	return tok.toOAuthToken("codex"), nil
}

// ---------------------------------------------------------------------------
// Internal token exchange helpers
// ---------------------------------------------------------------------------

// tokenResponse is the JSON shape of a successful token endpoint response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// toOAuthToken converts the raw JSON response to a credential.OAuthToken.
func (r *tokenResponse) toOAuthToken(provider string) *credential.OAuthToken {
	expiresAt := time.Now().UTC().Add(time.Duration(r.ExpiresIn) * time.Second)
	return &credential.OAuthToken{
		Provider:     provider,
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresAt:    expiresAt,
		Scope:        r.Scope,
	}
}

// postTokenRequest performs an application/x-www-form-urlencoded POST to
// tokenURL, decodes the JSON response, and returns a tokenResponse.
func postTokenRequest(ctx context.Context, client *http.Client, tokenURL string, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned HTTP %d: %s",
			resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty access_token")
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty refresh_token")
	}

	return &tok, nil
}

// ---------------------------------------------------------------------------
// PKCE + random helpers
// ---------------------------------------------------------------------------

// generatePKCEVerifier returns a 43-byte base64url-encoded random string
// suitable as a PKCE code_verifier (RFC 7636 §4.1 — verifier length 43-128).
func generatePKCEVerifier() (string, error) {
	return generateRandomBase64URL(43)
}

// computePKCEChallenge computes the S256 code_challenge from a verifier.
// challenge = BASE64URL(SHA256(ASCII(verifier)))
func computePKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// generateRandomBase64URL returns a base64url-encoded (no padding) string
// built from n random bytes.
func generateRandomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
